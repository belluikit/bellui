package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestPrefs_SetBool(t *testing.T) {
	p := NewInMemoryPreferences()
	bellui.SetPref(p, "testBool", true)

	assert.Equal(t, true, bellui.PrefOr(p, "testBool", false))
}

func TestPrefs_Bool(t *testing.T) {
	p := NewInMemoryPreferences()
	p.WriteValues(func(val map[string]any) {
		val["testBool"] = true
	})

	assert.Equal(t, true, bellui.PrefOr(p, "testBool", false))
	bellui.SetPref(p, "testBool", "fail")

	assert.Equal(t, false, bellui.PrefOr(p, "testBool", false))
}

func TestPrefs_BoolListWithFallback(t *testing.T) {
	p := NewInMemoryPreferences()
	boolstf := []bool{true, false}
	boolstt := []bool{true, true}

	assert.Equal(t, boolstf, bellui.PrefOr(p, "testBoolList", boolstf))
	bellui.SetPref(p, "testBool", "fail")
	assert.Equal(t, boolstt, bellui.PrefOr(p, "testBool", boolstt))
}

func TestPrefs_BoolWithFallback(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, true, bellui.PrefOr(p, "testBool", true))
	bellui.SetPref(p, "testBool", false)
	assert.Equal(t, 1, bellui.PrefOr(p, "testBool", 1))
	bellui.SetPref(p, "testBool", "fail")
	assert.Equal(t, "fail", bellui.PrefOr(p, "testBool", "fail"))
}

func TestPrefs_Bool_Zero(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, false, bellui.PrefOr(p, "testBool", false))
}

func TestPrefs_SetFloat(t *testing.T) {
	p := NewInMemoryPreferences()
	bellui.SetPref(p, "testFloat", 1.7)

	assert.Equal(t, 1.7, bellui.PrefOr(p, "testFloat", 0.0))
}

func TestPrefs_Float(t *testing.T) {
	p := NewInMemoryPreferences()
	p.WriteValues(func(val map[string]any) {
		val["testFloat"] = 1.2
	})

	assert.Equal(t, 1.2, bellui.PrefOr(p, "testFloat", 0.0))
}

func TestPrefs_FloatWithFallback(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, 1.0, bellui.PrefOr(p, "testFloat", 1.0))
	p.WriteValues(func(val map[string]any) {
		val["testFloat"] = 1.2
	})
	assert.Equal(t, 1.2, bellui.PrefOr(p, "testFloat", 1.0))

	assert.Equal(t, "bad", bellui.PrefOr(p, "testFloat", "bad"))

	assert.Equal(t, 1.2, bellui.PrefOr(p, "testFloat", 1.3))
}

func TestPrefs_Float_Zero(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, 0.0, bellui.PrefOr(p, "testFloat", 0.0))
}

func TestPrefs_SetInt(t *testing.T) {
	p := NewInMemoryPreferences()
	bellui.SetPref(p, "testInt", 5)

	assert.Equal(t, 5, bellui.PrefOr(p, "testInt", 0))
}

func TestPrefs_Int(t *testing.T) {
	p := NewInMemoryPreferences()
	p.WriteValues(func(val map[string]any) {
		val["testInt"] = 5
	})
	assert.Equal(t, 5, bellui.PrefOr(p, "testInt", 0))
}

func TestPrefs_IntWithFallback(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, 2, bellui.PrefOr(p, "testInt", 2))
	p.WriteValues(func(val map[string]any) {
		val["testInt"] = 5
	})
	assert.Equal(t, 5, bellui.PrefOr(p, "testInt", 2))

	assert.Equal(t, true, bellui.PrefOr(p, "testInt", true))

	assert.Equal(t, 5.0, bellui.PrefOr(p, "testInt", 1.2))

	assert.Equal(t, 5, bellui.PrefOr(p, "testInt", 2))
}

func TestPrefs_Int_Zero(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, 0, bellui.PrefOr(p, "testInt", 0))
}

func TestPrefs_SetString(t *testing.T) {
	p := NewInMemoryPreferences()
	bellui.SetPref(p, "test", "value")

	assert.Equal(t, "value", bellui.PrefOr(p, "test", ""))
}

func TestPrefs_String(t *testing.T) {
	p := NewInMemoryPreferences()
	p.WriteValues(func(val map[string]any) {
		val["test"] = "value"
	})

	assert.Equal(t, "value", bellui.PrefOr(p, "test", ""))
}

func TestPrefs_StringWithFallback(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, "default", bellui.PrefOr(p, "test", "default"))
	p.WriteValues(func(val map[string]any) {
		val["test"] = "value"
	})
	assert.Equal(t, "value", bellui.PrefOr(p, "test", "default"))

	assert.Equal(t, true, bellui.PrefOr(p, "test", true))

	assert.Equal(t, "value", bellui.PrefOr(p, "test", "default"))
}

func TestPrefs_String_Zero(t *testing.T) {
	p := NewInMemoryPreferences()

	assert.Equal(t, "", bellui.PrefOr(p, "test", ""))
}

func TestInMemoryPreferences_OnChange(t *testing.T) {
	p := NewInMemoryPreferences()
	called := false
	p.AddChangeListener(func() {
		called = true
	})

	bellui.SetPref(p, "dummy", "another")
	time.Sleep(time.Millisecond * 100)
	assert.True(t, called)

	called = false
	p.RemoveValue("dummy")
	time.Sleep(time.Millisecond * 100)
	assert.True(t, called)
}

func TestRemoveValue(t *testing.T) {
	p := NewInMemoryPreferences()

	bellui.SetPref(p, "dummy", true)
	bellui.SetPref(p, "pi", 3.14)
	bellui.SetPref(p, "number", 2)
	bellui.SetPref(p, "month", "January")

	p.RemoveValue("dummy")
	p.RemoveValue("pi")
	p.RemoveValue("number")
	p.RemoveValue("month")

	assert.Equal(t, false, bellui.PrefOr(p, "dummy", false))
	assert.Equal(t, float64(0), bellui.PrefOr(p, "pi", float64(0)))
	assert.Equal(t, 0, bellui.PrefOr(p, "number", 0))
	assert.Equal(t, "", bellui.PrefOr(p, "month", ""))
}

func TestPrefs_SetSameValue(t *testing.T) {
	p := NewInMemoryPreferences()
	called := 0
	p.AddChangeListener(func() {
		called++
	})

	// We should not fire change when it hasn't changed.
	for i := 0; i < 2; i++ {
		bellui.SetPref(p, "enabled", true)
		time.Sleep(time.Millisecond * 100)

		assert.Equal(t, 1, called)
	}

	bellui.SetPref(p, "enabled", false)
	time.Sleep(time.Millisecond * 100)

	assert.Equal(t, 2, called)
}

func TestPrefs_SetSameSliceValue(t *testing.T) {
	p := NewInMemoryPreferences()
	called := 0
	p.AddChangeListener(func() {
		called++
	})

	// We should not fire change when it hasn't changed.
	for i := 0; i < 2; i++ {
		bellui.SetPref(p, "items", []string{"1", "2"})
		time.Sleep(time.Millisecond * 100)

		assert.Equal(t, 1, called)
	}

	bellui.SetPref(p, "items", []string{"3", "4"})
	time.Sleep(time.Millisecond * 100)

	assert.Equal(t, 2, called)
}

func TestPrefs_Coerce_IntFromFloat(t *testing.T) {
	p := NewInMemoryPreferences()
	// Simulate JSON deserialization: int stored as float64
	p.WriteValues(func(val map[string]any) {
		val["num"] = float64(42)
	})

	v, ok := bellui.Pref[int](p, "num")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestPrefs_Coerce_FloatFromInt(t *testing.T) {
	p := NewInMemoryPreferences()
	p.WriteValues(func(val map[string]any) {
		val["num"] = 42
	})

	v, ok := bellui.Pref[float64](p, "num")
	assert.True(t, ok)
	assert.Equal(t, 42.0, v)
}

func TestPrefs_Coerce_IntListFromFloatList(t *testing.T) {
	p := NewInMemoryPreferences()
	p.WriteValues(func(val map[string]any) {
		val["nums"] = []float64{1.0, 2.0, 3.0}
	})

	v, ok := bellui.Pref[[]int](p, "nums")
	assert.True(t, ok)
	assert.Equal(t, []int{1, 2, 3}, v)
}

func TestPrefs_Coerce_IntListFromAnyList(t *testing.T) {
	p := NewInMemoryPreferences()
	// Simulate raw JSON array deserialization
	p.WriteValues(func(val map[string]any) {
		val["nums"] = []any{float64(1), float64(2)}
	})

	v, ok := bellui.Pref[[]int](p, "nums")
	assert.True(t, ok)
	assert.Equal(t, []int{1, 2}, v)
}

func TestPrefs_ChangeListenersCopy(t *testing.T) {
	p := NewInMemoryPreferences()
	p.AddChangeListener(func() {})
	p.AddChangeListener(func() {})

	listeners := p.ChangeListenersCopy()
	assert.Len(t, listeners, 2)
}
