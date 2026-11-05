#!/bin/bash
# build.sh <variant-name> <configChanges> : builds vkrot-<variant>.apk
set -e
V=$1; CC_ATTR=$2
SDK=$HOME/Library/Android/sdk; NDK=$SDK/ndk/26.3.11579264; BT=$SDK/build-tools/36.0.0
unset CGO_LDFLAGS LDFLAGS CFLAGS
TC=$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin
mkdir -p out/$V/lib/arm64-v8a
$TC/aarch64-linux-android26-clang -shared -fPIC -O2 $EXTRA -Wl,-z,max-page-size=16384 \
  -I$NDK/sources/android/native_app_glue main.c $NDK/sources/android/native_app_glue/android_native_app_glue.c \
  -u ANativeActivity_onCreate -landroid -llog -lvulkan -o out/$V/lib/arm64-v8a/libvkrot.so
cat > out/$V/AndroidManifest.xml <<M
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="io.bellui.vkrot.$V" android:versionCode="1" android:versionName="1">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="34"/>
  <application android:label="vkrot-$V" android:hasCode="false" android:debuggable="true">
    <activity android:name="android.app.NativeActivity" android:exported="true" $CC_ATTR>
      <meta-data android:name="android.app.lib_name" android:value="vkrot"/>
      <intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter>
    </activity>
  </application>
</manifest>
M
cd out/$V
$BT/aapt2 link -o base.apk --manifest AndroidManifest.xml -I $SDK/platforms/android-34/android.jar
zip -q -0 base.apk lib/arm64-v8a/libvkrot.so
$BT/zipalign -f -P 16 4 base.apk aligned.apk
$BT/apksigner sign --ks ~/.android/debug.keystore --ks-pass pass:android --out ../../vkrot-$V.apk aligned.apk
echo built vkrot-$V.apk
