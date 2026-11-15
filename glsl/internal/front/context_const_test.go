package front

// Contracts of the lowering context that the rest of the frontend builds on:
// Rust naga's Context::lift_up_const_expression (front/glsl/types.rs:335-371),
// the module-scope initializer rule of declarations.rs:221-253, the
// const-expression arena (consteval.Arena/Types as context.rs implements them)
// and the const-name resolution of Context::variable (context.rs:1024-1082).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/ir"
)

// appendRawExpr appends an expression without the constant evaluator, so the
// arena holds exactly the shape the test asks for.
func appendRawExpr(c *Context, kind ir.ExpressionKind) ir.ExpressionHandle {
	return c.appendExpr(kind)
}

// TestLiftUpConstExpressionLiftsScalars is variables.rs:335-352: a literal, a
// constant and a zero value each move into the module's global arena and come
// back as a global handle.
func TestLiftUpConstExpressionLiftsScalars(t *testing.T) {
	fe, c := newTestContext(t)
	uintTy := fe.registerType(u32Scalar)
	lit := appendRawExpr(c, ir.Literal{Value: ir.LiteralU32(7)})
	constant := appendRawExpr(c, ir.ExprConstant{Constant: 0})
	zero := appendRawExpr(c, ir.ExprZeroValue{Type: uintTy})

	tests := []struct {
		name string
		expr ir.ExpressionHandle
	}{
		{"literal", lit},
		{"constant", constant},
		{"zero value", zero},
	}
	before := len(fe.mod.GlobalExpressions)
	for i, tt := range tests {
		lifted, err := c.liftUpConstExpression(tt.expr, ir.Span{})
		if err != nil {
			t.Fatalf("%s: liftUpConstExpression: %v", tt.name, err)
		}
		want := ir.ExpressionHandle(before + i)
		if lifted != want {
			t.Fatalf("%s: lifted handle = %d, want the appended global %d", tt.name, lifted, want)
		}
	}
	if len(fe.mod.GlobalExpressions) != before+len(tests) {
		t.Fatalf("the module holds %d global expressions, want %d", len(fe.mod.GlobalExpressions), before+len(tests))
	}
}

// TestLiftUpConstExpressionLiftsComposites is the recursive arm of
// types.rs:352-370: a compose or splat is rebuilt in the global arena from its
// lifted operands.
func TestLiftUpConstExpressionLiftsComposites(t *testing.T) {
	fe, c := newTestContext(t)
	vec2 := fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar})
	first := appendRawExpr(c, ir.Literal{Value: ir.LiteralF32(1)})
	second := appendRawExpr(c, ir.Literal{Value: ir.LiteralF32(2)})
	compose := appendRawExpr(c, ir.ExprCompose{Type: vec2, Components: []ir.ExpressionHandle{first, second}})

	before := len(fe.mod.GlobalExpressions)
	lifted, err := c.liftUpConstExpression(compose, ir.Span{})
	if err != nil {
		t.Fatalf("liftUpConstExpression: %v", err)
	}
	// The two components are lifted first, then the compose.
	if int(lifted) != before+2 {
		t.Fatalf("the lifted compose is at %d, want the third appended expression %d", lifted, before+2)
	}
	global, ok := fe.mod.GlobalExpressions[lifted].Kind.(ir.ExprCompose)
	if !ok {
		t.Fatalf("the lifted expression is %T, want a compose", fe.mod.GlobalExpressions[lifted].Kind)
	}
	for i, component := range global.Components {
		if int(component) != before+i {
			t.Fatalf("component %d points at %d, want the lifted operand %d", i, component, before+i)
		}
	}

	splat := appendRawExpr(c, ir.ExprSplat{Size: ir.Vec2, Value: first})
	liftedSplat, err := c.liftUpConstExpression(splat, ir.Span{})
	if err != nil {
		t.Fatalf("liftUpConstExpression(splat): %v", err)
	}
	splatKind, ok := fe.mod.GlobalExpressions[liftedSplat].Kind.(ir.ExprSplat)
	if !ok || splatKind.Size != ir.Vec2 {
		t.Fatalf("the lifted splat is %#v, want a vec2 splat", fe.mod.GlobalExpressions[liftedSplat].Kind)
	}
}

// TestLiftUpConstExpressionRejectsRuntime is types.rs:369-371: anything that is
// not a const-expression is refused with Rust's message, and the caller is
// expected to ignore the error.
func TestLiftUpConstExpressionRejectsRuntime(t *testing.T) {
	_, c := newTestContext(t)
	lit := appendRawExpr(c, ir.Literal{Value: ir.LiteralF32(1)})
	load := appendRawExpr(c, ir.ExprLoad{Pointer: lit})
	_, err := c.liftUpConstExpression(load, ir.Span{})
	if err == nil || err.Message != "Expression is not const-expression" {
		t.Fatalf("error = %v, want Rust's not-a-const-expression message", err)
	}
}

// TestModuleScopeInitializerIsLifted is declarations.rs:250-253: a module-scope
// initializer that is a const-expression becomes the global's InitExpr.
func TestModuleScopeInitializerIsLifted(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
vec4 tint = vec4(1.0);
void main() { o = tint; }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tint := findGlobal(t, mod, "tint")
	if tint.InitExpr == nil {
		t.Fatal("tint has no InitExpr, want the lifted constant initializer")
	}
	if int(*tint.InitExpr) >= len(mod.GlobalExpressions) {
		t.Fatalf("InitExpr %d is outside the %d global expressions", *tint.InitExpr, len(mod.GlobalExpressions))
	}
	if _, isLiteral := mod.GlobalExpressions[*tint.InitExpr].Kind.(ir.Literal); isLiteral {
		t.Fatalf("the initializer lifted to a bare literal; a vec4 constructor is expected to reach the global arena")
	}
}

// TestModuleScopeRuntimeInitializerIsDropped is the other half of
// declarations.rs:250-253: a runtime expression is silently dropped and the
// global keeps no initializer.
func TestModuleScopeRuntimeInitializerIsDropped(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float v; } u;
float c = u.v;
layout(location = 0) out vec4 o;
void main() { o = vec4(c); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c := findGlobal(t, mod, "c"); c.InitExpr != nil {
		t.Fatalf("c has InitExpr %d, want the runtime initializer dropped", *c.InitExpr)
	}
}

// TestContextTypeAndArray is the consteval.Types contract (context.go:427-446):
// a type is registered anonymously and deduplicated, and an array's stride is
// the base's size rounded up to its alignment.
func TestContextTypeAndArray(t *testing.T) {
	fe, c := newTestContext(t)
	vec3 := c.Type(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar})
	if again := c.Type(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar}); again != vec3 {
		t.Fatalf("Type registered the same inner twice: %d then %d", vec3, again)
	}
	if got := fe.mod.Types[vec3].Inner.(ir.VectorType); got.Size != ir.Vec3 {
		t.Fatalf("registered type = %v, want vec3", got)
	}

	size := uint32(3)
	array := c.Array(vec3, ir.ArraySize{Constant: &size})
	inner, ok := fe.mod.Types[array].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("Array registered %T, want an array", fe.mod.Types[array].Inner)
	}
	// A vec3<f32> is 12 bytes with alignment 16, so the stride is 16.
	if inner.Stride != 16 {
		t.Fatalf("array stride = %d, want the padded vec3 stride 16", inner.Stride)
	}
	if inner.Base != vec3 || inner.Size.Constant == nil || *inner.Size.Constant != 3 {
		t.Fatalf("array shape = %+v, want base vec3 and size 3", inner)
	}
	if got := c.Array(vec3, ir.ArraySize{Constant: &size}); got != array {
		t.Fatalf("Array registered the same array twice: %d then %d", array, got)
	}
}

// TestConstArrayConstructorStrides is the array type a const array constructor
// produces: the elements' stride comes from the base type, and the module
// carries one constant whose initializer is the constructor.
func TestConstArrayConstructorStrides(t *testing.T) {
	source := `#version 450
const int a[3] = int[3](1, 2, 3);
layout(location = 0) out vec4 o;
void main() { o = vec4(float(a[0]), 0.0, 0.0, 1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(mod.Constants) != 1 {
		t.Fatalf("module has %d constants, want the one array constant", len(mod.Constants))
	}
	array, ok := mod.Types[mod.Constants[0].Type].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("the constant's type is %T, want an array", mod.Types[mod.Constants[0].Type].Inner)
	}
	if array.Stride != 4 {
		t.Fatalf("array stride = %d, want the int32 stride 4", array.Stride)
	}
	if array.Size.Constant == nil || *array.Size.Constant != 3 {
		t.Fatalf("array size = %+v, want 3", array.Size)
	}
}

// TestConstNameResolvesInConstContext is Context::variable's const arm
// (context.rs:1024-1082) reached through a module-scope array size: the name is
// materialized on first use and the size evaluates from the constant.
func TestConstNameResolvesInConstContext(t *testing.T) {
	source := `#version 450
const int N = 4;
layout(location = 0) out vec4 o;
void main() {
    float a[N];
    a[0] = 1.0;
    o = vec4(a[0]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	found := false
	for _, local := range main.LocalVars {
		array, ok := mod.Types[local.Type].Inner.(ir.ArrayType)
		if !ok {
			continue
		}
		if array.Size.Constant == nil || *array.Size.Constant != 4 {
			t.Fatalf("array size = %+v, want the constant's 4", array.Size)
		}
		found = true
	}
	if !found {
		t.Fatal("main has no array local")
	}
}

// TestOverrideArraySizeIsRefused pins how a pipeline-overridable constant in a
// constant expression is reported: it cannot be a uint constant. Rust's
// parse_uint_constant evaluates the expression and reports "Expected a uint
// constant" for anything that is not a literal (parser.rs:203-229); our const
// evaluator stops one step earlier with its own override error, which the
// frontend renders as Rust's message for a const-evaluator error.
func TestOverrideArraySizeIsRefused(t *testing.T) {
	source := `#version 450
layout(constant_id = 0) const int K = 2;
float a[K];
void main() {}
`
	_, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("Parse accepted an override as an array size")
	}
	if !strings.Contains(err.Error(), "override-expression") && !strings.Contains(err.Error(), "Expected a uint constant") {
		t.Fatalf("error = %v, want the const evaluator's override refusal", err)
	}
}

// TestConstArrayDynamicIndexUsesProxy is context.rs:1037-1061: a const name has
// no pointer to index dynamically, so the access goes through a local
// initialized with the constant.
func TestConstArrayDynamicIndexUsesProxy(t *testing.T) {
	source := `#version 450
const int a[2] = int[2](1, 2);
layout(location = 0) out vec4 o;
void main() {
    int i = 1;
    o = vec4(float(a[i]));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	// The proxy local is initialized with the constant.
	initialized := false
	for _, local := range main.LocalVars {
		if local.Init != nil {
			if _, ok := exprKindOf(t, main, *local.Init).(ir.ExprConstant); ok {
				initialized = true
			}
		}
	}
	if !initialized {
		t.Fatalf("no local is initialized with the constant: %+v", main.LocalVars)
	}
}

// TestConstantErrorText is context.go:489-502: the two structural evaluator
// errors carry Rust's spelling, everything else its own message.
func TestConstantErrorText(t *testing.T) {
	notConst := &consteval.Error{Class: consteval.ErrNotConst, Msg: "Subexpression(s) are not constant"}
	if got := constantErrorText(notConst); got != "Unexpected runtime-expression" {
		t.Fatalf("not-const text = %q", got)
	}
	override := &consteval.Error{Class: consteval.ErrOverrideInConst, Msg: "override"}
	if got := constantErrorText(override); got != "Unexpected override-expression" {
		t.Fatalf("override text = %q", got)
	}
	other := &consteval.Error{Class: consteval.ErrOverflow, Msg: "other"}
	if got := constantErrorText(other); got != "other" {
		t.Fatalf("other text = %q", got)
	}
}

// TestVectorSizeCount is context.go:812-823.
func TestVectorSizeCount(t *testing.T) {
	tests := []struct {
		size ir.VectorSize
		want int
	}{
		{ir.Vec2, 2},
		{ir.Vec3, 3},
		{ir.Vec4, 4},
		{ir.VectorSize(0), 0},
		{ir.VectorSize(9), 0},
	}
	for _, tt := range tests {
		if got := vectorSizeCount(tt.size); got != tt.want {
			t.Errorf("vectorSizeCount(%v) = %d, want %d", tt.size, got, tt.want)
		}
	}
}
