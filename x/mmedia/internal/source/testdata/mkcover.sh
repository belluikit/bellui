#!/bin/sh
# Makes cover.mp3: 3 s of a 440 Hz tone with a 64x64 JPEG embedded as cover
# art (an attached picture), the shape of a song from a music library (#850).
set -eu
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
ffmpeg -v error -f lavfi -i "sine=frequency=440:duration=3" -ac 1 -ar 44100 -b:a 64k "$tmp/tone.mp3"
ffmpeg -v error -f lavfi -i "color=c=orange:s=64x64:d=1" -frames:v 1 "$tmp/cover.jpg"
ffmpeg -v error -y -i "$tmp/tone.mp3" -i "$tmp/cover.jpg" -map 0:a -map 1:v -c copy \
	-id3v2_version 3 -metadata:s:v title="Album cover" -disposition:v attached_pic cover.mp3
