package snapshot_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// Shared type indices of builtinProbeTypes.
const (
	tyU32 = iota
	tyF32
	tyVec2F
	tyVec3U
	tyVec4F
	tyArrayF1
	tyVertexOut
)

// builtinProbeTypes is the type table of the probes below.
func builtinProbeTypes() []ir.Type {
	one := uint32(1)
	pos := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	cull := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinCullDistance})
	return []ir.Type{
		{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{Name: "vec2<f32>", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		{Name: "vec3<u32>", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		{Name: "vec4<f32>", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		{Name: "array<f32, 1>", Inner: ir.ArrayType{Base: tyF32, Size: ir.ArraySize{Constant: &one}, Stride: 4}},
		{Name: "VertexOut", Inner: ir.StructType{Members: []ir.StructMember{
			{Name: "position", Type: tyVec4F, Offset: 0, Binding: &pos},
			{Name: "cull", Type: tyArrayF1, Offset: 16, Binding: &cull},
		}, Span: 32}},
	}
}

// builtinBinding returns a *ir.Binding holding a built-in binding.
func builtinBinding(b ir.BuiltinValue) *ir.Binding {
	var bind ir.Binding = ir.BuiltinBinding{Builtin: b}
	return &bind
}

// builtinProbe builds a one-entry-point module over builtinProbeTypes.
func builtinProbe(stage ir.ShaderStage, argType ir.TypeHandle, argBinding *ir.Binding, resultType ir.TypeHandle, resultBinding *ir.Binding, workgroup [3]uint32) *ir.Module {
	fn := ir.Function{Name: "main"}
	if argBinding != nil {
		fn.Arguments = []ir.FunctionArgument{{Name: "arg", Type: argType, Binding: argBinding}}
	}
	if resultType != 0 || resultBinding != nil {
		fn.Result = &ir.FunctionResult{Type: resultType, Binding: resultBinding}
		// A zero value of the result type, so the module is valid IR and the
		// only thing a backend can refuse is the built-in itself. The zero
		// value is a pre-emit expression: it is in scope without an Emit.
		z := ir.ExpressionHandle(0)
		fn.Expressions = []ir.Expression{{Kind: ir.ExprZeroValue{Type: resultType}}}
		fn.Body = []ir.Statement{{Kind: ir.StmtReturn{Value: &z}}}
	}
	return &ir.Module{
		Types: builtinProbeTypes(),
		EntryPoints: []ir.EntryPoint{{
			Name: "main", Stage: stage, Workgroup: workgroup, Function: fn,
		}},
	}
}

// compileBackends runs every backend's public Compile on a probe module.
// GLSL targets 4.60, where the draw-parameter built-ins are core.
func compileBackends() []struct {
	name string
	fn   func(*ir.Module) error
} {
	return []struct {
		name string
		fn   func(*ir.Module) error
	}{
		{"spirv", func(m *ir.Module) error { _, _, err := spirv.Compile(m, spirv.DefaultOptions()); return err }},
		{"msl", func(m *ir.Module) error { _, _, err := msl.Compile(m, msl.DefaultOptions()); return err }},
		{"hlsl", func(m *ir.Module) error { _, _, err := hlsl.Compile(m, hlsl.DefaultOptions()); return err }},
		{"glsl", func(m *ir.Module) error {
			opts := glsl.DefaultOptions()
			opts.LangVersion = glsl.Version460
			_, _, err := glsl.Compile(m, opts)
			return err
		}},
		{"dxil", func(m *ir.Module) error { _, _, err := dxil.Compile(m, dxil.DefaultOptions()); return err }},
	}
}

// TestBackendBuiltinsAtCompile pins the public Compile behavior of the five
// built-ins the IR gained: an expressible one compiles, an inexpressible one
// is refused with an error naming it, and @builtin(cull_distance) is refused
// by every backend's capability gate exactly as Rust naga refuses it. The
// inner-layer tests alone cannot see the gate.
func TestBackendBuiltinsAtCompile(t *testing.T) {
	backends := compileBackends()

	cull := builtinProbe(ir.StageVertex, 0, nil, tyVertexOut, nil, [3]uint32{})
	// The probe must be valid IR once the capability is granted, so the
	// refusals below prove the capability gate and nothing else.
	if _, err := valid.Validate(cull, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("the cull_distance probe is not valid IR: %v", err)
	}
	for _, bc := range backends {
		t.Run(bc.name+"/cull_distance_gate", func(t *testing.T) {
			err := bc.fn(cull)
			if err == nil || !strings.Contains(err.Error(), "CULL_DISTANCE") {
				t.Fatalf("Compile = %v, want a refusal mentioning CULL_DISTANCE", err)
			}
		})
	}

	zero := [3]uint32{}
	workgroup := [3]uint32{1, 1, 1}
	vertexResult := func() *ir.Binding { return builtinBinding(ir.BuiltinPosition) }
	probes := []struct {
		name   string
		module *ir.Module
		// wantErr maps a backend name to the error substring it must return;
		// an empty string means Compile must succeed.
		wantErr map[string]string
	}{
		{
			name:   "base_instance",
			module: builtinProbe(ir.StageVertex, tyU32, builtinBinding(ir.BuiltinBaseInstance), tyVec4F, vertexResult(), zero),
			wantErr: map[string]string{
				"spirv": "", "msl": "", "glsl": "",
				"hlsl": "base_instance", "dxil": "base_instance",
			},
		},
		{
			name:   "base_vertex",
			module: builtinProbe(ir.StageVertex, tyU32, builtinBinding(ir.BuiltinBaseVertex), tyVec4F, vertexResult(), zero),
			wantErr: map[string]string{
				"spirv": "", "msl": "", "glsl": "",
				"hlsl": "base_vertex", "dxil": "base_vertex",
			},
		},
		{
			name:   "point_coord",
			module: builtinProbe(ir.StageFragment, tyVec2F, builtinBinding(ir.BuiltinPointCoord), 0, nil, zero),
			wantErr: map[string]string{
				"spirv": "", "msl": "", "glsl": "",
				"hlsl": "point_coord", "dxil": "point_coord",
			},
		},
		{
			name:   "workgroup_size",
			module: builtinProbe(ir.StageCompute, tyVec3U, builtinBinding(ir.BuiltinWorkGroupSize), 0, nil, workgroup),
			wantErr: map[string]string{
				"spirv": "workgroup_size", "glsl": "", "msl": "",
				"hlsl": "workgroup_size", "dxil": "workgroup_size",
			},
		},
	}
	for _, probe := range probes {
		// Each probe is valid IR with every capability on, so a backend's
		// refusal names its own limit, not a defect of the probe.
		if _, err := valid.Validate(probe.module, valid.Options{Capabilities: valid.CapAll}); err != nil {
			t.Fatalf("%s probe is not valid IR: %v", probe.name, err)
		}
		for _, bc := range backends {
			t.Run(bc.name+"/"+probe.name, func(t *testing.T) {
				want := probe.wantErr[bc.name]
				err := bc.fn(probe.module)
				if want == "" {
					if err != nil {
						t.Fatalf("Compile = %v, want success", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Compile = %v, want an error containing %q", err, want)
				}
			})
		}
	}

	// MSL 1.0 has no [[base_instance]]: the same probe must be refused there
	// and accepted at the default version (tested above).
	t.Run("msl/base_instance_needs_1_2", func(t *testing.T) {
		opts := msl.DefaultOptions()
		opts.LangVersion = msl.Version1_0
		mod := builtinProbe(ir.StageVertex, tyU32, builtinBinding(ir.BuiltinBaseInstance), tyVec4F, vertexResult(), zero)
		if _, _, err := msl.Compile(mod, opts); err == nil || !strings.Contains(err.Error(), "1.2") {
			t.Fatalf("Compile at MSL 1.0 = %v, want a refusal mentioning 1.2", err)
		}
	})
}
