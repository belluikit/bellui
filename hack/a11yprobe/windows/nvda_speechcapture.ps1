<#
.SYNOPSIS
  Capture what NVDA actually speaks against the a11yprobe app (bellui #566).

.DESCRIPTION
  The UIA PowerShell clients (uiatree/uiaact/uiatext) verify the *provider*
  tree the way a screen reader queries it. This script goes one layer further:
  it runs the real NVDA screen reader with the sibling speechcapture global
  plugin loaded, drives a scripted walk over Probe A + Probe B, and merges
  NVDA's spoken utterances with per-step markers into a diffable transcript.
  It turns the human "does it sound right" listen into a reproducible artifact.

  What this captures RELIABLY (fires on UIA events, independent of key
  injection): focus tracking, role/label/value/state on focus, password
  masking, per-window identity, and live-region announcements (Toast/Snackbar).

  What it does NOT reliably drive: widget behaviour reached by arrow keys
  (slider increment, tree-row navigation, caret motion). NVDA installs a
  low-level keyboard hook that filters *synthetically injected* keystrokes, so
  those steps behave differently than physical keys and MUST be confirmed in a
  physical interactive session with a human listen (that is the residual of
  #566). Text typing via injected character keys does reach the widget.

  Must run in a PHYSICAL interactive desktop (RDP/console, not SSH): over SSH
  the app's windows land in a session-0 window station, invisible to UIA and
  never foreground, so focus tracking never engages. See README.md.

.PARAMETER NvdaExe
  Path to nvda.exe (a portable copy is fine; create one with
  `nvda_<ver>.exe --create-portable-silent --portable-path <dir> --minimal`).

.PARAMETER ProbeExe
  Path to the built probe:
  `go build -tags accessibility -o a11yprobe.exe ./hack/a11yprobe`

.PARAMETER WorkDir
  Scratch dir for the NVDA config, plugin, logs and the merged transcript.

.EXAMPLE
  .\nvda_speechcapture.ps1 -NvdaExe C:\nvda_portable\nvda.exe -ProbeExe .\a11yprobe.exe
#>
param(
  [Parameter(Mandatory = $true)][string]$NvdaExe,
  [Parameter(Mandatory = $true)][string]$ProbeExe,
  [string]$WorkDir = (Join-Path $env:TEMP "a11yprobe_nvda")
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

# --- Layout: config dir with scratchpad-loaded plugin ------------------------
$cfg      = Join-Path $WorkDir "nvdacfg"
$plugDst  = Join-Path $cfg "scratchpad\globalPlugins\speechcapture"
$plugSrc  = Join-Path $PSScriptRoot "speechcapture"
$speechLog= Join-Path $WorkDir "nvda_speech.log"
$markLog  = Join-Path $WorkDir "marks.log"
$report   = Join-Path $WorkDir "transcript.txt"

New-Item -ItemType Directory -Force -Path $plugDst | Out-Null
Copy-Item (Join-Path $plugSrc "__init__.py") (Join-Path $plugDst "__init__.py") -Force
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
Remove-Item $speechLog,$markLog -ErrorAction SilentlyContinue

# --- Key + marker helpers ----------------------------------------------------
$KEYUP = 0x0002
$VK = @{ TAB=0x09; HOME=0x24; END=0x23; X=0x58 }
function Key([int]$vk) {
  [Native]::keybd_event([byte]$vk,0,0,[IntPtr]::Zero); Start-Sleep -Milliseconds 45
  [Native]::keybd_event([byte]$vk,0,$KEYUP,[IntPtr]::Zero)
}
function Mark([string]$d) {
  $ms = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
  Add-Content -Path $markLog -Value ("{0}`tMARK`t{1}" -f $ms,$d)
  Write-Host ("MARK: " + $d)
  Start-Sleep -Milliseconds 1500
}

# --- Launch NVDA with the capture plugin -------------------------------------
Write-Host "Starting NVDA ($NvdaExe) ..."
$env:NVDA_SPEECH_LOG = $speechLog
Start-Process -FilePath $NvdaExe -ArgumentList '-c',$cfg,'--minimal' | Out-Null
Start-Sleep -Seconds 8

# --- Launch probe, locate windows via UIA ------------------------------------
$AE = [System.Windows.Automation.AutomationElement]
$TS = [System.Windows.Automation.TreeScope]
$CT = [System.Windows.Automation.ControlType]
Write-Host "Launching probe ..."
$probe = Start-Process -FilePath $ProbeExe -PassThru -RedirectStandardOutput (Join-Path $WorkDir "probe_stdout.log")
Start-Sleep -Seconds 6
$root = $AE::RootElement
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $probe.Id)
$wins = $root.FindAll($TS::Children, $cond)
$wA = $null; $wB = $null
foreach ($w in $wins) {
  if ($w.Current.Name -like "*Probe A*") { $wA = $w }
  if ($w.Current.Name -like "*Probe B*") { $wB = $w }
}
if (-not $wA) { Write-Host "ERROR: Probe A window not found (SSH session? see README)"; $probe | Stop-Process -Force; exit 1 }
function Focus-Win($w) { [Native]::SetForegroundWindow([IntPtr]$w.Current.NativeWindowHandle) | Out-Null; Start-Sleep -Milliseconds 700 }
Focus-Win $wA
$allA = $wA.FindAll($TS::Descendants, [System.Windows.Automation.Condition]::TrueCondition)

# --- Walk: focus order (roles/labels/values + focus tracking) ----------------
Mark "Probe A foreground (initial focus)"
for ($i = 1; $i -le 3; $i++) { Key $VK.TAB; Mark "Tab #$i (announce focused role+label+value)" }
# real bellui focus reached the entry by now; type a char (injected char keys DO reach it)
Key $VK.END; Key $VK.X; Mark "type 'x' in entry (announce edit / content change)"

# --- Password masking (UIA SetFocus -> focus announcement) -------------------
$pw = $null; foreach ($e in $allA) { if ($e.Current.IsPassword) { $pw = $e; break } }
if ($pw) { Mark "focus PASSWORD (expect 'protected' + bullet count, NEVER the content)"; try { $pw.SetFocus() } catch {} ; Start-Sleep -Milliseconds 400 }
else { Mark "PASSWORD control not found" }

# --- Live-region announcements (Invoke -> unprompted speech) ------------------
foreach ($n in @("Show Toast","Show Snackbar")) {
  $b = $null; foreach ($e in $allA) { if ($e.Current.Name -like "*$n*") { $b = $e; break } }
  if ($b) {
    Mark "focus '$n' button"
    try { $b.SetFocus() } catch {}; Start-Sleep -Milliseconds 300
    try { ($b.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)).Invoke() } catch {}
    Mark "invoke '$n' (expect UNPROMPTED announcement, cursor unmoved)"
  } else { Mark "'$n' button not found" }
}

# --- Per-window identity: switch to Probe B ----------------------------------
if ($wB) {
  Focus-Win $wB
  Mark "switch to Probe B (expect focus follows the key window)"
  Key $VK.TAB; Mark "Tab in Probe B (disjoint per-window tree)"
}

Mark "END"
Start-Sleep -Milliseconds 500
$probe | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process nvda -ErrorAction SilentlyContinue | Stop-Process -Force

# --- Merge speech + markers into a diffable transcript -----------------------
$rows = @()
foreach ($l in (Get-Content $speechLog -ErrorAction SilentlyContinue)) { if ($l -match '^\d+\t') { $rows += $l } }
foreach ($l in (Get-Content $markLog   -ErrorAction SilentlyContinue)) { if ($l -match '^\d+\t') { $rows += $l } }
$sorted = $rows | Sort-Object { [long](($_ -split '\t')[0]) }
$marks = $sorted | Where-Object { $_ -match '\tMARK\t' }
if ($marks.Count -ge 1) {
  $t0 = [long](((($marks | Select-Object -First 1)) -split '\t')[0])
  $t1 = [long](((($marks | Select-Object -Last  1)) -split '\t')[0])
} else { $t0 = 0; $t1 = [long]::MaxValue }
$out = $sorted | Where-Object { $t = [long](($_ -split '\t')[0]); $t -ge $t0 -and $t -le $t1 } | ForEach-Object {
  $p = $_ -split '\t',3
  if ($p[1] -eq 'MARK') { ""; "### " + $p[2] } else { "   | " + $p[2] }
}
$out | Set-Content -Path $report -Encoding UTF8
Write-Host ""
Write-Host "==== NVDA transcript ($report) ===="
$out | ForEach-Object { Write-Host $_ }
