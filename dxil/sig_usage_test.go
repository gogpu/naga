package dxil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// helper to make a minimal Module with types.
func makeModuleWithTypes(types ...ir.Type) *ir.Module {
	return &ir.Module{Types: types}
}

func TestComputeInputUsedMasks_EmptyBody(t *testing.T) {
	// Fragment shader with struct input but empty body — no argument accessed.
	// Mirrors the interpolate shader: fn frag_main(val: FragmentInput) { }
	f32Type := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 4}}
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var locBinding ir.Binding = ir.LocationBinding{Location: 0}
	structType := ir.Type{Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "position", Type: 1, Binding: &posBinding},
			{Name: "color", Type: 0, Binding: &locBinding},
		},
	}}
	irMod := makeModuleWithTypes(f32Type, vec4Type, structType)

	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "val", Type: 2}, // struct type
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
		Body: nil, // empty body
	}

	result := computeInputUsedMasks(irMod, fn)

	// With empty body, no expressions are alive, so no usage.
	if mask, ok := result[inputUsageKey{argIdx: 0, memberIdx: 0}]; ok && mask != 0 {
		t.Errorf("position member should not be used, got mask 0x%02x", mask)
	}
	if mask, ok := result[inputUsageKey{argIdx: 0, memberIdx: 1}]; ok && mask != 0 {
		t.Errorf("color member should not be used, got mask 0x%02x", mask)
	}
}

func TestComputeInputUsedMasks_DirectScalarArg(t *testing.T) {
	// fn main(@builtin(position) pos: vec4<f32>) -> @builtin(frag_depth) f32 {
	//     return pos.z - 0.1;
	// }
	// pos.z access: ExprFunctionArgument -> ExprAccessIndex{idx:2}
	f32Type := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 4}}
	irMod := makeModuleWithTypes(f32Type, vec4Type)

	// Expressions:
	// 0: FunctionArgument(0) - pos: vec4<f32>
	// 1: AccessIndex(base=0, index=2) - pos.z
	// 2: Literal(0.1)
	// 3: Binary(Sub, 1, 2) - pos.z - 0.1
	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: 1}, // vec4<f32>
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.1)}},
			{Kind: ir.ExprBinary{Op: ir.BinarySubtract, Left: 1, Right: 2}},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 4}}},
			{Kind: ir.StmtReturn{Value: ptrExprHandle(3)}},
		},
	}

	result := computeInputUsedMasks(irMod, fn)

	// Only component z (bit 2) should be used.
	key := inputUsageKey{argIdx: 0, memberIdx: -1}
	mask := result[key]
	if mask != 0x04 { // bit 2 = z
		t.Errorf("expected mask 0x04 (z only), got 0x%02x", mask)
	}
}

func TestComputeInputUsedMasks_StructMemberFullUse(t *testing.T) {
	// Shader reads in._varying (struct member 1, a float scalar) directly.
	// fn fragment(in: VertexOutput) -> f32 { return in._varying; }
	f32Type := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 4}}
	structType := ir.Type{Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "position", Type: 1}, // vec4
			{Name: "_varying", Type: 0}, // f32
		},
	}}
	irMod := makeModuleWithTypes(f32Type, vec4Type, structType)

	// Expressions:
	// 0: FunctionArgument(0) - the struct
	// 1: AccessIndex(base=0, index=1) - in._varying
	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "in", Type: 2},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
			{Kind: ir.StmtReturn{Value: ptrExprHandle(1)}},
		},
	}

	result := computeInputUsedMasks(irMod, fn)

	// Member 0 (position) should NOT be used.
	if mask, ok := result[inputUsageKey{argIdx: 0, memberIdx: 0}]; ok && mask != 0 {
		t.Errorf("position should not be used, got mask 0x%02x", mask)
	}
	// Member 1 (_varying) should be fully used (0xFF sentinel).
	mask := result[inputUsageKey{argIdx: 0, memberIdx: 1}]
	if mask != 0xFF {
		t.Errorf("_varying should be fully used (0xFF), got 0x%02x", mask)
	}
}

func TestComputeInputUsedMasks_SwizzleAccess(t *testing.T) {
	// Shader reads in.position.xy via swizzle.
	f32Type := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 4}}
	vec2Type := ir.Type{Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 2}}
	structType := ir.Type{Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "position", Type: 1}, // vec4
			{Name: "color", Type: 2},    // vec2
		},
	}}
	irMod := makeModuleWithTypes(f32Type, vec4Type, vec2Type, structType)

	// Expressions:
	// 0: FunctionArgument(0)
	// 1: AccessIndex(base=0, index=0) - in.position (vec4)
	// 2: Swizzle(vector=1, pattern=[X,Y], size=2) - in.position.xy
	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "in", Type: 3},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			{Kind: ir.ExprSwizzle{Size: 2, Vector: 1, Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, 0, 0}}},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
			{Kind: ir.StmtReturn{Value: ptrExprHandle(2)}},
		},
	}

	result := computeInputUsedMasks(irMod, fn)

	// position should have xy used (bits 0,1 = 0x03).
	mask := result[inputUsageKey{argIdx: 0, memberIdx: 0}]
	if mask != 0x03 {
		t.Errorf("expected mask 0x03 (xy), got 0x%02x", mask)
	}
	// color should not be used.
	if mask, ok := result[inputUsageKey{argIdx: 0, memberIdx: 1}]; ok && mask != 0 {
		t.Errorf("color should not be used, got 0x%02x", mask)
	}
}

func TestComputeInputUsedMasks_UnusedVSInput(t *testing.T) {
	// VS with @builtin(vertex_index) that is NOT used in body.
	// fn vs_main(vertex: Vertex) -> ... { ... } where vertex_index is a separate arg.
	u32Type := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	irMod := makeModuleWithTypes(u32Type)

	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "vertex_index", Type: 0},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralU32(42)}},
		},
		Body: []ir.Statement{
			// Only emit the literal, not the argument.
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
			{Kind: ir.StmtReturn{Value: ptrExprHandle(1)}},
		},
	}

	result := computeInputUsedMasks(irMod, fn)

	// vertex_index should have mask 0 (not used).
	key := inputUsageKey{argIdx: 0, memberIdx: -1}
	if mask, ok := result[key]; ok && mask != 0 {
		t.Errorf("vertex_index should not be used, got mask 0x%02x", mask)
	}
}

func TestComputeInputUsedMasks_MultipleArgs(t *testing.T) {
	// VS with vertex_index (used) and instance_index (unused).
	u32Type := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	irMod := makeModuleWithTypes(u32Type)

	// Expressions:
	// 0: FunctionArgument(0) - vertex_index
	// 1: FunctionArgument(1) - instance_index (unused)
	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "vertex_index", Type: 0},
			{Name: "instance_index", Type: 0},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
		},
	}

	result := computeInputUsedMasks(irMod, fn)

	// vertex_index should be fully used.
	key0 := inputUsageKey{argIdx: 0, memberIdx: -1}
	if result[key0] != 0xFF {
		t.Errorf("vertex_index should be fully used (0xFF), got 0x%02x", result[key0])
	}
	// instance_index should not be used.
	key1 := inputUsageKey{argIdx: 1, memberIdx: -1}
	if mask, ok := result[key1]; ok && mask != 0 {
		t.Errorf("instance_index should not be used, got mask 0x%02x", mask)
	}
}

// ptrExprHandle is a helper for creating *ExpressionHandle from a value.
func ptrExprHandle(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}
