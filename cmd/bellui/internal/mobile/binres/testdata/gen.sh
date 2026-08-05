#! /usr/bin/env bash

# Regenerates the aapt golden files (*.bin, *.arsc) from the *.xml sources.
#
# The platform jar must be the one binres.SDKPlatform pins: the goldens are
# compared against binres output, and binres resolves attributes and
# synthesizes the build-platform attributes (compileSdkVersion & co) from
# exactly that platform (#287, #297). The build-tools version only needs to be
# modern enough to record those attributes (aapt >= 28).

BUILD_TOOLS=${BUILD_TOOLS:-35.0.0}
AAPT=${ANDROID_HOME}/build-tools/${BUILD_TOOLS}/aapt

# keep in sync with binres.SDKPlatform
SDK_PLATFORM=android-35
APIJAR=${ANDROID_HOME}/platforms/${SDK_PLATFORM}/android.jar

for f in *.xml; do
	RES=""
	if [ -d "${f:0:-4}-res" ]; then
		RES="-S ${f:0:-4}-res"
	fi
	cp "$f" AndroidManifest.xml
	"$AAPT" p -M AndroidManifest.xml $RES -I "$APIJAR" -F tmp.apk
	unzip -qq -o tmp.apk AndroidManifest.xml resources.arsc
	mv AndroidManifest.xml "${f:0:-3}bin"
	mv resources.arsc "${f:0:-3}arsc"
	rm tmp.apk
done
