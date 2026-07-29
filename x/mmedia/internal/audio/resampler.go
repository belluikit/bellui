package audio

import "math"

// Resampler converts PCM audio between sample rates using linear interpolation.
// Input and output are signed 16-bit little-endian interleaved PCM.
type Resampler struct {
	srcRate  int
	dstRate  int
	channels int
	ratio    float64

	// Fractional sample position for sub-sample accuracy.
	phase float64

	// Previous sample for interpolation (one per channel).
	prev []int16
}

// NewResampler creates a new resampler.
// If srcRate == dstRate, the resampler is a no-op passthrough.
func NewResampler(srcRate, dstRate, channels int) *Resampler {
	r := &Resampler{
		srcRate:  srcRate,
		dstRate:  dstRate,
		channels: channels,
		ratio:    float64(srcRate) / float64(dstRate),
		prev:     make([]int16, channels),
	}
	return r
}

// NeedsResampling returns true if the sample rates differ.
func (r *Resampler) NeedsResampling() bool {
	return r.srcRate != r.dstRate
}

// Resample converts PCM data from srcRate to dstRate.
// Input: S16LE interleaved at srcRate. Output: S16LE interleaved at dstRate.
func (r *Resampler) Resample(input []byte) []byte {
	if r.srcRate == r.dstRate {
		return input
	}

	bytesPerSample := 2
	frameSize := bytesPerSample * r.channels
	inputFrames := len(input) / frameSize
	if inputFrames == 0 {
		return nil
	}

	// Calculate output frames.
	outputFrames := int(float64(inputFrames) / r.ratio)
	if outputFrames == 0 {
		outputFrames = 1
	}

	output := make([]byte, outputFrames*frameSize)

	for i := 0; i < outputFrames; i++ {
		// Source position for this output sample.
		srcPos := r.phase
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		for ch := 0; ch < r.channels; ch++ {
			var s0, s1 int16

			if srcIdx < inputFrames {
				s0 = readS16LE(input, srcIdx*frameSize+ch*2)
			} else {
				s0 = r.prev[ch]
			}

			if srcIdx+1 < inputFrames {
				s1 = readS16LE(input, (srcIdx+1)*frameSize+ch*2)
			} else {
				s1 = s0
			}

			// Linear interpolation.
			val := float64(s0)*(1.0-frac) + float64(s1)*frac
			sample := int16(math.Round(clampF64(val, -32768, 32767)))
			writeS16LE(output, i*frameSize+ch*2, sample)
		}

		r.phase += r.ratio
	}

	// Save last samples for next buffer's interpolation.
	if inputFrames > 0 {
		lastFrame := (inputFrames - 1) * frameSize
		for ch := 0; ch < r.channels; ch++ {
			r.prev[ch] = readS16LE(input, lastFrame+ch*2)
		}
	}

	// Adjust phase for the consumed input.
	r.phase -= float64(inputFrames)
	if r.phase < 0 {
		r.phase = 0
	}

	return output
}

// Reset clears the resampler state.
func (r *Resampler) Reset() {
	r.phase = 0
	for i := range r.prev {
		r.prev[i] = 0
	}
}

func readS16LE(data []byte, offset int) int16 {
	if offset+1 >= len(data) {
		return 0
	}
	return int16(data[offset]) | int16(data[offset+1])<<8
}

func writeS16LE(data []byte, offset int, val int16) {
	if offset+1 >= len(data) {
		return
	}
	data[offset] = byte(val)
	data[offset+1] = byte(val >> 8)
}

func clampF64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
