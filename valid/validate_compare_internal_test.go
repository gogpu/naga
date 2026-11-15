package valid

// White-box tests of the type comparison the validator uses for store and
// initializer checks (Rust naga's proc::compare_types in canonical form):
// sameTypeInner, canonicalInner, resolutionKnown and compareTypes. The
// end-to-end behaviour is pinned from the outside (validate_store_test.go);
// these cover the comparison's own branches, including the kinds a small
// shader cannot reach.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// compareModule builds the type arena the tests index into.
//
//	0 f32                 1 vec3<f32>           2 mat2x2<f32>
//	3 struct S { f32 }    4 array<f32, 4>       5 array<f32, runtime>
//	6 atomic<u32>         7 ptr<function, f32>  8 binding_array<f32>
//	9 sampler             10 image2D f32
func compareModule() *ir.Module {
	four := uint32(4)
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	return &ir.Module{Types: []ir.Type{
		{Inner: f32},
		{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}},
		{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32}},
		{Inner: ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}}, Span: 4}},
		{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &four}, Stride: 4}},
		{Inner: ir.ArrayType{Base: 0, Stride: 4}},
		{Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
		{Inner: ir.BindingArrayType{Base: 0}},
		{Inner: ir.SamplerType{}},
		{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}}
}

func handle(h ir.TypeHandle) *ir.TypeHandle { return &h }

func TestSameTypeInner(t *testing.T) {
	m := compareModule()
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vec3 := ir.VectorType{Size: ir.Vec3, Scalar: f32}
	four, five := uint32(4), uint32(5)
	vec2, vec3Size := ir.Vec2, ir.Vec3

	tests := []struct {
		name string
		a, b ir.TypeInner
		want bool
	}{
		{"scalar equal", f32, f32, true},
		{"scalar differs", f32, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, false},
		{"vector equal", vec3, vec3, true},
		{"vector size differs", vec3, ir.VectorType{Size: ir.Vec4, Scalar: f32}, false},
		{"matrix equal", m.Types[2].Inner, m.Types[2].Inner, true},
		{"matrix differs", m.Types[2].Inner, ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: f32}, false},
		{"array equal", m.Types[4].Inner, m.Types[4].Inner, true},
		{"array size differs", m.Types[4].Inner, ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &five}, Stride: 4}, false},
		{"array stride differs", m.Types[4].Inner, ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &four}, Stride: 8}, false},
		{"array base differs", m.Types[4].Inner, ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &four}, Stride: 4}, false},
		{"struct equal", m.Types[3].Inner, m.Types[3].Inner, true},
		{"struct span differs", m.Types[3].Inner, ir.StructType{Members: []ir.StructMember{{Type: 0, Offset: 0}}, Span: 8}, false},
		{"struct member count differs", m.Types[3].Inner, ir.StructType{Span: 4}, false},
		{"struct member offset differs", m.Types[3].Inner, ir.StructType{Members: []ir.StructMember{{Type: 0, Offset: 4}}, Span: 4}, false},
		{"struct member type differs", m.Types[3].Inner, ir.StructType{Members: []ir.StructMember{{Type: 1, Offset: 0}}, Span: 4}, false},
		{"value pointer equal", ir.ValuePointerType{Scalar: f32, Space: ir.SpaceFunction}, ir.ValuePointerType{Scalar: f32, Space: ir.SpaceFunction}, true},
		{"value pointer vector size differs", ir.ValuePointerType{Size: &vec2, Scalar: f32}, ir.ValuePointerType{Size: &vec3Size, Scalar: f32}, false},
		{"value pointer space differs", ir.ValuePointerType{Scalar: f32, Space: ir.SpaceFunction}, ir.ValuePointerType{Scalar: f32, Space: ir.SpacePrivate}, false},
		{"binding array equal", ir.BindingArrayType{Base: 0}, ir.BindingArrayType{Base: 0}, true},
		{"binding array size differs", ir.BindingArrayType{Base: 0}, ir.BindingArrayType{Base: 0, Size: &four}, false},
		{"binding array base differs", ir.BindingArrayType{Base: 0}, ir.BindingArrayType{Base: 1}, false},
		{"atomic equal", m.Types[6].Inner, m.Types[6].Inner, true},
		{"pointer equal", m.Types[7].Inner, m.Types[7].Inner, true},
		{"sampler equal", m.Types[9].Inner, m.Types[9].Inner, true},
		{"image differs", m.Types[10].Inner, ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled}, false},
		{"cross kind", m.Types[3].Inner, m.Types[4].Inner, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameTypeInner(tt.a, tt.b); got != tt.want {
				t.Errorf("sameTypeInner(%T, %T) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSameVectorSizeAndBindingArraySize(t *testing.T) {
	four, five := uint32(4), uint32(5)
	vec2, vec3 := ir.Vec2, ir.Vec3
	if !sameVectorSize(nil, nil) || sameVectorSize(nil, &vec2) || sameVectorSize(&vec2, nil) || sameVectorSize(&vec2, &vec3) {
		t.Errorf("sameVectorSize: nil handling or size comparison is wrong")
	}
	if !sameVectorSize(&vec2, &vec2) {
		t.Errorf("sameVectorSize: equal sizes are not equal")
	}
	if !sameBindingArraySize(nil, nil) || sameBindingArraySize(nil, &four) || sameBindingArraySize(&four, &five) {
		t.Errorf("sameBindingArraySize: nil handling or size comparison is wrong")
	}
	if !sameBindingArraySize(&four, &four) {
		t.Errorf("sameBindingArraySize: equal sizes are not equal")
	}
}

func TestCanonicalInner(t *testing.T) {
	v := &validator{module: compareModule()}

	// A pointer to a scalar is the value pointer of that scalar.
	got := v.canonicalInner(ir.TypeResolution{Handle: handle(7)})
	want := ir.ValuePointerType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Space: ir.SpaceFunction}
	if !sameTypeInner(got, want) {
		t.Errorf("pointer to scalar = %#v, want %#v", got, want)
	}

	// A pointer to a vector carries the vector's size.
	m := compareModule()
	ptr := ir.TypeHandle(len(m.Types))
	m.Types = append(m.Types, ir.Type{Inner: ir.PointerType{Base: 1, Space: ir.SpacePrivate}})
	v = &validator{module: m}
	got = v.canonicalInner(ir.TypeResolution{Handle: &ptr})
	vec3 := ir.Vec3
	want = ir.ValuePointerType{Size: &vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Space: ir.SpacePrivate}
	if vp, ok := got.(ir.ValuePointerType); !ok || vp.Size == nil || *vp.Size != ir.Vec3 ||
		!sameTypeInner(got, want) {
		t.Errorf("pointer to vector = %#v, want a value pointer with size Vec3", got)
	}

	// Anything else is unchanged: a struct, and a pointer to a struct.
	if got := v.canonicalInner(ir.TypeResolution{Handle: handle(3)}); !sameTypeInner(got, m.Types[3].Inner) {
		t.Errorf("struct changed canonical form: %#v", got)
	}
	ptrStruct := ir.TypeHandle(len(m.Types))
	m.Types = append(m.Types, ir.Type{Inner: ir.PointerType{Base: 3, Space: ir.SpaceFunction}})
	if got := v.canonicalInner(ir.TypeResolution{Handle: &ptrStruct}); !sameTypeInner(got, m.Types[ptrStruct].Inner) {
		t.Errorf("pointer to struct changed canonical form: %#v", got)
	}
	// A pointer whose base is out of range stays as it is.
	badPtr := ir.TypeHandle(len(m.Types))
	m.Types = append(m.Types, ir.Type{Inner: ir.PointerType{Base: 99, Space: ir.SpaceFunction}})
	if got := v.canonicalInner(ir.TypeResolution{Handle: &badPtr}); !sameTypeInner(got, m.Types[badPtr].Inner) {
		t.Errorf("out-of-range pointer base changed canonical form: %#v", got)
	}
}

func TestResolutionKnown(t *testing.T) {
	v := &validator{module: compareModule()}
	bad := ir.TypeHandle(99)
	if !v.resolutionKnown(ir.TypeResolution{Value: ir.ScalarType{}}) {
		t.Errorf("an inline value resolution is not known")
	}
	if !v.resolutionKnown(ir.TypeResolution{Handle: handle(0)}) {
		t.Errorf("a handle in the arena is not known")
	}
	if v.resolutionKnown(ir.TypeResolution{Handle: &bad}) {
		t.Errorf("an out-of-range handle is reported known")
	}
}

func TestCompareTypes(t *testing.T) {
	v := &validator{module: compareModule()}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	// A struct is unique by handle.
	if !v.compareTypes(ir.TypeResolution{Handle: handle(3)}, ir.TypeResolution{Handle: handle(3)}) {
		t.Errorf("the same struct handle compared unequal")
	}
	if v.compareTypes(ir.TypeResolution{Handle: handle(3)}, ir.TypeResolution{Handle: handle(4)}) {
		t.Errorf("two different handles compared equal")
	}
	// Inline values compare structurally.
	if !v.compareTypes(ir.TypeResolution{Value: f32}, ir.TypeResolution{Value: f32}) {
		t.Errorf("equal inline scalars compared unequal")
	}
	// A pointer to a scalar matches the scalar's value pointer (canonical form).
	valuePtr := ir.ValuePointerType{Scalar: f32, Space: ir.SpaceFunction}
	if !v.compareTypes(ir.TypeResolution{Handle: handle(7)}, ir.TypeResolution{Value: valuePtr}) {
		t.Errorf("pointer to scalar did not match its value pointer")
	}
	// An unknown handle on either side never matches.
	bad := ir.TypeHandle(99)
	if v.compareTypes(ir.TypeResolution{Handle: &bad}, ir.TypeResolution{Value: f32}) ||
		v.compareTypes(ir.TypeResolution{Value: f32}, ir.TypeResolution{Handle: &bad}) {
		t.Errorf("an unknown handle compared equal")
	}
}
