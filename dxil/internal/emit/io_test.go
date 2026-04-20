package emit

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestIsArgRead_UnusedArgument verifies that isArgRead returns false for
// a function argument that is never referenced in any emitted expression
// or statement (DCE parity with DXC's LLVM ADCE for dead loadInput removal).
func TestIsArgRead_UnusedArgument(t *testing.T) {
	// @vertex fn main(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
	//     return vec4(0.0, 0.0, 0.0, 1.0);
	// }
	// The vertex_index argument is declared but never used.
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{
				Name:    "vi",
				Type:    ir.TypeHandle(0),
				Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex}),
			},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // expr[0]: vi
			{Kind: ir.ExprZeroValue{Type: 0}},         // expr[1]: zero constant
		},
		Body: ir.Block{
			// Only emit range covers expr[1], not expr[0]
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
			{Kind: ir.StmtReturn{Value: exprPtr(1)}},
		},
	}

	if isArgRead(&fn, 0) {
		t.Error("expected isArgRead to return false for unused argument")
	}
}

// TestIsArgRead_UsedArgument verifies that isArgRead returns true when
// the argument IS referenced by an emitted expression.
func TestIsArgRead_UsedArgument(t *testing.T) {
	// @fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	//     return vec4(uv, 0.0, 1.0);
	// }
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{
				Name:    "uv",
				Type:    ir.TypeHandle(0),
				Binding: bindingPtr(ir.LocationBinding{Location: 0}),
			},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // expr[0]: uv
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0}}}, // expr[1]: uses uv
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{Value: exprPtr(1)}},
		},
	}

	if !isArgRead(&fn, 0) {
		t.Error("expected isArgRead to return true for used argument")
	}
}

// TestIsArgRead_UsedViaImageSample verifies that isArgRead detects usage
// through ExprImageSample's Coordinate field. This was a previously missed
// case that caused incorrect DCE of fragment shader inputs used as texture
// coordinates.
func TestIsArgRead_UsedViaImageSample(t *testing.T) {
	// @fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	//     return textureSample(tex, samp, uv);
	// }
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{
				Name:    "uv",
				Type:    ir.TypeHandle(0),
				Binding: bindingPtr(ir.LocationBinding{Location: 0}),
			},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                           // expr[0]: uv
			{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(0)}}, // expr[1]: texture
			{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(1)}}, // expr[2]: sampler
			{Kind: ir.ExprImageSample{ // expr[3]: textureSample(tex, samp, uv)
				Image:      1,
				Sampler:    2,
				Coordinate: 0, // references uv
				Level:      ir.SampleLevelAuto{},
			}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: exprPtr(3)}},
		},
	}

	if !isArgRead(&fn, 0) {
		t.Error("expected isArgRead to return true for argument used as texture coordinate")
	}
}

// TestIsArgRead_UsedViaSwizzle verifies detection of usage through Swizzle.
func TestIsArgRead_UsedViaSwizzle(t *testing.T) {
	// @fragment fn main(@builtin(position) pos: vec4<f32>) -> @builtin(frag_depth) f32 {
	//     return pos.z;
	// }
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{
				Name:    "pos",
				Type:    ir.TypeHandle(0),
				Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}),
			},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // expr[0]: pos
			{Kind: ir.ExprSwizzle{ // expr[1]: pos.z
				Size:    1,
				Vector:  0,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleZ, 0, 0, 0},
			}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{Value: exprPtr(1)}},
		},
	}

	if !isArgRead(&fn, 0) {
		t.Error("expected isArgRead to return true for argument accessed via swizzle")
	}
}

// TestUsedComponentMask_SingleComponent verifies that only the accessed
// component is marked when a vector is accessed through a single swizzle.
func TestUsedComponentMask_SingleComponent(t *testing.T) {
	// pos.z -> only component 2 needed
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // expr[0]: pos (vec4)
			{Kind: ir.ExprSwizzle{ // expr[1]: pos.z
				Size:    1,
				Vector:  0,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleZ, 0, 0, 0},
			}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{Value: exprPtr(1)}},
		},
	}

	mask := usedComponentMask(&fn, 0, 4)
	// Only bit 2 (z component) should be set
	if mask != 0x4 {
		t.Errorf("expected component mask 0x4 (only z), got 0x%x", mask)
	}
}

// TestUsedComponentMask_MultipleComponents verifies that multiple accessed
// components are correctly tracked.
func TestUsedComponentMask_MultipleComponents(t *testing.T) {
	// vec.xy -> components 0 and 1 needed
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // expr[0]: vec (vec4)
			{Kind: ir.ExprSwizzle{ // expr[1]: vec.xy
				Size:    2,
				Vector:  0,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, 0, 0},
			}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{Value: exprPtr(1)}},
		},
	}

	mask := usedComponentMask(&fn, 0, 4)
	// Bits 0 and 1 (x and y) should be set
	if mask != 0x3 {
		t.Errorf("expected component mask 0x3 (xy), got 0x%x", mask)
	}
}

// TestUsedComponentMask_AllComponents verifies that using the vector in a
// non-swizzle context marks all components as used.
func TestUsedComponentMask_AllComponents(t *testing.T) {
	// Using pos in a binary operation -> all components needed
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // expr[0]: pos (vec4)
			{Kind: ir.ExprFunctionArgument{Index: 1}}, // expr[1]: other (vec4)
			{Kind: ir.ExprBinary{ // expr[2]: pos + other
				Op:    ir.BinaryAdd,
				Left:  0,
				Right: 1,
			}},
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtReturn{Value: exprPtr(2)}},
		},
	}

	mask := usedComponentMask(&fn, 0, 4)
	if mask != 0xF {
		t.Errorf("expected component mask 0xF (all), got 0x%x", mask)
	}
}

// TestUsedComponentMask_ViaStatement verifies that statement-level references
// mark all components.
func TestUsedComponentMask_ViaStatement(t *testing.T) {
	// return pos -> all components needed via statement
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // expr[0]: pos (vec4)
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtReturn{Value: exprPtr(0)}}, // directly returns arg
		},
	}

	mask := usedComponentMask(&fn, 0, 4)
	if mask != 0xF {
		t.Errorf("expected component mask 0xF (all via return), got 0x%x", mask)
	}
}

// TestExpressionReferences_ImageSample verifies that expressionReferences
// correctly detects references through ExprImageSample fields.
func TestExpressionReferences_ImageSample(t *testing.T) {
	target := ir.ExpressionHandle(5)

	cases := []struct {
		name   string
		expr   ir.ExpressionKind
		expect bool
	}{
		{"coordinate", ir.ExprImageSample{Image: 0, Sampler: 1, Coordinate: 5, Level: ir.SampleLevelAuto{}}, true},
		{"image", ir.ExprImageSample{Image: 5, Sampler: 1, Coordinate: 2, Level: ir.SampleLevelAuto{}}, true},
		{"sampler", ir.ExprImageSample{Image: 0, Sampler: 5, Coordinate: 2, Level: ir.SampleLevelAuto{}}, true},
		{"no match", ir.ExprImageSample{Image: 0, Sampler: 1, Coordinate: 2, Level: ir.SampleLevelAuto{}}, false},
		{"array_index", ir.ExprImageSample{Image: 0, Sampler: 1, Coordinate: 2, ArrayIndex: &target, Level: ir.SampleLevelAuto{}}, true},
		{"depth_ref", ir.ExprImageSample{Image: 0, Sampler: 1, Coordinate: 2, DepthRef: &target, Level: ir.SampleLevelAuto{}}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expressionReferences(tc.expr, target)
			if got != tc.expect {
				t.Errorf("expressionReferences for %s: got %v, want %v", tc.name, got, tc.expect)
			}
		})
	}
}

// TestExpressionReferences_ImageLoad verifies ExprImageLoad reference detection.
func TestExpressionReferences_ImageLoad(t *testing.T) {
	target := ir.ExpressionHandle(3)

	cases := []struct {
		name   string
		expr   ir.ExpressionKind
		expect bool
	}{
		{"image", ir.ExprImageLoad{Image: 3, Coordinate: 1}, true},
		{"coordinate", ir.ExprImageLoad{Image: 0, Coordinate: 3}, true},
		{"sample", ir.ExprImageLoad{Image: 0, Coordinate: 1, Sample: &target}, true},
		{"no match", ir.ExprImageLoad{Image: 0, Coordinate: 1}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expressionReferences(tc.expr, target)
			if got != tc.expect {
				t.Errorf("got %v, want %v", got, tc.expect)
			}
		})
	}
}

// TestExpressionReferences_Alias verifies ExprAlias reference detection.
func TestExpressionReferences_Alias(t *testing.T) {
	if !expressionReferences(ir.ExprAlias{Source: 7}, 7) {
		t.Error("expected ExprAlias to reference its source")
	}
	if expressionReferences(ir.ExprAlias{Source: 7}, 8) {
		t.Error("expected ExprAlias not to reference a different handle")
	}
}

// helper to create *ir.Binding from a binding value
func bindingPtr(b ir.Binding) *ir.Binding {
	return &b
}

// helper to create *ir.ExpressionHandle
func exprPtr(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}
