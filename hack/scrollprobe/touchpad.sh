# touchpad.sh runs ON an Android device: plugs in a virtual multitouch
# touchpad through the uinput shell command, makes one two-finger swipe down,
# one two-finger swipe up, then a one-finger move. Steps logged with tag TP.
ai() { echo "{\"code\":$1,\"info\":{\"value\":0,\"minimum\":0,\"maximum\":$2,\"fuzz\":0,\"flat\":0,\"resolution\":$3}}"; }
reg="{\"id\":1,\"command\":\"register\",\"name\":\"scrollprobe-touchpad\",\"vid\":1452,\"pid\":613,\"bus\":\"usb\",\"configuration\":[{\"type\":100,\"data\":[1,3]},{\"type\":101,\"data\":[272,325,330,333,334]},{\"type\":103,\"data\":[0,1,47,53,54,57]},{\"type\":110,\"data\":[0,2]}],\"abs_info\":[$(ai 0 1000 10),$(ai 1 700 10),$(ai 47 4 0),$(ai 53 1000 10),$(ai 54 700 10),$(ai 57 65535 0)]}"
ev() { echo "{\"id\":1,\"command\":\"inject\",\"events\":[$1,0,0,0]}"; }
mark() { log -t TP "$1"; }
two() { # y0 step count
  y=$1
  ev "3,47,0,3,57,11,3,53,400,3,54,$y,3,47,1,3,57,12,3,53,600,3,54,$y,1,330,1,1,333,1,3,0,400,3,1,$y"; sleep 0.02
  i=0; while [ $i -lt $3 ]; do y=$((y+$2)); ev "3,47,0,3,54,$y,3,47,1,3,54,$y,3,1,$y"; sleep 0.016; i=$((i+1)); done
  sleep 0.2
  ev "3,47,0,3,57,-1,3,47,1,3,57,-1,1,330,0,1,333,0"
}
{
  echo "$reg"; sleep 3
  mark SWIPE-DOWN; two 200 8 30; sleep 1.5
  mark SWIPE-UP; two 500 -8 30; sleep 1.5
  mark SWIPE-RIGHT; x=300; ev "3,47,0,3,57,31,3,53,$x,3,54,350,3,47,1,3,57,32,3,53,$((x+200)),3,54,350,1,330,1,1,333,1,3,0,$x,3,1,350"; sleep 0.02
  i=0; while [ $i -lt 20 ]; do x=$((x+8)); ev "3,47,0,3,53,$x,3,47,1,3,53,$((x+200)),3,0,$x"; sleep 0.016; i=$((i+1)); done; sleep 0.2
  ev "3,47,0,3,57,-1,3,47,1,3,57,-1,1,330,0,1,333,0"; sleep 1.5
  mark PINCH-OUT; a=450; b=550; ev "3,47,0,3,57,41,3,53,$a,3,54,350,3,47,1,3,57,42,3,53,$b,3,54,350,1,330,1,1,333,1,3,0,$a,3,1,350"; sleep 0.02
  i=0; while [ $i -lt 20 ]; do a=$((a-10)); b=$((b+10)); ev "3,47,0,3,53,$a,3,47,1,3,53,$b,3,0,$a"; sleep 0.016; i=$((i+1)); done; sleep 0.2
  ev "3,47,0,3,57,-1,3,47,1,3,57,-1,1,330,0,1,333,0"; sleep 1.5
  mark ONE-FINGER; ev "3,47,0,3,57,21,3,53,500,3,54,350,1,330,1,1,325,1,3,0,500,3,1,350"; sleep 0.02
  for d in 1 2 3 4 5 6 7 8 9 10; do ev "3,47,0,3,53,$((500+d*10)),3,0,$((500+d*10))"; sleep 0.016; done
  ev "3,47,0,3,57,-1,1,330,0,1,325,0"; sleep 1
  mark END
} | uinput -
