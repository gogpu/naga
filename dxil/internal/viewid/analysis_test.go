package viewid

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestScalarSet exercises scalarSet basic operations (add, addAll, copy, union).
func TestScalarSet(t *testing.T) {
	t.Run("add_and_contains", func(t *testing.T) {
		s := newScalarSet()
		s.add(3)
		s.add(7)
		if _, ok := s[3]; !ok {
			t.Error("expected set to contain 3")
		}
		if _, ok := s[7]; !ok {
			t.Error("expected set to contain 7")
		}
		if _, ok := s[5]; ok {
			t.Error("expected set to NOT contain 5")
		}
	})

	t.Run("addAll_merges", func(t *testing.T) {
		a := newScalarSet()
		a.add(1)
		a.add(2)
		b := newScalarSet()
		b.add(3)
		b.add(4)
		a.addAll(b)
		for _, v := range []uint32{1, 2, 3, 4} {
			if _, ok := a[v]; !ok {
				t.Errorf("expected merged set to contain %d", v)
			}
		}
	})

	t.Run("copy_independent", func(t *testing.T) {
		s := newScalarSet()
		s.add(10)
		cp := s.copy()
		cp.add(20)
		if _, ok := s[20]; ok {
			t.Error("original should not be affected by copy mutation")
		}
		if _, ok := cp[10]; !ok {
			t.Error("copy should contain original element 10")
		}
	})
}

// TestComponentTaintUnion exercises the union() method on componentTaint.
func TestComponentTaintUnion(t *testing.T) {
	t.Run("empty_components", func(t *testing.T) {
		var ct componentTaint
		u := ct.union()
		if u != nil {
			t.Errorf("union of empty componentTaint should be nil, got %v", u)
		}
	})

	t.Run("single_component", func(t *testing.T) {
		s := newScalarSet()
		s.add(5)
		ct := componentTaint{s}
		u := ct.union()
		if _, ok := u[5]; !ok {
			t.Error("union should contain 5")
		}
	})

	t.Run("multi_component", func(t *testing.T) {
		s0 := newScalarSet()
		s0.add(1)
		s1 := newScalarSet()
		s1.add(2)
		s2 := newScalarSet()
		s2.add(3)
		ct := componentTaint{s0, s1, s2}
		u := ct.union()
		for _, v := range []uint32{1, 2, 3} {
			if _, ok := u[v]; !ok {
				t.Errorf("union should contain %d", v)
			}
		}
	})
}

// TestMarkEverythingDependsOnEverything verifies the conservative fallback
// when the analyzer gives up (e.g., on function calls).
func TestMarkEverythingDependsOnEverything(t *testing.T) {
	mod := makeBasicMod()

	// Two f32 inputs at location 0 and 1, one vec4 output.
	argABinding := ir.Binding(ir.LocationBinding{Location: 0})
	argBBinding := ir.Binding(ir.LocationBinding{Location: 1})
	outBinding := ir.Binding(ir.LocationBinding{Location: 0})

	fn := ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0, Binding: &argABinding},
			{Name: "b", Type: 0, Binding: &argBBinding},
		},
		Result: &ir.FunctionResult{Type: 2, Binding: &outBinding},
	}

	// Use StmtCall to trigger giveUp.
	fn.Expressions = []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},
		{Kind: ir.ExprFunctionArgument{Index: 1}},
		{Kind: ir.ExprCallResult{Function: 0}},
	}
	f32H := ir.TypeHandle(0)
	vec4H := ir.TypeHandle(2)
	fn.ExpressionTypes = []ir.TypeResolution{
		{Handle: &f32H},
		{Handle: &f32H},
		{Handle: &vec4H},
	}
	callResult := ir.ExpressionHandle(2)
	ret := ir.ExpressionHandle(2)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{0, 1}, Result: &callResult}},
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name:     "main",
		Stage:    ir.StageFragment,
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

	// Every input should contribute to every output (conservative).
	outMask := MaskDwordsForScalars(deps.NumOutputScalars)
	for inS := uint32(0); inS < deps.NumInputScalars; inS++ {
		idx := inS * outMask
		if idx >= uint32(len(deps.InputScalarToOutputs)) {
			continue
		}
		got := deps.InputScalarToOutputs[idx]
		// Non-padding inputs should have some output bits set.
		isPadding := inS != 0 && inS != 4
		if !isPadding && got == 0 {
			t.Errorf("InputScalarToOutputs[%d] = 0x%x, expected non-zero (giveUp should mark all deps)", inS, got)
		}
	}
}

// TestTaintForSigEdgeCases covers taintForSig with out-of-range sigIdx and
// SystemManaged elements.
func TestTaintForSigEdgeCases(t *testing.T) {
	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 2, VectorRow: 0, SystemManaged: false},
		{ScalarStart: 2, NumChannels: 1, VectorRow: 1, SystemManaged: true},
	}

	s := &analysisState{inputs: inputs}

	t.Run("negative_sigIdx", func(t *testing.T) {
		taint := s.taintForSig(-1)
		if len(taint) != 0 {
			t.Errorf("expected empty taint for negative sigIdx, got %d entries", len(taint))
		}
	})

	t.Run("out_of_range_sigIdx", func(t *testing.T) {
		taint := s.taintForSig(99)
		if len(taint) != 0 {
			t.Errorf("expected empty taint for out-of-range sigIdx, got %d entries", len(taint))
		}
	})

	t.Run("system_managed_returns_empty", func(t *testing.T) {
		taint := s.taintForSig(1)
		if len(taint) != 0 {
			t.Errorf("expected empty taint for SystemManaged element, got %d entries", len(taint))
		}
	})

	t.Run("normal_element", func(t *testing.T) {
		taint := s.taintForSig(0)
		if len(taint) != 2 {
			t.Fatalf("expected 2 entries for 2-channel element, got %d", len(taint))
		}
		if _, ok := taint[0]; !ok {
			t.Error("expected taint to contain scalar 0")
		}
		if _, ok := taint[1]; !ok {
			t.Error("expected taint to contain scalar 1")
		}
	})
}

// TestExpandTaintToType covers expandTaintToType for scalar, vector, matrix,
// and struct types.
func TestExpandTaintToType(t *testing.T) {
	mod := makeBasicMod()

	taint := newScalarSet()
	taint.add(10)
	taint.add(11)

	t.Run("scalar", func(t *testing.T) {
		ct := expandTaintToType(mod, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, taint)
		if len(ct) != 1 {
			t.Fatalf("expected 1 component for scalar, got %d", len(ct))
		}
		if _, ok := ct[0][10]; !ok {
			t.Error("scalar component should inherit taint {10}")
		}
		if _, ok := ct[0][11]; !ok {
			t.Error("scalar component should inherit taint {11}")
		}
	})

	t.Run("vector_per_component", func(t *testing.T) {
		ct := expandTaintToType(mod, ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, taint)
		if len(ct) != 2 {
			t.Fatalf("expected 2 components for vec2, got %d", len(ct))
		}
		// Per-component distribution from sorted taint set.
		if _, ok := ct[0][10]; !ok {
			t.Error("vec2 component 0 should inherit scalar 10")
		}
		if _, ok := ct[1][11]; !ok {
			t.Error("vec2 component 1 should inherit scalar 11")
		}
	})

	t.Run("matrix_conservative", func(t *testing.T) {
		inner := ir.MatrixType{
			Columns: 2, Rows: 2,
			Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		}
		ct := expandTaintToType(mod, inner, taint)
		if len(ct) != 4 {
			t.Fatalf("expected 4 components for mat2x2, got %d", len(ct))
		}
		// Matrix is conservative: all components get the full taint.
		for i, c := range ct {
			if _, ok := c[10]; !ok {
				t.Errorf("matrix component %d should contain scalar 10", i)
			}
			if _, ok := c[11]; !ok {
				t.Errorf("matrix component %d should contain scalar 11", i)
			}
		}
	})
}

// TestTotalScalarCount verifies recursive scalar counting for all type kinds.
func TestTotalScalarCount(t *testing.T) {
	mod := makeBasicMod()
	// mod.Types: [0]=f32, [1]=vec3<f32>, [2]=vec4<f32>

	tests := []struct {
		name  string
		inner ir.TypeInner
		want  int
	}{
		{"scalar", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 1},
		{"vec3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 3},
		{"vec4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 4},
		{"mat2x3", ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 6},
		{"array", ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}}, 1},
		{
			"struct_2_members",
			ir.StructType{Members: []ir.StructMember{
				{Name: "a", Type: 0}, // f32 -> 1
				{Name: "b", Type: 1}, // vec3 -> 3
			}},
			4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := totalScalarCount(mod, tt.inner)
			if got != tt.want {
				t.Errorf("totalScalarCount = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestSortedScalars verifies the ascending sort of scalar set values.
func TestSortedScalars(t *testing.T) {
	s := newScalarSet()
	s.add(5)
	s.add(1)
	s.add(3)
	s.add(9)
	sorted := sortedScalars(s)
	want := []uint32{1, 3, 5, 9}
	if len(sorted) != len(want) {
		t.Fatalf("sorted len = %d, want %d", len(sorted), len(want))
	}
	for i := range want {
		if sorted[i] != want[i] {
			t.Errorf("sorted[%d] = %d, want %d", i, sorted[i], want[i])
		}
	}
}

// TestEmptyTaint verifies emptyTaint returns correct-length zero-dep taints.
func TestEmptyTaint(t *testing.T) {
	et := emptyTaint(3)
	if len(et) != 3 {
		t.Fatalf("emptyTaint(3) len = %d, want 3", len(et))
	}
	for i, s := range et {
		if len(s) != 0 {
			t.Errorf("emptyTaint component %d should be empty, has %d entries", i, len(s))
		}
	}
}

// TestFindArgHandle verifies findArgHandle locates (or fails to locate)
// function argument expressions.
func TestFindArgHandle(t *testing.T) {
	mod := makeBasicMod()
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
			{Kind: ir.ExprFunctionArgument{Index: 1}},
		},
	}
	ep := ir.EntryPoint{Function: fn}
	s := &analysisState{irMod: mod, ep: &ep}

	t.Run("found_arg0", func(t *testing.T) {
		h, ok := s.findArgHandle(0)
		if !ok || h != 1 {
			t.Errorf("findArgHandle(0) = (%d, %v), want (1, true)", h, ok)
		}
	})

	t.Run("found_arg1", func(t *testing.T) {
		h, ok := s.findArgHandle(1)
		if !ok || h != 3 {
			t.Errorf("findArgHandle(1) = (%d, %v), want (3, true)", h, ok)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		_, ok := s.findArgHandle(99)
		if ok {
			t.Error("findArgHandle(99) should return false")
		}
	})
}

// TestSystemManagedInputsSkipped verifies that SystemManaged inputs do not
// contribute to scalar/vector counts.
func TestSystemManagedInputsSkipped(t *testing.T) {
	mod := makeBasicMod()
	resBinding := ir.Binding(ir.LocationBinding{Location: 0})
	fn := ir.Function{
		Result: &ir.FunctionResult{Type: 0, Binding: &resBinding},
	}
	fn.Expressions = []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
	}
	f32H := ir.TypeHandle(0)
	fn.ExpressionTypes = []ir.TypeResolution{{Handle: &f32H}}
	ret := ir.ExpressionHandle(0)
	fn.Body = []ir.Statement{
		{Kind: ir.StmtReturn{Value: &ret}},
	}

	ep := ir.EntryPoint{
		Name: "main", Stage: ir.StageFragment, Function: fn,
	}

	inputs := []SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0, SystemManaged: true},
	}
	outputs := []SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0},
	}

	deps := Analyze(mod, &ep, inputs, outputs)
	if deps.NumInputScalars != 0 {
		t.Errorf("SystemManaged input should not count: NumInputScalars = %d, want 0", deps.NumInputScalars)
	}
	if deps.SigInputVectors != 0 {
		t.Errorf("SystemManaged input should not count: SigInputVectors = %d, want 0", deps.SigInputVectors)
	}
}

// TestBuildViewIDSigMapNonVSSort verifies that non-vertex-stage inputs
// are sorted by interface key (locations first, builtins last).
func TestBuildViewIDSigMapNonVSSort(t *testing.T) {
	mod := makeBasicMod()

	builtinBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	loc0Bind := ir.Binding(ir.LocationBinding{Location: 0})

	args := []ir.FunctionArgument{
		{Name: "pos", Type: 0, Binding: &builtinBind},
		{Name: "color", Type: 0, Binding: &loc0Bind},
	}

	// Non-VS: should sort locations before builtins.
	sigMap := buildViewIDSigMap(mod, args, false)

	// color (location 0) should come first (sigIdx 0).
	colorIdx, ok := sigMap[viewIDArgKey{argIdx: 1, memberIdx: -1}]
	if !ok {
		t.Fatal("color not found in sigMap")
	}
	// pos (builtin) should come second (sigIdx 1).
	posIdx, ok := sigMap[viewIDArgKey{argIdx: 0, memberIdx: -1}]
	if !ok {
		t.Fatal("pos not found in sigMap")
	}

	if colorIdx >= posIdx {
		t.Errorf("location binding should sort before builtin: colorIdx=%d, posIdx=%d", colorIdx, posIdx)
	}
}
