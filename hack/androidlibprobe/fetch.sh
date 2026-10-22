#!/bin/sh
# fetch.sh downloads the AndroidX artifacts androidlibprobe links against
# into android/libs/ (gitignored). bellui does no Maven resolution, so the
# list is the whole dependency closure of androidx.core:core:1.3.2, read from
# its POM and its dependencies' POMs: three AARs and four jars. Versions are
# pinned; re-running replaces the files.
set -eu
cd "$(dirname "$0")"
mkdir -p android/libs
M=https://dl.google.com/android/maven2
for p in \
	androidx/core/core/1.3.2/core-1.3.2.aar \
	androidx/lifecycle/lifecycle-runtime/2.0.0/lifecycle-runtime-2.0.0.aar \
	androidx/versionedparcelable/versionedparcelable/1.1.0/versionedparcelable-1.1.0.aar \
	androidx/annotation/annotation/1.1.0/annotation-1.1.0.jar \
	androidx/lifecycle/lifecycle-common/2.0.0/lifecycle-common-2.0.0.jar \
	androidx/arch/core/core-common/2.0.0/core-common-2.0.0.jar \
	androidx/collection/collection/1.0.0/collection-1.0.0.jar; do
	echo "fetch $M/$p"
	curl -fsSL -o "android/libs/$(basename "$p")" "$M/$p"
done
