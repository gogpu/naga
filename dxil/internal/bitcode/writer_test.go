package bitcode

import (
	"encoding/binary"
	"testing"
)

func TestWriteBits_SingleBit(t *testing.T) {
	w := NewWriter(2)
	w.WriteBits(1, 1)
	w.Align32()
	got := w.Bytes()
	if len(got) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(got))
	}
	if got[0] != 1 {
		t.Errorf("expected byte 0x01, got 0x%02x", got[0])
	}
}

func TestWriteBits_MultipleBits(t *testing.T) {
	w := NewWriter(2)
	// Write 0b1101 (13) in 4 bits
	w.WriteBits(0b1101, 4)
	w.Align32()
	got := w.Bytes()
	if len(got) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(got))
	}
	// Little-endian: low bits first → byte 0 = 0x0D
	if got[0] != 0x0D {
		t.Errorf("expected byte 0x0D, got 0x%02x", got[0])
	}
}

func TestWriteBits_CrossDword(t *testing.T) {
	w := NewWriter(2)
	// Write 28 bits, then 8 more → should cross the 32-bit boundary
	w.WriteBits(0x0FFFFFFF, 28)
	w.WriteBits(0xAB, 8)
	w.Align32()
	got := w.Bytes()
	if len(got) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(got))
	}
	dw0 := binary.LittleEndian.Uint32(got[0:4])
	dw1 := binary.LittleEndian.Uint32(got[4:8])
	// First 28 bits = 0x0FFFFFFF, next 4 bits from 0xAB (low nibble 0xB) fill dword 0
	// Remaining 4 bits of 0xAB (high nibble 0xA) go to dword 1
	expect0 := uint32(0x0FFFFFFF) | (uint32(0xAB&0x0F) << 28)
	expect1 := uint32(0xAB >> 4)
	if dw0 != expect0 {
		t.Errorf("dword0: expected 0x%08X, got 0x%08X", expect0, dw0)
	}
	if dw1 != expect1 {
		t.Errorf("dword1: expected 0x%08X, got 0x%08X", expect1, dw1)
	}
}

func TestWriteVBR_SmallValue(t *testing.T) {
	// VBR(4): values 0-7 fit in one chunk (3 data bits + 0 continuation)
	tests := []struct {
		name  string
		value uint64
		width uint
		want  uint32 // expected bits written
		bits  uint   // total bits used
	}{
		{"zero_vbr4", 0, 4, 0b0000, 4},
		{"seven_vbr4", 7, 4, 0b0111, 4},
		{"one_vbr6", 1, 6, 0b000001, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWriter(2)
			w.WriteVBR(tt.value, tt.width)
			w.Align32()
			got := binary.LittleEndian.Uint32(w.Bytes()[:4])
			mask := uint32((1 << tt.bits) - 1)
			if got&mask != tt.want {
				t.Errorf("VBR(%d) of %d: got 0x%X, want 0x%X (mask 0x%X)",
					tt.width, tt.value, got&mask, tt.want, mask)
			}
		})
	}
}

func TestWriteVBR_LargeValue(t *testing.T) {
	// VBR(4) encoding of 27:
	// 27 = 0b11011
	// Chunk 1: 011 | 1 (continuation) = 1011 (0xB)
	// Chunk 2: 011 | 0 (last)         = 0011 (0x3)
	// Total: 0011_1011 in bit order = 0x3B when read as 8 bits
	w := NewWriter(2)
	w.WriteVBR(27, 4)
	w.Align32()
	got := binary.LittleEndian.Uint32(w.Bytes()[:4])
	// First 4 bits: 1011 (0xB), next 4 bits: 0011 (0x3)
	first := got & 0xF
	second := (got >> 4) & 0xF
	if first != 0xB {
		t.Errorf("VBR(4) of 27, first chunk: got 0x%X, want 0xB", first)
	}
	if second != 0x3 {
		t.Errorf("VBR(4) of 27, second chunk: got 0x%X, want 0x3", second)
	}
}

func TestWriteVBR_ThreeChunks(t *testing.T) {
	// VBR(4) encoding of 256:
	// 256 = 0b100000000
	// Chunk 1: 000 | 1 = 1000 (low 3 bits of 256 = 0, continuation)
	// Chunk 2: 000 | 1 = 1000 (next 3 bits = 0, continuation)
	// Chunk 3: 100 | 0 = 0100 (final 3 bits = 4, done)
	w := NewWriter(2)
	w.WriteVBR(256, 4)
	w.Align32()
	got := binary.LittleEndian.Uint32(w.Bytes()[:4])
	c1 := got & 0xF
	c2 := (got >> 4) & 0xF
	c3 := (got >> 8) & 0xF
	if c1 != 0x8 {
		t.Errorf("chunk1: got 0x%X, want 0x8", c1)
	}
	if c2 != 0x8 {
		t.Errorf("chunk2: got 0x%X, want 0x8", c2)
	}
	if c3 != 0x4 {
		t.Errorf("chunk3: got 0x%X, want 0x4", c3)
	}
}

func TestEncodeSignedVBR(t *testing.T) {
	// Reference: LLVM 3.7 BitcodeWriter.cpp emitSignedInt64.
	//   v >= 0 → v << 1
	//   v <  0 → (-v << 1) | 1
	tests := []struct {
		name  string
		input int64
		want  uint64
	}{
		{"zero", 0, 0},
		{"positive_one", 1, 2},
		{"positive_max_small", 7, 14},
		{"negative_one", -1, 3},
		{"negative_two", -2, 5},
		{"negative_seven", -7, 15},
		{"large_positive", 1024, 2048},
		{"large_negative", -1024, 2049},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeSignedVBR(tt.input)
			if got != tt.want {
				t.Errorf("EncodeSignedVBR(%d) = %d, want %d", tt.input, got, tt.want)
			}
			// Round-trip check: LSB carries sign, the rest is magnitude.
			lsb := got & 1
			mag := got >> 1
			var roundtrip int64
			if lsb == 0 {
				roundtrip = int64(mag)
			} else {
				roundtrip = -int64(mag)
			}
			if roundtrip != tt.input {
				t.Errorf("round-trip(%d): got %d", tt.input, roundtrip)
			}
		})
	}
}

func TestEncodeChar6(t *testing.T) {
	tests := []struct {
		ch   byte
		want uint32
	}{
		{'a', 0}, {'z', 25},
		{'A', 26}, {'Z', 51},
		{'0', 52}, {'9', 61},
		{'.', 62}, {'_', 63},
	}
	for _, tt := range tests {
		got := EncodeChar6(tt.ch)
		if got != tt.want {
			t.Errorf("EncodeChar6(%q) = %d, want %d", tt.ch, got, tt.want)
		}
	}
}

func TestIsChar6String(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"dxil.ms.dx", true},
		{"hello_world", true},
		{"ABC123", true},
		{"hello world", false},  // space
		{"hello-world", false},  // hyphen
		{"hello\nworld", false}, // newline
		{"", true},
	}
	for _, tt := range tests {
		got := IsChar6String(tt.s)
		if got != tt.want {
			t.Errorf("IsChar6String(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestWriteChar6(t *testing.T) {
	w := NewWriter(2)
	// Write "dxil" in char6
	for _, ch := range []byte("dxil") {
		w.WriteChar6(ch)
	}
	w.Align32()
	got := binary.LittleEndian.Uint32(w.Bytes()[:4])
	// 'd' = 3, 'x' = 23, 'i' = 8, 'l' = 11
	// Bits: 000011 010111 001000 001011
	// In order: 3(6bits) | 23(6bits) | 8(6bits) | 11(6bits) = 24 bits
	d := got & 0x3F
	x := (got >> 6) & 0x3F
	i := (got >> 12) & 0x3F
	l := (got >> 18) & 0x3F
	if d != 3 || x != 23 || i != 8 || l != 11 {
		t.Errorf("char6(dxil): got d=%d x=%d i=%d l=%d, want 3 23 8 11",
			d, x, i, l)
	}
}

func TestEnterExitBlock(t *testing.T) {
	w := NewWriter(2)
	// Enter a block with ID=8 (MODULE), abbrev width=3
	w.EnterBlock(8, 3)
	// The block body is empty
	w.ExitBlock()

	got := w.Bytes()
	if len(got) == 0 {
		t.Fatal("expected non-empty output")
	}
	// The output should be a valid sequence that starts with ENTER_SUBBLOCK
	// and the block size should be backpatched to 1 (one 32-bit word for END_BLOCK+align).
}

func TestEnterExitBlock_NestedBlocks(t *testing.T) {
	w := NewWriter(2)
	w.EnterBlock(8, 3)  // outer block
	w.EnterBlock(17, 4) // inner block (TYPE_BLOCK)
	w.ExitBlock()       // exit inner
	w.ExitBlock()       // exit outer

	got := w.Bytes()
	if len(got) == 0 {
		t.Fatal("expected non-empty output")
	}
	// Check that all data is 32-bit aligned
	if len(got)%4 != 0 {
		t.Errorf("output length %d not 32-bit aligned", len(got))
	}
}

func TestEnterExitBlock_SizeBackpatch(t *testing.T) {
	w := NewWriter(2)
	w.EnterBlock(8, 3)

	// Write some data inside the block
	w.EmitRecord(1, []uint64{42})

	w.ExitBlock()

	got := w.Bytes()
	// Output should be 32-bit aligned
	if len(got)%4 != 0 {
		t.Errorf("output length %d not 32-bit aligned", len(got))
	}
}

func TestEmitRecord_Empty(t *testing.T) {
	w := NewWriter(2)
	w.EnterBlock(8, 3)
	w.EmitRecord(1, nil)
	w.ExitBlock()

	got := w.Bytes()
	if len(got) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestEmitRecord_WithValues(t *testing.T) {
	w := NewWriter(2)
	w.EnterBlock(8, 3)
	w.EmitRecord(7, []uint64{32}) // TYPE_CODE_INTEGER, 32 bits
	w.ExitBlock()

	got := w.Bytes()
	if len(got) == 0 {
		t.Fatal("expected non-empty output")
	}
	if len(got)%4 != 0 {
		t.Errorf("output length %d not 32-bit aligned", len(got))
	}
}

func TestBitcodeHeader(t *testing.T) {
	// LLVM bitcode files start with magic: 'B','C', 0xC0, 0xDE
	w := NewWriter(2)
	w.WriteBits('B', 8)
	w.WriteBits('C', 8)
	w.WriteBits(0xC0, 8)
	w.WriteBits(0xDE, 8)
	got := w.Bytes()
	if len(got) < 4 {
		t.Fatalf("expected at least 4 bytes, got %d", len(got))
	}
	if got[0] != 'B' || got[1] != 'C' || got[2] != 0xC0 || got[3] != 0xDE {
		t.Errorf("magic: got %02X %02X %02X %02X, want 42 43 C0 DE",
			got[0], got[1], got[2], got[3])
	}
}

func TestAlign32_NoOp(t *testing.T) {
	w := NewWriter(2)
	// Already aligned — Align32 should be a no-op
	w.Align32()
	if len(w.Bytes()) != 0 {
		t.Errorf("expected empty output, got %d bytes", len(w.Bytes()))
	}
}

func TestAlign32_WithPendingBits(t *testing.T) {
	w := NewWriter(2)
	w.WriteBits(1, 1) // 1 bit pending
	w.Align32()
	got := w.Bytes()
	if len(got) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(got))
	}
	// Only the lowest bit should be set
	dw := binary.LittleEndian.Uint32(got)
	if dw != 1 {
		t.Errorf("expected 0x00000001, got 0x%08X", dw)
	}
}

func TestWriteFixed_ZeroWidth(t *testing.T) {
	w := NewWriter(2)
	w.WriteFixed(999, 0) // should write nothing
	if w.bufBits != 0 {
		t.Errorf("expected 0 buf bits after WriteFixed(0), got %d", w.bufBits)
	}
}

func TestWriteVBR_Zero(t *testing.T) {
	w := NewWriter(2)
	w.WriteVBR(0, 6)
	w.Align32()
	got := binary.LittleEndian.Uint32(w.Bytes()[:4])
	// VBR(6) of 0 should be 000000
	if got&0x3F != 0 {
		t.Errorf("VBR(6) of 0: got 0x%X, want 0x0", got&0x3F)
	}
}

func TestEmitRecord_LargeValues(t *testing.T) {
	w := NewWriter(2)
	w.EnterBlock(8, 4)
	// Emit a record with several values to exercise the VBR encoding
	values := []uint64{1, 2, 3, 100, 1000, 65535}
	w.EmitRecord(42, values)
	w.ExitBlock()

	got := w.Bytes()
	if len(got)%4 != 0 {
		t.Errorf("output length %d not 32-bit aligned", len(got))
	}
}

func TestFullBitcodeModule(t *testing.T) {
	// Integration test: write a minimal bitcode structure
	// Magic + MODULE_BLOCK(id=8) with VERSION record (code=1, value=1)
	w := NewWriter(2)

	// Magic
	w.WriteBits('B', 8)
	w.WriteBits('C', 8)
	w.WriteBits(0xC0, 8)
	w.WriteBits(0xDE, 8)

	// MODULE_BLOCK (id=8, abbrev=3)
	w.EnterBlock(8, 3)

	// VERSION record: code=1, value=1 (LLVM 3.7)
	w.EmitRecord(1, []uint64{1})

	w.ExitBlock()

	got := w.Bytes()

	// Verify magic
	if got[0] != 'B' || got[1] != 'C' || got[2] != 0xC0 || got[3] != 0xDE {
		t.Error("invalid magic")
	}

	// Verify 32-bit aligned
	if len(got)%4 != 0 {
		t.Errorf("output not 32-bit aligned: %d bytes", len(got))
	}

	// Verify non-trivial size (magic + enter + size + record + exit)
	if len(got) < 12 {
		t.Errorf("output too small: %d bytes", len(got))
	}
}
