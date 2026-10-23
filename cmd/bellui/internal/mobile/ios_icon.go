package mobile

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// iosAppIconSizes are the square PNGs, Icon_<size>.png, that the AppIcon
// set's Contents.json (templates.XCAssetsDarwin) names: iPhone 60pt @2x and
// @3x, iPad 76pt @1x and @2x, and the 1024 px marketing icon.
var iosAppIconSizes = []int{120, 180, 76, 152, 1024}

// writeIOSAppIcon fills the generated Xcode project's AppIcon set with icon,
// resized to each size the set names. Xcode then compiles the set while it
// builds and signs the app, so the icons are sealed with everything else.
// They used to be compiled into the finished .app afterwards, which broke
// its signature (#677). With no icon the set is left empty, which Xcode
// still expects.
func writeIOSAppIcon(icon, setDir string) error {
	if err := mkdir(setDir); err != nil {
		return err
	}
	if icon == "" {
		if buildN {
			return nil
		}
		return os.WriteFile(filepath.Join(setDir, "Contents.json"), []byte(contentsJSON), 0o600)
	}
	contents := new(bytes.Buffer)
	if err := templates.XCAssetsDarwin.Execute(contents, nil); err != nil {
		return err
	}
	if !buildN {
		if err := os.WriteFile(filepath.Join(setDir, "Contents.json"), contents.Bytes(), 0o600); err != nil {
			return err
		}
	}
	for _, size := range iosAppIconSizes {
		s := strconv.Itoa(size)
		cmd := exec.Command("sips", "-o", filepath.Join(setDir, "Icon_"+s+".png"), "-Z", s, icon)
		if err := runCmd(cmd); err != nil {
			return err
		}
	}
	return nil
}
