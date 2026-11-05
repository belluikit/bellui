//go:build !gl

package webgpu

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
)

// captureLog redirects the standard logger, which bellui.LogError writes to.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })
	return &buf
}

func refusingPainter() *painter {
	return &painter{dev: &Device{
		config: &wgpu.SurfaceConfiguration{Format: wgpu.TextureFormatRGBA8Unorm},
	}}
}

// TestRefusedDrawIsLoggedOnce pins #958: a draw whose uniform block is larger
// than an arena slot is refused, and the refusal is logged once, naming the
// shader and the size, not dropped in silence and not logged every frame.
func TestRefusedDrawIsLoggedOnce(t *testing.T) {
	buf := captureLog(t)
	p := refusingPainter()
	quad := make([]float32, 16)
	oversized := make([]byte, uniformBlockStride+32)

	p.recordWithBlend("simple", quad, 4, oversized, nil, BlendPremul)
	out := buf.String()
	assert.Contains(t, out, `shader "simple"`)
	assert.Contains(t, out, "uniform block 288 bytes exceeds arena stride 256")

	p.recordWithBlend("simple", quad, 4, oversized, nil, BlendPremul)
	assert.Equal(t, 1, strings.Count(buf.String(), "was refused"), "a second identical draw must not log again")

	// Another shader refused for the same reason is a new report.
	p.recordWithBlend("rectangle", quad, 4, oversized, nil, BlendPremul)
	assert.Equal(t, 2, strings.Count(buf.String(), "was refused"))
}

// TestRecordRefusalIsLogged covers the refusals Device.Record itself returns,
// here a draw outside a frame, and an unknown shader.
func TestRecordRefusalIsLogged(t *testing.T) {
	buf := captureLog(t)
	p := refusingPainter()
	quad := make([]float32, 16)

	p.recordWithBlend("simple", quad, 4, nil, nil, BlendPremul)
	assert.Contains(t, buf.String(), "Record called outside a frame")

	p.record("no-such-shader", quad, 4, nil, nil)
	assert.Contains(t, buf.String(), `shader "no-such-shader"`)
	assert.Contains(t, buf.String(), "no such shader")
	assert.Equal(t, 2, strings.Count(buf.String(), "was refused"))
}
