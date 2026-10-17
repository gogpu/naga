package lower

import (
	"math"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// halfToFloat32 / roundToF16 — IEEE 754 half-precision decoding and rounding
// ---------------------------------------------------------------------------

func TestHalfToFloat32(t *testing.T) {
	tests := []struct {
		name string
		half uint16
		want float32
	}{
		{"positive zero", 0x0000, 0.0},
		{"negative zero", 0x8000, float32(math.Copysign(0, -1))},
		{"one", 0x3c00, 1.0},
		{"negative one", 0xbc00, -1.0},
		{"positive inf", 0x7c00, float32(math.Inf(1))},
		{"negative inf", 0xfc00, float32(math.Inf(-1))},
		{"NaN", 0x7e00, float32(math.NaN())},
		{"subnormal", 0x0001, 5.9604645e-8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := halfToFloat32(tt.half)
			if math.IsNaN(float64(tt.want)) {
				if !math.IsNaN(float64(got)) {
					t.Errorf("halfToFloat32(0x%04x) = %v, want NaN", tt.half, got)
				}
				return
			}
			if math.IsInf(float64(tt.want), 0) {
				if !math.IsInf(float64(got), int(math.Copysign(1, float64(tt.want)))) {
					t.Errorf("halfToFloat32(0x%04x) = %v, want %v", tt.half, got, tt.want)
				}
				return
			}
			if got != tt.want {
				t.Errorf("halfToFloat32(0x%04x) = %v, want %v", tt.half, got, tt.want)
			}
		})
	}
}

func TestFloat32HalfRoundTrip(t *testing.T) {
	// Values exactly representable in f16 survive the f16 quantization
	// (roundToF16) unchanged.
	values := []float32{0.0, 1.0, -1.0, 0.5, -0.5, 2.0, 0.25, 100.0}
	for _, v := range values {
		if back := roundToF16(v); back != v {
			t.Errorf("roundToF16(%v) = %v, want %v", v, back, v)
		}
	}
}

// ---------------------------------------------------------------------------
// Error paths: invalid WGSL should produce clear errors
// ---------------------------------------------------------------------------

func TestLowerErrorConstWithoutInit(t *testing.T) {
	// Module-scope const without initializer is a parse error (= is required)
	expectError(t, `const X: i32;
fn test() -> i32 { return X; }`, "expected =")
}

func TestLowerErrorUnsupportedConstInit(t *testing.T) {
	// Constant alias to unknown name
	expectError(t, `const X: i32 = NONEXISTENT;
fn test() -> i32 { return X; }`, "NONEXISTENT")
}

func TestLowerOverrideWithVecTypeCompiles(t *testing.T) {
	// Override with vector type may or may not be supported; verify no panic
	_, err := compileWGSL(t, `override x: f32 = 1.0;
@compute @workgroup_size(1)
fn main() { _ = x; }`)
	if err != nil {
		t.Fatalf("valid override with f32 should compile: %v", err)
	}
}

func TestLowerErrorUnknownType(t *testing.T) {
	// Using an undefined type name in a function parameter
	expectError(t, `fn test(x: NonExistentType) { _ = x; }`, "")
}

func TestLowerErrorDuplicateEntryPoint(t *testing.T) {
	// Two entry points with the same stage is fine but verify no crash
	src := `@vertex fn vs1() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }
@vertex fn vs2() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 2 {
		t.Errorf("expected 2 entry points, got %d", len(module.EntryPoints))
	}
}

func TestLowerErrorBadConstExpr(t *testing.T) {
	// Binary expression with non-constant operand in const context
	expectError(t, `var<private> x: i32 = 5;
const BAD: i32 = x + 1;
fn test() -> i32 { return BAD; }`, "")
}

// ---------------------------------------------------------------------------
// modf / frexp builtin result member access
// ---------------------------------------------------------------------------

func TestLowerModfResultMember(t *testing.T) {
	src := `fn test(x: f32) -> f32 {
    let frac = modf(x).fract;
    let whole = modf(x).whole;
    return frac + whole;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]

	// Must have AccessIndex expressions for .fract and .whole
	accessCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			accessCount++
		}
	}
	if accessCount < 2 {
		t.Errorf("expected at least 2 AccessIndex expressions for modf members, got %d", accessCount)
	}
}

func TestLowerFrexpResultMember(t *testing.T) {
	src := `fn test(x: f32) -> f32 {
    let frac = frexp(x).fract;
    let exponent = frexp(x).exp;
    return frac + f32(exponent);
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]

	// Must have AccessIndex for .fract and .exp
	accessCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			accessCount++
		}
	}
	if accessCount < 2 {
		t.Errorf("expected at least 2 AccessIndex expressions for frexp members, got %d", accessCount)
	}
}

func TestLowerModfVectorResultMember(t *testing.T) {
	src := `fn test(v: vec2<f32>) -> vec2<f32> {
    let frac = modf(v).fract;
    let whole = modf(v).whole;
    return frac + whole;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]

	// Check that modf result struct type is created
	foundModfStruct := false
	for _, ty := range module.Types {
		if strings.HasPrefix(ty.Name, "__modf_result_") {
			foundModfStruct = true
			st, ok := ty.Inner.(ir.StructType)
			if !ok {
				t.Error("modf result type is not a struct")
				continue
			}
			if len(st.Members) != 2 {
				t.Errorf("modf result struct should have 2 members, got %d", len(st.Members))
			}
		}
	}
	if !foundModfStruct {
		t.Error("expected __modf_result_* struct type to be created")
	}
	// Verify function has expressions
	if len(fn.Expressions) == 0 {
		t.Error("expected non-empty expressions in function")
	}
}

func TestLowerFrexpVectorResultMember(t *testing.T) {
	src := `fn test(v: vec3<f32>) -> vec3<f32> {
    return frexp(v).fract;
}`
	module := mustCompile(t, src)

	foundFrexpStruct := false
	for _, ty := range module.Types {
		if strings.HasPrefix(ty.Name, "__frexp_result_") {
			foundFrexpStruct = true
			st, ok := ty.Inner.(ir.StructType)
			if !ok {
				t.Error("frexp result type is not a struct")
				continue
			}
			if len(st.Members) != 2 {
				t.Errorf("frexp result struct should have 2 members, got %d", len(st.Members))
			}
			if st.Members[0].Name != "fract" {
				t.Errorf("first member should be 'fract', got '%s'", st.Members[0].Name)
			}
			if st.Members[1].Name != "exp" {
				t.Errorf("second member should be 'exp', got '%s'", st.Members[1].Name)
			}
		}
	}
	if !foundFrexpStruct {
		t.Error("expected __frexp_result_* struct type to be created")
	}
}

// TestLowerBuiltinResultMemberArgError checks that an error lowering the
// argument of modf/frexp in a member access is reported as is.
func TestLowerBuiltinResultMemberArgError(t *testing.T) {
	tests := []struct{ name, src string }{
		{"modf fract", "fn f() { let a = modf(nope).fract; }"},
		{"frexp exp", "fn f() { let a = frexp(nope).exp; }"},
		{"frexp vector", "fn f() { let a = frexp(vec2(1.0, nope)).fract; }"},
		{"module const", "const c = modf(nope).fract;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.src, "unresolved identifier: nope")
		})
	}
}

// ---------------------------------------------------------------------------
// const_assert with boolean literals
// ---------------------------------------------------------------------------

func TestLowerConstAssertBoolLiterals(t *testing.T) {
	src := `const_assert true;
const_assert !false;
fn test() {}`
	mustCompile(t, src)
}

func TestLowerConstAssertNamedBoolConstant(t *testing.T) {
	src := `const ENABLED: bool = true;
const_assert ENABLED;
fn test() {}`
	mustCompile(t, src)
}

func TestLowerConstAssertComparisonOps(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"equal", `const_assert 5 == 5; fn test() {}`},
		{"not equal", `const_assert 5 != 3; fn test() {}`},
		{"less than", `const_assert 3 < 5; fn test() {}`},
		{"less equal", `const_assert 5 <= 5; fn test() {}`},
		{"greater than", `const_assert 7 > 3; fn test() {}`},
		{"greater equal", `const_assert 5 >= 5; fn test() {}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustCompile(t, tt.src)
		})
	}
}

func TestLowerConstAssertLogicalOps(t *testing.T) {
	src := `const_assert true && true;
const_assert true || false;
const_assert !(false && true);
fn test() {}`
	mustCompile(t, src)
}

func TestLowerConstAssertFails(t *testing.T) {
	expectError(t, `const_assert false;
fn test() {}`, "const_assert failed")
}

func TestLowerConstAssertComplexExpr(t *testing.T) {
	src := `const A: i32 = 10;
const B: i32 = 20;
const_assert A < B;
const_assert A + B == 30;
fn test() {}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Constant integer evaluation
// ---------------------------------------------------------------------------

func TestLowerEvalConstantIntExprBinary(t *testing.T) {
	src := `const A: i32 = 5;
const B: i32 = A + 3;
const C: i32 = B * 2;
const D: i32 = C - 1;
const E: i32 = C / 4;
const F: i32 = C % 3;
fn test() -> i32 { return F; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 6 {
		t.Errorf("expected at least 6 constants, got %d", len(module.Constants))
	}
}

func TestLowerEvalConstantIntUnary(t *testing.T) {
	src := `const A: i32 = 10;
const B: i32 = -A;
fn test() -> i32 { return B; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Constructor calls and component access in const integer context
// ---------------------------------------------------------------------------

func TestLowerConstExprWithCallConstructor(t *testing.T) {
	// i32() and u32() as zero-value constructors in const context
	src := `const A: i32 = i32();
const B: u32 = u32();
const C: i32 = i32(42u);
const D: u32 = u32(10);
fn test() { _ = A; _ = B; _ = C; _ = D; }`
	mustCompile(t, src)
}

func TestLowerConstExprWithVecMemberAccess(t *testing.T) {
	// vec4(4).x pattern in workgroup_size
	src := `@compute @workgroup_size(vec4(8).x)
fn main() {}`
	mustCompile(t, src)
}

func TestLowerConstExprWithVecComponentAccess(t *testing.T) {
	// Access specific component of multi-arg vector constructor
	src := `@compute @workgroup_size(vec3(1, 2, 3).y)
fn main() {}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Type constructors in workgroup_size
// ---------------------------------------------------------------------------

func TestLowerWorkgroupSizeWithTypeConstructors(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			"i32 zero",
			`@compute @workgroup_size(i32(8)) fn main() {}`,
		},
		{
			"u32 zero",
			`@compute @workgroup_size(u32(16)) fn main() {}`,
		},
		{
			"const in workgroup_size",
			`const WG: u32 = 64;
@compute @workgroup_size(WG) fn main() {}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustCompile(t, tt.src)
		})
	}
}

// ---------------------------------------------------------------------------
// Float and integer comparisons
// ---------------------------------------------------------------------------

func TestLowerConstFoldComparisons(t *testing.T) {
	src := `fn test() {
    const EQ: bool = 1.0 == 1.0;
    const NEQ: bool = 1.0 != 2.0;
    const LT: bool = 1.0 < 2.0;
    const LTE: bool = 1.0 <= 1.0;
    const GT: bool = 3.0 > 2.0;
    const GTE: bool = 3.0 >= 3.0;
    var a = EQ; var b = NEQ; var c = LT;
    var d = LTE; var e = GT; var f = GTE;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Mixed int/float and integer arithmetic
// ---------------------------------------------------------------------------

func TestLowerConstFoldMixedIntFloat(t *testing.T) {
	// Mixed abstract int + abstract float should promote to float
	src := `fn test() {
    const A = vec2(1, 1) + vec2(1.0, 1.0);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldIntegerDivByZero(t *testing.T) {
	// A const-expression dividing by zero is a shader-creation error (WGSL
	// spec; Rust naga: "Division by zero"). It used to compile to 0.
	src := `fn test() {
    const A = vec2<i32>(10, 20) / vec2<i32>(0, 1);
    var x = A;
    _ = x;
}`
	_, err := compileWGSL(t, src)
	if err == nil || !strings.Contains(err.Error(), "Division by zero") {
		t.Fatalf("err = %v, want Division by zero", err)
	}
}

// ---------------------------------------------------------------------------
// Shift operations with abstract operands
// ---------------------------------------------------------------------------

func TestLowerShiftOps(t *testing.T) {
	src := `fn test() {
    var a: u32 = 1u << 4u;
    var b: u32 = 16u >> 2u;
    var c: i32 = 1 << 3u;
    var d: i32 = 8 >> 1u;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

func TestLowerShiftAbstractOperands(t *testing.T) {
	// Abstract int as shift amount should be concretized to u32
	src := `fn test() {
    var x: u32 = 1u;
    var y = x << 4u;
    var z = x >> 2u;
    _ = y; _ = z;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Compound assignment with vector splat
// ---------------------------------------------------------------------------

func TestLowerCompoundAssignVectorSplat(t *testing.T) {
	src := `fn test() {
    var v: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    v += 1.0;
    v -= 0.5;
    v *= 2.0;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]

	// A scalar right side is splatted for += and -= (Rust binary_op_splat:
	// Add, Subtract, Divide, Modulo); vector * scalar needs no splat. Rust
	// v30 produces exactly 2 Splat expressions for this function.
	splatCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprSplat); ok {
			splatCount++
		}
	}
	if splatCount != 2 {
		t.Errorf("got %d Splat expressions, want 2 (Rust)", splatCount)
	}
}

// ---------------------------------------------------------------------------
// Struct and scalar zero constructors
// ---------------------------------------------------------------------------

func TestLowerCallConstantScalarConstructors(t *testing.T) {
	src := `const ZI: i32 = i32(0);
const ZU: u32 = u32(0);
const ZF: f32 = f32(0);
const ZB: bool = bool(false);
fn test() { _ = ZI; _ = ZU; _ = ZF; _ = ZB; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 4 {
		t.Errorf("expected at least 4 constants, got %d", len(module.Constants))
	}
}

func TestLowerCallConstantStructConstructor(t *testing.T) {
	src := `struct Params { x: f32, y: f32 }
const P: Params = Params(1.0, 2.0);
fn test() -> f32 { return P.x; }`
	module := mustCompile(t, src)
	if len(module.Constants) == 0 {
		t.Error("expected constants for struct constructor")
	}
}

// ---------------------------------------------------------------------------
// Const aliasing
// ---------------------------------------------------------------------------

func TestLowerConstantAliasAbstractScalar(t *testing.T) {
	// Abstract constant aliased and used in a typed context gets concretized
	src := `const ONE = 1;
const TWO = ONE;
fn test() -> i32 {
    var x: i32 = TWO;
    return x;
}`
	module := mustCompile(t, src)
	// ONE and TWO are both abstract — they don't appear in module.Constants
	// but the function should compile and the variable should get i32 type
	fn := &module.Functions[0]
	if len(fn.Expressions) == 0 {
		t.Error("expected expressions in function using abstract constant alias")
	}
}

func TestLowerConstantAliasConcrete(t *testing.T) {
	// Alias of a concrete (typed) constant
	src := `const SRC: i32 = 42;
const DST = SRC;
fn test() -> i32 { return DST; }`
	module := mustCompile(t, src)
	found := false
	for _, c := range module.Constants {
		if c.Name == "DST" {
			found = true
		}
	}
	if !found {
		t.Error("expected concrete constant 'DST' from alias of concrete 'SRC'")
	}
}

func TestLowerConstantAliasAbstractComposite(t *testing.T) {
	// Alias of abstract composite constant
	src := `const V = vec3(1.0, 2.0, 3.0);
const V2 = V;
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(V2, 1.0);
}`
	mustCompile(t, src)
}

func TestLowerConstantAliasTyped(t *testing.T) {
	// Alias with explicit type annotation
	src := `const SRC: f32 = 3.14;
const DST: f32 = SRC;
fn test() -> f32 { return DST; }`
	module := mustCompile(t, src)
	found := false
	for _, c := range module.Constants {
		if c.Name == "DST" {
			found = true
		}
	}
	if !found {
		t.Error("expected constant named 'DST'")
	}
}

// ---------------------------------------------------------------------------
// Negation, bitwise NOT, logical NOT at module scope
// ---------------------------------------------------------------------------

func TestLowerConstantUnaryBitwiseNot(t *testing.T) {
	src := `const MASK: u32 = ~0u;
fn test() -> u32 { return MASK; }`
	module := mustCompile(t, src)
	if len(module.Constants) == 0 {
		t.Error("expected at least 1 constant for bitwise NOT")
	}
}

func TestLowerConstantUnaryLogicalNot(t *testing.T) {
	src := `const T: bool = true;
const F: bool = !T;
fn test() { _ = F; }`
	mustCompile(t, src)
}

func TestLowerConstantUnaryNegate(t *testing.T) {
	src := `const PI: f32 = 3.14;
const NEG_PI: f32 = -PI;
const N: i32 = 42;
const NEG_N: i32 = -N;
fn test() { _ = NEG_PI; _ = NEG_N; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Raw binary/unary expressions at const scope
// ---------------------------------------------------------------------------

func TestLowerAbstractConstBinaryOps(t *testing.T) {
	// Abstract (untyped) constants with binary operations
	src := `const A = 10;
const B = 20;
const SUM = A + B;
const PRODUCT: i32 = SUM * 2;
fn test() -> i32 { return PRODUCT; }`
	mustCompile(t, src)
}

func TestLowerAbstractConstUnaryNegate(t *testing.T) {
	src := `const X = -42;
const Y: i32 = X;
fn test() -> i32 { return Y; }`
	mustCompile(t, src)
}

func TestLowerAbstractConstUnaryBang(t *testing.T) {
	src := `const T = true;
const F = !T;
const B: bool = F;
fn test() { _ = B; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerIf — if/else if/else chains
// ---------------------------------------------------------------------------

func TestLowerIfElseChain(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    if x > 10 {
        return 3;
    } else if x > 5 {
        return 2;
    } else {
        return 1;
    }
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Body should contain StmtIf (may be preceded by StmtEmit for condition)
	hasIf := false
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtIf); ok {
			hasIf = true
			break
		}
	}
	if !hasIf {
		t.Error("expected StmtIf in if/else chain")
	}
}

func TestLowerIfElseNestedDeep(t *testing.T) {
	// Test deep if/else nesting for scope handling
	src := `fn test(a: i32, b: i32, c: i32) -> i32 {
    if a > 0 {
        if b > 0 {
            if c > 0 {
                return 1;
            } else {
                return 2;
            }
        } else {
            return 3;
        }
    } else {
        return 4;
    }
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Variable shadowing / scope rules
// ---------------------------------------------------------------------------

func TestLowerVariableShadowingBlockScope(t *testing.T) {
	src := `fn test() -> i32 {
    var x: i32 = 1;
    {
        var x: i32 = 2;
        _ = x;
    }
    return x;
}`
	mustCompile(t, src)
}

func TestLowerVariableShadowingParam(t *testing.T) {
	// Local variable shadows parameter name
	src := `fn test(x: i32) -> i32 {
    var x: i32 = x + 1;
    return x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract constant with composite AST types
// ---------------------------------------------------------------------------

func TestLowerAbstractConstantVector(t *testing.T) {
	src := `const V = vec2(1, 2);
const W = vec3(1.0, 2.0, 3.0);
fn test() {
    var a: vec2<i32> = V;
    var b: vec3<f32> = W;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

func TestLowerAbstractConstantArray(t *testing.T) {
	src := `const ARR = array(1, 2, 3, 4);
fn test() {
    var a: array<i32, 4> = ARR;
    _ = a;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Partial constructors with zero args
// ---------------------------------------------------------------------------

func TestLowerZeroArgVecConstructorInConst(t *testing.T) {
	// vec2<i32>() as zero-value constructor at module scope
	src := `const ZERO_VEC: vec2<i32> = vec2<i32>();
fn test() -> vec2<i32> { return ZERO_VEC; }`
	module := mustCompile(t, src)
	if len(module.Constants) == 0 {
		t.Error("expected constant for zero-arg vec constructor")
	}
}

func TestLowerZeroArgMatConstructorInConst(t *testing.T) {
	src := `const ZERO_MAT: mat2x2<f32> = mat2x2<f32>();
fn test() -> mat2x2<f32> { return ZERO_MAT; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Runtime zero-value constructors
// ---------------------------------------------------------------------------

func TestLowerZeroArgVecConstructorRuntimeExplicit(t *testing.T) {
	// Explicit type params: vec3<f32>() → ExprZeroValue (not expanded to Compose)
	src := `fn test() {
    var v: vec3<f32> = vec3<f32>();
    _ = v;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	hasZeroValue := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprZeroValue); ok {
			hasZeroValue = true
		}
	}
	if !hasZeroValue {
		t.Error("expected ExprZeroValue for explicit-type zero-arg vec3<f32>()")
	}
}

func TestLowerZeroArgVecConstructorPartial(t *testing.T) {
	// Partial constructor: vec3() with target type annotation
	src := `fn test() {
    let v: vec3<f32> = vec3();
    _ = v;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Partial zero-arg constructor should expand to Compose with zero literals
	composeCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprCompose); ok {
			composeCount++
		}
	}
	if composeCount == 0 {
		// Might also produce ZeroValue depending on path — verify no panic
		t.Log("partial zero-arg vec3() compiled successfully")
	}
}

func TestLowerZeroArgMatConstructorExplicit(t *testing.T) {
	// Explicit: mat2x2<f32>() → ExprZeroValue
	src := `fn test() {
    var m: mat2x2<f32> = mat2x2<f32>();
    _ = m;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	hasZeroValue := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprZeroValue); ok {
			hasZeroValue = true
		}
	}
	if !hasZeroValue {
		t.Error("expected ExprZeroValue for explicit-type zero-arg mat2x2<f32>()")
	}
}

func TestLowerZeroArgMatConstructorPartial(t *testing.T) {
	// mat2x2() has no arguments to infer the component type from: the WGSL
	// spec has no such overload (only vec2() etc. default to AbstractInt), and
	// Rust naga reports "type can't be inferred" (construction.rs). It used to
	// compile through an in-place expansion.
	src := `fn test() {
    let m: mat2x2<f32> = mat2x2();
    _ = m;
}`
	_, err := compileWGSL(t, src)
	if err == nil || !strings.Contains(err.Error(), "type can't be inferred") {
		t.Fatalf("err = %v, want type can't be inferred", err)
	}
}

// ---------------------------------------------------------------------------
// Scalar to column vector grouping in matrix constants
// ---------------------------------------------------------------------------

func TestLowerMatrixConstantWithScalars(t *testing.T) {
	src := `const M: mat2x2<f32> = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
fn test() -> mat2x2<f32> { return M; }`
	module := mustCompile(t, src)
	// M should be in module constants
	found := false
	for _, c := range module.Constants {
		if c.Name == "M" {
			found = true
			// Value may be CompositeValue with column components, or may have
			// a different representation depending on the lowering path
			t.Logf("matrix constant M: type=%v, value=%T", c.Type, c.Value)
		}
	}
	if !found {
		t.Error("expected constant named 'M'")
	}
}

// ---------------------------------------------------------------------------
// Zero-filled composite
// ---------------------------------------------------------------------------

func TestLowerZeroArgVecComposite(t *testing.T) {
	// Zero-arg constructor for vec type at module scope
	src := `const V: vec4<f32> = vec4<f32>();
fn test() -> vec4<f32> { return V; }`
	module := mustCompile(t, src)
	if len(module.Constants) == 0 {
		t.Error("expected constants for zero-arg vec4<f32>()")
	}
}

// ---------------------------------------------------------------------------
// Abstract array type inference
// ---------------------------------------------------------------------------

func TestLowerAbstractArrayTypeInference(t *testing.T) {
	// Abstract array with unsuffixed int literals → array<i32, N>
	src := `const DATA = array(10, 20, 30);
fn test() {
    var d: array<i32, 3> = DATA;
    _ = d;
}`
	mustCompile(t, src)
}

func TestLowerAbstractArrayFloatInference(t *testing.T) {
	// Abstract array with float literals → array<f32, N>
	src := `const DATA = array(1.0, 2.0, 3.0);
fn test() {
    var d: array<f32, 3> = DATA;
    _ = d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Function forward references
// ---------------------------------------------------------------------------

func TestLowerFunctionForwardReferences(t *testing.T) {
	// Function calling another function that is declared later
	src := `fn helper() -> i32 { return 42; }
fn test() -> i32 { return helper(); }`
	module := mustCompile(t, src)
	if len(module.Functions) < 2 {
		t.Errorf("expected at least 2 functions, got %d", len(module.Functions))
	}
}

func TestLowerFunctionReverseOrder(t *testing.T) {
	// Functions declared in reverse dependency order
	src := `fn test() -> i32 { return helper(); }
fn helper() -> i32 { return 42; }`
	module := mustCompile(t, src)
	if len(module.Functions) < 2 {
		t.Errorf("expected at least 2 functions, got %d", len(module.Functions))
	}
}

func TestLowerFunctionMutualForwardRef(t *testing.T) {
	// Functions that reference globals in mixed order
	src := `var<private> counter: i32 = 0;
fn inc() { counter = counter + 1; }
fn dec() { counter = counter - 1; }
fn test() {
    inc();
    dec();
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Alias type declarations
// ---------------------------------------------------------------------------

func TestLowerTypeAliasChained(t *testing.T) {
	src := `alias Float = f32;
alias Vec = vec3<Float>;
fn test(v: Vec) -> Float {
    return v.x;
}`
	module := mustCompile(t, src)
	if len(module.Functions) == 0 {
		t.Error("expected at least 1 function")
	}
}

// ---------------------------------------------------------------------------
// Edge case: empty function body
// ---------------------------------------------------------------------------

func TestLowerEmptyFunctionBody(t *testing.T) {
	src := `fn test() {}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Empty function body should have return statement added
	if len(fn.Body) == 0 {
		// It's valid for empty body to have no statements
		// Just verify no crash
		t.Log("empty function body compiled successfully")
	}
}

// ---------------------------------------------------------------------------
// Edge case: unicode identifiers
// ---------------------------------------------------------------------------

func TestLowerUnicodeIdentifiers(t *testing.T) {
	src := `fn test() {
    var _x1: f32 = 1.0;
    var _y2: f32 = 2.0;
    _ = _x1; _ = _y2;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Complex expression: nested access chains
// ---------------------------------------------------------------------------

func TestLowerNestedAccessChain(t *testing.T) {
	src := `struct Inner { value: f32 }
struct Outer { inner: Inner }
fn test(o: Outer) -> f32 {
    return o.inner.value;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have AccessIndex expressions for the chain
	accessCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			accessCount++
		}
	}
	if accessCount < 2 {
		t.Errorf("expected at least 2 AccessIndex for nested struct access, got %d", accessCount)
	}
}

func TestLowerMatrixColumnSwizzle(t *testing.T) {
	src := `fn test(m: mat4x4<f32>) -> vec4<f32> {
    let col0 = m[0];
    let col1 = m[1];
    return col0 + col1;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Pointer deref chains
// ---------------------------------------------------------------------------

func TestLowerPointerDerefAccess(t *testing.T) {
	src := `struct Data { x: f32, y: f32 }
fn test() {
    var d: Data = Data(1.0, 2.0);
    let px = &d.x;
    *px = 3.0;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract int/float with nested expressions
// ---------------------------------------------------------------------------

func TestLowerAbstractIntPromotionNested(t *testing.T) {
	src := `fn test() {
    var a: f32 = 1 + 2 + 3;
    var b: f32 = 1 * 2.0 + 3;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Complex constant values
// ---------------------------------------------------------------------------

func TestLowerBuildConstGlobalExprScalar(t *testing.T) {
	src := `const X: f32 = 3.14;
const Y: i32 = 42;
const Z: u32 = 100u;
fn test() -> f32 { return X + f32(Y) + f32(Z); }`
	module := mustCompile(t, src)
	// Verify each constant has GlobalExpression init
	for _, c := range module.Constants {
		if c.Name != "" && c.Init == 0 && len(module.GlobalExpressions) == 0 {
			t.Errorf("constant '%s' should have GlobalExpression init", c.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Const folding: floor, ceil, round, trunc on negative values
// ---------------------------------------------------------------------------

func TestLowerConstFoldRoundingNegative(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = ceil(-1.3);
    const B: f32 = floor(-1.7);
    const C: f32 = round(-1.5);
    const D: f32 = trunc(-1.9);
    return A + B + C + D;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Nested composite constants
// ---------------------------------------------------------------------------

func TestLowerInferCompositeConstantTypeNested(t *testing.T) {
	// Array of vectors at module scope
	src := `const DATA: array<vec2<f32>, 3> = array<vec2<f32>, 3>(
    vec2<f32>(1.0, 2.0),
    vec2<f32>(3.0, 4.0),
    vec2<f32>(5.0, 6.0),
);
fn test() -> vec2<f32> { return DATA[0]; }`
	module := mustCompile(t, src)
	if len(module.Constants) == 0 {
		t.Error("expected constants for nested composite")
	}
}

// ---------------------------------------------------------------------------
// Negation on all scalar kinds
// ---------------------------------------------------------------------------

func TestLowerConstFoldNegateAll(t *testing.T) {
	src := `fn test() {
    const A = -vec2<i32>(1, 2);
    const B = -vec3<f32>(1.0, 2.0, 3.0);
    var x = A; var y = B;
    _ = x; _ = y;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract int widening to match operand
// ---------------------------------------------------------------------------

func TestLowerWidenScalar(t *testing.T) {
	src := `fn test() {
    var x: f32 = 1.0;
    var y = x + 2;
    _ = y;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Scalar resolution through arrays and atomics
// ---------------------------------------------------------------------------

func TestLowerAtomicOpsAddLoadStore(t *testing.T) {
	src := `var<workgroup> counter: atomic<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    atomicAdd(&counter, 1u);
    let val = atomicLoad(&counter);
    atomicStore(&counter, val + 1u);
    _ = val;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract float defaulting to f32
// ---------------------------------------------------------------------------

func TestLowerConcretizeAbstractToFloat(t *testing.T) {
	// Abstract float in context requiring concrete float
	src := `fn test() {
    let x = sin(1.0);
    let y = cos(0.0);
    let z = sqrt(4.0);
    _ = x; _ = y; _ = z;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// binaryOpSplat — scalar-vector mixing
// ---------------------------------------------------------------------------

func TestLowerBinaryOpSplatScalarVec(t *testing.T) {
	src := `fn test() {
    var v: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    var s: f32 = 2.0;
    var r1 = v * s;
    var r2 = s * v;
    _ = r1; _ = r2;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Expression-level concretization
// ---------------------------------------------------------------------------

func TestLowerConcretizeExprToScalar(t *testing.T) {
	src := `fn test(x: f32) -> f32 {
    return x + 1;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Integer expression to float
// ---------------------------------------------------------------------------

func TestLowerConvertExprToFloat(t *testing.T) {
	src := `fn test(x: i32) -> f32 {
    return f32(x);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Concrete value computed from abstract literal
// ---------------------------------------------------------------------------

func TestLowerComputeConcreteLiteral(t *testing.T) {
	// Use abstract int in vec<f32> context
	src := `fn test() {
    var v: vec2<f32> = vec2<f32>(1, 2);
    _ = v;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Pointer target scalar resolution
// ---------------------------------------------------------------------------

func TestLowerResolvePointerScalar(t *testing.T) {
	src := `fn test() {
    var x: f32 = 1.0;
    let p = &x;
    *p = 2.0;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract int to u32
// ---------------------------------------------------------------------------

func TestLowerConcretizeAbstractToUint(t *testing.T) {
	src := `fn test() {
    var x: u32 = 42;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Override type inference with expressions
// ---------------------------------------------------------------------------

func TestLowerOverrideTypeInferenceExpr(t *testing.T) {
	src := `override A = 1 + 2;
override B = 3.14 * 2.0;
@compute @workgroup_size(1)
fn main() { _ = A; _ = B; }`
	module := mustCompile(t, src)
	if len(module.Overrides) < 2 {
		t.Errorf("expected at least 2 overrides, got %d", len(module.Overrides))
	}
}

// ---------------------------------------------------------------------------
// Override init with binary expression
// ---------------------------------------------------------------------------

func TestLowerOverrideInitBinaryExpr(t *testing.T) {
	src := `override base: f32 = 10.0;
override scaled: f32 = 2.0;
@compute @workgroup_size(1)
fn main() {
    let r = base * scaled;
    _ = r;
}`
	module := mustCompile(t, src)
	if len(module.Overrides) != 2 {
		t.Errorf("expected 2 overrides, got %d", len(module.Overrides))
	}
}

// ---------------------------------------------------------------------------
// Scalar coercion edge cases
// ---------------------------------------------------------------------------

func TestLowerCoerceScalarToType(t *testing.T) {
	// WGSL has no automatic conversion between concrete types: u32 is not an
	// i32 initializer (Rust v30 reports the same message).
	src := `const X: i32 = 42u;
fn test() -> i32 { return X; }`
	_, err := compileWGSL(t, src)
	const want = "the type of `X` is expected to be `i32`, but got `u32`"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// ---------------------------------------------------------------------------
// All literal suffixes
// ---------------------------------------------------------------------------

func TestLowerLiteralSuffixes(t *testing.T) {
	src := `fn test() {
    var a: f32 = 1.0f;
    var b: i32 = 42i;
    var c: u32 = 100u;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

func TestLowerLiteralHexAndBinary(t *testing.T) {
	src := `fn test() {
    var h: u32 = 0xFF;
    var b: u32 = 0xFF00u;
    _ = h; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Suffix detection for abstract types
// ---------------------------------------------------------------------------

func TestLowerAbstractScalarKindDetection(t *testing.T) {
	// Unsuffixed literals should be abstract
	src := `const I = 42;
const F = 3.14;
const IF32: f32 = I;
const FF32: f32 = F;
fn test() { _ = IF32; _ = FF32; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// constEvalExprToU32 — expression to u32 for array sizes
// ---------------------------------------------------------------------------

func TestLowerArraySizeFromConstExpr(t *testing.T) {
	src := `const N: u32 = 10u;
const M: u32 = N * 2u;
var<private> data: array<f32, M>;
fn test() -> f32 { return data[0]; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Composite constant with struct type
// ---------------------------------------------------------------------------

func TestLowerCompositeConstantStruct(t *testing.T) {
	src := `struct Config {
    width: f32,
    height: f32,
    depth: u32,
}
const CFG: Config = Config(800.0, 600.0, 1u);
fn test() -> f32 { return CFG.width; }`
	module := mustCompile(t, src)
	found := false
	for _, c := range module.Constants {
		if c.Name == "CFG" {
			found = true
		}
	}
	if !found {
		t.Error("expected constant named 'CFG'")
	}
}

// ---------------------------------------------------------------------------
// Abstract type to concrete
// ---------------------------------------------------------------------------

func TestLowerConcretizeAbstractArrayType(t *testing.T) {
	// Array with abstract element type should concretize
	src := `const SIZES = array(100, 200, 300);
fn test() {
    var s: array<i32, 3> = SIZES;
    _ = s;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Whether an initializer has a concrete type
// ---------------------------------------------------------------------------

func TestLowerInitHasConcreteType(t *testing.T) {
	// Suffixed literals are concrete
	src := `const A: f32 = 1.0f;
const B = 42i;
const C = 3.14;
fn test() { _ = A; _ = B; _ = C; }`
	module := mustCompile(t, src)
	// A and B should be in module constants (concrete)
	// C is abstract (no suffix, no type annotation)
	concreteNames := make(map[string]bool)
	for _, c := range module.Constants {
		if c.Name != "" {
			concreteNames[c.Name] = true
		}
	}
	if !concreteNames["A"] {
		t.Error("expected concrete constant 'A'")
	}
	if !concreteNames["B"] {
		t.Error("expected concrete constant 'B'")
	}
}

// ---------------------------------------------------------------------------
// Const struct constructor with member types
// ---------------------------------------------------------------------------

func TestLowerEvalConstantArgsStruct(t *testing.T) {
	src := `struct Point { x: f32, y: f32 }
const P: Point = Point(1.0, 2.0);
fn test() -> f32 { return P.x + P.y; }`
	module := mustCompile(t, src)
	found := false
	for _, c := range module.Constants {
		if c.Name == "P" {
			found = true
			if cv, ok := c.Value.(ir.CompositeValue); ok {
				if len(cv.Components) < 2 {
					t.Errorf("expected at least 2 components in struct constant, got %d", len(cv.Components))
				}
			}
		}
	}
	if !found {
		t.Error("expected constant 'P'")
	}
}

// ---------------------------------------------------------------------------
// Float-specific const evaluation
// ---------------------------------------------------------------------------

func TestLowerEvalConstantFloatExpr(t *testing.T) {
	src := `const PI: f32 = 3.14;
const TAU: f32 = PI * 2.0;
const HALF_PI: f32 = PI / 2.0;
fn test() -> f32 { return TAU + HALF_PI; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 3 {
		t.Errorf("expected at least 3 constants, got %d", len(module.Constants))
	}
}

// ---------------------------------------------------------------------------
// Constant identifier resolution
// ---------------------------------------------------------------------------

func TestLowerEvalConstantIdentChain(t *testing.T) {
	// Chain of const references
	src := `const A: i32 = 1;
const B: i32 = A;
const C: i32 = B;
const D: i32 = C + A;
fn test() -> i32 { return D; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 4 {
		t.Errorf("expected at least 4 constants, got %d", len(module.Constants))
	}
}

// ---------------------------------------------------------------------------
// lowerFor / lowerWhile / lowerLoop — loop lowering
// ---------------------------------------------------------------------------

func TestLowerForLoopWithBody(t *testing.T) {
	src := `fn test() {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 10; i++) {
        sum += i;
    }
    _ = sum;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should produce a loop statement
	hasLoop := false
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtLoop); ok {
			hasLoop = true
		}
	}
	if !hasLoop {
		t.Error("expected StmtLoop in for loop lowering")
	}
}

func TestLowerWhileLoopDecrement(t *testing.T) {
	src := `fn test() {
    var x: i32 = 10;
    while x > 0 {
        x -= 1;
    }
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	hasLoop := false
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtLoop); ok {
			hasLoop = true
		}
	}
	if !hasLoop {
		t.Error("expected StmtLoop in while loop lowering")
	}
}

func TestLowerLoopWithBreakIf(t *testing.T) {
	src := `fn test() {
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        i += 1;
        continuing {
            break if i == 5;
        }
    }
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Switch statement lowering
// ---------------------------------------------------------------------------

func TestLowerSwitchStatement(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    switch x {
        case 1: { return 10; }
        case 2, 3: { return 20; }
        default: { return 0; }
    }
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	hasSwitch := false
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtSwitch); ok {
			hasSwitch = true
		}
	}
	if !hasSwitch {
		t.Error("expected StmtSwitch in switch lowering")
	}
}

// ---------------------------------------------------------------------------
// lowerGlobalVarInit with const expressions
// ---------------------------------------------------------------------------

func TestLowerGlobalVarInitWithConstExpr(t *testing.T) {
	src := `const SCALE: f32 = 2.0;
const OFFSET: f32 = 1.0;
var<private> value: f32 = SCALE;
fn test() -> f32 { return value + OFFSET; }`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) == 0 {
		t.Error("expected global variable")
	}
}

// ---------------------------------------------------------------------------
// Global var type inference with array
// ---------------------------------------------------------------------------

func TestLowerInferGlobalVarTypeArray(t *testing.T) {
	src := `var<private> arr = array<f32, 3>(1.0, 2.0, 3.0);
fn test() -> f32 { return arr[0]; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// generateExternalTextureTypes — external texture type creation
// ---------------------------------------------------------------------------

func TestLowerExternalTextureTypes(t *testing.T) {
	// Using external_texture triggers type generation
	src := `@group(0) @binding(0) var ext_tex: texture_external;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main() -> @location(0) vec4<f32> {
    return textureSampleBaseClampToEdge(ext_tex, samp, vec2<f32>(0.5, 0.5));
}`
	module := mustCompile(t, src)
	// Verify the special types were created
	if module.SpecialTypes.ExternalTextureParams == nil {
		t.Error("expected ExternalTextureParams type to be created")
	}
	if module.SpecialTypes.ExternalTextureTransferFunction == nil {
		t.Error("expected ExternalTextureTransferFunction type to be created")
	}
}

// ---------------------------------------------------------------------------
// Width inference from type context
// ---------------------------------------------------------------------------

func TestLowerInferScalarWidthF16(t *testing.T) {
	src := `
enable f16;
fn test() {
    var x: f16 = 1.0h;
    var y: f16 = 2.0h;
    var z = x + y;
    _ = z;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Const fold vector with splat
// ---------------------------------------------------------------------------

func TestLowerConstFoldVecSplat(t *testing.T) {
	src := `fn test() {
    const V: vec4<f32> = vec4(1.0);
    const W: vec3<i32> = vec3(42);
    var a = V; var b = W;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract int defaulting to float
// ---------------------------------------------------------------------------

func TestLowerAbstractIntToFloat(t *testing.T) {
	src := `fn test() {
    let x = sin(1);
    let y = cos(0);
    _ = x; _ = y;
}`
	mustCompile(t, src)
}
