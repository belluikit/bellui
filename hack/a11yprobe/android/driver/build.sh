#!/bin/sh
# Builds a11ydriver.apk without gradle: aapt2 + javac + d8 + zipalign + apksigner.
# Needs ANDROID_HOME (build-tools + a platforms/android-*/android.jar).
set -eu
cd "$(dirname "$0")"

SDK="${ANDROID_HOME:-$HOME/Library/Android/sdk}"
BT="$SDK/build-tools/$(ls "$SDK/build-tools" | grep -v rc | sort -V | tail -1)"
PLATFORM="$SDK/platforms/$(ls "$SDK/platforms" | sort -V | tail -1)/android.jar"
echo "build-tools: $BT"
echo "platform:    $PLATFORM"

rm -rf build && mkdir -p build/classes build/gen
"$BT/aapt2" compile --dir res -o build/res.zip
"$BT/aapt2" link -o build/base.apk -I "$PLATFORM" --manifest AndroidManifest.xml \
    --java build/gen build/res.zip
javac -source 1.8 -target 1.8 -nowarn -classpath "$PLATFORM" -d build/classes \
    build/gen/com/trendvidia/a11ydriver/R.java A11yDriverService.java
"$BT/d8" --lib "$PLATFORM" --release --output build \
    build/classes/com/trendvidia/a11ydriver/*.class
(cd build && zip -qj base.apk classes.dex)
"$BT/zipalign" -f 4 build/base.apk build/aligned.apk

KS="$HOME/.android/debug.keystore"
if [ ! -f "$KS" ]; then
    keytool -genkeypair -keystore "$KS" -storepass android -keypass android \
        -alias androiddebugkey -dname CN=Android-Debug -keyalg RSA -validity 10000
fi
"$BT/apksigner" sign --ks "$KS" --ks-pass pass:android --out a11ydriver.apk build/aligned.apk
echo "built: $(pwd)/a11ydriver.apk"
