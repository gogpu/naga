package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestCalculateOffsetScalarsAndVectors is the OpenGL spec's std140 rules 1-3
// (offset.rs:55-69), which std430 shares. A bool is the one member where the
// reference's rule and the driver's layout part: Rust naga takes the IR width
// (1 byte) and glslangValidator writes 4, so the reference compiler decides
// here (layoutScalarWidth, offset.go).
func TestCalculateOffsetScalarsAndVectors(t *testing.T) {
	tests := []struct {
		name  string
		inner ir.TypeInner
		align uint32
		span  uint32
	}{
		{"float", f32Scalar, 4, 4},
		// A bool is a word in a block: glslangValidator lays the members of
		// `bool a; bool b;` out at 0 and 4, and a bvec2 at 8 bytes with
		// alignment 8 (snapshot/glsl_plain_uniform_test.go has the probe).
		{"bool", boolScalar, 4, 4},
		{"bvec2", ir.VectorType{Size: ir.Vec2, Scalar: boolScalar}, 8, 8},
		{"bvec3", ir.VectorType{Size: ir.Vec3, Scalar: boolScalar}, 16, 12},
		{"vec2", ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar}, 8, 8},
		{"vec3", ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar}, 16, 12},
		{"vec4", ir.VectorType{Size: ir.Vec4, Scalar: f32Scalar}, 16, 16},
		{"uvec2", ir.VectorType{Size: ir.Vec2, Scalar: u32Scalar}, 8, 8},
	}
	for _, tt := range tests {
		fe := newFrontend(&ir.Module{})
		ty := fe.registerType(tt.inner)
		for _, layout := range []structLayout{structLayoutStd140, structLayoutStd430} {
			got := fe.calculateOffset(ty, ir.Span{}, layout)
			if got.align != tt.align || got.span != tt.span {
				t.Errorf("%s under layout %d: align %d span %d, want align %d span %d",
					tt.name, layout, got.align, got.span, tt.align, tt.span)
			}
		}
	}
}

// TestCalculateOffsetArrays is rule 4: the stride follows the element's
// alignment, and std140 rounds it up to the alignment of a vec4.
func TestCalculateOffsetArrays(t *testing.T) {
	f32Ty := func(fe *Frontend) ir.TypeHandle { return fe.registerType(f32Scalar) }
	three := uint32(3)

	std140 := func(fe *Frontend) ir.TypeHandle {
		return fe.registerType(ir.ArrayType{Base: f32Ty(fe), Size: ir.ArraySize{Constant: &three}, Stride: 4})
	}
	fe := newFrontend(&ir.Module{})
	got := fe.calculateOffset(std140(fe), ir.Span{}, structLayoutStd140)
	if got.align != 16 || got.span != 48 {
		t.Fatalf("std140 float[3]: align %d span %d, want align 16 span 48", got.align, got.span)
	}
	arr, ok := fe.mod.Types[got.ty].Inner.(ir.ArrayType)
	if !ok || arr.Stride != 16 {
		t.Fatalf("std140 float[3] stride = %+v, want 16", arr)
	}

	fe430 := newFrontend(&ir.Module{})
	got430 := fe430.calculateOffset(std140(fe430), ir.Span{}, structLayoutStd430)
	if got430.align != 4 || got430.span != 12 {
		t.Fatalf("std430 float[3]: align %d span %d, want align 4 span 12", got430.align, got430.span)
	}
	arr430 := fe430.mod.Types[got430.ty].Inner.(ir.ArrayType)
	if arr430.Stride != 4 {
		t.Fatalf("std430 float[3] stride = %d, want 4", arr430.Stride)
	}

	// A vec3 array: std140 strides by 16, the vec3's own size rounds up.
	feVec := newFrontend(&ir.Module{})
	vec3 := feVec.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar})
	vecArr := feVec.registerType(ir.ArrayType{Base: vec3, Size: ir.ArraySize{Constant: &three}, Stride: 12})
	gotVec := feVec.calculateOffset(vecArr, ir.Span{}, structLayoutStd140)
	if gotVec.align != 16 || gotVec.span != 48 {
		t.Fatalf("std140 vec3[3]: align %d span %d, want align 16 span 48", gotVec.align, gotVec.span)
	}
}

// TestCalculateOffsetStruct lays out a struct by rule 9: members are placed
// at the next multiple of their alignment and the struct's span is rounded to
// its own alignment.
func TestCalculateOffsetStruct(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	vec3 := fe.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar})
	float := fe.registerType(f32Scalar)
	vec2 := fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar})
	structTy := fe.registerType(ir.StructType{Members: []ir.StructMember{
		{Name: "a", Type: vec3},
		{Name: "b", Type: float},
		{Name: "c", Type: vec2},
	}, Span: 0})

	got := fe.calculateOffset(structTy, ir.Span{}, structLayoutStd140)
	// a at 0 (align 16, span 12), b at 12 (align 4, span 4), c at 16
	// (align 8, span 8): span 24, aligned to 16 gives 32.
	if got.align != 16 || got.span != 32 {
		t.Fatalf("struct align %d span %d, want align 16 span 32", got.align, got.span)
	}
	st := fe.mod.Types[got.ty].Inner.(ir.StructType)
	wantOffsets := []uint32{0, 12, 16}
	for i, want := range wantOffsets {
		if st.Members[i].Offset != want {
			t.Errorf("member %s offset = %d, want %d", st.Members[i].Name, st.Members[i].Offset, want)
		}
	}

	// Under std430 a struct of one float keeps its 4-byte span, but std140
	// rounds a struct's alignment up to a vec4's (rule 9): glslangValidator
	// lays `struct S { float a; }; S s; float after_s;` out with after_s at 16,
	// so the member occupies a 16-byte slot, not four. Rust naga's
	// calculate_offset keeps the natural alignment here.
	small := fe.registerType(ir.StructType{Members: []ir.StructMember{{Name: "x", Type: float}}})
	small430 := fe.calculateOffset(small, ir.Span{}, structLayoutStd430)
	if small430.align != 4 || small430.span != 4 {
		t.Fatalf("std430 struct{float}: align %d span %d, want align 4 span 4", small430.align, small430.span)
	}
	small140 := fe.calculateOffset(small, ir.Span{}, structLayoutStd140)
	if small140.align != 16 || small140.span != 16 {
		t.Fatalf("std140 struct{float}: align %d span %d, want align 16 span 16 (rule 9)", small140.align, small140.span)
	}
}

// TestCalculateOffsetMatrices is rule 5, including Rust's two std140
// refusals: a matrix of two rows and an f16 matrix cannot be expressed with
// the IR's stride (offset.rs:112-147).
func TestCalculateOffsetMatrices(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	mat3 := fe.registerType(ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32Scalar})
	got := fe.calculateOffset(mat3, ir.Span{}, structLayoutStd140)
	if got.align != 16 || got.span != 48 {
		t.Fatalf("std140 mat3: align %d span %d, want align 16 span 48", got.align, got.span)
	}
	if len(fe.errors) != 0 {
		t.Fatalf("mat3 reported %+v, want no errors", fe.errors)
	}

	bad := newFrontend(&ir.Module{})
	mat2 := bad.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	bad.calculateOffset(mat2, ir.Span{}, structLayoutStd140)
	if len(bad.errors) != 1 || bad.errors[0].Message != "unsupported matrix of the form matCx2 (in this case mat2x2) in std140 block layout." {
		t.Fatalf("mat2x2 errors = %+v, want the matCx2 refusal", bad.errors)
	}

	f16 := newFrontend(&ir.Module{})
	f16mat := f16.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}})
	f16.calculateOffset(f16mat, ir.Span{}, structLayoutStd140)
	if len(f16.errors) != 2 {
		t.Fatalf("f16mat2x2 reported %d errors, want both the f16 and the matCx2 refusal: %+v", len(f16.errors), f16.errors)
	}

	ok430 := newFrontend(&ir.Module{})
	mat2_430 := ok430.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	ok430.calculateOffset(mat2_430, ir.Span{}, structLayoutStd430)
	if len(ok430.errors) != 0 {
		t.Fatalf("std430 mat2x2 reported %+v, want std430 to accept it", ok430.errors)
	}
}

// TestCalculateOffsetRejectsOpaque checks the "Invalid struct member type"
// error for a type a block cannot hold, as Rust reports (offset.rs:180-186).
func TestCalculateOffsetRejectsOpaque(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	sampler := fe.registerType(ir.SamplerType{})
	got := fe.calculateOffset(sampler, ir.Span{}, structLayoutStd140)
	if len(fe.errors) != 1 || fe.errors[0].Message != "Invalid struct member type" {
		t.Fatalf("errors = %+v, want the invalid-member error", fe.errors)
	}
	if got.span != 0 {
		t.Fatalf("span = %d, want 0", got.span)
	}
}
