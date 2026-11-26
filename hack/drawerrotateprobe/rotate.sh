#!/bin/sh
# rotate.sh drives drawerrotateprobe on the one Android device adb sees:
# it launches the probe, turns the device 3 s into each phase, prints the
# probe's lines, and puts rotation back to automatic however it exits.
#
#   ./rotate.sh [serial]
set -u
ADB="adb${1:+ -s $1}"
PKG=com.trendvidia.drawerrotateprobe

restore() {
	$ADB shell cmd window user-rotation free >/dev/null
	$ADB shell settings put system accelerometer_rotation 1
	$ADB shell am force-stop $PKG
}
trap restore EXIT

waitfor() { # waitfor <text>: until the probe logs it, at most 30 s
	i=0
	until $ADB logcat -d -s 'Bellui:*' | grep -q "$1"; do
		i=$((i + 1))
		[ $i -gt 60 ] && { echo "rotate.sh: timed out waiting for: $1" >&2; exit 1; }
		sleep 0.5
	done
}

turn() { # turn 0|1|3: portrait, landscape, landscape the other way; read it back
	$ADB shell cmd window user-rotation lock "$1" >/dev/null
	sleep 1
	echo "rotate.sh: rotation now $($ADB shell dumpsys window displays | grep -o 'mCurrentRotation=[A-Z_0-9]*' | head -1)"
}

$ADB shell input keyevent KEYCODE_WAKEUP
$ADB shell am force-stop $PKG
turn 0
$ADB logcat -c
$ADB shell monkey -p $PKG -c android.intent.category.LAUNCHER 1 >/dev/null

waitfor "phase=1 shown"; sleep 3; turn 1 # opened in portrait, to landscape
waitfor "phase=2 shown"; sleep 3; turn 0 # opened in landscape, to portrait
waitfor "phase=3 shown"; sleep 3; turn 1
waitfor "phase=4 shown"; sleep 3; turn 0
waitfor "phase=4 done"; turn 1        # phase 5 opens in landscape
waitfor "phase=5 shown"; sleep 3; turn 3 # then the other way round
waitfor "drawerrotateprobe \(PASS\|FAIL\)"

$ADB logcat -d -s 'Bellui:*' | grep drawerrotateprobe
