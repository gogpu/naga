// bitstream.go — primitive bit-level reader for the LLVM 3.7 bitstream
// format.
//
// This is the mirror of dxil/internal/bitcode/writer.go — every read
// primitive here has a matching write primitive in the emitter. Any
// divergence between these two files means round-trip breaks.
//
// Layout, from the LLVM 3.7 bitcode reference:
//
//   - Bits are packed LSB-first into 32-bit little-endian dwords.
//   - The first bit written goes to the lowest bit of the first dword.
//   - Align32 pads with zero bits to the next 32-bit boundary.
//
// The reader never panics on malformed input; every primitive returns a
// typed error when the bit cursor would run past the end of the blob.
//
// Reference: https://releases.llvm.org/3.7.1/docs/BitCodeFormat.html
// Mirror: dxil/internal/bitcode/writer.go

package bitcheck

import (
	"errors"
	"fmt"
)

// ErrUnexpectedEOF is returned when a read primitive would advance the
// bit cursor past the end of the blob.
var ErrUnexpectedEOF = errors.New("bitcheck: unexpected end of bitstream")

// ErrVBRTooWide is returned when a VBR read accumulates more than 64
// bits of data without terminating. The stream is malformed.
var ErrVBRTooWide = errors.New("bitcheck: VBR value exceeds 64 bits")

// ErrInvalidWidth is returned when a read primitive is called with an
// out-of-range width argument (e.g. Fixed(0) / Fixed(>64), VBR(<2)).
var ErrInvalidWidth = errors.New("bitcheck: invalid read width")

// Reader reads individual bits from an LLVM 3.7 bitstream. The cursor
// is a bit offset from the start of the backing byte slice.
type Reader struct {
	data        []byte
	bitPos      uint64 // absolute bit offset from data[0]
	abbrevWidth uint   // current abbreviation ID width
}

// NewReader creates a Reader positioned at bit 0 with the given initial
// abbreviation ID width. The top-level bitstream abbreviation width in
// LLVM 3.7 is 2 (matching dxil/internal/bitcode/writer.go's NewWriter).
func NewReader(data []byte, abbrevWidth uint) *Reader {
	return &Reader{data: data, abbrevWidth: abbrevWidth}
}

// AbbrevWidth returns the current abbreviation ID width.
func (r *Reader) AbbrevWidth() uint { return r.abbrevWidth }

// SetAbbrevWidth overwrites the current abbreviation ID width.
// Used by block enter / exit to switch scopes.
func (r *Reader) SetAbbrevWidth(w uint) { r.abbrevWidth = w }

// BitPos returns the current bit offset from the start of the blob.
func (r *Reader) BitPos() uint64 { return r.bitPos }

// SetBitPos moves the cursor to an absolute bit offset. Used by block
// skip (fast-forward via the 32-bit block length word).
func (r *Reader) SetBitPos(bit uint64) { r.bitPos = bit }

// BitLen returns the total number of bits in the underlying blob.
func (r *Reader) BitLen() uint64 {
	// len(data) always fits in uint64 with headroom.
	return uint64(len(r.data)) * 8
}

// AtEnd reports whether the cursor has reached (or passed) the end.
func (r *Reader) AtEnd() bool {
	return r.bitPos >= r.BitLen()
}

// Remaining returns the number of bits left in the stream. Callers can
// use this to cheaply detect truncation before attempting a multi-bit
// read.
func (r *Reader) Remaining() uint64 {
	bl := r.BitLen()
	if r.bitPos >= bl {
		return 0
	}
	return bl - r.bitPos
}

// readBit reads a single bit without bounds checking. Callers MUST have
// pre-validated that r.bitPos < r.BitLen().
func (r *Reader) readBit() uint64 {
	byteIdx := r.bitPos >> 3
	bitIdx := uint(r.bitPos & 7)
	bit := uint64((r.data[byteIdx] >> bitIdx) & 1)
	r.bitPos++
	return bit
}

// ReadFixed reads a fixed-width integer value from the bitstream.
// width must be in [1, 64]. Returns ErrInvalidWidth otherwise and
// ErrUnexpectedEOF if the read would run past the end of the stream.
//
// Mirror: bitcode.Writer.WriteFixed.
func (r *Reader) ReadFixed(width uint) (uint64, error) {
	if width == 0 || width > 64 {
		return 0, fmt.Errorf("ReadFixed(%d): %w", width, ErrInvalidWidth)
	}
	if r.Remaining() < uint64(width) {
		return 0, fmt.Errorf("ReadFixed(%d) at bit %d: %w", width, r.bitPos, ErrUnexpectedEOF)
	}
	var out uint64
	for i := uint(0); i < width; i++ {
		out |= r.readBit() << i
	}
	return out, nil
}

// ReadVBR reads a variable-bit-rate integer from the bitstream.
//
// VBR(n) splits the value into chunks of (n-1) data bits. The high bit
// of each chunk is set to 1 when more chunks follow, 0 on the last
// chunk. width must be >= 2.
//
// Mirror: bitcode.Writer.WriteVBR.
func (r *Reader) ReadVBR(width uint) (uint64, error) {
	if width < 2 || width > 32 {
		return 0, fmt.Errorf("ReadVBR(%d): %w", width, ErrInvalidWidth)
	}
	dataBits := width - 1
	dataMask := (uint64(1) << dataBits) - 1
	contBit := uint64(1) << dataBits

	var value uint64
	var shift uint
	for {
		chunk, err := r.ReadFixed(width)
		if err != nil {
			return 0, err
		}
		// Guard against accumulating a value that would not fit in 64
		// bits — either the producer is malformed, or we are reading
		// outside a valid record boundary.
		if shift >= 64 && (chunk&dataMask) != 0 {
			return 0, fmt.Errorf("ReadVBR(%d) at bit %d: %w",
				width, r.bitPos, ErrVBRTooWide)
		}
		if shift < 64 {
			value |= (chunk & dataMask) << shift
		}
		if chunk&contBit == 0 {
			return value, nil
		}
		shift += dataBits
		if shift > 128 { // clearly pathological
			return 0, fmt.Errorf("ReadVBR(%d) at bit %d: %w",
				width, r.bitPos, ErrVBRTooWide)
		}
	}
}

// ReadChar6 reads a 6-bit character and returns its decoded ASCII byte.
//
// Mirror: bitcode.Writer.WriteChar6 / EncodeChar6.
func (r *Reader) ReadChar6() (byte, error) {
	v, err := r.ReadFixed(6)
	if err != nil {
		return 0, err
	}
	return DecodeChar6(uint32(v))
}

// DecodeChar6 converts a 6-bit encoded value back to its ASCII byte.
// The encoding is:
//
//	0..25  → 'a'..'z'
//	26..51 → 'A'..'Z'
//	52..61 → '0'..'9'
//	62     → '.'
//	63     → '_'
func DecodeChar6(v uint32) (byte, error) {
	switch {
	case v < 26:
		return byte('a' + v), nil
	case v < 52:
		return byte('A' + (v - 26)), nil
	case v < 62:
		return byte('0' + (v - 52)), nil
	case v == 62:
		return '.', nil
	case v == 63:
		return '_', nil
	default:
		return 0, fmt.Errorf("DecodeChar6(%d): %w", v, ErrInvalidWidth)
	}
}

// Align32 advances the bit cursor to the next 32-bit boundary, padding
// over zero bits. Returns an error only if the alignment walks past the
// end of the stream AND there were non-zero pad bits — callers may hit
// legitimate end-of-stream alignment on the last word.
//
// Mirror: bitcode.Writer.Align32.
func (r *Reader) Align32() error {
	rem := r.bitPos & 31
	if rem == 0 {
		return nil
	}
	skip := 32 - rem
	// End-of-stream alignment is fine as long as we don't overshoot.
	if r.bitPos+skip > r.BitLen() {
		// Tolerate alignment that sits exactly at the end of the blob
		// (the last word was already consumed). Reject alignment that
		// would land past the end.
		if r.bitPos > r.BitLen() {
			return fmt.Errorf("Align32 at bit %d: %w", r.bitPos, ErrUnexpectedEOF)
		}
		// Clamp to end — no more bits to pad over.
		r.bitPos = r.BitLen()
		return nil
	}
	r.bitPos += skip
	return nil
}
