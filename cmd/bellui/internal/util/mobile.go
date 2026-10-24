package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AndroidBuildToolsPath tries to find the location of the "build-tools" directory.
// This depends on ANDROID_HOME variable, so you should call RequireAndroidSDK() first.
// With several versions installed it picks the newest: an old d8 or aapt2
// cannot read the newest platform's android.jar, which is the one the build
// compiles and links against.
func AndroidBuildToolsPath() string {
	env := os.Getenv("ANDROID_HOME")
	dir := filepath.Join(env, "build-tools")

	// this may contain a version subdir
	files, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}

	childDir := ""
	for _, f := range files {
		if f.Name() == "zipalign" || f.Name() == "zipalign.exe" { // no version subdir
			return dir
		}
		if f.IsDir() && (childDir == "" || newerBuildTools(f.Name(), childDir)) {
			childDir = f.Name()
		}
	}

	if childDir == "" {
		return dir
	}
	return filepath.Join(dir, childDir)
}

// newerBuildTools reports whether build-tools version a is newer than b,
// comparing the dotted numbers numerically ("35.0.0" > "9.0.0"); a release
// is newer than a pre-release of the same numbers ("35.0.0" > "35.0.0-rc1").
func newerBuildTools(a, b string) bool {
	an, apre, _ := strings.Cut(a, "-")
	bn, bpre, _ := strings.Cut(b, "-")
	as, bs := strings.Split(an, "."), strings.Split(bn, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x > y
		}
	}
	if (apre == "") != (bpre == "") {
		return apre == ""
	}
	return apre > bpre
}

// IsAndroid returns true if the given os parameter represents one of the Android targets.
func IsAndroid(os string) bool {
	return strings.HasPrefix(os, "android")
}

// IsIOS returns true if the given os parameter represents one of the iOS targets (ios, iossimulator)
func IsIOS(os string) bool {
	return strings.HasPrefix(os, "ios")
}

// IsMobile returns true if the given os parameter represents a platform handled by gomobile.
func IsMobile(os string) bool {
	return IsIOS(os) || IsAndroid(os)
}

// RequireAndroidSDK will return an error if it cannot establish the location of a valid Android SDK installation.
// This is currently deduced using ANDROID_HOME environment variable.
func RequireAndroidSDK() error {
	if env, ok := os.LookupEnv("ANDROID_HOME"); !ok || env == "" {
		return fmt.Errorf("could not find android tools, missing ANDROID_HOME")
	}

	return nil
}

// Aapt2Path returns the path to the aapt2 executable in the build-tools.
func Aapt2Path() (string, error) {
	return AndroidBuildTool("aapt2")
}
