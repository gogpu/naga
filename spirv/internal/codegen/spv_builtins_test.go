package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestBuiltinToSPIRVSpirvOnlyBuiltins pins the SPIR-V spvenums.BuiltIn numbers of the
// built-ins WGSL cannot produce (SPIR-V core grammar: CullDistance 4,
// PointCoord 16, WorkgroupSize 25; SPV_KHR_shader_draw_parameters: BaseVertex
// 4424, BaseInstance 4425). A wrong number is a silently invalid interface.
func TestBuiltinToSPIRVSpirvOnlyBuiltins(t *testing.T) {
	tests := []struct {
		name    string
		builtin ir.BuiltinValue
		sc      spvenums.StorageClass
		want    spvenums.BuiltIn
	}{
		{"cull_distance", ir.BuiltinCullDistance, spvenums.StorageClassOutput, 4},
		{"point_coord", ir.BuiltinPointCoord, spvenums.StorageClassInput, 16},
		{"base_vertex", ir.BuiltinBaseVertex, spvenums.StorageClassInput, 4424},
		{"base_instance", ir.BuiltinBaseInstance, spvenums.StorageClassInput, 4425},
		// SPIR-V decorates WorkgroupSize on a constant; the variable form is
		// invalid, so the backend refuses it (see checkBuiltins).
		{"workgroup_size", ir.BuiltinWorkGroupSize, spvenums.StorageClassInput, builtInUnmapped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := builtinToSPIRV(tt.builtin, tt.sc); got != tt.want {
				t.Errorf("builtinToSPIRV(%v) = %d, want %d", tt.builtin, got, tt.want)
			}
		})
	}
}

// TestCapability_BuiltinCullDistance verifies that writing
// @builtin(cull_distance) declares spvenums.Capability CullDistance (Rust naga
// back/spv/writer.rs:3226).
func TestCapability_BuiltinCullDistance(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// 0: f32
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// 1: array<f32, 1>
			{Name: "array_f32_1", Inner: ir.ArrayType{
				Base:   0,
				Size:   ir.ArraySize{Constant: ptrUint32(1)},
				Stride: 4,
			}},
			// 2: vec4<f32>
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// 3: output struct
			{Name: "VertexOutput", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "position", Type: 2, Offset: 0, Binding: makeBuiltinBinding(ir.BuiltinPosition)},
					{Name: "cull_distance", Type: 1, Offset: 16, Binding: makeBuiltinBinding(ir.BuiltinCullDistance)},
				},
				Span: 20,
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Result: &ir.FunctionResult{
						Type: 3,
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprZeroValue{Type: 3}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	spvBytes := compileModule(t, module)
	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, spvenums.CapabilityCullDistance)
}
