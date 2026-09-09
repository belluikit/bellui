#!/usr/bin/env bash
# drive.sh — host-side automation for hack/safprobe (#303).
#
# The probe scripts itself: with no remembered folder it opens the SAF folder
# picker on launch and runs the whole scaffold sequence on the pick, printing
# one "safprobe: PASS|FAIL <claim>" line per step. This script only has to
# drive DocumentsUI, which is a real Android view hierarchy and so is
# uiautomator-drivable, and then read the lines back.
#
# Commands:
#   build           build the CLI from this checkout (embeds the local dex)
#                   and package the arm64 APK
#   install         adb install -r, and forget any previous folder grant
#   pick            fresh run: clear app data, launch, drive the picker to
#                   Documents, and report every PASS/FAIL line
#   rerun           relaunch WITHOUT clearing data — the remembered folder URI
#                   is only usable if the grant persisted (#303 item 4)
#   grants          print the persisted URI permissions the app holds
#   full            pick, then rerun, then grants
#
# SERIAL=<adb serial> selects the device when several are attached.
set -euo pipefail

APP=io.bellui.safprobe
ACT=org.golang.app.GoNativeActivity
SDK=${ANDROID_HOME:-$HOME/Library/Android/sdk}
adb() { "$SDK/platform-tools/adb" ${SERIAL:+-s "$SERIAL"} "$@"; }
cd "$(dirname "$0")"

note() { printf '%s\n' "$*" >&2; }

# ---- uiautomator helpers ----------------------------------------------------
# dismiss_anr taps "Wait" on an Application Not Responding dialog. A 50 MB Go
# binary on a loaded host ANRs SystemUI and DocumentsUI routinely, and the
# dialog steals focus from whatever the flow was driving; left alone it reads
# as "the button never appeared".
anr_count=0
dismiss_anr() {
	local xml b
	xml=$(dump_ui || true)
	printf '%s' "$xml" | grep -qi "responding" || return 0
	b=$(printf '%s' "$xml" | tr '>' '\n' |
		grep -iE 'text="Wait"' |
		grep -oE 'bounds="\[[0-9]+,[0-9]+\]\[[0-9]+,[0-9]+\]"' | head -1)
	[ -n "$b" ] || return 0
	read -r x1 y1 x2 y2 < <(printf '%s' "$b" | tr -cs '0-9' ' ')
	adb shell input tap $(((x1 + x2) / 2)) $(((y1 + y2) / 2))
	anr_count=$((anr_count + 1))
	note "  dismissed an ANR dialog ($anr_count)"
	# A device ANRing this persistently is starved, not slow. Carrying on
	# produces a wall of "never appeared" that looks like a product failure
	# and is a host-load one.
	if [ "$anr_count" -ge 12 ]; then
		note "ABORT: $anr_count ANRs — the device is starved of CPU, not failing."
		note "       Free the host (check its load average) and rerun."
		exit 3
	fi
	sleep 2
}

# DocumentsUI is an ordinary view hierarchy, so the picker can be driven by
# text rather than by coordinates. tap_text dumps the tree, finds the node
# whose text or content-desc matches, and taps the centre of its bounds.
# The dump goes to /data/local/tmp, not /sdcard: scoped storage makes the
# latter unreadable to the shell user, so `cat` answers "Permission denied"
# and every tap_text reads as "the button never appeared".
dump_ui() {
	adb shell uiautomator dump /data/local/tmp/ui.xml >/dev/null 2>&1 || return 1
	adb shell cat /data/local/tmp/ui.xml
}

tap_text() {
	local want="$1" tries="${2:-20}" xml bounds i
	for ((i = 0; i < tries; i++)); do
		dismiss_anr
		xml=$(dump_ui || true)
		bounds=$(printf '%s' "$xml" | tr '>' '\n' |
			grep -iE "(text|content-desc)=\"[^\"]*${want}[^\"]*\"" |
			grep -oE 'bounds="\[[0-9]+,[0-9]+\]\[[0-9]+,[0-9]+\]"' | head -1)
		if [ -n "$bounds" ]; then
			read -r x1 y1 x2 y2 < <(printf '%s' "$bounds" | tr -cs '0-9' ' ')
			adb shell input tap $(((x1 + x2) / 2)) $(((y1 + y2) / 2))
			note "  tapped '$want'"
			return 0
		fi
		sleep 1
	done
	note "  '$want' never appeared"
	return 1
}

# wait_for_start blocks until THIS run's probe process has reached its launch
# marker. Without it wait_for_picker can match a DocumentsUI window an earlier
# attempt left focused and start tapping before the probe has opened anything.
wait_for_start() {
	local i
	for ((i = 0; i < 60; i++)); do
		if adb logcat -d 2>/dev/null | grep -q 'safprobe: START'; then
			note "  probe started after ~$((i * 2))s"
			return 0
		fi
		dismiss_anr
		sleep 2
	done
	note "TIMEOUT: the probe never printed its START marker"
	return 1
}

# wait_for_picker blocks until DocumentsUI is the focused activity. The probe
# opens the picker from OnEnteredForeground, and on a slow device that is tens
# of seconds after am start — driving the UI before then taps the probe's own
# canvas and the run silently does nothing.
wait_for_picker() {
	local i focus
	for ((i = 0; i < 60; i++)); do
		dismiss_anr
		focus=$(adb shell dumpsys window 2>/dev/null | grep -i mCurrentFocus || true)
		case "$focus" in
		*documentsui*)
			note "  picker up after ~${i}s"
			return 0
			;;
		esac
		sleep 2
	done
	note "TIMEOUT: the SAF picker never came up"
	return 1
}

# ---- probe output -----------------------------------------------------------
# clear_log drops the log buffer. It fails on a busy device ("failed to clear
# the 'main' log"), which must not end the run: report reads only lines after
# the last DONE, so a stale buffer costs nothing.
clear_log() {
	adb logcat -c 2>/dev/null || note "  (could not clear the log buffer; reading the tail instead)"
}

# wait_for_done blocks until the probe prints its terminating DONE line, so the
# report is never read half-written. A timeout is a failure, not a quiet pass.
wait_for_done() {
	local i
	for ((i = 0; i < 90; i++)); do
		if probe_lines | grep -q '^DONE'; then
			return 0
		fi
		dismiss_anr
		sleep 2
	done
	note "TIMEOUT: the probe never reached its DONE line"
	return 1
}

# probe_lines is every probe line since the most recent START — the marker the
# probe prints on launch — so a log buffer that could not be cleared cannot
# make an earlier run's PASSes, or its DONE, count for this one.
probe_lines() {
	# No -s tag filter: gomobile routes the probe's stdout through the toolkit's
	# log bridge under the "Bellui" tag, not "GoLog", and that is not worth
	# depending on. The "safprobe: " prefix identifies our lines whatever the tag.
	adb logcat -d 2>/dev/null | sed -n 's/.*safprobe: //p' |
		awk '/^START$/ {n = 0; next} {a[n++] = $0} END {for (i = 0; i < n; i++) print a[i]}'
}

report() {
	local lines pass fail
	lines=$(probe_lines)
	printf '%s\n' "$lines"
	pass=$(printf '%s\n' "$lines" | grep -c '^PASS ' || true)
	fail=$(printf '%s\n' "$lines" | grep -c '^FAIL ' || true)
	note ""
	note "PASS=$pass FAIL=$fail"
	# A run with no PASS lines at all did not run; do not read it as clean.
	[ "$pass" -gt 0 ] || { note "no PASS lines — the probe did not run"; return 1; }
	[ "$fail" -eq 0 ]
}

# ---- commands ---------------------------------------------------------------
cmd_build() {
	local cli
	cli=$(mktemp -d)/bellui-cli
	note "building CLI (embeds the local dex) ..."
	(cd ../../cmd/bellui && go build -o "$cli" .)
	note "packaging arm64 APK ..."
	# CGO_LDFLAGS cleared: a host-mac's -Wl,-no_warn_duplicate_libraries is
	# fatal to the NDK's lld.
	CGO_LDFLAGS= ANDROID_HOME="$SDK" \
		ANDROID_NDK_HOME="$SDK/ndk/$(ls "$SDK/ndk" | sort | tail -1)" \
		"$cli" package --target android/arm64 --app-id $APP --name safprobe
	ls -la safprobe.apk
}

cmd_install() {
	adb install -r safprobe.apk
	adb shell pm clear $APP >/dev/null
}

cmd_pick() {
	note "fresh run: clearing app data so the picker opens ..."
	adb shell pm clear $APP >/dev/null
	clear_log
	adb shell am start -n $APP/$ACT >/dev/null
	wait_for_start
	wait_for_picker

	# ACTION_OPEN_DOCUMENT_TREE lands on the device's storage root, which
	# Android refuses to grant — it shows "Can't use this folder / To protect
	# your privacy, choose another folder" and leaves USE THIS FOLDER inert. So
	# enter a subfolder first; Documents is present on every image.
	#
	# Do NOT go via "Show roots" or "More options": the latter opens the
	# overflow menu, which then covers the file list, and every subsequent
	# tap_text reports "never appeared" while the picker is simply obscured.
	tap_text "Documents" 20
	tap_text "USE THIS FOLDER" 15
	tap_text "ALLOW" 15

	wait_for_done
	report
}

cmd_rerun() {
	note "rerun: app data KEPT, so the folder URI is remembered ..."
	adb shell am force-stop $APP
	clear_log
	adb shell am start -n $APP/$ACT >/dev/null
	wait_for_done
	report
}

cmd_grants() {
	adb shell dumpsys activity providers 2>/dev/null |
		grep -i -A 3 "$APP" | grep -i "persist" || true
	note "--- content provider grants ---"
	adb shell content query --uri content://com.android.externalstorage.documents/ 2>/dev/null || true
}

case "${1:-}" in
build) cmd_build ;;
install) cmd_install ;;
pick) cmd_pick ;;
rerun) cmd_rerun ;;
grants) cmd_grants ;;
full)
	cmd_pick
	cmd_rerun
	;;
*)
	note "usage: $0 {build|install|pick|rerun|grants|full}"
	exit 2
	;;
esac
