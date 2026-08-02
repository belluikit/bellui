#!/usr/bin/env bash
# drive.sh — host-side automation for hack/androidwebviewprobe (#143).
#
# The probe app is adb-drivable end to end: it prints tapinfo lines (canvas
# points + scale + inset) that this script converts to device pixels for
# `input tap`, and the #143 Java trace (`setprop log.tag.BelluiWebView DEBUG`)
# says what the two-window focus arbitration actually did around each tap.
# dumpsys input_method is the keyboard ground truth.
#
# Commands:
#   build                     build the CLI from this checkout (instrumented
#                             dex) and package the arm64 APK
#   install                   adb install -r the APK
#   coldtap [N]               N cold-tap iterations (default 15): launch →
#                             tap the full-viewport page editor → did the
#                             keyboard come? Trace + dumpsys per iteration.
#   latency [REPS] [MODE]     keystroke latency: MODE latency (default,
#                             spinner off) or latency-spinner; REPS bursts of
#                             20 chars (default 10)
#   full                      the scripted #76/#143 flow with steps 6/7
#                             driven by injected taps — no finger needed
#
# SERIAL=<adb serial> selects the device when several are attached.
set -euo pipefail

APP=com.trendvidia.androidwebviewprobe
ACT=org.golang.app.GoNativeActivity
SDK=${ANDROID_HOME:-$HOME/Library/Android/sdk}
adb() { "$SDK/platform-tools/adb" ${SERIAL:+-s "$SERIAL"} "$@"; }
cd "$(dirname "$0")"

RUNS="${TMPDIR:-/tmp}/androidwebviewprobe-runs/$(date +%Y%m%d-%H%M%S)"

note() { printf '%s\n' "$*" >&2; }

# ---- tapinfo parsing --------------------------------------------------------
# tapinfo host=(120,80 size 320x380) entry=(8,600 size 300x40) scale=2.63 inset=(0,24)
# Sets: HX HY HW HH EX EY EW EH SCALE IX IY (all canvas points except SCALE).
parse_tapinfo() {
	# Strip the logcat preamble (it contains digits), then flatten every
	# non-numeric run to a space and read the 11 numbers positionally.
	read -r HX HY HW HH EX EY EW EH SCALE IX IY < <(
		echo "$1" | sed 's/.*host=//' | tr -cs '0-9.\n' ' '
	)
}

last_tapinfo() {
	adb logcat -d -s Bellui 2>/dev/null | grep -F 'tapinfo host=' | tail -1
}

# tap_pt X Y — canvas points to device pixels. AbsolutePositionForObject is
# relative to the interactive area, which starts below the status bar, so
# device px = (points + inset) * scale — verified against the panel window's
# dumpsys frame ([12,180] for host=(6,48) inset=(0,42) scale=2). Closed-loop
# check: the Java trace logs `touch DOWN` when the tap lands in the panel.
tap_pt() {
	local px py
	px=$(awk -v v="$1" -v i="$IX" -v s="$SCALE" 'BEGIN{printf "%d", (v+i)*s}')
	py=$(awk -v v="$2" -v i="$IY" -v s="$SCALE" 'BEGIN{printf "%d", (v+i)*s}')
	note "  tap @ (${1},${2})pt -> (${px},${py})px"
	adb shell input tap "$px" "$py"
}

wait_line() { # wait_line PATTERN TIMEOUT_S -> prints the line, rc 1 on timeout
	local pat=$1 t=$2 i line
	for ((i = 0; i < t * 2; i++)); do
		line=$(adb logcat -d -s Bellui 2>/dev/null | grep -F "$pat" | tail -1) || true
		if [ -n "$line" ]; then
			printf '%s\n' "$line"
			return 0
		fi
		sleep 0.5
	done
	return 1
}

ime_shown() {
	adb shell dumpsys input_method 2>/dev/null |
		grep -qE 'mInputShown=true|isInputViewShown=true'
}

ime_snapshot() {
	adb shell dumpsys input_method 2>/dev/null |
		grep -E 'mInputShown|mCurFocusedWindow|mImeWindowVis|mShowRequested|mCurClient|mFocusedWindow' |
		head -8
}

launch() { # launch MODE
	# A sleeping display swallows injected taps silently.
	adb shell svc power stayon usb
	adb shell input keyevent KEYCODE_WAKEUP
	adb shell wm dismiss-keyguard 2>/dev/null || true
	adb shell am force-stop $APP
	adb shell setprop debug.bellui.probe "\"$1\""
	adb shell setprop log.tag.BelluiWebView DEBUG
	adb logcat -c 2>/dev/null || true
	adb shell am start -n "$APP/$ACT" >/dev/null
}

webview_trace() {
	adb logcat -d -s BelluiWebView 2>/dev/null | tail -40
}

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
		"$cli" package --target android/arm64 --app-id $APP --name androidwebviewprobe
	ls -la androidwebviewprobe.apk
}

cmd_install() {
	adb install -r androidwebviewprobe.apk
}

cmd_coldtap() {
	local n=${1:-15} pass=0 i line
	mkdir -p "$RUNS"
	note "cold-tap loop: $n iterations -> $RUNS"
	for ((i = 1; i <= n; i++)); do
		launch coldtap
		# One wait, on the line that matters: tapinfo implies ready, and two
		# separate waits can race a slow logcat -c into a false timeout.
		line=$(wait_line 'tapinfo host=' 40) || {
			note "  $i: LAUNCH-TIMEOUT"
			echo "$i LAUNCH-TIMEOUT" >>"$RUNS/results.txt"
			continue
		}
		parse_tapinfo "$line"
		sleep 1 # geometry settled, IME idle — the cold state
		ime_snapshot >"$RUNS/$i-ime-before.txt"
		tap_pt "$(awk -v a="$HX" -v b="$HW" 'BEGIN{print a+b/2}')" \
			"$(awk -v a="$HY" -v b="$HH" 'BEGIN{print a+b/2}')"
		local up=FAIL j
		for ((j = 0; j < 10; j++)); do
			sleep 0.5
			if ime_shown; then
				up=PASS
				break
			fi
		done
		ime_snapshot >"$RUNS/$i-ime-after.txt"
		webview_trace >"$RUNS/$i-trace.txt"
		[ $up = PASS ] && pass=$((pass + 1))
		note "  $i: $up"
		echo "$i $up" >>"$RUNS/results.txt"
		if [ $up = FAIL ]; then
			note "  ---- trace ----"
			sed 's/^/  /' "$RUNS/$i-trace.txt" >&2
		fi
	done
	note "cold-tap: $pass/$n keyboards raised — artifacts in $RUNS"
}

cmd_latency() {
	local reps=${1:-10} mode=${2:-latency} i line
	mkdir -p "$RUNS"
	launch "$mode"
	line=$(wait_line 'latency ready' 40) || {
		note "latency: launch timeout"
		exit 1
	}
	line=$(wait_line 'tapinfo host=' 10)
	parse_tapinfo "$line"
	tap_pt "$(awk -v a="$HX" -v b="$HW" 'BEGIN{print a+b/2}')" \
		"$(awk -v a="$HY" -v b="$HH" 'BEGIN{print a+b/2}')"
	sleep 2
	if ! ime_shown; then
		note "latency: keyboard did not come up after the focus tap — cold-tap defect bites here too; retap"
		tap_pt "$(awk -v a="$HX" -v b="$HW" 'BEGIN{print a+b/2}')" \
			"$(awk -v a="$HY" -v b="$HH" 'BEGIN{print a+b/2}')"
		sleep 2
	fi
	# One char per injection: a 20-char burst queues keys behind each other
	# and the later ones measure queue wait, not pipeline latency (measured:
	# burst deliver p50 ~90 ms vs per-char ~3 ms). adb's own round-trip
	# spaces the keystrokes like a human would.
	note "injecting $((reps * 4)) single chars ($mode) ..."
	for ((i = 0; i < reps * 4; i++)); do
		adb shell input text a
	done
	sleep 3
	adb logcat -d -s Bellui | grep -F 'latency ' | tail -6 | tee "$RUNS/latency-$mode.txt"
	note "artifacts in $RUNS"
}

cmd_full() {
	mkdir -p "$RUNS"
	launch ""
	note "full flow started; waiting for step 6 prompt (steps 1-5b take ~40s) ..."
	wait_line '6 focus: TAP' 120 >/dev/null || {
		note "full: never reached step 6"
		exit 1
	}
	parse_tapinfo "$(last_tapinfo)"
	# The default page's input: block flow puts its center near (146,151)
	# CSS px from the page's top-left; CSS px == canvas points here (the
	# WebView viewport is density-scaled like the canvas).
	tap_pt "$(awk -v a="$HX" 'BEGIN{print a+146}')" "$(awk -v a="$HY" 'BEGIN{print a+151}')"
	wait_line '6 focus native<-tap' 30 >/dev/null || true
	wait_line '7 focus: now TAP' 30 >/dev/null || {
		note "full: never reached step 7"
		exit 1
	}
	parse_tapinfo "$(last_tapinfo)" # FRESH line — the Entry moved with the keyboard relayout
	tap_pt "$(awk -v a="$EX" -v b="$EW" 'BEGIN{print a+b/2}')" \
		"$(awk -v a="$EY" -v b="$EH" 'BEGIN{print a+b/2}')"
	wait_line 'done.' 120 >/dev/null || note "full: flow did not finish in time"
	adb logcat -d -s Bellui | grep -F '[probe]' | tee "$RUNS/full.txt"
	webview_trace >"$RUNS/full-trace.txt"
	note "artifacts in $RUNS"
}

case "${1:-}" in
build) cmd_build ;;
install) cmd_install ;;
coldtap) cmd_coldtap "${2:-}" ;;
latency) cmd_latency "${2:-}" "${3:-}" ;;
full) cmd_full ;;
*)
	note "usage: $0 {build|install|coldtap [N]|latency [REPS] [latency|latency-spinner]|full}"
	exit 2
	;;
esac
