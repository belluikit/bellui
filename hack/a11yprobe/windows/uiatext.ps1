# uiatext.ps1 <pid> [name] — exercise the UIA Text pattern on the first text
# element (Edit/Document control with TextPattern) under the process, the way
# Narrator/NVDA text review does: document range, selection, unit navigation
# (character/word/line), and a settable selection.
param(
    [Parameter(Mandatory = $true)][int]$Pid_,
    [string]$Name
)
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes

$ae = [System.Windows.Automation.AutomationElement]
$procCond = New-Object System.Windows.Automation.PropertyCondition($ae::ProcessIdProperty, $Pid_)
$cond = New-Object System.Windows.Automation.PropertyCondition(
    $ae::IsTextPatternAvailableProperty, $true)
if ($Name) {
    $nameCond = New-Object System.Windows.Automation.PropertyCondition($ae::NameProperty, $Name)
    $cond = New-Object System.Windows.Automation.AndCondition($cond, $nameCond)
} else {
    # Prefer real text inputs; selectable labels expose TextPattern too.
    $editCond = New-Object System.Windows.Automation.PropertyCondition(
        $ae::ControlTypeProperty, [System.Windows.Automation.ControlType]::Edit)
    $cond = New-Object System.Windows.Automation.AndCondition($cond, $editCond)
}

$el = $null
foreach ($w in $ae::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $procCond)) {
    $el = $w.FindFirst([System.Windows.Automation.TreeScope]::Subtree, $cond)
    if ($el) { break }
}
if (-not $el) {
    Write-Error "no TextPattern element under pid $Pid_"
    exit 1
}
Write-Output ("element: [{0}] `"{1}`"" -f ($el.Current.ControlType.ProgrammaticName -replace '^ControlType\.', ''), $el.Current.Name)

$tp = $el.GetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern)
$doc = $tp.DocumentRange
Write-Output ("document ({0} chars): `"{1}`"" -f $doc.GetText(-1).Length, $doc.GetText(80))

$sel = $tp.GetSelection()
if ($sel.Count -gt 0) {
    Write-Output ("selection: `"{0}`"" -f $sel[0].GetText(-1))
}

# Unit navigation: first word, second word, first line.
$r = $doc.Clone()
$r.ExpandToEnclosingUnit([System.Windows.Automation.Text.TextUnit]::Word)
Write-Output ("word[0]: `"{0}`"" -f $r.GetText(-1))
$moved = $r.Move([System.Windows.Automation.Text.TextUnit]::Word, 1)
Write-Output ("word[1] (moved {0}): `"{1}`"" -f $moved, $r.GetText(-1))
$r = $doc.Clone()
$r.ExpandToEnclosingUnit([System.Windows.Automation.Text.TextUnit]::Line)
Write-Output ("line[0]: `"{0}`"" -f ($r.GetText(-1) -replace "`n", '\n'))
$moved = $r.Move([System.Windows.Automation.Text.TextUnit]::Line, 1)
Write-Output ("line[1] (moved {0}): `"{1}`"" -f $moved, ($r.GetText(-1) -replace "`n", '\n'))

# Settable selection: select the second word, then read it back.
$r = $doc.Clone()
$r.ExpandToEnclosingUnit([System.Windows.Automation.Text.TextUnit]::Word)
[void]$r.Move([System.Windows.Automation.Text.TextUnit]::Word, 1)
$r.Select()
$sel = $tp.GetSelection()
Write-Output ("selection after Select(): `"{0}`"" -f $sel[0].GetText(-1))
