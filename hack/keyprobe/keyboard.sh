# keyboard.sh runs ON an Android device. It plugs in a virtual USB keyboard
# through the uinput shell command (no root needed), taps (X, Y) to focus a
# target in keyprobe, then presses and holds keys. Every step is logged to
# logcat with tag KP, so each key's DOWN and UP can be lined up with what
# keyprobe received. The keyboard goes away when the script ends.
#
#   adb push keyboard.sh /data/local/tmp/
#   adb logcat -c; adb shell sh /data/local/tmp/keyboard.sh X Y
#   adb logcat -d -s Bellui:I KP:I
X=$1; Y=$2
reg='{"id":1,"command":"register","name":"keyprobe-kb","vid":6353,"pid":4660,"bus":"usb","configuration":[{"type":100,"data":[1]},{"type":101,"data":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58]}]}'
key() { echo "{\"id\":1,\"command\":\"inject\",\"events\":[1,$1,$2,0,0,0]}"; }
mark() { log -t KP "$1"; }
{
  echo "$reg"; sleep 3
  input tap $X $Y; sleep 1.5
  mark A-DOWN; key 30 1; sleep 1
  mark A-UP; key 30 0; sleep 1
  mark D-HOLD-DOWN; key 32 1; sleep 2
  mark D-HOLD-UP; key 32 0; sleep 1
  mark BS-DOWN; key 14 1; sleep 0.1
  mark BS-UP; key 14 0; sleep 1
  mark ESC-HOLD-DOWN; key 1 1; sleep 1.5
  mark ESC-HOLD-UP; key 1 0; sleep 1
  mark END
} | uinput -
