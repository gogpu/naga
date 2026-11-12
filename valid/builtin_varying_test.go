package valid

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestBuiltinVaryingStageAndType pins the stage, direction and type rules of
// the built-ins the IR gained for the SPIR-V frontend against Rust naga's
// VaryingContext::validate_impl (valid/interface.rs:291-405). WGSL cannot
// spell them, so the modules are built by hand, as the frontend will.
func TestBuiltinVaryingStageAndType(t *testing.T) {
	two := uint32(2)
	binding := func(b ir.BuiltinValue) *ir.Binding {
		var bind ir.Binding = ir.BuiltinBinding{Builtin: b}
		return &bind
	}
	types := []ir.Type{
		{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                              // 0
		{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                             // 1
		{Name: "vec2<f32>", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2
		{Name: "vec3<u32>", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},  // 3
		{Name: "vec4<f32>", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 4
		{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                              // 5
		{Name: "array<f32, 2>", Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &two}, Stride: 4}},            // 6
		{Name: "array<i32, 2>", Inner: ir.ArrayType{Base: 5, Size: ir.ArraySize{Constant: &two}, Stride: 4}},            // 7
		{Name: "Out", Inner: ir.StructType{Members: []ir.StructMember{ // 8
			{Name: "position", Type: 4, Offset: 0, Binding: binding(ir.BuiltinPosition)},
			{Name: "cull", Type: 6, Offset: 16, Binding: binding(ir.BuiltinCullDistance)},
		}, Span: 32}},
	}

	handle := func(i ir.TypeHandle) *ir.TypeHandle { return &i }

	tests := []struct {
		name    string
		stage   ir.ShaderStage
		argTy   *ir.TypeHandle
		argB    *ir.Binding
		resTy   *ir.TypeHandle
		resB    *ir.Binding
		wantErr string
	}{
		{
			name:  "base_instance vertex input",
			stage: ir.StageVertex, argTy: handle(0), argB: binding(ir.BuiltinBaseInstance),
			resTy: handle(4), resB: binding(ir.BuiltinPosition),
		},
		{
			name:  "base_instance fragment output",
			stage: ir.StageFragment, resTy: handle(0), resB: binding(ir.BuiltinBaseInstance),
			wantErr: "Built-in BaseInstance is not available at this stage",
		},
		{
			name:  "base_vertex compute input",
			stage: ir.StageCompute, argTy: handle(0), argB: binding(ir.BuiltinBaseVertex),
			wantErr: "Built-in BaseVertex is not available at this stage",
		},
		{
			name:  "point_coord fragment input",
			stage: ir.StageFragment, argTy: handle(2), argB: binding(ir.BuiltinPointCoord),
		},
		{
			name:  "point_coord vertex input",
			stage: ir.StageVertex, argTy: handle(2), argB: binding(ir.BuiltinPointCoord),
			resTy: handle(4), resB: binding(ir.BuiltinPosition),
			wantErr: "Built-in PointCoord is not available at this stage",
		},
		{
			name:  "workgroup_size compute input",
			stage: ir.StageCompute, argTy: handle(3), argB: binding(ir.BuiltinWorkGroupSize),
		},
		{
			name:  "workgroup_size wrong type",
			stage: ir.StageCompute, argTy: handle(2), argB: binding(ir.BuiltinWorkGroupSize),
			wantErr: "Built-in type for WorkGroupSize is invalid. Found Vector { size: Bi, scalar: Scalar { kind: Float, width: 4 } }",
		},
		{
			name:  "cull_distance vertex output",
			stage: ir.StageVertex, resTy: handle(8),
		},
		{
			name:  "cull_distance signed array",
			stage: ir.StageVertex, resTy: handle(7), resB: binding(ir.BuiltinCullDistance),
			wantErr: "Built-in type for CullDistance is invalid. Found Array { base: [5], size: Constant(2), stride: 4 }",
		},
		{
			name:  "cull_distance fragment output",
			stage: ir.StageFragment, resTy: handle(6), resB: binding(ir.BuiltinCullDistance),
			wantErr: "Built-in CullDistance is not available at this stage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := ir.Function{Name: "main"}
			if tt.argTy != nil {
				fn.Arguments = []ir.FunctionArgument{{Name: "arg", Type: *tt.argTy, Binding: tt.argB}}
			}
			if tt.resTy != nil {
				fn.Result = &ir.FunctionResult{Type: *tt.resTy, Binding: tt.resB}
			}
			ep := ir.EntryPoint{Name: "main", Stage: tt.stage, Function: fn}
			if tt.stage == ir.StageCompute {
				ep.Workgroup = [3]uint32{1, 1, 1}
			}
			module := &ir.Module{Types: types, EntryPoints: []ir.EntryPoint{ep}}
			errs, err := validateErrs(module, CapAll)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("expected no errors, got: %v", errs)
				}
				return
			}
			if len(errs) == 0 || !containsError(errs, tt.wantErr) {
				t.Fatalf("expected an error containing %q, got: %v", tt.wantErr, errs)
			}
		})
	}
}
