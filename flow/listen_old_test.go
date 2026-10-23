package flow

import "testing"

// Listen promises (old, new); #339 found it passed the new value twice.
func TestValue_ListenReceivesOldAndNew(t *testing.T) {
	v := New("a")
	var gotOld, gotNew string
	v.Listen(func(old, new string) { gotOld, gotNew = old, new })

	v.Set("b")
	if gotOld != "a" || gotNew != "b" {
		t.Fatalf("Set: got (old=%q, new=%q), want (a, b)", gotOld, gotNew)
	}
	v.Modify(func(s string) string { return s + "c" })
	if gotOld != "b" || gotNew != "bc" {
		t.Fatalf("Modify: got (old=%q, new=%q), want (b, bc)", gotOld, gotNew)
	}
}

func TestDerived_ListenReceivesOldAndNew(t *testing.T) {
	src := New(1)
	d := Derive(func() int { return src.Get() * 10 }, src)
	var gotOld, gotNew int
	d.Listen(func(old, new int) { gotOld, gotNew = old, new })

	src.Set(2)
	if gotOld != 10 || gotNew != 20 {
		t.Fatalf("got (old=%d, new=%d), want (10, 20)", gotOld, gotNew)
	}
}
