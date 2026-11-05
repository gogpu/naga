package snapshot_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestDxcGoldenProfile pins the DXC profile and 16-bit detection the goldens
// are generated with: SM 6.2 and -enable-16bit-types for a module with f16,
// i16 or u16 anywhere in its types, as wgpu-hal compiles it.
func TestDxcGoldenProfile(t *testing.T) {
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	u16 := ir.ScalarType{Kind: ir.ScalarUint, Width: 2}
	cases := []struct {
		name  string
		types []ir.Type
		want  bool
	}{
		{"none", nil, false},
		{"f32 only", []ir.Type{{Inner: f32}, {Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}}}, false},
		{"f16 scalar", []ir.Type{{Inner: f32}, {Inner: f16}}, true},
		{"f16 vector", []ir.Type{{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f16}}}, true},
		{"f16 matrix", []ir.Type{{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f16}}}, true},
		{"u16 scalar", []ir.Type{{Inner: u16}}, true},
	}
	for _, c := range cases {
		if got := uses16BitTypes(&ir.Module{Types: c.types}); got != c.want {
			t.Errorf("%s: uses16BitTypes = %v, want %v", c.name, got, c.want)
		}
	}

	profiles := []struct {
		stage      ir.ShaderStage
		sixteenBit bool
		want       string
	}{
		{ir.StageVertex, false, "vs_6_0"},
		{ir.StageFragment, true, "ps_6_2"},
		{ir.StageCompute, true, "cs_6_2"},
		{ir.StageMesh, true, "ms_6_5"},
		{ir.StageTask, false, "as_6_5"},
	}
	for _, p := range profiles {
		if got := stageDxcProfile(p.stage, p.sixteenBit); got != p.want {
			t.Errorf("stageDxcProfile(%v, %v) = %q, want %q", p.stage, p.sixteenBit, got, p.want)
		}
	}
}
