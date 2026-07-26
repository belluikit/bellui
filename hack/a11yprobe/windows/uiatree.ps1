# uiatree.ps1 <pid> [-Raw] — dump the UIA tree of every top-level window of
# the process, as a UIA client sees it: control type, name, value, supported
# patterns, bounds. The default walks the control view (what screen readers
# navigate); -Raw walks the raw view, which also shows structural groups the
# provider excludes from the control view.
param(
    [Parameter(Mandatory = $true)][int]$Pid_,
    [switch]$Raw
)
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes

$ae = [System.Windows.Automation.AutomationElement]
$root = $ae::RootElement
$cond = New-Object System.Windows.Automation.PropertyCondition($ae::ProcessIdProperty, $Pid_)
$windows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $cond)
if ($windows.Count -eq 0) {
    Write-Error "no top-level windows for pid $Pid_"
    exit 1
}

$walker = if ($Raw) {
    [System.Windows.Automation.TreeWalker]::RawViewWalker
} else {
    [System.Windows.Automation.TreeWalker]::ControlViewWalker
}

function Describe($el) {
    $c = $el.Current
    $ct = $c.ControlType.ProgrammaticName -replace '^ControlType\.', ''
    $parts = @("[$ct]")
    if ($c.Name) { $parts += "`"$($c.Name)`"" }
    $patterns = ($el.GetSupportedPatterns() | ForEach-Object {
            $_.ProgrammaticName -replace 'PatternIdentifiers\.Pattern$', '' -replace '^.*\.', ''
        }) -join ','
    if ($patterns) { $parts += "{$patterns}" }
    try {
        $vp = $el.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
        $parts += "value=`"$($vp.Current.Value)`""
    } catch {}
    try {
        $rp = $el.GetCurrentPattern([System.Windows.Automation.RangeValuePattern]::Pattern)
        $parts += ("range={0}..{1}={2}" -f $rp.Current.Minimum, $rp.Current.Maximum, $rp.Current.Value)
    } catch {}
    try {
        $tp = $el.GetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern)
        $parts += "toggle=$($tp.Current.ToggleState)"
    } catch {}
    try {
        $sp = $el.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern)
        $parts += "selected=$($sp.Current.IsSelected)"
    } catch {}
    try {
        $ep = $el.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern)
        $parts += "expand=$($ep.Current.ExpandCollapseState)"
    } catch {}
    try {
        $gp = $el.GetCurrentPattern([System.Windows.Automation.GridPattern]::Pattern)
        $parts += ("grid={0}x{1}" -f $gp.Current.RowCount, $gp.Current.ColumnCount)
    } catch {}
    try {
        $gi = $el.GetCurrentPattern([System.Windows.Automation.GridItemPattern]::Pattern)
        $parts += ("cell={0},{1}" -f $gi.Current.Row, $gi.Current.Column)
    } catch {}
    $r = $c.BoundingRectangle
    $parts += ("({0},{1} {2}x{3})" -f [int]$r.X, [int]$r.Y, [int]$r.Width, [int]$r.Height)
    if ($c.HasKeyboardFocus) { $parts += 'FOCUSED' }
    if ($c.IsOffscreen) { $parts += 'offscreen' }
    if (-not $c.IsEnabled) { $parts += 'disabled' }
    $parts -join ' '
}

function Walk($el, $depth) {
    Write-Output (('  ' * $depth) + (Describe $el))
    $child = $walker.GetFirstChild($el)
    while ($null -ne $child) {
        Walk $child ($depth + 1)
        $child = $walker.GetNextSibling($child)
    }
}

foreach ($w in $windows) {
    Write-Output "=== window: `"$($w.Current.Name)`" ==="
    Walk $w 0
}
