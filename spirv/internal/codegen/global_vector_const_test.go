package codegen

import (
	"fmt"
	"math"
	"strings"
	"testing"
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
