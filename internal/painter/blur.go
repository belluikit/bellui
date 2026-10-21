package painter

import "math"

// blurKernelSD is the standard deviation, in taps, of the separable 9-tap
// kernel the painters' blur shaders share (weights 0.2270, 0.1945, 0.1216,
// 0.0540 and 0.0162 at 0 to ±4 taps).
const blurKernelSD = 1.689

// BlurSigma is the Gaussian sigma a BlurRadius of canvas.BoxShadow or
// canvas.BackdropBlur asks for, in the radius's units: half the radius, as in
// CSS (#442).
func BlurSigma(radius float32) float32 {
	return radius / 2
}

// PlanBlur is how a painter blurs a texture by sigma texels with the 9-tap
// kernel: it halves the texture halvings times, then blurs it along each axis
// BlurPasses times, with taps step texels apart at that size. Taps much more
// than a texel apart would sample a hard edge as separate copies of it, which
// show as bands; the halved texture also costs less to blur, and the blur
// hides the detail halving loses.
//
// Halving, and drawing the halved texture stretched, blur too: measured on
// WebGPU, by halvingVariance·4^halvings texels² on average (#442), and the
// kernel's own step leaves that out. How much depends on where the edges fall
// on the halved texels, between none and about twice that, so a blur halved
// several times can be off by up to about a tenth of sigma. Passes that each
// blur by sigma/√2 reach further before the texture has to be halved: with
// one pass per axis, DialogLevel's shadow came out 11% wide.
func PlanBlur(sigma float32) (halvings int, step float32) {
	perPass := sigma / float32(math.Sqrt(BlurPasses))
	step = perPass / blurKernelSD
	for step > maxBlurStep {
		step /= 2
		halvings++
	}
	if halvings == 0 {
		return 0, step
	}
	scale := float32(int(1) << halvings)
	own := (sigma*sigma - halvingVariance*scale*scale) / BlurPasses
	return halvings, float32(math.Sqrt(float64(own))) / (blurKernelSD * scale)
}

// BlurPasses is how many times PlanBlur's blur runs along each axis.
const BlurPasses = 2

// maxBlurStep is the widest step, in texels, PlanBlur blurs with before it
// halves the texture.
const maxBlurStep = 1

// halvingVariance is the variance, in halved texels², that halving a texture
// and drawing it stretched add to a blur.
const halvingVariance = 0.25

// HalveTexels is a texture side after one halving: never less than a texel.
func HalveTexels(n int) int {
	return max(1, (n+1)/2)
}
