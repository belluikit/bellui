#!/bin/sh
# gen.sh [notices.txt] writes the probe's assets (gitignored) and prints each
# one's size and CRC-32, the values the probe must read back on the device.
#
# assets/THIRD_PARTY_NOTICES.txt is the file given, such as the notices a real
# app ships (unzip -p Daysong.apk assets/THIRD_PARTY_NOTICES.txt > n.txt).
# Without one it is built from the distinct license files in the Go module
# cache, each under a header naming it: real license text, not a repeated
# string that would compress better than the real thing.
# assets/media/tone.ogg is 64 KB of random bytes: only its extension matters
# to how it is stored.
set -eu
cd "$(dirname "$0")"
mkdir -p assets/media
out=assets/THIRD_PARTY_NOTICES.txt
if [ $# -gt 0 ]; then
	cp "$1" "$out"
else
	: > "$out"
	seen=$(mktemp)
	mod=$(go env GOMODCACHE)
	find "$mod" -type f \( -name LICENSE -o -name LICENSE.txt -o -name LICENSE.md -o -name COPYING \) 2>/dev/null | sort | while read -r f; do
		sum=$(shasum "$f" | cut -d' ' -f1)
		if grep -q "$sum" "$seen"; then continue; fi
		echo "$sum" >> "$seen"
		printf '\n==== %s ====\n\n' "${f#"$mod"/}" >> "$out"
		cat "$f" >> "$out"
		if [ "$(wc -c < "$out")" -gt 4194304 ]; then break; fi
	done
	rm -f "$seen"
fi
head -c 65536 /dev/urandom > assets/media/tone.ogg
python3 -c '
import gzip, sys, zlib
for p in sys.argv[1:]:
    b = open(p, "rb").read()
    print("%s %d/%08x gzip-9=%d" % (p, len(b), zlib.crc32(b), len(gzip.compress(b, 9))))
' "$out" assets/media/tone.ogg
