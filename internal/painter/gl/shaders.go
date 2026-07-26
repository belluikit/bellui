//go:build (gl || (!darwin && !linux && !windows && !wasm)) && ((!gles && !arm && !arm64 && !android && !ios && !mobile && !wasm) || (darwin && !mobile && !ios && !wasm))

package gl

import _ "embed"

var (
	//go:embed shaders/line.frag
	shaderLineFrag []byte

	//go:embed shaders/line.vert
	shaderLineVert []byte

	//go:embed shaders/rectangle.frag
	shaderRectangleFrag []byte

	//go:embed shaders/rectangle.vert
	shaderRectangleVert []byte

	//go:embed shaders/round_rectangle.frag
	shaderRoundrectangleFrag []byte

	//go:embed shaders/simple.frag
	shaderSimpleFrag []byte

	//go:embed shaders/simple.vert
	shaderSimpleVert []byte

	//go:embed shaders/polygon.frag
	shaderPolygonFrag []byte

	//go:embed shaders/arc.frag
	shaderArcFrag []byte

	//go:embed shaders/ripple.frag
	shaderRippleFrag []byte

	//go:embed shaders/blur.vert
	shaderBlurVert []byte

	//go:embed shaders/blur_h.frag
	shaderBlurHFrag []byte

	//go:embed shaders/blur_v.frag
	shaderBlurVFrag []byte

	//go:embed shaders/path.vert
	shaderPathVert []byte

	//go:embed shaders/path.frag
	shaderPathFrag []byte

	//go:embed shaders/scene3d.vert
	shaderScene3dVert []byte

	//go:embed shaders/scene3d.frag
	shaderScene3dFrag []byte
)

func shaderSourceNamed(name string) ([]byte, []byte) {
	switch name {
	case "line":
		return shaderLineVert, shaderLineFrag
	case "simple":
		return shaderSimpleVert, shaderSimpleFrag
	case "rectangle":
		return shaderRectangleVert, shaderRectangleFrag
	case "round_rectangle":
		return shaderRectangleVert, shaderRoundrectangleFrag
	case "polygon":
		return shaderRectangleVert, shaderPolygonFrag
	case "arc":
		return shaderRectangleVert, shaderArcFrag
	case "ripple":
		return shaderRectangleVert, shaderRippleFrag
	case "blur_h":
		return shaderBlurVert, shaderBlurHFrag
	case "blur_v":
		return shaderBlurVert, shaderBlurVFrag
	case "path":
		return shaderPathVert, shaderPathFrag
	case "scene3d":
		return shaderScene3dVert, shaderScene3dFrag
	}
	return nil, nil
}
