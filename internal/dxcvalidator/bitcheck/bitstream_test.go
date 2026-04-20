package bitcheck

import (
	"encoding/binary"
	"errors"
	"testing"
)

// testWriter is a minimal bit writer that mirrors
// dxil/internal/bitcode/writer.go's packing layout (LSB-first into
// little-endian 32-bit words). We reimplement it locally here because
// bitcheck lives outside the dxil/internal tree and cannot import the
// writer package. Any divergence between this helper and the real
// writer is a test bug — not a bitcheck bug.
type testWriter struct {
	data    []byte
	buf     uint64
	bufBits uint
}

func newTestWriter() *testWriter { return &testWriter{} }

func (w *testWriter) flushDword() {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], uint32(w.buf&0xFFFFFFFF))
	w.data = append(w.data, tmp[:]...)
	w.buf >>= 32
	w.bufBits -= 32
}

func (w *testWriter) writeBits(value uint32, width uint) {
	w.buf |= uint64(value) << w.bufBits
	w.bufBits += width
	if w.bufBits >= 32 {
		w.flushDword()
	}
}

func (w *testWriter) writeFixed(value uint64, width uint) {
	if width == 0 {
		return
	}
	if width > 32 {
		w.writeBits(uint32(value&0xFFFFFFFF), 32)
		w.writeBits(uint32((value>>32)&0xFFFFFFFF), width-32)
		return
	}
	w.writeBits(uint32(value&0xFFFFFFFF), width)
}

func (w *testWriter) writeVBR(value uint64, width uint) {
	tag := uint32(1) << (width - 1)
	mask := tag - 1
	for value > uint64(mask) {
		chunk := uint32(value&uint64(mask)) | tag
		value >>= width - 1
		w.writeBits(chunk, width)
	}
	w.writeBits(uint32(value&0xFFFFFFFF), width)
}

func encodeChar6(ch byte) uint32 {
	switch {
	case ch >= 'a' && ch <= 'z':
		return uint32(ch - 'a')
	case ch >= 'A' && ch <= 'Z':
		return 26 + uint32(ch-'A')
	case ch >= '0' && ch <= '9':
		return 52 + uint32(ch-'0')
	case ch == '.':
		return 62
	case ch == '_':
		return 63
	}
	panic("bad char6")
}

func (w *testWriter) writeChar6(ch byte) { w.writeBits(encodeChar6(ch), 6) }

func (w *testWriter) align32() {
	if w.bufBits > 0 {
		w.bufBits = 32
		w.flushDword()
	}
}

func (w *testWriter) bytes() []byte {
	if w.bufBits > 0 {
		w.align32()
	}
	return w.data
}

func roundTripWriter(t *testing.T, _ uint, fn func(w *testWriter)) []byte {
	t.Helper()
	w := newTestWriter()
	fn(w)
	return w.bytes()
}

func TestReadFixed_RoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		width uint
		value uint64
	}{
		{"1bit_0", 1, 0},
		{"1bit_1", 1, 1},
		{"4bit_0xF", 4, 0xF},
		{"8bit_0x55", 8, 0x55},
		{"16bit_0xBEEF", 16, 0xBEEF},
		{"24bit_0x123456", 24, 0x123456},
		{"32bit_max", 32, 0xFFFFFFFF},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := roundTripWriter(t, 2, func(w *testWriter) {
				w.writeFixed(tc.value, tc.width)
			})
			r := NewReader(data, 2)
			got, err := r.ReadFixed(tc.width)
			if err != nil {
				t.Fatalf("ReadFixed: %v", err)
			}
			if got != tc.value {
				t.Fatalf("ReadFixed(%d) = %#x, want %#x", tc.width, got, tc.value)
			}
		})
	}
}

func TestReadFixed_InvalidWidth(t *testing.T) {
	r := NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF}, 2)
	if _, err := r.ReadFixed(0); !errors.Is(err, ErrInvalidWidth) {
		t.Fatalf("ReadFixed(0) err = %v, want ErrInvalidWidth", err)
	}
	if _, err := r.ReadFixed(65); !errors.Is(err, ErrInvalidWidth) {
		t.Fatalf("ReadFixed(65) err = %v, want ErrInvalidWidth", err)
	}
}

func TestReadFixed_UnexpectedEOF(t *testing.T) {
	r := NewReader([]byte{0x00}, 2)
	if _, err := r.ReadFixed(16); !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("ReadFixed past end err = %v, want ErrUnexpectedEOF", err)
	}
}

func TestReadVBR_RoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		width uint
		value uint64
	}{
		{"vbr6_0", 6, 0},
		{"vbr6_1", 6, 1},
		{"vbr6_31", 6, 31},
		{"vbr6_32", 6, 32}, // first value that needs continuation
		{"vbr6_100", 6, 100},
		{"vbr8_0", 8, 0},
		{"vbr8_255", 8, 255},
		{"vbr8_65535", 8, 65535},
		{"vbr4_27", 4, 27}, // the example from the writer godoc
		{"vbr32_big", 32, 0x12345678},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := roundTripWriter(t, 2, func(w *testWriter) {
				w.writeVBR(tc.value, tc.width)
			})
			r := NewReader(data, 2)
			got, err := r.ReadVBR(tc.width)
			if err != nil {
				t.Fatalf("ReadVBR: %v", err)
			}
			if got != tc.value {
				t.Fatalf("ReadVBR(%d) = %d, want %d", tc.width, got, tc.value)
			}
		})
	}
}

func TestReadVBR_InvalidWidth(t *testing.T) {
	r := NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF}, 2)
	if _, err := r.ReadVBR(1); !errors.Is(err, ErrInvalidWidth) {
		t.Fatalf("ReadVBR(1) err = %v, want ErrInvalidWidth", err)
	}
	if _, err := r.ReadVBR(33); !errors.Is(err, ErrInvalidWidth) {
		t.Fatalf("ReadVBR(33) err = %v, want ErrInvalidWidth", err)
	}
}

func TestReadVBR_TruncatedMidValue(t *testing.T) {
	// Write a VBR with a continuation bit then truncate mid-chunk.
	data := roundTripWriter(t, 2, func(w *testWriter) {
		w.writeVBR(10000, 6) // needs multiple chunks
	})
	// Chop last byte to provoke truncation.
	data = data[:1]
	r := NewReader(data, 2)
	if _, err := r.ReadVBR(6); !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("ReadVBR truncated err = %v, want ErrUnexpectedEOF", err)
	}
}

func TestReadChar6_RoundTrip(t *testing.T) {
	in := []byte("helloWORLD012._")
	data := roundTripWriter(t, 2, func(w *testWriter) {
		for _, c := range in {
			w.writeChar6(c)
		}
	})
	r := NewReader(data, 2)
	for i, want := range in {
		got, err := r.ReadChar6()
		if err != nil {
			t.Fatalf("ReadChar6[%d]: %v", i, err)
		}
		if got != want {
			t.Fatalf("ReadChar6[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestDecodeChar6_Invalid(t *testing.T) {
	if _, err := DecodeChar6(64); !errors.Is(err, ErrInvalidWidth) {
		t.Fatalf("DecodeChar6(64) err = %v, want ErrInvalidWidth", err)
	}
}

func TestAlign32_MidWord(t *testing.T) {
	// Write 5 bits then align — reader should end up at bit 32.
	data := roundTripWriter(t, 2, func(w *testWriter) {
		w.writeFixed(0b10101, 5)
	})
	r := NewReader(data, 2)
	if _, err := r.ReadFixed(5); err != nil {
		t.Fatalf("ReadFixed: %v", err)
	}
	if err := r.Align32(); err != nil {
		t.Fatalf("Align32: %v", err)
	}
	if r.BitPos() != 32 {
		t.Fatalf("Align32 left cursor at bit %d, want 32", r.BitPos())
	}
}

func TestAlign32_AlreadyAligned(t *testing.T) {
	r := NewReader([]byte{0, 0, 0, 0, 0, 0, 0, 0}, 2)
	r.SetBitPos(32)
	if err := r.Align32(); err != nil {
		t.Fatalf("Align32: %v", err)
	}
	if r.BitPos() != 32 {
		t.Fatalf("Align32 aligned cursor moved to %d", r.BitPos())
	}
}

func TestAtEnd(t *testing.T) {
	r := NewReader([]byte{0xFF, 0xFF}, 2)
	if r.AtEnd() {
		t.Fatalf("AtEnd at bit 0 = true, want false")
	}
	r.SetBitPos(16)
	if !r.AtEnd() {
		t.Fatalf("AtEnd at bit 16 = false, want true")
	}
}

func TestAbbrevWidth(t *testing.T) {
	r := NewReader([]byte{0}, 2)
	if r.AbbrevWidth() != 2 {
		t.Fatalf("initial AbbrevWidth = %d, want 2", r.AbbrevWidth())
	}
	r.SetAbbrevWidth(3)
	if r.AbbrevWidth() != 3 {
		t.Fatalf("SetAbbrevWidth: got %d, want 3", r.AbbrevWidth())
	}
}

func TestReadFixed_Large64bit(t *testing.T) {
	data := roundTripWriter(t, 2, func(w *testWriter) {
		w.writeFixed(0xDEADBEEFCAFEBABE, 64)
	})
	r := NewReader(data, 2)
	got, err := r.ReadFixed(64)
	if err != nil {
		t.Fatalf("ReadFixed(64): %v", err)
	}
	if got != 0xDEADBEEFCAFEBABE {
		t.Fatalf("ReadFixed(64) = %#x, want 0xDEADBEEFCAFEBABE", got)
	}
}
