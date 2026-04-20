// Package bitcode implements a bit-level writer for LLVM 3.7 bitcode format.
//
// DXIL uses LLVM 3.7 bitcode as its binary encoding. This writer implements
// the primitives specified in the LLVM Bitcode Format documentation:
// https://releases.llvm.org/3.7.1/docs/BitCodeFormat.html
//
// The writer supports fixed-width integers, variable-width integers (VBR),
// 6-bit character encoding, block enter/exit with size backpatching, and
// unabbreviated record emission.
//
// Reference implementation: Mesa's dxil_buffer.c + dxil_module.c
// (src/microsoft/compiler/ in the Mesa source tree).
package bitcode

import (
	"encoding/binary"
	"fmt"
)

// Abbreviation IDs defined by the LLVM bitstream format.
const (
	EndBlock       = 0 // END_BLOCK — marks end of current block
	EnterSubblock  = 1 // ENTER_SUBBLOCK — begins a new block
	DefineAbbrev   = 2 // DEFINE_ABBREV — defines a new abbreviation
	UnabbrevRecord = 3 // UNABBREV_RECORD — unabbreviated record
)

// Writer writes individual bits to build LLVM 3.7 bitcode output.
//
// Bits accumulate in a 64-bit buffer and are flushed as little-endian
// 32-bit words when the buffer reaches 32 bits, matching Mesa's
// dxil_buffer implementation.
type Writer struct {
	data    []byte // output bytes (always multiple of 4 after flush)
	buf     uint64 // bit accumulator (up to 63 bits)
	bufBits uint   // number of valid bits in buf

	abbrevWidth uint // current abbreviation ID width

	// Block stack for enter/exit with size backpatching.
	blocks []blockState
}

// blockState saves the state when entering a sub-block, so it can be
// restored when the block is exited.
type blockState struct {
	abbrevWidth uint // outer block's abbrev width
	sizeOffset  int  // byte offset of the 32-bit size placeholder
}

// NewWriter creates a Writer with the given initial abbreviation ID width.
// The LLVM bitstream starts with abbrevWidth=2 for the outermost scope.
func NewWriter(abbrevWidth uint) *Writer {
	return &Writer{
		data:        make([]byte, 0, 256),
		abbrevWidth: abbrevWidth,
	}
}

// flushDword writes the lower 32 bits of buf to data.
func (w *Writer) flushDword() {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], uint32(w.buf&0xFFFFFFFF))
	w.data = append(w.data, tmp[:]...)
	w.buf >>= 32
	w.bufBits -= 32
}

// WriteBits writes the lowest `width` bits of data to the bitstream.
// width must be in [1, 32] and data must fit in width bits.
func (w *Writer) WriteBits(data uint32, width uint) {
	w.buf |= uint64(data) << w.bufBits
	w.bufBits += width
	if w.bufBits >= 32 {
		w.flushDword()
	}
}

// WriteFixed writes a fixed-width integer value. If width is 0, nothing
// is written. For values > 32 bits wide, the value is split across two
// WriteBits calls.
func (w *Writer) WriteFixed(value uint64, width uint) {
	if width == 0 {
		return
	}
	if value > 0xFFFFFFFF {
		w.WriteBits(uint32(value&0xFFFFFFFF), width)          //nolint:mnd // bit mask
		w.WriteBits(uint32((value>>32)&0xFFFFFFFF), width-32) //nolint:mnd // bit mask
	} else {
		w.WriteBits(uint32(value&0xFFFFFFFF), width) //nolint:mnd // bit mask
	}
}

// WriteVBR writes a variable bit-rate encoded integer.
//
// VBR(n) splits the value into chunks of (n-1) data bits. The high bit
// of each chunk is set to 1 if more chunks follow, 0 for the last chunk.
//
// For example, VBR(4) encoding of 27:
//
//	27 = 0b11011
//	First chunk: 011 with continuation=1 → 1011
//	Second chunk: 011 with continuation=0 → 0011
//	Written as: 1011 0011
//
// width must be >= 2.
func (w *Writer) WriteVBR(value uint64, width uint) {
	tag := uint32(1) << (width - 1)
	mask := tag - 1
	for value > uint64(mask) {
		chunk := uint32(value&uint64(mask)) | tag //nolint:gosec // masked to fit
		value >>= width - 1
		w.WriteBits(chunk, width)
	}
	w.WriteBits(uint32(value&0xFFFFFFFF), width) //nolint:mnd // bit mask
}

// EncodeSignedVBR ZigZag-encodes a signed integer for VBR transmission.
// LLVM bitcode (release_37 lib/Bitcode/Writer/BitcodeWriter.cpp emitSignedInt64)
// uses this for instruction operands that may be forward references — most
// notably FUNC_CODE_INST_PHI value operands, which can reference instructions
// that appear later in basic-block order than the phi itself.
//
// Mapping (LSB carries the sign, magnitude in the upper bits):
//
//	v >= 0 → v << 1
//	v <  0 → (-v << 1) | 1
//
// Symmetric to LLVM's BitstreamReader::ReadVBR + sign decode on the read side.
func EncodeSignedVBR(value int64) uint64 {
	if value >= 0 {
		return uint64(value) << 1
	}
	return (uint64(-value) << 1) | 1
}

// WriteChar6 writes a 6-bit character code. Only the characters
// [a-zA-Z0-9._] are representable.
func (w *Writer) WriteChar6(ch byte) {
	w.WriteBits(EncodeChar6(ch), 6)
}

// EncodeChar6 converts a character to its 6-bit encoding.
//
//	'a'..'z' → 0..25
//	'A'..'Z' → 26..51
//	'0'..'9' → 52..61
//	'.'      → 62
//	'_'      → 63
func EncodeChar6(ch byte) uint32 {
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
	default:
		panic(fmt.Sprintf("bitcode: invalid char6 character: %q", ch))
	}
}

// IsChar6String returns true if all bytes in s are representable in char6.
func IsChar6String(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isChar6(s[i]) {
			return false
		}
	}
	return true
}

func isChar6(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '.' || ch == '_'
}

// Align32 pads the bitstream with zero bits until the bit position is
// a multiple of 32. This is required after END_BLOCK and before
// the block size word in ENTER_SUBBLOCK.
func (w *Writer) Align32() {
	if w.bufBits > 0 {
		w.bufBits = 32
		w.flushDword()
	}
}

// emitAbbrevID writes the abbreviation ID using the current block's
// abbreviation width.
func (w *Writer) emitAbbrevID(id uint32) {
	w.WriteBits(id, w.abbrevWidth)
}

// EnterBlock begins a new sub-block with the given block ID and
// abbreviation width for the block body.
//
// Format: [ENTER_SUBBLOCK, blockID(vbr8), newAbbrevLen(vbr4), <align32>, blockLen(32)]
//
// The block length is backpatched when ExitBlock is called.
func (w *Writer) EnterBlock(blockID uint, abbrevLen uint) {
	w.emitAbbrevID(EnterSubblock)
	w.WriteVBR(uint64(blockID), 8)
	w.WriteVBR(uint64(abbrevLen), 4)
	w.Align32()

	// Save current state on the block stack.
	w.blocks = append(w.blocks, blockState{
		abbrevWidth: w.abbrevWidth,
		sizeOffset:  len(w.data), // position where size word goes
	})

	// Write a placeholder for the block size (in 32-bit words).
	w.data = append(w.data, 0, 0, 0, 0)

	// Switch to the new abbreviation width.
	w.abbrevWidth = abbrevLen
}

// ExitBlock ends the current sub-block.
//
// Format: [END_BLOCK, <align32>]
//
// The block size placeholder written by EnterBlock is backpatched with
// the actual size in 32-bit words.
func (w *Writer) ExitBlock() {
	w.emitAbbrevID(EndBlock)
	w.Align32()

	// Pop the block stack and compute the block body size.
	n := len(w.blocks) - 1
	state := w.blocks[n]
	w.blocks = w.blocks[:n]

	// Size = number of 32-bit words in block body (after the size word).
	bodyStart := state.sizeOffset + 4
	bodySize := len(w.data) - bodyStart
	wordSize := bodySize / 4
	binary.LittleEndian.PutUint32(w.data[state.sizeOffset:], uint32(wordSize)) //nolint:gosec // block size always fits in uint32

	// Restore the outer block's abbreviation width.
	w.abbrevWidth = state.abbrevWidth
}

// EmitRecord writes an unabbreviated record.
//
// Format: [UNABBREV_RECORD, code(vbr6), numops(vbr6), [op(vbr6)]*]
func (w *Writer) EmitRecord(code uint, values []uint64) {
	w.emitAbbrevID(UnabbrevRecord)
	w.WriteVBR(uint64(code), 6)
	w.WriteVBR(uint64(len(values)), 6)
	for _, v := range values {
		w.WriteVBR(v, 6)
	}
}

// EmitRecordWithBlob writes an unabbreviated record followed by a blob.
// This is used for encoding raw byte data (e.g., strings) in records.
func (w *Writer) EmitRecordWithBlob(code uint, values []uint64, blob []byte) {
	// Emit the record first.
	allValues := make([]uint64, len(values)+len(blob))
	copy(allValues, values)
	for i, b := range blob {
		allValues[len(values)+i] = uint64(b)
	}
	w.EmitRecord(code, allValues)
}

// Bytes returns the finalized bitcode output. The caller must ensure
// all blocks have been exited before calling this method.
func (w *Writer) Bytes() []byte {
	// Flush any remaining bits.
	if w.bufBits > 0 {
		w.Align32()
	}
	return w.data
}

// Len returns the current output size in bytes.
func (w *Writer) Len() int {
	return len(w.data)
}
