#!/bin/bash
# rot.sh <apk> <pkg> <label> [n=30]: launch, rotate n times 3 s apart; stop at the first ANR.
# Positive markers: display mRotation read back each step; screen differs over 1 s at the end.
APK=$1; PKG=$2; L=$3; N=${4:-30}; ACT=${ACT:-org.golang.app.GoNativeActivity}
OUT=$(dirname "$0")/runs; A="$HOME/Library/Android/sdk/platform-tools/adb -s emulator-5554"
[ -n "$APK" ] && { $A install -r -d -t "$APK" >/dev/null || { echo "[$L] install failed"; exit 1; }; }
$A shell settings put system accelerometer_rotation 0; $A shell settings put system user_rotation 0
$A shell am force-stop $PKG; sleep 1; $A logcat -c
$A shell am start -n $PKG/$ACT >/dev/null; sleep 8
echo "[$L] pid=$($A shell pidof $PKG | tr -d '\r')"
guard(){ f=$($A shell dumpsys window | grep mCurrentFocus | head -1); case "$f" in *$PKG*) ;; *) echo "[$L] CONTAMINATED at $1: focus $f"; exit 2;; esac; }
guard launch
ok=0; for i in $(seq 1 $N); do
  r=$((i % 4)); $A shell cmd window user-rotation lock $r; sleep 3; guard "rotation $i"
  rot=$($A shell dumpsys window displays | awk "/^ *mRotation=[0-9]/{print \$1}" | head -1 | tr -d '\r')
  if $A logcat -d | grep -q "ANR in $PKG\|VkEncoder"; then echo "[$L] rotation $i (to $r): HANG ($rot)"; sample "$(pgrep -f "^$HOME/Library/Android/sdk/emulator/qemu/darwin-aarch64/qemu-system-aarch64-headless -avd pixel143 ")" 3 -file "$OUT/sample_$L.txt" >/dev/null 2>&1; break; fi
  [ "$rot" = "mRotation=$r" ] && ok=$((ok+1)) || echo "[$L] rotation $i: display did not turn ($rot)"
done
sleep 2
h1=$($A exec-out screencap | md5); sleep 1; h2=$($A exec-out screencap | md5)
[ "$h1" != "$h2" ] && live=painting || live=FROZEN
$A logcat -d > "$OUT/logcat_$L.txt"
echo "[$L] RESULT turned=$ok/$N live=$live ANR=$(grep -c "ANR in $PKG" $OUT/logcat_$L.txt) VkEncoder=$(grep -c VkEncoder $OUT/logcat_$L.txt) lastTick=$(grep -o 'tick=[0-9]*' $OUT/logcat_$L.txt | tail -1)"
$A shell am force-stop $PKG
$A shell cmd window user-rotation lock 0
