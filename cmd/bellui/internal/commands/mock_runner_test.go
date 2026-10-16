package commands

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type expectedValue struct {
	dir   any
	env   []string
	osEnv bool
	args  []string
}

type mockReturn struct {
	ret []byte
	err error
}

type expectedCall struct {
	dirSet bool
	envSet bool
}

type mockRunner struct {
	expectedValue
	expectedCall
	mockReturn
}

type testCommandRuns struct {
	runs       []mockRunner
	currentRun int
	t          *testing.T
}

func (t *testCommandRuns) runOutput(args ...string) ([]byte, error) {
	require.Less(t.t, t.currentRun, len(t.runs))
	require.Equal(t.t, len(t.runs[t.currentRun].args), len(args))

	expectedArgs := t.runs[t.currentRun].args
	require.Equal(t.t, expectedArgs, args)

	ret, err := t.runs[t.currentRun].ret, t.runs[t.currentRun].err
	t.currentRun++

	return ret, err
}

func (t *testCommandRuns) setDir(dir string) {
	require.Less(t.t, t.currentRun, len(t.runs))

	require.Equal(t.t, t.runs[t.currentRun].dir.(string), dir)
	t.runs[t.currentRun].dirSet = true
}

func (t *testCommandRuns) setEnv(env []string) {
	require.Less(t.t, t.currentRun, len(t.runs))

	// Prepare array for comparison
	expectedEnv := t.runs[t.currentRun].env
	if t.runs[t.currentRun].osEnv {
		expectedEnv = append(expectedEnv, os.Environ()...)
	}
	// Never print the environment: it holds whatever secrets the developer's
	// shell or the CI runner exports. Report the names that differ (#371).
	if missing, unexpected := envDiff(expectedEnv, env); len(missing)+len(unexpected) > 0 {
		t.t.Fatalf("command environment differs (variable names only): missing %v, unexpected %v", missing, unexpected)
	}

	t.runs[t.currentRun].envSet = true
}

func (t *testCommandRuns) verifyExpectation() {
	require.Equal(t.t, len(t.runs), t.currentRun)

	for _, value := range t.runs {
		if value.dir != nil {
			assert.Equal(t.t, true, value.dirSet)
		}
		if len(value.env) > 0 {
			assert.Equal(t.t, true, value.envSet)
		}
	}
}

// envDiff returns the names of the entries in want that are not in got, and of
// the entries in got that are not in want, counting duplicates.
func envDiff(want, got []string) (missing, unexpected []string) {
	count := map[string]int{}
	for _, kv := range got {
		count[kv]++
	}
	for _, kv := range want {
		if count[kv] > 0 {
			count[kv]--
			continue
		}
		missing = append(missing, envName(kv))
	}
	for kv, n := range count {
		for ; n > 0; n-- {
			unexpected = append(unexpected, envName(kv))
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	return missing, unexpected
}

func envName(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}
