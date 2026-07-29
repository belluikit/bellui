package audio

import "testing"

func TestResampler_NoOp(t *testing.T) {
	r := NewResampler(44100, 44100, 2)
	if r.NeedsResampling() {
		t.Error("same rate should not need resampling")
	}

	input := []byte{0x00, 0x10, 0x00, 0x20} // 2 bytes per sample, stereo = 4 bytes per frame
	out := r.Resample(input)
	if len(out) != len(input) {
		t.Errorf("passthrough: output %d bytes, expected %d", len(out), len(input))
	}
}

func TestResampler_Downsample(t *testing.T) {
	// 48000 → 24000 = halve the samples.
	r := NewResampler(48000, 24000, 1)
	if !r.NeedsResampling() {
		t.Error("different rates should need resampling")
	}

	// 100 mono samples at 48kHz → ~50 at 24kHz.
	input := make([]byte, 200) // 100 samples * 2 bytes
	for i := 0; i < 100; i++ {
		writeS16LE(input, i*2, int16(i*100))
	}

	out := r.Resample(input)
	outSamples := len(out) / 2
	if outSamples < 40 || outSamples > 60 {
		t.Errorf("2:1 downsample: got %d samples, expected ~50", outSamples)
	}
}

func TestResampler_Upsample(t *testing.T) {
	// 22050 → 44100 = double.
	r := NewResampler(22050, 44100, 1)

	input := make([]byte, 20) // 10 samples
	for i := 0; i < 10; i++ {
		writeS16LE(input, i*2, int16(i*1000))
	}

	out := r.Resample(input)
	outSamples := len(out) / 2
	if outSamples < 15 || outSamples > 25 {
		t.Errorf("1:2 upsample: got %d samples, expected ~20", outSamples)
	}
}

func TestResampler_Reset(t *testing.T) {
	r := NewResampler(48000, 44100, 2)
	input := make([]byte, 40)
	r.Resample(input) // advance state

	r.Reset()
	// Should be able to resample again without issues.
	out := r.Resample(input)
	if out == nil {
		t.Error("resample after reset returned nil")
	}
}
