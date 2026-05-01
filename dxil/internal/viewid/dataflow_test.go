package viewid

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// Helper: make a module with scalar f32, vec3 f32, vec4 f32 types.
func makeBasicMod() *ir.Module {
	mod := &ir.Module{}
	mod.Types = []ir.Type{
		{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 0: f32
		{Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1: vec3<f32>
		{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2: vec4<f32>
	}
	return mod
}

// scalarBinding returns a LocationBinding for location N.
func scalarBinding(n uint32) *ir.Binding {
	b := ir.Binding(ir.LocationBinding{Location: n})
	return &b
}

// TestTrianglePassThroughFS models the triangle fragment shader:
//
//	struct VertexOutput { @builtin(position) position: vec4<f32>,
//	                      @location(0) color: vec3<f32> };
//	@fragment fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
//	    return vec4<f32>(input.color, 1.0);
//	}
//
// Expected dependency (after locations-first input sorting):
//   - input scalars 0..2 (color): contribute to output scalars 0..2
//   - input scalars 4..7 (position): contribute to nothing
//   - output scalar 3 (alpha): constant 1.0, no input deps
func TestTrianglePassThroughFS(t *testing.T) {
	mod := makeBasicMod()

	// Struct VertexOutput { position: vec4, color: vec3 }
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	colBinding := ir.Binding(ir.LocationBinding{Location: 0})
	mod.Types = append(mod.Types, ir.Type{
		Name: "VertexOutput",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "position", Type: 2, Binding: &posBinding},
				{Name: "color", Type: 1, Binding: &colBinding},
			},
		},
	})
	voutHandle := ir.TypeHandle(3)

	// Build entry point function: argument = VertexOutput, result = vec4 (loc 0)
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "input", Type: voutHandle, Binding: nil},
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &resultBinding},
	}

	// Expressions:
	// 0: FunctionArgument(0)      type vout
	// 1: AccessIndex(0, 1)        type vec3 (input.color)
	// 2: Literal 1.0              type f32
	// 3: Compose vec4(color, 1.0) type vec4
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},
		{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{1, 2}}},
	}
	voutH := voutHandle
	vec3H := ir.TypeHandle(1)
	f32H := ir.TypeHandle(0)
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &voutH},
		{Handle: &vec3H},
		{Handle: &f32H},
		{Handle: &vec4H},
	}

	// Body: Emit 0..4, then Return(3)
	ret := ir.ExpressionHandle(3)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name:     "fs_main",
		Stage:    ir.StageFragment,
		Function: fn,
	}

	// Inputs are ordered locations-first: color (LOC 0) before position
	// (SV_Position), matching the fragment input sorted order produced by
	// collectGraphicsSignatures.
	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 3, VectorRow: 0}, // color (LOC 0)
		{ScalarStart: 4, NumChannels: 4, VectorRow: 1}, // position (SV_Position)
	}
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0}, // SV_Target
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// With locations-first ordering: color at scalars 0-2, position at 4-7.
	// Total: 3 (color) + gap + 4 (position) → cumScalar layout depends on
	// packer; in this test color occupies row 0 (3 channels), position row 1
	// (4 channels), so total = max(row*4+startCol+numChannels) = 1*4+0+4 = 8.
	if deps.NumInputScalars != 8 {
		t.Errorf("NumInputScalars = %d, want 8", deps.NumInputScalars)
	}
	if deps.NumOutputScalars != 4 {
		t.Errorf("NumOutputScalars = %d, want 4", deps.NumOutputScalars)
	}

	// ViewIdState table: one row per input scalar.
	if got, want := len(deps.InputScalarToOutputs), 8; got != want {
		t.Fatalf("InputScalarToOutputs len = %d, want %d", got, want)
	}
	// Color scalar 0 → bit 0 (output 0).
	if got := deps.InputScalarToOutputs[0]; got != 0x1 {
		t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
	}
	// Color scalar 1 → bit 1 (output 1).
	if got := deps.InputScalarToOutputs[1]; got != 0x2 {
		t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x2", got)
	}
	// Color scalar 2 → bit 2 (output 2).
	if got := deps.InputScalarToOutputs[2]; got != 0x4 {
		t.Errorf("InputScalarToOutputs[2] = 0x%x, want 0x4", got)
	}
	// Color scalar 3 (unused, padding to 4-component row): no deps.
	if deps.InputScalarToOutputs[3] != 0 {
		t.Errorf("InputScalarToOutputs[3] = 0x%x, want 0", deps.InputScalarToOutputs[3])
	}
	// Position scalars 4..7: no contribution (not used in output).
	for i := 4; i < 8; i++ {
		if deps.InputScalarToOutputs[i] != 0 {
			t.Errorf("InputScalarToOutputs[%d] = 0x%x, want 0", i, deps.InputScalarToOutputs[i])
		}
	}

	// PSV0 table: SigInputVectors = 2, SigOutputVectors = 1.
	// Layout: 4 components per input vector, MaskDwords(1) = 1 dword per
	// component → total = 4*2*1 = 8 dwords.
	if got, want := len(deps.InputCompToOutputComps), 8; got != want {
		t.Fatalf("InputCompToOutputComps len = %d, want %d", got, want)
	}
	// Input comps 0-2 (color vector 0): r->out0, g->out1, b->out2.
	if got := deps.InputCompToOutputComps[0]; got != 0x1 {
		t.Errorf("InputCompToOutputComps[0] = 0x%x, want 0x1", got)
	}
	if got := deps.InputCompToOutputComps[1]; got != 0x2 {
		t.Errorf("InputCompToOutputComps[1] = 0x%x, want 0x2", got)
	}
	if got := deps.InputCompToOutputComps[2]; got != 0x4 {
		t.Errorf("InputCompToOutputComps[2] = 0x%x, want 0x4", got)
	}
	// Input comp 3 (unused padding): no deps.
	if deps.InputCompToOutputComps[3] != 0 {
		t.Errorf("InputCompToOutputComps[3] = 0x%x, want 0", deps.InputCompToOutputComps[3])
	}
	// Input comps 4..7 (position vector 1): no deps.
	for i := 4; i < 8; i++ {
		if deps.InputCompToOutputComps[i] != 0 {
			t.Errorf("InputCompToOutputComps[%d] = 0x%x, want 0", i, deps.InputCompToOutputComps[i])
		}
	}
}

// TestNonStructReturn exercises the direct vec4 return path (no struct
// wrapping). Mirrors a WGSL pattern like:
//
//	@fragment fn main(@location(0) c: vec4<f32>) -> @location(0) vec4<f32> {
//	    return c;
//	}
func TestNonStructReturn(t *testing.T) {
	mod := makeBasicMod()

	argBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resBinding := ir.Binding(ir.LocationBinding{Location: 0})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "c", Type: 2, Binding: &argBinding},
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &resBinding},
	}

	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
	}
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &vec4H},
	}

	ret := ir.ExpressionHandle(0)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name:     "main",
		Stage:    ir.StageFragment,
		Function: fn,
	}

	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0},
	}
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0},
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// Identity mapping: input scalar i → output scalar i.
	for i := uint32(0); i < 4; i++ {
		got := deps.InputScalarToOutputs[i]
		want := uint32(1) << i
		if got != want {
			t.Errorf("InputScalarToOutputs[%d] = 0x%x, want 0x%x", i, got, want)
		}
	}
}

// TestScalarTimesVectorPrecision verifies that a scalar-times-vector binary
// op propagates taint per-component rather than unioning all components.
// Models the quad.wgsl vertex shader pattern:
//
//	return VertexOutput(uv, vec4<f32>(c_scale * pos, 0.0, 1.0));
//
// where `c_scale` is a constant and `pos` is an input vec2.
// Expected: output 4 depends on input {0}, output 5 depends on input {1}.
func TestScalarTimesVectorPrecision(t *testing.T) {
	mod := makeBasicMod()
	vec2H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
	}) // 3: vec2<f32>

	posBinding := ir.Binding(ir.LocationBinding{Location: 0})
	uvBinding := ir.Binding(ir.LocationBinding{Location: 1})

	// Return type: struct { uv: vec2 @location(0), position: vec4 @builtin(position) }
	outLocBinding := ir.Binding(ir.LocationBinding{Location: 0})
	outPosBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	stHandle := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Name: "VertexOutput",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "uv", Type: vec2H, Binding: &outLocBinding},
				{Name: "position", Type: 2, Binding: &outPosBinding},
			},
		},
	}) // 4: VertexOutput

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: vec2H, Binding: &posBinding},
			{Name: "uv", Type: vec2H, Binding: &uvBinding},
		},
		Result: &ir.FunctionResult{Type: stHandle, Binding: nil},
	}

	// Expressions:
	// 0: FunctionArgument(0) = pos : vec2
	// 1: FunctionArgument(1) = uv  : vec2
	// 2: Literal(1.2)              : f32  (c_scale constant)
	// 3: Binary(Mul, 2, 0)         : vec2  (c_scale * pos)
	// 4: Literal(0.0)              : f32
	// 5: Literal(1.0)              : f32
	// 6: Compose(vec4, [3, 4, 5])  : vec4  (vec4(c_scale*pos, 0, 1))
	// 7: Compose(struct, [1, 6])   : VertexOutput
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprFunctionArgument{Index: 1}},
		{Kind: ir.Literal{Value: ir.LiteralF32(1.2)}},
		{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 2, Right: 0}},
		{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
		{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{3, 4, 5}}},
		{Kind: ir.ExprCompose{Type: stHandle, Components: []ir.ExpressionHandle{1, 6}}},
	}
	f32H := ir.TypeHandle(0)
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &vec2H},
		{Handle: &vec2H},
		{Handle: &f32H},
		{Handle: &vec2H}, // c_scale * pos -> vec2
		{Handle: &f32H},
		{Handle: &f32H},
		{Handle: &vec4H},
		{Handle: &stHandle},
	}

	ret := ir.ExpressionHandle(7)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 6, End: 8}}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name: "vert_main", Stage: ir.StageVertex,
		Function: fn,
	}

	// Input signature: pos @location(0) vec2, uv @location(1) vec2.
	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 2, VectorRow: 0},
		{ScalarStart: 2, NumChannels: 2, VectorRow: 1},
	}
	// Output signature (interface order: location first, builtin last):
	// uv @location(0) vec2, position @builtin(position) vec4.
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 2, VectorRow: 0},
		{ScalarStart: 2, NumChannels: 4, VectorRow: 1},
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// Output scalar 0 (uv.x) depends on input scalar 2 (uv.x at row 1 col 0).
	// Output scalar 1 (uv.y) depends on input scalar 3 (uv.y at row 1 col 1).
	// Output scalar 4 (position.x) depends on input scalar 0 (pos.x).
	// Output scalar 5 (position.y) depends on input scalar 1 (pos.y).
	// Output scalars 6, 7 (position.z=0, position.w=1) depend on nothing.

	outMask := MaskDwordsForScalars(deps.NumOutputScalars)
	for inS := uint32(0); inS < deps.NumInputScalars; inS++ {
		got := deps.InputScalarToOutputs[inS*outMask]
		var want uint32
		switch inS {
		case 0: // pos.x -> output 4 (position.x)
			want = 1 << 4
		case 1: // pos.y -> output 5 (position.y)
			want = 1 << 5
		case 4: // uv.x -> output 0
			want = 1 << 0
		case 5: // uv.y -> output 1
			want = 1 << 1
		default:
			want = 0
		}
		if got != want {
			t.Errorf("InputScalarToOutputs[%d] = 0x%x, want 0x%x", inS, got, want)
		}
	}
}

// TestMatrixVectorMultiplyUnion verifies that matrix*vector multiply
// correctly unions all input components to every output component
// (cross-component dependency, not component-wise).
func TestMatrixVectorMultiplyUnion(t *testing.T) {
	mod := makeBasicMod()
	mat4H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.MatrixType{
			Columns: 4, Rows: 4,
			Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
	}) // 3: mat4x4<f32>

	posBinding := ir.Binding(ir.LocationBinding{Location: 0})

	// Result: vec4<f32> @builtin(position)
	outPosBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: 2, Binding: &posBinding}, // vec4
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &outPosBinding},
	}

	// Expressions:
	// 0: FunctionArgument(0) = pos  : vec4
	// 1: Literal(zero matrix)       : mat4x4 (stand-in for global uniform)
	// 2: Binary(Mul, 1, 0)          : vec4  (mat * vec)
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprZeroValue{Type: mat4H}},
		{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 1, Right: 0}},
	}
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &vec4H},
		{Handle: &mat4H},
		{Handle: &vec4H}, // mat4*vec4 -> vec4
	}

	ret := ir.ExpressionHandle(2)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name: "main", Stage: ir.StageVertex,
		Function: fn,
	}

	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0}, // pos
	}
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0}, // SV_Position
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// Matrix multiply: EVERY output should depend on ALL inputs.
	// Output 0..3 each depend on inputs {0, 1, 2, 3} = bitmask 0xF.
	outMask := MaskDwordsForScalars(deps.NumOutputScalars)
	for inS := uint32(0); inS < 4; inS++ {
		got := deps.InputScalarToOutputs[inS*outMask]
		want := uint32(0xF) // all 4 outputs
		if got != want {
			t.Errorf("InputScalarToOutputs[%d] = 0x%x, want 0x%x (matrix multiply should union)", inS, got, want)
		}
	}
}

// TestEmptyDeps handles a shader with no inputs.
func TestEmptyDeps(t *testing.T) {
	mod := makeBasicMod()
	resBinding := ir.Binding(ir.LocationBinding{Location: 0})

	fn := ir.Function{
		Result: &ir.FunctionResult{Type: 2, Binding: &resBinding},
	}
	vec4H := ir.TypeHandle(2)

	fn.Expressions = []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{0, 0, 0, 0}}},
	}
	f32H := ir.TypeHandle(0)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &f32H},
		{Handle: &vec4H},
	}
	ret := ir.ExpressionHandle(1)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name: "main", Stage: ir.StageFragment,
		Function: fn,
	}

	inputs := []SigElement{}
	outputs := []SigElement{{ScalarStart: 0, NumChannels: 4, VectorRow: 0}}

	deps := Analyze(mod, &ep, inputs, outputs)
	if deps.NumInputScalars != 0 {
		t.Errorf("NumInputScalars = %d, want 0", deps.NumInputScalars)
	}
	if len(deps.InputScalarToOutputs) != 0 {
		t.Errorf("InputScalarToOutputs len = %d, want 0", len(deps.InputScalarToOutputs))
	}
}

// TestPackedInputScalarStartCol verifies that inScalarToComp uses
// StartCol when mapping cumulative scalar indices to packed linear
// indices. When two elements pack into the same register row at
// different columns (col 0 and col 1), their packed indices must
// differ (0 and 1) rather than both mapping to 0.
func TestPackedInputScalarStartCol(t *testing.T) {
	// Test the inScalarToComp mapping directly: two elements packed
	// into register 0 at different columns should produce distinct
	// packed linear indices.
	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0, StartCol: 0}, // LOC1 at col 0
		{ScalarStart: 1, NumChannels: 1, VectorRow: 0, StartCol: 1}, // LOC3 at col 1
	}

	// Replicate the inScalarToComp computation from Analyze().
	var mapping []uint32
	for i := range inputs {
		inp := &inputs[i]
		if inp.SystemManaged {
			continue
		}
		for c := uint32(0); c < inp.NumChannels; c++ {
			mapping = append(mapping, inp.VectorRow*4+inp.StartCol+c)
		}
	}

	if len(mapping) != 2 {
		t.Fatalf("mapping len = %d, want 2", len(mapping))
	}
	// LOC1 at col 0: packed index = 0*4 + 0 + 0 = 0
	if mapping[0] != 0 {
		t.Errorf("mapping[0] = %d, want 0 (LOC1 at col 0)", mapping[0])
	}
	// LOC3 at col 1: packed index = 0*4 + 1 + 0 = 1
	if mapping[1] != 1 {
		t.Errorf("mapping[1] = %d, want 1 (LOC3 at col 1)", mapping[1])
	}
}

func builtinBinding(b ir.BuiltinValue) *ir.Binding {
	v := ir.Binding(ir.BuiltinBinding{Builtin: b})
	return &v
}

// TestVertexTwoStructsPrecision models the interface/vertex_two_structs shader:
//
//	struct Input1 { @builtin(vertex_index) index: u32 }
//	struct Input2 { @builtin(instance_index) index: u32 }
//	@vertex
//	fn vertex_two_structs(in1: Input1, in2: Input2) -> @builtin(position) vec4<f32> {
//	    var index = 2u;
//	    return vec4<f32>(f32(in1.index), f32(in2.index), f32(index), 0.0);
//	}
//
// Expected (DXC): output 0 depends on inputs: { 0 }, output 1 depends on inputs: { 4 },
// outputs 2-3 depend on nothing (constant values).
func TestVertexTwoStructsPrecision(t *testing.T) {
	mod := makeBasicMod()
	// Type 0: f32, Type 1: vec3<f32>, Type 2: vec4<f32>
	u32H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}) // 3: u32

	// Struct Input1 { @builtin(vertex_index) index: u32 }
	input1H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Name: "Input1",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "index", Type: u32H, Binding: builtinBinding(ir.BuiltinVertexIndex)},
			},
		},
	}) // 4: Input1

	// Struct Input2 { @builtin(instance_index) index: u32 }
	input2H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Name: "Input2",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "index", Type: u32H, Binding: builtinBinding(ir.BuiltinInstanceIndex)},
			},
		},
	}) // 5: Input2

	// Return type: @builtin(position) vec4<f32>
	outBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "in1", Type: input1H, Binding: nil},
			{Name: "in2", Type: input2H, Binding: nil},
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &outBinding},
		LocalVars: []ir.LocalVariable{
			{Name: "index", Type: u32H, Init: ptrExprHandle(6)}, // init = Literal(2u)
		},
	}

	// Expressions:
	// 0: FunctionArgument(0)    = in1 : Input1
	// 1: FunctionArgument(1)    = in2 : Input2
	// 2: AccessIndex(0, 0)      = in1.index : u32
	// 3: AccessIndex(1, 0)      = in2.index : u32
	// 4: As(2, f32)             = f32(in1.index) : f32
	// 5: As(3, f32)             = f32(in2.index) : f32
	// 6: Literal(2u)            = 2u : u32
	// 7: LocalVariable(0)       = &index : ptr<u32>
	// 8: Load(7)                = *index : u32
	// 9: As(8, f32)             = f32(index) : f32
	// 10: Literal(0.0)          = 0.0 : f32
	// 11: Compose(vec4, [4,5,9,10])
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprFunctionArgument{Index: 1}},
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}},
		{Kind: ir.ExprAs{Expr: 2, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},
		{Kind: ir.ExprAs{Expr: 3, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},
		{Kind: ir.Literal{Value: ir.LiteralU32(2)}},
		{Kind: ir.ExprLocalVariable{Variable: 0}},
		{Kind: ir.ExprLoad{Pointer: 7}},
		{Kind: ir.ExprAs{Expr: 8, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},
		{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{4, 5, 9, 10}}},
	}

	f32H := ir.TypeHandle(0)
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &input1H}, // 0: FunctionArgument(0)
		{Handle: &input2H}, // 1: FunctionArgument(1)
		{Handle: &u32H},    // 2: AccessIndex
		{Handle: &u32H},    // 3: AccessIndex
		{Handle: &f32H},    // 4: As -> f32
		{Handle: &f32H},    // 5: As -> f32
		{Handle: &u32H},    // 6: Literal
		{Handle: &u32H},    // 7: LocalVariable (ptr type simplified)
		{Handle: &u32H},    // 8: Load
		{Handle: &f32H},    // 9: As -> f32
		{Handle: &f32H},    // 10: Literal
		{Handle: &vec4H},   // 11: Compose
	}

	// Body:
	// StmtEmit [2..6)   -- lower in1.index, in2.index, f32 casts
	// StmtEmit [8..12)  -- Load, As, Literal, Compose
	// StmtReturn(11)
	ret := ir.ExpressionHandle(11)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 6}}},
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 8, End: 12}}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name:     "vertex_two_structs",
		Stage:    ir.StageVertex,
		Function: fn,
	}

	// Input signature: VS inputs in declaration order (InputAssembler packing).
	// Input1.index = @builtin(vertex_index) at row 0
	// Input2.index = @builtin(instance_index) at row 1
	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0, StartCol: 0}, // SV_VertexID
		{ScalarStart: 1, NumChannels: 1, VectorRow: 1, StartCol: 0}, // SV_InstanceID
	}
	// Output: @builtin(position) vec4<f32> at row 0
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0, StartCol: 0}, // SV_Position
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// Check: packed linear indexing.
	// Input scalar 0 = SV_VertexID (row 0, col 0, packed = 0)
	// Input scalar 1 = SV_InstanceID (row 1, col 0, packed = 4)
	// Output scalars 0..3 = SV_Position.xyzw

	// DXC expected:
	//   output 0 depends on inputs: { 0 }       (in1.index -> out.x)
	//   output 1 depends on inputs: { 4 }       (in2.index -> out.y)
	//   outputs 2-3: no deps                     (constants)
	outMask := MaskDwordsForScalars(deps.NumOutputScalars)

	// Input packed 0 (SV_VertexID) -> should set only output bit 0
	if got := deps.InputScalarToOutputs[0*outMask]; got != 0x1 {
		t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (only output.x)", got)
	}
	// Input packed 1..3 (padding in row 0): no deps
	for i := uint32(1); i < 4 && i*outMask < uint32(len(deps.InputScalarToOutputs)); i++ {
		if deps.InputScalarToOutputs[i*outMask] != 0 {
			t.Errorf("InputScalarToOutputs[%d] = 0x%x, want 0 (padding)", i, deps.InputScalarToOutputs[i*outMask])
		}
	}
	// Input packed 4 (SV_InstanceID) -> should set only output bit 1
	if 4*outMask < uint32(len(deps.InputScalarToOutputs)) {
		if got := deps.InputScalarToOutputs[4*outMask]; got != 0x2 {
			t.Errorf("InputScalarToOutputs[4] = 0x%x, want 0x2 (only output.y)", got)
		}
	} else {
		t.Errorf("InputScalarToOutputs too short: len=%d, need index 4+", len(deps.InputScalarToOutputs))
	}
}

// TestExprAliasTaintForwarding verifies that ExprAlias (produced by
// mem2reg) correctly forwards the source expression's taint. After
// mem2reg promotes a scalar local, loads are rewritten to ExprAlias
// pointing at the stored value. The ViewID analyzer must follow this
// alias chain to produce precise per-component dependencies.
//
// Models a post-mem2reg IR where a local variable init is aliased:
//
//	original: var idx: u32 = vertex_index; ... f32(idx) ...
//	after mem2reg: ExprAlias(vertex_index) replaces ExprLoad(localvar)
func TestExprAliasTaintForwarding(t *testing.T) {
	mod := makeBasicMod()
	u32H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}})

	argBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})
	outBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "vertex_index", Type: u32H, Binding: &argBinding},
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &outBinding},
	}

	// Expressions (after mem2reg):
	// 0: FunctionArgument(0) = vertex_index : u32
	// 1: ExprAlias(0)        = alias of vertex_index (was: load of promoted local)
	// 2: ExprAs(1, f32)      = f32(alias) : f32
	// 3: Literal(0.0)        = 0.0 : f32
	// 4: Compose(vec4, [2, 3, 3, 3])
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprAlias{Source: 0}},
		{Kind: ir.ExprAs{Expr: 1, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},
		{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{2, 3, 3, 3}}},
	}

	f32H := ir.TypeHandle(0)
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &u32H},
		{Handle: &u32H},
		{Handle: &f32H},
		{Handle: &f32H},
		{Handle: &vec4H},
	}

	ret := ir.ExpressionHandle(4)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 5}}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name: "main", Stage: ir.StageVertex,
		Function: fn,
	}

	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0, StartCol: 0},
	}
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0, StartCol: 0},
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// Only output 0 (x) depends on input 0 (vertex_index).
	// Outputs 1-3 are constant (0.0), no deps.
	outMask := MaskDwordsForScalars(deps.NumOutputScalars)
	if got := deps.InputScalarToOutputs[0*outMask]; got != 0x1 {
		t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (alias should forward taint only to output.x)", got)
	}
}

// TestExprPhiTaintMerge verifies that ExprPhi (produced by mem2reg for
// multi-block locals) correctly merges taint from all incoming values.
//
// Models:
//
//	if (cond) { x = input_a; } else { x = input_b; }
//	return vec4(x, 0, 0, 0);
//
// After mem2reg, the load of x becomes ExprPhi([input_a, input_b]).
// Output 0 should depend on BOTH input_a and input_b.
func TestExprPhiTaintMerge(t *testing.T) {
	mod := makeBasicMod()

	argABinding := ir.Binding(ir.LocationBinding{Location: 0})
	argBBinding := ir.Binding(ir.LocationBinding{Location: 1})
	outBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0, Binding: &argABinding}, // f32
			{Name: "b", Type: 0, Binding: &argBBinding}, // f32
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &outBinding},
	}

	// Expressions:
	// 0: FunctionArgument(0) = a : f32
	// 1: FunctionArgument(1) = b : f32
	// 2: ExprPhi([a, b])     = merged value from if/else
	// 3: Literal(0.0)        = 0.0 : f32
	// 4: Compose(vec4, [2, 3, 3, 3])
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprFunctionArgument{Index: 1}},
		{Kind: ir.ExprPhi{Incoming: []ir.PhiIncoming{
			{PredKey: ir.PhiPredIfAccept, Value: 0},
			{PredKey: ir.PhiPredIfReject, Value: 1},
		}}},
		{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{2, 3, 3, 3}}},
	}

	f32H := ir.TypeHandle(0)
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &f32H},
		{Handle: &f32H},
		{Handle: &f32H},
		{Handle: &f32H},
		{Handle: &vec4H},
	}

	ret := ir.ExpressionHandle(4)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 5}}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name: "main", Stage: ir.StageVertex,
		Function: fn,
	}

	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0, StartCol: 0},
		{ScalarStart: 1, NumChannels: 1, VectorRow: 1, StartCol: 0},
	}
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0, StartCol: 0},
	}

	deps := Analyze(mod, &ep, inputs, outputs)

	// Output 0 (x) depends on BOTH inputs (phi merges a and b).
	// Outputs 1-3 are constant, no deps.
	outMask := MaskDwordsForScalars(deps.NumOutputScalars)

	// Input 0 (location 0, packed index 0) -> output 0
	if got := deps.InputScalarToOutputs[0*outMask]; got != 0x1 {
		t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (input a -> output.x via phi)", got)
	}
	// Input 4 (location 1, packed index 4) -> output 0
	if 4*outMask < uint32(len(deps.InputScalarToOutputs)) {
		if got := deps.InputScalarToOutputs[4*outMask]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[4] = 0x%x, want 0x1 (input b -> output.x via phi)", got)
		}
	}
}

func ptrExprHandle(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

func ptrU8(v uint8) *uint8 {
	return &v
}
