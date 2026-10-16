package codegen

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// A module-scope vector value used as an *operand* used to reach the shader as
// (0, 0, 0): the WGSL front-end put the initializer in Module.GlobalExpressions
// and left Constant.Value nil, and the SPIR-V backend answered nil with
// OpConstantNull. Nothing reported it — the shader compiled and every color
// authored that way was black.
//
// These tests pin the whole matrix: `const` and `var<private>`, vec2/vec3/vec4,
// used as an operand rather than returned directly.

// globalVectorOperandShader wraps a module-scope declaration in a fragment shader
// that uses the value as an operand to mix() — the form that used to lose it.
func globalVectorOperandShader(decl, size, use string) string {
	return fmt.Sprintf(`%s
@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let v = mix(%s, %s, 0.5);
    return vec4<f32>(v.x, v.x, v.x, 1.0);
}
`, decl, swizzleOf("p", size), use)
}

func swizzleOf(base, size string) string {
	switch size {
	case "vec2<f32>":
		return base + ".xy"
	case "vec3<f32>":
		return base + ".xyz"
	default:
		return base
	}
}

func TestGlobalVectorConstReachesSPIRVAsOperand(t *testing.T) {
	cases := []struct {
		name       string
		size       string
		components []float32
	}{
		{"vec2", "vec2<f32>", []float32{0.577, 0.156}},
		{"vec3", "vec3<f32>", []float32{0.577, 0.156, 0.021}},
		{"vec4", "vec4<f32>", []float32{0.577, 0.156, 0.021, 0.937}},
	}

	for _, tc := range cases {
		literal := fmt.Sprintf("%s(%s)", tc.size, floatList(tc.components))

		forms := []struct {
			form string
			decl string
		}{
			{"const", fmt.Sprintf("const K: %s = %s;", tc.size, literal)},
			{"var_private", fmt.Sprintf("var<private> K: %s = %s;", tc.size, literal)},
		}

		for _, f := range forms {
			t.Run(tc.name+"/"+f.form, func(t *testing.T) {
				src := globalVectorOperandShader(f.decl, tc.size, "K")
				spirvBytes := compileWGSLToSPIRV(t, tc.name+"/"+f.form, src)
				assertVectorConstantMaterialized(t, spirvBytes, tc.components)
			})
		}

		t.Run(tc.name+"/const_via_let", func(t *testing.T) {
			decl := fmt.Sprintf("const K: %s = %s;", tc.size, literal)
			src := fmt.Sprintf(`%s
@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let k = K;
    let v = mix(%s, k, 0.5);
    return vec4<f32>(v.x, v.x, v.x, 1.0);
}
`, decl, swizzleOf("p", tc.size))
			spirvBytes := compileWGSLToSPIRV(t, tc.name+"/const_via_let", src)
			assertVectorConstantMaterialized(t, spirvBytes, tc.components)
		})
	}
}

// TestGlobalVectorSplatConstReachesSPIRV covers the pbr.wgsl shape:
// a splat constructor (one scalar broadcast across the vector) used in mix().
func TestGlobalVectorSplatConstReachesSPIRV(t *testing.T) {
	const src = `
const DIELECTRIC_F0: vec3<f32> = vec3<f32>(0.04);

@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let f0 = mix(DIELECTRIC_F0, p.xyz, 0.5);
    return vec4<f32>(f0, 1.0);
}
`
	spirvBytes := compileWGSLToSPIRV(t, "splat", src)
	assertVectorConstantMaterialized(t, spirvBytes, []float32{0.04, 0.04, 0.04})
}

// TestGlobalMatrixConstReachesSPIRV covers a module-scope matrix operand, which
// goes through nested OpConstantComposite (columns, then the matrix).
func TestGlobalMatrixConstReachesSPIRV(t *testing.T) {
	const src = `
const M: mat2x2<f32> = mat2x2<f32>(0.577, 0.156, 0.021, 0.937);

@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let v = M * p.xy;
    return vec4<f32>(v.x, v.y, 0.0, 1.0);
}
`
	spirvBytes := compileWGSLToSPIRV(t, "mat2x2", src)
	assertVectorConstantMaterialized(t, spirvBytes, []float32{0.577, 0.156, 0.021, 0.937})
}

// assertVectorConstantMaterialized checks that every component float was emitted as
// an OpConstant and that some OpConstantComposite gathers them in order.
func assertVectorConstantMaterialized(t *testing.T, spirvBytes []byte, components []float32) {
	t.Helper()

	instrs := decodeSPIRVInstructions(spirvBytes)

	// bits -> the OpConstant IDs carrying that value.
	byBits := map[uint32][]uint32{}
	for _, in := range instrs {
		if in.opcode == OpConstant && in.wordCount == 4 {
			byBits[in.words[3]] = append(byBits[in.words[3]], in.words[2])
		}
	}

	var missing []string
	for _, c := range components {
		if len(byBits[math.Float32bits(c)]) == 0 {
			missing = append(missing, fmt.Sprintf("%v", c))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("component floats never emitted as OpConstant: %s\n"+
			"the module-scope value reached the shader zeroed\n%s",
			strings.Join(missing, ", "), disassembleSPIRV(spirvBytes))
	}

	wanted := make([]uint32, len(components))
	for i, c := range components {
		wanted[i] = math.Float32bits(c)
	}
	if !hasCompositeOver(instrs, byBits, wanted) {
		t.Fatalf("no OpConstantComposite gathers %v in order\n%s",
			components, disassembleSPIRV(spirvBytes))
	}
}

// hasCompositeOver reports whether some OpConstantComposite has constituents whose
// values are exactly wanted, in order. Matrices nest, so a composite of composites
// counts when the flattened leaves match.
func hasCompositeOver(instrs []spirvInstruction, byBits map[uint32][]uint32, wanted []uint32) bool {
	// id -> the constant value bits it holds (scalars only).
	valueOf := map[uint32]uint32{}
	for bits, ids := range byBits {
		for _, id := range ids {
			valueOf[id] = bits
		}
	}

	composites := map[uint32][]uint32{}
	for _, in := range instrs {
		if in.opcode == OpConstantComposite && in.wordCount >= 3 {
			composites[in.words[2]] = in.words[3:]
		}
	}

	var leaves func(id uint32, depth int) ([]uint32, bool)
	leaves = func(id uint32, depth int) ([]uint32, bool) {
		if depth > 4 {
			return nil, false
		}
		if bits, ok := valueOf[id]; ok {
			return []uint32{bits}, true
		}
		parts, ok := composites[id]
		if !ok {
			return nil, false
		}
		var out []uint32
		for _, p := range parts {
			sub, ok := leaves(p, depth+1)
			if !ok {
				return nil, false
			}
			out = append(out, sub...)
		}
		return out, true
	}

	for id := range composites {
		got, ok := leaves(id, 0)
		if !ok || len(got) != len(wanted) {
			continue
		}
		match := true
		for i := range got {
			if got[i] != wanted[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func floatList(values []float32) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return strings.Join(parts, ", ")
}

// The WGSL front-end does not yet produce a Splat, a vector ZeroValue or a
// vector Constant as a component of a module-scope vector, so these tests build
// the IR by hand, in the shape Rust naga's front-end gives those initializers.

// Type handles shared by the hand-built modules below.
const (
	tF32  ir.TypeHandle = 0
	tVec2 ir.TypeHandle = 1
	tVec3 ir.TypeHandle = 2
	tVec4 ir.TypeHandle = 3
)

// globalInitModule wraps a global expression arena, its constants and its
// private variables in an otherwise empty compute shader.
func globalInitModule(exprs []ir.Expression, constants []ir.Constant, globals []ir.GlobalVariable) *ir.Module {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	return &ir.Module{
		Types: []ir.Type{
			{Inner: f32},
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}},
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		},
		Constants:         constants,
		GlobalVariables:   globals,
		GlobalExpressions: exprs,
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{1, 1, 1},
		}},
	}
}

func lit(v float32) ir.Expression {
	return ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(v)}}
}

func compileGlobalInitModule(t *testing.T, mod *ir.Module) []byte {
	t.Helper()
	spirvBytes, err := NewBackend(DefaultOptions()).Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	validateWithVulkanSDK(t, spirvBytes)
	return spirvBytes
}

// assertCompositeLanes checks that some OpConstantComposite has exactly the
// given lanes, in order: a float, or "null" for an OpConstantNull.
func assertCompositeLanes(t *testing.T, spirvBytes []byte, lanes ...any) uint32 {
	t.Helper()
	instrs := decodeSPIRVInstructions(spirvBytes)
	laneOf := map[uint32]string{}
	for _, in := range instrs {
		switch {
		case in.opcode == OpConstant && in.wordCount == 4:
			laneOf[in.words[2]] = fmt.Sprint(math.Float32frombits(in.words[3]))
		case in.opcode == OpConstantNull:
			laneOf[in.words[2]] = "null"
		}
	}
	want := make([]string, len(lanes))
	for i, lane := range lanes {
		want[i] = fmt.Sprint(lane)
	}
	for _, in := range instrs {
		if in.opcode != OpConstantComposite || len(in.words)-3 != len(want) {
			continue
		}
		match := true
		for i, constituent := range in.words[3:] {
			if laneOf[constituent] != want[i] {
				match = false
				break
			}
		}
		if match {
			return in.words[2]
		}
	}
	t.Fatalf("no OpConstantComposite with lanes %v\n%s", want, disassembleSPIRV(spirvBytes))
	return 0
}

// TestGlobalVectorSplatComponent covers vec4(vec3(0.25), 0.75): a Splat is a
// component of the vector, so it contributes one lane per component.
func TestGlobalVectorSplatComponent(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		lit(0.25),
		{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 0}},
		lit(0.75),
		{Kind: ir.ExprCompose{Type: tVec4, Components: []ir.ExpressionHandle{1, 2}}},
	}, []ir.Constant{{Name: "K", Type: tVec4, Init: 3}}, nil)

	spirvBytes := compileGlobalInitModule(t, mod)
	assertCompositeLanes(t, spirvBytes, 0.25, 0.25, 0.25, 0.75)
}

// TestGlobalVectorZeroValueComponent covers vec4(vec2<f32>(), 0.5, 0.75): a
// vector ZeroValue has no scalar expressions, so its lanes are scalar nulls.
func TestGlobalVectorZeroValueComponent(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		{Kind: ir.ExprZeroValue{Type: tVec2}},
		lit(0.5),
		lit(0.75),
		{Kind: ir.ExprCompose{Type: tVec4, Components: []ir.ExpressionHandle{0, 1, 2}}},
	}, []ir.Constant{{Name: "K", Type: tVec4, Init: 3}}, nil)

	spirvBytes := compileGlobalInitModule(t, mod)
	assertCompositeLanes(t, spirvBytes, "null", "null", 0.5, 0.75)
}

// TestGlobalVectorConstantComponent covers const A = vec2(0.125, 0.375);
// const B = vec3(A, 0.625): a vector Constant expands into the lanes of its
// initializer.
func TestGlobalVectorConstantComponent(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		lit(0.125),
		lit(0.375),
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{0, 1}}},
		{Kind: ir.ExprConstant{Constant: 0}},
		lit(0.625),
		{Kind: ir.ExprCompose{Type: tVec3, Components: []ir.ExpressionHandle{3, 4}}},
	}, []ir.Constant{
		{Name: "A", Type: tVec2, Init: 2},
		{Name: "B", Type: tVec3, Init: 5},
	}, nil)

	spirvBytes := compileGlobalInitModule(t, mod)
	assertCompositeLanes(t, spirvBytes, 0.125, 0.375)
	assertCompositeLanes(t, spirvBytes, 0.125, 0.375, 0.625)
}

// TestGlobalVarComposedInitializer covers var<private> V = vec4(A, f32(), 0.875)
// with A a vector constant, and checks the variable is initialized with it.
func TestGlobalVarComposedInitializer(t *testing.T) {
	init := ir.ExpressionHandle(6)
	mod := globalInitModule([]ir.Expression{
		lit(0.125),
		lit(0.375),
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{0, 1}}},
		{Kind: ir.ExprConstant{Constant: 0}},
		{Kind: ir.ExprZeroValue{Type: tF32}},
		lit(0.875),
		{Kind: ir.ExprCompose{Type: tVec4, Components: []ir.ExpressionHandle{3, 4, 5}}},
	}, []ir.Constant{{Name: "A", Type: tVec2, Init: 2}}, []ir.GlobalVariable{
		{Name: "V", Space: ir.SpacePrivate, Type: tVec4, InitExpr: &init},
	})

	spirvBytes := compileGlobalInitModule(t, mod)
	compositeID := assertCompositeLanes(t, spirvBytes, 0.125, 0.375, "null", 0.875)

	for _, in := range decodeSPIRVInstructions(spirvBytes) {
		if in.opcode == OpVariable && StorageClass(in.words[3]) == StorageClassPrivate {
			if in.wordCount != 5 || in.words[4] != compositeID {
				t.Fatalf("private variable is not initialized with %%%d\n%s", compositeID, disassembleSPIRV(spirvBytes))
			}
			return
		}
	}
	t.Fatalf("no private OpVariable\n%s", disassembleSPIRV(spirvBytes))
}

// TestGlobalInitOverrideIsAnError checks that an initializer depending on an
// unprocessed override is reported instead of becoming a silent null.
func TestGlobalInitOverrideIsAnError(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		{Kind: ir.ExprOverride{Override: 0}},
		lit(0.5),
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{0, 1}}},
	}, []ir.Constant{{Name: "K", Type: tVec2, Init: 2}}, nil)
	mod.Overrides = []ir.Override{{Name: "o", Ty: tF32}}

	_, err := NewBackend(DefaultOptions()).Compile(mod)
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Fatalf("Compile error = %v, want an override error", err)
	}
}

// TestGlobalInitOverrideDefaultIsNotAnError checks that an override's own
// default expression, which no constant or variable uses, does not stop the
// module from compiling.
func TestGlobalInitOverrideDefaultIsNotAnError(t *testing.T) {
	def := ir.ExpressionHandle(2)
	mod := globalInitModule([]ir.Expression{
		{Kind: ir.ExprOverride{Override: 0}},
		lit(2),
		{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 0, Right: 1}},
	}, nil, nil)
	mod.Overrides = []ir.Override{{Name: "a", Ty: tF32}, {Name: "b", Ty: tF32, Init: &def}}

	compileGlobalInitModule(t, mod)
}

// TestGlobalInitUnfoldedKeepsFallback pins the fallback kept for a Unary the
// front-end has not folded yet, such as vec2<f16>(-1.0h, 2.0h): a constant
// becomes OpConstantNull and a variable has no initializer, rather than the
// valid shader being rejected.
func TestGlobalInitUnfoldedKeepsFallback(t *testing.T) {
	init := ir.ExpressionHandle(3)
	mod := globalInitModule([]ir.Expression{
		lit(1),
		{Kind: ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0}},
		lit(2),
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{1, 2}}},
	}, []ir.Constant{{Name: "K", Type: tVec2, Init: 3}}, []ir.GlobalVariable{
		{Name: "V", Space: ir.SpacePrivate, Type: tVec2, InitExpr: &init},
	})

	spirvBytes := compileGlobalInitModule(t, mod)
	for _, in := range decodeSPIRVInstructions(spirvBytes) {
		if in.opcode == OpVariable && StorageClass(in.words[3]) == StorageClassPrivate && in.wordCount != 4 {
			t.Fatalf("private variable has an initializer\n%s", disassembleSPIRV(spirvBytes))
		}
	}
}

// TestGlobalInitMalformedVectorIsAnError checks that a vector whose components
// do not add up to its size is reported, not emitted as invalid SPIR-V.
func TestGlobalInitMalformedVectorIsAnError(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		lit(0.5),
		{Kind: ir.ExprCompose{Type: tVec3, Components: []ir.ExpressionHandle{0}}},
		lit(0.75),
		{Kind: ir.ExprCompose{Type: tVec4, Components: []ir.ExpressionHandle{1, 2}}},
	}, []ir.Constant{{Name: "K", Type: tVec4, Init: 3}}, nil)

	_, err := NewBackend(DefaultOptions()).Compile(mod)
	if err == nil || !strings.Contains(err.Error(), "has 2 components, want 4") {
		t.Fatalf("Compile error = %v, want a component count error", err)
	}
}

// TestGlobalInitForwardOperand covers the arena ir.ProcessOverrides leaves:
// an override's value appended after the expression that now refers to it.
func TestGlobalInitForwardOperand(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		{Kind: ir.ExprConstant{Constant: 0}},
		lit(0.5),
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{0, 1}}},
		lit(0.25),
	}, []ir.Constant{
		{Name: "o", Type: tF32, Init: 3},
		{Name: "K", Type: tVec2, Init: 2},
	}, nil)

	spirvBytes := compileGlobalInitModule(t, mod)
	assertCompositeLanes(t, spirvBytes, 0.25, 0.5)
}

// TestGlobalInitCycleIsAnError checks that an arena whose expression reaches
// itself is reported instead of recursing.
func TestGlobalInitCycleIsAnError(t *testing.T) {
	mod := globalInitModule([]ir.Expression{
		{Kind: ir.ExprConstant{Constant: 0}},
		lit(0.5),
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{0, 1}}},
	}, []ir.Constant{{Name: "K", Type: tVec2, Init: 2}}, nil)

	_, err := NewBackend(DefaultOptions()).Compile(mod)
	if err == nil || !strings.Contains(err.Error(), "refers to itself") {
		t.Fatalf("Compile error = %v, want a cycle error", err)
	}
}
