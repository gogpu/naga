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

// TestUsedStructMembers_AllUsed verifies that when all struct members are
// accessed, usedStructMembers returns nil (meaning all used).
func TestUsedStructMembers_AllUsed(t *testing.T) {
	// struct Input { @location(0) uv: vec2<f32>, @location(1) color: vec4<f32> }
	// fn fs_main(in: Input) -> @location(0) vec4<f32> { return vec4(in.uv, 0, 0) * in.color; }
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "in", Type: ir.TypeHandle(0)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                       // [0]: in
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                   // [1]: in.uv
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},                   // [2]: in.color
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 1, Right: 2}}, // [3]: in.uv * in.color
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 4}}},
			{Kind: ir.StmtReturn{Value: exprPtr(3)}},
		},
	}

	result := usedStructMembers(&fn, 0)
	if result == nil {
		// nil means "all used" -- this is acceptable
		return
	}
	if !result[0] || !result[1] {
		t.Errorf("expected both members used, got %v", result)
	}
}

// TestUsedStructMembers_OnlyOneUsed verifies per-member DCE: when only
// one struct member is accessed, the other is marked as dead.
func TestUsedStructMembers_OnlyOneUsed(t *testing.T) {
	// struct VertexOutput { @builtin(position) pos: vec4, @location(0) uv: vec3 }
	// fn fs_main(in: VertexOutput) -> @location(0) vec4 { return textureSample(tex, s, in.uv); }
	// Only member 1 (uv) is accessed; member 0 (pos) is dead.
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "in", Type: ir.TypeHandle(0)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0]: in
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]: in.pos (DEAD)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // [2]: in.uv (LIVE)
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [3]: texture
			{Kind: ir.ExprGlobalVariable{Variable: 1}},    // [4]: sampler
			{Kind: ir.ExprImageSample{ // [5]: textureSample
				Image: 3, Sampler: 4, Coordinate: 2, Level: ir.SampleLevelAuto{},
			}},
		},
		Body: ir.Block{
			// Emit range only covers the used expressions
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 6}}},
			{Kind: ir.StmtReturn{Value: exprPtr(5)}},
		},
	}

	result := usedStructMembers(&fn, 0)
	if result == nil {
		t.Fatal("expected per-member result, got nil (all used)")
	}
	if result[0] {
		t.Error("expected member 0 (pos) to be dead, but it was marked as used")
	}
	if !result[1] {
		t.Error("expected member 1 (uv) to be used, but it was marked as dead")
	}
}

// TestUsedStructMembers_TransitiveChain verifies that member liveness is
// detected through transitive expression chains (AccessIndex -> Swizzle ->
// BinaryOp), even when the intermediate Swizzle is NOT in any StmtEmit range.
// This is the exact pattern that occurs after function inlining.
func TestUsedStructMembers_TransitiveChain(t *testing.T) {
	// After inlining rrect_clip_coverage(in.clip_position.xy):
	// [0] FunctionArgument(0)              -- not emitted
	// [1] AccessIndex{Base:0, Index:0}     -- emitted (clip_position)
	// [2] Swizzle{Vector:1, .xy}           -- NOT emitted (inlining artifact)
	// [3] Binary{Left:2, Right:...}        -- emitted (inlined body uses swizzle)
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "in", Type: ir.TypeHandle(0)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0]: in
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]: in.clip_position
			{Kind: ir.ExprSwizzle{ // [2]: in.clip_position.xy
				Size: 2, Vector: 1,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, 0, 0},
			}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},              // [3]: in.coverage
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 2, Right: 3}}, // [4]: uses swizzle
		},
		Body: ir.Block{
			// Emit range covers [1] and [3,4] but NOT [2] (the Swizzle)
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
			{Kind: ir.StmtReturn{Value: exprPtr(4)}},
		},
	}

	result := usedStructMembers(&fn, 0)
	if result == nil {
		// nil means all used -- acceptable since both members are used
		return
	}
	if !result[0] {
		t.Error("expected member 0 (clip_position) to be used via transitive chain")
	}
	if !result[1] {
		t.Error("expected member 1 (coverage) to be used")
	}
}

// TestUsedStructMembers_WholeStructRef verifies that when the struct arg
// is referenced directly (not through AccessIndex), all members are alive.
func TestUsedStructMembers_WholeStructRef(t *testing.T) {
	// fn main(in: Input) { var x = in; } -- whole-struct copy
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "in", Type: ir.TypeHandle(0)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0]: in
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]: in.x
			{Kind: ir.ExprZeroValue{Type: 0}},             // [2]: placeholder for local var pointer
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			// Store whole struct to a local variable (pointer expr [2])
			{Kind: ir.StmtStore{Pointer: 2, Value: 0}},
		},
	}

	result := usedStructMembers(&fn, 0)
	if result != nil {
		t.Errorf("expected nil (all used) when struct is referenced directly, got %v", result)
	}
}

// TestComputeLiveExprs_TransitiveReachability verifies that computeLiveExprs
// correctly marks transitively reachable expressions, including intermediate
// nodes not in any StmtEmit range.
func TestComputeLiveExprs_TransitiveReachability(t *testing.T) {
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0]: arg
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1]: arg.x
			{Kind: ir.ExprSwizzle{ // [2]: arg.x.xy (NOT emitted)
				Size: 2, Vector: 1,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, 0, 0},
			}},
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 2, Right: 2}}, // [3]: uses swizzle
		},
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 2}}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
			{Kind: ir.StmtReturn{Value: exprPtr(3)}},
		},
	}

	live := computeLiveExprs(&fn)

	// Handle 1 is directly emitted
	if !live[1] {
		t.Error("handle 1 (emitted) should be live")
	}
	// Handle 2 is NOT emitted but IS referenced by handle 3
	if !live[2] {
		t.Error("handle 2 (not emitted, but referenced by 3) should be live")
	}
	// Handle 3 is directly emitted
	if !live[3] {
		t.Error("handle 3 (emitted) should be live")
	}
	// Handle 0 is transitively reached via 1 -> AccessIndex.Base
	if !live[0] {
		t.Error("handle 0 (transitively reached via AccessIndex) should be live")
	}
}

// TestExpressionOperands verifies that expressionOperands correctly extracts
// all operand handles from expression kinds.
func TestExpressionOperands(t *testing.T) {
	cases := []struct {
		name   string
		kind   ir.ExpressionKind
		expect []ir.ExpressionHandle
	}{
		{"AccessIndex", ir.ExprAccessIndex{Base: 5, Index: 2}, []ir.ExpressionHandle{5}},
		{"Binary", ir.ExprBinary{Op: ir.BinaryAdd, Left: 1, Right: 2}, []ir.ExpressionHandle{1, 2}},
		{"Swizzle", ir.ExprSwizzle{Vector: 3}, []ir.ExpressionHandle{3}},
		{"Load", ir.ExprLoad{Pointer: 7}, []ir.ExpressionHandle{7}},
		{"Unary", ir.ExprUnary{Expr: 4}, []ir.ExpressionHandle{4}},
		{"As", ir.ExprAs{Expr: 6}, []ir.ExpressionHandle{6}},
		{"Compose", ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 3}}, []ir.ExpressionHandle{1, 2, 3}},
		{"FunctionArgument", ir.ExprFunctionArgument{Index: 0}, nil},
		{"Literal", ir.Literal{Value: ir.LiteralF64(1.0)}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expressionOperands(tc.kind)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %d operands, got %d: %v", len(tc.expect), len(got), got)
			}
			for i, h := range got {
				if h != tc.expect[i] {
					t.Errorf("operand[%d]: expected %d, got %d", i, tc.expect[i], h)
				}
			}
		})
	}
}

// TestBuildInputRowMapSequentialSigId verifies that buildInputRowMap assigns
// sequential signature element indices (sigId) rather than packed register
// rows. When two scalar elements pack into the same register row (e.g.,
// @location(1) and @location(3) sharing register 0 with different columns),
// each must get a unique sigId (0 and 1), not both mapped to register 0.
func TestBuildInputRowMapSequentialSigId(t *testing.T) {
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 0: f32
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1: vec4<f32>
			{Inner: ir.StructType{Members: []ir.StructMember{ // 2: FragmentIn
				{Name: "value", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 1})},
				{Name: "value2", Type: 0, Binding: bindingPtr(ir.LocationBinding{Location: 3})},
				{Name: "position", Type: 1, Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
			}}},
		},
	}

	fn := ir.Function{
		Name: "fs_main",
		Arguments: []ir.FunctionArgument{
			{Name: "v_out", Type: 2},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
	}

	e := &Emitter{ir: irMod}
	rowMap := e.buildInputRowMap(&fn, ir.StageFragment)

	// LOC1 and LOC3 pack into the same register (row 0, cols 0 and 1).
	// They must get DIFFERENT sigIds: 0 and 1.
	loc1SigId, ok1 := rowMap[inputRowKey{argIdx: 0, memberIdx: 0}]
	loc3SigId, ok3 := rowMap[inputRowKey{argIdx: 0, memberIdx: 1}]

	if !ok1 {
		t.Fatal("LOC1 (member 0) not in rowMap")
	}
	if !ok3 {
		t.Fatal("LOC3 (member 1) not in rowMap")
	}

	if loc1SigId == loc3SigId {
		t.Errorf("LOC1 and LOC3 have same sigId=%d (both mapped to register row); want different sequential indices", loc1SigId)
	}
	if loc1SigId != 0 {
		t.Errorf("LOC1 sigId=%d, want 0 (first element in sorted order)", loc1SigId)
	}
	if loc3SigId != 1 {
		t.Errorf("LOC3 sigId=%d, want 1 (second element in sorted order)", loc3SigId)
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
