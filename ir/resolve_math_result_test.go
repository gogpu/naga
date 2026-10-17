package ir

import (
	"reflect"
	"testing"
)

// Result types of the math built-ins whose result is not the argument's
// type (WGSL spec; Rust naga's overload conclusions).
func TestResolveMathResultTypes(t *testing.T) {
	f16 := ScalarType{Kind: ScalarFloat, Width: 2}
	f32 := ScalarType{Kind: ScalarFloat, Width: 4}
	m := &Module{Types: []Type{
		{Inner: MatrixType{Columns: Vec2, Rows: Vec3, Scalar: f32}}, // 0: mat2x3<f32>
		{Inner: VectorType{Size: Vec3, Scalar: f16}},                // 1: vec3<f16>
		{Inner: VectorType{Size: Vec2, Scalar: f32}},                // 2: vec2<f32>
		{Inner: VectorType{Size: Vec4, Scalar: f32}},                // 3: vec4<f32>
		{Inner: MatrixType{Columns: Vec3, Rows: Vec3, Scalar: f32}}, // 4: mat3x3<f32>
	}}
	h := func(i TypeHandle) *TypeHandle { return &i }
	fn := &Function{Expressions: []Expression{
		{Kind: ExprZeroValue{Type: 0}}, // 0: mat2x3<f32>
		{Kind: ExprZeroValue{Type: 1}}, // 1: vec3<f16>
		{Kind: ExprZeroValue{Type: 2}}, // 2: vec2<f32>
		{Kind: ExprZeroValue{Type: 3}}, // 3: vec4<f32>
		{Kind: ExprZeroValue{Type: 4}}, // 4: mat3x3<f32>
	}}
	fn.ExpressionTypes = []TypeResolution{{Handle: h(0)}, {Handle: h(1)}, {Handle: h(2)}, {Handle: h(3)}, {Handle: h(4)}}
	one, two, three := ExpressionHandle(1), ExpressionHandle(2), ExpressionHandle(3)
	tests := []struct {
		name string
		math ExprMath
		want TypeInner
	}{
		{"determinant", ExprMath{Fun: MathDeterminant, Arg: 4}, f32},
		{"transpose", ExprMath{Fun: MathTranspose, Arg: 0}, MatrixType{Columns: Vec3, Rows: Vec2, Scalar: f32}},
		{"outer", ExprMath{Fun: MathOuter, Arg: 2, Arg1: &three}, MatrixType{Columns: Vec4, Rows: Vec2, Scalar: f32}},
		{"length f16", ExprMath{Fun: MathLength, Arg: one}, f16},
		{"distance f16", ExprMath{Fun: MathDistance, Arg: one, Arg1: &one}, f16},
		{"dot", ExprMath{Fun: MathDot, Arg: two, Arg1: &two}, f32},
	}
	for _, tt := range tests {
		fn.Expressions = append(fn.Expressions[:5], Expression{Kind: tt.math})
		res, err := ResolveExpressionType(m, fn, 5)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got := TypeResInner(m, res); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: result %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

// A sampled image's texel type is its SampledKind (Rust ImageClass::Sampled
// { kind }); i32 is a kind like any other, not "unset".
func TestResolveImageLoadSampledKind(t *testing.T) {
	for _, kind := range []ScalarKind{ScalarSint, ScalarUint, ScalarFloat} {
		m := &Module{Types: []Type{{Inner: ImageType{Dim: Dim2D, Class: ImageClassSampled, SampledKind: kind}}}}
		fn := &Function{Expressions: []Expression{
			{Kind: ExprGlobalVariable{Variable: 0}},
			{Kind: Literal{Value: LiteralI32(0)}},
			{Kind: ExprImageLoad{Image: 0, Coordinate: 1}},
		}}
		m.GlobalVariables = []GlobalVariable{{Name: "t", Space: SpaceHandle, Type: 0}}
		res, err := ResolveExpressionType(m, fn, 2)
		if err != nil {
			t.Fatal(err)
		}
		want := VectorType{Size: Vec4, Scalar: ScalarType{Kind: kind, Width: 4}}
		if got := TypeResInner(m, res); got != want {
			t.Errorf("kind %v: textureLoad type = %#v, want %#v", kind, got, want)
		}
	}
}
