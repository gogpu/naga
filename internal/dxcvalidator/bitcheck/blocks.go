// blocks.go — block enumeration and abbreviated-record decoding for
// the LLVM 3.7 bitstream format.
//
// The bitstream is a sequence of records nested inside blocks. Every
// record starts with an "abbrev ID" whose width is the current block's
// abbreviation-ID width. Four IDs are reserved:
//
//	0 END_BLOCK      — close the current block
//	1 ENTER_SUBBLOCK — open a new block
//	2 DEFINE_ABBREV  — declare a new abbreviation in the current scope
//	3 UNABBREV_RECORD — an unabbreviated record (code + operands as VBR6)
//
// Any ID >= 4 refers to a previously-declared abbreviation in the
// current block's abbrev table. Our own emitter never declares
// abbreviations (it only uses UNABBREV_RECORD), but DXC-generated
// bitcode uses them extensively inside metadata records — so the reader
// must handle them to parse third-party input correctly.
//
// DEFINE_ABBREV encoding (reference: LLVM BitCodes.h):
//
//	numops: VBR(5)
//	for each operand:
//	  isLiteral: Fixed(1)
//	  if literal: value: VBR(8)        → literal operand
//	  else:
//	    encoding: Fixed(3)             → one of {Fixed, VBR, Array, Char6, Blob}
//	    if encoding in {Fixed, VBR}:
//	      data: VBR(5)                 → bit width / chunk width
//
// Array/Char6/Blob operands have no extra data in the definition; the
// element operand for Array is the NEXT operand in the abbrev list (so
// "[Array, Fixed(8)]" describes a variable-length array of 8-bit ints).
// Blob is a VBR6 length + align32 + length bytes + align32.
//
// This file also knows just enough about the two standard blocks we
// care about (MODULE_BLOCK=8 and METADATA_BLOCK=15) to let the metadata
// walker in metadata.go skip over everything else via the 32-bit block
// length word written after ENTER_SUBBLOCK.

package bitcheck

import (
	"errors"
	"fmt"
)

// Standard abbreviation IDs (mirror bitcode.writer constants).
const (
	abbrevEndBlock       = 0 // END_BLOCK
	abbrevEnterSubblock  = 1 // ENTER_SUBBLOCK
	abbrevDefineAbbrev   = 2 // DEFINE_ABBREV
	abbrevUnabbrevRecord = 3 // UNABBREV_RECORD
)

// Abbrev operand kinds (from LLVM 3.7 BitCodes.h).
const (
	operandFixed = 1
	operandVBR   = 2
	operandArray = 3
	operandChar6 = 4
	operandBlob  = 5
)

// ErrMalformedBitstream is returned when any structural invariant of
// the bitstream is violated.
var ErrMalformedBitstream = errors.New("bitcheck: malformed bitstream")

// operand describes one slot of an abbreviation definition.
type operand struct {
	kind    int    // operand* constants above, or 0 for literal
	literal uint64 // used when kind == 0
	data    uint64 // Fixed/VBR bit width
}

// abbrev is a parsed DEFINE_ABBREV record.
type abbrev struct {
	ops []operand
}

// Record is a decoded bitstream record — one entry inside a block.
type Record struct {
	Code uint64
	Ops  []uint64
	// Blob holds the raw bytes of a BLOB-encoded operand, if any. Only
	// the metadata walker uses this for METADATA_STRING records that
	// happen to be encoded as abbreviated blobs. Nil for unabbrev'd
	// records.
	Blob []byte
}

// Entry categorizes the next structural element in the stream. The
// walker in metadata.go uses this to decide whether to recurse into a
// block, consume a record, or return.
type Entry struct {
	Kind     entryKind
	BlockID  uint64 // valid when Kind == entrySubBlock
	AbbrevID uint64 // valid when Kind == entryRecord
}

type entryKind int

const (
	entryEnd entryKind = iota
	entrySubBlock
	entryDefineAbbrev
	entryRecord // regular record (either unabbrev'd or abbreviated)
	entryEOF
)

// blockScope tracks the information the reader needs to skip or exit
// a sub-block cleanly. The abbrev table is scoped per-block — entering
// a new block saves the outer one, entering END_BLOCK pops back to it.
type blockScope struct {
	abbrevWidth uint
	abbrevs     []abbrev
	endBitPos   uint64 // precomputed body-end bit position
}

// BlockReader walks a bitstream one entry at a time, tracking the
// nested block scopes and per-block abbreviation tables. It is built
// on top of a Reader and never owns the underlying slice.
type BlockReader struct {
	r      *Reader
	scopes []blockScope
	// abbrevs is the abbrev table for the currently-open block. It is
	// swapped on enter / exit.
	abbrevs []abbrev
}

// NewBlockReader wraps an existing Reader. The top-level scope has no
// end position (the stream ends with the last byte of the blob) and
// starts with an empty abbrev table.
func NewBlockReader(r *Reader) *BlockReader {
	return &BlockReader{r: r}
}

// Next returns the next structural entry at the current cursor. The
// caller must act on it:
//
//	entrySubBlock     → EnterBlock or SkipBlock
//	entryDefineAbbrev → ReadDefineAbbrev (already consumed header)
//	entryRecord       → ReadRecord (pass e.AbbrevID)
//	entryEnd          → block body complete; caller should ExitBlock
//	entryEOF          → cursor is at (or past) end of top-level stream
func (b *BlockReader) Next() (Entry, error) {
	if b.r.AtEnd() {
		return Entry{Kind: entryEOF}, nil
	}
	// If we are inside a sub-block with a known body end, stop there.
	if len(b.scopes) > 0 {
		top := b.scopes[len(b.scopes)-1]
		if top.endBitPos != 0 && b.r.BitPos() >= top.endBitPos {
			return Entry{Kind: entryEOF}, nil
		}
	}
	abbrevID, err := b.r.ReadFixed(b.r.AbbrevWidth())
	if err != nil {
		return Entry{}, fmt.Errorf("read abbrev id: %w", err)
	}
	switch abbrevID {
	case abbrevEndBlock:
		return Entry{Kind: entryEnd}, nil
	case abbrevEnterSubblock:
		blockID, err := b.r.ReadVBR(8)
		if err != nil {
			return Entry{}, fmt.Errorf("read sub-block id: %w", err)
		}
		return Entry{Kind: entrySubBlock, BlockID: blockID}, nil
	case abbrevDefineAbbrev:
		return Entry{Kind: entryDefineAbbrev}, nil
	default:
		return Entry{Kind: entryRecord, AbbrevID: abbrevID}, nil
	}
}

// EnterBlock is called after Next() returns entrySubBlock. It reads
// the new abbrev width, aligns, consumes the 32-bit block length word,
// pushes a new scope, and switches the reader to the inner abbrev
// width. The block-length word is remembered so SkipBlock / end-of-
// block detection is O(1).
func (b *BlockReader) EnterBlock() error {
	newAbbrevWidth, err := b.r.ReadVBR(4)
	if err != nil {
		return fmt.Errorf("read new abbrev width: %w", err)
	}
	if newAbbrevWidth == 0 || newAbbrevWidth > 32 {
		return fmt.Errorf("sub-block abbrev width %d: %w",
			newAbbrevWidth, ErrMalformedBitstream)
	}
	if err := b.r.Align32(); err != nil {
		return fmt.Errorf("align before block length: %w", err)
	}
	blockLen32, err := b.r.ReadFixed(32)
	if err != nil {
		return fmt.Errorf("read block length: %w", err)
	}
	// Block body ends at current bit position + blockLen32 * 32 bits.
	bodyStart := b.r.BitPos()
	bodyBits := blockLen32 * 32
	endBit := bodyStart + bodyBits
	if endBit > b.r.BitLen() {
		return fmt.Errorf("block body end %d > stream len %d: %w",
			endBit, b.r.BitLen(), ErrMalformedBitstream)
	}
	// Push outer scope.
	b.scopes = append(b.scopes, blockScope{
		abbrevWidth: b.r.AbbrevWidth(),
		abbrevs:     b.abbrevs,
		endBitPos:   endBit,
	})
	// Switch to inner scope.
	b.r.SetAbbrevWidth(uint(newAbbrevWidth))
	b.abbrevs = nil
	return nil
}

// SkipBlock fast-forwards past a sub-block using the 32-bit block
// length word, without decoding any records. The caller must NOT have
// already called EnterBlock. SkipBlock is used to skip non-metadata
// blocks (TYPE_BLOCK, CONSTANTS_BLOCK, FUNCTION_BLOCK, …) in O(1).
func (b *BlockReader) SkipBlock() error {
	// VBR4 new-abbrev-width (ignored).
	if _, err := b.r.ReadVBR(4); err != nil {
		return fmt.Errorf("skip: read new abbrev width: %w", err)
	}
	if err := b.r.Align32(); err != nil {
		return fmt.Errorf("skip: align: %w", err)
	}
	blockLen32, err := b.r.ReadFixed(32)
	if err != nil {
		return fmt.Errorf("skip: read block length: %w", err)
	}
	skipBits := blockLen32 * 32
	target := b.r.BitPos() + skipBits
	if target > b.r.BitLen() {
		return fmt.Errorf("skip: block body end %d > stream %d: %w",
			target, b.r.BitLen(), ErrMalformedBitstream)
	}
	b.r.SetBitPos(target)
	return nil
}

// ExitBlock is called after Next() returns entryEnd. It aligns the
// cursor to the next 32-bit boundary (matching the writer's Align32
// after END_BLOCK) and pops back to the outer block's abbrev state.
func (b *BlockReader) ExitBlock() error {
	if err := b.r.Align32(); err != nil {
		return fmt.Errorf("align after end block: %w", err)
	}
	if len(b.scopes) == 0 {
		return fmt.Errorf("end block with empty scope stack: %w", ErrMalformedBitstream)
	}
	n := len(b.scopes) - 1
	outer := b.scopes[n]
	b.scopes = b.scopes[:n]
	b.r.SetAbbrevWidth(outer.abbrevWidth)
	b.abbrevs = outer.abbrevs
	return nil
}

// ReadDefineAbbrev parses a DEFINE_ABBREV record and appends it to the
// current block's abbreviation table. Called after Next() returns
// entryDefineAbbrev.
func (b *BlockReader) ReadDefineAbbrev() error {
	numops, err := b.r.ReadVBR(5)
	if err != nil {
		return fmt.Errorf("read numops: %w", err)
	}
	if numops > 256 { // sanity bound — real abbrevs rarely exceed ~10
		return fmt.Errorf("numops %d too large: %w", numops, ErrMalformedBitstream)
	}
	ops := make([]operand, 0, numops)
	for i := uint64(0); i < numops; i++ {
		isLit, err := b.r.ReadFixed(1)
		if err != nil {
			return fmt.Errorf("read isLiteral: %w", err)
		}
		if isLit == 1 {
			v, err := b.r.ReadVBR(8)
			if err != nil {
				return fmt.Errorf("read literal value: %w", err)
			}
			ops = append(ops, operand{kind: 0, literal: v})
			continue
		}
		enc, err := b.r.ReadFixed(3)
		if err != nil {
			return fmt.Errorf("read encoding: %w", err)
		}
		switch enc {
		case operandFixed, operandVBR:
			data, err := b.r.ReadVBR(5)
			if err != nil {
				return fmt.Errorf("read operand data: %w", err)
			}
			if data == 0 || data > 64 {
				return fmt.Errorf("operand width %d: %w", data, ErrMalformedBitstream)
			}
			ops = append(ops, operand{kind: int(enc), data: data})
		case operandArray, operandChar6, operandBlob:
			ops = append(ops, operand{kind: int(enc)})
		default:
			return fmt.Errorf("unknown operand encoding %d: %w", enc, ErrMalformedBitstream)
		}
	}
	b.abbrevs = append(b.abbrevs, abbrev{ops: ops})
	return nil
}

// ReadRecord consumes a record body given its abbrev id. For
// abbrevUnabbrevRecord (id 3) the format is
// [code(VBR6), numops(VBR6), op(VBR6)*]. For id >= 4 it is an
// abbreviated record against the table slot (id - 4).
func (b *BlockReader) ReadRecord(abbrevID uint64) (Record, error) {
	if abbrevID == abbrevUnabbrevRecord {
		return b.readUnabbrevRecord()
	}
	return b.readAbbrevRecord(abbrevID)
}

func (b *BlockReader) readUnabbrevRecord() (Record, error) {
	code, err := b.r.ReadVBR(6)
	if err != nil {
		return Record{}, fmt.Errorf("unabbrev: read code: %w", err)
	}
	numops, err := b.r.ReadVBR(6)
	if err != nil {
		return Record{}, fmt.Errorf("unabbrev: read numops: %w", err)
	}
	if numops > uint64(len(b.r.data))*8 {
		return Record{}, fmt.Errorf("unabbrev: numops %d too large: %w",
			numops, ErrMalformedBitstream)
	}
	ops := make([]uint64, 0, numops)
	for i := uint64(0); i < numops; i++ {
		v, err := b.r.ReadVBR(6)
		if err != nil {
			return Record{}, fmt.Errorf("unabbrev: op[%d]: %w", i, err)
		}
		ops = append(ops, v)
	}
	return Record{Code: code, Ops: ops}, nil
}

func (b *BlockReader) readAbbrevRecord(abbrevID uint64) (Record, error) {
	if abbrevID < 4 {
		return Record{}, fmt.Errorf("abbrev id %d < 4: %w", abbrevID, ErrMalformedBitstream)
	}
	idx := abbrevID - 4
	if idx >= uint64(len(b.abbrevs)) {
		return Record{}, fmt.Errorf("abbrev id %d out of range [0,%d): %w",
			abbrevID, len(b.abbrevs)+4, ErrMalformedBitstream)
	}
	ab := b.abbrevs[idx]
	if len(ab.ops) == 0 {
		return Record{}, fmt.Errorf("empty abbrev %d: %w", abbrevID, ErrMalformedBitstream)
	}
	var rec Record
	// The first operand is always the record code.
	codeOp := ab.ops[0]
	code, err := b.readOperandValue(codeOp)
	if err != nil {
		return Record{}, fmt.Errorf("abbrev code: %w", err)
	}
	rec.Code = code
	// Subsequent operands: literals / fixed / vbr go straight in; array
	// takes a preceding length(VBR6); blob takes length(VBR6)+align32+
	// bytes+align32.
	for i := 1; i < len(ab.ops); i++ {
		op := ab.ops[i]
		switch op.kind {
		case 0, operandFixed, operandVBR, operandChar6:
			v, err := b.readOperandValue(op)
			if err != nil {
				return Record{}, fmt.Errorf("abbrev op[%d]: %w", i, err)
			}
			rec.Ops = append(rec.Ops, v)
		case operandArray:
			if i+1 >= len(ab.ops) {
				return Record{}, fmt.Errorf("array without element op: %w",
					ErrMalformedBitstream)
			}
			if err := b.readArrayInto(&rec, ab.ops[i+1]); err != nil {
				return Record{}, err
			}
			i++ // consumed the element operand
		case operandBlob:
			if err := b.readBlobInto(&rec); err != nil {
				return Record{}, err
			}
		default:
			return Record{}, fmt.Errorf("unknown abbrev op kind %d: %w",
				op.kind, ErrMalformedBitstream)
		}
	}
	return rec, nil
}

// readArrayInto consumes [count(VBR6), element*count] and appends the
// elements to rec.Ops.
func (b *BlockReader) readArrayInto(rec *Record, elemOp operand) error {
	count, err := b.r.ReadVBR(6)
	if err != nil {
		return fmt.Errorf("array count: %w", err)
	}
	if count > uint64(len(b.r.data))*8 {
		return fmt.Errorf("array count %d: %w", count, ErrMalformedBitstream)
	}
	for j := uint64(0); j < count; j++ {
		v, err := b.readOperandValue(elemOp)
		if err != nil {
			return fmt.Errorf("array[%d]: %w", j, err)
		}
		rec.Ops = append(rec.Ops, v)
	}
	return nil
}

// readBlobInto consumes [length(VBR6), align32, length bytes, align32]
// and stores the raw bytes on rec.Blob.
func (b *BlockReader) readBlobInto(rec *Record) error {
	n, err := b.r.ReadVBR(6)
	if err != nil {
		return fmt.Errorf("blob length: %w", err)
	}
	if err := b.r.Align32(); err != nil {
		return fmt.Errorf("blob pre-align: %w", err)
	}
	if n > uint64(len(b.r.data)) {
		return fmt.Errorf("blob length %d > blob %d: %w",
			n, len(b.r.data), ErrMalformedBitstream)
	}
	blob := make([]byte, n)
	for j := uint64(0); j < n; j++ {
		v, err := b.r.ReadFixed(8)
		if err != nil {
			return fmt.Errorf("blob byte %d: %w", j, err)
		}
		blob[j] = byte(v)
	}
	if err := b.r.Align32(); err != nil {
		return fmt.Errorf("blob post-align: %w", err)
	}
	rec.Blob = blob
	return nil
}

// readOperandValue reads a single scalar abbrev operand value —
// literal, Fixed, VBR, or Char6 — from the bitstream and returns it.
func (b *BlockReader) readOperandValue(op operand) (uint64, error) {
	switch op.kind {
	case 0:
		return op.literal, nil
	case operandFixed:
		return b.r.ReadFixed(uint(op.data))
	case operandVBR:
		return b.r.ReadVBR(uint(op.data))
	case operandChar6:
		v, err := b.r.ReadFixed(6)
		if err != nil {
			return 0, err
		}
		return v, nil
	default:
		return 0, fmt.Errorf("scalar read on op kind %d: %w",
			op.kind, ErrMalformedBitstream)
	}
}

// Depth returns the current block nesting depth (0 at the top level).
func (b *BlockReader) Depth() int { return len(b.scopes) }
