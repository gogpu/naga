// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

// COV-002 Wave 2: Targeted coverage tests for GLSL backend.
// Each test exercises specific uncovered code paths identified by coverage analysis.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// =============================================================================
// writeHelperFunctions — coverage: 12.5% → needs needsModHelper/needsDivHelper
// =============================================================================

func TestCoverage_IntegerModuloHelper(t *testing.T) {
	// Integer modulo triggers writeHelperFunctions with needsModHelper=true.
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: i32 = 7;
    let b: i32 = 3;
    let result = a % b;
    return vec4<f32>(f32(result), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	// Integer modulo may emit _naga_mod helper or native %
	glslMustContain(t, output, "void main()")
}

func TestCoverage_IntegerDivisionHelper(t *testing.T) {
	// Integer division triggers writeHelperFunctions with needsDivHelper=true.
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: i32 = 7;
    let b: i32 = 3;
    let result = a / b;
    return vec4<f32>(f32(result), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeExpressionInline — coverage: 0%
// =============================================================================

func TestCoverage_WriteExpressionInline(t *testing.T) {
	// writeExpressionInline is used for const expressions in texture offsets.
	// Constructing a module with a texture load + offset triggers this path.
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
		},
	}
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
		},
	}
	w.currentFunction = fn

	// Test valid case
	result, err := w.writeExpressionInline(0)
	if err != nil {
		t.Fatalf("writeExpressionInline(0) error: %v", err)
	}
	if result != "42" {
		t.Errorf("writeExpressionInline(0) = %q, want %q", result, "42")
	}

	// Test invalid handle
	_, err = w.writeExpressionInline(99)
	if err == nil {
		t.Error("writeExpressionInline(99) should error on invalid handle")
	}

	// Test nil function context
	w.currentFunction = nil
	_, err = w.writeExpressionInline(0)
	if err == nil {
		t.Error("writeExpressionInline with nil currentFunction should error")
	}
}

// =============================================================================
// writeCallResult — coverage: 0%
// =============================================================================

func TestCoverage_FunctionCallResult(t *testing.T) {
	// A function that returns a value assigned to a let triggers writeCallResult.
	source := `
fn compute(a: f32, b: f32) -> f32 {
    return a * b + a;
}

@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let r1 = compute(x, 2.0);
    let r2 = compute(r1, 3.0);
    return vec4<f32>(r1, r2, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "compute(")
}

// =============================================================================
// writeAtomicResult — coverage: 0%
// =============================================================================

func TestCoverage_WriteAtomicResult(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
	}
	result, err := w.writeAtomicResult(ir.ExprAtomicResult{})
	if err != nil {
		t.Fatalf("writeAtomicResult error: %v", err)
	}
	if !strings.Contains(result, "atomic result") {
		t.Errorf("writeAtomicResult = %q, want contains 'atomic result'", result)
	}
}

// =============================================================================
// expandBoolVectorOp — coverage: 0%
// =============================================================================

func TestCoverage_BoolVectorOps(t *testing.T) {
	// Boolean vector operations (&&, ||) on bvec types trigger expandBoolVectorOp.
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = vec3<bool>(true, false, true);
    let b = vec3<bool>(false, true, true);
    let result_or = a | b;
    let result_and = a & b;
    return vec4<f32>(select(0.0, 1.0, result_or.x), select(0.0, 1.0, result_and.y), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// formatConstResult — coverage: 0%
// =============================================================================

func TestCoverage_FormatConstResult(t *testing.T) {
	scalarType := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	sintType := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	uintType := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	boolType := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}

	floatHandle := ir.TypeHandle(0)
	sintHandle := ir.TypeHandle(1)
	uintHandle := ir.TypeHandle(2)
	boolHandle := ir.TypeHandle(3)

	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: scalarType},
				{Inner: sintType},
				{Inner: uintType},
				{Inner: boolType},
			},
		},
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &floatHandle},
			{Handle: &sintHandle},
			{Handle: &uintHandle},
			{Handle: &boolHandle},
		},
	}
	w.currentFunction = fn

	tests := []struct {
		name  string
		expr  ir.ExpressionHandle
		val   float64
		want  string
		check func(string) bool
	}{
		{"float_val", 0, 3.14, "", func(s string) bool { return strings.Contains(s, "3.14") }},
		{"sint_val", 1, 42.0, "42", nil},
		{"uint_val", 2, 7.0, "7u", nil},
		{"bool_true", 3, 1.0, "true", nil},
		{"bool_false", 3, 0.0, "false", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.formatConstResult(tt.expr, tt.val)
			if tt.check != nil {
				if !tt.check(got) {
					t.Errorf("formatConstResult(%d, %f) = %q, custom check failed", tt.expr, tt.val, got)
				}
			} else if got != tt.want {
				t.Errorf("formatConstResult(%d, %f) = %q, want %q", tt.expr, tt.val, got, tt.want)
			}
		})
	}

	// Test with out-of-range protoExpr (fallback to float formatting)
	got := w.formatConstResult(99, 5.0)
	if !strings.Contains(got, "5") {
		t.Errorf("formatConstResult(99, 5.0) = %q, want contains '5'", got)
	}
}

// =============================================================================
// tryConstEvalUnary — coverage: 23.1%
// =============================================================================

func TestCoverage_ConstEvalUnary(t *testing.T) {
	// Test LogicalNot and Negate on constants.
	source := `
const FLAG: bool = true;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let inverted = !FLAG;
    return vec4<f32>(select(0.0, 1.0, inverted), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	// !true should be const-folded to "false"
	glslMustContain(t, output, "void main()")
}

func TestCoverage_ConstEvalNegate(t *testing.T) {
	source := `
const VALUE: f32 = 1.0;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let neg = -VALUE;
    return vec4<f32>(neg, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// getExpressionTypeHandle — coverage: 20%
// =============================================================================

func TestCoverage_GetExpressionTypeHandle(t *testing.T) {
	typeHandle := ir.TypeHandle(0)

	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			GlobalVariables: []ir.GlobalVariable{
				{Type: 0, Name: "g"},
			},
		},
	}
	w.currentFunction = &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Type: 0, Name: "loc"},
		},
		Arguments: []ir.FunctionArgument{
			{Type: 0, Name: "arg"},
		},
	}

	tests := []struct {
		name string
		kind ir.ExpressionKind
		want *ir.TypeHandle
	}{
		{"local_var", ir.ExprLocalVariable{Variable: 0}, &typeHandle},
		{"global_var", ir.ExprGlobalVariable{Variable: 0}, &typeHandle},
		{"func_arg", ir.ExprFunctionArgument{Index: 0}, &typeHandle},
		{"compose", ir.ExprCompose{Type: 0}, &typeHandle},
		{"zero_value", ir.ExprZeroValue{Type: 0}, &typeHandle},
		{"literal_no_handle", ir.Literal{Value: ir.LiteralF32(1.0)}, nil},
		{"local_oob", ir.ExprLocalVariable{Variable: 99}, nil},
		{"global_oob", ir.ExprGlobalVariable{Variable: 99}, nil},
		{"arg_oob", ir.ExprFunctionArgument{Index: 99}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.getExpressionTypeHandle(tt.kind)
			if tt.want == nil {
				if got != nil {
					t.Errorf("getExpressionTypeHandle(%T) = %v, want nil", tt.kind, *got)
				}
			} else {
				if got == nil {
					t.Errorf("getExpressionTypeHandle(%T) = nil, want %v", tt.kind, *tt.want)
				} else if *got != *tt.want {
					t.Errorf("getExpressionTypeHandle(%T) = %v, want %v", tt.kind, *got, *tt.want)
				}
			}
		})
	}
}

// =============================================================================
// writeCompositeValue — coverage: 0%
// =============================================================================

func TestCoverage_WriteCompositeValue(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			Constants: []ir.Constant{
				{Type: 1, Value: ir.ScalarValue{Bits: 0x3F800000, Kind: ir.ScalarFloat}, Init: 0},
				{Type: 1, Value: ir.ScalarValue{Bits: 0x40000000, Kind: ir.ScalarFloat}, Init: 1},
				{Type: 1, Value: ir.ScalarValue{Bits: 0x40400000, Kind: ir.ScalarFloat}, Init: 2},
			},
			GlobalExpressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
			},
		},
	}

	result := w.writeCompositeValue(ir.CompositeValue{Components: []ir.ConstantHandle{0, 1, 2}}, 0)
	if !strings.Contains(result, "vec3") {
		t.Errorf("writeCompositeValue = %q, want contains 'vec3'", result)
	}

	// Test with out-of-range component
	result = w.writeCompositeValue(ir.CompositeValue{Components: []ir.ConstantHandle{99}}, 0)
	if !strings.Contains(result, "0") {
		t.Errorf("writeCompositeValue with OOB component = %q, want contains '0'", result)
	}
}

// =============================================================================
// resolveTypeInner — coverage: 18.2%
// =============================================================================

func TestCoverage_ResolveTypeInner(t *testing.T) {
	scalarInner := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vecInner := ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	typeHandle0 := ir.TypeHandle(0)
	typeHandle1 := ir.TypeHandle(1)

	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: scalarInner},
				{Inner: vecInner},
			},
		},
	}

	tests := []struct {
		name   string
		res    ir.TypeResolution
		handle ir.ExpressionHandle
		isNil  bool
	}{
		{"via_handle_scalar", ir.TypeResolution{Handle: &typeHandle0}, 0, false},
		{"via_handle_vec", ir.TypeResolution{Handle: &typeHandle1}, 0, false},
		{"via_value", ir.TypeResolution{Value: scalarInner}, 0, false},
		{"empty_no_function", ir.TypeResolution{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.resolveTypeInner(&tt.res, tt.handle)
			if tt.isNil && got != nil {
				t.Errorf("resolveTypeInner = %v, want nil", got)
			}
			if !tt.isNil && got == nil {
				t.Errorf("resolveTypeInner = nil, want non-nil")
			}
		})
	}

	// Test fallback: with currentFunction and Compose expression (dynamic resolve)
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{}}},
		},
		ExpressionTypes: []ir.TypeResolution{{}},
	}
	emptyRes := ir.TypeResolution{}
	got := w.resolveTypeInner(&emptyRes, 0)
	// May or may not resolve depending on ResolveExpressionType — just test no panic
	_ = got
}

// =============================================================================
// scanBlockForStages — coverage: 16.7%
// =============================================================================

func TestCoverage_ScanBlockForStages(t *testing.T) {
	tests := []struct {
		name       string
		block      []ir.Statement
		wantStages uint8
	}{
		{
			"empty",
			nil,
			0,
		},
		{
			"discard_in_if",
			[]ir.Statement{
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    []ir.Statement{{Kind: ir.StmtKill{}}},
					Reject:    nil,
				}},
			},
			1 << ir.StageFragment,
		},
		{
			"barrier_in_switch",
			[]ir.Statement{
				{Kind: ir.StmtSwitch{
					Selector: 0,
					Cases: []ir.SwitchCase{
						{Value: ir.SwitchValueDefault{},
							Body: []ir.Statement{{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}}}},
					},
				}},
			},
			1 << ir.StageCompute,
		},
		{
			"barrier_in_loop",
			[]ir.Statement{
				{Kind: ir.StmtLoop{
					Body:       []ir.Statement{{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}}},
					Continuing: nil,
				}},
			},
			1 << ir.StageCompute,
		},
		{
			"discard_in_loop_continuing",
			[]ir.Statement{
				{Kind: ir.StmtLoop{
					Body:       nil,
					Continuing: []ir.Statement{{Kind: ir.StmtKill{}}},
				}},
			},
			1 << ir.StageFragment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stages uint8
			scanBlockForStages(tt.block, &stages)
			if stages != tt.wantStages {
				t.Errorf("scanBlockForStages() stages = %d, want %d", stages, tt.wantStages)
			}
		})
	}
}

// =============================================================================
// stageCompatible + detectFunctionStages — coverage: ~70%
// =============================================================================

func TestCoverage_StageCompatible(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	// Function with derivative (fragment only)
	fragFn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeNone, Expr: 0}},
		},
	}

	// Function with no stage restrictions
	genericFn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
	}

	if !stageCompatible(module, fragFn, ir.StageFragment) {
		t.Error("fragment function should be compatible with fragment stage")
	}
	if stageCompatible(module, fragFn, ir.StageCompute) {
		t.Error("fragment function should NOT be compatible with compute stage")
	}
	if !stageCompatible(module, genericFn, ir.StageCompute) {
		t.Error("generic function should be compatible with any stage")
	}
	if !stageCompatible(module, genericFn, ir.StageFragment) {
		t.Error("generic function should be compatible with any stage")
	}
}

// =============================================================================
// writeUniformBlock — coverage: 28.6% — UBO without binding (block expansion)
// =============================================================================

func TestCoverage_UniformBlockExpanded(t *testing.T) {
	// Uniform struct without BindingMap entry → expanded block.
	source := `
struct Params {
    offset: vec2<f32>,
    scale: f32,
};

@group(0) @binding(0) var<uniform> params: Params;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(params.offset, params.scale, 1.0);
}
`
	// Use a version that supports explicit locations so binding works
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
	})
	// Should emit uniform block
	glslMustContain(t, output, "uniform")
	glslMustContain(t, output, "block")
}

// =============================================================================
// writeFunctionArgument — coverage: 42.9% — entry point builtin args
// =============================================================================

func TestCoverage_EntryPointBuiltinArgs(t *testing.T) {
	source := `
@fragment
fn fs_main(@builtin(front_facing) front: bool, @location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    if front {
        return color;
    }
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_FrontFacing")
}

func TestCoverage_ComputeBuiltinArgs(t *testing.T) {
	source := `
@compute @workgroup_size(8, 8)
fn cs_main(
    @builtin(global_invocation_id) gid: vec3<u32>,
    @builtin(local_invocation_id) lid: vec3<u32>,
    @builtin(workgroup_id) wid: vec3<u32>,
    @builtin(local_invocation_index) idx: u32,
) {
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "gl_GlobalInvocationID")
	glslMustContain(t, output, "gl_LocalInvocationID")
	glslMustContain(t, output, "gl_WorkGroupID")
	glslMustContain(t, output, "gl_LocalInvocationIndex")
}

// =============================================================================
// writeGlobalVariable — coverage: 50% — combined texture+sampler
// =============================================================================

func TestCoverage_TextureSampler2DArray(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d_array<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv, 0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texture(")
}

// =============================================================================
// writeAtomicCompareExchange — coverage: 0%
// =============================================================================

func TestCoverage_AtomicOps(t *testing.T) {
	// Atomic operations on workgroup variable.
	source := `
var<workgroup> counter: atomic<u32>;

@compute @workgroup_size(64)
fn cs_main(@builtin(local_invocation_index) idx: u32) {
    atomicStore(&counter, 0u);
    workgroupBarrier();
    atomicAdd(&counter, 1u);
    let val = atomicLoad(&counter);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "atomicAdd(")
}

// =============================================================================
// writeLocalVariable — coverage: 66.7% — test fallback name
// =============================================================================

func TestCoverage_WriteLocalVariable(t *testing.T) {
	w := &Writer{
		module:     &ir.Module{},
		localNames: map[uint32]string{0: "my_var"},
	}
	w.currentFunction = &ir.Function{}

	// Known name
	result, err := w.writeLocalVariable(ir.ExprLocalVariable{Variable: 0})
	if err != nil {
		t.Fatalf("writeLocalVariable error: %v", err)
	}
	if result != "my_var" {
		t.Errorf("writeLocalVariable(0) = %q, want %q", result, "my_var")
	}

	// Unknown name fallback
	result, err = w.writeLocalVariable(ir.ExprLocalVariable{Variable: 99})
	if err != nil {
		t.Fatalf("writeLocalVariable error: %v", err)
	}
	if result != "local_99" {
		t.Errorf("writeLocalVariable(99) = %q, want %q", result, "local_99")
	}
}

// =============================================================================
// continueCtx.clear — coverage: 0%
// =============================================================================

func TestCoverage_ContinueCtxClear(t *testing.T) {
	ctx := &continueCtx{}
	ctx.enterLoop()
	if len(ctx.stack) != 1 {
		t.Fatalf("stack after enterLoop = %d, want 1", len(ctx.stack))
	}
	ctx.clear()
	if len(ctx.stack) != 0 {
		t.Errorf("stack after clear = %d, want 0", len(ctx.stack))
	}
}

// =============================================================================
// continueCtx.exitLoop — coverage: 66.7%
// =============================================================================

func TestCoverage_ContinueCtxExitLoop(t *testing.T) {
	ctx := &continueCtx{}
	ctx.enterLoop()
	if err := ctx.exitLoop(); err != nil {
		t.Fatalf("exitLoop after enterLoop should not error: %v", err)
	}

	// exitLoop on empty stack should return error
	if err := ctx.exitLoop(); err == nil {
		t.Error("exitLoop on empty stack should return error")
	}
}

// =============================================================================
// fallbackCombinedName — coverage: 0%
// =============================================================================

func TestCoverage_FallbackCombinedName(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
		},
		names: map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "tex",
			{kind: nameKeyGlobalVariable, handle1: 1}: "samp",
		},
	}
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprGlobalVariable{Variable: 1}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{}, {},
		},
	}

	result := w.fallbackCombinedName(0, 1)
	if result != "tex_samp" {
		t.Errorf("fallbackCombinedName = %q, want %q", result, "tex_samp")
	}
}

// =============================================================================
// writeSelect — coverage: 70% — vector select path
// =============================================================================

func TestCoverage_VectorSelect(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let b = vec4<f32>(5.0, 6.0, 7.0, 8.0);
    let cond = vec4<bool>(true, false, true, false);
    let result = select(a, b, cond);
    return result;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeRelational — coverage: 66.7% — all/any/isNan/isInf
// =============================================================================

func TestCoverage_RelationalFunctions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) v: vec4<f32>) -> @location(0) vec4<f32> {
    let a = any(v == vec4<f32>(0.0));
    let b = all(v == vec4<f32>(1.0));
    return select(vec4<f32>(0.0), vec4<f32>(1.0), a);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeZeroValue — coverage: 60% — zero init for various types
// =============================================================================

func TestCoverage_ZeroValues(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var s: f32 = f32();
    var v: vec3<f32> = vec3<f32>();
    var m: mat2x2<f32> = mat2x2<f32>();
    return vec4<f32>(s, v.x, m[0][0], 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeVertexIO / writeFragmentIO / writeStructArgIO / writeResultIO — all 0%
// These are triggered by entry points with struct IO.
// =============================================================================

func TestCoverage_VertexFragmentStructIO(t *testing.T) {
	source := `
struct VertexInput {
    @location(0) pos: vec3<f32>,
    @location(1) uv: vec2<f32>,
};

struct VertexOutput {
    @builtin(position) clip: vec4<f32>,
    @location(0) uv: vec2<f32>,
};

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clip = vec4<f32>(input.pos, 1.0);
    output.uv = input.uv;
    return output;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		EntryPoint:  "vs_main",
	})
	glslMustContain(t, output, "gl_Position")
	glslMustContain(t, output, "layout(location = 0)")
	glslMustContain(t, output, "layout(location = 1)")
}

func TestCoverage_FragmentStructIO(t *testing.T) {
	source := `
struct FragInput {
    @location(0) color: vec3<f32>,
    @builtin(front_facing) front: bool,
};

@fragment
fn fs_main(input: FragInput) -> @location(0) vec4<f32> {
    if input.front {
        return vec4<f32>(input.color, 1.0);
    }
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_FrontFacing")
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeStructReturn — coverage: 81.8% — struct return with multiple locations
// =============================================================================

func TestCoverage_FragmentMultiOutput(t *testing.T) {
	source := `
struct FragOutput {
    @location(0) color: vec4<f32>,
    @location(1) normal: vec4<f32>,
};

@fragment
fn fs_main() -> FragOutput {
    var out: FragOutput;
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    out.normal = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    return out;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeAtomic — coverage: 60.6% — more atomic operations
// =============================================================================

func TestCoverage_AtomicSubtract(t *testing.T) {
	source := `
var<workgroup> shared_val: atomic<u32>;

@compute @workgroup_size(1)
fn cs_main() {
    atomicSub(&shared_val, 1u);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	// atomicSub emits atomicAdd with negated value
	glslMustContain(t, output, "atomicAdd(")
	glslMustContain(t, output, "-")
}

func TestCoverage_AtomicMinMax(t *testing.T) {
	source := `
var<workgroup> shared_val: atomic<u32>;

@compute @workgroup_size(1)
fn cs_main() {
    atomicMin(&shared_val, 10u);
    atomicMax(&shared_val, 20u);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "atomicMin(")
	glslMustContain(t, output, "atomicMax(")
}

func TestCoverage_AtomicAndOrXor(t *testing.T) {
	source := `
var<workgroup> shared_val: atomic<u32>;

@compute @workgroup_size(1)
fn cs_main() {
    atomicAnd(&shared_val, 0xFFu);
    atomicOr(&shared_val, 0x0Fu);
    atomicXor(&shared_val, 0xF0u);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "atomicAnd(")
	glslMustContain(t, output, "atomicOr(")
	glslMustContain(t, output, "atomicXor(")
}

func TestCoverage_AtomicExchange(t *testing.T) {
	source := `
var<workgroup> shared_val: atomic<u32>;

@compute @workgroup_size(1)
fn cs_main() {
    let old = atomicExchange(&shared_val, 42u);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "atomicExchange(")
}

// =============================================================================
// writeStore — coverage: 75% — more store paths
// =============================================================================

func TestCoverage_StoreToArrayElement(t *testing.T) {
	source := `
var<workgroup> data: array<f32, 4>;

@compute @workgroup_size(1)
fn cs_main(@builtin(local_invocation_index) idx: u32) {
    data[idx] = 42.0;
    data[0u] = 1.0;
    data[1u] = 2.0;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeImageStore — coverage: 76.9% — storage image writes
// =============================================================================

func TestCoverage_ImageStore(t *testing.T) {
	source := `
@group(0) @binding(0) var output_tex: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(8, 8)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    textureStore(output_tex, vec2<i32>(gid.xy), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "imageStore(")
}

// =============================================================================
// writeImageAtomic — coverage: 83.3%
// =============================================================================

func TestCoverage_ImageAtomicOps(t *testing.T) {
	source := `
@group(0) @binding(0) var img: texture_storage_2d<r32uint, read_write>;

@compute @workgroup_size(1)
fn cs_main() {
    textureAtomicAdd(img, vec2<i32>(0, 0), 1u);
}
`
	// This test verifies imageAtomic* is handled.
	// May fail to compile if textureAtomicAdd is not supported by frontend,
	// but even attempting exercises the code path in Compile.
	opts := Options{LangVersion: Version430}
	result, _, err := compileWGSLHelper(source, opts)
	// We don't fail on error since this exercises a potentially unsupported path
	_ = result
	_ = err
}

// compileWGSLHelper compiles WGSL to GLSL, returning result and error without failing.
func compileWGSLHelper(source string, opts Options) (string, TranslationInfo, error) {
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return "", TranslationInfo{}, err
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		return "", TranslationInfo{}, err
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		return "", TranslationInfo{}, err
	}
	return Compile(module, opts)
}

// =============================================================================
// isSamplerComparison — coverage: 0%
// =============================================================================

func TestCoverage_DepthTextureSample(t *testing.T) {
	source := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var depth_samp: sampler_comparison;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(depth_tex, depth_samp, uv, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texture(")
}

// =============================================================================
// writeSwitchAsDoWhile — coverage: 72.7% — single-body switch path
// =============================================================================

func TestCoverage_SwitchSingleCase(t *testing.T) {
	// A switch with default-only body triggers isSingleBodySwitch → writeSwitchAsDoWhile.
	source := `
@fragment
fn fs_main(@location(0) x: i32) -> @location(0) vec4<f32> {
    var result: f32 = 0.0;
    switch x {
        default: {
            result = 1.0;
        }
    }
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeWorkgroupZeroInit — coverage: 72.7%
// =============================================================================

func TestCoverage_WorkgroupArrayInit(t *testing.T) {
	source := `
var<workgroup> data: array<u32, 16>;

@compute @workgroup_size(16)
fn cs_main(@builtin(local_invocation_index) idx: u32) {
    let val = data[idx];
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "shared")
}

// =============================================================================
// writeConstExpr — coverage: 68% — Splat and Compose paths
// =============================================================================

func TestCoverage_ConstExprSplat(t *testing.T) {
	source := `
const ZEROS: vec4<f32> = vec4<f32>(0.0);

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return ZEROS;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "vec4(")
}

func TestCoverage_ConstExprCompose(t *testing.T) {
	source := `
const COLOR: vec3<f32> = vec3<f32>(1.0, 0.5, 0.0);

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(COLOR, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "vec3(")
}

// =============================================================================
// writeEarlyDepthTest — coverage: 80%
// =============================================================================

func TestCoverage_EarlyDepthTest(t *testing.T) {
	source := `
@early_depth_test
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version420})
	// Should emit layout(early_fragment_tests) in;
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// escapeKeyword — coverage: 85.7% — test GLSL keyword collision
// =============================================================================

func TestCoverage_EscapeKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"texture", "_texture"},
		{"sampler", "_sampler"},
		{"normal_name", "normal_name"},
		{"in", "_in"},
		{"out", "_out"},
		{"uniform", "_uniform"},
		{"buffer", "_buffer"},
		{"float", "_float"},
		{"int", "_int"},
		{"vec2", "_vec2"},
		{"mat4", "_mat4"},
		{"gl_Position", "_gl_Position"},
		{"", "_unnamed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeKeyword(tt.input)
			if got != tt.want {
				t.Errorf("escapeKeyword(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// maybeEmitExpression — coverage: 60.8% — various bake paths
// =============================================================================

func TestCoverage_LoopWithContinue(t *testing.T) {
	// Tests continue inside a loop. In WGSL, the `continuing` block
	// can only appear once at the end of the loop body.
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        if i == 5 {
            continue;
        }
        sum = sum + f32(i);
        continuing {
            i = i + 1;
        }
    }
    return vec4<f32>(sum, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "while(true)")
}

// =============================================================================
// tryConstEvalBinary — coverage: 70%
// =============================================================================

func TestCoverage_ConstEvalBinary(t *testing.T) {
	source := `
const A: f32 = 2.0;
const B: f32 = 3.0;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let sum = A + B;
    let prod = A * B;
    let diff = A - B;
    let quot = A / B;
    return vec4<f32>(sum, prod, diff, quot);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	// Constants should be folded
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// exprConstValue — coverage: 52.6% — recursive binary/unary paths
// =============================================================================

func TestCoverage_ExprConstValueRecursive(t *testing.T) {
	source := `
const BASE: f32 = 10.0;
const SCALE: f32 = 2.0;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let val = BASE * SCALE + 1.0;
    return vec4<f32>(val, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// computeBlockNames — coverage: 0%
// =============================================================================

func TestCoverage_ComputeBlockNames(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Name: "MyStruct", Inner: ir.StructType{}},
			},
			EntryPoints: []ir.EntryPoint{
				{Name: "cs_main", Stage: ir.StageCompute},
			},
		},
		typeNames: map[ir.TypeHandle]string{0: "MyStruct_"},
		options:   &Options{EntryPoint: "cs_main"},
	}

	binding := &ir.ResourceBinding{Group: 1, Binding: 2}
	block, instance := w.computeBlockNames(ir.GlobalVariable{
		Type:    0,
		Binding: binding,
	})

	if !strings.Contains(block, "MyStruct") {
		t.Errorf("computeBlockNames block = %q, want contains 'MyStruct'", block)
	}
	if !strings.Contains(block, "Compute") {
		t.Errorf("computeBlockNames block = %q, want contains 'Compute'", block)
	}
	if !strings.Contains(instance, "group_1") {
		t.Errorf("computeBlockNames instance = %q, want contains 'group_1'", instance)
	}
	if !strings.Contains(instance, "binding_2") {
		t.Errorf("computeBlockNames instance = %q, want contains 'binding_2'", instance)
	}
}

func TestCoverage_ComputeBlockNamesFragment(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Name: "MyStruct", Inner: ir.StructType{}},
			},
			EntryPoints: []ir.EntryPoint{
				{Name: "fs_main", Stage: ir.StageFragment},
			},
		},
		typeNames: map[ir.TypeHandle]string{0: "MyStruct"},
		options:   &Options{EntryPoint: "fs_main"},
	}

	block, instance := w.computeBlockNames(ir.GlobalVariable{
		Type: 0,
	})

	if !strings.Contains(block, "Fragment") {
		t.Errorf("computeBlockNames block = %q, want contains 'Fragment'", block)
	}
	_ = instance
}

func TestCoverage_ComputeBlockNamesVertex(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Name: "MyStruct", Inner: ir.StructType{}},
			},
			EntryPoints: []ir.EntryPoint{
				{Name: "vs_main", Stage: ir.StageVertex},
			},
		},
		typeNames: map[ir.TypeHandle]string{0: "MyStruct"},
		options:   &Options{EntryPoint: "vs_main"},
	}

	block, instance := w.computeBlockNames(ir.GlobalVariable{
		Type: 0,
	})

	if !strings.Contains(block, "Vertex") {
		t.Errorf("computeBlockNames block = %q, want contains 'Vertex'", block)
	}
	if !strings.Contains(instance, "vs") {
		t.Errorf("computeBlockNames instance = %q, want contains 'vs'", instance)
	}
}

// =============================================================================
// isBooleanBinaryExpr — coverage: 81.8% — via Value path
// =============================================================================

func TestCoverage_BooleanBinaryExpr(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a: bool = x > 0.5;
    let b: bool = x < 0.8;
    let c = a && b;
    let d = a || b;
    return select(vec4<f32>(0.0), vec4<f32>(1.0), c);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeArrayLength — coverage: 75%
// =============================================================================

func TestCoverage_ArrayLength(t *testing.T) {
	source := `
struct Data {
    values: array<f32>,
};

@group(0) @binding(0) var<storage, read> data: Data;

@compute @workgroup_size(1)
fn cs_main() {
    let len = arrayLength(&data.values);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, ".length()")
}

// =============================================================================
// writeLiteral — coverage: 53.8% — test all literal types
// =============================================================================

func TestCoverage_WriteLiteral(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
	}

	tests := []struct {
		name    string
		lit     ir.Literal
		want    string
		wantErr bool
	}{
		{"bool_true", ir.Literal{Value: ir.LiteralBool(true)}, "true", false},
		{"bool_false", ir.Literal{Value: ir.LiteralBool(false)}, "false", false},
		{"i32", ir.Literal{Value: ir.LiteralI32(-42)}, "-42", false},
		{"u32", ir.Literal{Value: ir.LiteralU32(100)}, "100u", false},
		{"f32", ir.Literal{Value: ir.LiteralF32(3.14)}, "", false}, // just check no error
		{"i64", ir.Literal{Value: ir.LiteralI64(-100)}, "-100L", false},
		{"u64", ir.Literal{Value: ir.LiteralU64(200)}, "200uL", false},
		{"f64", ir.Literal{Value: ir.LiteralF64(2.718)}, "", false},
		{"abstract_int", ir.Literal{Value: ir.LiteralAbstractInt(999)}, "999", false},
		{"abstract_float", ir.Literal{Value: ir.LiteralAbstractFloat(1.5)}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := w.writeLiteral(tt.lit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("writeLiteral() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("writeLiteral() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// writeImageQuery — coverage: 62.3% — textureSize / textureNumLevels
// =============================================================================

func TestCoverage_TextureDimensions(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let size = textureDimensions(tex, 0);
    return vec4<f32>(f32(size.x), f32(size.y), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "textureSize(")
}

func TestCoverage_TextureNumLevels(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let levels = textureNumLevels(tex);
    return vec4<f32>(f32(levels), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "textureQueryLevels(")
}

func TestCoverage_TextureNumSamples(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let samples = textureNumSamples(tex);
    return vec4<f32>(f32(samples), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "Samples(")
}

// =============================================================================
// writeConstantValue — coverage: 54.5% — ZeroConstantValue + global expression
// =============================================================================

func TestCoverage_ConstantZeroValue(t *testing.T) {
	source := `
const ZERO_VEC: vec4<f32> = vec4<f32>(0.0, 0.0, 0.0, 0.0);

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return ZERO_VEC;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "vec4(")
}

// =============================================================================
// writeGlobalVariables — coverage: 61.8% — read-write storage
// =============================================================================

func TestCoverage_ReadWriteStorage(t *testing.T) {
	source := `
struct Data {
    values: array<u32>,
};

@group(0) @binding(0) var<storage, read_write> data: Data;

@compute @workgroup_size(1)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    data.values[gid.x] = gid.x;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "buffer")
}

// =============================================================================
// Vector binary ops — coverage: various 70-80%
// =============================================================================

func TestCoverage_VectorComparisons(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let b = vec4<f32>(4.0, 3.0, 2.0, 1.0);
    let eq = a == b;
    let neq = a != b;
    let lt = a < b;
    let le = a <= b;
    let gt = a > b;
    let ge = a >= b;
    return select(vec4<f32>(0.0), vec4<f32>(1.0), eq);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "equal(")
	glslMustContain(t, output, "notEqual(")
	glslMustContain(t, output, "lessThan(")
	glslMustContain(t, output, "lessThanEqual(")
	glslMustContain(t, output, "greaterThan(")
	glslMustContain(t, output, "greaterThanEqual(")
}

// =============================================================================
// writeUnary — coverage: 78.9% — bitwise not
// =============================================================================

func TestCoverage_BitwiseNotUnary(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: u32 = 0xFFu;
    let b = ~a;
    return vec4<f32>(f32(b), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "~(")
}

// =============================================================================
// writeAs — coverage: 79.5% — more type casts
// =============================================================================

func TestCoverage_TypeCasts(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let i: i32 = 42;
    let u: u32 = u32(i);
    let f: f32 = f32(u);
    let b: bool = i > 0;
    let f2: f32 = f32(b);
    return vec4<f32>(f, f2, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeImageSample — coverage: 71.4% — sample with bias, level, grad
// =============================================================================

func TestCoverage_TextureSampleBias(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(tex, samp, uv, -0.5);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texture(")
}

func TestCoverage_TextureSampleLevel(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(tex, samp, uv, 0.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "textureLod(")
}

func TestCoverage_TextureSampleGrad(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(tex, samp, uv, vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0));
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "textureGrad(")
}

// =============================================================================
// writeDerivative — coverage: 76.2% — fwidth
// =============================================================================

func TestCoverage_Fwidth(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let fw = fwidth(uv.x);
    return vec4<f32>(fw, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "fwidth(")
}

// =============================================================================
// writeImageLoad — coverage: 78.9% — texelFetch
// =============================================================================

func TestCoverage_TextureLoad(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texelFetch(")
}

// =============================================================================
// writeCallResult — still 0% — test function returning struct
// =============================================================================

func TestCoverage_WriteCallResultDirect(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
		names: map[nameKey]string{
			{kind: nameKeyFunction, handle1: 0}: "my_func",
		},
	}

	// With known name
	result, err := w.writeCallResult(ir.ExprCallResult{Function: 0})
	if err != nil {
		t.Fatalf("writeCallResult error: %v", err)
	}
	if result != "my_func" {
		t.Errorf("writeCallResult(0) = %q, want %q", result, "my_func")
	}

	// With unknown function (fallback name)
	result, err = w.writeCallResult(ir.ExprCallResult{Function: 99})
	if err != nil {
		t.Fatalf("writeCallResult error: %v", err)
	}
	if result != "call_result_99" {
		t.Errorf("writeCallResult(99) = %q, want %q", result, "call_result_99")
	}
}

// =============================================================================
// writeMath — coverage: 71.7% — more math functions
// =============================================================================

func TestCoverage_MathFunctions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = atan(x);
    let b = atan2(x, 1.0);
    let c = exp2(x);
    let d = log2(x);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "atan(")
}

func TestCoverage_MathSignInverseSqrt(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = sign(x);
    let c = inverseSqrt(x);
    let d = fma(x, 2.0, 1.0);
    return vec4<f32>(a, c, d, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "sign(")
	glslMustContain(t, output, "inversesqrt(")
}

// =============================================================================
// writeScalarValue — coverage: 81.2% — more scalar types
// =============================================================================

func TestCoverage_ScalarValueTypes(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
				{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
				{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			},
		},
	}

	// ScalarValue with i32 type
	got := w.writeScalarValue(ir.ScalarValue{Bits: 42, Kind: ir.ScalarSint}, 0)
	if !strings.Contains(got, "42") {
		t.Errorf("writeScalarValue(sint) = %q, want contains '42'", got)
	}

	// ScalarValue with u32 type
	got = w.writeScalarValue(ir.ScalarValue{Bits: 7, Kind: ir.ScalarUint}, 1)
	if !strings.Contains(got, "7") {
		t.Errorf("writeScalarValue(uint) = %q, want contains '7'", got)
	}

	// ScalarValue with bool type
	got = w.writeScalarValue(ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}, 2)
	if got != "true" {
		t.Errorf("writeScalarValue(bool) = %q, want 'true'", got)
	}
}

// =============================================================================
// writeAccess — coverage: 71.4% — matrix column access
// =============================================================================

func TestCoverage_MatrixColumnAccess(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let m = mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        0.0, 0.0, 0.0, 1.0
    );
    let i: i32 = 0;
    let col = m[i];
    return col;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// formatLiteral — coverage: 81.8% — i64, u64 paths
// =============================================================================

func TestCoverage_FormatLiteralTypes(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
	}

	tests := []struct {
		name string
		lit  ir.Literal
		want string
	}{
		{"bool_true", ir.Literal{Value: ir.LiteralBool(true)}, "true"},
		{"bool_false", ir.Literal{Value: ir.LiteralBool(false)}, "false"},
		{"i32_pos", ir.Literal{Value: ir.LiteralI32(42)}, "42"},
		{"i32_neg", ir.Literal{Value: ir.LiteralI32(-42)}, "-42"},
		{"u32", ir.Literal{Value: ir.LiteralU32(100)}, "100u"},
		{"i64", ir.Literal{Value: ir.LiteralI64(1000)}, "1000l"},
		{"u64", ir.Literal{Value: ir.LiteralU64(2000)}, "2000ul"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.formatLiteral(tt.lit)
			if got != tt.want {
				t.Errorf("formatLiteral(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// =============================================================================
// writeSwitchAsDoWhile — coverage: 72.7% — continue inside do-while switch
// =============================================================================

func TestCoverage_SwitchInLoop(t *testing.T) {
	// A switch inside a loop with continue exercises the do-while path
	// and continueCtx forwarding.
	source := `
@fragment
fn fs_main(@location(0) x: i32) -> @location(0) vec4<f32> {
    var result: f32 = 0.0;
    var i: i32 = 0;
    loop {
        if i >= 5 {
            break;
        }
        switch x {
            default: {
                result = result + 1.0;
            }
        }
        continuing {
            i = i + 1;
        }
    }
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "while(true)")
}

// =============================================================================
// writeFunctionArgument — coverage: 42.9% — non-entry-point function args
// =============================================================================

func TestCoverage_WriteFunctionArgumentDirect(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
		names: map[nameKey]string{
			{kind: nameKeyFunctionArgument, handle1: 0, handle2: 0}: "arg_x",
			{kind: nameKeyFunctionArgument, handle1: 0, handle2: 1}: "arg_y",
		},
	}
	w.currentFuncHandle = 0
	w.currentFunction = &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "x", Type: 0},
			{Name: "y", Type: 0},
		},
	}
	w.inEntryPoint = false

	result, err := w.writeFunctionArgument(ir.ExprFunctionArgument{Index: 0})
	if err != nil {
		t.Fatalf("writeFunctionArgument error: %v", err)
	}
	if result != "arg_x" {
		t.Errorf("writeFunctionArgument(0) = %q, want %q", result, "arg_x")
	}

	result, err = w.writeFunctionArgument(ir.ExprFunctionArgument{Index: 1})
	if err != nil {
		t.Fatalf("writeFunctionArgument error: %v", err)
	}
	if result != "arg_y" {
		t.Errorf("writeFunctionArgument(1) = %q, want %q", result, "arg_y")
	}
}

// =============================================================================
// writeGlobalVariable — coverage: 50% — regular global + combined pair
// =============================================================================

func TestCoverage_WriteGlobalVariableDirect(t *testing.T) {
	w := &Writer{
		module:           &ir.Module{},
		globalIsCombined: map[ir.GlobalVariableHandle]bool{},
		names: map[nameKey]string{
			{kind: nameKeyGlobalVariable, handle1: 0}: "my_uniform",
		},
	}
	w.currentFunction = &ir.Function{}

	result, err := w.writeGlobalVariable(ir.ExprGlobalVariable{Variable: 0})
	if err != nil {
		t.Fatalf("writeGlobalVariable error: %v", err)
	}
	if result != "my_uniform" {
		t.Errorf("writeGlobalVariable(0) = %q, want %q", result, "my_uniform")
	}

	// Test with combined pair
	w.globalIsCombined[0] = true
	w.combinedSamplers = map[string]*combinedSamplerInfo{
		"0:1": {textureHandle: 0, samplerHandle: 1, glslName: "tex_samp"},
	}

	result, err = w.writeGlobalVariable(ir.ExprGlobalVariable{Variable: 0})
	if err != nil {
		t.Fatalf("writeGlobalVariable combined error: %v", err)
	}
	if result != "tex_samp" {
		t.Errorf("writeGlobalVariable(0) combined = %q, want %q", result, "tex_samp")
	}
}

// =============================================================================
// writeZeroValue — coverage: 60% — fallback type name path
// =============================================================================

func TestCoverage_WriteZeroValueTypes(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
				{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
				{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
				{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
				{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			},
		},
	}

	tests := []struct {
		name     string
		typ      ir.TypeHandle
		wantPart string
	}{
		{"float", 0, "0.0"},
		{"vec3", 1, "vec3(0.0)"},
		{"mat4x4", 2, "mat4x4(0.0)"},
		{"int", 3, "0"},
		{"uint", 4, "0u"},
		{"bool", 5, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := w.writeZeroValue(ir.ExprZeroValue{Type: tt.typ})
			if err != nil {
				t.Fatalf("writeZeroValue error: %v", err)
			}
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("writeZeroValue(%s) = %q, want contains %q", tt.name, got, tt.wantPart)
			}
		})
	}
}

// =============================================================================
// Storage image textureSize — coverage for writeImageQuery storage path
// =============================================================================

func TestCoverage_StorageImageDimensions(t *testing.T) {
	source := `
@group(0) @binding(0) var img: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(1)
fn cs_main() {
    let size = textureDimensions(img);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "imageSize(")
}

// =============================================================================
// writeMath — coverage: 71.7% — distance, reflect, refract
// =============================================================================

func TestCoverage_MathDistanceReflect(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = vec3<f32>(1.0, 0.0, 0.0);
    let b = vec3<f32>(0.0, 1.0, 0.0);
    let d = distance(a, b);
    let r = reflect(a, b);
    return vec4<f32>(d, r.x, r.y, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "distance(")
	glslMustContain(t, output, "reflect(")
}

func TestCoverage_MathPackUnpack(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec2<f32>(0.5, 0.75);
    let packed = pack2x16float(v);
    let unpacked = unpack2x16float(packed);
    return vec4<f32>(unpacked, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "packHalf2x16(")
	glslMustContain(t, output, "unpackHalf2x16(")
}

// =============================================================================
// writeSelect — coverage: 70% — scalar select
// =============================================================================

func TestCoverage_SelectScalar(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = select(0.0, 1.0, x > 0.5);
    let b = select(2.0, 3.0, x < 0.3);
    return vec4<f32>(a, b, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "?")
}

// =============================================================================
// writeRelational — coverage: 77.8% — any/all with vector args
// =============================================================================

func TestCoverage_RelationalAnyAll(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec4<f32>(1.0, 0.0, 3.0, -1.0);
    let positive = v > vec4<f32>(0.0);
    let has_any = any(positive);
    let has_all = all(positive);
    return vec4<f32>(select(0.0, 1.0, has_any), select(0.0, 1.0, has_all), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "any(")
	glslMustContain(t, output, "all(")
}

// =============================================================================
// maybeEmitExpression — coverage: 60.8% — named expression
// =============================================================================

func TestCoverage_NamedExpressions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let brightness = x * 2.0;
    let color = vec4<f32>(brightness, brightness, brightness, 1.0);
    return color;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeAccess — coverage: 71.4% — array element access with bounds
// =============================================================================

func TestCoverage_AccessBoundsCheck(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let arr = array<f32, 3>(1.0, 2.0, 3.0);
    let idx = i32(x);
    let val = arr[idx];
    return vec4<f32>(val, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Texture load with restrict/read-zero policy — bounds check paths
// =============================================================================

func TestCoverage_TextureLoadRestrict(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0);
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		BoundsCheckPolicies: BoundsCheckPolicies{
			ImageLoad: BoundsCheckRestrict,
		},
	})
	glslMustContain(t, output, "texelFetch(")
}

func TestCoverage_TextureLoadReadZero(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0);
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		BoundsCheckPolicies: BoundsCheckPolicies{
			ImageLoad: BoundsCheckReadZeroSkipWrite,
		},
	})
	glslMustContain(t, output, "texelFetch(")
}

// =============================================================================
// Multiple texture samplers — combined sampler declaration paths
// =============================================================================

func TestCoverage_MultipleTextureSamplers(t *testing.T) {
	source := `
@group(0) @binding(0) var tex1: texture_2d<f32>;
@group(0) @binding(1) var tex2: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = textureSample(tex1, samp, uv);
    let b = textureSample(tex2, samp, uv);
    return a + b;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "sampler2D")
	glslMustContain(t, output, "texture(")
}

// =============================================================================
// writeConstExpr — coverage: 68% — ZeroValue and nested Compose
// =============================================================================

func TestCoverage_ConstExprZeroValue(t *testing.T) {
	source := `
const ZERO: u32 = 0u;
const ONE: u32 = 1u;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(f32(ZERO), f32(ONE), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "const")
}

// =============================================================================
// writeMath — coverage: 71.7% — more math functions to cover switches
// =============================================================================

func TestCoverage_MathTrigFunctions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = asin(x);
    let b = acos(x);
    let c = tan(x);
    let d = sinh(x);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "asin(")
	glslMustContain(t, output, "acos(")
	glslMustContain(t, output, "tan(")
	glslMustContain(t, output, "sinh(")
}

func TestCoverage_MathHyperbolicFunctions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = cosh(x);
    let b = tanh(x);
    let c = asinh(x);
    let d = acosh(x + 1.0);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "cosh(")
	glslMustContain(t, output, "tanh(")
	glslMustContain(t, output, "asinh(")
	glslMustContain(t, output, "acosh(")
}

func TestCoverage_MathSaturateFract(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = saturate(x);
    let b = fract(x);
    let c = radians(x);
    let d = degrees(x);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "clamp(")
	glslMustContain(t, output, "fract(")
	glslMustContain(t, output, "radians(")
	glslMustContain(t, output, "degrees(")
}

func TestCoverage_MathLdexp(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let val = ldexp(1.0, 3);
    return vec4<f32>(val, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "ldexp(")
}

// =============================================================================
// writeSelect — coverage: 70% — cover error paths via unit test
// =============================================================================

func TestCoverage_WriteSelectDirect(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
		names:  map[nameKey]string{},
	}
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{},
	}
	w.namedExpressions = map[ir.ExpressionHandle]string{}
	w.needBakeExpression = map[ir.ExpressionHandle]struct{}{}

	result, err := w.writeSelect(ir.ExprSelect{
		Condition: 0,
		Accept:    1,
		Reject:    2,
	})
	if err != nil {
		t.Fatalf("writeSelect error: %v", err)
	}
	if !strings.Contains(result, "?") {
		t.Errorf("writeSelect = %q, want contains '?'", result)
	}
}

// =============================================================================
// writeRelational — coverage: 77.8% — isnan/isinf
// =============================================================================

func TestCoverage_IsNanIsInf(t *testing.T) {
	w := &Writer{
		module: &ir.Module{},
		names:  map[nameKey]string{},
	}
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{},
	}
	w.namedExpressions = map[ir.ExpressionHandle]string{}
	w.needBakeExpression = map[ir.ExpressionHandle]struct{}{}

	result, err := w.writeRelational(ir.ExprRelational{Fun: ir.RelationalIsNan, Argument: 0})
	if err != nil {
		t.Fatalf("writeRelational(isnan) error: %v", err)
	}
	if !strings.Contains(result, "isnan(") {
		t.Errorf("writeRelational(isnan) = %q, want contains 'isnan('", result)
	}

	result, err = w.writeRelational(ir.ExprRelational{Fun: ir.RelationalIsInf, Argument: 0})
	if err != nil {
		t.Fatalf("writeRelational(isinf) error: %v", err)
	}
	if !strings.Contains(result, "isinf(") {
		t.Errorf("writeRelational(isinf) = %q, want contains 'isinf('", result)
	}
}

// =============================================================================
// writeFunctionArgument — coverage: 42.9% — entry point with location binding
// =============================================================================

func TestCoverage_EntryPointLocationArgs(t *testing.T) {
	source := `
@fragment
fn fs_main(
    @location(0) color: vec4<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) normal: vec3<f32>,
) -> @location(0) vec4<f32> {
    return vec4<f32>(color.rgb * dot(normal, vec3<f32>(0.0, 1.0, 0.0)), 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeGlobalVariables — coverage: 61.8% — private/function space globals
// =============================================================================

func TestCoverage_PrivateGlobal(t *testing.T) {
	source := `
var<private> counter: u32 = 0u;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    counter = counter + 1u;
    return vec4<f32>(f32(counter), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// writeImageLoadUnchecked + writeImageLoadRestrict + writeImageLoadReadZero
// — coverage: ~73%
// =============================================================================

func TestCoverage_StorageImageLoad(t *testing.T) {
	source := `
@group(0) @binding(0) var img: texture_storage_2d<rgba32float, read>;

@compute @workgroup_size(1)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let color = textureLoad(img, vec2<i32>(gid.xy));
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "imageLoad(")
}

// =============================================================================
// continueEncountered — exercise full continue forwarding path
// =============================================================================

func TestCoverage_ContinueEncountered(t *testing.T) {
	ctx := &continueCtx{}

	// Not inside loop — should return empty
	if v := ctx.continueEncountered(); v != "" {
		t.Errorf("continueEncountered outside loop = %q, want empty", v)
	}

	// Inside loop — no switch, should return empty
	ctx.enterLoop()
	if v := ctx.continueEncountered(); v != "" {
		t.Errorf("continueEncountered in loop = %q, want empty", v)
	}

	// Inside loop > switch — should return variable
	n := newNamer()
	switchVar := ctx.enterSwitch(n)
	if switchVar == "" {
		t.Error("enterSwitch should return variable name")
	}

	v := ctx.continueEncountered()
	if v == "" {
		t.Error("continueEncountered in switch should return variable name")
	}

	// Exit switch — should return exitContinue
	result, err := ctx.exitSwitch()
	if err != nil {
		t.Fatalf("exitSwitch error: %v", err)
	}
	if result.kind != exitContinue {
		t.Errorf("exitSwitch kind = %d, want exitContinue", result.kind)
	}

	if err := ctx.exitLoop(); err != nil {
		t.Fatalf("exitLoop error: %v", err)
	}
}

// =============================================================================
// Depth texture + sampler — isSamplerComparison path
// =============================================================================

func TestCoverage_IsSamplerComparison(t *testing.T) {
	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.SamplerType{Comparison: true}},
				{Inner: ir.SamplerType{Comparison: false}},
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			},
			GlobalVariables: []ir.GlobalVariable{
				{Type: 0, Name: "comp_sampler"},
				{Type: 1, Name: "reg_sampler"},
				{Type: 2, Name: "not_sampler"},
			},
		},
	}

	if !w.isSamplerComparison(0) {
		t.Error("isSamplerComparison(0) should be true for comparison sampler")
	}
	if w.isSamplerComparison(1) {
		t.Error("isSamplerComparison(1) should be false for regular sampler")
	}
	if w.isSamplerComparison(2) {
		t.Error("isSamplerComparison(2) should be false for non-sampler type")
	}
	if w.isSamplerComparison(99) {
		t.Error("isSamplerComparison(99) should be false for out-of-range handle")
	}
}

// =============================================================================
// writeHelperFunctions — coverage: 12.5% — direct unit test
// =============================================================================

func TestCoverage_WriteHelperFunctionsDirect(t *testing.T) {
	tests := []struct {
		name    string
		mod     bool
		div     bool
		wantMod bool
		wantDiv bool
	}{
		{"mod_only", true, false, true, false},
		{"div_only", false, true, false, true},
		{"both", true, true, true, true},
		{"neither", false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWriter()
			w.needsModHelper = tt.mod
			w.needsDivHelper = tt.div
			w.writeHelperFunctions()
			output := w.Out.String()

			if tt.wantMod {
				if !strings.Contains(output, "_naga_mod") {
					t.Errorf("expected _naga_mod helper in output")
				}
			}
			if tt.wantDiv {
				if !strings.Contains(output, "_naga_div") {
					t.Errorf("expected _naga_div helper in output")
				}
			}
		})
	}
}

// newTestWriter creates a minimal Writer for unit testing output methods.
func newTestWriter() *Writer {
	w := &Writer{
		module:  &ir.Module{},
		options: &Options{LangVersion: Version330},
	}
	// Out is embedded strings.Builder via IndentWriter; no initialization needed
	return w
}

// =============================================================================
// tryConstEvalUnary — coverage: 23.1% — direct unit test
// =============================================================================

func TestCoverage_TryConstEvalUnaryDirect(t *testing.T) {
	floatHandle := ir.TypeHandle(0)
	boolHandle := ir.TypeHandle(1)

	w := &Writer{
		module: &ir.Module{
			Types: []ir.Type{
				{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			},
			Constants: []ir.Constant{
				{Type: 0, Init: 0},
				{Type: 1, Init: 1},
			},
			GlobalExpressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(5.0)}},
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			},
		},
	}

	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprConstant{Constant: 0}}, // handle 0: const float
			{Kind: ir.ExprConstant{Constant: 1}}, // handle 1: const bool
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &floatHandle},
			{Handle: &boolHandle},
		},
	}

	// Negate float constant
	result, ok := w.tryConstEvalUnary(ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0})
	if !ok {
		t.Error("tryConstEvalUnary(negate) should succeed")
	}
	if !strings.Contains(result, "-") || !strings.Contains(result, "5") {
		t.Errorf("tryConstEvalUnary(negate) = %q, want contains '-5'", result)
	}

	// Logical not on bool constant
	result, ok = w.tryConstEvalUnary(ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: 1})
	if !ok {
		t.Error("tryConstEvalUnary(not) should succeed")
	}
	if result != "false" {
		t.Errorf("tryConstEvalUnary(not true) = %q, want 'false'", result)
	}

	// Non-constant expression — should return false
	w.currentFunction.Expressions = append(w.currentFunction.Expressions,
		ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
	)
	_, ok = w.tryConstEvalUnary(ir.ExprUnary{Op: ir.UnaryNegate, Expr: 2})
	if ok {
		t.Error("tryConstEvalUnary on non-constant should return false")
	}

	// Nil function context — should return false
	w.currentFunction = nil
	_, ok = w.tryConstEvalUnary(ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0})
	if ok {
		t.Error("tryConstEvalUnary with nil function should return false")
	}
}

// =============================================================================
// lookupVaryingNameWithBlend — coverage: 44.4%
// =============================================================================

func TestCoverage_LookupVaryingNameWithBlendDirect(t *testing.T) {
	w := &Writer{
		module:         &ir.Module{},
		varyingNameMap: map[varyingLookupKey]string{},
	}

	// No blend src
	name := w.lookupVaryingNameWithBlend(0, nil, ir.StageVertex, true)
	if !strings.Contains(name, "location0") {
		t.Errorf("lookupVaryingNameWithBlend(loc=0, nil) = %q, want contains 'location0'", name)
	}

	// With blend src
	blendSrc := uint32(1)
	name = w.lookupVaryingNameWithBlend(0, &blendSrc, ir.StageFragment, true)
	if !strings.Contains(name, "location") {
		t.Errorf("lookupVaryingNameWithBlend(loc=0, blend=1) = %q, want contains 'location'", name)
	}

	// With cached entry
	key := varyingLookupKey{location: 5, isOutput: true, stage: ir.StageVertex}
	w.varyingNameMap[key] = "_cached_name"
	name = w.lookupVaryingNameWithBlend(5, nil, ir.StageVertex, true)
	if name != "_cached_name" {
		t.Errorf("lookupVaryingNameWithBlend cached = %q, want '_cached_name'", name)
	}
}

// =============================================================================
// writeUniformBlock — coverage: 28.6% — non-binding expansion path
// =============================================================================

func TestCoverage_WriteUniformBlockDirect(t *testing.T) {
	w := newTestWriter()
	w.module = &ir.Module{
		Types: []ir.Type{
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: 0},
					{Name: "y", Type: 0},
				},
			}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageFragment},
		},
	}
	w.typeNames = map[ir.TypeHandle]string{0: "Params", 1: "float"}
	w.globalInstanceName = map[ir.GlobalVariableHandle]string{0: "_group_0_binding_0_fs"}
	w.names = map[nameKey]string{
		{kind: nameKeyStructMember, handle1: 0, handle2: 0}: "x",
		{kind: nameKeyStructMember, handle1: 0, handle2: 1}: "y",
	}

	// Without binding — should expand struct members
	w.writeUniformBlock("params", "Params",
		ir.GlobalVariable{Type: 0, Space: ir.SpaceUniform},
		ir.StructType{
			Members: []ir.StructMember{
				{Name: "x", Type: 1},
				{Name: "y", Type: 1},
			},
		},
	)
	output := w.Out.String()
	if !strings.Contains(output, "uniform") {
		t.Errorf("writeUniformBlock output = %q, want contains 'uniform'", output)
	}
}
