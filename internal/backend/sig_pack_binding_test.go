package backend

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// buildModule creates a minimal ir.Module with the given types.
func buildModule(types ...ir.TypeInner) *ir.Module {
	m := &ir.Module{
		Types: make([]ir.Type, len(types)),
	}
	for i, inner := range types {
		m.Types[i] = ir.Type{Inner: inner}
	}
	return m
}

// ---------------------------------------------------------------------------
// SigElementInfoForBinding — binding-to-packing-class mapping
//
// This function drives 6 signature producers (OSG1, ISG1, PSV0, metadata,
// storeOutput, loadInput). A wrong classification means the DXIL container
// has inconsistent signatures and IDxcValidator rejects it.
// ---------------------------------------------------------------------------

// TestSigElementInfoForBinding_AllBuiltinClasses verifies that each SV_*
// builtin maps to the correct SigPackKind. DXC allocates registers
// differently per class; wrong classification = wrong register = validator error.
func TestSigElementInfoForBinding_AllBuiltinClasses(t *testing.T) {
	mod := buildModule(
		ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // type 0: vec4<f32>
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},                                       // type 1: f32
		ir.ScalarType{Kind: ir.ScalarUint, Width: 4},                                        // type 2: u32
	)

	tests := []struct {
		name     string
		builtin  ir.BuiltinValue
		th       ir.TypeHandle
		stage    ir.ShaderStage
		isOutput bool
		wantKind SigPackKind
	}{
		// SV_Position: always 4-component own row.
		{"SV_Position VS out", ir.BuiltinPosition, 0, ir.StageVertex, true, SigPackBuiltinSVPosition},
		{"SV_Position PS in", ir.BuiltinPosition, 0, ir.StageFragment, false, SigPackBuiltinSVPosition},

		// SV_Depth (FragDepth): system-managed ONLY when PS output.
		{"SV_Depth PS out", ir.BuiltinFragDepth, 1, ir.StageFragment, true, SigPackBuiltinSystemManaged},
		{"SV_Depth non-PS", ir.BuiltinFragDepth, 1, ir.StageVertex, true, SigPackBuiltinSystemValue},

		// SV_Coverage (SampleMask): system-managed ONLY when PS output.
		{"SV_Coverage PS out", ir.BuiltinSampleMask, 2, ir.StageFragment, true, SigPackBuiltinSystemManaged},
		{"SV_Coverage non-PS", ir.BuiltinSampleMask, 2, ir.StageVertex, false, SigPackBuiltinSystemValue},

		// SV_SampleIndex: system-managed ONLY when PS input (Shadow interp).
		{"SV_SampleIndex PS in", ir.BuiltinSampleIndex, 2, ir.StageFragment, false, SigPackBuiltinSystemManaged},
		{"SV_SampleIndex VS in", ir.BuiltinSampleIndex, 2, ir.StageVertex, false, SigPackBuiltinSystemValue},
		{"SV_SampleIndex PS out", ir.BuiltinSampleIndex, 2, ir.StageFragment, true, SigPackBuiltinSystemValue},

		// Other builtins: generic system-value.
		{"SV_VertexID", ir.BuiltinVertexIndex, 2, ir.StageVertex, false, SigPackBuiltinSystemValue},
		{"SV_InstanceID", ir.BuiltinInstanceIndex, 2, ir.StageVertex, false, SigPackBuiltinSystemValue},
		{"SV_IsFrontFace", ir.BuiltinFrontFacing, 2, ir.StageFragment, false, SigPackBuiltinSystemValue},
		{"SV_GlobalInvocationID", ir.BuiltinGlobalInvocationID, 0, ir.StageCompute, false, SigPackBuiltinSystemValue},
		{"SV_LocalInvocationID", ir.BuiltinLocalInvocationID, 0, ir.StageCompute, false, SigPackBuiltinSystemValue},
		{"SV_WorkGroupID", ir.BuiltinWorkGroupID, 0, ir.StageCompute, false, SigPackBuiltinSystemValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: tt.builtin}, tt.th, tt.stage, tt.isOutput, nil)
			if info.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", info.Kind, tt.wantKind)
			}
		})
	}
}

// TestSigElementInfoForBinding_ClipDistance verifies SV_ClipDistance uses
// ScalarArray packing with rows = array size capped at 4.
func TestSigElementInfoForBinding_ClipDistance(t *testing.T) {
	three := uint32(3)
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &three}},
	)
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinClipDistance}, 1, ir.StageVertex, true, nil)
	if info.Kind != SigPackBuiltinScalarArray {
		t.Fatalf("Kind = %v, want SigPackBuiltinScalarArray", info.Kind)
	}
	if info.ColCount != 1 {
		t.Errorf("ColCount = %d, want 1 (scalar per row)", info.ColCount)
	}
	if info.Rows != 3 {
		t.Errorf("Rows = %d, want 3 (array size)", info.Rows)
	}
}

// TestSigElementInfoForBinding_LocationFragTarget verifies that fragment
// shader @location outputs map to SV_Target with Register = SemanticIndex.
func TestSigElementInfoForBinding_LocationFragTarget(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})

	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 2}, 0, ir.StageFragment, true, nil)
	if info.Kind != SigPackTargetOutput {
		t.Fatalf("Kind = %v, want SigPackTargetOutput", info.Kind)
	}
	if info.SemanticIdx != 2 {
		t.Errorf("SemanticIdx = %d, want 2 (from Location)", info.SemanticIdx)
	}
	if info.ColCount != 4 {
		t.Errorf("ColCount = %d, want 4 (vec4)", info.ColCount)
	}
}

// TestSigElementInfoForBinding_DualSourceBlending verifies that when
// BlendSrc is set, SemanticIdx comes from BlendSrc not Location.
func TestSigElementInfoForBinding_DualSourceBlending(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	blendSrc := uint32(1)
	binding := ir.LocationBinding{Location: 0, BlendSrc: &blendSrc}

	info := SigElementInfoForBinding(mod, binding, 0, ir.StageFragment, true, nil)
	if info.SemanticIdx != 1 {
		t.Errorf("dual-source SemanticIdx = %d, want 1 (from BlendSrc)", info.SemanticIdx)
	}
}

// TestSigElementInfoForBinding_LocationWithInterp verifies that non-fragment
// location bindings call interpFn and preserve its result.
func TestSigElementInfoForBinding_LocationWithInterp(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	called := false
	interpFn := func(loc ir.LocationBinding) SigPackInterp {
		called = true
		if loc.Location != 3 {
			t.Errorf("interpFn received Location=%d, want 3", loc.Location)
		}
		return 5
	}

	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 3}, 0, ir.StageVertex, true, interpFn)
	if !called {
		t.Fatal("interpFn was not called for location binding")
	}
	if info.Kind != SigPackLocation {
		t.Errorf("Kind = %v, want SigPackLocation", info.Kind)
	}
	if info.Interp != 5 {
		t.Errorf("Interp = %d, want 5", info.Interp)
	}
	if info.ColCount != 2 {
		t.Errorf("ColCount = %d, want 2 (vec2)", info.ColCount)
	}
}

// ---------------------------------------------------------------------------
// componentDimensions — type -> (cols, rows) for register allocation
// ---------------------------------------------------------------------------

// TestComponentDimensions verifies the type-to-register-dimensions mapping
// that feeds all 6 DXIL signature producers. Wrong dimensions = wrong
// component mask = validator error or GPU corruption.
func TestComponentDimensions(t *testing.T) {
	tests := []struct {
		name     string
		types    []ir.TypeInner
		th       ir.TypeHandle
		wantCols uint8
		wantRows uint8
	}{
		{
			name:     "f32 scalar -> 1 col 1 row",
			types:    []ir.TypeInner{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			th:       0,
			wantCols: 1, wantRows: 1,
		},
		{
			name:     "vec2<f32> -> 2 cols 1 row",
			types:    []ir.TypeInner{ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			th:       0,
			wantCols: 2, wantRows: 1,
		},
		{
			name:     "vec3<f32> -> 3 cols 1 row",
			types:    []ir.TypeInner{ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			th:       0,
			wantCols: 3, wantRows: 1,
		},
		{
			name:     "vec4<f32> -> 4 cols 1 row",
			types:    []ir.TypeInner{ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			th:       0,
			wantCols: 4, wantRows: 1,
		},
		{
			name: "array<f32, 3> -> 1 col 3 rows (ClipDistance)",
			types: func() []ir.TypeInner {
				three := uint32(3)
				return []ir.TypeInner{
					ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
					ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &three}},
				}
			}(),
			th:       1,
			wantCols: 1, wantRows: 3,
		},
		{
			name: "array<f32, 8> -> capped at 4 rows",
			types: func() []ir.TypeInner {
				eight := uint32(8)
				return []ir.TypeInner{
					ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
					ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &eight}},
				}
			}(),
			th:       1,
			wantCols: 1, wantRows: 4,
		},
		{
			name: "array<vec3, 2> -> non-scalar base defaults to 4/1",
			types: func() []ir.TypeInner {
				two := uint32(2)
				return []ir.TypeInner{
					ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
					ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &two}},
				}
			}(),
			th:       1,
			wantCols: 4, wantRows: 1,
		},
		{
			name: "array with out-of-bounds base -> defaults to 4/1",
			types: func() []ir.TypeInner {
				two := uint32(2)
				return []ir.TypeInner{
					ir.ArrayType{Base: 99, Size: ir.ArraySize{Constant: &two}},
				}
			}(),
			th:       0,
			wantCols: 4, wantRows: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := buildModule(tt.types...)
			// Use a simple location binding to exercise componentDimensions
			// through SigElementInfoForBinding (componentDimensions is unexported).
			info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, tt.th, ir.StageVertex, true, nil)
			if info.ColCount != tt.wantCols || info.Rows != tt.wantRows {
				t.Errorf("got cols=%d rows=%d, want cols=%d rows=%d",
					info.ColCount, info.Rows, tt.wantCols, tt.wantRows)
			}
		})
	}
}

// TestComponentDimensions_NilModule verifies safe default for nil module.
func TestComponentDimensions_NilModule(t *testing.T) {
	info := SigElementInfoForBinding(nil, ir.LocationBinding{Location: 0}, 0, ir.StageVertex, true, nil)
	if info.ColCount != 4 || info.Rows != 1 {
		t.Errorf("nil module: cols=%d rows=%d, want 4/1", info.ColCount, info.Rows)
	}
}

// ---------------------------------------------------------------------------
// PackStructMembers — the full pipeline: sort + classify + pack
// ---------------------------------------------------------------------------

// TestPackStructMembers_VertexShaderOutput simulates a typical VS output
// struct with position builtin and location varyings. Verifies the entire
// sort+classify+pack pipeline produces the correct register assignment
// that OSG1, metadata, and storeOutput must agree on.
func TestPackStructMembers_VertexShaderOutput(t *testing.T) {
	mod := buildModule(
		ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: vec4<f32>
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},                                       // 1: f32
		ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 2: vec2<f32>
	)
	// Struct declared as: position first, then locations (reverse order).
	members := []ir.StructMember{
		{Name: "pos", Type: 0, Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
		{Name: "uv", Type: 2, Binding: bindingPtr(ir.LocationBinding{Location: 1})},
		{Name: "brightness", Type: 1, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, true, false, nil)
	if len(packed) != 3 {
		t.Fatalf("len = %d, want 3", len(packed))
	}

	// After interface sort: brightness(loc 0), uv(loc 1), pos(builtin).
	// Packing: loc 0 (f32, 1 col) -> row 0 col 0
	//          loc 1 (vec2, 2 cols) -> row 0 col 1 (packs with loc 0)
	//          SV_Position (4 cols) -> row 1
	if packed[0].OrigIdx != 2 {
		t.Errorf("packed[0] should be brightness (OrigIdx=2), got %d", packed[0].OrigIdx)
	}
	if packed[0].Register != 0 || packed[0].StartCol != 0 || packed[0].ColCount != 1 {
		t.Errorf("brightness: Register=%d StartCol=%d ColCount=%d, want 0/0/1",
			packed[0].Register, packed[0].StartCol, packed[0].ColCount)
	}
	if packed[1].OrigIdx != 1 {
		t.Errorf("packed[1] should be uv (OrigIdx=1), got %d", packed[1].OrigIdx)
	}
	if packed[1].Register != 0 || packed[1].StartCol != 1 || packed[1].ColCount != 2 {
		t.Errorf("uv: Register=%d StartCol=%d ColCount=%d, want 0/1/2",
			packed[1].Register, packed[1].StartCol, packed[1].ColCount)
	}
	if packed[2].OrigIdx != 0 {
		t.Errorf("packed[2] should be pos (OrigIdx=0), got %d", packed[2].OrigIdx)
	}
	if packed[2].Register != 1 || packed[2].ColCount != 4 {
		t.Errorf("pos: Register=%d ColCount=%d, want 1/4",
			packed[2].Register, packed[2].ColCount)
	}
}

// TestPackStructMembers_VSInputNoPacking verifies that VS input uses
// PackingKind::InputAssembler (no packing, each element own row).
func TestPackStructMembers_VSInputNoPacking(t *testing.T) {
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
	)
	members := []ir.StructMember{
		{Name: "a", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
		{Name: "b", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 1})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, false, true, nil)
	if len(packed) != 2 {
		t.Fatalf("len = %d, want 2", len(packed))
	}
	// Two f32 scalars would normally pack into row 0 cols 0-1.
	// With VSInput, each gets its own row.
	if packed[0].Register == packed[1].Register {
		t.Errorf("VSInput: both on row %d, expected separate rows", packed[0].Register)
	}
}

// TestPackStructMembers_NilBinding verifies that members without bindings
// get HasBinding=false so callers can skip them in signature iteration.
func TestPackStructMembers_NilBinding(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	members := []ir.StructMember{
		{Name: "noBinding", Type: 0, Binding: nil},
		{Name: "loc", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, true, false, nil)

	// Find the nil-binding member.
	for _, pm := range packed {
		if pm.OrigIdx == 0 && pm.HasBinding {
			t.Error("nil-binding member should have HasBinding=false")
		}
		if pm.OrigIdx == 1 && !pm.HasBinding {
			t.Error("location member should have HasBinding=true")
		}
	}
}

func TestPackStructMembers_Empty(t *testing.T) {
	if got := PackStructMembers(nil, nil, ir.StageVertex, true, false, nil); got != nil {
		t.Fatalf("PackStructMembers(nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// PackSignatureElements — additional edge cases
// ---------------------------------------------------------------------------

// TestPackSignatureElements_ZeroColCountLocation verifies that packLocation
// treats ColCount=0 as 1 (defensive default).
func TestPackSignatureElements_ZeroColCountLocation(t *testing.T) {
	got := PackSignatureElements([]SigElementInfo{
		{Kind: SigPackLocation, ColCount: 0, Rows: 1, Interp: 0},
	}, false)
	if got[0].ColCount != 1 {
		t.Errorf("ColCount=0 should be treated as 1, got %d", got[0].ColCount)
	}
}

// TestPackSignatureElements_ScalarArrayZeroRows verifies Rows=0 defaults to 1.
func TestPackSignatureElements_ScalarArrayZeroRows(t *testing.T) {
	got := PackSignatureElements([]SigElementInfo{
		{Kind: SigPackBuiltinScalarArray, ColCount: 1, Rows: 0},
	}, false)
	if got[0].Rows != 1 {
		t.Errorf("Rows=0 should default to 1, got %d", got[0].Rows)
	}
}

// TestPackSignatureElements_TargetOutputAdvancesNextRow verifies that
// SV_Target with high semantic index correctly advances the row counter
// for subsequent elements.
func TestPackSignatureElements_TargetOutputAdvancesNextRow(t *testing.T) {
	got := PackSignatureElements([]SigElementInfo{
		{Kind: SigPackTargetOutput, ColCount: 4, Rows: 1, SemanticIdx: 5},
		{Kind: SigPackBuiltinSVPosition, ColCount: 4, Rows: 1},
	}, false)
	want := []PackedElement{
		{OrigIdx: 0, Register: 5, StartCol: 0, ColCount: 4, Rows: 1},
		{OrigIdx: 1, Register: 6, StartCol: 0, ColCount: 4, Rows: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("target(5) then position:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestPackSignatureElements_BuiltinClearsOpenRows verifies that placing
// a builtin (SV_Position, SV_VertexID, etc.) invalidates open rows from
// previous location packing — DXC PackPrefixStable behavior.
func TestPackSignatureElements_BuiltinClearsOpenRows(t *testing.T) {
	got := PackSignatureElements([]SigElementInfo{
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0}, // row 0 col 0, row open
		{Kind: SigPackBuiltinSVPosition, ColCount: 4, Rows: 1},   // row 1, clears open rows
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0}, // must be row 2, NOT row 0
	}, false)
	if got[2].Register != 2 {
		t.Errorf("location after SV_Position should be row 2, got %d (row 0 leak)", got[2].Register)
	}
}
