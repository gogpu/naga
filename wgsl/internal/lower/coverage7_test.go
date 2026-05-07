package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// evalCallAsConstantInt — i32()/u32() in const integer context (workgroup_size)
// ---------------------------------------------------------------------------

func TestLowerEvalCallAsConstantIntZeroArg(t *testing.T) {
	// i32() and u32() zero-arg constructors used in workgroup_size
	// Note: workgroup_size evaluation may fall back to defaults for complex expressions
	src := `@compute @workgroup_size(i32(8), u32(4), 1)
fn main() {}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(module.EntryPoints))
	}
	// Just verify compilation — the actual workgroup values depend on eval path
	t.Logf("workgroup = %v", module.EntryPoints[0].Workgroup)
}

func TestLowerEvalCallAsConstantIntConversion(t *testing.T) {
	// Test i32() conversion in const context via function-local const
	src := `fn test() {
    const A: i32 = i32(42u);
    const B: u32 = u32(10);
    var x = A; var y = B;
    _ = x; _ = y;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalMemberAsConstantInt — vec.component in workgroup_size
// ---------------------------------------------------------------------------

func TestLowerEvalMemberAsConstantIntSplat(t *testing.T) {
	// vec4(8).x splat access in const context
	src := `fn test() {
    const V = vec4(8);
    const X: i32 = V.x;
    var r = X;
    _ = r;
}`
	mustCompile(t, src)
}

func TestLowerEvalMemberAsConstantIntMultiArg(t *testing.T) {
	// vec3(4, 8, 16).y access in const context
	src := `fn test() {
    const V = vec3(4, 8, 16);
    const Y: i32 = V.y;
    var r = Y;
    _ = r;
}`
	mustCompile(t, src)
}

func TestLowerEvalMemberComponentNames(t *testing.T) {
	// Test x/y/z/w component access on const vectors
	src := `fn test() {
    const V = vec4(10, 20, 30, 40);
    const A: i32 = V.x;
    const B: i32 = V.y;
    const C: i32 = V.z;
    const D: i32 = V.w;
    var a = A; var b = B; var c = C; var d = D;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// functionHasForwardRefs — forward reference detection in mixed declarations
// ---------------------------------------------------------------------------

func TestLowerFunctionForwardRefGlobals(t *testing.T) {
	// Function references global declared after it
	src := `fn test() -> i32 { return counter; }
var<private> counter: i32 = 42;`
	module := mustCompile(t, src)
	if len(module.Functions) == 0 {
		t.Error("expected at least 1 function")
	}
}

func TestLowerFunctionForwardRefFunctions(t *testing.T) {
	// Three functions: test calls b, b calls a
	src := `fn a() -> i32 { return 1; }
fn b() -> i32 { return a() + 1; }
fn test() -> i32 { return b() + 1; }`
	module := mustCompile(t, src)
	if len(module.Functions) < 3 {
		t.Errorf("expected 3 functions, got %d", len(module.Functions))
	}
}

func TestLowerFunctionForwardRefReverse(t *testing.T) {
	// Functions in reverse dependency order
	src := `fn test() -> i32 { return b(); }
fn b() -> i32 { return a(); }
fn a() -> i32 { return 42; }`
	module := mustCompile(t, src)
	if len(module.Functions) < 3 {
		t.Errorf("expected 3 functions, got %d", len(module.Functions))
	}
}

// ---------------------------------------------------------------------------
// concretizeShiftLeft — abstract int/float shift left operand
// ---------------------------------------------------------------------------

func TestLowerConcretizeShiftLeftAbstractInt(t *testing.T) {
	// Abstract int on left side of shift should concretize to i32
	src := `fn test(n: u32) -> i32 {
    return 1 << n;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have a Literal(I32(1)) expression, not AbstractInt
	hasI32Lit := false
	for _, expr := range fn.Expressions {
		if lit, ok := expr.Kind.(ir.Literal); ok {
			if _, isI32 := lit.Value.(ir.LiteralI32); isI32 {
				hasI32Lit = true
			}
		}
	}
	if !hasI32Lit {
		t.Error("expected concretized I32 literal from abstract int in shift left")
	}
}

// ---------------------------------------------------------------------------
// tryConstEvalPhonyExpr — select() with all-literal args in phony assignment
// ---------------------------------------------------------------------------

func TestLowerPhonySelectConstEval(t *testing.T) {
	src := `fn test() {
    _ = select(1, 2, true);
    _ = select(3.0, 4.0, false);
    _ = select(1, 2f, false);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// constFoldSelect — scalar and vector select
// ---------------------------------------------------------------------------

func TestLowerConstFoldSelectScalar(t *testing.T) {
	src := `fn test() {
    const A: i32 = select(10, 20, true);
    const B: i32 = select(10, 20, false);
    const C: f32 = select(1.0, 2.0, true);
    var a = A; var b = B; var c = C;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldSelectVector(t *testing.T) {
	src := `fn test() {
    const A = select(vec2<i32>(1, 2), vec2<i32>(3, 4), vec2<bool>(true, false));
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// constFoldAs — const type conversion
// ---------------------------------------------------------------------------

func TestLowerConstFoldAsScalar(t *testing.T) {
	src := `fn test() {
    const A: i32 = i32(3.14f);
    const B: f32 = f32(42i);
    const C: u32 = u32(100i);
    const D: bool = bool(1i);
    const E: bool = bool(0i);
    var a = A; var b = B; var c = C; var d = D; var e = E;
    _ = a; _ = b; _ = c; _ = d; _ = e;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldAsVector(t *testing.T) {
	src := `fn test() {
    const V: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    const W: vec3<i32> = vec3<i32>(V);
    var x = W;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// convertLiteral — all type conversion paths
// ---------------------------------------------------------------------------

func TestLowerConvertLiteralAllPaths(t *testing.T) {
	src := `fn test() {
    const FI: i32 = i32(3.14f);
    const FU: u32 = u32(3.14f);
    const IF: f32 = f32(42i);
    const UF: f32 = f32(42u);
    var a = FI; var b = FU; var c = IF; var d = UF;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// deepCopyConstExpr — deep copy of const expressions
// ---------------------------------------------------------------------------

func TestLowerDeepCopyConstExprCompose(t *testing.T) {
	// select() with vector operands triggers deepCopyConstExpr
	src := `fn test() {
    const V = vec3<f32>(1.0, 2.0, 3.0);
    const W = vec3<f32>(4.0, 5.0, 6.0);
    const R = select(V, W, vec3<bool>(true, false, true));
    var x = R;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerMemberForRef — member access in reference context (store target)
// ---------------------------------------------------------------------------

func TestLowerMemberForRefStruct(t *testing.T) {
	src := `struct Point { x: f32, y: f32 }
fn test() {
    var p: Point = Point(1.0, 2.0);
    p.x = 3.0;
    p.y = 4.0;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have Store statements
	storeCount := 0
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			storeCount++
		}
	}
	if storeCount < 2 {
		t.Errorf("expected at least 2 Store statements, got %d", storeCount)
	}
}

func TestLowerMemberForRefVectorComponent(t *testing.T) {
	// Store to vector component via member access
	src := `fn test() {
    var v: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    v.x = 10.0;
    v.y = 20.0;
    v.z = 30.0;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// tryConstantArrayIndex — compile-time array indexing
// ---------------------------------------------------------------------------

func TestLowerConstantArrayIndex(t *testing.T) {
	src := `const DATA: array<i32, 4> = array<i32, 4>(10, 20, 30, 40);
fn test() -> i32 {
    return DATA[2];
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstantArgsAsGlobalExprs — global expression creation for const args
// ---------------------------------------------------------------------------

func TestLowerEvalConstantArgsAsGlobalExprs(t *testing.T) {
	src := `struct Config { a: f32, b: f32 }
const CFG: Config = Config(1.0, 2.0);
var<private> cfg: Config = CFG;
fn test() -> f32 { return cfg.a; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// buildGlobalExprFromAST — struct constructor as global init
// ---------------------------------------------------------------------------

func TestLowerBuildGlobalExprFromASTStruct(t *testing.T) {
	src := `struct Vertex { pos: vec3<f32>, uv: vec2<f32> }
var<private> default_vertex: Vertex = Vertex(vec3<f32>(0.0, 0.0, 0.0), vec2<f32>(0.0, 0.0));
fn test() -> f32 { return default_vertex.pos.x; }`
	mustCompile(t, src)
}

func TestLowerBuildGlobalExprFromASTMatrix(t *testing.T) {
	// Matrix constructor in global var init
	src := `var<private> identity: mat2x2<f32> = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
fn test() -> f32 { return identity[0].x; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// expandZeroConstructGE — zero-arg constructor in global expression
// ---------------------------------------------------------------------------

func TestLowerExpandZeroConstructGEVec(t *testing.T) {
	src := `var<private> v: vec3<f32> = vec3<f32>();
fn test() { v.x = 1.0; }`
	mustCompile(t, src)
}

func TestLowerExpandZeroConstructGEMat(t *testing.T) {
	src := `var<private> m: mat2x2<f32> = mat2x2<f32>();
fn test() { _ = m; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeConstantScalar — scalar concretization in composite context
// ---------------------------------------------------------------------------

func TestLowerConcretizeConstantScalarInComposite(t *testing.T) {
	// Use abstract int in typed vector constant
	src := `const V: vec3<i32> = vec3<i32>(1, 2, 3);
fn test() -> i32 { return V.x; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstantArgExpr — binary expression as constant arg
// ---------------------------------------------------------------------------

func TestLowerEvalConstantArgExpr(t *testing.T) {
	src := `const A: i32 = 5;
const V: vec2<i32> = vec2<i32>(A + 1, A * 2);
fn test() -> i32 { return V.x; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// createZeroComponents — zero-value components for vector/matrix
// ---------------------------------------------------------------------------

func TestLowerCreateZeroComponentsVector(t *testing.T) {
	src := `const Z: vec4<f32> = vec4<f32>();
fn test() -> f32 { return Z.x; }`
	mustCompile(t, src)
}

func TestLowerCreateZeroComponentsMatrix(t *testing.T) {
	src := `const Z: mat3x3<f32> = mat3x3<f32>();
fn test() { _ = Z; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// inferCompositeConstantType — type inference for composites
// ---------------------------------------------------------------------------

func TestLowerInferCompositeConstTypeVec(t *testing.T) {
	// Constructor without explicit type — infer from args
	src := `const V = vec2(1.0, 2.0);
const W: vec2<f32> = V;
fn test() { _ = W; }`
	mustCompile(t, src)
}

func TestLowerInferCompositeConstTypeMat(t *testing.T) {
	src := `const M = mat2x2(1.0, 0.0, 0.0, 1.0);
const N: mat2x2<f32> = M;
fn test() { _ = N; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// innerScalar — scalar extraction from nested types
// ---------------------------------------------------------------------------

func TestLowerInnerScalarFromArray(t *testing.T) {
	// Array of f32 in a context that needs scalar extraction
	src := `fn test() {
    var a: array<f32, 4> = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    a[0] = 5.0;
    _ = a;
}`
	mustCompile(t, src)
}

func TestLowerInnerScalarFromVec(t *testing.T) {
	src := `fn test() {
    var v: vec4<i32> = vec4<i32>(1, 2, 3, 4);
    v.x = 10;
    _ = v;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// inlineCompositeConstant / inlineConstantValue — inlining at use sites
// ---------------------------------------------------------------------------

func TestLowerInlineCompositeConstantAtUse(t *testing.T) {
	// Abstract composite constant used at multiple sites with different types
	src := `const POS = vec2(0.5, 0.5);
fn test() {
    var a: vec2<f32> = POS;
    var b: vec2<f32> = POS;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

func TestLowerInlineScalarConstantAtUse(t *testing.T) {
	// Abstract scalar constant inlined at use site
	src := `const HALF = 0.5;
fn test() -> f32 {
    var x: f32 = HALF;
    var y: f32 = HALF;
    return x + y;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// abstractScalarKind — suffix detection
// ---------------------------------------------------------------------------

func TestLowerAbstractScalarKindSuffixed(t *testing.T) {
	// Suffixed literals are concrete
	src := `const A = 42i;
const B = 42u;
const C = 3.14f;
fn test() { _ = A; _ = B; _ = C; }`
	module := mustCompile(t, src)
	// A, B, C should be in module constants (not abstract)
	concreteCount := 0
	for _, c := range module.Constants {
		if c.Name == "A" || c.Name == "B" || c.Name == "C" {
			concreteCount++
		}
	}
	if concreteCount < 3 {
		t.Errorf("expected 3 concrete constants (suffixed literals), got %d", concreteCount)
	}
}

func TestLowerAbstractScalarKindUnsuffixed(t *testing.T) {
	// Unsuffixed literals should be abstract
	src := `const A = 42;
const B = 3.14;
fn test() {
    var x: i32 = A;
    var y: f32 = B;
    _ = x; _ = y;
}`
	module := mustCompile(t, src)
	// A and B should NOT be in module constants (they're abstract)
	for _, c := range module.Constants {
		if c.Name == "A" || c.Name == "B" {
			t.Errorf("unsuffixed constant '%s' should not be in module.Constants (should be abstract)", c.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Struct member access in const context via evalConstantArgs
// ---------------------------------------------------------------------------

func TestLowerEvalConstantArgsWithStruct(t *testing.T) {
	src := `struct Pair { a: i32, b: i32 }
const P: Pair = Pair(10, 20);
fn test() -> i32 { return P.a + P.b; }`
	module := mustCompile(t, src)
	found := false
	for _, c := range module.Constants {
		if c.Name == "P" {
			found = true
		}
	}
	if !found {
		t.Error("expected constant 'P'")
	}
}

// ---------------------------------------------------------------------------
// Global var init with various expression types (exercises buildGlobalExprFromAST)
// ---------------------------------------------------------------------------

func TestLowerGlobalVarInitVectorConstructor(t *testing.T) {
	src := `var<private> pos: vec4<f32> = vec4<f32>(1.0, 2.0, 3.0, 4.0);
fn test() -> f32 { return pos.x; }`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) == 0 {
		t.Error("expected global variable")
	}
	// Should have global expressions for the init
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions for vector constructor init")
	}
}

func TestLowerGlobalVarInitScalarLiteral(t *testing.T) {
	src := `var<private> scale: f32 = 2.5;
fn test() -> f32 { return scale; }`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) == 0 {
		t.Error("expected global variable")
	}
}

// ---------------------------------------------------------------------------
// lowerGlobalVarInit with const reference
// ---------------------------------------------------------------------------

func TestLowerGlobalVarInitConstRef(t *testing.T) {
	src := `const BASE: f32 = 10.0;
var<private> val: f32 = BASE;
fn test() -> f32 { return val; }`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) == 0 {
		t.Error("expected global variable")
	}
}

// ---------------------------------------------------------------------------
// inferGlobalVarType — various types
// ---------------------------------------------------------------------------

func TestLowerInferGlobalVarTypeBool(t *testing.T) {
	src := `var<private> flag = true;
fn test() { flag = false; }`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "flag" {
			found = true
		}
	}
	if !found {
		t.Error("expected global variable 'flag'")
	}
}

func TestLowerInferGlobalVarTypeMatrix(t *testing.T) {
	src := `var<private> m = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
fn test() { _ = m; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// resolveScalarFromName — all scalar type names
// ---------------------------------------------------------------------------

func TestLowerResolveScalarFromAllNames(t *testing.T) {
	src := `fn test() {
    var a: bool = true;
    var b: i32 = 0;
    var c: u32 = 0u;
    var d: f32 = 0.0;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// extractVec3Floats / tryFoldCross / tryFoldDot
// ---------------------------------------------------------------------------

func TestLowerConstFoldDotProduct(t *testing.T) {
	src := `fn test() {
    const V = vec3<f32>(1.0, 0.0, 0.0);
    const W = vec3<f32>(0.0, 1.0, 0.0);
    const D: f32 = dot(V, W);
    var x = D;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldCrossProduct(t *testing.T) {
	src := `fn test() {
    const V = vec3<f32>(1.0, 0.0, 0.0);
    const W = vec3<f32>(0.0, 1.0, 0.0);
    const C = cross(V, W);
    var x = C;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerTextureAtomic / lowerSubgroupBallot / lowerSubgroupCollective — complex builtins
// These are at 0% but require special extensions or ray tracing types, so we test
// the error paths and simple cases.
// ---------------------------------------------------------------------------

func TestLowerTextureOperations(t *testing.T) {
	// Basic texture sampling exercises texture lowering
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv);
}`
	mustCompile(t, src)
}

func TestLowerTextureLoadInteger(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@fragment
fn main() -> @location(0) vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0);
}`
	mustCompile(t, src)
}

func TestLowerTextureDimensionsVec(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@fragment
fn main() -> @location(0) vec4<f32> {
    let dims = textureDimensions(tex);
    return vec4<f32>(f32(dims.x), f32(dims.y), 0.0, 1.0);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Error path: bad binary expression operand in const context
// ---------------------------------------------------------------------------

func TestLowerErrorUnknownFunctionCall(t *testing.T) {
	// Calling a function that doesn't exist
	expectError(t, `fn test() -> i32 { return nonexistent_func(42); }`, "")
}

// ---------------------------------------------------------------------------
// Edge case: multiple return paths
// ---------------------------------------------------------------------------

func TestLowerMultipleReturnPaths(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    if x > 0 {
        return x * 2;
    }
    if x < 0 {
        return -x;
    }
    return 0;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	if len(fn.Body) == 0 {
		t.Error("expected non-empty body for function with multiple returns")
	}
}

// ---------------------------------------------------------------------------
// Large switch with many cases
// ---------------------------------------------------------------------------

func TestLowerSwitchManyCase(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    switch x {
        case 0: { return 100; }
        case 1: { return 200; }
        case 2: { return 300; }
        case 3: { return 400; }
        case 4: { return 500; }
        default: { return 0; }
    }
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	hasSw := false
	for _, stmt := range fn.Body {
		if sw, ok := stmt.Kind.(ir.StmtSwitch); ok {
			hasSw = true
			if len(sw.Cases) < 6 {
				t.Errorf("expected at least 6 switch cases, got %d", len(sw.Cases))
			}
		}
	}
	if !hasSw {
		t.Error("expected switch statement")
	}
}

// ---------------------------------------------------------------------------
// Discard statement (fragment shader)
// ---------------------------------------------------------------------------

func TestLowerDiscardStatement(t *testing.T) {
	src := `@fragment
fn main(@location(0) alpha: f32) -> @location(0) vec4<f32> {
    if alpha < 0.5 {
        discard;
    }
    return vec4<f32>(1.0, 1.0, 1.0, alpha);
}`
	module := mustCompile(t, src)
	ep := module.EntryPoints[0]
	// Should contain StmtKill (discard) inside the if
	hasKill := false
	var findKill func(stmts []ir.Statement)
	findKill = func(stmts []ir.Statement) {
		for _, stmt := range stmts {
			if _, ok := stmt.Kind.(ir.StmtKill); ok {
				hasKill = true
				return
			}
			if ifStmt, ok := stmt.Kind.(ir.StmtIf); ok {
				findKill(ifStmt.Accept)
				findKill(ifStmt.Reject)
			}
		}
	}
	findKill(ep.Function.Body)
	if !hasKill {
		t.Error("expected StmtKill for discard statement")
	}
}

// ---------------------------------------------------------------------------
// phony assignment (exercises tryConstEvalPhonyExpr)
// ---------------------------------------------------------------------------

func TestLowerPhonyAssignments(t *testing.T) {
	src := `fn test(x: f32) {
    _ = x;
    _ = 42;
    _ = vec3<f32>(1.0, 2.0, 3.0);
    _ = sin(x);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeLiteralValue — abstract→concrete in select result
// ---------------------------------------------------------------------------

func TestLowerConcretizeLiteralValueInSelect(t *testing.T) {
	src := `fn test() {
    _ = select(1, 2f, false);
    _ = select(1, 2f, true);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// literalScalarType — scalar type from literal
// ---------------------------------------------------------------------------

func TestLowerLiteralBoolTypes(t *testing.T) {
	src := `fn test() {
    const T: bool = true;
    const F: bool = false;
    var a = T; var b = F;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstU32Expr — constant u32 expression evaluation (used for array sizes)
// ---------------------------------------------------------------------------

func TestLowerEvalConstU32ExprForArraySize(t *testing.T) {
	src := `const N: u32 = 4u;
const M: u32 = N + 4u;
var<private> data: array<f32, M>;
fn test() -> f32 { return data[0]; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Compound assignment operators (+=, -=, *=, /=, %=, &=, |=, ^=)
// ---------------------------------------------------------------------------

func TestLowerCompoundAssignAllOps(t *testing.T) {
	src := `fn test() {
    var a: i32 = 10;
    a += 5;
    a -= 3;
    a *= 2;
    a /= 4;
    a %= 3;
    a &= 0xFF;
    a |= 0x0F;
    a ^= 0xAA;
    _ = a;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Increment/decrement operators
// ---------------------------------------------------------------------------

func TestLowerIncrementDecrement(t *testing.T) {
	src := `fn test() {
    var x: i32 = 0;
    x++;
    x++;
    x--;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Ternary-like select with runtime args
// ---------------------------------------------------------------------------

func TestLowerRuntimeSelect(t *testing.T) {
	src := `fn test(a: f32, b: f32, cond: bool) -> f32 {
    return select(a, b, cond);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeAbstractToDefaultFloat — abstract float default
// ---------------------------------------------------------------------------

func TestLowerAbstractFloatDefaultInMath(t *testing.T) {
	// Abstract literals used directly in math builtins should concretize to f32
	src := `fn test() -> f32 {
    return sin(0.5) + cos(0.0) + sqrt(2.0);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Complex nested expressions — chains of operations
// ---------------------------------------------------------------------------

func TestLowerComplexExpressionChain(t *testing.T) {
	src := `fn test(a: f32, b: f32, c: f32) -> f32 {
    return (a * b + c) / (a - b) + sin(a * c);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Global var with struct init (exercises buildGlobalExprFromAST deeply)
// ---------------------------------------------------------------------------

func TestLowerGlobalVarStructInit(t *testing.T) {
	src := `struct Light {
    position: vec3<f32>,
    color: vec3<f32>,
    intensity: f32,
}
var<private> main_light: Light = Light(
    vec3<f32>(0.0, 10.0, 0.0),
    vec3<f32>(1.0, 1.0, 1.0),
    1.0
);
fn test() -> f32 { return main_light.intensity; }`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) == 0 {
		t.Error("expected global variable for struct init")
	}
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions for struct constructor init")
	}
}

// ---------------------------------------------------------------------------
// Binding attributes — group/binding on storage buffers
// ---------------------------------------------------------------------------

func TestLowerStorageBufferBinding(t *testing.T) {
	src := `struct Data { values: array<f32> }
@group(0) @binding(0) var<storage, read> data: Data;
@compute @workgroup_size(1)
fn main() {
    let v = data.values[0];
    _ = v;
}`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "data" && gv.Space == ir.SpaceStorage {
			found = true
			if gv.Binding == nil {
				t.Error("expected binding for storage buffer")
			}
		}
	}
	if !found {
		t.Error("expected storage buffer variable")
	}
}

// ---------------------------------------------------------------------------
// Array with runtime size in storage buffer
// ---------------------------------------------------------------------------

func TestLowerRuntimeSizedArrayAccess(t *testing.T) {
	src := `struct Buffer { data: array<f32> }
@group(0) @binding(0) var<storage, read> buf: Buffer;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let val = buf.data[gid.x];
    _ = val;
}`
	mustCompile(t, src)
}
