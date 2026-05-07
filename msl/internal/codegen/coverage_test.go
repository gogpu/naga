package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// backend.go coverage: Version, BoundsCheckPolicies, Options
// =============================================================================

func TestVersion_Less(t *testing.T) {
	tests := []struct {
		name string
		a, b Version
		want bool
	}{
		{"same_version", Version{2, 1}, Version{2, 1}, false},
		{"minor_less", Version{2, 0}, Version{2, 1}, true},
		{"minor_greater", Version{2, 1}, Version{2, 0}, false},
		{"major_less", Version{1, 2}, Version{2, 0}, true},
		{"major_greater", Version{3, 0}, Version{2, 1}, false},
		{"major_dominates_minor", Version{1, 9}, Version{2, 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Less(tt.b)
			if got != tt.want {
				t.Errorf("Version{%d,%d}.Less(Version{%d,%d}) = %v, want %v",
					tt.a.Major, tt.a.Minor, tt.b.Major, tt.b.Minor, got, tt.want)
			}
		})
	}
}

func TestBoundsCheckPolicies_Contains(t *testing.T) {
	tests := []struct {
		name   string
		policy BoundsCheckPolicies
		check  BoundsCheckPolicy
		want   bool
	}{
		{"index_matches", BoundsCheckPolicies{Index: BoundsCheckRestrict}, BoundsCheckRestrict, true},
		{"buffer_matches", BoundsCheckPolicies{Buffer: BoundsCheckReadZeroSkipWrite}, BoundsCheckReadZeroSkipWrite, true},
		{"image_matches", BoundsCheckPolicies{Image: BoundsCheckUnchecked}, BoundsCheckUnchecked, true},
		{"binding_array_ignored", BoundsCheckPolicies{BindingArray: BoundsCheckRestrict}, BoundsCheckRestrict, false},
		{"no_match", BoundsCheckPolicies{Index: BoundsCheckUnchecked, Buffer: BoundsCheckUnchecked, Image: BoundsCheckUnchecked}, BoundsCheckRestrict, false},
		{"all_match", BoundsCheckPolicies{Index: BoundsCheckRestrict, Buffer: BoundsCheckRestrict, Image: BoundsCheckRestrict}, BoundsCheckRestrict, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Contains(tt.check)
			if got != tt.want {
				t.Errorf("Contains(%v) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

func TestDefaultBoundsCheckPolicies(t *testing.T) {
	p := DefaultBoundsCheckPolicies()
	if p.Index != BoundsCheckReadZeroSkipWrite {
		t.Errorf("Index = %v, want ReadZeroSkipWrite", p.Index)
	}
	if p.Buffer != BoundsCheckReadZeroSkipWrite {
		t.Errorf("Buffer = %v, want ReadZeroSkipWrite", p.Buffer)
	}
	if p.Image != BoundsCheckReadZeroSkipWrite {
		t.Errorf("Image = %v, want ReadZeroSkipWrite", p.Image)
	}
	if p.BindingArray != BoundsCheckReadZeroSkipWrite {
		t.Errorf("BindingArray = %v, want ReadZeroSkipWrite", p.BindingArray)
	}
}

// =============================================================================
// functions.go coverage: blockEndsWithReturn, needsPassThrough, helpers
// =============================================================================

func TestBlockEndsWithReturn(t *testing.T) {
	tests := []struct {
		name  string
		block ir.Block
		want  bool
	}{
		{"empty_block", ir.Block{}, false},
		{"ends_with_return", ir.Block{{Kind: ir.StmtReturn{}}}, true},
		{"ends_with_kill", ir.Block{{Kind: ir.StmtKill{}}}, true},
		{"ends_with_break", ir.Block{{Kind: ir.StmtBreak{}}}, false},
		{"ends_with_continue", ir.Block{{Kind: ir.StmtContinue{}}}, false},
		{"nested_block_returns", ir.Block{
			{Kind: ir.StmtBlock{Block: []ir.Statement{{Kind: ir.StmtReturn{}}}}},
		}, true},
		{"nested_block_no_return", ir.Block{
			{Kind: ir.StmtBlock{Block: []ir.Statement{{Kind: ir.StmtBreak{}}}}},
		}, false},
		{"if_both_return", ir.Block{
			{Kind: ir.StmtIf{
				Condition: 0,
				Accept:    []ir.Statement{{Kind: ir.StmtReturn{}}},
				Reject:    []ir.Statement{{Kind: ir.StmtReturn{}}},
			}},
		}, true},
		{"if_only_accept_returns", ir.Block{
			{Kind: ir.StmtIf{
				Condition: 0,
				Accept:    []ir.Statement{{Kind: ir.StmtReturn{}}},
				Reject:    []ir.Statement{{Kind: ir.StmtBreak{}}},
			}},
		}, false},
		{"if_only_reject_returns", ir.Block{
			{Kind: ir.StmtIf{
				Condition: 0,
				Accept:    []ir.Statement{{Kind: ir.StmtBreak{}}},
				Reject:    []ir.Statement{{Kind: ir.StmtReturn{}}},
			}},
		}, false},
		{"switch_all_cases_return", ir.Block{
			{Kind: ir.StmtSwitch{
				Selector: 0,
				Cases: []ir.SwitchCase{
					{Value: ir.SwitchValueI32(0), Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
					{Value: ir.SwitchValueDefault{}, Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
				},
			}},
		}, true},
		{"switch_empty_cases", ir.Block{
			{Kind: ir.StmtSwitch{Selector: 0, Cases: nil}},
		}, false},
		{"switch_fallthrough_case", ir.Block{
			{Kind: ir.StmtSwitch{
				Selector: 0,
				Cases: []ir.SwitchCase{
					{Value: ir.SwitchValueI32(0), Body: nil, FallThrough: true},
					{Value: ir.SwitchValueDefault{}, Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
				},
			}},
		}, true},
		{"store_at_end", ir.Block{
			{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockEndsWithReturn(tt.block)
			if got != tt.want {
				t.Errorf("blockEndsWithReturn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockEndsWithTerminator(t *testing.T) {
	tests := []struct {
		name  string
		block ir.Block
		want  bool
	}{
		{"empty", ir.Block{}, false},
		{"return", ir.Block{{Kind: ir.StmtReturn{}}}, true},
		{"kill", ir.Block{{Kind: ir.StmtKill{}}}, true},
		{"break", ir.Block{{Kind: ir.StmtBreak{}}}, true},
		{"continue", ir.Block{{Kind: ir.StmtContinue{}}}, true},
		{"store", ir.Block{{Kind: ir.StmtStore{}}}, false},
		{"emit", ir.Block{{Kind: ir.StmtEmit{}}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockEndsWithTerminator(tt.block)
			if got != tt.want {
				t.Errorf("blockEndsWithTerminator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsPassThrough(t *testing.T) {
	tests := []struct {
		space ir.AddressSpace
		want  bool
	}{
		{ir.SpaceUniform, true},
		{ir.SpaceStorage, true},
		{ir.SpaceHandle, true},
		{ir.SpacePrivate, true},
		{ir.SpaceWorkGroup, true},
		{ir.SpaceImmediate, true},
		{ir.SpaceFunction, false},
		{ir.SpacePushConstant, false},
	}

	for _, tt := range tests {
		got := needsPassThrough(tt.space)
		if got != tt.want {
			t.Errorf("needsPassThrough(%v) = %v, want %v", tt.space, got, tt.want)
		}
	}
}

func TestNeedsCustomTypeName(t *testing.T) {
	tests := []struct {
		name  string
		inner ir.TypeInner
		want  bool
	}{
		{"struct", ir.StructType{}, true},
		{"array", ir.ArrayType{}, true},
		{"scalar", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, false},
		{"vector", ir.VectorType{Size: ir.Vec4}, false},
		{"matrix", ir.MatrixType{Columns: 4, Rows: 4}, false},
		{"sampler", ir.SamplerType{}, false},
		{"image", ir.ImageType{Dim: ir.Dim2D}, false},
		{"atomic", ir.AtomicType{}, false},
		{"pointer", ir.PointerType{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsCustomTypeName(tt.inner)
			if got != tt.want {
				t.Errorf("needsCustomTypeName(%T) = %v, want %v", tt.inner, got, tt.want)
			}
		})
	}
}

// =============================================================================
// functions.go coverage: builtin attributes
// =============================================================================

func TestBuiltinInputAttribute(t *testing.T) {
	tests := []struct {
		name    string
		builtin ir.BuiltinValue
		stage   ir.ShaderStage
		want    string
	}{
		{"position_vertex", ir.BuiltinPosition, ir.StageVertex, ""},
		{"position_fragment", ir.BuiltinPosition, ir.StageFragment, "[[position]]"},
		{"vertex_index", ir.BuiltinVertexIndex, ir.StageVertex, "[[vertex_id]]"},
		{"instance_index", ir.BuiltinInstanceIndex, ir.StageVertex, "[[instance_id]]"},
		{"front_facing", ir.BuiltinFrontFacing, ir.StageFragment, "[[front_facing]]"},
		{"sample_index", ir.BuiltinSampleIndex, ir.StageFragment, "[[sample_id]]"},
		{"sample_mask", ir.BuiltinSampleMask, ir.StageFragment, "[[sample_mask]]"},
		{"local_invocation_id", ir.BuiltinLocalInvocationID, ir.StageCompute, "[[thread_position_in_threadgroup]]"},
		{"local_invocation_index", ir.BuiltinLocalInvocationIndex, ir.StageCompute, "[[thread_index_in_threadgroup]]"},
		{"global_invocation_id", ir.BuiltinGlobalInvocationID, ir.StageCompute, "[[thread_position_in_grid]]"},
		{"workgroup_id", ir.BuiltinWorkGroupID, ir.StageCompute, "[[threadgroup_position_in_grid]]"},
		{"num_workgroups", ir.BuiltinNumWorkGroups, ir.StageCompute, "[[threadgroups_per_grid]]"},
		{"barycentric", ir.BuiltinBarycentric, ir.StageFragment, "[[barycentric_coord]]"},
		{"view_index", ir.BuiltinViewIndex, ir.StageVertex, "[[amplification_id]]"},
		{"primitive_index", ir.BuiltinPrimitiveIndex, ir.StageFragment, "[[primitive_id]]"},
		{"num_subgroups", ir.BuiltinNumSubgroups, ir.StageCompute, "[[simdgroups_per_threadgroup]]"},
		{"subgroup_size", ir.BuiltinSubgroupSize, ir.StageCompute, "[[threads_per_simdgroup]]"},
		{"subgroup_id", ir.BuiltinSubgroupID, ir.StageCompute, "[[simdgroup_index_in_threadgroup]]"},
		{"subgroup_invocation_id", ir.BuiltinSubgroupInvocationID, ir.StageCompute, "[[thread_index_in_simdgroup]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builtinInputAttribute(tt.builtin, tt.stage)
			if got != tt.want {
				t.Errorf("builtinInputAttribute(%v, %v) = %q, want %q", tt.builtin, tt.stage, got, tt.want)
			}
		})
	}
}

func TestBuiltinOutputAttribute(t *testing.T) {
	tests := []struct {
		name    string
		binding ir.BuiltinBinding
		want    string
	}{
		{"position", ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, "[[position]]"},
		{"position_invariant", ir.BuiltinBinding{Builtin: ir.BuiltinPosition, Invariant: true}, "[[position, invariant]]"},
		{"frag_depth", ir.BuiltinBinding{Builtin: ir.BuiltinFragDepth}, "[[depth(any)]]"},
		{"sample_mask", ir.BuiltinBinding{Builtin: ir.BuiltinSampleMask}, "[[sample_mask]]"},
		{"point_size", ir.BuiltinBinding{Builtin: ir.BuiltinPointSize}, "[[point_size]]"},
		{"unknown", ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := builtinOutputAttribute(tt.binding)
			if got != tt.want {
				t.Errorf("builtinOutputAttribute(%+v) = %q, want %q", tt.binding, got, tt.want)
			}
		})
	}
}

func TestResolveInterpolationString(t *testing.T) {
	tests := []struct {
		name   string
		interp *ir.Interpolation
		want   string
	}{
		{"nil_default", nil, "center_perspective"},
		{"flat", &ir.Interpolation{Kind: ir.InterpolationFlat}, "flat"},
		{"linear_center", &ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingCenter}, "center_no_perspective"},
		{"linear_centroid", &ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingCentroid}, "centroid_no_perspective"},
		{"linear_sample", &ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingSample}, "sample_no_perspective"},
		{"perspective_center", &ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingCenter}, "center_perspective"},
		{"perspective_centroid", &ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingCentroid}, "centroid_perspective"},
		{"perspective_sample", &ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingSample}, "sample_perspective"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveInterpolationString(tt.interp)
			if got != tt.want {
				t.Errorf("resolveInterpolationString(%+v) = %q, want %q", tt.interp, got, tt.want)
			}
		})
	}
}

// =============================================================================
// functions.go coverage: writeBindingAttribute, outputMemberAttribute
// =============================================================================

func TestWriteBindingAttribute(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	tests := []struct {
		name    string
		binding ir.Binding
		want    string
	}{
		{"builtin_position", ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, "[[position]]"},
		{"builtin_frag_depth", ir.BuiltinBinding{Builtin: ir.BuiltinFragDepth}, "[[depth(any)]]"},
		{"location_0", ir.LocationBinding{Location: 0}, "[[color(0)]]"},
		{"location_3", ir.LocationBinding{Location: 3}, "[[color(3)]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.writeBindingAttribute(tt.binding)
			if got != tt.want {
				t.Errorf("writeBindingAttribute(%+v) = %q, want %q", tt.binding, got, tt.want)
			}
		})
	}
}

func TestOutputMemberAttribute(t *testing.T) {
	opts := DefaultOptions()
	opts.LangVersion = Version2_4
	w := &Writer{
		module:             &ir.Module{},
		options:            &opts,
		minRequiredVersion: Version1_0,
	}

	tests := []struct {
		name    string
		binding ir.Binding
		stage   ir.ShaderStage
		want    string
	}{
		{"builtin_position_vertex", ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, ir.StageVertex, "[[position]]"},
		{"builtin_invariant_position", ir.BuiltinBinding{Builtin: ir.BuiltinPosition, Invariant: true}, ir.StageVertex, "[[position, invariant]]"},
		{"location_0_vertex", ir.LocationBinding{Location: 0}, ir.StageVertex, "[[user(loc0), center_perspective]]"},
		{"location_2_fragment", ir.LocationBinding{Location: 2}, ir.StageFragment, "[[color(2)]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.outputMemberAttribute(tt.binding, tt.stage)
			if got != tt.want {
				t.Errorf("outputMemberAttribute(%+v, %v) = %q, want %q", tt.binding, tt.stage, got, tt.want)
			}
		})
	}
}

// =============================================================================
// functions.go coverage: locationInputAttribute
// =============================================================================

func TestLocationInputAttribute(t *testing.T) {
	tests := []struct {
		name       string
		loc        ir.LocationBinding
		stage      ir.ShaderStage
		scalarKind ir.ScalarKind
		wantSub    string
	}{
		{"vertex_loc0", ir.LocationBinding{Location: 0}, ir.StageVertex, ir.ScalarFloat, "[[attribute(0)]]"},
		{"vertex_loc3", ir.LocationBinding{Location: 3}, ir.StageVertex, ir.ScalarFloat, "[[attribute(3)]]"},
		{"fragment_float_default_interp", ir.LocationBinding{Location: 0}, ir.StageFragment, ir.ScalarFloat, "center_perspective"},
		{"fragment_sint_flat", ir.LocationBinding{Location: 0}, ir.StageFragment, ir.ScalarSint, "flat"},
		{"fragment_uint_flat", ir.LocationBinding{Location: 0}, ir.StageFragment, ir.ScalarUint, "flat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := locationInputAttribute(tt.loc, tt.stage, tt.scalarKind)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("locationInputAttribute() = %q, want substring %q", got, tt.wantSub)
			}
		})
	}
}

// =============================================================================
// functions.go coverage: literalScalarTypeName
// =============================================================================

func TestLiteralScalarTypeName(t *testing.T) {
	w := &Writer{}

	tests := []struct {
		name string
		lit  ir.Literal
		want string
	}{
		{"i32", ir.Literal{Value: ir.LiteralI32(1)}, "int"},
		{"u32", ir.Literal{Value: ir.LiteralU32(1)}, "uint"},
		{"f32", ir.Literal{Value: ir.LiteralF32(1.0)}, "float"},
		{"bool", ir.Literal{Value: ir.LiteralBool(true)}, "bool"},
		{"f64", ir.Literal{Value: ir.LiteralF64(1.0)}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.literalScalarTypeName(tt.lit)
			if got != tt.want {
				t.Errorf("literalScalarTypeName(%+v) = %q, want %q", tt.lit, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Compile-level coverage: workgroup zero-init, private globals, ForLoopBounding
// =============================================================================

func TestCompile_WorkgroupZeroInit(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "shared_data", Type: tI32, Space: ir.SpaceWorkGroup},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "cs_main", Stage: ir.StageCompute, Function: ir.Function{
				Name: "cs_main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			}, Workgroup: [3]uint32{64, 1, 1}},
		},
	}

	opts := DefaultOptions()
	opts.ZeroInitializeWorkgroupMemory = true

	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	mustContainMSL(t, result, "threadgroup_barrier")
	mustContainMSL(t, result, "thread_position_in_threadgroup")
	mustContainMSL(t, result, "shared_data")
}

func TestCompile_PrivateGlobalVariable(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "counter", Type: tF32, Space: ir.SpacePrivate},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "cs_main", Stage: ir.StageCompute, Function: ir.Function{
				Name: "cs_main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprLoad{Pointer: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Value: ir.PointerType{Base: tF32, Space: ir.SpacePrivate}},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}, Workgroup: [3]uint32{1, 1, 1}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Private globals should be declared as local variables in the entry point
	mustContainMSL(t, result, "counter")
	mustContainMSL(t, result, "= {}")
}

func TestCompile_ForLoopStatement(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	tBool := ir.TypeHandle(1)

	init := ir.ExpressionHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				LocalVars: []ir.LocalVariable{
					{Name: "i", Type: tI32, Init: &init},
				},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
					{Kind: ir.ExprLocalVariable{Variable: 0}},
					{Kind: ir.ExprLoad{Pointer: 1}},
					{Kind: ir.Literal{Value: ir.LiteralI32(10)}},
					{Kind: ir.ExprBinary{Op: ir.BinaryLess, Left: 2, Right: 3}},
					{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
					{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 2, Right: 5}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
					{Value: ir.PointerType{Base: tI32, Space: ir.SpaceFunction}},
					{Handle: &tI32},
					{Handle: &tI32},
					{Handle: &tBool},
					{Handle: &tI32},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 5}}},
							{Kind: ir.StmtIf{
								Condition: 4,
								Accept:    nil,
								Reject:    []ir.Statement{{Kind: ir.StmtBreak{}}},
							}},
						},
						Continuing: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 5, End: 7}}},
							{Kind: ir.StmtStore{Pointer: 1, Value: 6}},
						},
					}},
				},
			},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	mustContainMSL(t, result, "while(true)")
}

// =============================================================================
// Compile-level coverage: point size, dual-source blending
// =============================================================================

func TestCompile_AllowAndForcePointSize(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	opts := DefaultOptions()
	opts.AllowAndForcePointSize = true

	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	mustContainMSL(t, result, "[[point_size]]")
}

func TestCompile_DualSourceBlending(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	tStruct := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(0)

	blendSrc0 := uint32(0)
	blendSrc1 := uint32(1)
	var loc0BlendSrc0 ir.Binding = ir.LocationBinding{Location: 0, BlendSrc: &blendSrc0}
	var loc0BlendSrc1 ir.Binding = ir.LocationBinding{Location: 0, BlendSrc: &blendSrc1}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "FragOutput", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "color0", Type: tVec4, Offset: 0, Binding: &loc0BlendSrc0},
					{Name: "color1", Type: tVec4, Offset: 16, Binding: &loc0BlendSrc1},
				},
				Span: 32,
			}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Result: &ir.FunctionResult{
					Type: tStruct,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tStruct}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tStruct},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Dual-source blending uses index() attribute
	mustContainMSL(t, result, "[[color(0) index(0)]]")
	mustContainMSL(t, result, "[[color(0) index(1)]]")
}

// =============================================================================
// Compile-level coverage: storage buffer (device space) with read-only
// =============================================================================

func TestCompile_StorageBufferAsEntryParam(t *testing.T) {
	tVec4 := ir.TypeHandle(1)
	tBuf := ir.TypeHandle(2)
	retExpr := ir.ExpressionHandle(2)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "StorageData", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "value", Type: tVec4, Offset: 0},
				},
				Span: 16,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "storage_buf", Type: tBuf, Space: ir.SpaceStorage,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
					{Kind: ir.ExprLoad{Pointer: 1}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Value: ir.PointerType{Base: tBuf, Space: ir.SpaceStorage}},
					{Value: ir.PointerType{Base: tVec4, Space: ir.SpaceStorage}},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Storage buffer should be "device" address space
	mustContainMSL(t, result, "device")
	mustContainMSL(t, result, "[[buffer(0)]]")
}

// =============================================================================
// types.go coverage: array wrapper struct, BindingArray type emission
// =============================================================================

func TestCompile_BindingArrayType(t *testing.T) {
	tTex := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)

	retExpr := ir.ExpressionHandle(0)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "tex2d", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "tex_array", Inner: ir.BindingArrayType{Base: tTex, Size: func() *uint32 { v := uint32(4); return &v }()}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// BindingArrayType should emit NagaArgumentBufferWrapper template
	mustContainMSL(t, result, "NagaArgumentBufferWrapper")
}

// =============================================================================
// Compile-level coverage: multiple entry points with TranslationInfo
// =============================================================================

func TestCompile_MultipleEntryPoints_Info(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var loc0Binding ir.Binding = ir.LocationBinding{Location: 0}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &loc0Binding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
			{Name: "cs_main", Stage: ir.StageCompute, Function: ir.Function{
				Name: "cs_main",
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			}, Workgroup: [3]uint32{64, 1, 1}},
		},
	}

	result, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(info.EntryPointNames) != 3 {
		t.Errorf("Expected 3 entry point names, got %d", len(info.EntryPointNames))
	}

	// All three entry point stage keywords should be present
	mustContainMSL(t, result, "vertex")
	mustContainMSL(t, result, "fragment")
	mustContainMSL(t, result, "kernel")
}

// =============================================================================
// functions.go coverage: typeContainsAtomics
// =============================================================================

func TestCompile_WorkgroupAtomicZeroInit(t *testing.T) {
	tAtomicI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "atomic_i32", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "counter", Type: tAtomicI32, Space: ir.SpaceWorkGroup},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "cs_main", Stage: ir.StageCompute, Function: ir.Function{
				Name: "cs_main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tAtomicI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			}, Workgroup: [3]uint32{64, 1, 1}},
		},
	}

	opts := DefaultOptions()
	opts.ZeroInitializeWorkgroupMemory = true

	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Atomic zero-init uses atomic_store_explicit
	mustContainMSL(t, result, "atomic_store_explicit")
	mustContainMSL(t, result, "memory_order_relaxed")
}

// =============================================================================
// functions.go coverage: FakeMissingBindings
// =============================================================================

func TestCompile_FakeMissingBindings(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(1)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "my_uniform", Type: tF32, Space: ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	opts := DefaultOptions()
	opts.FakeMissingBindings = true

	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// With FakeMissingBindings, resources get [[user(fake0)]] attributes
	mustContainMSL(t, result, "[[user(fake0)]]")
}

// =============================================================================
// functions.go coverage: computeResourceMap with PerEntryPointMap
// =============================================================================

func TestCompile_PerEntryPointMap(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(1)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	bufSlot := uint8(5)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "my_uniform", Type: tF32, Space: ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	opts := DefaultOptions()
	opts.PerEntryPointMap = map[string]EntryPointResources{
		"vs_main": {
			Resources: map[ir.ResourceBinding]BindTarget{
				{Group: 0, Binding: 0}: {Buffer: &bufSlot},
			},
		},
	}

	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// With explicit per-entry-point map, buffer should get slot 5
	mustContainMSL(t, result, "[[buffer(5)]]")
}

// =============================================================================
// expressions.go coverage: scalarCastTypeName edge cases
// =============================================================================

func TestScalarCastTypeName_Width8(t *testing.T) {
	w := &Writer{}
	tests := []struct {
		name    string
		kind    ir.ScalarKind
		convert *uint8
		want    string
	}{
		{"sint_64", ir.ScalarSint, ptrUint8(8), "long"},
		{"uint_64", ir.ScalarUint, ptrUint8(8), "ulong"},
		{"float_default", ir.ScalarFloat, nil, "float"},
		{"unknown_default", ir.ScalarKind(99), nil, "int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.scalarCastTypeName(tt.kind, tt.convert)
			if got != tt.want {
				t.Errorf("scalarCastTypeName(%v, %v) = %q, want %q", tt.kind, tt.convert, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Compile-level coverage: multiple functions with pass-through globals
// =============================================================================

func TestCompile_MultipleFunctionsWithSharedGlobals(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tTex := ir.TypeHandle(2)
	tSamp := ir.TypeHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "tex2d", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
			{Name: "samp", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Type: tTex, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
			{Name: "smp", Space: ir.SpaceHandle, Type: tSamp, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}},
		},
		Functions: []ir.Function{
			{
				Name:   "helper_a",
				Result: &ir.FunctionResult{Type: tF32},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tTex},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: ptrExpr(1)}},
				},
			},
			{
				Name:   "helper_b",
				Result: &ir.FunctionResult{Type: tF32},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tTex},
					{Handle: &tSamp},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: ptrExpr(2)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "cs_main", Stage: ir.StageCompute, Function: ir.Function{
				Name: "cs_main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: 0}},
					{Kind: ir.ExprCallResult{Function: 1}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0, Result: ptrExpr(0)}},
					{Kind: ir.StmtCall{Function: 1, Result: ptrExpr(1)}},
					{Kind: ir.StmtReturn{}},
				},
			}, Workgroup: [3]uint32{1, 1, 1}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Helper functions should have pass-through globals as params (no binding attrs)
	mustContainMSL(t, result, "helper_a")
	mustContainMSL(t, result, "helper_b")
	mustContainMSL(t, result, "kernel")
}

// =============================================================================
// Compile-level coverage: ImageSample expression
// =============================================================================

func TestCompile_ImageSampleExpression(t *testing.T) {
	tVec2 := ir.TypeHandle(1)
	tVec4 := ir.TypeHandle(2)
	tTex := ir.TypeHandle(3)
	tSamp := ir.TypeHandle(4)

	retExpr := ir.ExpressionHandle(4)
	var loc0Binding ir.Binding = ir.LocationBinding{Location: 0}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec2f", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "tex2d", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
			{Name: "samp", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "my_tex", Space: ir.SpaceHandle, Type: tTex, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
			{Name: "my_smp", Space: ir.SpaceHandle, Type: tSamp, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: tVec2, Binding: &loc0Binding},
				},
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: bindingPtr(ir.LocationBinding{Location: 0}),
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.ExprZeroValue{Type: tVec2}},
					{Kind: ir.ExprImageSample{
						Image: 1, Sampler: 2, Coordinate: 3,
						Level: ir.SampleLevelAuto{},
					}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec2},
					{Handle: &tTex},
					{Handle: &tSamp},
					{Handle: &tVec2},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// ImageSample should produce .sample() call
	mustContainMSL(t, result, ".sample(")
	mustContainMSL(t, result, "[[texture(0)]]")
	mustContainMSL(t, result, "[[sampler(0)]]")
}

// =============================================================================
// Compile-level coverage: constant emission with named constants
// =============================================================================

func TestCompile_NamedConstants(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants: []ir.Constant{
			{Name: "PI", Type: tF32, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40490fdb}},
			{Name: "MAX_COUNT", Type: tF32, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x42c80000}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(3.14159)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(100.0)}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	mustContainMSL(t, result, "constant")
	mustContainMSL(t, result, "PI")
	mustContainMSL(t, result, "MAX_COUNT")
}

// =============================================================================
// functions.go coverage: isEntryPointFunction (always false for Functions[])
// =============================================================================

func TestIsEntryPointFunction(t *testing.T) {
	w := &Writer{module: &ir.Module{}}
	// Module.Functions[] handles are never entry points (they are inline in EntryPoint)
	if w.isEntryPointFunction(0) {
		t.Error("isEntryPointFunction(0) should return false")
	}
	if w.isEntryPointFunction(100) {
		t.Error("isEntryPointFunction(100) should return false")
	}
}

// =============================================================================
// functions.go coverage: writeScalarZeroLiteral
// =============================================================================

func TestWriteScalarZeroLiteral(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"float_zero", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "0.0"},
		{"uint_zero", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "0u"},
		{"bool_zero", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "false"},
		{"sint_zero", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Writer{}
			w.writeScalarZeroLiteral(tt.scalar)
			got := w.Out.String()
			if got != tt.want {
				t.Errorf("writeScalarZeroLiteral(%+v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Compile-level coverage: empty Emit, nested blocks
// =============================================================================

func TestCompile_EmptyBlock(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtBlock{Block: nil}}, // empty block
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	mustContainMSL(t, result, "return;")
}

// =============================================================================
// Compile-level coverage: PushConstant address space
// =============================================================================

func TestAddressSpaceName_PushConstant(t *testing.T) {
	got := addressSpaceName(ir.SpacePushConstant)
	if got != "constant" {
		t.Errorf("addressSpaceName(SpacePushConstant) = %q, want %q", got, "constant")
	}
}

// =============================================================================
// Compile-level coverage: struct padding
// =============================================================================

func TestCompile_StructWithPadding(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Padded", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: tF32, Offset: 0},
					// Padding between offset 4 and 16
					{Name: "y", Type: tVec4, Offset: 16},
				},
				Span: 32,
			}},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	mustContainMSL(t, result, "struct Padded")
	mustContainMSL(t, result, "float x")
}
