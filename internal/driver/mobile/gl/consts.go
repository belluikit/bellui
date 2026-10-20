// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || linux || openbsd || freebsd || windows

package gl

/*
Partially generated from the Khronos OpenGL API specification in XML
format, which is covered by the license:

	Copyright (c) 2013-2014 The Khronos Group Inc.

	Permission is hereby granted, free of charge, to any person obtaining a
	copy of this software and/or associated documentation files (the
	"Materials"), to deal in the Materials without restriction, including
	without limitation the rights to use, copy, modify, merge, publish,
	distribute, sublicense, and/or sell copies of the Materials, and to
	permit persons to whom the Materials are furnished to do so, subject to
	the following conditions:

	The above copyright notice and this permission notice shall be included
	in all copies or substantial portions of the Materials.

	THE MATERIALS ARE PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
	EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
	MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
	IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
	CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
	MATERIALS OR THE USE OR OTHER DEALINGS IN THE MATERIALS.

*/

// Contains Khronos OpenGL API specification constants.
const (
	False            = 0
	True             = 1
	One              = 1
	Triangles        = 0x0004
	TriangleStrip    = 0x0005
	SrcAlpha         = 0x0302
	OneMinusSrcAlpha = 0x0303
	Front            = 0x0404
	DepthTest        = 0x0B71
	Blend            = 0x0BE2
	ScissorTest      = 0x0C11
	Texture2D        = 0x0DE1

	UnsignedByte = 0x1401
	Float        = 0x1406
	RED          = 0x1903
	RGBA         = 0x1908

	Nearest          = 0x2600
	Linear           = 0x2601
	TextureMagFilter = 0x2800
	TextureMinFilter = 0x2801
	TextureWrapS     = 0x2802
	TextureWrapT     = 0x2803

	ConstantAlpha            = 0x8003
	OneMinusConstantAlpha    = 0x8004
	ClampToEdge              = 0x812F
	Texture0                 = 0x84C0
	StaticDraw               = 0x88E4
	DynamicDraw              = 0x88E8
	FragmentShader           = 0x8B30
	VertexShader             = 0x8B31
	AttachedShaders          = 0x8B85
	ActiveUniformMaxLength   = 0x8B87
	ActiveAttributeMaxLength = 0x8B8A
	ArrayBuffer              = 0x8892
	CompileStatus            = 0x8B81
	LinkStatus               = 0x8B82
	InfoLogLength            = 0x8B84
	ShaderSourceLength       = 0x8B88

	DepthBufferBit = 0x00000100
	ColorBufferBit = 0x00004000

	// Framebuffer object constants. FramebufferTarget is GL_FRAMEBUFFER
	// (named to avoid colliding with the Framebuffer type).
	FramebufferTarget   = 0x8D40
	ColorAttachment0    = 0x8CE0
	FramebufferComplete = 0x8CD5

	// Depth constants
	DepthAttachment = 0x8D00
	DepthComponent  = 0x1902
	UnsignedShort   = 0x1403

	// Stencil constants
	StencilTest      = 0x0B90
	StencilBufferBit = 0x00000400
	Always           = 0x0207
	Equal            = 0x0202
	NotEqual         = 0x0205
	Keep             = 0x1E00
	Replace          = 0x1E01
	IncrWrap         = 0x8507
	DecrWrap         = 0x8508
	Invert           = 0x150A
	Back             = 0x0405
)
