<#
.SYNOPSIS
  Assert the fytables grid's Windows UIA provider tree for issues #15/#16.

.DESCRIPTION
  Launches the fytables a11yprobe, dumps the live UIA tree with the fyne fork's
  app-agnostic uiatree.ps1, prints it, then asserts the grid exposes the
  platform semantics the a11y work added:

    #15  A Table element (the DataGrid, named "Problems") whose Grid pattern
         reports ColumnCount = 5 - pinned-left + 3 main + pinned-right, the
         absolute column model - and RowCount = the full display extent.
         DataItem cells expose GridItem row/column in that absolute space,
         including the extreme (pinned) columns. Region group-header rows appear
         as Group elements.
    #16  RowCount (full extent) far exceeds the count of materialized DataItem
         cells - the grid virtualizes, surfacing "row R of N" without realizing
         every row.

  Must run in an interactive desktop session (RDP/console). Over SSH / a
  session-0 service the window is invisible to UIA and this fails. See
  hack/a11yprobe/README (fork) for the session requirement.

.PARAMETER ProbeExe
  Path to the built probe (go build -tags accessibility -o a11yprobe.exe .).

.PARAMETER UiaTree
  Path to the fork's uiatree.ps1 (resolve via
  `go list -m -f '{{.Dir}}' github.com/belluikit/bellui` + hack/a11yprobe/windows).

.PARAMETER SettleSeconds
  Seconds to wait for GPU init + window/UIA registration before dumping.
#>
param(
  [Parameter(Mandatory = $true)][string]$ProbeExe,
  [Parameter(Mandatory = $true)][string]$UiaTree,
  [int]$SettleSeconds = 10
)
$ErrorActionPreference = "Stop"

$proc = Start-Process -FilePath $ProbeExe -PassThru
try {
  Start-Sleep -Seconds $SettleSeconds

  $tree = & powershell -NoProfile -ExecutionPolicy Bypass -File $UiaTree $proc.Id -Raw 2>&1 | Out-String
  Write-Host "===================== UIA TREE (pid $($proc.Id)) ====================="
  Write-Host $tree
  Write-Host "======================================================================"

  $fail = @()

  # #15/#16: the DataGrid is a Table with a Grid pattern reporting the extent.
  $grid = [regex]::Match($tree, '\[Table\]\s+"Problems".*?grid=(\d+)x(\d+)')
  $rows = 0; $cols = 0
  if (-not $grid.Success) {
    $fail += "no Table `"Problems`" carrying a Grid pattern (grid=RxC)"
  }
  else {
    $rows = [int]$grid.Groups[1].Value
    $cols = [int]$grid.Groups[2].Value
    if ($cols -ne 5) { $fail += "Grid ColumnCount=$cols, want 5 (pinned-left + 3 main + pinned-right)" }
    if ($rows -lt 100) { $fail += "Grid RowCount=$rows, want >=100 (full virtualized extent)" }
  }

  # #15: cells expose GridItem row/column.
  $cells = [regex]::Matches($tree, '\[DataItem\].*?cell=(\d+),(\d+)')
  if ($cells.Count -eq 0) {
    $fail += "no DataItem cells carrying a GridItem pattern (cell=r,c)"
  }

  # #16: materialized cells must be far fewer than the full RowCount (virtualized).
  if ($grid.Success -and $cells.Count -ge $rows) {
    $fail += "materialized $($cells.Count) cells >= RowCount $rows - not virtualizing"
  }

  # #15: pinned columns occupy the extreme absolute columns 0 and cols-1.
  if ($cells.Count -gt 0 -and $cols -gt 0) {
    $haveCol0 = $false; $haveColLast = $false
    foreach ($m in $cells) {
      $c = [int]$m.Groups[2].Value
      if ($c -eq 0) { $haveCol0 = $true }
      if ($c -eq ($cols - 1)) { $haveColLast = $true }
    }
    if (-not $haveCol0) { $fail += "no cell in absolute column 0 (pinned-left 'id')" }
    if (-not $haveColLast) { $fail += "no cell in absolute column $($cols - 1) (pinned-right 'status')" }
  }

  # #15: region group-header rows surface as Group elements.
  $groups = [regex]::Matches($tree, '\[Group\]\s+"(North|South|East|West)[^"]*"')
  if ($groups.Count -eq 0) {
    $fail += "no region Group elements (group-header rows)"
  }

  if ($fail.Count -gt 0) {
    Write-Host "A11Y UIA ASSERTIONS FAILED:" -ForegroundColor Red
    $fail | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    exit 1
  }

  Write-Host "A11Y UIA ASSERTIONS PASSED" -ForegroundColor Green
  Write-Host ("  Table 'Problems' grid={0}x{1}; {2} materialized cells; {3} group headers" -f `
      $rows, $cols, $cells.Count, $groups.Count)
}
finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
