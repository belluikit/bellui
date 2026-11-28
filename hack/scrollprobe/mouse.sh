# mouse.sh runs ON an Android device: plugs in a virtual USB mouse through the
# uinput shell command (no root), moves it, turns the wheel, unplugs.
# Steps are logged to logcat with tag MS.
reg='{"id":1,"command":"register","name":"scrollprobe-mouse","vid":1133,"pid":49271,"bus":"usb","configuration":[{"type":100,"data":[1,2]},{"type":101,"data":[272,273,274]},{"type":102,"data":[0,1,6,8]}]}'
ev() { echo "{\"id\":1,\"command\":\"inject\",\"events\":[$1,0,0,0]}"; }
mark() { log -t MS "$1"; }
{
  echo "$reg"; sleep 3
  mark MOVE-UP; for i in 1 2 3 4 5 6 7 8 9 10; do ev "2,1,-30"; sleep 0.05; done; sleep 1
  mark WHEEL-PAD; for i in 1 2 3; do ev "2,8,-1"; sleep 0.3; done; sleep 1
  mark HWHEEL-PAD; ev "2,6,1"; sleep 1
  mark MOVE-DOWN; for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do ev "2,1,30"; sleep 0.05; done; sleep 1
  mark WHEEL-LIST; for i in 1 2 3 4 5; do ev "2,8,-1"; sleep 0.3; done; sleep 1
  mark CLICK-LIST; ev "1,272,1"; sleep 0.1; ev "1,272,0"; sleep 1
  mark END
} | uinput -
