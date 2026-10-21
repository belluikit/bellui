package painter

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The kernel's standard deviation follows from its weights, which the GL and
// WebGPU blur shaders spell out.
func TestBlurKernelSD(t *testing.T) {
	weights := []float64{0.0162, 0.0540, 0.1216, 0.1945, 0.2270, 0.1945, 0.1216, 0.0540, 0.0162}
	var sum, moment float64
	for i, w := range weights {
		k := float64(i - 4)
		sum += w
		moment += w * k * k
	}
	assert.InDelta(t, math.Sqrt(moment/sum), blurKernelSD, 0.001)
}

// PlanBlur keeps the taps at most a texel apart, and reaches the sigma asked
// for at the halved size (#442).
func TestPlanBlur(t *testing.T) {
	for _, sigma := range []float32{0.5, 1.689, 1.7, 6, 12, 36, 200} {
		halvings, step := PlanBlur(sigma)
		assert.LessOrEqual(t, step, float32(1), "sigma %v", sigma)
		if halvings == 0 {
			perPass := step * blurKernelSD
			assert.InDelta(t, sigma*sigma, BlurPasses*perPass*perPass, 1e-3, "sigma %v", sigma)
		} else {
			// The passes and halving together make up sigma.
			scale := float32(int(1) << halvings)
			perPass := step * blurKernelSD * scale
			assert.InDelta(t, sigma*sigma, BlurPasses*perPass*perPass+halvingVariance*scale*scale, 1e-2, "sigma %v", sigma)
			assert.Greater(t, step, float32(0.4), "sigma %v: halved once too often", sigma)
		}
	}
	halvings, _ := PlanBlur(2.3)
	assert.Zero(t, halvings, "a sigma the kernel covers at full size is not halved")
}

func TestHalveTexels(t *testing.T) {
	assert.Equal(t, 50, HalveTexels(100))
	assert.Equal(t, 51, HalveTexels(101))
	assert.Equal(t, 1, HalveTexels(1))
}
