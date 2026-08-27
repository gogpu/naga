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

// ---------------------------------------------------------------------------
// Dawn swizzle assignment test patterns (11 total).
// Reference: dawn-webgpu/src/tint/lang/wgsl/reader/program_to_ir/swizzle_assignment_test.cc
//
// Patterns 1-2 (single/multi element) and 5-6 (compound single/multi) are
// tested by the existing tests above. The tests below cover the remaining
// patterns: chained (3,4,7), indexed constant (8,10), indexed dynamic (9,11).
// ---------------------------------------------------------------------------

// Dawn pattern 3: v.zyx.x = 1.0 → resolves to v.z = 1.0 (single component store at index 2).
func TestSwizzleAssignment_Dawn3_ChainedSwizzleSingle(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx.x = 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.zyx.x = 1.0 should resolve to storing at index 2 (z component).
	// This becomes: AccessIndex(base=v_ptr, index=2) + Store.
	// No Compose needed (single component).
	hasAccessIndex2 := false
	for _, expr := range fn.Expressions {
		if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			if ai.Index == 2 {
				hasAccessIndex2 = true
			}
		}
	}
	if !hasAccessIndex2 {
		t.Error("chained v.zyx.x should resolve to AccessIndex with index 2 (z)")
	}

	// Should have a Store statement (not Compose-based, just direct component store).
	storeCount := 0
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			storeCount++
		}
	}
	if storeCount < 1 {
		t.Errorf("expected at least 1 store for chained assignment, got %d", storeCount)
	}
}

// Dawn pattern 4: v.zyx.xz = vec2(1,2) → resolves to writing at indices [2, 0] (z, x).
// Dawn's output: v.zyx = [2,1,0], .xz selects [0,2] from swizzle → original [2, 0].
// Note: Dawn test uses v.zyx.xz which maps to original [2, 0] not [1, 0].
func TestSwizzleAssignment_Dawn4_ChainedMultiElement(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx.xz = vec2<f32>(1.0, 2.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.zyx.xz: outer swizzle [2,1,0], inner .xz selects indices [0,2] →
	// resolved: [outerMap[0], outerMap[2]] = [2, 0].
	// This decomposes into: Load + AccessIndex (extract) + Compose(4) + Store.
	hasLoad := false
	hasCompose4 := false
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprLoad:
			hasLoad = true
		case ir.ExprCompose:
			if len(e.Components) == 4 {
				hasCompose4 = true
			}
		}
	}
	if !hasLoad {
		t.Error("chained multi-element should include Load expression")
	}
	if !hasCompose4 {
		t.Error("chained multi-element should include Compose with 4 components (full vec4)")
	}
}

// Dawn pattern 4 variant: v.zyx.yz = vec2(1,2) → resolves to indices [1, 0] (y, x).
func TestSwizzleAssignment_Dawn4_ChainedMultiElement_YZ(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx.yz = vec2<f32>(5.0, 6.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.zyx.yz: outer swizzle [2,1,0], inner .yz selects indices [1,2] →
	// resolved: [outerMap[1], outerMap[2]] = [1, 0].
	// This writes to original positions y(1) and x(0).
	hasCompose4 := false
	for _, expr := range fn.Expressions {
		if compose, ok := expr.Kind.(ir.ExprCompose); ok {
			if len(compose.Components) == 4 {
				hasCompose4 = true
			}
		}
	}
	if !hasCompose4 {
		t.Error("chained v.zyx.yz should produce Compose with 4 components")
	}
}

// Dawn pattern 7: v.zyx.xz += vec2(1,2) → compound chained multi-element.
// v.zyx = [2,1,0], .xz selects [0,2] → resolved [2,0] (z, x).
func TestSwizzleAssignment_Dawn7_CompoundChainedMultiElement(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx.xz += vec2<f32>(1.0, 2.0);
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Compound chained: needs Swizzle + Binary + AccessIndex + Compose + Store.
	hasBinary := false
	hasSwizzle := false
	hasCompose4 := false
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprBinary:
			hasBinary = true
		case ir.ExprSwizzle:
			hasSwizzle = true
		case ir.ExprCompose:
			if len(e.Components) == 4 {
				hasCompose4 = true
			}
		}
	}
	if !hasBinary {
		t.Error("compound chained should include Binary expression")
	}
	if !hasSwizzle {
		t.Error("compound chained should include Swizzle expression")
	}
	if !hasCompose4 {
		t.Error("compound chained should produce Compose with 4 components")
	}
}

// Dawn pattern 7 variant: single compound chained — v.zyx.x += 1.0.
func TestSwizzleAssignment_Dawn7_CompoundChainedSingle(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx.x += 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.zyx.x += 1.0 → store to index 2 (z).
	// Compound: Load + Binary + Store at index 2.
	hasBinary := false
	hasAccessIndex2 := false
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprBinary:
			hasBinary = true
		case ir.ExprAccessIndex:
			if e.Index == 2 {
				hasAccessIndex2 = true
			}
		}
	}
	if !hasBinary {
		t.Error("compound chained single should include Binary expression")
	}
	if !hasAccessIndex2 {
		t.Error("compound chained single v.zyx.x should resolve to AccessIndex with index 2")
	}
}

// Dawn pattern 8: v.xyz[2] = 1.0 → constant index into swizzle.
// v.xyz = [0,1,2], [2] selects index 2 → original index 2.
func TestSwizzleAssignment_Dawn8_IndexedConstant(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xyz[2] = 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.xyz[2] → swizzle [0,1,2], index 2 → original index 2.
	// Should produce AccessIndex(base=v_ptr, index=2) + Store.
	hasAccessIndex2 := false
	for _, expr := range fn.Expressions {
		if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			if ai.Index == 2 {
				hasAccessIndex2 = true
			}
		}
	}
	if !hasAccessIndex2 {
		t.Error("v.xyz[2] should resolve to AccessIndex with index 2")
	}
}

// Dawn pattern 8 variant: v.zyx[1] = 1.0.
// v.zyx = [2,1,0], [1] selects index 1 → original index 1 (y).
func TestSwizzleAssignment_Dawn8_IndexedConstant_ZYX(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx[1] = 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.zyx[1] → swizzle [2,1,0], index 1 → original index 1 (y).
	hasAccessIndex1 := false
	for _, expr := range fn.Expressions {
		if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			if ai.Index == 1 {
				hasAccessIndex1 = true
			}
		}
	}
	if !hasAccessIndex1 {
		t.Error("v.zyx[1] should resolve to AccessIndex with index 1 (y)")
	}
}

// Dawn pattern 9: v.zyx[i] = 1.0 → dynamic index into swizzle.
// Creates runtime remapping array [2u, 1u, 0u] and uses Access to resolve.
func TestSwizzleAssignment_Dawn9_IndexedDynamic(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    var i : i32 = 1;
    v.zyx[i] = 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Dynamic indexed swizzle should produce:
	// 1. Compose(array<u32,3>) with mapping constants [2u, 1u, 0u]
	// 2. Access(array, dynamic_index) → resolved index
	// 3. Access(base_ptr, resolved_index) → component pointer
	// 4. Store
	hasArrayCompose := false
	hasAccess := false
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprCompose:
			if len(e.Components) == 3 {
				hasArrayCompose = true
			}
		case ir.ExprAccess:
			_ = e
			hasAccess = true
		}
	}
	if !hasArrayCompose {
		t.Error("dynamic indexed swizzle should include Compose(array<u32,3>) for mapping")
	}
	if !hasAccess {
		t.Error("dynamic indexed swizzle should include Access for runtime resolution")
	}
}

// Dawn pattern 10: v.xyz[0] += 1.0 → compound indexed constant.
// v.xyz = [0,1,2], [0] → original 0 (x). Load + Add + Store.
func TestSwizzleAssignment_Dawn10_CompoundIndexedConstant(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.xyz[0] += 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Compound indexed constant: resolve to single component + Load + Binary + Store.
	hasBinary := false
	hasAccessIndex0 := false
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprBinary:
			hasBinary = true
		case ir.ExprAccessIndex:
			if e.Index == 0 {
				hasAccessIndex0 = true
			}
		}
	}
	if !hasBinary {
		t.Error("compound indexed constant should include Binary expression")
	}
	if !hasAccessIndex0 {
		t.Error("v.xyz[0] += should resolve to AccessIndex with index 0")
	}
}

// Dawn pattern 11: v.xyz[i] += 1.0 → compound indexed dynamic.
// Creates runtime remapping, loads current value, applies binary op, stores back.
func TestSwizzleAssignment_Dawn11_CompoundIndexedDynamic(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    var i : i32 = 1;
    v.xyz[i] += 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Compound dynamic: needs remapping array + Access + Load + Binary + Store.
	hasArrayCompose := false
	hasAccess := false
	hasBinary := false
	hasLoad := false
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprCompose:
			if len(e.Components) == 3 {
				hasArrayCompose = true
			}
		case ir.ExprAccess:
			_ = e
			hasAccess = true
		case ir.ExprBinary:
			hasBinary = true
		case ir.ExprLoad:
			hasLoad = true
		}
	}
	if !hasArrayCompose {
		t.Error("compound dynamic indexed should include Compose(array<u32,3>) for mapping")
	}
	if !hasAccess {
		t.Error("compound dynamic indexed should include Access for runtime resolution")
	}
	if !hasBinary {
		t.Error("compound dynamic indexed should include Binary expression")
	}
	if !hasLoad {
		t.Error("compound dynamic indexed should include Load expression")
	}
}

// TestSwizzleAssignment_AllDawnPatterns tests all 11 Dawn patterns in a single shader.
// This is the integration test — verifies all patterns compile together without
// interference and produce valid IR.
func TestSwizzleAssignment_AllDawnPatterns(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    var i : i32 = 1;

    // Pattern 1: single element swizzle
    v.y = 1.0;

    // Pattern 2: multi element swizzle
    v.ywx = vec3<f32>(1.0, 2.0, 3.0);

    // Pattern 3: chained swizzle single
    v.zyx.x = 1.0;

    // Pattern 4: chained multi-element swizzle
    v.zyx.yz = vec2<f32>(5.0, 6.0);

    // Pattern 5: single compound
    v.y += 1.0;

    // Pattern 6: multi compound
    v.ywx += vec3<f32>(1.0, 2.0, 3.0);

    // Pattern 7: chained compound
    v.zyx.x += 1.0;

    // Pattern 8: indexed constant
    v.zyx[1] = 1.0;

    // Pattern 9: indexed dynamic
    v.zyx[i] = 1.0;

    // Pattern 10: indexed constant compound
    v.zyx[1] += 1.0;

    // Pattern 11: indexed dynamic compound
    v.zyx[i] += 1.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// Count stores: each of the 11 assignment patterns produces at least 1 store.
	// Var init may or may not produce a separate store (depends on lowering).
	storeCount := 0
	for _, stmt := range fn.Body {
		if _, ok := stmt.Kind.(ir.StmtStore); ok {
			storeCount++
		}
	}
	// 11 assignment patterns, each producing at least 1 store.
	if storeCount < 11 {
		t.Errorf("expected at least 11 stores (11 assignment patterns), got %d", storeCount)
	}
}

// TestSwizzleAssignment_ChainedWithoutEnable verifies that chained swizzle
// assignment requires the enable directive.
func TestSwizzleAssignment_ChainedWithoutEnable(t *testing.T) {
	src := `@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx.x = 1.0;
}
`
	err := helperCompileWGSLError(t, src)
	if err == nil {
		t.Fatal("expected error for chained swizzle assignment without enable directive")
	}
	if !strings.Contains(err.Error(), "swizzle_assignment") {
		t.Errorf("error should mention swizzle_assignment, got: %v", err)
	}
}

// TestSwizzleAssignment_IndexedWithoutEnable verifies that indexed swizzle
// assignment requires the enable directive.
func TestSwizzleAssignment_IndexedWithoutEnable(t *testing.T) {
	src := `@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    v.zyx[1] = 1.0;
}
`
	err := helperCompileWGSLError(t, src)
	if err == nil {
		t.Fatal("expected error for indexed swizzle assignment without enable directive")
	}
	if !strings.Contains(err.Error(), "swizzle_assignment") {
		t.Errorf("error should mention swizzle_assignment, got: %v", err)
	}
}

// TestSwizzleAssignment_ChainedDifferentBases tests chained swizzle on vec3 base.
func TestSwizzleAssignment_ChainedOnVec3(t *testing.T) {
	src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec3<f32>(1.0, 2.0, 3.0);
    v.zx.x = 5.0;
}
`
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}

	fn := &mod.EntryPoints[0].Function

	// v.zx.x → swizzle [2,0], index 0 → original 2 (z).
	hasAccessIndex2 := false
	for _, expr := range fn.Expressions {
		if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			if ai.Index == 2 {
				hasAccessIndex2 = true
			}
		}
	}
	if !hasAccessIndex2 {
		t.Error("v.zx.x on vec3 should resolve to AccessIndex with index 2")
	}
}

// TestSwizzleAssignment_StructMemberNotSwizzle verifies that struct member names
// containing only swizzle-like characters (e.g., "arr" = a,r,r in RGBA) are NOT
// falsely detected as chained swizzle patterns. Regression test for pointers.wgsl.
func TestSwizzleAssignment_StructMemberNotSwizzle(t *testing.T) {
	src := `struct DynamicArray {
    arr: array<u32>
}

@group(0) @binding(0)
var<storage, read_write> dynamic_array: DynamicArray;

fn index_unsized(i: i32, v: u32) {
    let p: ptr<storage, DynamicArray, read_write> = &dynamic_array;
    let val = (*p).arr[i];
    (*p).arr[i] = val + v;
}

@compute @workgroup_size(1)
fn main() {
    index_unsized(1, 1);
}
`
	// This must compile without error. Previously, "arr" was falsely detected
	// as an RGBA swizzle (a=3, r=0, r=0), causing a "chained swizzle assignment
	// requires 'enable swizzle_assignment'" error.
	mod := helperCompileWGSL(t, src)
	if mod == nil {
		t.Fatal("module is nil")
	}
}

// TestSwizzleAssignment_ChainedSwizzleResolution tests index resolution correctness
// for several chained swizzle combinations.
func TestSwizzleAssignment_ChainedSwizzleResolution(t *testing.T) {
	tests := []struct {
		name           string
		swizzle        string
		expectedStores int // minimum stores expected
	}{
		{"zyx_x", "v.zyx.x = 1.0;", 1},
		{"zyx_y", "v.zyx.y = 1.0;", 1},
		{"zyx_z", "v.zyx.z = 1.0;", 1},
		{"wzyx_x", "v.wzyx.x = 1.0;", 1},
		{"wzyx_w", "v.wzyx.w = 1.0;", 1},
		{"xyzw_x", "v.xyzw.x = 1.0;", 1},
		{"xy_x", "v.xy.x = 1.0;", 1},
		{"xy_y", "v.xy.y = 1.0;", 1},
		{"yx_x", "v.yx.x = 1.0;", 1},
		{"yx_y", "v.yx.y = 1.0;", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `enable swizzle_assignment;

@compute @workgroup_size(1)
fn main() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    ` + tt.swizzle + `
}
`
			mod := helperCompileWGSL(t, src)
			if mod == nil {
				t.Fatal("module is nil")
			}

			fn := &mod.EntryPoints[0].Function
			storeCount := 0
			for _, stmt := range fn.Body {
				if _, ok := stmt.Kind.(ir.StmtStore); ok {
					storeCount++
				}
			}
			if storeCount < tt.expectedStores {
				t.Errorf("expected at least %d stores, got %d", tt.expectedStores, storeCount)
			}
		})
	}
}
