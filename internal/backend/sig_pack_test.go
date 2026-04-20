package backend

import (
	"reflect"
	"testing"
)

// TestPackSignatureElements_Empty verifies the nil/empty case.
func TestPackSignatureElements_Empty(t *testing.T) {
	if got := PackSignatureElements(nil, false); got != nil {
		t.Fatalf("PackSignatureElements(nil) = %v, want nil", got)
	}
	if got := PackSignatureElements([]SigElementInfo{}, false); got != nil {
		t.Fatalf("PackSignatureElements(empty) = %v, want nil", got)
	}
}

// TestPackSignatureElements_LocationsSameInterpPack mirrors the f16-native
// test_direct golden — 8 location elements with same interpolation mode pack
// into 6 rows: (1+1+2) (2) (3) (3) (4) (4).
func TestPackSignatureElements_LocationsSameInterpPack(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0}, // LOC 0 scalar
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0}, // LOC 1 scalar
		{Kind: SigPackLocation, ColCount: 2, Rows: 1, Interp: 0}, // LOC 2 vec2
		{Kind: SigPackLocation, ColCount: 2, Rows: 1, Interp: 0}, // LOC 3 vec2
		{Kind: SigPackLocation, ColCount: 3, Rows: 1, Interp: 0}, // LOC 4 vec3
		{Kind: SigPackLocation, ColCount: 3, Rows: 1, Interp: 0}, // LOC 5 vec3
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0}, // LOC 6 vec4
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0}, // LOC 7 vec4
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 1, Register: 0, StartCol: 1, ColCount: 1, Rows: 1},
		{OrigIdx: 2, Register: 0, StartCol: 2, ColCount: 2, Rows: 1},
		{OrigIdx: 3, Register: 1, StartCol: 0, ColCount: 2, Rows: 1},
		{OrigIdx: 4, Register: 2, StartCol: 0, ColCount: 3, Rows: 1},
		{OrigIdx: 5, Register: 3, StartCol: 0, ColCount: 3, Rows: 1},
		{OrigIdx: 6, Register: 4, StartCol: 0, ColCount: 4, Rows: 1},
		{OrigIdx: 7, Register: 5, StartCol: 0, ColCount: 4, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packing mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_DifferentInterpDoesNotPack mirrors the
// interpolate.wgsl golden — different interpolation modes never share a row,
// even when columns would fit. 3 flat scalars pack into row 0; the single
// linear scalar starts a new row 1.
func TestPackSignatureElements_DifferentInterpDoesNotPack(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 1}, // LOC 0 flat u32
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 1}, // LOC 1 flat u32
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 1}, // LOC 2 flat u32
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 4}, // LOC 3 linear f32
		{Kind: SigPackLocation, ColCount: 3, Rows: 1, Interp: 4}, // LOC 7 linear vec3
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 1, Register: 0, StartCol: 1, ColCount: 1, Rows: 1},
		{OrigIdx: 2, Register: 0, StartCol: 2, ColCount: 1, Rows: 1},
		{OrigIdx: 3, Register: 1, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 4, Register: 1, StartCol: 1, ColCount: 3, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interp grouping mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_SVPositionAlone verifies SV_Position consumes a
// full row of 4 components and forces subsequent locations to a new row.
func TestPackSignatureElements_SVPositionAlone(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0}, // LOC 0 scalar
		{Kind: SigPackBuiltinSVPosition, ColCount: 4, Rows: 1},   // SV_Position
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0}, // LOC 1 scalar — must NOT pack into row 0 with LOC 0
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 1, Register: 1, StartCol: 0, ColCount: 4, Rows: 1},
		{OrigIdx: 2, Register: 2, StartCol: 0, ColCount: 1, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SV_Position grouping mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_SystemManaged verifies system-managed PS-stage
// elements get Register=0xFFFFFFFF and do not consume a row counter.
func TestPackSignatureElements_SystemManaged(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0},  // LOC 0 vec4
		{Kind: SigPackBuiltinSystemManaged, ColCount: 1, Rows: 1}, // SV_Depth
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0},  // LOC 1 vec4 — should occupy row 1
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 4, Rows: 1},
		{OrigIdx: 1, Register: 0xFFFFFFFF, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 2, Register: 1, StartCol: 0, ColCount: 4, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("system-managed mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_ScalarArrayMultiRow verifies SV_ClipDistance with
// array<f32, 3> takes 3 contiguous rows starting at col 0 and pushes
// subsequent locations past row 3.
func TestPackSignatureElements_ScalarArrayMultiRow(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackBuiltinScalarArray, ColCount: 1, Rows: 3}, // ClipDistance[3]
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0},
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 1, Rows: 3},
		{OrigIdx: 1, Register: 3, StartCol: 0, ColCount: 4, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scalar array mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_BuiltinSystemValueAlone verifies a non-position
// system-value builtin (VertexID etc.) gets its own row.
func TestPackSignatureElements_BuiltinSystemValueAlone(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackBuiltinSystemValue, ColCount: 1, Rows: 1}, // SV_VertexID
		{Kind: SigPackBuiltinSystemValue, ColCount: 1, Rows: 1}, // SV_InstanceID
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 1, Register: 1, StartCol: 0, ColCount: 1, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("builtin sysvalue mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_FullRowWithVec4 verifies a 4-column element fills
// the row exactly and the next element starts in a new row.
func TestPackSignatureElements_FullRowWithVec4(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0},
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0},
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 4, Rows: 1},
		{OrigIdx: 1, Register: 1, StartCol: 0, ColCount: 1, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vec4-then-scalar mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_LinearAfterFlatStartsNewRow verifies that an
// already-open flat row is not reused for a linear element, even when it has
// room (gap left in the flat row stays unused — DXC's PrefixStable behavior).
func TestPackSignatureElements_LinearAfterFlatStartsNewRow(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 2, Rows: 1, Interp: 1}, // flat xy
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 4}, // linear x — would fit at col 2 of row 0 but interp differs
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 2, Rows: 1},
		{OrigIdx: 1, Register: 1, StartCol: 0, ColCount: 1, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interp-aware row-open mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_TargetOutputUsesSemanticIndex verifies SV_Target
// outputs use Register = SemanticIndex (DXIL.rst PackingKind::Target). No
// packing across targets — each gets its own row.
func TestPackSignatureElements_TargetOutputUsesSemanticIndex(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackTargetOutput, ColCount: 1, Rows: 1, SemanticIdx: 0}, // SV_Target0 scalar
		{Kind: SigPackTargetOutput, ColCount: 1, Rows: 1, SemanticIdx: 1}, // SV_Target1 scalar
		{Kind: SigPackTargetOutput, ColCount: 2, Rows: 1, SemanticIdx: 2}, // SV_Target2 vec2
		{Kind: SigPackTargetOutput, ColCount: 4, Rows: 1, SemanticIdx: 7}, // SV_Target7 vec4
	}
	want := []PackedElement{
		{OrigIdx: 0, Register: 0, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 1, Register: 1, StartCol: 0, ColCount: 1, Rows: 1},
		{OrigIdx: 2, Register: 2, StartCol: 0, ColCount: 2, Rows: 1},
		{OrigIdx: 3, Register: 7, StartCol: 0, ColCount: 4, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SV_Target packing mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_OtherBindingPlaceholder verifies SigPackOther
// produces a zero-rows placeholder so downstream slice indexing works.
func TestPackSignatureElements_OtherBindingPlaceholder(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0},
		{Kind: SigPackOther},
		{Kind: SigPackLocation, ColCount: 4, Rows: 1, Interp: 0},
	}
	got := PackSignatureElements(in, false)
	if len(got) != 3 {
		t.Fatalf("got %d packed elements, want 3", len(got))
	}
	if got[0].Register != 0 || got[2].Register != 1 {
		t.Fatalf("placeholder consumed a row: got registers [%d, %d, %d]",
			got[0].Register, got[1].Register, got[2].Register)
	}
	if got[1].Rows != 0 {
		t.Fatalf("placeholder Rows = %d, want 0 (no row consumed)", got[1].Rows)
	}
}
