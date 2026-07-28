package webview

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadState_ReportsTransitions(t *testing.T) {
	w := New("")
	assert.Equal(t, LoadStateIdle, w.LoadState())

	var events []LoadEvent
	w.OnLoadChanged = func(ev LoadEvent) { events = append(events, ev) }

	w.deliverLoad(LoadStateLoading, "https://example.com", "")
	w.deliverLoad(LoadStateComplete, "https://example.com/landed", "")

	require.Len(t, events, 2)
	assert.Equal(t, LoadStateLoading, events[0].State)
	assert.NoError(t, events[0].Err)
	assert.Equal(t, LoadStateComplete, w.LoadState())

	// A redirect makes the engine's URL the truth, so URL() follows it.
	assert.Equal(t, "https://example.com/landed", w.URL())
}

func TestLoadState_FailureCarriesTheReason(t *testing.T) {
	w := New("")

	var got LoadEvent
	w.OnLoadChanged = func(ev LoadEvent) { got = ev }
	w.deliverLoad(LoadStateFailed, "https://example.com", "A server with the specified hostname could not be found.")

	assert.Equal(t, LoadStateFailed, got.State)
	assert.ErrorContains(t, got.Err, "could not be found")
	assert.Equal(t, LoadStateFailed, w.LoadState())
}

func TestLoadState_String(t *testing.T) {
	assert.Equal(t, "idle", LoadStateIdle.String())
	assert.Equal(t, "loading", LoadStateLoading.String())
	assert.Equal(t, "complete", LoadStateComplete.String())
	assert.Equal(t, "failed", LoadStateFailed.String())
	assert.Equal(t, "unknown", LoadState(42).String())
}

func TestNavigationKind_String(t *testing.T) {
	assert.Equal(t, "other", NavigationOther.String())
	assert.Equal(t, "link", NavigationLinkClicked.String())
	assert.Equal(t, "form", NavigationFormSubmitted.String())
	assert.Equal(t, "back-forward", NavigationBackForward.String())
	assert.Equal(t, "reload", NavigationReload.String())
	assert.Equal(t, "unknown", NavigationKind(9).String())
}

func TestOnNavigate_AllowsEverythingWhenUnset(t *testing.T) {
	w := New("")
	assert.True(t, w.decideNavigation(NavigationRequest{URL: "https://example.com", MainFrame: true}))
}

func TestOnNavigate_CanVetoAndReportsItAsCancelled(t *testing.T) {
	w := New("")
	w.OnLoadChanged = func(LoadEvent) {}

	var seen []NavigationRequest
	w.OnNavigate = func(req NavigationRequest) bool {
		seen = append(seen, req)
		return req.URL == "https://allowed.example/"
	}

	var events []LoadEvent
	w.OnLoadChanged = func(ev LoadEvent) { events = append(events, ev) }

	assert.True(t, w.decideNavigation(NavigationRequest{
		URL: "https://allowed.example/", Kind: NavigationLinkClicked, MainFrame: true,
	}))
	assert.False(t, w.decideNavigation(NavigationRequest{
		URL: "https://elsewhere.example/", Kind: NavigationLinkClicked, MainFrame: true,
	}))

	require.Len(t, seen, 2)
	assert.Equal(t, NavigationLinkClicked, seen[0].Kind)

	// A refusal is a load failure with a reason of its own, so a UI watching
	// OnLoadChanged never leaves a spinner up and never blames the network.
	require.Len(t, events, 1)
	assert.Equal(t, LoadStateFailed, events[0].State)
	assert.ErrorIs(t, events[0].Err, ErrNavigationCancelled)
	assert.Equal(t, "https://elsewhere.example/", events[0].URL)
}

func TestOnNavigate_RefusalLeavesURLWhereTheViewActuallyIs(t *testing.T) {
	w := New("")
	w.deliverLoad(LoadStateComplete, "https://home.example/", "")

	w.OnNavigate = func(NavigationRequest) bool { return false }
	assert.False(t, w.decideNavigation(NavigationRequest{URL: "https://blocked.example/", MainFrame: true}))

	// A refused navigation never happened, so URL() must not name the page the
	// view was kept away from — otherwise a caller reading it back would think
	// the policy had failed.
	assert.Equal(t, "https://home.example/", w.URL())
}

func TestLoadFailure_LeavesURLWhereTheViewActuallyIs(t *testing.T) {
	w := New("")
	w.deliverLoad(LoadStateComplete, "https://home.example/", "")
	w.deliverLoad(LoadStateFailed, "https://gone.example/", "hostname could not be found")

	assert.Equal(t, "https://home.example/", w.URL())
	assert.Equal(t, LoadStateFailed, w.LoadState())
}

func TestOnNavigate_SubFrameRefusalIsNotALoadFailure(t *testing.T) {
	w := New("")
	w.deliverLoad(LoadStateComplete, "https://example.com", "")

	w.OnNavigate = func(NavigationRequest) bool { return false }
	w.OnLoadChanged = func(LoadEvent) { t.Fatal("a blocked frame must not fail the page's own load") }

	assert.False(t, w.decideNavigation(NavigationRequest{URL: "https://ads.example", MainFrame: false}))
	assert.Equal(t, LoadStateComplete, w.LoadState())
}

func TestLoadURL_IsRememberedBeforeAttach(t *testing.T) {
	w := New("https://first.example")
	assert.Equal(t, "https://first.example", w.URL())

	w.LoadURL("https://second.example")
	assert.Equal(t, "https://second.example", w.URL(),
		"an unattached view remembers the URL and loads it on attach")
}

func TestNavigation_UnattachedQueriesAreFalse(t *testing.T) {
	w := New("")
	assert.False(t, w.CanGoBack())
	assert.False(t, w.GoBack())
	assert.False(t, w.CanGoForward())
	assert.False(t, w.GoForward())

	w.Reload()      // no-op, not a panic
	w.StopLoading() // ditto
	w.LoadHTML("<p>hi</p>", "")
	assert.Equal(t, LoadStateIdle, w.LoadState())
}
