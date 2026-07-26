<#
.SYNOPSIS
  Assert the fytables grid's Windows UIA tree for issue #45 - master-detail rows
  exposing an accessible ExpandCollapse disclosure.

.DESCRIPTION
  Launches the a11yprobe with -masterdetail (expandable master rows; rows 0 and 2
  pre-expanded), dumps the live UIA tree, and asserts:

    Each master row carries a Button with the ExpandCollapse pattern (the
    masterRowA11y disclosure node, #45): some read Expanded (the pre-expanded
    rows) and some Collapsed (the rest). Driving one disclosure's Collapse() then
    Expand() flips its state for real - proving the call routes through
    ToggleDetail, not a read-only state.

  Must run in an interactive desktop session (RDP/console); over SSH / a
  session-0 service the window is invisible to UIA and this fails.

.PARAMETER ProbeExe   Path to the built probe.
.PARAMETER UiaTree    Path to the fork's uiatree.ps1.
.PARAMETER SettleSeconds  Seconds to wait for window/UIA registration.
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
function Find-Buttons($root) {
  $cond = New-Object System.Windows.Automation.PropertyCondition($AE::ControlTypeProperty, $CT::Button)
  $root.FindAll($Scope::Descendants, $cond)
}
function ExpandState($el) {
  try { return "$($el.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Current.ExpandCollapseState)" }
  catch { return $null }
}

$proc = Start-Process -FilePath $ProbeExe -ArgumentList '-masterdetail' -PassThru
try {
  Start-Sleep -Seconds $SettleSeconds

  $tree = & powershell -NoProfile -ExecutionPolicy Bypass -File $UiaTree $proc.Id -Raw 2>&1 | Out-String
  Write-Host "===================== UIA TREE (pid $($proc.Id), -masterdetail) ====================="
  Write-Host $tree
  Write-Host "==================================================================================="

  $fail = @()

  $win = Find-Window $proc.Id
  if (-not $win) { Write-Host "no probe window (SSH / session-0?)"; exit 1 }

  # Disclosure nodes are Buttons that carry an ExpandCollapse pattern.
  $disc = @(Find-Buttons $win | Where-Object { $null -ne (ExpandState $_) })
  if ($disc.Count -lt 1) { $fail += "no Button carrying an ExpandCollapse pattern (master-row disclosure)" }
  $expanded = @($disc | Where-Object { (ExpandState $_) -eq 'Expanded' })
  $collapsed = @($disc | Where-Object { (ExpandState $_) -eq 'Collapsed' })
  Write-Host ("MASTERDETAIL disclosures: {0} total, {1} expanded, {2} collapsed" -f $disc.Count, $expanded.Count, $collapsed.Count)
  if ($expanded.Count -lt 1) { $fail += "no expanded master-row disclosure (rows 0 and 2 were pre-expanded)" }
  if ($collapsed.Count -lt 1) { $fail += "no collapsed master-row disclosure (other rows should be collapsed)" }

  # Drive one for real: collapse an expanded disclosure, then expand it back.
  if ($expanded.Count -ge 1) {
    $lbl = $expanded[0].Current.Name
    $expanded[0].GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Collapse()
    Start-Sleep -Milliseconds 1200
    $refind = @(Find-Buttons (Find-Window $proc.Id) | Where-Object { $_.Current.Name -eq $lbl }) | Select-Object -First 1
    $st = if ($refind) { ExpandState $refind } else { '<gone>' }
    Write-Host ("MASTERDETAIL '$lbl' after Collapse(): $st")
    if ($st -ne 'Collapsed') { $fail += "disclosure '$lbl' did not become Collapsed after Collapse() (got $st)" }
    if ($refind) {
      $refind.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Expand()
      Start-Sleep -Milliseconds 1200
      $refind2 = @(Find-Buttons (Find-Window $proc.Id) | Where-Object { $_.Current.Name -eq $lbl }) | Select-Object -First 1
      $st2 = if ($refind2) { ExpandState $refind2 } else { '<gone>' }
      Write-Host ("MASTERDETAIL '$lbl' after Expand(): $st2")
      if ($st2 -ne 'Expanded') { $fail += "disclosure '$lbl' did not return to Expanded after Expand() (got $st2)" }
    }
  }

  if ($fail.Count -gt 0) {
    Write-Host "A11Y UIA #45 ASSERTIONS FAILED:" -ForegroundColor Red
    $fail | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    exit 1
  }
  Write-Host "A11Y UIA #45 ASSERTIONS PASSED" -ForegroundColor Green
}
finally {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
}
