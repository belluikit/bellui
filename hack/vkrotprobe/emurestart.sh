#!/bin/bash
# emurestart.sh: kill pixel143 (if up), wait for the process to exit, boot it fresh.
S=$(dirname "$0"); A="$HOME/Library/Android/sdk/platform-tools/adb -s emulator-5554"
$A emu kill >/dev/null 2>&1
for i in $(seq 60); do pgrep -f "^$HOME/Library/Android/sdk/emulator/qemu/darwin-aarch64/qemu-system-aarch64-headless -avd pixel143 " >/dev/null || break; sleep 2; done
nohup $HOME/Library/Android/sdk/emulator/emulator -avd pixel143 -no-window -gpu swiftshader_indirect -no-snapshot-save -port 5554 > $S/emu-last.log 2>&1 &
for i in $(seq 90); do [ "$($A shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = 1 ] && { sleep 20; echo booted; exit 0; }; sleep 3; done
echo "BOOT FAILED"; tail -5 $S/emu-last.log; exit 1
