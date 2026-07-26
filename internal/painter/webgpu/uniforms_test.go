package webgpu

import (
	"reflect"
	"testing"
	"unsafe"
)

// fieldOffsets describes the expected std140 byte offset of each named field of
// a uniform struct, matching the offsets documented in the corresponding .wgsl
// module. The painter writes these structs straight into a GPU buffer, so any
// drift between a struct and its shader would silently corrupt rendering; this
// test is the guard.
func TestUniformLayouts(t *testing.T) {
	cases := []struct {
		name    string
		typ     reflect.Type
		size    uintptr
		offsets map[string]uintptr
	}{
		{
			name: "SimpleUniforms", typ: reflect.TypeOf(SimpleUniforms{}), size: 32,
			offsets: map[string]uintptr{"Alpha": 0, "CornerRadius": 4, "Size": 8, "Inset": 16},
		},
		{
			name: "LineUniforms", typ: reflect.TypeOf(LineUniforms{}), size: 32,
			offsets: map[string]uintptr{"Color": 0, "LineWidth": 16, "Feather": 20},
		},
		{
			name: "PathUniforms", typ: reflect.TypeOf(PathUniforms{}), size: 16,
			offsets: map[string]uintptr{"FillColor": 0},
		},
		{
			name: "RectangleUniforms", typ: reflect.TypeOf(RectangleUniforms{}), size: 112,
			offsets: map[string]uintptr{
				"FrameSize": 0, "StrokeWidth": 8, "RectCoords": 16,
				"FillColor": 32, "StrokeColor": 48, "InvTransform": 64,
			},
		},
		{
			name: "RoundRectangleUniforms", typ: reflect.TypeOf(RoundRectangleUniforms{}), size: 144,
			offsets: map[string]uintptr{
				"FrameSize": 0, "StrokeWidthHalf": 8, "EdgeSoftness": 12, "RectCoords": 16,
				"RectSizeHalf": 32, "Radius": 48, "FillColor": 64, "StrokeColor": 80, "InvTransform": 96,
			},
		},
		{
			name: "PolygonUniforms", typ: reflect.TypeOf(PolygonUniforms{}), size: 128,
			offsets: map[string]uintptr{
				"FrameSize": 0, "EdgeSoftness": 8, "OuterRadius": 12, "RectCoords": 16,
				"Angle": 32, "Sides": 36, "CornerRadius": 40, "StrokeWidth": 44,
				"FillColor": 48, "StrokeColor": 64, "InvTransform": 80,
			},
		},
		{
			name: "ArcUniforms", typ: reflect.TypeOf(ArcUniforms{}), size: 144,
			offsets: map[string]uintptr{
				"FrameSize": 0, "EdgeSoftness": 8, "InnerRadius": 12, "RectCoords": 16,
				"OuterRadius": 32, "StartAngle": 36, "EndAngle": 40, "CornerRadius": 44,
				"FillColor": 48, "StrokeWidth": 64, "StrokeColor": 80, "InvTransform": 96,
			},
		},
		{
			name: "RippleUniforms", typ: reflect.TypeOf(RippleUniforms{}), size: 128,
			offsets: map[string]uintptr{
				"FrameSize": 0, "Radius": 8, "EdgeSoftness": 12, "RectCoords": 16,
				"RectSizeHalf": 32, "RippleCenter": 40, "RippleRadius": 48,
				"RippleColor": 64, "InvTransform": 80,
			},
		},
		{
			name: "BlurUniforms", typ: reflect.TypeOf(BlurUniforms{}), size: 16,
			offsets: map[string]uintptr{"Resolution": 0, "BlurRadius": 8},
		},
		{
			name: "Scene3DUniforms", typ: reflect.TypeOf(Scene3DUniforms{}), size: 96,
			offsets: map[string]uintptr{"MVP": 0, "LightDir": 64, "LightColor": 80, "Ambient": 92},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.typ.Size(); got != tc.size {
				t.Errorf("%s size = %d, want %d", tc.name, got, tc.size)
			}
			// A uniform buffer's size must be a multiple of 16 (std140).
			if tc.size%16 != 0 {
				t.Errorf("%s size %d is not a multiple of 16", tc.name, tc.size)
			}
			for field, want := range tc.offsets {
				f, ok := tc.typ.FieldByName(field)
				if !ok {
					t.Errorf("%s: field %q not found", tc.name, field)
					continue
				}
				if f.Offset != want {
					t.Errorf("%s.%s offset = %d, want %d", tc.name, field, f.Offset, want)
				}
			}
		})
	}
}

func TestMat3ToStd140(t *testing.T) {
	// column-major 3x3 -> three vec4-padded columns
	in := [9]float32{1, 2, 3, 4, 5, 6, 7, 8, 9}
	got := Mat3ToStd140(in)
	want := [12]float32{1, 2, 3, 0, 4, 5, 6, 0, 7, 8, 9, 0}
	if got != want {
		t.Errorf("Mat3ToStd140 = %v, want %v", got, want)
	}
	if unsafe.Sizeof(got) != 48 {
		t.Errorf("std140 mat3x3 size = %d, want 48", unsafe.Sizeof(got))
	}
}
