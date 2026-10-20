// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || linux || openbsd || freebsd || windows

package gl

import "unsafe"

type call struct {
	args     fnargs
	parg     unsafe.Pointer
	blocking bool
}

type fnargs struct {
	fn glfn

	a0 uintptr
	a1 uintptr
	a2 uintptr
	a3 uintptr
	a4 uintptr
	a5 uintptr
	a6 uintptr
	a7 uintptr
	a8 uintptr
	a9 uintptr
}

type glfn int

const (
	glfnUNDEFINED glfn = iota
	glfnActiveTexture
	glfnAttachShader
	glfnBindBuffer
	glfnBindTexture
	glfnBindVertexArray
	glfnBlendColor
	glfnBlendFunc
	glfnBufferData
	glfnBufferSubData
	glfnClear
	glfnClearColor
	glfnCompileShader
	glfnCreateProgram
	glfnCreateShader
	glfnDeleteBuffer
	glfnDeleteTexture
	glfnDisable
	glfnDrawArrays
	glfnEnable
	glfnEnableVertexAttribArray
	glfnFlush
	glfnGenBuffer
	glfnGenTexture
	glfnGenVertexArray
	glfnGetAttribLocation
	glfnGetError
	glfnGetProgramInfoLog
	glfnGetProgramiv
	glfnGetShaderInfoLog
	glfnGetShaderSource
	glfnGetShaderiv
	glfnGetTexParameteriv
	glfnGetUniformLocation
	glfnLinkProgram
	glfnReadPixels
	glfnScissor
	glfnShaderSource
	glfnTexImage2D
	glfnTexSubImage2D
	glfnTexParameteri
	glfnUniform1f
	glfnUniform2f
	glfnUniform4f
	glfnUniform4fv
	glfnUniformMatrix3fv
	glfnUseProgram
	glfnVertexAttribPointer
	glfnViewport

	// Framebuffer methods.
	glfnCreateFramebuffer
	glfnBindFramebuffer
	glfnFramebufferTexture2D
	glfnCheckFramebufferStatus
	glfnDeleteFramebuffer

	// Additional methods.
	glfnCopyTexSubImage2D
	glfnColorMask
	glfnClearStencil
	glfnStencilFunc
	glfnStencilOp
	glfnStencilOpSeparate
	glfnStencilMask

	// Scene3D uniforms.
	glfnUniform3f
	glfnUniformMatrix4fv

	// RepaintBoundary targets blend their alpha apart from their color (#570).
	glfnBlendFuncSeparate
)

func goString(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	panic("buf is not NUL-terminated")
}

func glBoolean(b bool) uintptr {
	if b {
		return True
	}
	return False
}
