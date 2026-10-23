package emit

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// Unit tests for the byte-layout helper elemByteSize.
// These pin down the pure-function contracts that drive UAV coord0 byte-offset
// computation per DXIL.rst:1789 (RWRawBuffer expects coord0 in bytes).
//
// End-to-end coverage:
//   - snapshot.TestDxilDxcGolden  — golden parity vs DXC HLSL roundtrip
//   - snapshot.TestDxilValSummary — IDxcValidator type validity
//
// Expected values are aligned with DXC's actual struct/matrix layout (verified
// against the dxc compiler for non-trivial cases like mat3x3<f32> column
// padding to 16-byte alignment).

func TestElemByteSize_Scalar(t *testing.T) {
	cases := []struct {
		name  string
		ty    ir.ScalarType
		bytes uint32
	}{
		{"u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, 4},
		{"i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, 4},
		{"f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 4},
		{"u64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, 8},
		{"i64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, 8},
		{"f64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, 8},
		{"f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := elemByteSize(&ir.Module{}, tc.ty)
			if got != tc.bytes {
				t.Errorf("%s: want %d got %d", tc.name, tc.bytes, got)
			}
		})
	}
}

func TestElemByteSize_Vector(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	cases := []struct {
		name  string
		size  ir.VectorSize
		bytes uint32
	}{
		{"vec2<f32>", 2, 8},
		{"vec3<f32>", 3, 12},
		{"vec4<f32>", 4, 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := elemByteSize(&ir.Module{}, ir.VectorType{Size: tc.size, Scalar: f32})
			if got != tc.bytes {
				t.Errorf("%s: want %d got %d", tc.name, tc.bytes, got)
			}
		})
	}
}

func TestElemByteSize_Matrix(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	cases := []struct {
		name          string
		cols, rows    ir.VectorSize
		expectedBytes uint32
	}{
		{"mat2x2<f32>", 2, 2, 16},
		{"mat3x2<f32>", 3, 2, 24},
		{"mat4x2<f32>", 4, 2, 32},
		{"mat2x3<f32>", 2, 3, 32},
		{"mat3x3<f32>", 3, 3, 48},
		{"mat4x3<f32>", 4, 3, 64},
		{"mat2x4<f32>", 2, 4, 32},
		{"mat3x4<f32>", 3, 4, 48},
		{"mat4x4<f32>", 4, 4, 64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mt := ir.MatrixType{Columns: tc.cols, Rows: tc.rows, Scalar: f32}
			got := elemByteSize(&ir.Module{}, mt)
			if got != tc.expectedBytes {
				t.Errorf("%s: want elem size %d got %d", tc.name, tc.expectedBytes, got)
			}
		})
	}
}

func TestElemByteSize_Atomic(t *testing.T) {
	cases := []struct {
		name  string
		width uint8
		bytes uint32
	}{
		{"atomic<u32>", 4, 4},
		{"atomic<i64>", 8, 8},
		{"atomic<u64>", 8, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: tc.width}}
			got := elemByteSize(&ir.Module{}, at)
			if got != tc.bytes {
				t.Errorf("%s: want %d got %d", tc.name, tc.bytes, got)
			}
		})
	}
}

func TestElemByteSize_Array(t *testing.T) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	mod := &ir.Module{Types: []ir.Type{{Inner: u32}}}

	t.Run("array<u32, 8> with stride", func(t *testing.T) {
		count := uint32(8)
		ty := ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &count}, Stride: 4}
		got := elemByteSize(mod, ty)
		if got != 32 {
			t.Errorf("array<u32, 8>: want 32 got %d", got)
		}
	})

	t.Run("array<u32, 4> stride 16 (vec4 padding)", func(t *testing.T) {
		count := uint32(4)
		ty := ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &count}, Stride: 16}
		got := elemByteSize(mod, ty)
		if got != 64 {
			t.Errorf("array<u32, 4> stride 16: want 64 got %d", got)
		}
	})
}

func TestElemByteSize_Struct(t *testing.T) {
	// struct PathMonoid { trans_ix, path_seg_ix, path_seg_offset, style_ix, path_ix: u32 } — Span 20
	ty := ir.StructType{
		Span: 20,
		Members: []ir.StructMember{
			{Type: 0, Offset: 0},
			{Type: 0, Offset: 4},
			{Type: 0, Offset: 8},
			{Type: 0, Offset: 12},
			{Type: 0, Offset: 16},
		},
	}
	got := elemByteSize(&ir.Module{}, ty)
	if got != 20 {
		t.Errorf("PathMonoid struct: want 20 got %d", got)
	}
}

// TestElemByteSize_F16Vector locks the f16 vector size accounting that drives
// f16 storage buffer access. f16 is 2 bytes; vec4<f16> = 8 bytes (not 16).
func TestElemByteSize_F16Vector(t *testing.T) {
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	ty := ir.VectorType{Size: 4, Scalar: f16}
	got := elemByteSize(&ir.Module{}, ty)
	if got != 8 {
		t.Errorf("vec4<f16>: want 8 got %d", got)
	}
}
