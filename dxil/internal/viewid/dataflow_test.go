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
