package component

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/style"
)

func testResolver() *style.Resolver {
	app := test.NewApp()
	th := app.Settings().Theme()
	variant := app.Settings().ThemeVariant()
	return style.NewResolver(th, variant)
}

func TestRenderTitle_NonEmpty(t *testing.T) {
	resolver := testResolver()
	cfg := &TitleConfig{Text: "My Chart"}
	objs, height := RenderTitle(cfg, resolver, 10, 10, 500)

	if len(objs) == 0 {
		t.Fatal("expected at least one object for non-empty title")
	}
	if height <= 0 {
		t.Errorf("expected positive height, got %f", height)
	}
}

func TestRenderTitle_WithSubText(t *testing.T) {
	resolver := testResolver()
	cfg := &TitleConfig{Text: "Main", SubText: "Sub"}
	objs, height := RenderTitle(cfg, resolver, 10, 10, 500)

	// Main title + subtitle = 2 objects
	if len(objs) != 2 {
		t.Errorf("expected 2 objects (title + subtitle), got %d", len(objs))
	}
	if height <= 0 {
		t.Errorf("expected positive height, got %f", height)
	}
}

func TestRenderTitle_Empty(t *testing.T) {
	resolver := testResolver()

	// Nil config
	objs, height := RenderTitle(nil, resolver, 10, 10, 500)
	if objs != nil {
		t.Errorf("expected nil objects for nil config, got %d", len(objs))
	}
	if height != 0 {
		t.Errorf("expected zero height for nil config, got %f", height)
	}

	// Empty text
	cfg := &TitleConfig{Text: ""}
	objs, height = RenderTitle(cfg, resolver, 10, 10, 500)
	if objs != nil {
		t.Errorf("expected nil objects for empty text, got %d", len(objs))
	}
	if height != 0 {
		t.Errorf("expected zero height for empty text, got %f", height)
	}
}

func TestRenderTitle_Position(t *testing.T) {
	resolver := testResolver()
	cfg := &TitleConfig{Text: "Test"}
	objs, _ := RenderTitle(cfg, resolver, 20, 30, 400)

	if len(objs) == 0 {
		t.Fatal("no objects returned")
	}
	pos := objs[0].Position()
	if pos.X != 20 {
		t.Errorf("expected X=20, got %f", pos.X)
	}
	// Y should be offset from 30 by padding
	if pos.Y < 30 {
		t.Errorf("expected Y >= 30, got %f", pos.Y)
	}
}

func TestRenderTitle_SizeConstraint(t *testing.T) {
	resolver := testResolver()
	cfg := &TitleConfig{Text: "Wide Title"}
	objs, _ := RenderTitle(cfg, resolver, 10, 10, 200)

	if len(objs) == 0 {
		t.Fatal("no objects returned")
	}
	size := objs[0].Size()
	if size.Width != 200 {
		t.Errorf("expected width 200, got %f", size.Width)
	}
}

// TestRenderTitle_OnlyTitle verifies that title-only (no subtitle) returns exactly one object.
func TestRenderTitle_OnlyTitle(t *testing.T) {
	resolver := testResolver()
	cfg := &TitleConfig{Text: "Solo"}
	objs, _ := RenderTitle(cfg, resolver, 0, 0, 300)

	if len(objs) != 1 {
		t.Errorf("expected 1 object for title-only, got %d", len(objs))
	}
}

// Ensure the test app is valid
var _ bellui.App = test.NewApp()
