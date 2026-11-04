package mobile

import (
	"testing"

	"github.com/belluikit/bellui/cmd/bellui/internal/mobile/binres"
)

// TestAndroidTargetSDK pins the target API level at Google Play's floor
// (#935), so the next floor is a deliberate bump that reads that level's
// behaviour changes, not a number edited on its own. The resource table must
// come from a platform at least that new, or attributes the target relies on
// would not resolve.
func TestAndroidTargetSDK(t *testing.T) {
	if androidTargetSDK != 36 {
		t.Errorf("androidTargetSDK = %d, want 36 (Play's floor since 2026-08-31)", androidTargetSDK)
	}
	if binres.SDKPlatformVersion < androidTargetSDK {
		t.Errorf("binres.SDKPlatformVersion = %d is older than androidTargetSDK %d", binres.SDKPlatformVersion, androidTargetSDK)
	}
}
