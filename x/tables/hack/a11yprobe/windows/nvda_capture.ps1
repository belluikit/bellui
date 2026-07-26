<#
.SYNOPSIS
  Capture what NVDA actually speaks when navigating the fytables grid (#566),
  as a diffable transcript. Companion to uia_assert.ps1 (which checks the
  provider tree NVDA reads).

.DESCRIPTION
  Runs real NVDA with the fyne fork's speechcapture global plugin (logs every
  queued utterance), launches the a11yprobe grid, drives a focus walk, and
  merges NVDA's speech with per-step markers into a transcript.

  Reliable (fires on UIA focus events, independent of key injection): the grid
  announcing as a table, and role/label/position spoken when focus lands on it.
  NOT reliable via injection: arrow-key cell-to-cell navigation (NVDA filters
  synthetic keystrokes) - that half needs a human listen. Both are exercised
  below; read the transcript to see which the platform surfaced.

  With -Checkbox the probe runs with its checkbox-selection column (#29/#31) and
  two extra steps are driven through real UIA events NVDA tracks (not injected
  keys): SetFocus on the interactive CheckBox inside a pre-selected row (expect
  its checked state spoken), and the group header's ExpandCollapse pattern driven
  Collapse()/Expand() (expect the state-change announced). This covers #36.

  Interactive desktop session only (RDP/console). Over SSH / a session-0
  service NVDA never sees the window. See README.md.

.PARAMETER NvdaExe    Path to portable nvda.exe.
.PARAMETER ProbeExe   Path to the built a11yprobe.exe.
.PARAMETER PluginSrc  Path to the fork's speechcapture plugin dir (contains __init__.py).
.PARAMETER WorkDir    Scratch dir for NVDA config, logs and the transcript.
.PARAMETER Checkbox   Launch the probe with -checkbox and run the checkbox-cell +
                      group ExpandCollapse steps (#36).
#>
param(
  [Parameter(Mandatory = $true)][string]$NvdaExe,
  [Parameter(Mandatory = $true)][string]$ProbeExe,
  [Parameter(Mandatory = $true)][string]$PluginSrc,
  [string]$WorkDir = (Join-Path $env:TEMP "fytables_nvda"),
  [switch]$Checkbox
)
$ErrorActionPreference = "Continue"
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @"
using System;using System.Runtime.InteropServices;
public class Native {
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern void keybd_event(byte vk,byte s,uint f,IntPtr e);
}
"@

$cfg       = Join-Path $WorkDir "nvdacfg"
$plugDst   = Join-Path $cfg "scratchpad\globalPlugins\speechcapture"
$speechLog = Join-Path $WorkDir "nvda_speech.log"
$markLog   = Join-Path $WorkDir "marks.log"
$report    = Join-Path $WorkDir "transcript.txt"

New-Item -ItemType Directory -Force -Path $plugDst | Out-Null
Copy-Item (Join-Path $PluginSrc "__init__.py") (Join-Path $plugDst "__init__.py") -Force
@"
schemaVersion = 13
[development]
	enableScratchpadDir = True
[general]
	showWelcomeDialogAtStartup = False
	askToExit = False
	playStartAndExitSounds = False
[update]
	autoCheck = False
[speech]
	synth = espeak
	[[espeak]]
		rate = 90
"@ | Set-Content -Path (Join-Path $cfg "nvda.ini") -Encoding ASCII
Remove-Item $speechLog, $markLog -ErrorAction SilentlyContinue

$KEYUP = 0x0002
$VK = @{ TAB = 0x09; DOWN = 0x28; UP = 0x26; LEFT = 0x25; RIGHT = 0x27 }
function Key([int]$vk) {
  [Native]::keybd_event([byte]$vk, 0, 0, [IntPtr]::Zero); Start-Sleep -Milliseconds 60
  [Native]::keybd_event([byte]$vk, 0, $KEYUP, [IntPtr]::Zero)
}
function Mark([string]$d) {
  $ms = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
  Add-Content -Path $markLog -Value ("{0}`tMARK`t{1}" -f $ms, $d)
  Write-Host ("MARK: " + $d)
  Start-Sleep -Milliseconds 1600
}

Write-Host "Starting NVDA ($NvdaExe) ..."
$env:NVDA_SPEECH_LOG = $speechLog
Start-Process -FilePath $NvdaExe -ArgumentList '-c', $cfg, '--minimal' | Out-Null
Start-Sleep -Seconds 9

$AE = [System.Windows.Automation.AutomationElement]
$TS = [System.Windows.Automation.TreeScope]
$CT = [System.Windows.Automation.ControlType]
Write-Host "Launching probe ..."
if ($Checkbox) { $probe = Start-Process -FilePath $ProbeExe -ArgumentList '-checkbox' -PassThru }
else { $probe = Start-Process -FilePath $ProbeExe -PassThru }
Start-Sleep -Seconds 7

$root = $AE::RootElement
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $probe.Id)
$win = $root.FindFirst($TS::Children, $cond)
if (-not $win) { Write-Host "ERROR: probe window not found (SSH / session-0? see README)"; $probe | Stop-Process -Force; Get-Process nvda -ErrorAction SilentlyContinue | Stop-Process -Force; exit 1 }
[Native]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
Start-Sleep -Milliseconds 800

function FindByType($type) {
  $c = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, $type)
  return $win.FindFirst($TS::Descendants, $c)
}

# --- Walk ---------------------------------------------------------------------
Mark "grid window foreground"

# Tab moves fyne focus into the grid; capture what NVDA announces on entry.
for ($i = 1; $i -le 2; $i++) { Key $VK.TAB; Mark "Tab #$i (expect grid/table focus announcement)" }

# UIA SetFocus on the Table itself (fires a focus event NVDA tracks).
$table = FindByType $CT::Table
if ($table) { Mark ("focus Table '" + $table.Current.Name + "' (expect table + dimensions)"); try { $table.SetFocus() } catch {} }
else { Mark "Table element not found" }

# Arrow within the grid: cell-to-cell. Injected arrows may be filtered by NVDA;
# captured here to show what the platform surfaces (the human-listen residual).
foreach ($k in @('DOWN', 'RIGHT', 'RIGHT')) { Key $VK.$k; Mark "arrow $k (expect next cell label + row/col)" }

# Focus a data cell and the group header directly via UIA.
$cell = FindByType $CT::DataItem
if ($cell) { Mark ("focus DataItem cell (expect label + grid position)"); try { $cell.SetFocus() } catch {} }
else { Mark "DataItem cell not found" }

$group = FindByType $CT::Group
if ($group) { Mark ("focus Group '" + $group.Current.Name + "' (expect name + expanded state)"); try { $group.SetFocus() } catch {} }
else { Mark "Group element not found" }

# --- #36: checkbox cell + group ExpandCollapse (driven by real UIA events) -----
if ($Checkbox) {
  # Focus the interactive select control. The first CheckBox is in a pre-selected
  # row, so NVDA should speak a checked state (not an injected key - a focus event).
  $check = FindByType $CT::CheckBox
  if ($check) { Mark "focus CheckBox in pre-selected row (expect 'check box checked')"; try { $check.SetFocus() } catch {} }
  else { Mark "CheckBox not found" }

  # Drive the group's ExpandCollapse pattern and capture NVDA's state-change
  # announcement. Collapse()/Expand() raise real UIA events, unlike injected keys.
  $grp = FindByType $CT::Group
  if ($grp) {
    try { $grp.SetFocus() } catch {}
    Mark ("focus Group '" + $grp.Current.Name + "' for toggle (expect 'expanded')")
    try {
      $grp.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Collapse()
      Mark "ExpandCollapse.Collapse() (expect 'collapsed')"
      # Provider rebuilds on toggle; re-acquire the group before expanding.
      $grp2 = FindByType $CT::Group
      if ($grp2) {
        try { $grp2.SetFocus() } catch {}
        $grp2.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern).Expand()
        Mark "ExpandCollapse.Expand() (expect 'expanded')"
      }
    } catch { Mark ("ExpandCollapse drive failed: " + $_.Exception.Message) }
  }
  else { Mark "Group not found for toggle" }
}

Mark "END"
Start-Sleep -Milliseconds 600
$probe | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process nvda -ErrorAction SilentlyContinue | Stop-Process -Force

# --- Merge speech + markers ---------------------------------------------------
$rows = @()
foreach ($l in (Get-Content $speechLog -ErrorAction SilentlyContinue)) { if ($l -match '^\d+\t') { $rows += $l } }
foreach ($l in (Get-Content $markLog -ErrorAction SilentlyContinue)) { if ($l -match '^\d+\t') { $rows += $l } }
$sorted = $rows | Sort-Object { [long](($_ -split '\t')[0]) }
$out = $sorted | ForEach-Object {
  $p = $_ -split '\t', 3
  if ($p[1] -eq 'MARK') { ""; "### " + $p[2] } else { "   | " + $p[2] }
}
$out | Set-Content -Path $report -Encoding UTF8
Write-Host ""
Write-Host "==== NVDA transcript ($report) ===="
$out | ForEach-Object { Write-Host $_ }
