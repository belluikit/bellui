#!/bin/sh
# Builds ios/Frameworks/Probe.xcframework — a dynamic framework with a device
# slice and a simulator slice — from ios/framework-src. Not committed: run
# this before packaging the probe.
set -eu
cd "$(dirname "$0")"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

build() { # sdk out targets...
	sdk=$1 out=$2
	shift 2
	fw="$work/$out/Probe.framework"
	mkdir -p "$fw/Headers"
	cp ios/framework-src/Probe.h "$fw/Headers/"
	set --
	for t in $targets; do
		xcrun --sdk "$sdk" clang -target "$t" -dynamiclib -install_name @rpath/Probe.framework/Probe \
			-o "$work/$out/Probe-$t" ios/framework-src/probe.c
		set -- "$@" "$work/$out/Probe-$t"
	done
	xcrun lipo -create -output "$fw/Probe" "$@"
	cat > "$fw/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>Probe</string>
	<key>CFBundleIdentifier</key><string>com.trendvidia.iosframeworkprobe.probe</string>
	<key>CFBundlePackageType</key><string>FMWK</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>MinimumOSVersion</key><string>15.0</string>
</dict>
</plist>
PLIST
}

targets="arm64-apple-ios15.0" build iphoneos device
targets="arm64-apple-ios15.0-simulator x86_64-apple-ios15.0-simulator" build iphonesimulator sim

rm -rf ios/Frameworks/Probe.xcframework
mkdir -p ios/Frameworks
xcodebuild -create-xcframework \
	-framework "$work/device/Probe.framework" \
	-framework "$work/sim/Probe.framework" \
	-output ios/Frameworks/Probe.xcframework
