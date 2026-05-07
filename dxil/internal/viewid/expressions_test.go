package viewid

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestCopyAllAsUnion verifies that copyAllAsUnion collapses all components
// into a single union and replicates to n components.
func TestCopyAllAsUnion(t *testing.T) {
	s0 := newScalarSet()
	s0.add(1)
	s1 := newScalarSet()
	s1.add(2)
	ct := componentTaint{s0, s1}

	result := ct.copyAllAsUnion(3)
	if len(result) != 3 {
		t.Fatalf("copyAllAsUnion(3) len = %d, want 3", len(result))
	}
	for i, s := range result {
		if _, ok := s[1]; !ok {
			t.Errorf("component %d should contain scalar 1", i)
		}
		if _, ok := s[2]; !ok {
			t.Errorf("component %d should contain scalar 2", i)
		}
	}
}

// TestCopyAllAsUnionNilComponents handles nil-union edge case.
func TestCopyAllAsUnionNilComponents(t *testing.T) {
	var ct componentTaint
	result := ct.copyAllAsUnion(2)
	if len(result) != 2 {
		t.Fatalf("copyAllAsUnion(2) len = %d, want 2", len(result))
	}
	for i, s := range result {
		if len(s) != 0 {
			t.Errorf("component %d should be empty, got %d entries", i, len(s))
		}
	}
}

// TestPerComponentCopy verifies per-component taint copy with padding.
func TestPerComponentCopy(t *testing.T) {
	s0 := newScalarSet()
	s0.add(10)
	s1 := newScalarSet()
	s1.add(20)
	ct := componentTaint{s0, s1}

	t.Run("exact_size", func(t *testing.T) {
		result := ct.perComponentCopy(2)
		if len(result) != 2 {
			t.Fatalf("len = %d, want 2", len(result))
		}
		if _, ok := result[0][10]; !ok {
			t.Error("component 0 should contain 10")
		}
		if _, ok := result[1][20]; !ok {
			t.Error("component 1 should contain 20")
		}
	})

	t.Run("larger_than_source", func(t *testing.T) {
		result := ct.perComponentCopy(4)
		if len(result) != 4 {
			t.Fatalf("len = %d, want 4", len(result))
		}
		if _, ok := result[0][10]; !ok {
			t.Error("component 0 should contain 10")
		}
		if _, ok := result[1][20]; !ok {
			t.Error("component 1 should contain 20")
		}
		if len(result[2]) != 0 {
			t.Errorf("component 2 should be empty, got %d entries", len(result[2]))
		}
		if len(result[3]) != 0 {
			t.Errorf("component 3 should be empty, got %d entries", len(result[3]))
		}
	})

	t.Run("smaller_than_source", func(t *testing.T) {
		result := ct.perComponentCopy(1)
		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}
		if _, ok := result[0][10]; !ok {
			t.Error("component 0 should contain 10")
		}
	})
}

// TestMergeComponentWise verifies binary merge with scalar broadcast.
func TestMergeComponentWise(t *testing.T) {
	t.Run("vector_vector", func(t *testing.T) {
		l0 := newScalarSet()
		l0.add(1)
		l1 := newScalarSet()
		l1.add(2)
		lhs := componentTaint{l0, l1}

		r0 := newScalarSet()
		r0.add(3)
		r1 := newScalarSet()
		r1.add(4)
		rhs := componentTaint{r0, r1}

		result := mergeComponentWise(lhs, rhs, 2)
		if len(result) != 2 {
			t.Fatalf("len = %d, want 2", len(result))
		}
		// Component 0: lhs[0] union rhs[0] = {1, 3}
		if _, ok := result[0][1]; !ok {
			t.Error("component 0 should contain 1")
		}
		if _, ok := result[0][3]; !ok {
			t.Error("component 0 should contain 3")
		}
		// Component 1: lhs[1] union rhs[1] = {2, 4}
		if _, ok := result[1][2]; !ok {
			t.Error("component 1 should contain 2")
		}
		if _, ok := result[1][4]; !ok {
			t.Error("component 1 should contain 4")
		}
	})

	t.Run("scalar_broadcast_lhs", func(t *testing.T) {
		// lhs scalar, rhs vector: lhs is broadcast to each component.
		l := newScalarSet()
		l.add(100)
		lhs := componentTaint{l}

		r0 := newScalarSet()
		r0.add(10)
		r1 := newScalarSet()
		r1.add(20)
		rhs := componentTaint{r0, r1}

		result := mergeComponentWise(lhs, rhs, 2)
		if len(result) != 2 {
			t.Fatalf("len = %d, want 2", len(result))
		}
		// Each component gets lhs[0] union rhs[i].
		if _, ok := result[0][100]; !ok {
			t.Error("component 0 should contain broadcast scalar 100")
		}
		if _, ok := result[0][10]; !ok {
			t.Error("component 0 should contain 10")
		}
		if _, ok := result[1][100]; !ok {
			t.Error("component 1 should contain broadcast scalar 100")
		}
		if _, ok := result[1][20]; !ok {
			t.Error("component 1 should contain 20")
		}
	})
}

// TestMergeSelectComponentWise verifies component-wise merge for select().
func TestMergeSelectComponentWise(t *testing.T) {
	a0 := newScalarSet()
	a0.add(1)
	a1 := newScalarSet()
	a1.add(2)
	accept := componentTaint{a0, a1}

	r0 := newScalarSet()
	r0.add(3)
	r1 := newScalarSet()
	r1.add(4)
	reject := componentTaint{r0, r1}

	c0 := newScalarSet()
	c0.add(5)
	cond := componentTaint{c0} // scalar condition broadcast

	result := mergeSelectComponentWise(accept, reject, cond, 2)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	// Component 0: accept[0] union reject[0] union cond[0] = {1, 3, 5}
	for _, v := range []uint32{1, 3, 5} {
		if _, ok := result[0][v]; !ok {
			t.Errorf("component 0 should contain %d", v)
		}
	}
	// Component 1: accept[1] union reject[1] union cond[0 broadcast] = {2, 4, 5}
	for _, v := range []uint32{2, 4, 5} {
		if _, ok := result[1][v]; !ok {
			t.Errorf("component 1 should contain %d", v)
		}
	}
}

// TestResultScalarCount verifies resultScalarCount returns correct values
// including the nil-type fallback.
func TestResultScalarCount(t *testing.T) {
	mod := makeBasicMod()
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
		},
	}
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &vec4H}, // expr 0: vec4 -> 4
		{},               // expr 1: no resolved type -> fallback 1
	}
	ep := ir.EntryPoint{Function: fn}
	s := &analysisState{irMod: mod, ep: &ep}

	t.Run("vec4_returns_4", func(t *testing.T) {
		n := s.resultScalarCount(0)
		if n != 4 {
			t.Errorf("resultScalarCount(0) = %d, want 4", n)
		}
	})

	t.Run("nil_type_returns_1", func(t *testing.T) {
		n := s.resultScalarCount(1)
		if n != 1 {
			t.Errorf("resultScalarCount(1) = %d, want 1 (nil fallback)", n)
		}
	})
}

// TestInnerOfExpr verifies type resolution for expressions including
// Handle, Value, and out-of-range handle cases.
func TestInnerOfExpr(t *testing.T) {
	mod := makeBasicMod()
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
		},
	}
	vec4H := ir.TypeHandle(2)
	outOfRange := ir.TypeHandle(999)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &vec4H}, // 0: Handle -> vec4
		{Value: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 1: Value -> f32
		{Handle: &outOfRange}, // 2: out-of-range handle
	}
	ep := ir.EntryPoint{Function: fn}
	s := &analysisState{irMod: mod, ep: &ep}

	t.Run("handle_resolved", func(t *testing.T) {
		inner := s.innerOfExpr(0)
		if _, ok := inner.(ir.VectorType); !ok {
			t.Errorf("expected VectorType, got %T", inner)
		}
	})

	t.Run("value_resolved", func(t *testing.T) {
		inner := s.innerOfExpr(1)
		if _, ok := inner.(ir.ScalarType); !ok {
			t.Errorf("expected ScalarType, got %T", inner)
		}
	})

	t.Run("out_of_range_handle", func(t *testing.T) {
		inner := s.innerOfExpr(2)
		if inner != nil {
			t.Errorf("expected nil for out-of-range handle, got %T", inner)
		}
	})

	t.Run("out_of_range_expr", func(t *testing.T) {
		inner := s.innerOfExpr(999)
		if inner != nil {
			t.Errorf("expected nil for out-of-range expr, got %T", inner)
		}
	})
}

// TestComputeTaintExpressionKinds exercises computeTaint for expression
// kinds that are not covered by the main integration tests.
func TestComputeTaintExpressionKinds(t *testing.T) {
	mod := makeBasicMod()
	vec2H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
	})

	argBinding := ir.Binding(ir.LocationBinding{Location: 0})
	outBinding := ir.Binding(ir.LocationBinding{Location: 0})

	t.Run("ExprSplat", func(t *testing.T) {
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "s", Type: 0, Binding: &argBinding},
			},
			Result: &ir.FunctionResult{Type: vec2H, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 0}},
		}
		f32H := ir.TypeHandle(0)
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &vec2H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Splat broadcasts scalar to both components -> both outputs depend on input 0.
		if got := deps.InputScalarToOutputs[0]; got != 0x3 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x3 (splat broadcasts to both outputs)", got)
		}
	})

	t.Run("ExprSwizzle", func(t *testing.T) {
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: vec2H, Binding: &argBinding},
			},
			Result: &ir.FunctionResult{Type: vec2H, Binding: &outBinding},
		}
		// Swizzle: v.yx (reverse components)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprSwizzle{
				Size:    ir.Vec2,
				Vector:  0,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleY, ir.SwizzleX, 0, 0},
			}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &vec2H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Swizzle .yx: output[0] depends on input[1], output[1] depends on input[0].
		if got := deps.InputScalarToOutputs[0]; got != 0x2 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x2 (swizzle reverses: input.x -> output.y)", got)
		}
		if got := deps.InputScalarToOutputs[1]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x1 (swizzle reverses: input.y -> output.x)", got)
		}
	})

	t.Run("ExprDerivative", func(t *testing.T) {
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: vec2H, Binding: &argBinding},
			},
			Result: &ir.FunctionResult{Type: vec2H, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeFine, Expr: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &vec2H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Derivative is component-wise: input.x -> output.x, input.y -> output.y.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (derivative is component-wise)", got)
		}
		if got := deps.InputScalarToOutputs[1]; got != 0x2 {
			t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x2 (derivative is component-wise)", got)
		}
	})

	t.Run("ExprRelational_collapses_vector", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		boolH := ir.TypeHandle(len(mod.Types))
		mod.Types = append(mod.Types, ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}})

		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: vec2H, Binding: &argBinding},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprRelational{Fun: ir.RelationalAny, Argument: 0}},
			{Kind: ir.ExprAs{Expr: 1, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &boolH},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(2)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Relational collapses vector to scalar. Both inputs contribute to output.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
		if got := deps.InputScalarToOutputs[1]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x1", got)
		}
	})

	t.Run("ExprAccess_dynamic_index", func(t *testing.T) {
		f32H := ir.TypeHandle(0)

		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: vec2H, Binding: &argBinding},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBinding},
		}
		// Dynamic access: v[0] — even though index is literal, ExprAccess
		// is treated as dynamic.
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
			{Kind: ir.ExprAccess{Base: 0, Index: 1}},
		}
		u32H := ir.TypeHandle(len(mod.Types))
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &u32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(2)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Dynamic access is conservative: union of base + index taint.
		// Both input components feed the output.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
		if got := deps.InputScalarToOutputs[1]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x1", got)
		}
	})

	t.Run("ExprMath_multi_arg", func(t *testing.T) {
		f32H := ir.TypeHandle(0)

		argA := ir.Binding(ir.LocationBinding{Location: 0})
		argB := ir.Binding(ir.LocationBinding{Location: 1})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})

		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "a", Type: 0, Binding: &argA},
				{Name: "b", Type: 0, Binding: &argB},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		arg1 := ir.ExpressionHandle(1)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
			{Kind: ir.ExprMath{Fun: ir.MathMin, Arg: 0, Arg1: &arg1}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(2)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{
			{ScalarStart: 0, NumChannels: 1, VectorRow: 0, StartCol: 0},
			{ScalarStart: 1, NumChannels: 1, VectorRow: 1, StartCol: 0},
		}
		outputs := []SigElement{
			{ScalarStart: 0, NumChannels: 1, VectorRow: 0, StartCol: 0},
		}

		deps := Analyze(mod, &ep, inputs, outputs)
		// min(a, b): output depends on both inputs.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (input a)", got)
		}
		// Input b is at packed index 4 (row 1, col 0).
		if 4 < uint32(len(deps.InputScalarToOutputs)) {
			if got := deps.InputScalarToOutputs[4]; got != 0x1 {
				t.Errorf("InputScalarToOutputs[4] = 0x%x, want 0x1 (input b)", got)
			}
		}
	})

	t.Run("ExprSelect_vector", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})

		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: vec2H, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: vec2H, Binding: &outBind},
		}
		// select(v, v, true) -> just v (both branches same).
		boolH := ir.TypeHandle(len(mod.Types) - 1) // reuse from earlier
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			{Kind: ir.ExprSelect{Accept: 0, Reject: 0, Condition: 1}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &boolH},
			{Handle: &vec2H},
		}
		ret := ir.ExpressionHandle(2)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// select with same accept/reject: component-wise identity.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
		if got := deps.InputScalarToOutputs[1]; got != 0x2 {
			t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x2", got)
		}
	})

	t.Run("ExprImageQuery_returns_empty", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		fn := ir.Function{
			Result: &ir.FunctionResult{Type: 0, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{Image: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		deps := Analyze(mod, &ep, nil, []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}})
		// ImageQuery returns empty taint, no input deps.
		if len(deps.InputScalarToOutputs) > 0 && deps.InputScalarToOutputs[0] != 0 {
			t.Errorf("ImageQuery should have empty taint")
		}
	})

	t.Run("ExprArrayLength_returns_empty", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		fn := ir.Function{
			Result: &ir.FunctionResult{Type: 0, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprArrayLength{Array: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		deps := Analyze(mod, &ep, nil, []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}})
		if len(deps.InputScalarToOutputs) > 0 && deps.InputScalarToOutputs[0] != 0 {
			t.Errorf("ArrayLength should have empty taint")
		}
	})

	t.Run("ExprGlobalVariable_returns_empty", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		fn := ir.Function{
			Result: &ir.FunctionResult{Type: 0, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		deps := Analyze(mod, &ep, nil, []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}})
		if len(deps.InputScalarToOutputs) > 0 && deps.InputScalarToOutputs[0] != 0 {
			t.Errorf("GlobalVariable should have empty taint")
		}
	})

	t.Run("ExprCallResult_triggers_giveUp", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "x", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBinding},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprCallResult{Function: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
		}
		callResult := ir.ExpressionHandle(1)
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtCall{Function: 0, Arguments: nil, Result: &callResult}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// CallResult triggers giveUp -> everything depends on everything.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (giveUp)", got)
		}
	})

	t.Run("ExprImageSample_with_depthref", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "uv", Type: vec2H, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		depthRef := ir.ExpressionHandle(2)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                                            // 0: uv
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                                           // 1: texture
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                                        // 2: depth ref
			{Kind: ir.ExprImageSample{Image: 1, Sampler: 1, Coordinate: 0, DepthRef: &depthRef}}, // 3
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &f32H},
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(3)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// ImageSample with coordinate from input: output depends on input.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (coordinate taint)", got)
		}
		if got := deps.InputScalarToOutputs[1]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[1] = 0x%x, want 0x1 (coordinate taint)", got)
		}
	})

	t.Run("ExprImageLoad_with_array_index", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "uv", Type: vec2H, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		arrIdx := ir.ExpressionHandle(2)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                              // 0: uv
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                             // 1: texture
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},                            // 2: array index
			{Kind: ir.ExprImageLoad{Image: 1, Coordinate: 0, ArrayIndex: &arrIdx}}, // 3
		}
		u32H := ir.TypeHandle(len(mod.Types) - 1) // reuse any existing type handle
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &vec2H},
			{Handle: &f32H},
			{Handle: &u32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(3)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// ImageLoad coordinate feeds output.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
	})

	t.Run("ExprAccessIndex_matrix_column", func(t *testing.T) {
		mat2H := ir.TypeHandle(len(mod.Types))
		mod.Types = append(mod.Types, ir.Type{
			Inner: ir.MatrixType{
				Columns: 2, Rows: 2,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			},
		})

		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "m", Type: mat2H, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: vec2H, Binding: &outBind},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // m[1] = second column
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &mat2H},
			{Handle: &vec2H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 4, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Matrix input taint is conservative (all scalars get full taint
		// from expandTaintToType for matrix type). Column 1 access picks
		// flat range [2..4), so each output depends on all 4 input scalars.
		// Both outputs depend on all inputs: 0x3 = bits 0,1.
		for i := uint32(0); i < 4; i++ {
			got := deps.InputScalarToOutputs[i]
			if got != 0x3 {
				t.Errorf("InputScalarToOutputs[%d] = 0x%x, want 0x3 (matrix conservative)", i, got)
			}
		}
	})

	t.Run("ExprAccessIndex_array_conservative", func(t *testing.T) {
		arrSize := uint32(4)
		arrTyH := ir.TypeHandle(len(mod.Types))
		mod.Types = append(mod.Types, ir.Type{
			Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &arrSize}},
		})
		f32H := ir.TypeHandle(0)
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "a", Type: arrTyH, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &arrTyH},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Array access is conservative union.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (array conservative)", got)
		}
	})

	t.Run("unknown_expression_kind_triggers_giveUp", func(t *testing.T) {
		f32H := ir.TypeHandle(0)
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "x", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAtomicResult{Ty: 0}}, // triggers empty taint path
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(1)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		// AtomicResult returns emptyTaint, does not trigger giveUp.
		deps := Analyze(mod, &ep, inputs, outputs)
		// Output has no deps since atomic result is not fed by inputs.
		_ = deps
	})
}
