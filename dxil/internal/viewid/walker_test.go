package viewid

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestWalkStatementTypes exercises walkStatement for statement types that
// are not covered by the main integration tests.
func TestWalkStatementTypes(t *testing.T) {
	mod := makeBasicMod()

	t.Run("StmtIf_walks_branches", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result:    &ir.FunctionResult{Type: 0, Binding: &outBind},
			LocalVars: []ir.LocalVariable{{Name: "r", Type: 0}},
		}
		f32H := ir.TypeHandle(0)
		boolH := ir.TypeHandle(len(mod.Types))
		mod.Types = append(mod.Types, ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}})

		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},       // 0
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // 1: condition
			{Kind: ir.ExprLocalVariable{Variable: 0}},       // 2: &r
			{Kind: ir.ExprLoad{Pointer: 2}},                 // 3: load r
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &boolH},
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(3)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtIf{
				Condition: 1,
				Accept: ir.Block{
					{Kind: ir.StmtStore{Pointer: 2, Value: 0}}, // r = v
				},
				Reject: ir.Block{},
			}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Input feeds output via if-accept branch store.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (if-accept store)", got)
		}
	})

	t.Run("StmtSwitch_walks_cases", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		u32H := ir.TypeHandle(len(mod.Types))
		mod.Types = append(mod.Types, ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}})

		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // selector
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &u32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtSwitch{
				Selector: 1,
				Cases: []ir.SwitchCase{
					{Value: ir.SwitchValueDefault{}, Body: ir.Block{
						{Kind: ir.StmtBreak{}},
					}},
				},
			}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
	})

	t.Run("StmtLoop_walks_body_and_continuing", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		boolH := ir.TypeHandle(len(mod.Types) - 1) // reuse bool type

		breakIf := ir.ExpressionHandle(1)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &boolH},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtLoop{
				Body:       ir.Block{{Kind: ir.StmtBreak{}}},
				Continuing: ir.Block{},
				BreakIf:    &breakIf,
			}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
	})

	t.Run("StmtBlock_nested", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtBlock{Block: ir.Block{
				{Kind: ir.StmtBlock{Block: ir.Block{}}}, // deeply nested empty block
			}}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
	})

	t.Run("StmtImageStore_no_effect", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtImageStore{Image: 1, Coordinate: 0, Value: 0}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// ImageStore does not affect signature outputs.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
	})

	t.Run("StmtKill_no_effect", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtKill{}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		_ = deps // Kill does not crash, does not affect deps.
	})

	t.Run("StmtBarrier_no_effect", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtBarrier{Flags: ir.BarrierStorage}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		_ = deps // Barrier does not crash.
	})

	t.Run("StmtContinue_no_effect", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtLoop{
				Body:       ir.Block{{Kind: ir.StmtContinue{}}},
				Continuing: ir.Block{{Kind: ir.StmtBreak{}}},
			}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		_ = deps // Continue does not crash.
	})

	t.Run("StmtCall_triggers_giveUp", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtCall{Function: 0, Arguments: nil}},
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// StmtCall triggers giveUp -> everything depends on everything.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (giveUp from call)", got)
		}
	})
}

// TestHandleStore exercises the handleStore path for local variable stores
// including struct-member stores and conservative fallbacks.
func TestHandleStore(t *testing.T) {
	mod := makeBasicMod()
	vec2H := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
	})

	t.Run("store_scalar_to_local", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result:    &ir.FunctionResult{Type: 0, Binding: &outBind},
			LocalVars: []ir.LocalVariable{{Name: "x", Type: 0}},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // 0
			{Kind: ir.ExprLocalVariable{Variable: 0}}, // 1: &x
			{Kind: ir.ExprLoad{Pointer: 1}},           // 2: load x
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(2)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 1, Value: 0}}, // x = v
			{Kind: ir.StmtReturn{Value: &ret}},         // return x
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (store to local)", got)
		}
	})

	t.Run("store_into_vector_member_via_access_index", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "s", Type: 0, Binding: &argBind},
			},
			Result:    &ir.FunctionResult{Type: vec2H, Binding: &outBind},
			LocalVars: []ir.LocalVariable{{Name: "v", Type: vec2H}},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // 0: s (f32)
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // 1: &v
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}}, // 2: &v.x
			{Kind: ir.ExprLoad{Pointer: 1}},               // 3: load v
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &vec2H},
			{Handle: &f32H},
			{Handle: &vec2H},
		}
		ret := ir.ExpressionHandle(3)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 2, Value: 0}}, // v.x = s
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 2, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Input scalar 0 -> output scalar 0 (v.x). Output scalar 1 (v.y) has no deps.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1 (store to v.x)", got)
		}
	})

	t.Run("store_into_non_local_ignored", func(t *testing.T) {
		argBind := ir.Binding(ir.LocationBinding{Location: 0})
		outBind := ir.Binding(ir.LocationBinding{Location: 0})
		fn := ir.Function{
			Arguments: []ir.FunctionArgument{
				{Name: "v", Type: 0, Binding: &argBind},
			},
			Result: &ir.FunctionResult{Type: 0, Binding: &outBind},
		}
		f32H := ir.TypeHandle(0)
		fn.Expressions = []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprGlobalVariable{Variable: 0}}, // global, not local
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H},
			{Handle: &f32H},
		}
		ret := ir.ExpressionHandle(0)
		fn.Body = []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 1, Value: 0}}, // store to global -> ignored
			{Kind: ir.StmtReturn{Value: &ret}},
		}
		ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
		inputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}
		outputs := []SigElement{{ScalarStart: 0, NumChannels: 1, VectorRow: 0}}

		deps := Analyze(mod, &ep, inputs, outputs)
		// Store to global does not crash and does not affect output deps.
		if got := deps.InputScalarToOutputs[0]; got != 0x1 {
			t.Errorf("InputScalarToOutputs[0] = 0x%x, want 0x1", got)
		}
	})
}

// TestRootLocalVar verifies rootLocalVar path tracing.
func TestRootLocalVar(t *testing.T) {
	mod := makeBasicMod()
	f32H := ir.TypeHandle(0)

	t.Run("direct_local_var", func(t *testing.T) {
		fn := ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 5}},
			},
		}
		fn.ExpressionTypes = []ir.TypeResolution{{Handle: &f32H}}
		ep := ir.EntryPoint{Function: fn}
		s := &analysisState{irMod: mod, ep: &ep}

		varIdx, path, ok := s.rootLocalVar(0)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if varIdx != 5 {
			t.Errorf("varIdx = %d, want 5", varIdx)
		}
		if len(path) != 0 {
			t.Errorf("path len = %d, want 0 (direct local)", len(path))
		}
	})

	t.Run("access_index_chain", func(t *testing.T) {
		fn := ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 3}},
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}},
				{Kind: ir.ExprAccessIndex{Base: 1, Index: 1}},
			},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H}, {Handle: &f32H}, {Handle: &f32H},
		}
		ep := ir.EntryPoint{Function: fn}
		s := &analysisState{irMod: mod, ep: &ep}

		varIdx, path, ok := s.rootLocalVar(2)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if varIdx != 3 {
			t.Errorf("varIdx = %d, want 3", varIdx)
		}
		if len(path) != 2 {
			t.Fatalf("path len = %d, want 2", len(path))
		}
		// Path should be root→leaf: [2, 1] (reversed from accumulation order).
		if !path[0].isStatic || path[0].staticIdx != 2 {
			t.Errorf("path[0] = {%v, %d}, want {true, 2}", path[0].isStatic, path[0].staticIdx)
		}
		if !path[1].isStatic || path[1].staticIdx != 1 {
			t.Errorf("path[1] = {%v, %d}, want {true, 1}", path[1].isStatic, path[1].staticIdx)
		}
	})

	t.Run("dynamic_access_in_chain", func(t *testing.T) {
		fn := ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},
				{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // index expr
				{Kind: ir.ExprAccess{Base: 0, Index: 1}},    // dynamic access
			},
		}
		fn.ExpressionTypes = []ir.TypeResolution{
			{Handle: &f32H}, {Handle: &f32H}, {Handle: &f32H},
		}
		ep := ir.EntryPoint{Function: fn}
		s := &analysisState{irMod: mod, ep: &ep}

		varIdx, path, ok := s.rootLocalVar(2)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if varIdx != 0 {
			t.Errorf("varIdx = %d, want 0", varIdx)
		}
		if len(path) != 1 {
			t.Fatalf("path len = %d, want 1", len(path))
		}
		if path[0].isStatic {
			t.Error("expected dynamic step (isStatic=false)")
		}
		if path[0].dynamic != 1 {
			t.Errorf("dynamic handle = %d, want 1", path[0].dynamic)
		}
	})

	t.Run("non_local_root_returns_false", func(t *testing.T) {
		fn := ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},
			},
		}
		fn.ExpressionTypes = []ir.TypeResolution{{Handle: &f32H}}
		ep := ir.EntryPoint{Function: fn}
		s := &analysisState{irMod: mod, ep: &ep}

		_, _, ok := s.rootLocalVar(0)
		if ok {
			t.Error("expected ok=false for global variable root")
		}
	})
}

// TestResolveFlatRange exercises resolveFlatRange for struct, vector,
// array, and dynamic-index cases.
func TestResolveFlatRange(t *testing.T) {
	mod := makeBasicMod()
	// mod.Types: [0]=f32, [1]=vec3<f32>, [2]=vec4<f32>

	// Add struct type: { a: f32, b: vec3<f32> }
	stH := ir.TypeHandle(len(mod.Types))
	mod.Types = append(mod.Types, ir.Type{
		Inner: ir.StructType{Members: []ir.StructMember{
			{Name: "a", Type: 0}, // f32 -> 1 scalar
			{Name: "b", Type: 1}, // vec3 -> 3 scalars
		}},
	})
	_ = stH

	structInner := mod.Types[stH].Inner

	t.Run("empty_path_returns_total", func(t *testing.T) {
		start, count, precise := resolveFlatRange(mod, structInner, nil)
		if !precise || start != 0 || count != 4 {
			t.Errorf("got (%d, %d, %v), want (0, 4, true)", start, count, precise)
		}
	})

	t.Run("struct_member_0", func(t *testing.T) {
		path := []pathStep{{isStatic: true, staticIdx: 0}}
		start, count, precise := resolveFlatRange(mod, structInner, path)
		if !precise || start != 0 || count != 1 {
			t.Errorf("got (%d, %d, %v), want (0, 1, true)", start, count, precise)
		}
	})

	t.Run("struct_member_1", func(t *testing.T) {
		path := []pathStep{{isStatic: true, staticIdx: 1}}
		start, count, precise := resolveFlatRange(mod, structInner, path)
		if !precise || start != 1 || count != 3 {
			t.Errorf("got (%d, %d, %v), want (1, 3, true)", start, count, precise)
		}
	})

	t.Run("struct_member_out_of_range", func(t *testing.T) {
		path := []pathStep{{isStatic: true, staticIdx: 99}}
		_, _, precise := resolveFlatRange(mod, structInner, path)
		if precise {
			t.Error("expected precise=false for out-of-range struct member")
		}
	})

	t.Run("vector_component", func(t *testing.T) {
		vecInner := ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
		path := []pathStep{{isStatic: true, staticIdx: 2}} // v.z
		start, count, precise := resolveFlatRange(mod, vecInner, path)
		if !precise || start != 2 || count != 1 {
			t.Errorf("got (%d, %d, %v), want (2, 1, true)", start, count, precise)
		}
	})

	t.Run("vector_component_out_of_range", func(t *testing.T) {
		vecInner := ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
		path := []pathStep{{isStatic: true, staticIdx: 5}}
		_, _, precise := resolveFlatRange(mod, vecInner, path)
		if precise {
			t.Error("expected precise=false for out-of-range vector component")
		}
	})

	t.Run("array_is_opaque", func(t *testing.T) {
		arrSize := uint32(4)
		arrInner := ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &arrSize}}
		path := []pathStep{{isStatic: true, staticIdx: 0}}
		_, _, precise := resolveFlatRange(mod, arrInner, path)
		if precise {
			t.Error("expected precise=false for array (opaque)")
		}
	})

	t.Run("dynamic_step_not_precise", func(t *testing.T) {
		path := []pathStep{{isStatic: false, dynamic: 0}}
		_, _, precise := resolveFlatRange(mod, structInner, path)
		if precise {
			t.Error("expected precise=false for dynamic step")
		}
	})

	t.Run("struct_then_vector_nested_path", func(t *testing.T) {
		// Path: struct.b.y -> member 1 (vec3) then component 1
		path := []pathStep{
			{isStatic: true, staticIdx: 1}, // struct member b (vec3, offset 1)
			{isStatic: true, staticIdx: 1}, // vector component y
		}
		start, count, precise := resolveFlatRange(mod, structInner, path)
		if !precise || start != 2 || count != 1 {
			t.Errorf("got (%d, %d, %v), want (2, 1, true) for struct.b.y", start, count, precise)
		}
	})

	t.Run("unknown_type_not_precise", func(t *testing.T) {
		// A type that is not struct, vector, or array.
		scalarInner := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		path := []pathStep{{isStatic: true, staticIdx: 0}}
		_, _, precise := resolveFlatRange(mod, scalarInner, path)
		if precise {
			t.Error("expected precise=false for scalar with non-empty path")
		}
	})
}

// TestHandleReturnNilValue verifies that handleReturn does not crash when
// the return value is nil (void return).
func TestHandleReturnNilValue(t *testing.T) {
	mod := makeBasicMod()
	fn := ir.Function{}
	ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
	s := &analysisState{irMod: mod, ep: &ep, outputTaint: nil}
	// Nil return value should not crash.
	s.handleReturn(ir.StmtReturn{Value: nil})
}

// TestHandleReturnNoResult verifies that handleReturn handles nil Result.
func TestHandleReturnNoResult(t *testing.T) {
	mod := makeBasicMod()
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
		},
	}
	f32H := ir.TypeHandle(0)
	fn.ExpressionTypes = []ir.TypeResolution{{Handle: &f32H}}
	ep := ir.EntryPoint{Name: "main", Stage: ir.StageFragment, Function: fn}
	s := newAnalysisState(mod, &ep, nil, nil)
	val := ir.ExpressionHandle(0)
	// Non-nil value but nil Function.Result should not crash.
	s.handleReturn(ir.StmtReturn{Value: &val})
}
