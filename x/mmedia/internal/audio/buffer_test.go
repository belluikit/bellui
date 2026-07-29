package audio

import "testing"

func TestRingBuffer_WriteRead(t *testing.T) {
	rb := NewRingBuffer(64)

	data := []byte{1, 2, 3, 4, 5}
	rb.Write(data)

	if rb.Available() != 5 {
		t.Errorf("available = %d, expected 5", rb.Available())
	}

	out := make([]byte, 5)
	rb.Read(out)
	for i, v := range data {
		if out[i] != v {
			t.Errorf("out[%d] = %d, expected %d", i, out[i], v)
		}
	}
}

func TestRingBuffer_Wraparound(t *testing.T) {
	rb := NewRingBuffer(8)

	// Write 6 bytes.
	rb.Write([]byte{1, 2, 3, 4, 5, 6})
	// Read 4.
	out := make([]byte, 4)
	rb.Read(out)
	if out[0] != 1 || out[3] != 4 {
		t.Errorf("first read: %v", out)
	}

	// Write 6 more (wraps around).
	rb.Write([]byte{7, 8, 9, 10, 11, 12})

	// Read all available.
	avail := rb.Available()
	out2 := make([]byte, avail)
	rb.Read(out2)
	// Should get 5,6,7,8,9,10,11,12 (the oldest that fit).
	if out2[0] == 0 {
		t.Error("got zeros instead of data")
	}
}

func TestRingBuffer_EmptyReadReturnsSilence(t *testing.T) {
	rb := NewRingBuffer(16)
	out := make([]byte, 4)
	n, err := rb.Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("read %d bytes, expected 4 (silence)", n)
	}
	for _, v := range out {
		if v != 0 {
			t.Error("expected silence (zeros)")
			break
		}
	}
}

func TestRingBuffer_TotalRead(t *testing.T) {
	rb := NewRingBuffer(64)
	rb.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8})

	out := make([]byte, 4)
	rb.Read(out)
	if rb.TotalRead() != 4 {
		t.Errorf("totalRead = %d, expected 4", rb.TotalRead())
	}

	rb.Read(out)
	if rb.TotalRead() != 8 {
		t.Errorf("totalRead = %d, expected 8", rb.TotalRead())
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	rb := NewRingBuffer(64)
	rb.Write([]byte{1, 2, 3})
	rb.Reset()

	if rb.Available() != 0 {
		t.Errorf("available after reset = %d", rb.Available())
	}
	if rb.TotalRead() != 0 {
		t.Errorf("totalRead after reset = %d", rb.TotalRead())
	}
}
