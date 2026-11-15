package front

// Contracts of GLSL constructors, calls and function declarations, Rust
// naga's front/glsl/functions.rs: constructor_single (61-208), matrix_one_arg
// (211-356), vector_constructor (358-412), constructor_many (414-514),
// function_call (515-868), process_lhs_argument (873-1026), add_function
// (1028-1117) and add_prototype (1119-1186).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// functionNamed returns the module function of a name.
func functionNamed(t *testing.T, mod *ir.Module, name string) ir.Function {
	t.Helper()
	for _, function := range mod.Functions {
		if function.Name == name {
			return function
		}
	}
	t.Fatalf("no module function named %q", name)
	return ir.Function{}
}

// firstCallOf returns the first Statement::Call of a block.
func firstCallOf(t *testing.T, block ir.Block) ir.StmtCall {
	t.Helper()
	for _, stmt := range block {
		if call, ok := stmt.Kind.(ir.StmtCall); ok {
			return call
		}
	}
	t.Fatalf("block has no call: %+v", block)
	return ir.StmtCall{}
}

// composeOfType returns the first ExprCompose whose type inner is want.
func composeOfType(t *testing.T, mod *ir.Module, function ir.Function, want ir.TypeInner) (ir.ExprCompose, ir.ExpressionHandle) {
	t.Helper()
	for h, expr := range function.Expressions {
		compose, ok := expr.Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		if typeInnerEqual(mod.Types[compose.Type].Inner, want) {
			return compose, ir.ExpressionHandle(h)
		}
	}
	t.Fatalf("no compose of type %v in function %q", want, function.Name)
	return ir.ExprCompose{}, 0
}

// TestConstructorMatrixFromScalarFillsDiagonal is matrix_one_arg's scalar arm
// (functions.rs:216-247): a scalar fills the diagonal, every other component is
// zero.
func TestConstructorMatrixFromScalarFillsDiagonal(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
void main() {
    mat2 m = mat2(u.f);
    o = vec4(m[0], m[1]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	matrix, _ := composeOfType(t, mod, main, ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	if len(matrix.Components) != 2 {
		t.Fatalf("the matrix has %d columns, want 2", len(matrix.Components))
	}
	col0 := exprKindOf(t, main, matrix.Components[0]).(ir.ExprCompose)
	col1 := exprKindOf(t, main, matrix.Components[1]).(ir.ExprCompose)
	if col0.Components[0] != col1.Components[1] {
		t.Fatalf("the diagonal components %d and %d differ, want the same scalar", col0.Components[0], col1.Components[1])
	}
	if col0.Components[1] != col1.Components[0] {
		t.Fatalf("the off-diagonal components %d and %d differ, want the same zero", col0.Components[1], col1.Components[0])
	}
	if zero, ok := exprKindOf(t, main, col0.Components[1]).(ir.Literal); !ok || zero.Value != ir.LiteralF32(0) {
		t.Fatalf("the off-diagonal component is %#v, want the zero literal", exprKindOf(t, main, col0.Components[1]))
	}
}

// TestConstructorMatrixPadsIdentity is matrix_one_arg's matrix arm
// (functions.rs:249-355): a smaller matrix is copied into the top-left corner
// and the rest of the identity is filled in.
func TestConstructorMatrixPadsIdentity(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
void main() {
    mat2 m2 = mat2(u.f);
    mat3 m = mat3(m2);
    o = vec4(m[0], m[1]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	matrix, _ := composeOfType(t, mod, main, ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32Scalar})
	if len(matrix.Components) != 3 {
		t.Fatalf("the matrix has %d columns, want 3", len(matrix.Components))
	}
	third, ok := exprKindOf(t, main, matrix.Components[2]).(ir.ExprCompose)
	if !ok {
		t.Fatalf("the padded column is %#v, want a compose", exprKindOf(t, main, matrix.Components[2]))
	}
	one, ok := exprKindOf(t, main, third.Components[2]).(ir.Literal)
	if !ok || one.Value != ir.LiteralF32(1) {
		t.Fatalf("the identity's diagonal component is %#v, want the one literal", exprKindOf(t, main, third.Components[2]))
	}
}

// TestConstructorMatrixTrimsRows is matrix_one_arg's shrinking arm
// (functions.rs:284-293): a matrix with more rows is trimmed by a vector
// swizzle per column.
func TestConstructorMatrixTrimsRows(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
void main() {
    mat3 m3 = mat3(u.f);
    mat2 m = mat2(m3);
    o = vec4(m[0], m[1]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	matrix, _ := composeOfType(t, mod, main, ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	for i, column := range matrix.Components {
		swizzle, ok := exprKindOf(t, main, column).(ir.ExprSwizzle)
		if !ok || swizzle.Size != ir.Vec2 {
			t.Fatalf("column %d is %#v, want the vec2 swizzle that trims the rows", i, exprKindOf(t, main, column))
		}
	}
}

// TestConstructorVectorFromMatrixFlattens is vector_constructor's matrix arm
// (functions.rs:390-409): the matrix's columns flatten into the vector's lanes
// and anything past them is dropped.
func TestConstructorVectorFromMatrixFlattens(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
void main() {
    mat2 m2 = mat2(u.f);
    vec4 v = vec4(m2);
    o = v;
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	vector, _ := composeOfType(t, mod, main, ir.VectorType{Size: ir.Vec4, Scalar: f32Scalar})
	if len(vector.Components) != 4 {
		t.Fatalf("the vector has %d components, want the matrix's 4 lanes", len(vector.Components))
	}
}

// TestConstructorVectorTruncates is constructor_single's vector arm
// (functions.rs:120-127): a wider vector is swizzled down to the target size
// and then converted.
func TestConstructorVectorTruncates(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { vec4 v; } u;
layout(location = 0) out vec4 o;
void main() {
    vec2 c = vec2(u.v);
    o = vec4(c, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	swizzles := 0
	for _, expr := range main.Expressions {
		if swizzle, ok := expr.Kind.(ir.ExprSwizzle); ok && swizzle.Size == ir.Vec2 {
			swizzles++
		}
	}
	if swizzles == 0 {
		t.Fatal("no vec2 swizzle: a vec4 argument to vec2 must be truncated")
	}
}

// TestConstructorVectorFromScalarSplats is constructor_single's splat arm
// (functions.rs:107-118): a scalar fills every lane.
func TestConstructorVectorFromScalarSplats(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
void main() {
    vec3 v = vec3(u.f);
    o = vec4(v, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	for _, expr := range main.Expressions {
		if splat, ok := expr.Kind.(ir.ExprSplat); ok && splat.Size == ir.Vec3 {
			return
		}
	}
	t.Fatal("no vec3 splat: a scalar constructor argument must splat")
}

// TestConstructorBoolToScalarIsSelect is the bool-cast arm of
// constructor_single (functions.rs:74-100): the IR's As cannot express
// bool-to-number, so the lowering selects between the zero and the one.
func TestConstructorBoolToScalarIsSelect(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float x; } u;
layout(location = 0) out vec4 o;
void main() {
    bool b = u.x > 0.0;
    float f = float(b);
    o = vec4(f);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	for _, expr := range main.Expressions {
		selectExpr, ok := expr.Kind.(ir.ExprSelect)
		if !ok {
			continue
		}
		accept, ok := exprKindOf(t, main, selectExpr.Accept).(ir.Literal)
		if !ok || accept.Value != ir.LiteralF32(1) {
			t.Fatalf("select accept = %#v, want the one literal", exprKindOf(t, main, selectExpr.Accept))
		}
		reject, ok := exprKindOf(t, main, selectExpr.Reject).(ir.Literal)
		if !ok || reject.Value != ir.LiteralF32(0) {
			t.Fatalf("select reject = %#v, want the zero literal", exprKindOf(t, main, selectExpr.Reject))
		}
		return
	}
	t.Fatal("no select: a bool constructor argument must select between 0 and 1")
}

// TestConstructorBoolToVectorSplatsTheSelect is the same rule for a vector
// target (functions.rs:91-97): both select arms are splatted to the vector's
// size.
func TestConstructorBoolToVectorSplatsTheSelect(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float x; } u;
layout(location = 0) out vec4 o;
void main() {
    bool b = u.x > 0.0;
    bvec2 bv = bvec2(b);
    vec2 v = vec2(bv);
    o = vec4(v, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	for _, expr := range main.Expressions {
		selectExpr, ok := expr.Kind.(ir.ExprSelect)
		if !ok {
			continue
		}
		for _, arm := range []ir.ExpressionHandle{selectExpr.Accept, selectExpr.Reject} {
			splat, ok := exprKindOf(t, main, arm).(ir.ExprSplat)
			if !ok || splat.Size != ir.Vec2 {
				t.Fatalf("select arm = %#v, want a vec2 splat", exprKindOf(t, main, arm))
			}
		}
		return
	}
	t.Fatal("no select for the vector bool constructor")
}

// TestConstructorStructOfOneArgument is constructor_single's struct arm
// (functions.rs:159-168): the argument is converted to the member's scalar and
// composed.
func TestConstructorStructOfOneArgument(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
struct S { float x; };
void main() {
    S s = S(u.f);
    o = vec4(s.x);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	compose, _ := composeOfType(t, mod, main, ir.StructType{
		Members: []ir.StructMember{{Name: "x", Type: findTypeHandle(t, mod, f32Scalar)}},
		Span:    4,
	})
	if len(compose.Components) != 1 {
		t.Fatalf("the struct compose has %d components, want the one argument", len(compose.Components))
	}
}

// TestConstructorArrayConvertsElements is constructor_many's array arm
// (functions.rs:474-485): every element is converted to the array's base type,
// so an int argument to a float array becomes an f32 literal.
func TestConstructorArrayConvertsElements(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    float a[2] = float[2](1, 2);
    o = vec4(a[0], a[1], 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	var arrayCompose *ir.ExprCompose
	for _, expr := range main.Expressions {
		compose, ok := expr.Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		if _, isArray := mod.Types[compose.Type].Inner.(ir.ArrayType); isArray {
			c := compose
			arrayCompose = &c
		}
	}
	if arrayCompose == nil {
		t.Fatal("no array compose")
	}
	if len(arrayCompose.Components) != 2 {
		t.Fatalf("the array compose has %d components, want 2", len(arrayCompose.Components))
	}
	for i, component := range arrayCompose.Components {
		lit, ok := exprKindOf(t, main, component).(ir.Literal)
		if !ok {
			t.Fatalf("component %d is %#v, want a folded literal", i, exprKindOf(t, main, component))
		}
		if _, ok := lit.Value.(ir.LiteralF32); !ok {
			t.Fatalf("component %d = %T, want the base type's f32", i, lit.Value)
		}
	}
}

// TestConstructorScalarTooManyArguments is constructor_many's default arm
// (functions.rs:511-513): a scalar constructor takes one argument.
func TestConstructorScalarTooManyArguments(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
void main() {
    int x = int(1, 2);
    o = vec4(float(x));
}
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Constructor: Too many arguments") {
		t.Fatalf("error = %v, want Rust's too-many-arguments constructor error", err)
	}
}

// TestCallOutSwizzleArgumentWritesBack is process_lhs_argument's swizzle arm
// (functions.rs:960-1020): a swizzled out argument is spilled into a local and
// the value is written back one component at a time.
func TestCallOutSwizzleArgumentWritesBack(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void f(out vec2 v) { v = vec2(1.0); }
void main() {
    vec4 v = vec4(0.0);
    f(v.xy);
    o = v;
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	if _, _, cerr := spirv.Compile(mod, spirv.Options{}); cerr != nil {
		t.Fatalf("the SPIR-V backend refused the module: %v", cerr)
	}
	main := functionNamed(t, mod, "main")
	call := firstCallOf(t, main.Body)
	if len(call.Arguments) != 1 {
		t.Fatalf("call has %d arguments, want the spilled one", len(call.Arguments))
	}
	if _, ok := exprKindOf(t, main, call.Arguments[0]).(ir.ExprLocalVariable); !ok {
		t.Fatalf("the out argument is %#v, want the spill local", exprKindOf(t, main, call.Arguments[0]))
	}
	// Two write-backs, one per swizzled component, after the call.
	callIndex := -1
	for i, stmt := range main.Body {
		if _, ok := stmt.Kind.(ir.StmtCall); ok {
			callIndex = i
		}
	}
	stores := 0
	for _, stmt := range main.Body[callIndex+1:] {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			stores++
		}
	}
	if stores < 2 {
		t.Fatalf("body has %d stores after the call, want one per swizzled component", stores)
	}
}

// TestCallOutStructMemberPassesThePointer is process_lhs_argument's
// function-space arm (functions.rs:900-950): a pointer into the function's own
// storage is passed as is, with no spill and no write-back.
func TestCallOutStructMemberPassesThePointer(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
struct S { float x; float y; };
void f(out float v) { v = 1.0; }
void main() {
    S s;
    f(s.x);
    o = vec4(s.x, s.y, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	call := firstCallOf(t, main.Body)
	if _, ok := exprKindOf(t, main, call.Arguments[0]).(ir.ExprAccessIndex); !ok {
		t.Fatalf("the out argument is %#v, want the member pointer itself", exprKindOf(t, main, call.Arguments[0]))
	}
}

// TestCallOutUniformMemberSpills is the non-function-space arm of
// process_lhs_argument (functions.rs:920-935): a pointer into a uniform block
// cannot be written by the callee's space rule, so the argument is spilled and
// written back through the access chain.
func TestCallOutUniformMemberSpills(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float x; } u;
layout(location = 0) out vec4 o;
void f(out float v) { v = 2.0; }
void main() {
    f(u.x);
    o = vec4(u.x);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	call := firstCallOf(t, main.Body)
	if _, ok := exprKindOf(t, main, call.Arguments[0]).(ir.ExprLocalVariable); !ok {
		t.Fatalf("the uniform out argument is %#v, want a spill local", exprKindOf(t, main, call.Arguments[0]))
	}
	wroteBack := false
	for _, stmt := range main.Body {
		store, ok := stmt.Kind.(ir.StmtStore)
		if !ok {
			continue
		}
		if _, ok := exprKindOf(t, main, store.Pointer).(ir.ExprAccessIndex); ok {
			wroteBack = true
		}
	}
	if !wroteBack {
		t.Fatal("no store back through the uniform member's access chain")
	}
}

// TestCallPrototypeThenDefinition is add_function's matching-prototype arm
// (functions.rs:1040-1068): the definition takes the prototype's slot instead
// of adding a second function.
func TestCallPrototypeThenDefinition(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
float f(float x);
float f(float x) { return x * 2.0; }
void main() {
    o = vec4(f(1.0));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// main is lowered as an ordinary module function too; f must exist once.
	count := 0
	for _, function := range mod.Functions {
		if function.Name == "f" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("module has %d functions named f, want the prototype's one slot", count)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
}

// TestCallDuplicateDefinition is add_function's already-defined arm
// (functions.rs:1055-1058).
func TestCallDuplicateDefinition(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
float f(float x) { return x; }
float f(float x) { return x + 1.0; }
void main() { o = vec4(f(1.0)); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Function already defined") {
		t.Fatalf("error = %v, want Rust's already-defined error", err)
	}
}

// TestCallDuplicatePrototype is add_prototype's duplicate arm
// (functions.rs:1130-1135).
func TestCallDuplicatePrototype(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
float f(float x);
float f(float x);
void main() { o = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Prototype already defined") {
		t.Fatalf("error = %v, want Rust's already-defined prototype error", err)
	}
}

// TestCallAmbiguousOverload is the conversion comparison of function_call
// (functions.rs:694-760): within one overload, one argument may prefer it and
// another may prefer the previous one; the two crossed int-to-float and
// int-to-double ranks are what makes the call ambiguous.
func TestCallAmbiguousOverload(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
void g(float a, double b);
void g(double a, float b);
void main() { g(1, 2); o = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Ambiguous best function for 'g'") {
		t.Fatalf("error = %v, want Rust's ambiguous-overload error", err)
	}
}

// TestCallCrossedExactAndOtherIsNotAmbiguous is the other half of the same
// comparison rule (functions.rs:700-724): an exact argument is always better,
// so f(int, uint) beats f(uint, int) for two int arguments - the crossing
// involves an exact match, which settles the overload instead of making it
// ambiguous.
func TestCallCrossedExactAndOtherIsNotAmbiguous(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void f(int a, uint b) { }
void f(uint a, int b) { }
void main() { f(1, 2); o = vec4(1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse rejected the overloads Rust accepts: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	call := firstCallOf(t, main.Body)
	chosen := mod.Functions[call.Function]
	if len(chosen.Arguments) != 2 {
		t.Fatalf("the chosen overload has %d arguments, want 2", len(chosen.Arguments))
	}
	// The exact first argument decides: the (int, uint) overload is chosen.
	first := mod.Types[chosen.Arguments[0].Type].Inner
	if first != i32Scalar {
		t.Fatalf("the chosen overload's first parameter is %v, want int", first)
	}
}

// TestCallUnknownFunction is function_call's no-overload arm
// (functions.rs:760-764).
func TestCallUnknownFunction(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
void main() { o = vec4(unknownFn(1.0)); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Unknown function 'unknownFn'") {
		t.Fatalf("error = %v, want Rust's unknown-function error", err)
	}
}

// TestCallInOutArgumentMustMatchExactly is the inout rule of function_call
// (functions.rs:640-650): an inout argument's value is written back, so only an
// exact type match is accepted.
func TestCallInOutArgumentMustMatchExactly(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
void f(inout int v) { v = v + 1; }
void main() {
    float x = 1.0;
    f(x);
    o = vec4(x);
}
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Unknown function 'f'") {
		t.Fatalf("error = %v, want the inout mismatch to leave no viable overload", err)
	}
}

// TestCallArgumentConversions is conversionOf's rank table
// (functions.rs:1507-1589): int to float, float to double, int to double and
// the same-kind conversion are all accepted, and the argument carries the
// parameter's type.
func TestCallArgumentConversions(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		arg     string
		wantLit any
	}{
		{"int to float", "float", "1", ir.LiteralF32(1)},
		{"float to double", "double", "1.0", ir.LiteralF64(1)},
		{"int to double", "double", "1", ir.LiteralF64(1)},
		{"int to uint", "uint", "1", ir.LiteralU32(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `#version 450
layout(location = 0) out vec4 o;
void f(` + tt.param + ` x) { }
void main() {
    f(` + tt.arg + `);
    o = vec4(1.0);
}
`
			mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			main := functionNamed(t, mod, "main")
			call := firstCallOf(t, main.Body)
			lit, ok := exprKindOf(t, main, call.Arguments[0]).(ir.Literal)
			if !ok {
				t.Fatalf("the argument is %#v, want a folded literal", exprKindOf(t, main, call.Arguments[0]))
			}
			if lit.Value != tt.wantLit {
				t.Fatalf("the argument = %#v, want %#v (the parameter's type)", lit.Value, tt.wantLit)
			}
		})
	}
}

// TestFunctionBodyGetsReturn is ensureBlockReturns (Rust's
// proc/terminator.rs:7-60): a function whose body can fall through gets the
// return appended, descending into a trailing block, if or switch.
func TestFunctionBodyGetsReturn(t *testing.T) {
	tests := []struct {
		name string
		body string
		// want describes the terminal statement: "return", "if" or "switch",
		// where the last is checked in its case bodies.
		want string
	}{
		{"empty body", "", "return"},
		{"trailing block", "{ }", "block"},
		{"trailing if", "if (u.x > 0.0) { }", "if"},
		{"trailing switch", "switch (u.k) { case 0: o = vec4(1.0); break; }", "switch"},
		{"explicit return", "return;", "return"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `#version 450
layout(set = 0, binding = 0) uniform U { float x; int k; } u;
layout(location = 0) out vec4 o;
void f() {
    ` + tt.body + `
}
void main() { f(); o = vec4(1.0); }
`
			mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			f := functionNamed(t, mod, "f")
			if len(f.Body) == 0 {
				t.Fatal("f has an empty body")
			}
			last := f.Body[len(f.Body)-1].Kind
			switch tt.want {
			case "return":
				if _, ok := last.(ir.StmtReturn); !ok {
					t.Fatalf("f ends with %T, want the appended return", last)
				}
			case "block":
				block, ok := last.(ir.StmtBlock)
				if !ok {
					t.Fatalf("f ends with %T, want the trailing block", last)
				}
				wantBlockEndsWithReturn(t, block.Block, "the trailing block")
			case "if":
				ifStmt, ok := last.(ir.StmtIf)
				if !ok {
					t.Fatalf("f ends with %T, want the trailing if", last)
				}
				wantBlockEndsWithReturn(t, ifStmt.Accept, "the accept branch")
				wantBlockEndsWithReturn(t, ifStmt.Reject, "the reject branch")
			case "switch":
				switchStmt, ok := last.(ir.StmtSwitch)
				if !ok {
					t.Fatalf("f ends with %T, want the trailing switch", last)
				}
				for i, c := range switchStmt.Cases {
					if c.FallThrough {
						continue
					}
					wantBlockEndsWithReturn(t, c.Body, "a switch case")
					_ = i
				}
			}
		})
	}
}

// wantBlockEndsWithReturn asserts a block ends with a statement that cannot
// fall through, descending the same way ensureBlockReturns does.
func wantBlockEndsWithReturn(t *testing.T, block ir.Block, what string) {
	t.Helper()
	if len(block) == 0 {
		t.Fatalf("%s is empty, want the appended return", what)
	}
	last := block[len(block)-1].Kind
	switch kind := last.(type) {
	case ir.StmtReturn, ir.StmtBreak, ir.StmtContinue, ir.StmtKill:
		return
	case ir.StmtBlock:
		wantBlockEndsWithReturn(t, kind.Block, what+"'s inner block")
		return
	case ir.StmtIf:
		wantBlockEndsWithReturn(t, kind.Accept, what+"'s accept branch")
		wantBlockEndsWithReturn(t, kind.Reject, what+"'s reject branch")
		return
	case ir.StmtSwitch:
		for _, c := range kind.Cases {
			if c.FallThrough {
				continue
			}
			wantBlockEndsWithReturn(t, c.Body, what+"'s switch case")
		}
		return
	}
	t.Fatalf("%s ends with %T, want a terminator", what, last)
}

// exprKindOf returns the kind of an expression handle of a function.
func exprKindOf(t *testing.T, function ir.Function, h ir.ExpressionHandle) ir.ExpressionKind {
	t.Helper()
	if int(h) >= len(function.Expressions) {
		t.Fatalf("expression handle %d is outside the arena of %d", h, len(function.Expressions))
	}
	return function.Expressions[h].Kind
}

// findTypeHandle returns the handle of a type inner in the module.
func findTypeHandle(t *testing.T, mod *ir.Module, want ir.TypeInner) ir.TypeHandle {
	t.Helper()
	for h, ty := range mod.Types {
		if typeInnerEqual(ty.Inner, want) {
			return ir.TypeHandle(h)
		}
	}
	t.Fatalf("no type %v in the module", want)
	return 0
}

// TestCallOutArgumentConvertsOnWriteBack is process_lhs_argument's conversion
// arm (functions.rs:1000-1026): an out argument whose type differs from the
// parameter is spilled, and the value written back is converted to the
// argument's type.
func TestCallOutArgumentConvertsOnWriteBack(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void f(out int x) { x = 2; }
void main() {
    float v = 0.0;
    f(v);
    o = vec4(v);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	callIndex := -1
	for i, stmt := range main.Body {
		if _, ok := stmt.Kind.(ir.StmtCall); ok {
			callIndex = i
		}
	}
	if callIndex < 0 {
		t.Fatal("main has no call")
	}
	call := main.Body[callIndex].Kind.(ir.StmtCall)
	if _, ok := exprKindOf(t, main, call.Arguments[0]).(ir.ExprLocalVariable); !ok {
		t.Fatalf("the out argument is %#v, want the spill local", exprKindOf(t, main, call.Arguments[0]))
	}
	converted := false
	for _, stmt := range main.Body[callIndex+1:] {
		store, ok := stmt.Kind.(ir.StmtStore)
		if !ok {
			continue
		}
		if as, ok := exprKindOf(t, main, store.Value).(ir.ExprAs); ok && as.Kind == ir.ScalarFloat {
			converted = true
		}
	}
	if !converted {
		t.Fatal("no write-back converted to the argument's float type")
	}
}

// TestCallShadowSamplerMarksDepth is the depth arm of overload matching
// (functions.rs:596-605 with sampled_to_depth): a texture() call on a shadow
// sampler turns the image into a depth image before the sampler pair is used.
func TestCallShadowSamplerMarksDepth(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow sh;
layout(location = 0) out vec4 o;
void main() {
    o = vec4(texture(sh, vec3(0.5, 0.5, 0.25)));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	image := findGlobal(t, mod, "sh")
	inner, ok := mod.Types[image.Type].Inner.(ir.ImageType)
	if !ok || inner.Class != ir.ImageClassDepth {
		t.Fatalf("the shadow sampler's image is %v, want a depth image", mod.Types[image.Type].Inner)
	}
}
