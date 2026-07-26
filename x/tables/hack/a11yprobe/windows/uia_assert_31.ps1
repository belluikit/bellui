<#
.SYNOPSIS
  Assert the fytables grid's Windows UIA provider tree for issue #31 - the
  checkbox-column cell semantics (#29) and the group-header ExpandCollapse
  *action* (#29), on top of the #15/#16 model.

.DESCRIPTION
  Launches the a11yprobe with -checkbox (leading checkbox-selection column,
  two rows pre-selected), dumps the live UIA tree with the fork's uiatree.ps1,
  and asserts two things the Go unit tests can't reach:

    Checkbox cell (#29) - the Grid pattern now reports 6 columns (pinned-left +
      checkbox + 3 main + pinned-right); the checkbox is rendered in its visual
      slot just right of the pinned-left band (#35), so pinned-left 'id' stays at
      column 0 and the checkbox cell reports column 1, with the main/pinned-right
      columns shifted right; a nameless DataItem wraps an interactive CheckBox
      (Toggle pattern) whose state tracks the row's selection.

    Group ExpandCollapse action (#29) - invoking the ExpandCollapse pattern's
      Collapse()/Expand() on a group-header Group element actually toggles the
      group (state flips AND the Grid RowCount drops/restores as the group's
      child rows leave/re-enter the display), proving the call is routed through
      ToggleGroup rather than a no-op read-only state.

  Must run in an interactive desktop session (RDP/console). Over SSH / a
  session-0 service the window is invisible to UIA and this fails.

.PARAMETER ProbeExe
  Path to the built probe (go build -tags accessibility -o a11yprobe.exe .).

.PARAMETER UiaTree
  Path to the fork's uiatree.ps1.

.PARAMETER SettleSeconds
  Seconds to wait for GPU init + window/UIA registration before dumping.
#>
param(
  [Parameter(Mandatory = $true)][string]$ProbeExe,
  [Parameter(Mandatory = $true)][string]$UiaTree,
  [int]$SettleSeconds = 10
)
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes

$AE = [System.Windows.Automation.AutomationElement]
$Scope = [System.Windows.Automation.TreeScope]
$CT = [System.Windows.Automation.ControlType]

function Find-Window([int]$ProcId) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::ProcessIdProperty, $ProcId)
  $AE::RootElement.FindFirst($Scope::Children, $cond)
}
function Find-ByType($root, $controlType) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $controlType)
  $root.FindAll($Scope::Descendants, $cond)
}
function Grid-RowCount($win) {
  $t = (Find-ByType $win $CT::Table) | Select-Object -First 1
  if (-not $t) { return -1 }
  $gp = $t.GetCurrentPattern([System.Windows.Automation.GridPattern]::Pattern)
  $gp.Current.RowCount
}

$proc = Start-Process -FilePath $ProbeExe -ArgumentList '-checkbox' -PassThru
try {
  Start-Sleep -Seconds $SettleSeconds

  $tree = & powershell -NoProfile -ExecutionPolicy Bypass -File $UiaTree $proc.Id -Raw 2>&1 | Out-String
  Write-Host "===================== UIA TREE (pid $($proc.Id), -checkbox) ====================="
  Write-Host $tree
  Write-Host "==============================================================================="

  $fail = @()

  # --- Part 1: checkbox-column cell semantics (#29) ---
  $grid = [regex]::Match($tree, '\[Table\]\s+"Problems".*?grid=(\d+)x(\d+)')
  $cols = 0
  if (-not $grid.Success) {
    $fail += "no Table `"Problems`" carrying a Grid pattern (grid=RxC)"
  }
  else {
    $cols = [int]$grid.Groups[2].Value
    if ($cols -ne 6) { $fail += "Grid ColumnCount=$cols, want 6 (checkbox + pinned-left + 3 main + pinned-right)" }
  }

  $cells = [regex]::Matches($tree, '\[DataItem\][^\r\n]*?cell=(\d+),(\d+)')
  $colSet = @{}
  foreach ($m in $cells) { $colSet[[int]$m.Groups[2].Value] = $true }
  if ($cols -gt 0 -and -not $colSet.ContainsKey($cols - 1)) { $fail += "no DataItem in pinned-right column $($cols - 1) ('status')" }

  # Column model (#35): the checkbox column is rendered at the start of the main
  # area - its visual slot just right of the single pinned-left column - so the
  # pinned-left 'id' stays at column 0 and the checkbox cell reports column 1,
  # matching visual left-to-right order (id, checkbox, name, ...).
  if (([regex]::Matches($tree, '\[DataItem\]\s+"10\d\d"[^\r\n]*cell=\d+,0')).Count -lt 1) {
    $fail += "pinned-left 'id' is not at column 0"
  }
  # The checkbox cell is the only nameless DataItem (brace right after [DataItem],
  # no quoted name); every such cell must sit at the checkbox's visual slot, 1.
  $cbCells = [regex]::Matches($tree, '\[DataItem\]\s+\{[^\r\n]*cell=\d+,(\d+)')
  if ($cbCells.Count -lt 1) { $fail += "no nameless checkbox DataItem found" }
  foreach ($m in $cbCells) {
    if ([int]$m.Groups[1].Value -ne 1) {
      $fail += "checkbox cell at column $($m.Groups[1].Value), want 1 (visual slot after pinned-left)"
    }
  }

  # Interactive checkbox reachable inside the grid.
  if ($tree -notmatch '\[CheckBox\][^\r\n]*\{[^}]*Toggle') {
    $fail += "no CheckBox with a Toggle pattern (interactive select control not reachable in the grid)"
  }

  # The checkbox reflects the row's selection state: this fork build surfaces
  # StateSelected as the inner CheckBox's Toggle=On, so the two pre-selected rows
  # must read On and the rest Off - proving it is per-row state, not always-on.
  $on = ([regex]::Matches($tree, '\[CheckBox\][^\r\n]*toggle=On')).Count
  $off = ([regex]::Matches($tree, '\[CheckBox\][^\r\n]*toggle=Off')).Count
  if ($on -lt 2) { $fail += "expected the 2 pre-selected rows' checkboxes to read Toggle=On, got $on" }
  if ($off -lt 1) { $fail += "expected unselected rows' checkboxes to read Toggle=Off (per-row state), got $off" }

  # --- Part 2: group ExpandCollapse *action* (#29) ---
  $win = Find-Window $proc.Id
  if (-not $win) {
    $fail += "could not re-acquire the probe window for the ExpandCollapse action test"
  }
  else {
    $groups = Find-ByType $win $CT::Group
    $group = $null
    foreach ($g in $groups) { if ($g.Current.Name -like 'North*') { $group = $g; break } }
    if (-not $group) {
      $fail += "no 'North' Group element to drive ExpandCollapse"
    }
    else {
      $ecp = $group.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern)
      $stateBefore = $ecp.Current.ExpandCollapseState
      $rowsBefore = Grid-RowCount $win
      Write-Host ("EXPANDCOLLAPSE before: state={0} gridRows={1}" -f $stateBefore, $rowsBefore)

      $ecp.Collapse()
      Start-Sleep -Seconds 2
      # The provider rebuilt; re-acquire the group and grid.
      $win2 = Find-Window $proc.Id
      $g2 = $null
      foreach ($g in (Find-ByType $win2 $CT::Group)) { if ($g.Current.Name -like 'North*') { $g2 = $g; break } }
      $stateAfter = if ($g2) { $g2.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Current.ExpandCollapseState } else { '<gone>' }
      $rowsAfter = Grid-RowCount $win2
      Write-Host ("EXPANDCOLLAPSE afterCollapse: state={0} gridRows={1}" -f $stateAfter, $rowsAfter)

      if ("$stateAfter" -ne 'Collapsed') { $fail += "group state did not become Collapsed after Collapse() (got $stateAfter)" }
      if ($rowsAfter -ge $rowsBefore) { $fail += "Grid RowCount did not drop after Collapse() ($rowsBefore -> $rowsAfter): action did not reach ToggleGroup" }

      # Round-trip: expand again, expect the rows to come back.
      if ($g2) {
        $g2.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Expand()
        Start-Sleep -Seconds 2
        $win3 = Find-Window $proc.Id
        $rowsReExpanded = Grid-RowCount $win3
        Write-Host ("EXPANDCOLLAPSE afterExpand: gridRows={0}" -f $rowsReExpanded)
        if ($rowsReExpanded -lt $rowsBefore) { $fail += "Grid RowCount did not restore after Expand() ($rowsReExpanded < $rowsBefore)" }
      }
    }
  }

  if ($fail.Count -gt 0) {
    Write-Host "A11Y UIA #31 ASSERTIONS FAILED:" -ForegroundColor Red
    $fail | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    exit 1
  }
  Write-Host "A11Y UIA #31 ASSERTIONS PASSED" -ForegroundColor Green
}
finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
