#!/bin/bash
# Runs hack/pushprobe through every #1517 case on an iOS simulator and prints
# PASS or FAIL for each. Taps.xcodeproj is an XCUITest bundle that taps
# SpringBoard: the permission alert, banners and a banner's action button.
#
#   hack/pushprobe/simtest/run.sh [simulator-udid] [bellui-command]
#
# The simulator must be booted. The probe is packaged with the given bellui
# command (default: bellui on PATH), which must carry #1517, or the build has
# no aps-environment and registration fails.
set -u
here=$(cd "$(dirname "$0")" && pwd)
probe=$(dirname "$here")
dev=${1:-booted}
bellui=${2:-bellui}
app=com.trendvidia.pushprobe
work=$(mktemp -d)
fails=0

check() { # name, pattern in the probe's log
	if grep -q -- "$2" "$log"; then echo "PASS $1"; else echo "FAIL $1 (no \"$2\")"; fails=$((fails + 1)); fi
}
absent() { # name, pattern that must not be in the log
	if grep -q -- "$2" "$log"; then echo "FAIL $1 (found \"$2\")"; fails=$((fails + 1)); else echo "PASS $1"; fi
}
tap() { # test method, banner text
	TEST_RUNNER_TAP_TEXT="${2:-}" TEST_RUNNER_TAP_WAIT="${3:-30}" xcodebuild test-without-building \
		-project "$here/Taps.xcodeproj" -scheme UITests -destination "id=$udid" \
		-derivedDataPath "$work/dd" SYMROOT="$work/build" -only-testing:"UITests/Taps/$1" >"$work/xcode-$1.log" 2>&1
}
push() { # title, show
	printf '{"aps":{"alert":{"title":"%s","body":"%s"},"sound":"default"},"show":%s,"data":{"case":"%s"}}' \
		"$1" "$1" "$2" "$1" >"$work/p.apns"
	xcrun simctl push "$udid" $app "$work/p.apns" >/dev/null
}
fresh() { # extra launch args
	xcrun simctl terminate "$udid" $app 2>/dev/null
	xcrun simctl uninstall "$udid" $app
	xcrun simctl install "$udid" "$work/pushprobe.app"
	xcrun simctl launch "$udid" $app "$@" >/dev/null
	sleep 3
	log="$(xcrun simctl get_app_container "$udid" $app data)/tmp/pushprobe.log"
}

udid=$dev
[ "$dev" = booted ] && udid=$(xcrun simctl list devices booted | grep -o -E '[0-9A-F-]{36}' | head -1)
(cd "$probe" && "$bellui" package -os iossimulator --app-id $app >"$work/package.log" 2>&1 && mv pushprobe.app "$work/") ||
	{ echo "package failed: $work/package.log"; exit 1; }
git -C "$probe" checkout -q -- manifest.pxf 2>/dev/null # package bumps the build number
xcodebuild build-for-testing -project "$here/Taps.xcodeproj" -scheme UITests -destination "id=$udid" \
	-derivedDataPath "$work/dd" SYMROOT="$work/build" >"$work/build.log" 2>&1 || { echo "driver build failed: $work/build.log"; exit 1; }

fresh -asks 2
tap testDontAllow; sleep 3
check "device token issued" "token [0-9a-f]\{64,\}"
check "denied, both concurrent requests" "permission granted=false"
absent "denied reports no grant" "permission granted=true"

fresh -asks 2
tap testAllow; sleep 3
check "allowed, both concurrent requests" "permission granted=true"
absent "allowed reports no denial" "permission granted=false"

(sleep 6; push FgShown true) & tap testTapBanner FgShown; wait
check "foreground push reaches the handler" 'received show=true title="FgShown"'
check "a shown foreground push can be tapped" 'opened title="FgShown"'
(sleep 6; push FgHidden false) & tap testTapBanner FgHidden 15; wait
check "a hidden foreground push reaches the handler" 'received show=false title="FgHidden"'
absent "a hidden foreground push shows no banner" 'opened title="FgHidden"'

xcrun simctl launch "$udid" com.apple.Preferences >/dev/null; sleep 2
(sleep 6; push WarmTap true) & tap testTapBanner WarmTap; wait
check "tap while running in the background" 'opened title="WarmTap"'
absent "a background push is not a foreground one" 'received show=true title="WarmTap"'

xcrun simctl terminate "$udid" $app; sleep 2
(sleep 6; push ColdTap true) & tap testTapBanner ColdTap; wait; sleep 4
check "tap that launches the app" 'opened title="ColdTap"'
if [ "$(grep -c 'opened title="ColdTap"' "$log")" = 1 ]; then echo "PASS launch tap delivered once"; else echo "FAIL launch tap not delivered exactly once"; fails=$((fails + 1)); fi
if awk '/opened callback set/{set=1} /opened title="ColdTap"/{exit !set}' "$log"; then echo "PASS launch tap held for the late callback"; else echo "FAIL launch tap before the callback"; fails=$((fails + 1)); fi

xcrun simctl launch --terminate-running-process "$udid" $app -local >/dev/null; sleep 1
tap testTapBanner LocalRich; sleep 2
check "local notification body tap" "local tapped"
absent "a local tap is not a push" 'opened title="LocalRich"'
xcrun simctl launch --terminate-running-process "$udid" $app -local >/dev/null; sleep 1
TEST_RUNNER_TAP_ACTION=Reply tap testTapAction LocalRich; sleep 2
check "local notification action" "local action Reply"

echo "log: $log  work: $work"
[ $fails = 0 ] && echo "ALL PASS" || { echo "$fails FAILED"; exit 1; }
