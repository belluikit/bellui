package binding

import (
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

func TestBindPreference_DataRace(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	key := "test-key"
	const n = 100

	var wg sync.WaitGroup

	binds := make([]Int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		index := i
		bellui.Do(func() {
			bind := BindPreferenceInt(key, p)
			binds[index] = bind
			wg.Done()
		})
	}

	wg.Wait()
	expectedBind := binds[0]
	for i := 0; i < n; i++ {
		assert.Equal(t, expectedBind, binds[i])
	}
}

func TestBindPreferenceBool(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	key := "test-bool"

	bellui.SetPref(p, key, true)
	bind := BindPreferenceBool(key, p)
	v, err := bind.Get()
	assert.Nil(t, err)
	assert.True(t, v)

	err = bind.Set(false)
	assert.Nil(t, err)
	v, err = bind.Get()
	assert.Nil(t, err)
	assert.False(t, v)
	assert.Equal(t, false, bellui.PrefOr(p, key, false))
}

func TestBindPreferenceFloat(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	key := "test-float"

	bellui.SetPref(p, key, 1.3)
	bind := BindPreferenceFloat(key, p)
	v, err := bind.Get()
	assert.Nil(t, err)
	assert.Equal(t, 1.3, v)

	err = bind.Set(2.5)
	assert.Nil(t, err)
	v, err = bind.Get()
	assert.Nil(t, err)
	assert.Equal(t, 2.5, v)
	assert.Equal(t, 2.5, bellui.PrefOr(p, key, 0.0))
}

func TestBindPreferenceInt(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	key := "test-int"

	bellui.SetPref(p, key, 4)
	bind := BindPreferenceInt(key, p)
	v, err := bind.Get()
	assert.Nil(t, err)
	assert.Equal(t, 4, v)

	err = bind.Set(7)
	assert.Nil(t, err)
	v, err = bind.Get()
	assert.Nil(t, err)
	assert.Equal(t, 7, v)
	assert.Equal(t, 7, bellui.PrefOr[int](p, key, 0))
}

func TestBindPreferenceString(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	key := "test-string"

	bellui.SetPref(p, key, "aString")
	bind := BindPreferenceString(key, p)
	v, err := bind.Get()
	assert.Nil(t, err)
	assert.Equal(t, "aString", v)

	err = bind.Set("overwritten")
	assert.Nil(t, err)
	v, err = bind.Get()
	assert.Nil(t, err)
	assert.Equal(t, "overwritten", v)
	assert.Equal(t, "overwritten", bellui.PrefOr(p, key, ""))
}

func TestPreferenceBindingCopies(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	key := "test-string"

	bellui.SetPref(p, key, "aString")
	bind1 := BindPreferenceString(key, p)
	bind2 := BindPreferenceString(key, p)
	v1, err := bind1.Get()
	assert.Nil(t, err)
	v2, err := bind2.Get()
	assert.Nil(t, err)
	assert.Equal(t, v2, v1)

	err = bind1.Set("overwritten")
	assert.Nil(t, err)
	v1, err = bind1.Get()
	assert.Nil(t, err)
	v2, err = bind2.Get()
	assert.Nil(t, err)
	assert.Equal(t, v2, v1)
}

func TestPreferenceBindingTriggers(t *testing.T) {
	a := test.NewApp()
	p := a.Preferences()
	prefBinds.appPrefs = nil // reset app migration checks
	key := "test-string"

	bellui.SetPref(p, key, "aString")
	bind1 := BindPreferenceString(key, p)
	bind2 := BindPreferenceString(key, p)

	ch := make(chan any, 2)
	bind1.AddListener(NewDataListener(func() {
		ch <- struct{}{}
	}))

	select {
	case <-ch: // bind1 gets initial value
	case <-time.After(time.Millisecond * 100):
		t.Errorf("Timed out waiting for data binding to send initial value")
	}

	err := bind2.Set("overwritten") // write on a different listener, preferences should trigger all
	assert.Nil(t, err)
	select {
	case <-ch: // bind1 triggered by bind2 changing the same key
	case <-time.After(time.Millisecond * 100):
		t.Errorf("Timed out waiting for data binding change to trigger")
	}

	bellui.SetPref(p, key, "overwritten2") // changing preference should trigger as well
	select {
	case <-ch: // bind1 triggered by preferences changing the same key directly
	case <-time.After(time.Millisecond * 300):
		t.Errorf("Timed out waiting for data binding change to trigger")
	}
}
