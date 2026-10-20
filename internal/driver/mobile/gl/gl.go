// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || linux || openbsd || freebsd || windows

package gl

// TODO(crawshaw): should functions on specific types become methods? E.g.
//                 func (t Texture) Bind(target Enum)
//                 this seems natural in Go, but moves us slightly
//                 further away from the underlying OpenGL spec.

import (
	"math"
	"unsafe"
)

func (ctx *context) ActiveTexture(texture Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnActiveTexture,
			a0: texture.c(),
		},
	})
}

func (ctx *context) AttachShader(p Program, s Shader) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnAttachShader,
			a0: p.c(),
			a1: s.c(),
		},
	})
}

func (ctx *context) BindBuffer(target Enum, b Buffer) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBindBuffer,
			a0: target.c(),
			a1: b.c(),
		},
	})
}

func (ctx *context) BindTexture(target Enum, t Texture) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBindTexture,
			a0: target.c(),
			a1: t.c(),
		},
	})
}

func (ctx *context) BindVertexArray(va VertexArray) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBindVertexArray,
			a0: va.c(),
		},
	})
}

func (ctx *context) BlendColor(red, green, blue, alpha float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBlendColor,
			a0: uintptr(math.Float32bits(red)),
			a1: uintptr(math.Float32bits(green)),
			a2: uintptr(math.Float32bits(blue)),
			a3: uintptr(math.Float32bits(alpha)),
		},
	})
}

func (ctx *context) BlendFunc(sfactor, dfactor Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBlendFunc,
			a0: sfactor.c(),
			a1: dfactor.c(),
		},
	})
}

func (ctx *context) BlendFuncSeparate(sfactorRGB, dfactorRGB, sfactorAlpha, dfactorAlpha Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBlendFuncSeparate,
			a0: sfactorRGB.c(),
			a1: dfactorRGB.c(),
			a2: sfactorAlpha.c(),
			a3: dfactorAlpha.c(),
		},
	})
}

func (ctx *context) BufferData(target Enum, src []byte, usage Enum) {
	parg := unsafe.Pointer(nil)
	if len(src) > 0 {
		parg = unsafe.Pointer(&src[0])
	}
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBufferData,
			a0: target.c(),
			a1: uintptr(len(src)),
			a2: usage.c(),
		},
		parg: parg,
	})
}

func (ctx *context) BufferSubData(target Enum, src []byte) {
	parg := unsafe.Pointer(nil)
	if len(src) > 0 {
		parg = unsafe.Pointer(&src[0])
	}
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBufferSubData,
			a0: target.c(),
			a1: 0,
			a2: uintptr(len(src)),
		},
		parg: parg,
	})
}

func (ctx *context) Clear(mask Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnClear,
			a0: uintptr(mask),
		},
	})
}

func (ctx *context) ClearColor(red, green, blue, alpha float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnClearColor,
			a0: uintptr(math.Float32bits(red)),
			a1: uintptr(math.Float32bits(green)),
			a2: uintptr(math.Float32bits(blue)),
			a3: uintptr(math.Float32bits(alpha)),
		},
	})
}

func (ctx *context) CompileShader(s Shader) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnCompileShader,
			a0: s.c(),
		},
	})
}

func (ctx *context) CreateBuffer() Buffer {
	return Buffer{Value: uint32(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGenBuffer,
		},
		blocking: true,
	}))}
}

func (ctx *context) CreateProgram() Program {
	return Program{
		Init: true,
		Value: uint32(ctx.enqueue(
			call{
				args: fnargs{
					fn: glfnCreateProgram,
				},
				blocking: true,
			},
		)),
	}
}

func (ctx *context) CreateShader(ty Enum) Shader {
	return Shader{Value: uint32(ctx.enqueue(call{
		args: fnargs{
			fn: glfnCreateShader,
			a0: uintptr(ty),
		},
		blocking: true,
	}))}
}

func (ctx *context) CreateTexture() Texture {
	return Texture{Value: uint32(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGenTexture,
		},
		blocking: true,
	}))}
}

func (ctx *context) CreateVertexArray() VertexArray {
	return VertexArray{Value: uint32(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGenVertexArray,
		},
		blocking: true,
	}))}
}

func (ctx *context) DeleteBuffer(v Buffer) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnDeleteBuffer,
			a0: v.c(),
		},
	})
}

func (ctx *context) DeleteTexture(v Texture) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnDeleteTexture,
			a0: v.c(),
		},
	})
}

func (ctx *context) Disable(cap Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnDisable,
			a0: cap.c(),
		},
	})
}

func (ctx *context) DrawArrays(mode Enum, first, count int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnDrawArrays,
			a0: mode.c(),
			a1: uintptr(first),
			a2: uintptr(count),
		},
	})
}

func (ctx *context) Enable(cap Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnEnable,
			a0: cap.c(),
		},
	})
}

func (ctx *context) EnableVertexAttribArray(a Attrib) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnEnableVertexAttribArray,
			a0: a.c(),
		},
	})
}

func (ctx *context) Flush() {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnFlush,
		},
		blocking: true,
	})
}

func (ctx *context) GetAttribLocation(p Program, name string) Attrib {
	s, free := ctx.cString(name)
	defer free()
	return Attrib{Value: uint(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetAttribLocation,
			a0: p.c(),
			a1: s,
		},
		blocking: true,
	}))}
}

func (ctx *context) GetError() Enum {
	return Enum(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetError,
		},
		blocking: true,
	}))
}

func (ctx *context) GetProgrami(p Program, pname Enum) int {
	return int(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetProgramiv,
			a0: p.c(),
			a1: pname.c(),
		},
		blocking: true,
	}))
}

func (ctx *context) GetProgramInfoLog(p Program) string {
	infoLen := ctx.GetProgrami(p, InfoLogLength)
	if infoLen == 0 {
		return ""
	}
	buf := make([]byte, infoLen)

	ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetProgramInfoLog,
			a0: p.c(),
			a1: uintptr(infoLen),
		},
		parg:     unsafe.Pointer(&buf[0]),
		blocking: true,
	})

	return goString(buf)
}

func (ctx *context) GetShaderi(s Shader, pname Enum) int {
	return int(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetShaderiv,
			a0: s.c(),
			a1: pname.c(),
		},
		blocking: true,
	}))
}

func (ctx *context) GetShaderInfoLog(s Shader) string {
	infoLen := ctx.GetShaderi(s, InfoLogLength)
	if infoLen == 0 {
		return ""
	}
	buf := make([]byte, infoLen)

	ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetShaderInfoLog,
			a0: s.c(),
			a1: uintptr(infoLen),
		},
		parg:     unsafe.Pointer(&buf[0]),
		blocking: true,
	})

	return goString(buf)
}

func (ctx *context) GetShaderSource(s Shader) string {
	sourceLen := ctx.GetShaderi(s, ShaderSourceLength)
	if sourceLen == 0 {
		return ""
	}
	buf := make([]byte, sourceLen)

	ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetShaderSource,
			a0: s.c(),
			a1: uintptr(sourceLen),
		},
		parg:     unsafe.Pointer(&buf[0]),
		blocking: true,
	})

	return goString(buf)
}

func (ctx *context) GetTexParameteriv(dst []int32, target, pname Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetTexParameteriv,
			a0: target.c(),
			a1: pname.c(),
		},
		blocking: true,
	})
}

func (ctx *context) GetUniformLocation(p Program, name string) Uniform {
	s, free := ctx.cString(name)
	defer free()
	return Uniform{Value: int32(ctx.enqueue(call{
		args: fnargs{
			fn: glfnGetUniformLocation,
			a0: p.c(),
			a1: s,
		},
		blocking: true,
	}))}
}

func (ctx *context) LinkProgram(p Program) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnLinkProgram,
			a0: p.c(),
		},
	})
}

func (ctx *context) ReadPixels(dst []byte, x, y, width, height int, format, ty Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnReadPixels,
			// TODO(crawshaw): support PIXEL_PACK_BUFFER in GLES3, uses offset.
			a0: uintptr(x),
			a1: uintptr(y),
			a2: uintptr(width),
			a3: uintptr(height),
			a4: format.c(),
			a5: ty.c(),
		},
		parg:     unsafe.Pointer(&dst[0]),
		blocking: true,
	})
}

func (ctx *context) Scissor(x, y, width, height int32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnScissor,
			a0: uintptr(x),
			a1: uintptr(y),
			a2: uintptr(width),
			a3: uintptr(height),
		},
	})
}

func (ctx *context) ShaderSource(s Shader, src string) {
	strp, free := ctx.cStringPtr(src)
	defer free()
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnShaderSource,
			a0: s.c(),
			a1: 1,
			a2: strp,
		},
		blocking: true,
	})
}

func (ctx *context) TexImage2D(target Enum, level int, internalFormat int, width, height int, format Enum, ty Enum, data []byte) {
	// It is common to pass TexImage2D a nil data, indicating that a
	// bound GL buffer is being used as the source. In that case, it
	// is not necessary to block.
	parg := unsafe.Pointer(nil)
	if len(data) > 0 {
		parg = unsafe.Pointer(&data[0])
	}

	ctx.enqueue(call{
		args: fnargs{
			fn: glfnTexImage2D,
			// TODO(crawshaw): GLES3 offset for PIXEL_UNPACK_BUFFER and PIXEL_PACK_BUFFER.
			a0: target.c(),
			a1: uintptr(level),
			a2: uintptr(internalFormat),
			a3: uintptr(width),
			a4: uintptr(height),
			a5: format.c(),
			a6: ty.c(),
		},
		parg:     parg,
		blocking: parg != nil,
	})
}

func (ctx *context) TexSubImage2D(target Enum, level, xoffset, yoffset, width, height int, format Enum, ty Enum, data []byte) {
	parg := unsafe.Pointer(nil)
	if len(data) > 0 {
		parg = unsafe.Pointer(&data[0])
	}

	ctx.enqueue(call{
		args: fnargs{
			fn: glfnTexSubImage2D,
			a0: target.c(),
			a1: uintptr(level),
			a2: uintptr(xoffset),
			a3: uintptr(yoffset),
			a4: uintptr(width),
			a5: uintptr(height),
			a6: format.c(),
			a7: ty.c(),
		},
		parg:     parg,
		blocking: parg != nil,
	})
}

func (ctx *context) TexParameteri(target, pname Enum, param int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnTexParameteri,
			a0: target.c(),
			a1: pname.c(),
			a2: uintptr(param),
		},
	})
}

func (ctx *context) Uniform1f(dst Uniform, v float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniform1f,
			a0: dst.c(),
			a1: uintptr(math.Float32bits(v)),
		},
	})
}

func (ctx *context) Uniform2f(dst Uniform, v0, v1 float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniform2f,
			a0: dst.c(),
			a1: uintptr(math.Float32bits(v0)),
			a2: uintptr(math.Float32bits(v1)),
		},
	})
}

func (ctx *context) Uniform3f(dst Uniform, v0, v1, v2 float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniform3f,
			a0: dst.c(),
			a1: uintptr(math.Float32bits(v0)),
			a2: uintptr(math.Float32bits(v1)),
			a3: uintptr(math.Float32bits(v2)),
		},
	})
}

func (ctx *context) Uniform4f(dst Uniform, v0, v1, v2, v3 float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniform4f,
			a0: dst.c(),
			a1: uintptr(math.Float32bits(v0)),
			a2: uintptr(math.Float32bits(v1)),
			a3: uintptr(math.Float32bits(v2)),
			a4: uintptr(math.Float32bits(v3)),
		},
	})
}

func (ctx *context) Uniform4fv(dst Uniform, src []float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniform4fv,
			a0: dst.c(),
			a1: uintptr(len(src) / 4),
		},
		parg: unsafe.Pointer(&src[0]),
	})
}

func (ctx *context) UniformMatrix3fv(dst Uniform, src []float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniformMatrix3fv,
			a0: dst.c(),
			a1: 1, // count
			a2: 0, // transpose = false
		},
		parg: unsafe.Pointer(&src[0]),
	})
}

func (ctx *context) UniformMatrix4fv(dst Uniform, src []float32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUniformMatrix4fv,
			a0: dst.c(),
			a1: 1, // count
			a2: 0, // transpose = false
		},
		parg: unsafe.Pointer(&src[0]),
	})
}

func (ctx *context) UseProgram(p Program) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnUseProgram,
			a0: p.c(),
		},
	})
}

func (ctx *context) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnVertexAttribPointer,
			a0: dst.c(),
			a1: uintptr(size),
			a2: ty.c(),
			a3: glBoolean(normalized),
			a4: uintptr(stride),
			a5: uintptr(offset),
		},
	})
}

func (ctx *context) Viewport(x, y, width, height int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnViewport,
			a0: uintptr(x),
			a1: uintptr(y),
			a2: uintptr(width),
			a3: uintptr(height),
		},
	})
}

// --- Framebuffer methods ---

func (ctx *context) CreateFramebuffer() Framebuffer {
	return Framebuffer{Value: uint32(ctx.enqueue(call{
		args: fnargs{
			fn: glfnCreateFramebuffer,
		},
		blocking: true,
	}))}
}

func (ctx *context) BindFramebuffer(target Enum, fb Framebuffer) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnBindFramebuffer,
			a0: target.c(),
			a1: uintptr(fb.Value),
		},
	})
}

func (ctx *context) FramebufferTexture2D(target, attachment, textarget Enum, texture Texture, level int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnFramebufferTexture2D,
			a0: target.c(),
			a1: attachment.c(),
			a2: textarget.c(),
			a3: texture.c(),
			a4: uintptr(level),
		},
	})
}

func (ctx *context) CheckFramebufferStatus(target Enum) Enum {
	return Enum(ctx.enqueue(call{
		args: fnargs{
			fn: glfnCheckFramebufferStatus,
			a0: target.c(),
		},
		blocking: true,
	}))
}

func (ctx *context) DeleteFramebuffer(fb Framebuffer) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnDeleteFramebuffer,
			a0: uintptr(fb.Value),
		},
	})
}

// --- Additional methods ---

func (ctx *context) CopyTexSubImage2D(target Enum, level, xoffset, yoffset, x, y, width, height int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnCopyTexSubImage2D,
			a0: target.c(),
			a1: uintptr(level),
			a2: uintptr(xoffset),
			a3: uintptr(yoffset),
			a4: uintptr(x),
			a5: uintptr(y),
			a6: uintptr(width),
			a7: uintptr(height),
		},
	})
}

func (ctx *context) ColorMask(r, g, b, a bool) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnColorMask,
			a0: glBoolean(r),
			a1: glBoolean(g),
			a2: glBoolean(b),
			a3: glBoolean(a),
		},
	})
}

func (ctx *context) ClearStencil(s int) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnClearStencil,
			a0: uintptr(s),
		},
	})
}

func (ctx *context) StencilFunc(fn Enum, ref int, mask uint32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnStencilFunc,
			a0: fn.c(),
			a1: uintptr(ref),
			a2: uintptr(mask),
		},
	})
}

func (ctx *context) StencilOp(sfail, dpfail, dppass Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnStencilOp,
			a0: sfail.c(),
			a1: dpfail.c(),
			a2: dppass.c(),
		},
	})
}

func (ctx *context) StencilOpSeparate(face, sfail, dpfail, dppass Enum) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnStencilOpSeparate,
			a0: face.c(),
			a1: sfail.c(),
			a2: dpfail.c(),
			a3: dppass.c(),
		},
	})
}

func (ctx *context) StencilMask(mask uint32) {
	ctx.enqueue(call{
		args: fnargs{
			fn: glfnStencilMask,
			a0: uintptr(mask),
		},
	})
}
