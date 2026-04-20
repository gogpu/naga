package backend

import "github.com/gogpu/naga/ir"

// PackedElement is the per-element register layout produced by
// PackSignatureElements. Together with the shared SortedMemberIndices order
// it tells every signature producer (OSG1/ISG1, PSV0 PSVSignatureElement,
// !dx.entryPoints metadata, storeOutput / loadInput rowIndex) where each
// element lives in the register file.
//
// All packed values live in a single 4-component register row:
//
//	StartCol + ColCount <= 4
//
// Multi-row elements (e.g. arrays for SV_ClipDistance / SV_CullDistance)
// always start at column 0 and consume Rows register slots; that case is
// represented with Rows > 1.
type PackedElement struct {
	OrigIdx  int    // index into the original members slice (matches SortedMemberIndices)
	Register uint32 // assigned register row (StartRow)
	StartCol uint8  // starting component within the row, 0..3
	ColCount uint8  // number of components consumed, 1..4
	Rows     uint8  // number of register rows consumed, 1..N (N=array length for ClipDistance)
}

// SigPackKind categorizes a signature element for the packing algorithm.
// The packing rules in DXC's DxilSignatureAllocator group elements by kind
// and never pack two elements from different kinds in the same row.
type SigPackKind uint8

const (
	// SigPackLocation is a user @location varying — packed by interpolation
	// group, greedy first-fit within a row. Used for VS output / PS input.
	SigPackLocation SigPackKind = iota
	// SigPackTargetOutput is a fragment-stage SV_Target output. Packing rule
	// is "Register = SemanticIndex" (DXIL.rst PackingKind::Target) — each
	// SV_Target gets its own row matching its semantic index, no packing
	// across targets.
	SigPackTargetOutput
	// SigPackBuiltinSVPosition is SV_Position — always its own row of 4 components.
	SigPackBuiltinSVPosition
	// SigPackBuiltinScalarArray covers SV_ClipDistance / SV_CullDistance which
	// are multi-row arrays of scalars and are not packed with anything.
	SigPackBuiltinScalarArray
	// SigPackBuiltinSystemValue covers other SV_* builtins (VertexID, InstanceID,
	// IsFrontFace, PrimitiveID, etc.) — each on its own row.
	SigPackBuiltinSystemValue
	// SigPackBuiltinSystemManaged covers system-managed PS-stage elements
	// (SV_Depth family, SV_Coverage, SV_StencilRef, SV_SampleIndex on PS input)
	// which carry StartRow=-1 / 0xFF and do not consume a register row at all.
	SigPackBuiltinSystemManaged
	// SigPackOther is the catch-all for elements without a binding — they do
	// not appear in the signature and are allocated nothing.
	SigPackOther
)

// SigPackInterp is the DXC InterpolationMode enum value used to keep
// like-with-like when packing user locations. Two location elements may share
// a register row only if they have the same interpolation mode.
type SigPackInterp uint8

// SigElementInfo is the per-member input to PackSignatureElements.
type SigElementInfo struct {
	Kind        SigPackKind
	ColCount    uint8         // 1..4 for single-row, 1 for multi-row scalar arrays (column width per row)
	Rows        uint8         // 1 for single-row, N for scalar arrays
	Interp      SigPackInterp // matters only when Kind == SigPackLocation
	SemanticIdx uint32        // matters only when Kind == SigPackTargetOutput (Register = SemanticIdx)
}

// PackSignatureElements assigns (Register, StartCol) to every entry in elems
// using DXC's signature-packing rules. Pure function, no side effects.
//
// The returned slice has the same length and ordering as elems — caller
// indexes into it the same way it would index into elems or
// SortedMemberIndices.
//
// Algorithm (mirrors DXC DxilSignatureAllocator::PackPrefixStable):
//
//   - SV_Position consumes its own row of 4 components (Allocated, StartCol=0).
//   - SV_ClipDistance / SV_CullDistance consume Rows contiguous rows starting
//     at StartCol=0 (also unpacked with anything else).
//   - Other SV_* builtins consume their own row each (StartCol=0).
//   - System-managed PS elements get Register=0xFFFFFFFF, StartCol=0,
//     Rows=1 — they do not consume any output row.
//   - User @location elements are packed by interpolation group, greedy
//     first-fit within a 4-column row, in input (sorted) order. A new row is
//     started whenever the current one has no room for the element's columns
//     or its interpolation mode differs from the row's group.
//
// isInput is currently unused — VS input also packs (DXIL.rst: VSIn uses
// PackingKind::InputAssembler which is "no packing" — incremental row
// assignment) but for fragment input (PackingKind::Vertex) packing applies.
// Callers that hit a real divergence between input/output packing in the
// future can branch on the flag without changing the API.
func PackSignatureElements(elems []SigElementInfo, isInput bool) []PackedElement {
	_ = isInput // reserved for future VSIn / patch / mesh-out divergences
	if len(elems) == 0 {
		return nil
	}
	out := make([]PackedElement, len(elems))
	p := sigPacker{}

	for i, e := range elems {
		switch e.Kind {
		case SigPackTargetOutput:
			// PS output SV_Target: Register = SemanticIndex, never packed.
			row := e.SemanticIdx
			if row+1 > p.nextRow {
				p.nextRow = row + 1
			}
			out[i] = PackedElement{OrigIdx: i, Register: row, StartCol: 0, ColCount: e.ColCount, Rows: 1}
			p.openRows = nil
		case SigPackBuiltinSystemManaged:
			out[i] = PackedElement{OrigIdx: i, Register: 0xFFFFFFFF, StartCol: 0, ColCount: e.ColCount, Rows: 1}
		case SigPackOther:
			out[i] = PackedElement{OrigIdx: i, Register: p.nextRow, ColCount: 0, Rows: 0}
		case SigPackBuiltinSVPosition:
			out[i] = PackedElement{OrigIdx: i, Register: p.nextRow, StartCol: 0, ColCount: 4, Rows: 1}
			p.nextRow++
			p.openRows = nil
		case SigPackBuiltinScalarArray:
			rows := e.Rows
			if rows == 0 {
				rows = 1
			}
			out[i] = PackedElement{OrigIdx: i, Register: p.nextRow, StartCol: 0, ColCount: e.ColCount, Rows: rows}
			p.nextRow += uint32(rows)
			p.openRows = nil
		case SigPackBuiltinSystemValue:
			out[i] = PackedElement{OrigIdx: i, Register: p.nextRow, StartCol: 0, ColCount: e.ColCount, Rows: 1}
			p.nextRow++
			p.openRows = nil
		case SigPackLocation:
			out[i] = p.packLocation(i, e)
		}
	}

	return out
}

// sigPacker holds register allocation state for PackSignatureElements.
// DXC PackPrefixStable keeps ALL partially-filled rows available for
// future elements of the same interpolation group. When an element
// doesn't fit in any existing open row, a new row is allocated — but
// the existing partial rows remain available for subsequent narrower
// elements. This produces tighter packing than a single-open-row model.
type sigPacker struct {
	nextRow  uint32
	openRows []sigPackerRow
}

type sigPackerRow struct {
	register uint32
	nextCol  uint8
	interp   SigPackInterp
}

func (p *sigPacker) packLocation(idx int, e SigElementInfo) PackedElement {
	cols := e.ColCount
	if cols == 0 {
		cols = 1
	}
	// Find the first open row for this interp that has room.
	for ri := range p.openRows {
		r := &p.openRows[ri]
		if r.interp == e.Interp && r.nextCol+cols <= 4 {
			pe := PackedElement{OrigIdx: idx, Register: r.register, StartCol: r.nextCol, ColCount: cols, Rows: 1}
			r.nextCol += cols
			return pe
		}
	}
	// No existing row has room — allocate a new one.
	reg := p.nextRow
	p.nextRow++
	pe := PackedElement{OrigIdx: idx, Register: reg, StartCol: 0, ColCount: cols, Rows: 1}
	if cols < 4 {
		p.openRows = append(p.openRows, sigPackerRow{register: reg, nextCol: cols, interp: e.Interp})
	}
	return pe
}

// SigElementInfoForBinding builds a SigElementInfo from a naga binding plus
// its IR type. Mirrors the bind-to-semantic mapping used by
// dxil/internal/emit/emitter.go makeSigInfo and dxil/dxil.go
// bindingToSignatureElements so all 6 signature producers see the same
// classification.
//
// stage and isOutput are required because some bindings change packing class
// based on direction (SV_SampleIndex on PS input is system-managed; on
// nothing else). interpFn is provided so the caller can plug in its own
// interpolation-mode resolver — keeps this package free of any DXIL-emit
// dependency.
func SigElementInfoForBinding(
	irMod *ir.Module,
	binding ir.Binding,
	typeHandle ir.TypeHandle,
	stage ir.ShaderStage,
	isOutput bool,
	interpFn func(loc ir.LocationBinding) SigPackInterp,
) SigElementInfo {
	info := SigElementInfo{Rows: 1, ColCount: 1}
	cols, rows := componentDimensions(irMod, typeHandle)
	info.ColCount = cols
	info.Rows = rows

	switch b := binding.(type) {
	case ir.BuiltinBinding:
		switch b.Builtin {
		case ir.BuiltinPosition:
			return SigElementInfo{
				Kind:     SigPackBuiltinSVPosition,
				ColCount: 4,
				Rows:     1,
			}
		case ir.BuiltinClipDistance:
			if rows < 1 {
				rows = 1
			}
			return SigElementInfo{
				Kind:     SigPackBuiltinScalarArray,
				ColCount: 1,
				Rows:     rows,
			}
		case ir.BuiltinFragDepth, ir.BuiltinSampleMask:
			// PS outputs SV_Depth / SV_Coverage are system-managed (not
			// allocated). On PS input SV_SampleMask never appears (mapped
			// to SV_Coverage which is NotInSig).
			if isOutput && stage == ir.StageFragment {
				return SigElementInfo{
					Kind:     SigPackBuiltinSystemManaged,
					ColCount: 1,
					Rows:     1,
				}
			}
			info.Kind = SigPackBuiltinSystemValue
			return info
		case ir.BuiltinSampleIndex:
			// PS input SV_SampleIndex is system-managed (Shadow interpretation).
			if !isOutput && stage == ir.StageFragment {
				return SigElementInfo{
					Kind:     SigPackBuiltinSystemManaged,
					ColCount: 1,
					Rows:     1,
				}
			}
			info.Kind = SigPackBuiltinSystemValue
			return info
		default:
			info.Kind = SigPackBuiltinSystemValue
			return info
		}
	case ir.LocationBinding:
		// Fragment-stage outputs map to SV_Target which uses Target packing
		// (Register = SemanticIndex). All other location bindings use the
		// standard greedy-first-fit packing.
		if isOutput && stage == ir.StageFragment {
			info.Kind = SigPackTargetOutput
			info.SemanticIdx = b.Location
			if b.BlendSrc != nil {
				// Dual-source blending — second source maps to SV_Target1.
				info.SemanticIdx = *b.BlendSrc
			}
			return info
		}
		info.Kind = SigPackLocation
		if interpFn != nil {
			info.Interp = interpFn(b)
		}
		return info
	default:
		info.Kind = SigPackOther
		return info
	}
}

// PackedMember pairs a struct member's original index with its assigned
// register layout. Length and ordering match SortedMemberIndices for the
// same struct, so callers can iterate as:
//
//	for sigID, pm := range packed {
//	    member := members[pm.OrigIdx]
//	    storeOutput(sigID, pm.Register, pm.StartCol, ...)
//	}
type PackedMember struct {
	PackedElement
	HasBinding bool // false if the member has no binding (caller must skip)
}

// PackStructMembers sorts the struct members in graphics-interface order
// (locations first, builtins last) and returns the packed register layout
// per member in that sorted order. interpFn maps a LocationBinding to its
// DXIL InterpolationMode enum value used for grouping.
//
// stage and isOutput drive system-managed classification (SV_Depth on PS
// output is system-managed; SV_SampleIndex on PS input is system-managed).
//
// isVSInput=true selects "no packing" mode (each element on its own row
// at column 0) per DXIL.rst PackingKind::InputAssembler. All other graphics
// signatures use the standard packed layout.
func PackStructMembers(
	irMod *ir.Module,
	members []ir.StructMember,
	stage ir.ShaderStage,
	isOutput bool,
	isVSInput bool,
	interpFn func(loc ir.LocationBinding) SigPackInterp,
) []PackedMember {
	if len(members) == 0 {
		return nil
	}
	order := SortedMemberIndices(members)
	infos := make([]SigElementInfo, len(order))
	hasBindings := make([]bool, len(order))
	for i, idx := range order {
		m := members[idx]
		if m.Binding == nil {
			infos[i] = SigElementInfo{Kind: SigPackOther}
			continue
		}
		infos[i] = SigElementInfoForBinding(irMod, *m.Binding, m.Type, stage, isOutput, interpFn)
		hasBindings[i] = true
		if isVSInput && infos[i].Kind == SigPackLocation {
			// VSIn uses PackingKind::InputAssembler — each element on its own
			// row at col 0, no packing. Mark the location as system-value-like
			// for packer consumption.
			infos[i].Kind = SigPackBuiltinSystemValue
		}
	}
	packed := PackSignatureElements(infos, !isOutput)
	out := make([]PackedMember, len(order))
	for i := range packed {
		packed[i].OrigIdx = order[i]
		out[i] = PackedMember{
			PackedElement: packed[i],
			HasBinding:    hasBindings[i],
		}
	}
	return out
}

// componentDimensions returns (cols, rows) for an IR type, using the same
// rules as dxil/dxil.go sigCompTypeAndMask and dxil/internal/emit/emitter.go
// describeIRType. Cols = component count of the per-row vector; Rows = number
// of register rows consumed (only > 1 for scalar arrays like ClipDistance).
func componentDimensions(irMod *ir.Module, th ir.TypeHandle) (uint8, uint8) {
	if irMod == nil || int(th) >= len(irMod.Types) {
		return 4, 1
	}
	switch t := irMod.Types[th].Inner.(type) {
	case ir.ScalarType:
		return 1, 1
	case ir.VectorType:
		return uint8(t.Size), 1
	case ir.ArrayType:
		// Scalar arrays (ClipDistance / CullDistance): one component per row,
		// row count = constant array size capped at 4.
		if int(t.Base) >= len(irMod.Types) {
			return 4, 1
		}
		if _, ok := irMod.Types[t.Base].Inner.(ir.ScalarType); !ok {
			return 4, 1
		}
		n := uint8(1)
		if t.Size.Constant != nil {
			c := *t.Size.Constant
			switch {
			case c >= 1 && c <= 4:
				n = uint8(c)
			case c > 4:
				n = 4
			}
		}
		return 1, n
	}
	return 4, 1
}
