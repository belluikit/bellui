//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !wasm

package gl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// depthRecordingContext records the texture and framebuffer calls issued by
// attachSceneDepth. Every other context method comes from the embedded nil
// interface and panics if called, so the test also asserts that the depth
// attachment touches nothing else. Texture is an opaque handle (an integer on
// desktop, a struct on mobile), so the test compares calls and counts rather
// than assuming a numeric type.
type depthRecordingContext struct {
	context
	created int
	deleted int

	imageTarget         uint32
	imageWidth          int
	imageHeight         int
	imageFormat         uint32
	imageType           uint32
	fbAttachment        uint32
	framebufferTextures int
}

func (c *depthRecordingContext) CreateTexture() Texture {
	c.created++
	return noTexture
}

func (c *depthRecordingContext) BindTexture(_ uint32, _ Texture) {}

func (c *depthRecordingContext) TexImage2D(target uint32, _, width, height int, colorFormat, typ uint32, _ []uint8) {
	c.imageTarget, c.imageWidth, c.imageHeight = target, width, height
	c.imageFormat, c.imageType = colorFormat, typ
}

func (c *depthRecordingContext) TexParameteri(_, _ uint32, _ int32) {}

func (c *depthRecordingContext) FramebufferTexture2D(_, attachment uint32, _ Texture, _ int32) {
	c.fbAttachment = attachment
	c.framebufferTextures++
}

func (c *depthRecordingContext) DeleteTexture(_ Texture) {
	c.deleted++
}

// The non-wasm Scene3D depth attachment uses a depth *texture* (DEPTH_COMPONENT
// / UNSIGNED_SHORT) sized to the FBO, attaches it once at DEPTH_ATTACHMENT, and
// releases exactly that texture — never the colour texture, which the caller
// keeps for caching.
func TestAttachSceneDepth_Texture(t *testing.T) {
	ctx := &depthRecordingContext{}
	p := &painter{ctx: ctx}

	release := p.attachSceneDepth(64, 48)

	assert.Equal(t, 1, ctx.created, "one depth texture created")

	assert.Equal(t, uint32(texture2D), ctx.imageTarget)
	assert.Equal(t, 64, ctx.imageWidth)
	assert.Equal(t, 48, ctx.imageHeight)
	assert.Equal(t, uint32(depthComponent), ctx.imageFormat)
	assert.Equal(t, uint32(unsignedShort), ctx.imageType)

	assert.Equal(t, 1, ctx.framebufferTextures, "depth attached once")
	assert.Equal(t, uint32(depthAttachment), ctx.fbAttachment)

	assert.Zero(t, ctx.deleted, "nothing deleted before release")
	release()
	assert.Equal(t, 1, ctx.deleted, "release deletes exactly the depth texture")
}
