package commands

import (
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/belluikit/bellui/cmd/bellui/internal/mobile"
)

// An Android package needs Android 6 unless --android-min-sdk says
// otherwise (#1086), on both package and release.
func TestAndroidMinSDKFlag(t *testing.T) {
	var got int
	f := intFlags["android-min-sdk"](&got).(*cli.IntFlag)
	if f.Value != 23 || mobile.DefaultAndroidMinSDK != 23 {
		t.Errorf("default %d (mobile %d), want 23", f.Value, mobile.DefaultAndroidMinSDK)
	}
	for _, cmd := range []*cli.Command{Package(), Release()} {
		found := false
		for _, fl := range cmd.Flags {
			for _, n := range fl.Names() {
				found = found || n == "android-min-sdk"
			}
		}
		if !found {
			t.Errorf("bellui %s has no --android-min-sdk", cmd.Name)
		}
	}
	if (&Packager{}).androidMinSDKOrDefault() != 23 {
		t.Error("a Packager built without the flag doesn't get the default")
	}
	if err := mobile.SetAndroidMinSDK(20); err == nil {
		t.Error("--android-min-sdk 20 was accepted")
	}
	if err := mobile.SetAndroidMinSDK(mobile.DefaultAndroidMinSDK); err != nil {
		t.Error(err)
	}
}
