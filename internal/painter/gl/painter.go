//go:build gl || (!darwin && !linux && !windows && !wasm)

// Package gl provides a full Bellui render implementation using system OpenGL libraries.
package gl

import (
	"fmt"
	"image/color"
	"log"
	"runtime"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/gpu"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/theme"
)

// Verify painter implements the backend-neutral painter interface.
var _ gpu.Painter = (*painter)(nil)

// NewPainter creates a new GL based renderer for the provided canvas.
// If it is a master painter it will also initialise OpenGL
func NewPainter(c core.Canvas, ctx driver.WithContext) gpu.Painter {
	p := &painter{canvas: c, contextProvider: ctx}
	p.SetFrameBufferScale(1.0)
	return p
}

type painter struct {
	canvas                core.Canvas
	ctx                   context
	contextProvider       driver.WithContext
	program               ProgramState
	lineProgram           ProgramState
	rectangleProgram      ProgramState
	roundRectangleProgram ProgramState
	polygonProgram        ProgramState
	arcProgram            ProgramState
	rippleProgram         ProgramState
	blurHProgram          ProgramState
	blurVProgram          ProgramState
	pathProgram           ProgramState
	scene3dProgram        ProgramState
	texScale              float32
	groupOpacity          float32      // 1.0 = normal; < 1.0 = reduced opacity for disabled subtrees
	pixScale              float32      // pre-calculate scale*texScale for each draw
	currentTransform      geo.Matrix   // current world transform set via SetTransform
	scissorActive         bool         // a StartClipping scissor is currently enabled
	shaderInitFailed      bool         // a shader/program failed to build; logged once, rendering degrades
	baseProgramFailed     bool         // the base textured-quad ("simple") program failed; no GL diagnostic possible
	initFailureText       *canvas.Text // lazily-built "graphics unavailable" diagnostic message

	// boundaries maps RepaintBoundary objects to their cached FBO state.
	boundaries map[core.CanvasObject]*BoundaryFBOState

	// canvasFBO is the persistent full-canvas framebuffer for dirty-region culling.
	canvasFBO *canvasFBOState

	// targetStack tracks the active render targets (canvas FBO, nested
	// boundary FBOs). Temporary offscreen work (shadow blur, Scene3D) must
	// restore the top of this stack, NOT framebuffer 0 — mid-frame rendering
	// usually targets the canvas FBO, and binding 0 silently discards every
	// subsequent draw call of the frame.
	targetStack []renderTarget
}

// renderTarget is one entry of the painter's render-target stack.
type renderTarget struct {
	fbo     Framebuffer
	w, h    int     // viewport pixel dimensions
	logical float32 // logical height, for the scissor's y flip; 0 means the canvas's
}

type ProgramState struct {
	ref            Program
	buff           Buffer
	uniforms       map[string]*UniformState
	matrixUniforms map[string]*MatrixUniformState
	mat4Uniforms   map[string]*Mat4UniformState
	attributes     map[string]Attribute
}

type UniformState struct {
	ref  Uniform
	prev [4]float32
}

// MatrixUniformState caches a mat3 uniform value (9 floats).
type MatrixUniformState struct {
	ref  Uniform
	prev [9]float32
}

// Mat4UniformState caches a mat4 uniform value (16 floats).
type Mat4UniformState struct {
	ref  Uniform
	prev [16]float32
}

func (p *painter) SetUniform1f(pState ProgramState, name string, v float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v {
		return
	}
	u.prev[0] = v
	p.ctx.Uniform1f(u.ref, v)
}

func (p *painter) SetUniform2f(pState ProgramState, name string, v0, v1 float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v0 && u.prev[1] == v1 {
		return
	}
	u.prev[0] = v0
	u.prev[1] = v1
	p.ctx.Uniform2f(u.ref, v0, v1)
}

func (p *painter) SetUniform3f(pState ProgramState, name string, v0, v1, v2 float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v0 && u.prev[1] == v1 && u.prev[2] == v2 {
		return
	}
	u.prev[0] = v0
	u.prev[1] = v1
	u.prev[2] = v2
	p.ctx.Uniform3f(u.ref, v0, v1, v2)
}

func (p *painter) SetUniform4f(pState ProgramState, name string, v0, v1, v2, v3 float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v0 && u.prev[1] == v1 && u.prev[2] == v2 && u.prev[3] == v3 {
		return
	}
	u.prev[0] = v0
	u.prev[1] = v1
	u.prev[2] = v2
	u.prev[3] = v3
	p.ctx.Uniform4f(u.ref, v0, v1, v2, v3)
}

func (p *painter) SetUniformMatrix4fv(pState ProgramState, name string, m [16]float32) {
	u := pState.mat4Uniforms[name]
	if u == nil {
		return
	}
	if u.prev == m {
		return
	}
	u.prev = m
	p.ctx.UniformMatrix4fv(u.ref, m)
}

func (p *painter) UpdateVertexArray(pState ProgramState, name string, size, stride, offset int) {
	a := pState.attributes[name]

	p.ctx.VertexAttribPointerWithOffset(a, size, float, false, stride*floatSize, offset*floatSize)
	p.logError()
}

func (p *painter) Clear() {
	if p.shaderInitFailed {
		p.drawInitFailure()
		return
	}
	r, g, b, a := theme.Color(theme.ColorNameBackground).RGBA()
	p.ctx.ClearColor(float32(r)/max16bit, float32(g)/max16bit, float32(b)/max16bit, float32(a)/max16bit)
	p.ctx.Clear(bitColorBuffer | bitDepthBuffer)
	p.logError()
}

// drawInitFailure renders a clear, intentional diagnostic instead of the blank
// or garbled output that program-0 draws produce when shader programs fail to
// build (pre-rebrand tracker #300, follow-up to pre-rebrand tracker #299). It always clears to the theme background
// (glClear needs no shader) and, when the base textured-quad ("simple") program
// survived, centres a "graphics unavailable" message — that program drives both
// image and text drawing, so it is the one most likely to compile when only the
// advanced shaders (ripple/blur/arc/path/scene3d) failed. If even the base
// program failed the message cannot be presented and the clean background is the
// best the GL pipeline can manage (the full software-painter fallback is the
// larger, deferred follow-up).
func (p *painter) drawInitFailure() {
	r, g, b, a := theme.Color(theme.ColorNameBackground).RGBA()
	p.ctx.ClearColor(float32(r)/max16bit, float32(g)/max16bit, float32(b)/max16bit, float32(a)/max16bit)
	p.ctx.Clear(bitColorBuffer | bitDepthBuffer)

	size := p.canvas.Size()
	if p.baseProgramFailed || size.Width <= 0 || size.Height <= 0 {
		p.logError()
		return
	}

	if p.initFailureText == nil {
		t := canvas.NewText("Graphics unavailable — try updating your graphics drivers", theme.Color(theme.ColorNameError))
		t.Alignment = bellui.TextAlignCenter
		t.TextSize = theme.Size(theme.SizeNameText)
		p.initFailureText = t
	}
	p.initFailureText.Resize(size)
	p.drawText(p.initFailureText, geo.Position{}, size)
	p.logError()
}

func (p *painter) Free(obj core.CanvasObject) {
	p.freeTexture(obj)
	p.FreeBoundaryFBO(obj)
}

func (p *painter) Paint(obj core.CanvasObject, pos geo.Position, frame geo.Size) {
	if p.shaderInitFailed {
		// Content programs are unavailable; drawInitFailure (from Clear) already
		// presented the diagnostic. Skip per-object draws so program-0 calls don't
		// paint garbage over it.
		return
	}
	if obj.Visible() {
		p.drawObject(obj, pos, frame)
	}
}

// InitFailed reports whether a shader/program could not be built during Init, so
// the paint loop renders a clear diagnostic (see drawInitFailure) rather than the
// blank/garbled output of program-0 draws. Backends may also query it.
func (p *painter) InitFailed() bool {
	return p.shaderInitFailed
}

func (p *painter) SetFrameBufferScale(scale float32) {
	p.texScale = scale
	p.pixScale = p.canvas.Scale() * p.texScale
}

func (p *painter) SetOutputSize(width, height int) {
	p.ctx.Viewport(0, 0, width, height)
	p.logError()
}

func (p *painter) StartClipping(pos geo.Position, size geo.Size) {
	x := p.textureScale(pos.X)
	y := p.textureScale(p.targetHeight() - pos.Y - size.Height)
	w := p.textureScale(size.Width)
	h := p.textureScale(size.Height)
	p.ctx.Scissor(int32(x), int32(y), int32(w), int32(h))
	p.ctx.Enable(scissorTest)
	p.scissorActive = true
	p.logError()
}

// targetHeight returns the logical height of the bound render target, which
// GL's y-up scissor is flipped against. Inside a boundary FBO that is the
// boundary's height, not the canvas's: the paint walk hands StartClipping
// boundary-local rects there (#559).
func (p *painter) targetHeight() float32 {
	if n := len(p.targetStack); n > 0 && p.targetStack[n-1].logical > 0 {
		return p.targetStack[n-1].logical
	}
	return p.canvas.Size().Height
}

func (p *painter) StopClipping() {
	p.ctx.Disable(scissorTest)
	p.scissorActive = false
	p.logError()
}

// suspendScissor temporarily disables an active scissor clip and returns a
// func that re-enables it (the scissor rect itself persists in GL state).
// Scissor state is global and expressed in window coordinates, so it must be
// suspended while rendering into offscreen FBOs (Scene3D, shadow blur) —
// otherwise the FBO draw is clipped against an unrelated window region.
func (p *painter) suspendScissor() func() {
	if !p.scissorActive {
		return func() {}
	}
	p.ctx.Disable(scissorTest)
	return func() { p.ctx.Enable(scissorTest) }
}

// blurTexture performs a two-pass separable Gaussian blur on a texture using FBOs.
// It creates temporary FBOs, renders the texture through horizontal then vertical
// blur shaders, and returns the blurred texture. The caller must delete the returned
// texture and FBOs when done.
func (p *painter) blurTexture(srcTex Texture, width, height int, radius float32) Texture {
	// Create two FBOs for ping-pong blur
	fboA := p.ctx.CreateFramebuffer()
	texA := p.ctx.CreateTexture()
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, texA)
	p.ctx.TexImage2D(texture2D, 0, width, height, colorFormatRGBA, unsignedByte, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.BindFramebuffer(glFramebuffer, fboA)
	p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, texA, 0)
	p.checkFBOComplete()
	p.logError()

	fboB := p.ctx.CreateFramebuffer()
	texB := p.ctx.CreateTexture()
	p.ctx.BindTexture(texture2D, texB)
	p.ctx.TexImage2D(texture2D, 0, width, height, colorFormatRGBA, unsignedByte, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.BindFramebuffer(glFramebuffer, fboB)
	p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, texB, 0)
	p.checkFBOComplete()
	p.logError()

	// Full-screen quad vertices: position (x,y,z) + texcoord (u,v)
	quad := []float32{
		-1, -1, 0, 0, 0,
		-1, 1, 0, 0, 1,
		1, -1, 0, 1, 0,
		1, 1, 0, 1, 1,
	}
	resW, resH := float32(width), float32(height)

	// Pass 1: Horizontal blur (src → fboA)
	p.ctx.BindFramebuffer(glFramebuffer, fboA)
	p.ctx.Viewport(0, 0, width, height)
	p.ctx.ClearColor(0, 0, 0, 0)
	p.ctx.Clear(bitColorBuffer)

	p.ctx.UseProgram(p.blurHProgram.ref)
	p.updateBuffer(p.blurHProgram.buff, quad)
	p.UpdateVertexArray(p.blurHProgram, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.blurHProgram, "vertTexCoord", 2, 5, 3)
	p.SetUniform2f(p.blurHProgram, "resolution", resW, resH)
	p.SetUniform1f(p.blurHProgram, "blur_radius", radius)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, srcTex)
	p.ctx.BlendFunc(one, oneMinusSrcAlpha)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()

	// Pass 2: Vertical blur (fboA/texA → fboB)
	p.ctx.BindFramebuffer(glFramebuffer, fboB)
	p.ctx.ClearColor(0, 0, 0, 0)
	p.ctx.Clear(bitColorBuffer)

	p.ctx.UseProgram(p.blurVProgram.ref)
	p.updateBuffer(p.blurVProgram.buff, quad)
	p.UpdateVertexArray(p.blurVProgram, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.blurVProgram, "vertTexCoord", 2, 5, 3)
	p.SetUniform2f(p.blurVProgram, "resolution", resW, resH)
	p.SetUniform1f(p.blurVProgram, "blur_radius", radius)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, texA)
	p.ctx.BlendFunc(one, oneMinusSrcAlpha)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()

	// Restore the active render target (canvas/boundary FBO or screen).
	// The caller restores its own viewport if the stack is empty.
	p.rebindRenderTarget(0, 0)

	// Cleanup intermediate resources
	p.ctx.DeleteFramebuffer(fboA)
	p.ctx.DeleteTexture(texA)
	p.ctx.DeleteFramebuffer(fboB)

	// Return the blurred texture (caller owns texB)
	return texB
}

// StartRoundedClipping enables stencil-based rounded-corner clipping.
func (p *painter) StartRoundedClipping(pos geo.Position, size geo.Size, cornerRadius float32) {
	p.ctx.Enable(stencilTest)
	p.ctx.ClearStencil(0)
	p.ctx.Clear(bitStencilBuffer)
	p.ctx.StencilFunc(glAlways, 1, 0xFF)
	p.ctx.StencilOp(glKeep, glKeep, glReplace)
	p.ctx.StencilMask(0xFF)
	p.ctx.ColorMask(false, false, false, false)

	// Draw a rounded rectangle to the stencil buffer
	frame := p.canvas.Size()
	r := canvas.NewRectangle(color.Transparent)
	r.Resize(size)
	r.Move(pos)
	r.CornerRadius = cornerRadius
	r.FillColor = color.White // doesn't matter, color writes are off
	p.drawRectangle(r, pos, frame)

	// Now set stencil to only allow drawing where stencil == 1
	p.ctx.ColorMask(true, true, true, true)
	p.ctx.StencilFunc(glEqual, 1, 0xFF)
	p.ctx.StencilMask(0x00) // don't write to stencil anymore
	p.logError()
}

// StopRoundedClipping disables stencil-based clipping.
func (p *painter) StopRoundedClipping() {
	p.ctx.Disable(stencilTest)
	p.logError()
}

// StartPathClipping enables stencil-based clipping to an arbitrary path shape.
// Content drawn after this call will only appear inside the path's filled region.
func (p *painter) StartPathClipping(path *canvas.Path, pos geo.Position, size geo.Size) {
	if path == nil {
		return
	}

	elements := path.Elements()
	if len(elements) == 0 {
		return
	}

	subpaths := paint.FlattenPath(elements, p.pixScale)
	if len(subpaths) == 0 {
		return
	}

	fanTris := paint.FanTriangulate(subpaths)
	if len(fanTris) < 6 {
		return
	}

	frame := p.canvas.Size()
	program := p.pathProgram
	p.ctx.UseProgram(program.ref)

	// Render path fill to stencil buffer using winding rule.
	p.ctx.Enable(stencilTest)
	p.ctx.ClearStencil(0)
	p.ctx.Clear(bitStencilBuffer)
	p.ctx.StencilMask(0xFF)
	p.ctx.ColorMask(false, false, false, false)
	p.ctx.StencilFunc(glAlways, 0, 0xFF)
	p.ctx.StencilOpSeparate(glFront, glKeep, glKeep, glIncrWrap)
	p.ctx.StencilOpSeparate(glBack, glKeep, glKeep, glDecrWrap)

	ndcVerts := p.pathVertsToNDC(fanTris, pos, frame)
	p.ctx.BindBuffer(arrayBuffer, program.buff)
	p.ctx.BufferData(arrayBuffer, ndcVerts, staticDraw)
	p.UpdateVertexArray(program, "vert", 2, 2, 0)
	p.ctx.VertexAttribPointerWithOffset(program.attributes["alpha"], 1, float, false, 0, 0)
	p.SetUniform4f(program, "fill_color", 0, 0, 0, 0)
	p.ctx.DrawArrays(triangles, 0, len(ndcVerts)/2)
	p.logError()

	// Set stencil to only allow drawing where path is filled.
	p.ctx.ColorMask(true, true, true, true)
	p.ctx.StencilFunc(glNotEqual, 0, 0xFF)
	p.ctx.StencilMask(0x00)
	p.logError()
}

// StopPathClipping disables stencil-based path clipping.
func (p *painter) StopPathClipping() {
	p.ctx.Disable(stencilTest)
	p.logError()
}

func (p *painter) SetGroupOpacity(opacity float32) {
	p.groupOpacity = opacity
}

func (p *painter) ClearGroupOpacity() {
	p.groupOpacity = 1
}

func (p *painter) SetTransform(m geo.Matrix) {
	p.currentTransform = m
}

func (p *painter) SetUniformMatrix3fv(pState ProgramState, name string, m [9]float32) {
	u := pState.matrixUniforms[name]
	if u == nil {
		return
	}
	if u.prev == m {
		return
	}
	u.prev = m
	p.ctx.UniformMatrix3fv(u.ref, m)
}

// effectiveAlpha returns the alpha value multiplied by group opacity if active.
func (p *painter) effectiveAlpha(alpha float32) float32 {
	if p.groupOpacity > 0 && p.groupOpacity < 1 {
		return alpha * p.groupOpacity
	}
	return alpha
}

func (p *painter) compileShader(source string, shaderType uint32) (Shader, error) {
	shader := p.ctx.CreateShader(shaderType)

	p.ctx.ShaderSource(shader, source)
	p.logError()
	p.ctx.CompileShader(shader)
	p.logError()

	info := p.ctx.GetShaderInfoLog(shader)
	if p.ctx.GetShaderi(shader, compileStatus) == glFalse {
		return noShader, fmt.Errorf("failed to compile OpenGL shader:\n%s\n>>> SHADER SOURCE\n%s\n<<< SHADER SOURCE", info, source)
	}

	// The info is probably a null terminated string.
	// An empty info has been seen as "\x00" or "\x00\x00".
	if len(info) > 0 && info != "\x00" && info != "\x00\x00" {
		fmt.Printf("OpenGL shader compilation output:\n%s\n>>> SHADER SOURCE\n%s\n<<< SHADER SOURCE\n", info, source)
	}

	return shader, nil
}

func (p *painter) createProgram(shaderFilename string) Program {
	// Why a switch over a filename?
	// Because this allows for a minimal change, once we reach Go 1.16 and use go:embed instead of
	// bellui bundle.
	vertexSrc, fragmentSrc := shaderSourceNamed(shaderFilename)
	if vertexSrc == nil {
		return p.shaderInitFailure("shader not found", shaderFilename, nil)
	}

	vertShader, err := p.compileShader(string(vertexSrc), vertexShader)
	if err != nil {
		return p.shaderInitFailure("vertex shader compile failed", shaderFilename, err)
	}
	fragShader, err := p.compileShader(string(fragmentSrc), fragmentShader)
	if err != nil {
		return p.shaderInitFailure("fragment shader compile failed", shaderFilename, err)
	}

	prog := p.ctx.CreateProgram()
	p.ctx.AttachShader(prog, vertShader)
	p.ctx.AttachShader(prog, fragShader)
	p.ctx.LinkProgram(prog)

	info := p.ctx.GetProgramInfoLog(prog)
	if p.ctx.GetProgrami(prog, linkStatus) == glFalse {
		return p.shaderInitFailure("program link failed", shaderFilename,
			fmt.Errorf("link log:\n%s", info))
	}

	// The info is probably a null terminated string.
	// An empty info has been seen as "\x00" or "\x00\x00".
	if len(info) > 0 && info != "\x00" && info != "\x00\x00" {
		fmt.Printf("OpenGL program linking output:\n%s\n", info)
	}

	if glErr := p.ctx.GetError(); glErr != 0 {
		return p.shaderInitFailure("program link reported a GL error", shaderFilename,
			fmt.Errorf("GL error code: %x", glErr))
	}

	p.ctx.UseProgram(prog)

	return prog
}

// shaderInitFailure records that a shader/program could not be built (e.g. on an
// old GLES stack or a driver with a compiler quirk) and returns the zero Program
// so init continues. The whole app must not panic for a single broken shader;
// rendering degrades to whatever GL produces for program 0. The first failure is
// logged with the offending shader name so the cause is diagnosable; subsequent
// failures in the same init pass are suppressed to avoid a log flood across all
// of the programs built in Init.
func (p *painter) shaderInitFailure(reason, shaderFilename string, err error) Program {
	// The base textured-quad program is named "simple" ("simple_es" on GLES/wasm)
	// and drives all image/text drawing. If it fails the diagnostic message in
	// drawInitFailure cannot be presented either, so record that separately.
	if strings.HasPrefix(shaderFilename, "simple") {
		p.baseProgramFailed = true
	}
	if !p.shaderInitFailed {
		p.shaderInitFailed = true
		bellui.LogError(fmt.Sprintf("GL shader init failed (%s: %s); rendering may be degraded",
			shaderFilename, reason), err)
	}
	var zero Program
	return zero
}

func (p *painter) logError() {
	logGLError(p.ctx.GetError)
}

// checkFBOComplete verifies that the currently bound FBO is complete.
// Logs a warning if the FBO status indicates an error (missing attachment,
// unsupported format, insufficient VRAM, etc.).
func (p *painter) checkFBOComplete() {
	status := p.ctx.CheckFramebufferStatus(glFramebuffer)
	if status != framebufferComplete && status != 0 {
		log.Printf("GL FBO incomplete: status 0x%x", status)
		_, file, line, ok := runtime.Caller(1)
		if ok {
			log.Printf("  At: %s:%d", file, line)
		}
	}
}
