package app

import (
	"errors"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestBelluiApp_SetCloudProvider(t *testing.T) {
	a := test.NewTempApp(t)
	p := &mockCloud{}
	a.(bellui.CloudApp).SetCloudProvider(p)

	assert.Equal(t, p, a.(bellui.CloudApp).CloudProvider())
	assert.True(t, p.configured)
}

func TestBelluiApp_SetCloudProvider_Cleanup(t *testing.T) {
	a := test.NewTempApp(t)
	p1 := &mockCloud{}
	p2 := &mockCloud{}
	a.(bellui.CloudApp).SetCloudProvider(p1)

	assert.True(t, p1.configured)
	assert.False(t, p1.cleaned)

	a.(bellui.CloudApp).SetCloudProvider(p2)

	assert.True(t, p1.cleaned)
	assert.True(t, p2.configured)
}

func TestBelluiApp_transitionCloud(t *testing.T) {
	a := test.NewTempApp(t)
	p := &mockCloud{}
	preferenceChanged := false
	settingsChan := make(chan bellui.Settings)
	a.Preferences().AddChangeListener(func() {
		preferenceChanged = true
	})
	a.Settings().AddListener(func(s bellui.Settings) {
		go func() { settingsChan <- s }()
	})

	done := make(chan struct{})
	go func() {
		<-settingsChan // settings were updated
		assert.True(t, preferenceChanged)
		close(done)
	}()

	a.(bellui.CloudApp).SetCloudProvider(p)
	<-done
}

func TestBelluiApp_transitionCloud_Preferences(t *testing.T) {
	a := test.NewTempApp(t)
	bellui.SetPref(a.Preferences(), "key", "blank")

	assert.Equal(t, "blank", bellui.PrefOr(a.Preferences(), "key", ""))

	p := &mockCloud{}
	a.(bellui.CloudApp).SetCloudProvider(p)

	assert.Equal(t, "", bellui.PrefOr(a.Preferences(), "key", ""))
}

func TestBelluiApp_transitionCloud_Storage(t *testing.T) {
	a := test.NewTempApp(t)
	a.Storage().Create("nothere")

	// The test app storage root (os.TempDir()/Documents) is shared across test
	// apps and persists on self-hosted CI runners, so it may already hold files
	// from other tests or prior runs (e.g. a stray analytics spool). Assert our
	// file is present rather than that the shared dir has exactly one entry.
	assert.Contains(t, a.Storage().List(), "nothere")

	p := &mockCloud{}
	a.(bellui.CloudApp).SetCloudProvider(p)

	// The cloud provider does not implement CloudProviderStorage, so the app
	// swaps to a fresh (empty) local storage on transition.
	assert.Empty(t, a.Storage().List())
}

type mockCloud struct {
	configured, cleaned bool
}

func (c *mockCloud) Cleanup(_ bellui.App) {
	c.cleaned = true
}

func (c *mockCloud) CloudPreferences(bellui.App) bellui.Preferences {
	return &internal.InMemoryPreferences{}
}

func (c *mockCloud) CloudStorage(bellui.App) bellui.Storage {
	return &mockCloudStorage{}
}

func (c *mockCloud) ProviderDescription() string {
	return "Mock cloud implementation"
}

func (c *mockCloud) ProviderIcon() bellui.Resource {
	return theme.ComputerIcon()
}

func (c *mockCloud) ProviderName() string {
	return "mock"
}

func (c *mockCloud) Setup(_ bellui.App) error {
	c.configured = true
	return nil
}

type mockCloudStorage struct{}

func (s *mockCloudStorage) Create(name string) (bellui.URIWriteCloser, error) {
	return nil, errors.New("not implemented")
}

func (s *mockCloudStorage) List() []string {
	return []string{}
}

func (s *mockCloudStorage) Open(name string) (bellui.URIReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (s *mockCloudStorage) Remove(name string) error {
	return errors.New("not implemented")
}

func (s *mockCloudStorage) RootURI() bellui.URI {
	u, _ := storage.ParseURI("mock://")
	return u
}

func (s *mockCloudStorage) Save(name string) (bellui.URIWriteCloser, error) {
	return nil, errors.New("not implemented")
}
