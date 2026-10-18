package commands

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/mobile"
)

// actool compiles the asset catalog for the deployment target the binary and
// the Xcode project use, not a separate constant (it was a fixed 9.0, #410).
func TestIOSActoolArgsUseTheDeploymentTarget(t *testing.T) {
	args := iosActoolArgs("/tmp/App.app")
	i := slices.Index(args, "--minimum-deployment-target")
	require.GreaterOrEqual(t, i, 0)
	require.Less(t, i+1, len(args))
	assert.Equal(t, mobile.IOSDeploymentTarget(), args[i+1])
	assert.Equal(t, mobile.MinIOSVersion, args[i+1])
}
