//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (((gles || arm || arm64) && !android && !ios && !mobile && !darwin && !wasm) || android || ios || mobile || wasm)

package gl

import _ "embed"

var (
	//go:embed shaders/line_es.frag
	shaderLineesFrag []byte

	//go:embed shaders/line_es.vert
	shaderLineesVert []byte

	//go:embed shaders/rectangle_es.frag
	shaderRectangleesFrag []byte

	//go:embed shaders/rectangle_es.vert
	shaderRectangleesVert []byte

	//go:embed shaders/round_rectangle_es.frag
	shaderRoundrectangleesFrag []byte

	//go:embed shaders/simple_es.frag
	shaderSimpleesFrag []byte

	//go:embed shaders/simple_es.vert
	shaderSimpleesVert []byte

	//go:embed shaders/polygon_es.frag
	shaderPolygonesFrag []byte

	//go:embed shaders/arc_es.frag
	shaderArcesFrag []byte

	//go:embed shaders/ripple_es.frag
	shaderRippleesFrag []byte

	//go:embed shaders/blur_es.vert
	shaderBluresVert []byte

	//go:embed shaders/blur_h_es.frag
	shaderBlurHesFrag []byte

	//go:embed shaders/blur_v_es.frag
	shaderBlurVesFrag []byte

	//go:embed shaders/path_es.vert
	shaderPathesVert []byte

	//go:embed shaders/path_es.frag
	shaderPathesFrag []byte

	//go:embed shaders/scene3d_es.vert
	shaderScene3desVert []byte

	//go:embed shaders/scene3d_es.frag
	shaderScene3desFrag []byte
)

func shaderSourceNamed(name string) ([]byte, []byte) {
	switch name {
	case "line_es":
		return shaderLineesVert, shaderLineesFrag
	case "simple_es":
		return shaderSimpleesVert, shaderSimpleesFrag
	case "rectangle_es":
		return shaderRectangleesVert, shaderRectangleesFrag
	case "round_rectangle_es":
		return shaderRectangleesVert, shaderRoundrectangleesFrag
	case "polygon_es":
		return shaderRectangleesVert, shaderPolygonesFrag
	case "arc_es":
		return shaderRectangleesVert, shaderArcesFrag
	case "ripple_es":
		return shaderRectangleesVert, shaderRippleesFrag
	case "blur_h_es":
		return shaderBluresVert, shaderBlurHesFrag
	case "blur_v_es":
		return shaderBluresVert, shaderBlurVesFrag
	case "path_es":
		return shaderPathesVert, shaderPathesFrag
	case "scene3d_es":
		return shaderScene3desVert, shaderScene3desFrag
	}
	return nil, nil
}
