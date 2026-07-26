//go:build !gl

package cache

// TextureType is an opaque WebGPU texture handle under the `webgpu` build: a
// uint32 registry key into the webgpu painter's texture table (internal/painter/
// webgpu/texture.go), used uniformly on every platform (desktop/wasm/mobile)
// instead of the GL-specific gl.Texture types. The desktop GL path already used
// uint32 here, so this only changes wasm/mobile webgpu builds.
type TextureType = uint32

var NoTexture = TextureType(0)

type textureInfo struct {
	textureCacheBase

	texture  TextureType
	textFree func()
}

// IsValid will return true if the passed texture is potentially a texture.
func IsValid(texture TextureType) bool {
	return texture != NoTexture
}
