package snapshot_test

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestExpectedRustOutputs(t *testing.T) {
	eps := []ir.EntryPoint{
		{Name: "vs", Stage: ir.StageVertex},
		{Name: "fs", Stage: ir.StageFragment},
		{Name: "ts", Stage: ir.StageTask},
		{Name: "ms", Stage: ir.StageMesh},
		{Name: "cs", Stage: ir.StageCompute},
	}
	tests := []struct {
		name   string
		params rustParams
		eps    []ir.EntryPoint
		want   []rustOutput
	}{
		{
			name:   "no targets, no outputs",
			params: rustParams{},
			eps:    eps,
			want:   nil,
		},
		{
			name:   "single-file backends",
			params: rustParams{targets: rustTargetSPIRV | rustTargetMETAL | rustTargetHLSL | rustTargetWGSL},
			eps:    eps,
			want: []rustOutput{
				{kind: rustOutSPIRV, file: "wgsl-s.spvasm"},
				{kind: rustOutMSL, file: "wgsl-s.metal"},
				{kind: rustOutHLSL, file: "wgsl-s.hlsl"},
			},
		},
		{
			name:   "SPIR-V per entry point",
			params: rustParams{targets: rustTargetSPIRV, separateEntryPoints: true},
			eps:    eps[:2],
			want: []rustOutput{
				{kind: rustOutSPIRV, file: "wgsl-s.vs.spvasm", entryPoint: "vs", stage: ir.StageVertex},
				{kind: rustOutSPIRV, file: "wgsl-s.fs.spvasm", entryPoint: "fs", stage: ir.StageFragment},
			},
		},
		{
			name:   "GLSL per entry point with Rust stage names, minus the exclude list",
			params: rustParams{targets: rustTargetGLSL, glslExcludeList: map[string]bool{"fs": true}},
			eps:    eps,
			want: []rustOutput{
				{kind: rustOutGLSL, file: "wgsl-s.vs.Vertex.glsl", entryPoint: "vs", stage: ir.StageVertex},
				{kind: rustOutGLSL, file: "wgsl-s.ts.Task.glsl", entryPoint: "ts", stage: ir.StageTask},
				{kind: rustOutGLSL, file: "wgsl-s.ms.Mesh.glsl", entryPoint: "ms", stage: ir.StageMesh},
				{kind: rustOutGLSL, file: "wgsl-s.cs.Compute.glsl", entryPoint: "cs", stage: ir.StageCompute},
			},
		},
		{
			name:   "GLSL without entry points writes nothing; MSL still writes a file",
			params: rustParams{targets: rustTargetGLSL | rustTargetMETAL},
			eps:    nil,
			want:   []rustOutput{{kind: rustOutMSL, file: "wgsl-s.metal"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expectedRustOutputs("s", tt.params, tt.eps)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expectedRustOutputs() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestExpectedRustIROutputs(t *testing.T) {
	if got := expectedRustIROutputs("s"); !reflect.DeepEqual(got, []string{"wgsl-s.ron", "wgsl-s.compact.ron"}) {
		t.Errorf("expectedRustIROutputs() = %v", got)
	}
}

func TestRustAuxiliaryOutput(t *testing.T) {
	tests := []struct {
		kind, file string
		want       bool
	}{
		{rustOutSPIRV, "wgsl-s.spvasm.glsl", true},    // SPIRV-Cross review aid
		{rustOutSPIRV, "wgsl-s.vs.spvasm.glsl", true}, // per entry point
		{rustOutHLSL, "wgsl-s.ron", true},             // DXC profile config
		{rustOutSPIRV, "wgsl-s.spvasm", false},
		{rustOutHLSL, "wgsl-s.hlsl", false},
		{rustOutMSL, "wgsl-s.ron", false},
	}
	for _, tt := range tests {
		if got := isRustAuxiliaryOutput(tt.kind, tt.file); got != tt.want {
			t.Errorf("isRustAuxiliaryOutput(%s, %s) = %v, want %v", tt.kind, tt.file, got, tt.want)
		}
	}
}
