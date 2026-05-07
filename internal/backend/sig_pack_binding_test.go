package backend

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------- componentDimensions (via SigElementInfoForBinding) ----------

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

func TestComponentDimensions_Scalar(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, 0, ir.StageVertex, true, nil)
	if info.ColCount != 1 || info.Rows != 1 {
		t.Errorf("scalar: ColCount=%d Rows=%d, want 1/1", info.ColCount, info.Rows)
	}
}

func TestComponentDimensions_Vector(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, 0, ir.StageVertex, true, nil)
	if info.ColCount != 3 || info.Rows != 1 {
		t.Errorf("vec3: ColCount=%d Rows=%d, want 3/1", info.ColCount, info.Rows)
	}
}

func TestComponentDimensions_ArrayScalar(t *testing.T) {
	// array<f32, 3> — used for ClipDistance: 1 col, 3 rows.
	three := uint32(3)
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},             // type 0: f32
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &three}}, // type 1: array<f32,3>
	)
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinClipDistance}, 1, ir.StageVertex, true, nil)
	if info.ColCount != 1 || info.Rows != 3 {
		t.Errorf("array<f32,3>: ColCount=%d Rows=%d, want 1/3", info.ColCount, info.Rows)
	}
}

func TestComponentDimensions_ArrayScalarCappedAt4(t *testing.T) {
	// array<f32, 8> — capped at 4 rows.
	eight := uint32(8)
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &eight}},
	)
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinClipDistance}, 1, ir.StageVertex, true, nil)
	if info.Rows != 4 {
		t.Errorf("array<f32,8> capped: Rows=%d, want 4", info.Rows)
	}
}

func TestComponentDimensions_ArrayScalarZeroSize(t *testing.T) {
	// array<f32> with Constant=nil (runtime-sized) — defaults to 1 row.
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}},
	)
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinClipDistance}, 1, ir.StageVertex, true, nil)
	if info.Rows < 1 {
		t.Errorf("runtime-sized array: Rows=%d, want >=1", info.Rows)
	}
}

func TestComponentDimensions_ArrayNonScalarBase(t *testing.T) {
	// array<vec3, 2> — non-scalar base, falls through to default 4 cols.
	two := uint32(2)
	mod := buildModule(
		ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &two}},
	)
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, 1, ir.StageVertex, true, nil)
	if info.ColCount != 4 {
		t.Errorf("array<vec3,2> non-scalar base: ColCount=%d, want 4 (default)", info.ColCount)
	}
}

func TestComponentDimensions_ArrayOutOfBoundsBase(t *testing.T) {
	// Array with Base pointing beyond module types.
	two := uint32(2)
	mod := buildModule(
		ir.ArrayType{Base: 99, Size: ir.ArraySize{Constant: &two}}, // base 99 out of bounds
	)
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, 0, ir.StageVertex, true, nil)
	if info.ColCount != 4 {
		t.Errorf("out-of-bounds base: ColCount=%d, want 4 (default)", info.ColCount)
	}
}

func TestComponentDimensions_NilModule(t *testing.T) {
	info := SigElementInfoForBinding(nil, ir.LocationBinding{Location: 0}, 0, ir.StageVertex, true, nil)
	if info.ColCount != 4 || info.Rows != 1 {
		t.Errorf("nil module: ColCount=%d Rows=%d, want 4/1", info.ColCount, info.Rows)
	}
}

func TestComponentDimensions_TypeHandleOutOfBounds(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, 99, ir.StageVertex, true, nil)
	if info.ColCount != 4 || info.Rows != 1 {
		t.Errorf("out-of-bounds handle: ColCount=%d Rows=%d, want 4/1", info.ColCount, info.Rows)
	}
}

// ---------- SigElementInfoForBinding ----------

func TestSigElementInfoForBinding_BuiltinPosition(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, 0, ir.StageVertex, true, nil)
	if info.Kind != SigPackBuiltinSVPosition {
		t.Errorf("position Kind = %v, want SigPackBuiltinSVPosition", info.Kind)
	}
	if info.ColCount != 4 {
		t.Errorf("position ColCount = %d, want 4", info.ColCount)
	}
}

func TestSigElementInfoForBinding_BuiltinClipDistance(t *testing.T) {
	two := uint32(2)
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &two}},
	)
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinClipDistance}, 1, ir.StageVertex, true, nil)
	if info.Kind != SigPackBuiltinScalarArray {
		t.Errorf("clip distance Kind = %v, want SigPackBuiltinScalarArray", info.Kind)
	}
	if info.ColCount != 1 || info.Rows != 2 {
		t.Errorf("clip distance ColCount=%d Rows=%d, want 1/2", info.ColCount, info.Rows)
	}
}

func TestSigElementInfoForBinding_FragDepthPSOutput(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinFragDepth}, 0, ir.StageFragment, true, nil)
	if info.Kind != SigPackBuiltinSystemManaged {
		t.Errorf("frag depth PS output Kind = %v, want SigPackBuiltinSystemManaged", info.Kind)
	}
}

func TestSigElementInfoForBinding_FragDepthNonPSOutput(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinFragDepth}, 0, ir.StageVertex, true, nil)
	if info.Kind != SigPackBuiltinSystemValue {
		t.Errorf("frag depth non-PS Kind = %v, want SigPackBuiltinSystemValue", info.Kind)
	}
}

func TestSigElementInfoForBinding_SampleMaskPSOutput(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinSampleMask}, 0, ir.StageFragment, true, nil)
	if info.Kind != SigPackBuiltinSystemManaged {
		t.Errorf("sample mask PS output Kind = %v, want SigPackBuiltinSystemManaged", info.Kind)
	}
}

func TestSigElementInfoForBinding_SampleMaskNonPSOutput(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinSampleMask}, 0, ir.StageVertex, false, nil)
	if info.Kind != SigPackBuiltinSystemValue {
		t.Errorf("sample mask non-PS Kind = %v, want SigPackBuiltinSystemValue", info.Kind)
	}
}

func TestSigElementInfoForBinding_SampleIndexPSInput(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinSampleIndex}, 0, ir.StageFragment, false, nil)
	if info.Kind != SigPackBuiltinSystemManaged {
		t.Errorf("sample index PS input Kind = %v, want SigPackBuiltinSystemManaged", info.Kind)
	}
}

func TestSigElementInfoForBinding_SampleIndexNonPSInput(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinSampleIndex}, 0, ir.StageVertex, false, nil)
	if info.Kind != SigPackBuiltinSystemValue {
		t.Errorf("sample index non-PS Kind = %v, want SigPackBuiltinSystemValue", info.Kind)
	}
}

func TestSigElementInfoForBinding_OtherBuiltin(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex}, 0, ir.StageVertex, false, nil)
	if info.Kind != SigPackBuiltinSystemValue {
		t.Errorf("vertex index Kind = %v, want SigPackBuiltinSystemValue", info.Kind)
	}
}

func TestSigElementInfoForBinding_LocationFragOutput_Target(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 2}, 0, ir.StageFragment, true, nil)
	if info.Kind != SigPackTargetOutput {
		t.Errorf("location frag output Kind = %v, want SigPackTargetOutput", info.Kind)
	}
	if info.SemanticIdx != 2 {
		t.Errorf("location frag output SemanticIdx = %d, want 2", info.SemanticIdx)
	}
}

func TestSigElementInfoForBinding_LocationFragOutput_BlendSrc(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	blendSrc := uint32(1)
	binding := ir.LocationBinding{Location: 0, BlendSrc: &blendSrc}
	info := SigElementInfoForBinding(mod, binding, 0, ir.StageFragment, true, nil)
	if info.Kind != SigPackTargetOutput {
		t.Errorf("blend src Kind = %v, want SigPackTargetOutput", info.Kind)
	}
	if info.SemanticIdx != 1 {
		t.Errorf("blend src SemanticIdx = %d, want 1 (from BlendSrc)", info.SemanticIdx)
	}
}

func TestSigElementInfoForBinding_LocationNonFragOutput(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	called := false
	interpFn := func(_ ir.LocationBinding) SigPackInterp {
		called = true
		return 5
	}
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 3}, 0, ir.StageVertex, true, interpFn)
	if info.Kind != SigPackLocation {
		t.Errorf("location vertex output Kind = %v, want SigPackLocation", info.Kind)
	}
	if !called {
		t.Error("interpFn was not called for location binding")
	}
	if info.Interp != 5 {
		t.Errorf("Interp = %d, want 5", info.Interp)
	}
	if info.ColCount != 2 {
		t.Errorf("ColCount = %d, want 2 (vec2)", info.ColCount)
	}
}

func TestSigElementInfoForBinding_LocationNilInterpFn(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	info := SigElementInfoForBinding(mod, ir.LocationBinding{Location: 0}, 0, ir.StageVertex, true, nil)
	if info.Kind != SigPackLocation {
		t.Errorf("Kind = %v, want SigPackLocation", info.Kind)
	}
	if info.Interp != 0 {
		t.Errorf("nil interpFn should leave Interp=0, got %d", info.Interp)
	}
}

// ---------- PackStructMembers ----------

func TestPackStructMembers_Empty(t *testing.T) {
	if got := PackStructMembers(nil, nil, ir.StageVertex, true, false, nil); got != nil {
		t.Fatalf("PackStructMembers(nil) = %v, want nil", got)
	}
}

func TestPackStructMembers_SingleLocation(t *testing.T) {
	mod := buildModule(ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	members := []ir.StructMember{
		{Name: "color", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, true, false, nil)
	if len(packed) != 1 {
		t.Fatalf("len = %d, want 1", len(packed))
	}
	if !packed[0].HasBinding {
		t.Error("HasBinding should be true")
	}
	if packed[0].OrigIdx != 0 {
		t.Errorf("OrigIdx = %d, want 0", packed[0].OrigIdx)
	}
	if packed[0].ColCount != 4 {
		t.Errorf("ColCount = %d, want 4 (vec4)", packed[0].ColCount)
	}
}

func TestPackStructMembers_LocationsSortedAndPacked(t *testing.T) {
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},                                         // type 0: f32
		ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},   // type 1: vec3<f32>
	)
	// Members out of order by location.
	members := []ir.StructMember{
		{Name: "b", Type: 1, Binding: bindingPtr(ir.LocationBinding{Location: 1})},
		{Name: "a", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, true, false, nil)
	if len(packed) != 2 {
		t.Fatalf("len = %d, want 2", len(packed))
	}
	// After sorting: loc(0) first (orig idx 1), then loc(1) (orig idx 0).
	if packed[0].OrigIdx != 1 {
		t.Errorf("packed[0].OrigIdx = %d, want 1 (loc 0)", packed[0].OrigIdx)
	}
	if packed[1].OrigIdx != 0 {
		t.Errorf("packed[1].OrigIdx = %d, want 0 (loc 1)", packed[1].OrigIdx)
	}
}

func TestPackStructMembers_NilBinding(t *testing.T) {
	mod := buildModule(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	members := []ir.StructMember{
		{Name: "noBinding", Type: 0, Binding: nil},
		{Name: "loc", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, true, false, nil)
	if len(packed) != 2 {
		t.Fatalf("len = %d, want 2", len(packed))
	}
	// Location should come first after sort.
	var locPacked, nilPacked *PackedMember
	for i := range packed {
		if packed[i].HasBinding {
			locPacked = &packed[i]
		} else {
			nilPacked = &packed[i]
		}
	}
	if locPacked == nil {
		t.Fatal("missing location PackedMember")
	}
	if nilPacked == nil {
		t.Fatal("missing nil-binding PackedMember")
	}
	if nilPacked.HasBinding {
		t.Error("nil-binding member should have HasBinding=false")
	}
}

func TestPackStructMembers_VSInputNoPacking(t *testing.T) {
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, // type 0: f32
	)
	// Two scalars that would pack into one row in normal mode.
	members := []ir.StructMember{
		{Name: "a", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
		{Name: "b", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 1})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, false, true, nil)
	if len(packed) != 2 {
		t.Fatalf("len = %d, want 2", len(packed))
	}
	// VSInput: each element on its own row (Kind remapped to SigPackBuiltinSystemValue).
	if packed[0].Register == packed[1].Register {
		t.Errorf("VSInput: both elements on Register %d, expected different rows", packed[0].Register)
	}
}

func TestPackStructMembers_BuiltinAndLocation(t *testing.T) {
	mod := buildModule(
		ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // type 0: vec4<f32>
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},                                        // type 1: f32
	)
	members := []ir.StructMember{
		{Name: "pos", Type: 0, Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
		{Name: "uv", Type: 1, Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	packed := PackStructMembers(mod, members, ir.StageVertex, true, false, nil)
	if len(packed) != 2 {
		t.Fatalf("len = %d, want 2", len(packed))
	}
	// After interface sort: location first (uv), then builtin (pos).
	// packed[0] = uv (location), packed[1] = pos (builtin SV_Position).
	if packed[0].OrigIdx != 1 {
		t.Errorf("packed[0] should be uv (OrigIdx=1), got %d", packed[0].OrigIdx)
	}
	if packed[1].OrigIdx != 0 {
		t.Errorf("packed[1] should be pos (OrigIdx=0), got %d", packed[1].OrigIdx)
	}
}

// TestPackSignatureElements_ZeroColCountLocation verifies that packLocation
// treats ColCount=0 as 1.
func TestPackSignatureElements_ZeroColCountLocation(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 0, Rows: 1, Interp: 0},
	}
	got := PackSignatureElements(in, false)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ColCount != 1 {
		t.Errorf("ColCount=0 should be treated as 1, got %d", got[0].ColCount)
	}
}

// TestPackSignatureElements_ScalarArrayZeroRows verifies that
// SigPackBuiltinScalarArray with Rows=0 defaults to 1 row.
func TestPackSignatureElements_ScalarArrayZeroRows(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackBuiltinScalarArray, ColCount: 1, Rows: 0},
	}
	got := PackSignatureElements(in, false)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Rows != 1 {
		t.Errorf("Rows=0 should default to 1, got %d", got[0].Rows)
	}
}

// TestPackSignatureElements_MixedKinds verifies a realistic mix of all kinds.
func TestPackSignatureElements_MixedKinds(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackLocation, ColCount: 2, Rows: 1, Interp: 0},     // loc vec2
		{Kind: SigPackBuiltinSVPosition, ColCount: 4, Rows: 1},       // SV_Position
		{Kind: SigPackLocation, ColCount: 1, Rows: 1, Interp: 0},     // loc scalar
		{Kind: SigPackBuiltinSystemValue, ColCount: 1, Rows: 1},      // SV_VertexID
		{Kind: SigPackBuiltinSystemManaged, ColCount: 1, Rows: 1},    // SV_Depth
		{Kind: SigPackTargetOutput, ColCount: 4, Rows: 1, SemanticIdx: 0}, // SV_Target0
	}
	got := PackSignatureElements(in, false)
	if len(got) != 6 {
		t.Fatalf("len = %d, want 6", len(got))
	}
	// Location vec2 at row 0.
	if got[0].Register != 0 || got[0].StartCol != 0 || got[0].ColCount != 2 {
		t.Errorf("loc vec2: %+v", got[0])
	}
	// SV_Position at row 1 (clears open rows).
	if got[1].Register != 1 || got[1].ColCount != 4 {
		t.Errorf("SV_Position: %+v", got[1])
	}
	// Second location scalar at row 2 (open rows cleared by SV_Position).
	if got[2].Register != 2 || got[2].StartCol != 0 || got[2].ColCount != 1 {
		t.Errorf("loc scalar: %+v", got[2])
	}
	// SV_VertexID at row 3.
	if got[3].Register != 3 || got[3].ColCount != 1 {
		t.Errorf("SV_VertexID: %+v", got[3])
	}
	// SV_Depth system-managed.
	if got[4].Register != 0xFFFFFFFF {
		t.Errorf("SV_Depth: Register=%d, want 0xFFFFFFFF", got[4].Register)
	}
	// SV_Target0 at register 0 (semantic index).
	if got[5].Register != 0 || got[5].ColCount != 4 {
		t.Errorf("SV_Target0: %+v", got[5])
	}
}

// TestPackSignatureElements_TargetOutputAdvancesNextRow verifies that
// SV_Target with a high semantic index advances nextRow properly.
func TestPackSignatureElements_TargetOutputAdvancesNextRow(t *testing.T) {
	in := []SigElementInfo{
		{Kind: SigPackTargetOutput, ColCount: 4, Rows: 1, SemanticIdx: 5},
		{Kind: SigPackBuiltinSVPosition, ColCount: 4, Rows: 1},
	}
	got := PackSignatureElements(in, false)
	want := []PackedElement{
		{OrigIdx: 0, Register: 5, StartCol: 0, ColCount: 4, Rows: 1},
		{OrigIdx: 1, Register: 6, StartCol: 0, ColCount: 4, Rows: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("target then position:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestComponentDimensions_ArrayScalarSize1(t *testing.T) {
	one := uint32(1)
	mod := buildModule(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &one}},
	)
	info := SigElementInfoForBinding(mod, ir.BuiltinBinding{Builtin: ir.BuiltinClipDistance}, 1, ir.StageVertex, true, nil)
	if info.Rows != 1 {
		t.Errorf("array<f32,1> Rows=%d, want 1", info.Rows)
	}
}
