package ir

import "testing"

// TestTypeAlignment: alignments as Rust naga's proc::Layouter computes them
// (WGSL "Alignment and Size" table).
func TestTypeAlignment(t *testing.T) {
	f32 := ScalarType{Kind: ScalarFloat, Width: 4}
	f16 := ScalarType{Kind: ScalarFloat, Width: 2}
	m := &Module{Types: []Type{
		{Inner: f32}, // 0
		{Inner: VectorType{Size: Vec2, Scalar: f32}},                        // 1
		{Inner: VectorType{Size: Vec3, Scalar: f32}},                        // 2
		{Inner: VectorType{Size: Vec3, Scalar: f16}},                        // 3
		{Inner: MatrixType{Columns: 3, Rows: Vec3, Scalar: f32}},            // 4
		{Inner: ArrayType{Base: 2, Stride: 16}},                             // 5
		{Inner: StructType{Members: []StructMember{{Type: 0}, {Type: 2}}}},  // 6
		{Inner: SamplerType{}},                                              // 7
		{Inner: AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}}, // 8
	}}
	tests := []struct {
		h    TypeHandle
		want uint32
	}{{0, 4}, {1, 8}, {2, 16}, {3, 8}, {4, 16}, {5, 16}, {6, 16}, {7, 1}, {8, 4}, {99, 1}}
	for _, tc := range tests {
		if got := TypeAlignment(m, tc.h); got != tc.want {
			t.Errorf("TypeAlignment(%d) = %d, want %d", tc.h, got, tc.want)
		}
	}
}
