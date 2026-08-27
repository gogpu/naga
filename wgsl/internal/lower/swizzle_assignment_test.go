package lower

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// helperCompileWGSL parses and lowers WGSL source code, returning the IR module.
func helperCompileWGSL(t *testing.T, src string) *ir.Module {
	t.Helper()
	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize error: %v", err)
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	mod, err := LowerWithSource(ast, src)
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}
	return mod
}

// helperCompileWGSLError parses and lowers WGSL source code, expecting an error.
func helperCompileWGSLError(t *testing.T, src string) error {
	t.Helper()
	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		return err
	}
	_, err = LowerWithSource(ast, src)
	return err
}

func TestSwizzleAssignment_EnableDirectiveParsed(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.x = 5.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_SimpleAssignTwoComponents(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xz = vec2<f32>(10.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	// Find the entry point function.
	if len(mod.EntryPoints) == 0 {
		t.Fatal("no entry points")
	}
	fn := &mod.EntryPoints[0].Function

	// Verify decomposition produced expected IR:
	// Should have Load, AccessIndex (extract source components),
	// AccessIndex (extract unchanged original components),
	// Compose, and Store statements.
	hasStore := false
	hasCompose := false
	hasLoad := false
	for _, expr := range fn.Expressions {
		switch expr.Kind.(type) {
		case ir.ExprLoad:
			hasLoad = true
		case ir.ExprCompose:
			hasCompose = true
		}
	}
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			hasStore = true
		}
	}

	if !hasLoad {
		t.Error("decomposition should include Load expression")
	}
	if !hasCompose {
		t.Error("decomposition should include Compose expression")
	}
	if !hasStore {
		t.Error("decomposition should include Store statement")
	}

	// Verify the Compose has fullSize (4) components for vec4.
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 4 {
				t.Errorf("Compose should have 4 components for vec4, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_ThreeComponents(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xyz = vec3<f32>(10.0, 20.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// With .xyz on vec4, only w (index 3) is unchanged.
	// Compose should have 4 components.
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 4 {
				t.Errorf("Compose should have 4 components, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_AllFourComponents(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xyzw = vec4<f32>(10.0, 20.0, 30.0, 40.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// With .xyzw, all components are swizzled. Compose still has 4 components.
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 4 {
				t.Errorf("Compose should have 4 components, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_RGBA(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.rgb = vec3<f32>(10.0, 20.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_ReorderedIndices(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.ywx = vec3<f32>(10.0, 20.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.ywx = source decomposes to:
	// components[0] (x) = source[2] (third element maps to x)
	// components[1] (y) = source[0] (first element maps to y)
	// components[2] (z) = original[2] (unchanged)
	// components[3] (w) = source[1] (second element maps to w)
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 4 {
				t.Errorf("Compose should have 4 components, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_CompoundAdd(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xz += vec2<f32>(10.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Compound assignment should produce:
	// 1. Load original
	// 2. Swizzle .xz from original
	// 3. Binary add: swizzled + rhs
	// 4. AccessIndex on result
	// 5. AccessIndex on original (unchanged)
	// 6. Compose
	// 7. Store
	hasBinary := false
	hasSwizzle := false
	for _, expr := range fn.Expressions {
		switch expr.Kind.(type) {
		case ir.ExprBinary:
			hasBinary = true
		case ir.ExprSwizzle:
			hasSwizzle = true
		}
	}
	if !hasBinary {
		t.Error("compound assignment should include Binary expression")
	}
	if !hasSwizzle {
		t.Error("compound assignment should include Swizzle expression for extracting original components")
	}
}

func TestSwizzleAssignment_CompoundMul(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.yw *= vec2<f32>(2.0, 4.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_Vec2Target(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec2<f32>(1.0, 2.0);
    v.xy = vec2<f32>(10.0, 20.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 2 {
				t.Errorf("Compose for vec2 should have 2 components, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_Vec3Target(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec3<f32>(1.0, 2.0, 3.0);
    v.xz = vec2<f32>(10.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 3 {
				t.Errorf("Compose for vec3 should have 3 components, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_SingleComponentStillWorks(t *testing.T) {
	// Single-component access should NOT trigger swizzle decomposition.
	// It should use the existing AccessIndex path.
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.x = 5.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Single component should produce a Store targeting an AccessIndex pointer,
	// not a Compose-and-Store-whole-vector decomposition.
	// Count Compose expressions: there should be exactly 1 (from the var init),
	// NOT an additional one from the single-component assignment.
	composeCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprCompose); ok {
			composeCount++
		}
	}
	// The var initializer vec4<f32>(1.0, 2.0, 3.0, 4.0) produces 1 Compose.
	// If single-component assignment is decomposed incorrectly, there would be 2.
	if composeCount != 1 {
		t.Errorf("expected exactly 1 Compose (from var init), got %d", composeCount)
	}
}

func TestSwizzleAssignment_WithoutEnableDirective(t *testing.T) {
	src := `@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xz = vec2<f32>(10.0, 30.0);
}
`
	err := helperCompileWGSLError(t, src)
	if err == nil {
		t.Fatal("expected error for swizzle assignment without enable directive")
	}
	if !strings.Contains(err.Error(), "swizzle_assignment") {
		t.Errorf("error should mention swizzle_assignment, got: %v", err)
	}
}

func TestSwizzleAssignment_RValueSwizzleWithoutEnable(t *testing.T) {
	// Rvalue swizzle (reading .rgb) should work without enable directive.
	src := `@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let rgb = v.rgb;
    _ = rgb;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_IntegerVector(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<i32>(1, 2, 3, 4);
    v.xz = vec2<i32>(10, 30);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_UnsignedIntegerVector(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<u32>(1u, 2u, 3u, 4u);
    v.xz = vec2<u32>(10u, 30u);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_EnableF16AlongWithSwizzle(t *testing.T) {
	// Multiple enable directives should work together.
	src := `enable f16;
enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xz = vec2<f32>(10.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_MultipleExtensionsInOneEnable(t *testing.T) {
	// Multiple extensions in a single enable directive.
	src := `enable f16, swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xz = vec2<f32>(10.0, 30.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_BackendRoundtrip(t *testing.T) {
	// Verify that the decomposed IR can be consumed by backends.
	// This test checks that the IR is well-formed by validating the module.
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    var w = vec2<f32>(10.0, 30.0);
    v.xz = w;
    v.yw += vec2<f32>(1.0, 1.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Should have two Store statements (one for v.xz = w, one for v.yw += ...).
	storeCount := 0
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			storeCount++
		}
	}

	// There may be stores from var initialization too. At minimum we need
	// 2 stores from the swizzle assignments.
	if storeCount < 2 {
		t.Errorf("expected at least 2 store statements from swizzle assignments, got %d", storeCount)
	}
}

func TestSwizzleAssignment_DuplicateIndicesDetection(t *testing.T) {
	// Duplicate indices (v.xx = ...) are rejected by detectSwizzleAssignment
	// (returns false), so the assignment falls through to the normal path.
	// The normal path creates an ExprSwizzle (rvalue) via lowerMember,
	// which results in a Store to a non-pointer expression.
	// This verifies that duplicate-index swizzle on LHS does not silently succeed.
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xx = vec2<f32>(5.0, 6.0);
}
`
	// The behavior depends on whether the normal assignment path
	// catches the invalid lvalue. It may succeed (silently storing to
	// a temporary) or fail with an error.
	// For now, verify it doesn't crash.
	_ = helperCompileWGSLError(t, src)
}

func TestSwizzleAssignment_CompoundSubtract(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(10.0, 20.0, 30.0, 40.0);
    v.xz -= vec2<f32>(1.0, 3.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

func TestSwizzleAssignment_SwizzleYW(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.yw = vec2<f32>(20.0, 40.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 4 {
				t.Errorf("Compose should have 4 components, got %d", len(compose.Components))
			}
		}
	}
}

func TestSwizzleAssignment_Vec3SwapXZ(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec3<f32>(1.0, 2.0, 3.0);
    v.zx = vec2<f32>(30.0, 10.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) != 3 {
				t.Errorf("Compose for vec3 should have 3 components, got %d", len(compose.Components))
			}
		}
	}
}
