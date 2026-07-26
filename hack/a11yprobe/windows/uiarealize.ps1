# uiarealize.ps1 <pid> [row] [col] - exercise the off-viewport virtualized-cell
# realize seam (fyne#637, Stage 3) against a running a11yrealizeprobe.
#
# It finds the process's grid element (the one exposing the UIA Grid pattern),
# reports its full extent, then calls GridPattern.GetItem(row, col) for a cell
# far past the viewport. A non-null result whose Name / GridItem coordinates
# match the request proves the bridge realized the cell on demand through the
# seam rather than returning null (the old "only visible cells exist" behavior).
# It then round-trips ContainingGrid and calls VirtualizedItem.Realize (which
# routes to AccessibleScrollCellIntoView - watch the probe's stdout).
param(
    [Parameter(Mandatory = $true)][int]$Pid_,
    [int]$Row = 500000,
    [int]$Col = 1
)
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes
$A = [System.Windows.Automation.AutomationElement]
$scope = [System.Windows.Automation.TreeScope]

$cond = New-Object System.Windows.Automation.PropertyCondition($A::ProcessIdProperty, $Pid_)
$win = $A::RootElement.FindFirst($scope::Children, $cond)
if (-not $win) { Write-Error "no top-level window for pid $Pid_"; exit 1 }

$gridCond = New-Object System.Windows.Automation.PropertyCondition($A::IsGridPatternAvailableProperty, $true)
$grid = $win.FindFirst($scope::Descendants, $gridCond)
if (-not $grid) { Write-Error "no element exposing the Grid pattern"; exit 1 }

$gp = $grid.GetCurrentPattern([System.Windows.Automation.GridPattern]::Pattern)
Write-Output ("grid `"$($grid.Current.Name)`" extent: {0} rows x {1} cols" -f $gp.Current.RowCount, $gp.Current.ColumnCount)

Write-Output "GetItem($Row, $Col) [off-viewport realize]:"
$cell = $gp.GetItem($Row, $Col)
if (-not $cell) { Write-Error "GetItem returned null - realize failed"; exit 1 }
Write-Output ("  name    = `"$($cell.Current.Name)`"")
$expected = "R${Row}C${Col}"
if ($cell.Current.Name -ne $expected) {
    Write-Error "  MISMATCH: expected name `"$expected`""; exit 1
}

$gi = $cell.GetCurrentPattern([System.Windows.Automation.GridItemPattern]::Pattern)
Write-Output ("  gridItem= row $($gi.Current.Row), col $($gi.Current.Column)")
if ($gi.Current.Row -ne $Row -or $gi.Current.Column -ne $Col) {
    Write-Error "  MISMATCH: GridItem coordinates"; exit 1
}

$cg = $gi.Current.ContainingGrid
if ($cg) { Write-Output ("  containingGrid = `"$($cg.Current.Name)`"") }

try {
    $vi = $cell.GetCurrentPattern([System.Windows.Automation.VirtualizedItemPattern]::Pattern)
    $vi.Realize()
    Write-Output "  VirtualizedItem.Realize() -> AccessibleScrollCellIntoView (see probe stdout)"
} catch {
    Write-Output "  (no VirtualizedItem pattern: $($_.Exception.Message))"
}

# Stable identity: re-reading the same cell returns an element with the same
# UIA runtime id (the realized-cell cache reuses the provider across reads).
$again = $gp.GetItem($Row, $Col)
$same = [System.Windows.Automation.Automation]::Compare($cell, $again)
Write-Output ("  re-GetItem same element: {0}" -f $same)
if (-not $same) { Write-Error "  realized cell identity is not stable across reads"; exit 1 }

# Out-of-range coordinates must be rejected, not realized.
$oob = $gp.Current.RowCount
try {
    $null = $gp.GetItem($oob, 0)
    Write-Error "  GetItem out-of-range unexpectedly returned an element"; exit 1
} catch {
    Write-Output "  GetItem($oob, 0) out-of-range rejected as expected"
}

Write-Output "OK: off-viewport cell realized through the seam"
