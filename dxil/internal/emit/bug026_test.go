package emit

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// Unit tests for BUG-DXIL-026 helpers.
//
// End-to-end validation is covered by snapshot.TestDxilValGGProduction
// (57 production gg entry points) and snapshot.TestDxilValSummary (170
// baseline shaders). These unit tests pin down the pure-function
// contracts of the new helpers so future refactors cannot silently
// regress the decomposition logic.

// --- flattenScalarFields --------------------------------------------------

func TestFlattenScalarFields_Scalar(t *testing.T) {
	mod := &ir.Module{}
	fields := flattenScalarFields(mod, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, 0)
	if len(fields) != 1 {
		t.Fatalf("scalar: want 1 field, got %d", len(fields))
	}
	if fields[0].byteOffset != 0 || fields[0].scalar.Kind != ir.ScalarUint {
		t.Errorf("scalar: unexpected field %+v", fields[0])
	}
}

func TestFlattenScalarFields_Vector(t *testing.T) {
	mod := &ir.Module{}
	vec := ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	fields := flattenScalarFields(mod, vec, 8)
	if len(fields) != 3 {
		t.Fatalf("vec3: want 3 fields, got %d", len(fields))
	}
	wantOffsets := []uint32{8, 12, 16}
	for i, f := range fields {
		if f.byteOffset != wantOffsets[i] {
			t.Errorf("vec3[%d]: byteOffset want %d got %d", i, wantOffsets[i], f.byteOffset)
		}
		if f.scalar.Kind != ir.ScalarFloat || f.scalar.Width != 4 {
			t.Errorf("vec3[%d]: scalar want f32 got %+v", i, f.scalar)
		}
	}
}

func TestFlattenScalarFields_MixedStruct(t *testing.T) {
	// struct Segment { x: f32, w: i32 } — the BUG-DXIL-026 Group A seed.
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	i32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: f32},
			{Inner: i32},
		},
	}
	st := ir.StructType{Members: []ir.StructMember{
		{Type: 0, Offset: 0},
		{Type: 1, Offset: 4},
	}}
	fields := flattenScalarFields(mod, st, 0)
	if len(fields) != 2 {
		t.Fatalf("mixed struct: want 2 fields, got %d", len(fields))
	}
	if fields[0].scalar.Kind != ir.ScalarFloat || fields[0].byteOffset != 0 {
		t.Errorf("mixed struct [0]: want f32@0 got %+v", fields[0])
	}
	if fields[1].scalar.Kind != ir.ScalarSint || fields[1].byteOffset != 4 {
		t.Errorf("mixed struct [1]: want i32@4 got %+v", fields[1])
	}
}

// --- hasMixedScalarTypes --------------------------------------------------

func TestHasMixedScalarTypes_HomogeneousStruct(t *testing.T) {
	// struct PathMonoid { a,b,c,d,e: u32 } — NOT mixed.
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	mod := &ir.Module{Types: []ir.Type{{Inner: u32}}}
	st := ir.StructType{Members: []ir.StructMember{
		{Type: 0, Offset: 0}, {Type: 0, Offset: 4}, {Type: 0, Offset: 8},
		{Type: 0, Offset: 12}, {Type: 0, Offset: 16},
	}}
	if hasMixedScalarTypes(mod, st) {
		t.Errorf("homogeneous u32 struct should not be mixed-scalar")
	}
}

func TestHasMixedScalarTypes_SintUintHomogeneous(t *testing.T) {
	// Sint + Uint of same width share DXIL type (.iN) — not mixed.
	i32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	mod := &ir.Module{Types: []ir.Type{{Inner: i32}, {Inner: u32}}}
	st := ir.StructType{Members: []ir.StructMember{
		{Type: 0, Offset: 0}, {Type: 1, Offset: 4},
	}}
	if hasMixedScalarTypes(mod, st) {
		t.Errorf("i32+u32 struct should not be mixed-scalar (share .iN overload)")
	}
}

func TestHasMixedScalarTypes_FloatIntMixed(t *testing.T) {
	// struct { f32, i32 } — Group A pattern.
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	i32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	mod := &ir.Module{Types: []ir.Type{{Inner: f32}, {Inner: i32}}}
	st := ir.StructType{Members: []ir.StructMember{
		{Type: 0, Offset: 0}, {Type: 1, Offset: 4},
	}}
	if !hasMixedScalarTypes(mod, st) {
		t.Errorf("f32+i32 struct must be classified mixed-scalar (BUG-DXIL-026 Group A)")
	}
}

func TestHasMixedScalarTypes_DifferentWidths(t *testing.T) {
	// struct { u16, u32 } — width differs even though kind matches → mixed.
	u16 := ir.ScalarType{Kind: ir.ScalarUint, Width: 2}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	mod := &ir.Module{Types: []ir.Type{{Inner: u16}, {Inner: u32}}}
	st := ir.StructType{Members: []ir.StructMember{
		{Type: 0, Offset: 0}, {Type: 1, Offset: 4},
	}}
	if !hasMixedScalarTypes(mod, st) {
		t.Errorf("u16+u32 struct must be classified mixed-scalar (different widths)")
	}
}

// --- scalarsShareDXILType -------------------------------------------------

func TestScalarsShareDXILType(t *testing.T) {
	tests := []struct {
		name string
		a, b ir.ScalarType
		want bool
	}{
		{"sint+uint same width", siT(4), uiT(4), true},
		{"uint+uint same width", uiT(4), uiT(4), true},
		{"float+float same width", fT(4), fT(4), true},
		{"sint+uint different width", siT(4), uiT(2), false},
		{"float+sint same width", fT(4), siT(4), false},
		{"bool+bool", bT(), bT(), true},
		{"bool+sint", bT(), siT(4), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scalarsShareDXILType(tt.a, tt.b); got != tt.want {
				t.Errorf("scalarsShareDXILType(%v,%v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// --- resolvePointerAddrSpace ----------------------------------------------

func TestResolvePointerAddrSpace_WorkgroupRoot(t *testing.T) {
	// Expression chain: Access(AccessIndex(GlobalVariable[workgroup], 0), idx)
	// Result: addrspace 3.
	e := &Emitter{
		ir: &ir.Module{
			GlobalVariables: []ir.GlobalVariable{
				{Name: "sh", Space: ir.SpaceWorkGroup},
			},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                     // handle 0
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                  // handle 1
			{Kind: ir.ExprAccess{Base: 1, Index: ir.ExpressionHandle(99)}}, // handle 2
		},
	}
	got := e.resolvePointerAddrSpace(fn, ir.ExpressionHandle(2))
	if got != 3 {
		t.Errorf("workgroup-rooted pointer: want addrspace 3, got %d", got)
	}
}

func TestResolvePointerAddrSpace_PrivateRoot(t *testing.T) {
	e := &Emitter{
		ir: &ir.Module{
			GlobalVariables: []ir.GlobalVariable{
				{Name: "pv", Space: ir.SpacePrivate},
			},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		},
	}
	got := e.resolvePointerAddrSpace(fn, ir.ExpressionHandle(1))
	if got != 0 {
		t.Errorf("private-rooted pointer: want addrspace 0, got %d", got)
	}
}

func TestResolvePointerAddrSpace_LocalVar(t *testing.T) {
	e := &Emitter{ir: &ir.Module{}}
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "x"}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
	}
	got := e.resolvePointerAddrSpace(fn, ir.ExpressionHandle(0))
	if got != 0 {
		t.Errorf("local var: want addrspace 0, got %d", got)
	}
}

// --- workgroupElemRoot ----------------------------------------------------

func TestWorkgroupElemRoot_RoundTrip(t *testing.T) {
	e := &Emitter{
		workgroupElemPtrs: make(map[int]workgroupElemOrigin),
	}
	want := workgroupElemOrigin{
		globalAllocaID: 42,
		arrayTy:        &module.Type{Kind: module.TypeArray},
		elemIndexID:    17,
		elemTy:         &module.Type{Kind: module.TypeStruct},
	}
	e.workgroupElemPtrs[101] = want
	got, ok := e.workgroupElemRoot(101)
	if !ok {
		t.Fatalf("workgroupElemRoot(101): not found")
	}
	if got != want {
		t.Errorf("workgroupElemRoot(101): got %+v, want %+v", got, want)
	}
	if _, ok := e.workgroupElemRoot(202); ok {
		t.Errorf("workgroupElemRoot(202): should not be found")
	}
}

func TestWorkgroupElemRoot_NilMap(t *testing.T) {
	// Before emitter init the map is nil — must not panic and must return false.
	e := &Emitter{}
	if _, ok := e.workgroupElemRoot(1); ok {
		t.Errorf("nil map: should return false")
	}
}

// --- Small helpers to keep tests readable --------------------------------

func fT(w uint8) ir.ScalarType  { return ir.ScalarType{Kind: ir.ScalarFloat, Width: w} }
func siT(w uint8) ir.ScalarType { return ir.ScalarType{Kind: ir.ScalarSint, Width: w} }
func uiT(w uint8) ir.ScalarType { return ir.ScalarType{Kind: ir.ScalarUint, Width: w} }
func bT() ir.ScalarType         { return ir.ScalarType{Kind: ir.ScalarBool, Width: 1} }
