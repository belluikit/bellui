#!/bin/sh
# Downloads the libraries android-libs.txt lists into android/libs, from
# Google's Maven repository or Maven Central.
set -eu
cd "$(dirname "$0")"
mkdir -p android/libs
grep -v '^#' android-libs.txt | while IFS=: read -r g a v ext; do
	f="android/libs/$a-$v.$ext"
	[ -s "$f" ] && continue
	path="$(echo "$g" | tr . /)/$a/$v/$a-$v.$ext"
	curl -fsS -o "$f" "https://dl.google.com/dl/android/maven2/$path" 2>/dev/null ||
		curl -fsS --retry 5 -o "$f" "https://repo1.maven.org/maven2/$path"
done
