package bitcheck

import (
	"errors"
	"testing"
)

// emitNamedMD writes a METADATA_NAME record (operand per byte) followed
// by a METADATA_NAMED_NODE record with the given operand list. Mirrors
// dxil/internal/module/serialize.go emitNamedMetadata.
func (w *blockWriter) emitNamedMD(name string, nodeIDs []uint64) {
	nameOps := make([]uint64, len(name))
	for i := 0; i < len(name); i++ {
		nameOps[i] = uint64(name[i])
	}
	w.emitRecord(mdCodeName, nameOps)
	w.emitRecord(mdCodeNamedNode, nodeIDs)
}

// buildEntryPointsFixture builds a minimal METADATA_BLOCK with:
//
//	nodes[0] = METADATA_VALUE(type=1, value=1) (valid function ref)
//	nodes[1] = METADATA_STRING "main"
//	nodes[2] = METADATA_NODE [op0=(nodeID+1 for node 0), op1=(nodeID+1 for node 1)]
//	!dx.entryPoints = [2]  (raw node index, not +1)
//
// The writer returns a byte slice that NewReader can consume from the
// very first bit — no outer MODULE_BLOCK / magic wrapper.
func buildEntryPointsFixture(t *testing.T, op0 uint64, typ, val uint64) []byte {
	t.Helper()
	w := newBlockWriter(2)
	w.enterBlock(15, 3) // METADATA_BLOCK

	// Node 0: METADATA_VALUE.
	w.emitRecord(mdCodeValue, []uint64{typ, val})
	// Node 1: METADATA_STRING "main".
	w.emitRecord(mdCodeString, []uint64{'m', 'a', 'i', 'n'})
	// Node 2: METADATA_NODE with operand 0 (function ref) + operand 1 (name).
	w.emitRecord(mdCodeNode, []uint64{op0, 2 /* node 1 + 1 */})
	// Named metadata "dx.entryPoints" → [node 2].
	w.emitNamedMD(dxEntryPointsName, []uint64{2})

	w.exitBlock()
	return w.bytes()
}

// runWalker wraps the common BlockReader → walkMetadata sequence.
func runWalker(t *testing.T, data []byte) (*metadataTable, error) {
	t.Helper()
	r := NewReader(data, 2)
	br := NewBlockReader(r)
	e, err := br.Next()
	if err != nil {
		return nil, err
	}
	if e.Kind != entrySubBlock || e.BlockID != 15 {
		t.Fatalf("expected METADATA_BLOCK entry, got %+v", e)
	}
	if err := br.EnterBlock(); err != nil {
		return nil, err
	}
	return walkMetadata(br)
}

func TestMetadataWalker_ValidEntryPoint(t *testing.T) {
	// op0 = (nodeID 0 + 1) = 1 → METADATA_VALUE(type=1, val=1): non-null.
	data := buildEntryPointsFixture(t, 1, 1, 1)
	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); err != nil {
		t.Fatalf("verifyEntryPoints: %v", err)
	}
}

func TestMetadataWalker_NullOperand(t *testing.T) {
	data := buildEntryPointsFixture(t, 0, 1, 1) // op0=0 → null
	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); !errors.Is(err, ErrNullEntryPointFunction) {
		t.Fatalf("verifyEntryPoints err = %v, want ErrNullEntryPointFunction", err)
	}
}

func TestMetadataWalker_MetadataValueNull(t *testing.T) {
	// op0 = 1 → node 0 (METADATA_VALUE) but its type+value are both 0.
	data := buildEntryPointsFixture(t, 1, 0, 0)
	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); !errors.Is(err, ErrNullEntryPointFunction) {
		t.Fatalf("verifyEntryPoints err = %v, want ErrNullEntryPointFunction", err)
	}
}

func TestMetadataWalker_MissingEntryPoints(t *testing.T) {
	// Build a METADATA_BLOCK without any dx.entryPoints named metadata.
	w := newBlockWriter(2)
	w.enterBlock(15, 3)
	w.emitRecord(mdCodeValue, []uint64{1, 1})
	w.emitRecord(mdCodeString, []uint64{'a'})
	// Named metadata with a different name.
	w.emitNamedMD("llvm.ident", []uint64{0})
	w.exitBlock()
	data := w.bytes()

	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); !errors.Is(err, ErrMissingEntryPoints) {
		t.Fatalf("verifyEntryPoints err = %v, want ErrMissingEntryPoints", err)
	}
}

func TestMetadataWalker_EmptyTuple(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(15, 3)
	// Node 0: empty METADATA_NODE (zero operands).
	w.emitRecord(mdCodeNode, []uint64{})
	w.emitNamedMD(dxEntryPointsName, []uint64{0})
	w.exitBlock()
	data := w.bytes()

	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); !errors.Is(err, ErrEmptyEntryPointTuple) {
		t.Fatalf("verifyEntryPoints err = %v, want ErrEmptyEntryPointTuple", err)
	}
}

func TestMetadataWalker_NamedNodeWithoutName(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(15, 3)
	// NAMED_NODE without a preceding NAME record — malformed.
	w.emitRecord(mdCodeNamedNode, []uint64{0})
	w.exitBlock()
	data := w.bytes()

	if _, err := runWalker(t, data); !errors.Is(err, ErrMalformedBitstream) {
		t.Fatalf("walk err = %v, want ErrMalformedBitstream", err)
	}
}

func TestMetadataWalker_DistinctNodeAccepted(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(15, 3)
	w.emitRecord(mdCodeValue, []uint64{1, 5})        // node 0
	w.emitRecord(mdCodeString, []uint64{'x'})        // node 1
	w.emitRecord(mdCodeDistinctNode, []uint64{1, 2}) // node 2: distinct tuple
	w.emitNamedMD(dxEntryPointsName, []uint64{2})
	w.exitBlock()
	data := w.bytes()

	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); err != nil {
		t.Fatalf("verifyEntryPoints: %v", err)
	}
}

func TestMetadataWalker_MultipleEntryPoints(t *testing.T) {
	w := newBlockWriter(2)
	w.enterBlock(15, 3)
	w.emitRecord(mdCodeValue, []uint64{1, 1}) // node 0: first fn
	w.emitRecord(mdCodeValue, []uint64{1, 2}) // node 1: second fn
	w.emitRecord(mdCodeString, []uint64{'a'}) // node 2
	w.emitRecord(mdCodeNode, []uint64{1, 3})  // node 3: tuple → node 0
	w.emitRecord(mdCodeNode, []uint64{2, 3})  // node 4: tuple → node 1
	w.emitNamedMD(dxEntryPointsName, []uint64{3, 4})
	w.exitBlock()
	data := w.bytes()

	tbl, err := runWalker(t, data)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tbl.verifyEntryPoints(); err != nil {
		t.Fatalf("verifyEntryPoints: %v", err)
	}

	// Now break the SECOND tuple's operand 0 → null, must still fire.
	w2 := newBlockWriter(2)
	w2.enterBlock(15, 3)
	w2.emitRecord(mdCodeValue, []uint64{1, 1})
	w2.emitRecord(mdCodeValue, []uint64{1, 2})
	w2.emitRecord(mdCodeString, []uint64{'a'})
	w2.emitRecord(mdCodeNode, []uint64{1, 3})
	w2.emitRecord(mdCodeNode, []uint64{0, 3}) // null op0
	w2.emitNamedMD(dxEntryPointsName, []uint64{3, 4})
	w2.exitBlock()

	tbl2, err := runWalker(t, w2.bytes())
	if err != nil {
		t.Fatalf("walk2: %v", err)
	}
	if err := tbl2.verifyEntryPoints(); !errors.Is(err, ErrNullEntryPointFunction) {
		t.Fatalf("verifyEntryPoints2 err = %v, want ErrNullEntryPointFunction", err)
	}
}
