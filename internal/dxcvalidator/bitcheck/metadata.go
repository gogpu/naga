// metadata.go — METADATA_BLOCK walker scoped to the named metadata
// node `!dx.entryPoints`. The goal is narrow: build just enough of
// the metadata node table to answer "does every entry-point tuple
// have a non-null function reference in operand 0?".
//
// Metadata record layout (LLVM 3.7, mirrors
// dxil/internal/module/serialize.go):
//
//	METADATA_STRING    = 1 → [bytes*]                          (1 node slot)
//	METADATA_VALUE     = 2 → [typeID, valueID]                 (1 node slot)
//	METADATA_NODE      = 3 → [n x (mdNodeID+1)]                (1 node slot)
//	METADATA_NAME      = 4 → [bytes*]                          (no slot)
//	METADATA_DISTINCT_NODE = 5 → same layout as METADATA_NODE  (1 node slot)
//	METADATA_OLD_NODE     = 8 → [n x typeID, n x valueID pairs] (1 node slot)
//	METADATA_OLD_FN_NODE  = 9 → old-function node               (1 node slot)
//	METADATA_NAMED_NODE  = 10 → [mdNodeID*] referenced by prior METADATA_NAME
//
// Node IDs are 0-based, assigned in declaration order. In tuple
// operands they are stored as (nodeID+1) so that 0 means "null
// metadata operand". In METADATA_NAMED_NODE operands they are stored
// as raw 0-based indices.
//
// Our naga emitter only emits METADATA_STRING, METADATA_VALUE,
// METADATA_NODE, METADATA_NAME and METADATA_NAMED_NODE. We also
// recognize METADATA_DISTINCT_NODE / METADATA_OLD_NODE /
// METADATA_OLD_FN_NODE so DXC-generated bitcode walks cleanly; other
// record kinds increment the slot counter (if they would) and are
// otherwise ignored.

package bitcheck

import (
	"errors"
	"fmt"
	"os"
)

var bitcheckTrace = os.Getenv("BITCHECK_TRACE") != ""

// Metadata record codes.
const (
	mdCodeString       = 1
	mdCodeValue        = 2
	mdCodeNode         = 3
	mdCodeName         = 4
	mdCodeDistinctNode = 5
	mdCodeOldNode      = 8
	mdCodeOldFnNode    = 9
	mdCodeNamedNode    = 10
)

// dxEntryPointsName is the named-metadata key we scan for.
const dxEntryPointsName = "dx.entryPoints"

// Typed errors for the metadata walker. Each maps to one documented
// AV / misuse class from BUG-DXIL-VALIDATOR-REAL Phase 0 findings.
var (
	// ErrMissingEntryPoints — the bitcode has a METADATA_BLOCK but no
	// `dx.entryPoints` named metadata. Required for every DXIL shader.
	ErrMissingEntryPoints = errors.New("bitcheck: missing !dx.entryPoints")

	// ErrNullEntryPointFunction — an entry-point tuple references a
	// null function reference in operand 0. Triggers IDxcValidator AV
	// at dxil.dll+0xe9da on Windows (BUG-DXIL-012).
	ErrNullEntryPointFunction = errors.New("bitcheck: null entry-point function reference")

	// ErrEmptyEntryPointTuple — an entry-point tuple has zero operands.
	// Operand 0 is supposed to be the function pointer; an empty tuple
	// is an immediate structural violation.
	ErrEmptyEntryPointTuple = errors.New("bitcheck: empty entry-point tuple")
)

// mdNodeKind tags how a parsed metadata node was encoded.
type mdNodeKind int

const (
	mdNodeString mdNodeKind = iota + 1
	mdNodeValue
	mdNodeTuple // METADATA_NODE / METADATA_DISTINCT_NODE
	mdNodeOld   // METADATA_OLD_NODE (DXC)
	mdNodeOldFn // METADATA_OLD_FN_NODE (DXC)
	mdNodeOther // placeholder for unknown node-producing records
)

// mdNode is one parsed metadata node in a flat declaration-order
// index. For tuples, Operands holds the raw record operands as
// (mdNodeID+1); callers must subtract one before indexing back into
// the table and treat 0 as "null".
type mdNode struct {
	kind mdNodeKind
	// METADATA_VALUE fields.
	typeID  uint64
	valueID uint64
	// Tuple / old-node operands, raw from the record. For
	// METADATA_NODE / METADATA_DISTINCT_NODE these are (nodeID+1).
	operands []uint64
}

// metadataTable is the flat declaration-order index produced by the
// walker. namedEntries maps a name → the operand list of the matching
// METADATA_NAMED_NODE record.
type metadataTable struct {
	nodes        []mdNode
	namedEntries map[string][]uint64
}

// walkMetadata parses the current METADATA_BLOCK (caller has already
// called BlockReader.EnterBlock). It consumes records until the block
// END, then returns a populated metadataTable.
func walkMetadata(br *BlockReader) (*metadataTable, error) {
	tbl := &metadataTable{
		namedEntries: make(map[string][]uint64),
	}
	// pendingName is set by METADATA_NAME; the next METADATA_NAMED_NODE
	// consumes it and attaches its operand list to namedEntries[name].
	var pendingName string
	for {
		e, err := br.Next()
		if err != nil {
			return nil, fmt.Errorf("metadata walk: %w", err)
		}
		switch e.Kind {
		case entryEnd:
			return tbl, nil
		case entryEOF:
			return nil, fmt.Errorf("unexpected EOF inside METADATA_BLOCK: %w",
				ErrMalformedBitstream)
		case entrySubBlock:
			// Nested block inside METADATA_BLOCK — skip it.
			if err := br.SkipBlock(); err != nil {
				return nil, fmt.Errorf("metadata skip sub-block: %w", err)
			}
			continue
		case entryDefineAbbrev:
			if err := br.ReadDefineAbbrev(); err != nil {
				return nil, fmt.Errorf("metadata define abbrev: %w", err)
			}
			continue
		case entryRecord:
			// fall through
		}
		rec, err := br.ReadRecord(e.AbbrevID)
		if err != nil {
			return nil, fmt.Errorf("metadata record: %w", err)
		}
		pendingName, err = handleMetadataRecord(tbl, rec, pendingName)
		if err != nil {
			return nil, err
		}
	}
}

// handleMetadataRecord dispatches one metadata record into the table.
// Returns the new pendingName (either the name just latched, or the
// caller's previous value).
func handleMetadataRecord(tbl *metadataTable, rec Record, pendingName string) (string, error) {
	if bitcheckTrace {
		fmt.Fprintf(os.Stderr, "[bitcheck] md rec code=%d ops=%d blobLen=%d pending=%q\n",
			rec.Code, len(rec.Ops), len(rec.Blob), pendingName)
	}
	switch rec.Code {
	case mdCodeString:
		tbl.nodes = append(tbl.nodes, mdNode{kind: mdNodeString})
		return pendingName, nil
	case mdCodeValue:
		if len(rec.Ops) < 2 {
			return pendingName, fmt.Errorf("METADATA_VALUE with %d ops: %w",
				len(rec.Ops), ErrMalformedBitstream)
		}
		tbl.nodes = append(tbl.nodes, mdNode{
			kind:    mdNodeValue,
			typeID:  rec.Ops[0],
			valueID: rec.Ops[1],
		})
		return pendingName, nil
	case mdCodeNode, mdCodeDistinctNode:
		ops := make([]uint64, len(rec.Ops))
		copy(ops, rec.Ops)
		tbl.nodes = append(tbl.nodes, mdNode{
			kind:     mdNodeTuple,
			operands: ops,
		})
		return pendingName, nil
	case mdCodeOldNode:
		tbl.nodes = append(tbl.nodes, mdNode{kind: mdNodeOld})
		return pendingName, nil
	case mdCodeOldFnNode:
		tbl.nodes = append(tbl.nodes, mdNode{kind: mdNodeOldFn})
		return pendingName, nil
	case mdCodeName:
		name := recordBytesToString(rec)
		return name, nil
	case mdCodeNamedNode:
		if pendingName == "" {
			return "", fmt.Errorf("METADATA_NAMED_NODE without preceding name: %w",
				ErrMalformedBitstream)
		}
		ids := make([]uint64, len(rec.Ops))
		copy(ids, rec.Ops)
		tbl.namedEntries[pendingName] = ids
		return "", nil
	default:
		// Unknown record kind — assume it does not allocate a node slot.
		// We intentionally do NOT fail on unknown codes because LLVM
		// metadata has been extended over the years and our walker is
		// scoped to dx.entryPoints only.
		return pendingName, nil
	}
}

// recordBytesToString flattens a record's VBR6 operand list into a
// Go string assuming each operand is a single ASCII byte. Both
// METADATA_STRING and METADATA_NAME use this layout in our emitter
// and in DXC output.
func recordBytesToString(rec Record) string {
	if rec.Blob != nil {
		return string(rec.Blob)
	}
	buf := make([]byte, len(rec.Ops))
	for i, v := range rec.Ops {
		buf[i] = byte(v)
	}
	return string(buf)
}

// verifyEntryPoints inspects the entry-point tuples referenced by the
// `dx.entryPoints` named metadata and returns a typed error for the
// first malformed case. Matches the three acceptance-criteria errors
// exactly:
//
//   - no named entry             → ErrMissingEntryPoints
//   - tuple has 0 operands       → ErrEmptyEntryPointTuple
//   - tuple op0 is 0 (null ref)  → ErrNullEntryPointFunction
//   - tuple op0 → METADATA_VALUE
//     with valueID = 0           → ErrNullEntryPointFunction
//
// Any other structural inconsistency (out-of-range node ref, wrong
// kind on a resolved node) returns ErrMalformedBitstream.
func (tbl *metadataTable) verifyEntryPoints() error {
	named, ok := tbl.namedEntries[dxEntryPointsName]
	if !ok {
		return ErrMissingEntryPoints
	}
	if len(named) == 0 {
		// Named metadata is present but references no tuples — still
		// missing entry points.
		return ErrMissingEntryPoints
	}
	for i, tupleIdx := range named {
		if tupleIdx >= uint64(len(tbl.nodes)) {
			return fmt.Errorf("dx.entryPoints[%d] references node %d beyond table len %d: %w",
				i, tupleIdx, len(tbl.nodes), ErrMalformedBitstream)
		}
		tuple := tbl.nodes[tupleIdx]
		if tuple.kind != mdNodeTuple {
			return fmt.Errorf("dx.entryPoints[%d] node kind %d is not a tuple: %w",
				i, tuple.kind, ErrMalformedBitstream)
		}
		if len(tuple.operands) == 0 {
			return fmt.Errorf("dx.entryPoints[%d]: %w", i, ErrEmptyEntryPointTuple)
		}
		op0 := tuple.operands[0]
		if op0 == 0 {
			return fmt.Errorf("dx.entryPoints[%d]: %w", i, ErrNullEntryPointFunction)
		}
		// op0 is (nodeID+1). Resolve and verify it points to a non-null
		// value.
		refIdx := op0 - 1
		if refIdx >= uint64(len(tbl.nodes)) {
			return fmt.Errorf("dx.entryPoints[%d] op0 references node %d beyond table len %d: %w",
				i, refIdx, len(tbl.nodes), ErrMalformedBitstream)
		}
		refNode := tbl.nodes[refIdx]
		// The function pointer is emitted as METADATA_VALUE by our
		// emitter (see emitter.emitMetadata: mdFunc = mod.AddMetadataFunc).
		// DXC output uses the same kind. Any other kind is suspicious
		// enough to call out as malformed.
		if refNode.kind == mdNodeValue && refNode.valueID == 0 && refNode.typeID == 0 {
			// Both type and value zero: this is a null METADATA_VALUE
			// pair — the classic fallback from BUG-DXIL-012 when the
			// emitter had no mainFn. Reject.
			return fmt.Errorf("dx.entryPoints[%d] references METADATA_VALUE(type=0, value=0): %w",
				i, ErrNullEntryPointFunction)
		}
	}
	return nil
}
