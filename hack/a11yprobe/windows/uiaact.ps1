# uiaact.ps1 <pid> <name> <action> [value] — find the first element with the
# given UIA Name under the process and perform a pattern action on it:
#   invoke | toggle | select | expand | collapse | focus | setrange <n> | setvalue <text>
# An EMPTY <name> matches the first element that supports the action's pattern
# instead (e.g. `uiaact <pid> "" setvalue X` drives the first Value-settable
# element — useful for an unnamed Entry). The probe app prints a PROBE line when
# the action reaches the widget.
param(
    [Parameter(Mandatory = $true)][int]$Pid_,
    [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Name,
    [Parameter(Mandatory = $true)][string]$Action,
    [string]$Value
)
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes

$ae = [System.Windows.Automation.AutomationElement]
$procCond = New-Object System.Windows.Automation.PropertyCondition($ae::ProcessIdProperty, $Pid_)

if ($Name) {
    $cond = New-Object System.Windows.Automation.PropertyCondition($ae::NameProperty, $Name)
} else {
    # No name: match the first element that supports the action's pattern.
    $availProp = @{
        'invoke'   = $ae::IsInvokePatternAvailableProperty
        'toggle'   = $ae::IsTogglePatternAvailableProperty
        'select'   = $ae::IsSelectionItemPatternAvailableProperty
        'expand'   = $ae::IsExpandCollapsePatternAvailableProperty
        'collapse' = $ae::IsExpandCollapsePatternAvailableProperty
        'setrange' = $ae::IsRangeValuePatternAvailableProperty
        'setvalue' = $ae::IsValuePatternAvailableProperty
    }[$Action.ToLower()]
    if (-not $availProp) {
        Write-Error "an empty name needs a pattern-bearing action, not `"$Action`""
        exit 1
    }
    $cond = New-Object System.Windows.Automation.PropertyCondition($availProp, $true)
    # Read-only labels also expose ValuePattern, so for setvalue narrow to an
    # actual Edit control (the entry), not the first Value-bearing element.
    if ($Action.ToLower() -eq 'setvalue') {
        $editCond = New-Object System.Windows.Automation.PropertyCondition(
            $ae::ControlTypeProperty, [System.Windows.Automation.ControlType]::Edit)
        $cond = New-Object System.Windows.Automation.AndCondition($cond, $editCond)
    }
}

$el = $null
foreach ($w in $ae::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $procCond)) {
    $el = $w.FindFirst([System.Windows.Automation.TreeScope]::Subtree, $cond)
    if ($el) { break }
}
if (-not $el) {
    Write-Error "no element matching name=`"$Name`" / action=`"$Action`" under pid $Pid_"
    exit 1
}

switch ($Action.ToLower()) {
    'invoke' {
        $el.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    }
    'toggle' {
        $el.GetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern).Toggle()
    }
    'select' {
        $el.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Select()
    }
    'expand' {
        $el.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Expand()
    }
    'collapse' {
        $el.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Collapse()
    }
    'focus' {
        $el.SetFocus()
    }
    'setrange' {
        $el.GetCurrentPattern([System.Windows.Automation.RangeValuePattern]::Pattern).SetValue([double]$Value)
    }
    'setvalue' {
        $vp = $el.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
        $vp.SetValue($Value)
        Start-Sleep -Milliseconds 300
        Write-Output ("value readback: `"{0}`"" -f $el.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).Current.Value)
    }
    default {
        Write-Error "unknown action `"$Action`""
        exit 1
    }
}
$ct = $el.Current.ControlType.ProgrammaticName -replace '^ControlType\.', ''
Write-Output ("performed $Action on [{0}] `"{1}`"" -f $ct, $el.Current.Name)
