package source

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeToneWAV writes a 2 s mono S16 WAV of a sine at freq Hz, amplitude 0.5.
func writeToneWAV(t *testing.T, rate, freq int) string {
	t.Helper()
	n := 2 * rate
	data := make([]byte, 44+2*n)
	copy(data[0:], "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*n))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1) // PCM
	binary.LittleEndian.PutUint16(data[22:], 1) // mono
	binary.LittleEndian.PutUint32(data[24:], uint32(rate))
	binary.LittleEndian.PutUint32(data[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*n))
	for i := 0; i < n; i++ {
		v := 0.5 * math.Sin(2*math.Pi*float64(freq)*float64(i)/float64(rate))
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(int16(math.Round(v*32767))))
	}
	path := filepath.Join(t.TempDir(), "tone.wav")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// toneLevel fits a sine at freq to x and returns its gain against amplitude
// 0.5 in dB, and the SINAD (the tone over everything else) in dB.
func toneLevel(x []float64, rate, freq float64) (gainDB, sinadDB float64) {
	var ss, sc, s2, c2, cs float64
	for i, v := range x {
		w := 2 * math.Pi * freq * float64(i) / rate
		s, c := math.Sin(w), math.Cos(w)
		ss, sc = ss+v*s, sc+v*c
		s2, c2, cs = s2+s*s, c2+c*c, cs+s*c
	}
	det := s2*c2 - cs*cs
	a, b := (ss*c2-sc*cs)/det, (sc*s2-ss*cs)/det
	var sig, res float64
	for i, v := range x {
		w := 2 * math.Pi * freq * float64(i) / rate
		fit := a*math.Sin(w) + b*math.Cos(w)
		sig += fit * fit
		res += (v - fit) * (v - fit)
	}
	return 20 * math.Log10(math.Hypot(a, b)/0.5), 10 * math.Log10(sig/res)
}

// TestSampleRateConversionIsClean pins #582 and #506: a 44.1 kHz stream played
// on the 48 kHz device is converted by the decoder's swresample, chunk by
// chunk as playback decodes it. The linear resampler this replaced dropped
// 0.05% of frames at chunk boundaries and wiped out a 15 kHz tone (-41 dB).
//
// The source is mono, asked for stereo as the device plays it, and
// swresample spreads mono over two channels at -3 dB each, so -3.01 dB is
// the level that means the rate conversion lost nothing.
func TestSampleRateConversionIsClean(t *testing.T) {
	for _, freq := range []int{1000, 15000, 18000} {
		s, err := NewFFmpegSourceURL(writeToneWAV(t, 44100, freq))
		if err != nil {
			t.Fatalf("NewFFmpegSourceURL: %v", err)
		}
		if err := s.SetAudioChannels(2); err != nil {
			t.Fatal(err)
		}
		if err := s.SetAudioSampleRate(48000); err != nil {
			t.Fatal(err)
		}
		if s.SampleRate() != 48000 || s.Channels() != 2 {
			t.Fatalf("source reports %d Hz x %d, want 48000 x 2", s.SampleRate(), s.Channels())
		}

		var left []float64
		chunks := 0
		s.SetAudioCallback(func(b []byte) {
			chunks++
			for i := 0; i+4 <= len(b); i += 4 {
				left = append(left, float64(int16(binary.LittleEndian.Uint16(b[i:])))/32767)
			}
		})
		for i := 0; i < 1000 && !s.HasEnded(); i++ {
			s.Decode(40 * time.Millisecond)
		}
		s.Close()

		if chunks < 10 {
			t.Fatalf("%d Hz: %d chunks; the test needs many to cross chunk boundaries", freq, chunks)
		}
		// swresample's filter delay is not flushed at end of stream: a few
		// dozen frames short, never more, and never long.
		if want := 2 * 48000; len(left) > want || len(left) < want-64 {
			t.Errorf("%d Hz: %d frames, want %d (-64..0)", freq, len(left), want)
		}
		mid := left[len(left)/4 : 3*len(left)/4]
		gain, sinad := toneLevel(mid, 48000, float64(freq))
		t.Logf("%5d Hz: %d chunks, %d frames, gain %.3f dB, SINAD %.1f dB", freq, chunks, len(left), gain, sinad)
		if math.Abs(gain+3.01) > 0.1 {
			t.Errorf("%d Hz: gain %.2f dB, want -3.01 (the mono spread) within 0.1", freq, gain)
		}
		if sinad < 60 {
			t.Errorf("%d Hz: SINAD %.1f dB, want >= 60", freq, sinad)
		}
	}
}

// TestSampleRateConversionKeepsEveryFrame decodes the 5.1 AAC fixture, whose
// packets are 1,024 frames, once at its own 44.1 kHz and once at 48 kHz. The
// converted count must be the native count times 48000/44100, less at most
// swresample's unflushed tail: the linear resampler lost 0.05% (about 48
// frames a second) at 1,024-frame boundaries (#582).
func TestSampleRateConversionKeepsEveryFrame(t *testing.T) {
	frames := func(rate int) (n, chunks, largest int) {
		s := openMultichannel(t)
		if err := s.SetAudioChannels(2); err != nil {
			t.Fatal(err)
		}
		if err := s.SetAudioSampleRate(rate); err != nil {
			t.Fatal(err)
		}
		s.SetAudioCallback(func(b []byte) {
			chunks++
			n += len(b) / 4
			largest = max(largest, len(b)/4)
		})
		for i := 0; i < 1000 && !s.HasEnded(); i++ {
			s.Decode(40 * time.Millisecond)
		}
		return n, chunks, largest
	}
	// AAC trims its priming and padding, so the first and last chunks are
	// short; the rest are 1,024 frames.
	native, chunks, largest := frames(0)
	if native == 0 || largest != 1024 || chunks < 50 {
		t.Fatalf("native decode: %d frames in %d chunks of at most %d; the test needs 1,024-frame chunks", native, chunks, largest)
	}
	converted, _, _ := frames(48000)
	want := float64(native) * 48000 / 44100
	t.Logf("native %d frames in %d chunks, converted %d, exact ratio %.1f", native, chunks, converted, want)
	if d := float64(converted) - want; d > 1 || d < -64 {
		t.Errorf("converted %d frames, want %.0f (-64..+1)", converted, want)
	}
}
