package bitcheck

import (
	"errors"
	"testing"
)

// blockWriter extends testWriter with the block enter / exit and
// unabbreviated-record helpers we need to build end-to-end fixtures.
// Mirrors dxil/internal/bitcode/writer.go.
type blockWriter struct {
	*testWriter
	abbrevWidth uint
	stack       []bwScope
}

type bwScope struct {
	outerWidth uint
	sizeOffset int // byte offset of 32-bit size placeholder
}

func newBlockWriter(initialWidth uint) *blockWriter {
	return &blockWriter{
		testWriter:  newTestWriter(),
		abbrevWidth: initialWidth,
	}
}

func (b *blockWriter) emitAbbrevID(id uint32) { b.writeBits(id, b.abbrevWidth) }

func (b *blockWriter) enterBlock(blockID, newAbbrevWidth uint) {
	b.emitAbbrevID(abbrevEnterSubblock)
	b.writeVBR(uint64(blockID), 8)
	b.writeVBR(uint64(newAbbrevWidth), 4)
	b.align32()
	b.stack = append(b.stack, bwScope{
		outerWidth: b.abbrevWidth,
		sizeOffset: len(b.data),
	})
	// Placeholder 32-bit block size.
	b.data = append(b.data, 0, 0, 0, 0)
	b.abbrevWidth = newAbbrevWidth
}

func (b *blockWriter) exitBlock() {
	b.emitAbbrevID(abbrevEndBlock)
	b.align32()
	n := len(b.stack) - 1
	scope := b.stack[n]
	b.stack = b.stack[:n]
	bodyStart := scope.sizeOffset + 4
	bodySize := len(b.data) - bodyStart
	wordSize := uint32(bodySize / 4)
	b.data[scope.sizeOffset+0] = byte(wordSize)
	b.data[scope.sizeOffset+1] = byte(wordSize >> 8)
	b.data[scope.sizeOffset+2] = byte(wordSize >> 16)
	b.data[scope.sizeOffset+3] = byte(wordSize >> 24)
	b.abbrevWidth = scope.outerWidth
}

func (b *blockWriter) emitRecord(code uint, values []uint64) {
	b.emitAbbrevID(abbrevUnabbrevRecord)
	b.writeVBR(uint64(code), 6)
	b.writeVBR(uint64(len(values)), 6)
	for _, v := range values {
		b.writeVBR(v, 6)
	}
}

func TestBlockReader_EmptyBlock(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(8, 3) // MODULE_BLOCK, inner width 3
	w.exitBlock()
	data := w.bytes()

	r := NewReader(data, 2)
	br := NewBlockReader(r)
	e, err := br.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if e.Kind != entrySubBlock || e.BlockID != 8 {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if err := br.EnterBlock(); err != nil {
		t.Fatalf("EnterBlock: %v", err)
	}
	if r.AbbrevWidth() != 3 {
		t.Fatalf("inner width = %d, want 3", r.AbbrevWidth())
	}
	if br.Depth() != 1 {
		t.Fatalf("depth = %d, want 1", br.Depth())
	}
	e, err = br.Next()
	if err != nil {
		t.Fatalf("Next inside: %v", err)
	}
	if e.Kind != entryEnd {
		t.Fatalf("expected entryEnd, got %+v", e)
	}
	if err := br.ExitBlock(); err != nil {
		t.Fatalf("ExitBlock: %v", err)
	}
	if r.AbbrevWidth() != 2 {
		t.Fatalf("restored width = %d, want 2", r.AbbrevWidth())
	}
	if br.Depth() != 0 {
		t.Fatalf("depth after exit = %d, want 0", br.Depth())
	}
}

func TestBlockReader_UnabbrevRecordRoundTrip(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(15, 3) // METADATA_BLOCK
	w.emitRecord(4, []uint64{'d', 'x', '.', 'e', 'n', 't', 'r', 'y', 'P', 'o', 'i', 'n', 't', 's'})
	w.emitRecord(3, []uint64{0, 1, 2, 3}) // METADATA_NODE fake
	w.exitBlock()
	data := w.bytes()

	r := NewReader(data, 2)
	br := NewBlockReader(r)

	e, err := br.Next()
	if err != nil || e.Kind != entrySubBlock || e.BlockID != 15 {
		t.Fatalf("expected METADATA_BLOCK entry, got %+v err=%v", e, err)
	}
	if err := br.EnterBlock(); err != nil {
		t.Fatalf("EnterBlock: %v", err)
	}

	// Record 1 — METADATA_NAME.
	e, err = br.Next()
	if err != nil || e.Kind != entryRecord {
		t.Fatalf("expected record, got %+v err=%v", e, err)
	}
	rec, err := br.ReadRecord(e.AbbrevID)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	if rec.Code != 4 {
		t.Fatalf("record[0].code = %d, want 4", rec.Code)
	}
	if string([]byte{
		byte(rec.Ops[0]), byte(rec.Ops[1]), byte(rec.Ops[2]), byte(rec.Ops[3]),
		byte(rec.Ops[4]), byte(rec.Ops[5]), byte(rec.Ops[6]), byte(rec.Ops[7]),
		byte(rec.Ops[8]), byte(rec.Ops[9]), byte(rec.Ops[10]), byte(rec.Ops[11]),
		byte(rec.Ops[12]), byte(rec.Ops[13]),
	}) != "dx.entryPoints" {
		t.Fatalf("record[0] name mismatch: %v", rec.Ops)
	}

	// Record 2 — METADATA_NODE.
	e, err = br.Next()
	if err != nil || e.Kind != entryRecord {
		t.Fatalf("expected record 2, got %+v err=%v", e, err)
	}
	rec, err = br.ReadRecord(e.AbbrevID)
	if err != nil {
		t.Fatalf("ReadRecord 2: %v", err)
	}
	if rec.Code != 3 || len(rec.Ops) != 4 || rec.Ops[0] != 0 || rec.Ops[3] != 3 {
		t.Fatalf("record[1] mismatch: %+v", rec)
	}

	// End of block.
	e, err = br.Next()
	if err != nil || e.Kind != entryEnd {
		t.Fatalf("expected end, got %+v err=%v", e, err)
	}
	if err := br.ExitBlock(); err != nil {
		t.Fatalf("ExitBlock: %v", err)
	}
}

func TestBlockReader_SkipBlock(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(8, 3) // outer MODULE_BLOCK

	// Inner block we want to skip.
	w.enterBlock(17, 3) // TYPE_BLOCK
	w.emitRecord(1, []uint64{42})
	w.emitRecord(2, []uint64{100, 200})
	w.exitBlock()

	// A second inner block we'll enter and inspect.
	w.enterBlock(15, 3) // METADATA_BLOCK
	w.emitRecord(99, []uint64{7})
	w.exitBlock()

	w.exitBlock()
	data := w.bytes()

	r := NewReader(data, 2)
	br := NewBlockReader(r)

	// Enter MODULE_BLOCK.
	e, err := br.Next()
	if err != nil || e.Kind != entrySubBlock || e.BlockID != 8 {
		t.Fatalf("expected MODULE, got %+v err=%v", e, err)
	}
	if err := br.EnterBlock(); err != nil {
		t.Fatalf("EnterBlock: %v", err)
	}

	// First inner: TYPE_BLOCK — skip it.
	e, err = br.Next()
	if err != nil || e.Kind != entrySubBlock || e.BlockID != 17 {
		t.Fatalf("expected TYPE_BLOCK, got %+v err=%v", e, err)
	}
	if err := br.SkipBlock(); err != nil {
		t.Fatalf("SkipBlock: %v", err)
	}

	// Second inner: METADATA_BLOCK — enter and read.
	e, err = br.Next()
	if err != nil || e.Kind != entrySubBlock || e.BlockID != 15 {
		t.Fatalf("after skip expected METADATA_BLOCK, got %+v err=%v", e, err)
	}
	if err := br.EnterBlock(); err != nil {
		t.Fatalf("EnterBlock metadata: %v", err)
	}
	e, err = br.Next()
	if err != nil || e.Kind != entryRecord {
		t.Fatalf("expected record, got %+v err=%v", e, err)
	}
	rec, err := br.ReadRecord(e.AbbrevID)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	if rec.Code != 99 || len(rec.Ops) != 1 || rec.Ops[0] != 7 {
		t.Fatalf("record mismatch: %+v", rec)
	}
	e, _ = br.Next()
	if e.Kind != entryEnd {
		t.Fatalf("expected end, got %+v", e)
	}
	_ = br.ExitBlock()

	e, _ = br.Next()
	if e.Kind != entryEnd {
		t.Fatalf("expected module end, got %+v", e)
	}
	_ = br.ExitBlock()
	if br.Depth() != 0 {
		t.Fatalf("depth after full exit = %d", br.Depth())
	}
}

func TestBlockReader_MalformedInnerWidth(t *testing.T) {
	// Hand-craft a sub-block header with abbrev width 0, which is
	// invalid — ReadBlock must reject it.
	w := newBlockWriter(2)
	w.emitAbbrevID(abbrevEnterSubblock)
	w.writeVBR(8, 8) // block id
	w.writeVBR(0, 4) // invalid new abbrev width
	w.align32()
	w.data = append(w.data, 0, 0, 0, 0) // fake block size
	data := w.bytes()

	r := NewReader(data, 2)
	br := NewBlockReader(r)
	e, err := br.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if e.Kind != entrySubBlock {
		t.Fatalf("expected sub block, got %+v", e)
	}
	if err := br.EnterBlock(); !errors.Is(err, ErrMalformedBitstream) {
		t.Fatalf("EnterBlock with width 0 err = %v, want ErrMalformedBitstream", err)
	}
}

func TestBlockReader_MalformedBlockLength(t *testing.T) {
	// Block length that walks past end of blob.
	w := newBlockWriter(2)
	w.emitAbbrevID(abbrevEnterSubblock)
	w.writeVBR(8, 8) // block id
	w.writeVBR(3, 4) // inner abbrev width
	w.align32()
	// Huge fake block length.
	w.data = append(w.data, 0xFF, 0xFF, 0xFF, 0xFF)
	data := w.bytes()

	r := NewReader(data, 2)
	br := NewBlockReader(r)
	_, err := br.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if err := br.EnterBlock(); !errors.Is(err, ErrMalformedBitstream) {
		t.Fatalf("EnterBlock huge len err = %v, want ErrMalformedBitstream", err)
	}
}

// TestBlockReader_DefineAbbrev_FixedOperand round-trips a single
// DEFINE_ABBREV followed by a record that uses it. This is the one
// path our naga emitter never exercises but that DXC output uses
// heavily.
func TestBlockReader_DefineAbbrev_FixedOperand(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(15, 3) // METADATA_BLOCK, 3-bit abbrev id

	// Emit DEFINE_ABBREV:
	//   numops = 2
	//   op0 = Fixed(6)   → record code
	//   op1 = Fixed(8)   → one data byte
	w.emitAbbrevID(abbrevDefineAbbrev)
	w.writeVBR(2, 5)  // numops
	w.writeBits(0, 1) // op0: not literal
	w.writeFixed(uint64(operandFixed), 3)
	w.writeVBR(6, 5)  // op0 width
	w.writeBits(0, 1) // op1: not literal
	w.writeFixed(uint64(operandFixed), 3)
	w.writeVBR(8, 5) // op1 width

	// Emit record using the new abbrev (id = 4).
	w.writeBits(4, 3)     // abbrev id, 3-bit current width
	w.writeFixed(42, 6)   // code = 42
	w.writeFixed(0xCC, 8) // data = 0xCC

	w.exitBlock()
	data := w.bytes()

	r := NewReader(data, 2)
	br := NewBlockReader(r)

	e, err := br.Next()
	if err != nil || e.Kind != entrySubBlock {
		t.Fatalf("Next: %+v %v", e, err)
	}
	if err := br.EnterBlock(); err != nil {
		t.Fatalf("EnterBlock: %v", err)
	}

	// First entry: DEFINE_ABBREV.
	e, err = br.Next()
	if err != nil || e.Kind != entryDefineAbbrev {
		t.Fatalf("expected defineAbbrev, got %+v err=%v", e, err)
	}
	if err := br.ReadDefineAbbrev(); err != nil {
		t.Fatalf("ReadDefineAbbrev: %v", err)
	}

	// Second entry: abbreviated record.
	e, err = br.Next()
	if err != nil || e.Kind != entryRecord {
		t.Fatalf("expected record, got %+v err=%v", e, err)
	}
	if e.AbbrevID != 4 {
		t.Fatalf("AbbrevID = %d, want 4", e.AbbrevID)
	}
	rec, err := br.ReadRecord(e.AbbrevID)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	if rec.Code != 42 || len(rec.Ops) != 1 || rec.Ops[0] != 0xCC {
		t.Fatalf("record mismatch: %+v", rec)
	}
}
