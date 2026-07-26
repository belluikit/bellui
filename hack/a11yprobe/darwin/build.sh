#!/bin/sh
# Builds the raw-AX client drivers next to their sources.
set -e
cd "$(dirname "$0")"
for tool in axset axtree axsel axcell; do
    clang -framework Cocoa -framework ApplicationServices -o "$tool" "$tool.m"
    echo "built $tool"
done
