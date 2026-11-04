#!/bin/sh
# long.mp3: 60 s of a 440 Hz tone, mono 8 kHz at 8 kbit/s, about 60 KB.
# Long enough that a seek back leaves FFmpeg's 32 KB read buffer and needs a
# new HTTP request (player#237), small enough to keep in the repo.
set -e
cd "$(dirname "$0")"
ffmpeg -v error -y -f lavfi -i "sine=frequency=440:duration=60" \
  -ac 1 -ar 8000 -b:a 8k -map_metadata -1 long.mp3
