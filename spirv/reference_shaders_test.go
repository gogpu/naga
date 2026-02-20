package spirv

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// verifySPIRVOutput performs comprehensive SPIR-V binary validation.
// It checks the magic number, minimum size, and non-trivial output for
// shaders that are not empty stubs.
func verifySPIRVOutput(t *testing.T, name string, spvBytes []byte, minSize int) {
	t.Helper()
	if len(spvBytes) < 20 {
		t.Fatalf("[%s] SPIR-V output too small: %d bytes (minimum 20 for header)", name, len(spvBytes))
	}
	magic := binary.LittleEndian.Uint32(spvBytes[:4])
	if magic != MagicNumber {
		t.Fatalf("[%s] invalid SPIR-V magic: got 0x%08X, want 0x%08X", name, magic, MagicNumber)
	}
	if len(spvBytes) < minSize {
		t.Errorf("[%s] SPIR-V output suspiciously small: %d bytes (expected at least %d)", name, len(spvBytes), minSize)
	}
}

// compileWGSLSource compiles WGSL source code through the full pipeline:
// lex -> parse -> lower -> SPIR-V backend. Returns the SPIR-V binary bytes.
func compileWGSLSource(t *testing.T, name, source string) []byte {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("[%s] Tokenize failed: %v", name, err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("[%s] Parse failed: %v", name, err)
	}

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("[%s] Lower failed: %v", name, err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("[%s] SPIR-V compile failed: %v", name, err)
	}

	return spirvBytes
}

// TestEssentialReferenceShaders tests that all 15 Essential reference shaders from the
// Rust naga test suite compile to valid SPIR-V through our full pipeline.
// This acts as a regression test suite to prevent breakages in the compiler.
//
// Shaders are embedded as string literals so tests work on CI without the reference directory.
func TestEssentialReferenceShaders(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		minSize int // minimum expected SPIR-V output size in bytes
	}{
		{
			name:    "empty",
			minSize: 40,
			source:  refShaderEmpty,
		},
		{
			name:    "constructors",
			minSize: 100,
			source:  refShaderConstructors,
		},
		{
			name:    "operators",
			minSize: 100,
			source:  refShaderOperators,
		},
		{
			name:    "control-flow",
			minSize: 100,
			source:  refShaderControlFlow,
		},
		{
			name:    "functions",
			minSize: 100,
			source:  refShaderFunctions,
		},
		{
			name:    "globals",
			minSize: 100,
			source:  refShaderGlobals,
		},
		{
			name:    "interface",
			minSize: 100,
			source:  refShaderInterface,
		},
		{
			name:    "collatz",
			minSize: 100,
			source:  refShaderCollatz,
		},
		{
			name:    "quad",
			minSize: 100,
			source:  refShaderQuad,
		},
		{
			name:    "image",
			minSize: 100,
			source:  refShaderImage,
		},
		{
			name:    "shadow",
			minSize: 100,
			source:  refShaderShadow,
		},
		{
			name:    "boids",
			minSize: 100,
			source:  refShaderBoids,
		},
		{
			name:    "access",
			minSize: 100,
			source:  refShaderAccess,
		},
		{
			name:    "math-functions",
			minSize: 100,
			source:  refShaderMathFunctions,
		},
		{
			name:    "struct-layout",
			minSize: 100,
			source:  refShaderStructLayout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spirvBytes := compileWGSLSource(t, tt.name, tt.source)
			verifySPIRVOutput(t, tt.name, spirvBytes, tt.minSize)
			validateSPIRVBinary(t, spirvBytes)
			t.Logf("[%s] compiled successfully: %d bytes of SPIR-V", tt.name, len(spirvBytes))
		})
	}
}

// TestBonusReferenceShaders tests additional complex shaders from the wgpu examples
// that exercise advanced WGSL features: texture sampling, matrix math, noise functions.
func TestBonusReferenceShaders(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		minSize int
	}{
		{
			name:    "skybox",
			minSize: 100,
			source:  refShaderSkybox,
		},
		{
			name:    "water",
			minSize: 200,
			source:  refShaderWater,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spirvBytes := compileWGSLSource(t, tt.name, tt.source)
			verifySPIRVOutput(t, tt.name, spirvBytes, tt.minSize)
			validateSPIRVBinary(t, spirvBytes)
			t.Logf("[%s] compiled successfully: %d bytes of SPIR-V", tt.name, len(spirvBytes))
		})
	}
}

// ---------------------------------------------------------------------------
// Embedded reference shader sources
// ---------------------------------------------------------------------------

// refShaderEmpty is the empty compute shader — minimal valid WGSL.
// Source: naga/tests/in/wgsl/empty.wgsl
const refShaderEmpty = `
@compute @workgroup_size(1)
fn main() {}
`

// refShaderConstructors tests zero-value, identity, conversion, and inference constructors.
// Source: naga/tests/in/wgsl/constructors.wgsl
const refShaderConstructors = `
struct Foo {
    a: vec4<f32>,
    b: i32,
}

const const1 = vec3<f32>(0.0);
const const2 = vec3(0.0, 1.0, 2.0);
const const3 = mat2x2<f32>(0.0, 1.0, 2.0, 3.0);
const const4 = array<mat2x2<f32>, 1>(mat2x2<f32>(0.0, 1.0, 2.0, 3.0));

// zero value constructors
const cz0 = bool();
const cz1 = i32();
const cz2 = u32();
const cz3 = f32();
const cz4 = vec2<u32>();
const cz5 = mat2x2<f32>();
const cz6 = array<Foo, 3>();
const cz7 = Foo();

// constructors that infer their type from their parameters
const cp1 = vec2(0u);
const cp2 = mat2x2(vec2(0.), vec2(0.));
const cp3 = array(0, 1, 2, 3);

@compute @workgroup_size(1)
fn main() {
    var foo: Foo;
    foo = Foo(vec4<f32>(1.0), 1);

    let m0 = mat2x2<f32>(
        1.0, 0.0,
        0.0, 1.0,
    );
    let m1 = mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        0.0, 0.0, 0.0, 1.0,
    );

    // zero value constructors
    let zvc0 = bool();
    let zvc1 = i32();
    let zvc2 = u32();
    let zvc3 = f32();
    let zvc4 = vec2<u32>();
    let zvc5 = mat2x2<f32>();
    let zvc6 = array<Foo, 3>();
    let zvc7 = Foo();
    let zvc8: vec2<u32> = vec2();
    let zvc9: vec2<f32> = vec2();

    // constructors that infer their type from their parameters
    let cit0 = vec2(0u);
    let cit1 = mat2x2(vec2(0.), vec2(0.));
    let cit2 = array(0, 1, 2, 3);

    // identity constructors
    let ic0 = bool(bool());
    let ic1 = i32(i32());
    let ic2 = u32(u32());
    let ic3 = f32(f32());
    let ic4 = vec2<u32>(vec2<u32>());
    let ic5 = mat2x3<f32>(mat2x3<f32>());
    let ic6 = vec2(vec2<u32>());
    let ic7 = mat2x3(mat2x3<f32>());

    // conversion constructors
    let cc00 = i32(1u);
    let cc01 = i32(1f);
    let cc02 = i32(1);
    let cc03 = i32(1.0);
    let cc04 = i32(true);
    let cc05 = u32(1i);
    let cc06 = u32(1f);
    let cc07 = u32(1);
    let cc08 = u32(1.0);
    let cc09 = u32(true);
    let cc10 = f32(1i);
    let cc11 = f32(1u);
    let cc12 = f32(1);
    let cc13 = f32(1.0);
    let cc14 = f32(true);
    let cc15 = bool(1i);
    let cc16 = bool(1u);
    let cc17 = bool(1f);
    let cc18 = bool(1);
    let cc19 = bool(1.0);
}
`

// refShaderOperators tests arithmetic, logical, bitwise, comparison, and assignment operators.
// Source: naga/tests/in/wgsl/operators.wgsl
const refShaderOperators = `
const v_f32_one = vec4<f32>(1.0, 1.0, 1.0, 1.0);
const v_f32_zero = vec4<f32>(0.0, 0.0, 0.0, 0.0);
const v_f32_half = vec4<f32>(0.5, 0.5, 0.5, 0.5);
const v_i32_one = vec4<i32>(1, 1, 1, 1);

fn builtins() -> vec4<f32> {
    // select()
    let condition = true;
    let s1 = select(0, 1, condition);
    let s2 = select(v_f32_zero, v_f32_one, condition);
    let s3 = select(v_f32_one, v_f32_zero, vec4<bool>(false, false, false, false));
    // mix()
    let m1 = mix(v_f32_zero, v_f32_one, v_f32_half);
    let m2 = mix(v_f32_zero, v_f32_one, 0.1);
    // bitcast()
    let b1 = bitcast<f32>(v_i32_one.x);
    let b2 = bitcast<vec4<f32>>(v_i32_one);
    // convert
    let v_i32_zero = vec4<i32>(v_f32_zero);
    // done
    return vec4<f32>(vec4<i32>(s1) + v_i32_zero) + s2 + m1 + m2 + b1 + b2;
}

fn splat(m: f32, n: i32) -> vec4<f32> {
    let a = (2.0 + vec2<f32>(m) - 4.0) / 8.0;
    let b = vec4<i32>(n) % 2;
    return a.xyxy + vec4<f32>(b);
}

fn splat_assignment() -> vec2<f32> {
    var a = vec2<f32>(2.0);
    a += 1.0;
    a -= 3.0;
    a /= 4.0;
    return a;
}

fn bool_cast(x: vec3<f32>) -> vec3<f32> {
    let y = vec3<bool>(x);
    return vec3<f32>(y);
}

fn p() -> bool { return true; }
fn q() -> bool { return false; }
fn r() -> bool { return true; }
fn s() -> bool { return false; }

fn logical() {
    let t = true;
    let f = false;

    // unary
    let neg0 = !t;
    let neg1 = !vec2(t);

    // binary
    let or = t || f;
    let and = t && f;
    let bitwise_or0 = t | f;
    let bitwise_or1 = vec3(t) | vec3(f);
    let bitwise_and0 = t & f;
    let bitwise_and1 = vec4(t) & vec4(f);
    let short_circuit = (p() || q()) && (r() || s());
}

fn arithmetic() {
    let one_i = 1i;
    let one_u = 1u;
    let one_f = 1.0;
    let two_i = 2i;
    let two_u = 2u;
    let two_f = 2.0;

    // unary
    let neg0 = -one_f;
    let neg1 = -vec2(one_i);
    let neg2 = -vec2(one_f);

    // binary
    // Addition
    let add0 = two_i + one_i;
    let add1 = two_u + one_u;
    let add2 = two_f + one_f;
    let add3 = vec2(two_i) + vec2(one_i);
    let add4 = vec3(two_u) + vec3(one_u);
    let add5 = vec4(two_f) + vec4(one_f);

    // Subtraction
    let sub0 = two_i - one_i;
    let sub1 = two_u - one_u;
    let sub2 = two_f - one_f;
    let sub3 = vec2(two_i) - vec2(one_i);
    let sub4 = vec3(two_u) - vec3(one_u);
    let sub5 = vec4(two_f) - vec4(one_f);

    // Multiplication
    let mul0 = two_i * one_i;
    let mul1 = two_u * one_u;
    let mul2 = two_f * one_f;
    let mul3 = vec2(two_i) * vec2(one_i);
    let mul4 = vec3(two_u) * vec3(one_u);
    let mul5 = vec4(two_f) * vec4(one_f);

    // Division
    let div0 = two_i / one_i;
    let div1 = two_u / one_u;
    let div2 = two_f / one_f;
    let div3 = vec2(two_i) / vec2(one_i);
    let div4 = vec3(two_u) / vec3(one_u);
    let div5 = vec4(two_f) / vec4(one_f);

    // Remainder
    let rem0 = two_i % one_i;
    let rem1 = two_u % one_u;
    let rem2 = two_f % one_f;
    let rem3 = vec2(two_i) % vec2(one_i);
    let rem4 = vec3(two_u) % vec3(one_u);
    let rem5 = vec4(two_f) % vec4(one_f);

    // Binary arithmetic expressions with mixed scalar and vector operands
    {
        let add0 = vec2(two_i) + one_i;
        let add1 = two_i + vec2(one_i);
        let add2 = vec2(two_u) + one_u;
        let add3 = two_u + vec2(one_u);
        let add4 = vec2(two_f) + one_f;
        let add5 = two_f + vec2(one_f);

        let sub0 = vec2(two_i) - one_i;
        let sub1 = two_i - vec2(one_i);
        let sub2 = vec2(two_u) - one_u;
        let sub3 = two_u - vec2(one_u);
        let sub4 = vec2(two_f) - one_f;
        let sub5 = two_f - vec2(one_f);

        let mul0 = vec2(two_i) * one_i;
        let mul1 = two_i * vec2(one_i);
        let mul2 = vec2(two_u) * one_u;
        let mul3 = two_u * vec2(one_u);
        let mul4 = vec2(two_f) * one_f;
        let mul5 = two_f * vec2(one_f);

        let div0 = vec2(two_i) / one_i;
        let div1 = two_i / vec2(one_i);
        let div2 = vec2(two_u) / one_u;
        let div3 = two_u / vec2(one_u);
        let div4 = vec2(two_f) / one_f;
        let div5 = two_f / vec2(one_f);

        let rem0 = vec2(two_i) % one_i;
        let rem1 = two_i % vec2(one_i);
        let rem2 = vec2(two_u) % one_u;
        let rem3 = two_u % vec2(one_u);
        let rem4 = vec2(two_f) % one_f;
        let rem5 = two_f % vec2(one_f);
    }

    // Matrix arithmetic
    let add = mat3x3<f32>() + mat3x3<f32>();
    let sub = mat3x3<f32>() - mat3x3<f32>();

    let mul_scalar0 = mat3x3<f32>() * one_f;
    let mul_scalar1 = two_f * mat3x3<f32>();

    let mul_vector0 = mat4x3<f32>() * vec4(one_f);
    let mul_vector1 = vec3f(two_f) * mat4x3f();

    let mul = mat4x3<f32>() * mat3x4<f32>();

    // Arithmetic involving the minimum value i32 literal. What we're really testing here
    // is how this literal is expressed by Naga backends. eg in Metal, ` + "`" + `-2147483648` + "`" + ` is
    // silently promoted to a ` + "`" + `long` + "`" + ` which we don't want. The addition ensures this would
    // be caught as a compiler error, as we bitcast the operands to unsigned which fails
    // if the expression's type has an unexpected width.
    var prevent_const_eval: i32;
    var wgpu_7437 = prevent_const_eval + -2147483648;
}

fn bit() {
    let one_i = 1i;
    let one_u = 1u;
    let two_i = 2i;
    let two_u = 2u;

    // unary
    let flip0 = ~one_i;
    let flip1 = ~one_u;
    let flip2 = ~vec2(one_i);
    let flip3 = ~vec3(one_u);

    // binary
    let or0 = two_i | one_i;
    let or1 = two_u | one_u;
    let or2 = vec2(two_i) | vec2(one_i);
    let or3 = vec3(two_u) | vec3(one_u);

    let and0 = two_i & one_i;
    let and1 = two_u & one_u;
    let and2 = vec2(two_i) & vec2(one_i);
    let and3 = vec3(two_u) & vec3(one_u);

    let xor0 = two_i ^ one_i;
    let xor1 = two_u ^ one_u;
    let xor2 = vec2(two_i) ^ vec2(one_i);
    let xor3 = vec3(two_u) ^ vec3(one_u);

    let shl0 = two_i << one_u;
    let shl1 = two_u << one_u;
    let shl2 = vec2(two_i) << vec2(one_u);
    let shl3 = vec3(two_u) << vec3(one_u);

    let shr0 = two_i >> one_u;
    let shr1 = two_u >> one_u;
    let shr2 = vec2(two_i) >> vec2(one_u);
    let shr3 = vec3(two_u) >> vec3(one_u);
}

fn comparison() {
    let one_i = 1i;
    let one_u = 1u;
    let one_f = 1.0;
    let two_i = 2i;
    let two_u = 2u;
    let two_f = 2.0;

    let eq0 = two_i == one_i;
    let eq1 = two_u == one_u;
    let eq2 = two_f == one_f;
    let eq3 = vec2(two_i) == vec2(one_i);
    let eq4 = vec3(two_u) == vec3(one_u);
    let eq5 = vec4(two_f) == vec4(one_f);

    let neq0 = two_i != one_i;
    let neq1 = two_u != one_u;
    let neq2 = two_f != one_f;
    let neq3 = vec2(two_i) != vec2(one_i);
    let neq4 = vec3(two_u) != vec3(one_u);
    let neq5 = vec4(two_f) != vec4(one_f);

    let lt0 = two_i < one_i;
    let lt1 = two_u < one_u;
    let lt2 = two_f < one_f;
    let lt3 = vec2(two_i) < vec2(one_i);
    let lt4 = vec3(two_u) < vec3(one_u);
    let lt5 = vec4(two_f) < vec4(one_f);

    let lte0 = two_i <= one_i;
    let lte1 = two_u <= one_u;
    let lte2 = two_f <= one_f;
    let lte3 = vec2(two_i) <= vec2(one_i);
    let lte4 = vec3(two_u) <= vec3(one_u);
    let lte5 = vec4(two_f) <= vec4(one_f);

    let gt0 = two_i > one_i;
    let gt1 = two_u > one_u;
    let gt2 = two_f > one_f;
    let gt3 = vec2(two_i) > vec2(one_i);
    let gt4 = vec3(two_u) > vec3(one_u);
    let gt5 = vec4(two_f) > vec4(one_f);

    let gte0 = two_i >= one_i;
    let gte1 = two_u >= one_u;
    let gte2 = two_f >= one_f;
    let gte3 = vec2(two_i) >= vec2(one_i);
    let gte4 = vec3(two_u) >= vec3(one_u);
    let gte5 = vec4(two_f) >= vec4(one_f);
}

fn assignment() {
    let zero_i = 0i;
    let one_i = 1i;
    let one_u = 1u;
    let two_u = 2u;

    var a = one_i;

    a += one_i;
    a -= one_i;
    a *= a;
    a /= a;
    a %= one_i;
    a &= zero_i;
    a |= zero_i;
    a ^= zero_i;
    a <<= two_u;
    a >>= one_u;

    a++;
    a--;

    var vec0: vec3<i32> = vec3<i32>();
    vec0[one_i]++;
    vec0[one_i]--;
}

fn negation_avoids_prefix_decrement() {
    let i = 1;
    let i0 = -i;
    let i1 = - -i;
    let i2 = -(-i);
    let i3 = -(- i);
    let i4 = - - -i;
    let i5 = - - - - i;
    let i6 = - - -(- -i);
    let i7 = (- - - - -i);

    let f = 1.0;
    let f0 = -f;
    let f1 = - -f;
    let f2 = -(-f);
    let f3 = -(- f);
    let f4 = - - -f;
    let f5 = - - - - f;
    let f6 = - - -(- -f);
    let f7 = (- - - - -f);
}

@compute @workgroup_size(1)
fn main(@builtin(workgroup_id) id: vec3<u32>) {
    builtins();
    splat(f32(id.x), i32(id.y));
    splat_assignment();
    bool_cast(v_f32_one.xyz);

    logical();
    arithmetic();
    bit();
    comparison();
    assignment();

    negation_avoids_prefix_decrement();
}
`

// refShaderControlFlow tests switch statements, loops, break, continue, and barriers.
// Source: naga/tests/in/wgsl/control-flow.wgsl
const refShaderControlFlow = `
fn control_flow() {
    //TODO: execution-only barrier?
    storageBarrier();
    workgroupBarrier();
    textureBarrier();

    var pos: i32;
    // switch without cases
    switch 1 {
        default: {
            pos = 1;
        }
    }

    // non-empty switch *not* in last-statement-in-function position
    switch pos {
        case 1: {
            pos = 0;
            break;
        }
        case 2: {
            pos = 1;
        }
        case 3, 4: {
            pos = 2;
        }
        case 5: {
            pos = 3;
        }
        case default, 6: {
            pos = 4;
        }
    }

    // switch with unsigned integer selectors
    switch(0u) {
        case 0u: {
        }
        default: {
        }
    }

    // non-empty switch in last-statement-in-function position
    switch pos {
        case 1: {
            pos = 0;
            break;
        }
        case 2: {
            pos = 1;
        }
        case 3: {
            pos = 2;
        }
        case 4: {}
        default: {
            pos = 3;
        }
    }

    // trailing commas
    switch pos {
        case 1, {
            pos = 0;
        }
        case 2,: {
            pos = 1;
        }
        case 3, 4, {
            pos = 2;
        }
        case 5, 6,: {
            pos = 3;
        }
        default {
            pos = 4;
        }
    }
}

fn switch_default_break(i: i32) {
    switch i {
        default: {
            break;
        }
    }
}

fn switch_case_break() {
    switch(0) {
        case 0: {
            break;
        }
        default: {}
    }
    return;
}

fn switch_selector_type_conversion() {
    switch (0u) {
        case 0: {
        }
        default: {
        }
    }

    switch (0) {
        case 0u: {
        }
        default: {
        }
    }
}

const ONE = 1;
fn switch_const_expr_case_selectors() {
    const TWO = 2;
    switch (0) {
        case i32(): {
        }
        case ONE: {
        }
        case TWO: {
        }
        case 1 + 2: {
        }
        case vec4(4).x: {
        }
        default: {
        }
    }
}

fn loop_switch_continue(x: i32) {
    loop {
        switch x {
            case 1: {
                continue;
            }
            default: {}
        }
    }
}

fn loop_switch_continue_nesting(x: i32, y: i32, z: i32) {
    loop {
        switch x {
            case 1: {
                continue;
            }
            case 2: {
                switch y {
                    case 1: {
                        continue;
                    }
                    default: {
                        loop {
                            switch z {
                                case 1: {
                                    continue;
                                }
                                default: {}
                            }
                        }
                    }
                }
            }
            default: {}
        }


        // Degenerate switch with continue
        switch y {
            default: {
                continue;
            }
        }
    }

    // In separate loop to avoid spv validation error:
    // See https://github.com/gfx-rs/wgpu/issues/5658
    loop {
        // Nested degenerate switch with continue
        switch y {
            case 1, default: {
                switch z {
                    default: {
                        continue;
                    }
                }
            }
        }
    }
}

// Cases with some of the loop nested switches not containing continues.
fn loop_switch_omit_continue_variable_checks(x: i32, y: i32, z: i32, w: i32) {
    var pos: i32 = 0;
    loop {
        switch x {
            case 1: {
                pos = 1;
            }
            default: {}
        }
    }

    loop {
        switch x {
            case 1: {}
            case 2: {
                switch y {
                    case 1: {
                        continue;
                    }
                    default: {
                        switch z {
                            case 1: {
                                pos = 2;
                            }
                            default: {}
                        }
                    }
                }
            }
            default: {}
        }
    }
}

@compute @workgroup_size(1)
fn main() {
    control_flow();
    switch_default_break(1);
    switch_case_break();
    switch_selector_type_conversion();
    switch_const_expr_case_selectors();
    loop_switch_continue(1);
    loop_switch_continue_nesting(1, 2, 3);
    loop_switch_omit_continue_variable_checks(1, 2, 3, 4);
}
`

// refShaderFunctions tests fma, integer dot product, and packed dot product.
// Source: naga/tests/in/wgsl/functions.wgsl
const refShaderFunctions = `
fn test_fma() -> vec2<f32> {
    let a = vec2<f32>(2.0, 2.0);
    let b = vec2<f32>(0.5, 0.5);
    let c = vec2<f32>(0.5, 0.5);

    return fma(a, b, c);
}

fn test_integer_dot_product() -> i32 {
    let a_2 = vec2<i32>(1);
    let b_2 = vec2<i32>(1);
    let c_2: i32 = dot(a_2, b_2);

    let a_3 = vec3<u32>(1u);
    let b_3 = vec3<u32>(1u);
    let c_3: u32 = dot(a_3, b_3);

    // test baking of arguments
    let c_4: i32 = dot(vec4<i32>(4), vec4<i32>(2));
    return c_4;
}

fn test_packed_integer_dot_product() -> u32 {
    let a_5 = 1u;
    let b_5 = 2u;
    let c_5: i32 = dot4I8Packed(a_5, b_5);

    let a_6 = 3u;
    let b_6 = 4u;
    let c_6: u32 = dot4U8Packed(a_6, b_6);

    // test baking of arguments
    let c_7: i32 = dot4I8Packed(5u + c_6, 6u + c_6);
    let c_8: u32 = dot4U8Packed(7u + c_6, 8u + c_6);
    return c_8;
}

@compute @workgroup_size(1)
fn main() {
    let a = test_fma();
    let b = test_integer_dot_product();
    let c = test_packed_integer_dot_product();
}
`

// refShaderGlobals tests global variables, constants, workgroup vars, atomics, and packed vec3.
// Source: naga/tests/in/wgsl/globals.wgsl
const refShaderGlobals = `
// Global variable & constant declarations

const Foo: bool = true;

var<workgroup> wg : array<f32, 10u>;
var<workgroup> at: atomic<u32>;

struct FooStruct {
    v3: vec3<f32>,
    // test packed vec3
    v1: f32,
}
@group(0) @binding(1)
var<storage, read_write> alignment: FooStruct;

@group(0) @binding(2)
var<storage> dummy: array<vec2<f32>>;

@group(0) @binding(3)
var<uniform> float_vecs: array<vec4<f32>, 20>;

@group(0) @binding(4)
var<uniform> global_vec: vec3<f32>;

@group(0) @binding(5)
var<uniform> global_mat: mat3x2<f32>;

@group(0) @binding(6)
var<uniform> global_nested_arrays_of_matrices_2x4: array<array<mat2x4<f32>, 2>, 2>;

@group(0) @binding(7)
var<uniform> global_nested_arrays_of_matrices_4x2: array<array<mat4x2<f32>, 2>, 2>;

fn test_msl_packed_vec3_as_arg(arg: vec3<f32>) {}

fn test_msl_packed_vec3() {
    // stores
    alignment.v3 = vec3<f32>(1.0);
    var idx = 1;
    alignment.v3.x = 1.0;
    alignment.v3[0] = 2.0;
    alignment.v3[idx] = 3.0;

    // force load to happen here
    let data = alignment;

    // loads
    let l0 = data.v3;
    let l1 = data.v3.zx;
    test_msl_packed_vec3_as_arg(data.v3);

    // matrix vector multiplication
    let mvm0 = data.v3 * mat3x3<f32>();
    let mvm1 = mat3x3<f32>() * data.v3;

    // scalar vector multiplication
    let svm0 = data.v3 * 2.0;
    let svm1 = 2.0 * data.v3;
}

@compute @workgroup_size(1)
fn main() {
    test_msl_packed_vec3();

    wg[7] = (global_nested_arrays_of_matrices_4x2[0][0] * global_nested_arrays_of_matrices_2x4[0][0][0]).x;
    wg[6] = (global_mat * global_vec).x;
    wg[5] = dummy[1].y;
    wg[4] = float_vecs[0].w;
    wg[3] = alignment.v1;
    wg[2] = alignment.v3.x;
    alignment.v1 = 4.0;
    wg[1] = f32(arrayLength(&dummy));
    atomicStore(&at, 2u);

    // Valid, Foo and at is in function scope
    var Foo: f32 = 1.0;
    var at: bool = true;
}
`

// refShaderInterface tests pipeline interface: locations, built-ins, entry points, structs.
// Source: naga/tests/in/wgsl/interface.wgsl
const refShaderInterface = `
// Testing various parts of the pipeline interface: locations, built-ins, and entry points

struct VertexOutput {
    @builtin(position) @invariant position: vec4<f32>,
    @location(1) _varying: f32,
}

@vertex
fn vertex(
    @builtin(vertex_index) vertex_index: u32,
    @builtin(instance_index) instance_index: u32,
    @location(10) color: u32,
) -> VertexOutput {
    let tmp = vertex_index + instance_index + color;
    return VertexOutput(vec4<f32>(1.0), f32(tmp));
}

struct FragmentOutput {
    @builtin(frag_depth) depth: f32,
    @builtin(sample_mask) sample_mask: u32,
    @location(0) color: f32,
}

@fragment
fn fragment(
    in: VertexOutput,
    @builtin(front_facing) front_facing: bool,
    @builtin(sample_index) sample_index: u32,
    @builtin(sample_mask) sample_mask: u32,
) -> FragmentOutput {
    let mask = sample_mask & (1u << sample_index);
    let color = select(0.0, 1.0, front_facing);
    return FragmentOutput(in._varying, mask, color);
}

var<workgroup> output: array<u32, 1>;

@compute @workgroup_size(1)
fn compute(
    @builtin(global_invocation_id) global_id: vec3<u32>,
    @builtin(local_invocation_id) local_id: vec3<u32>,
    @builtin(local_invocation_index) local_index: u32,
    @builtin(workgroup_id) wg_id: vec3<u32>,
    @builtin(num_workgroups) num_wgs: vec3<u32>,
) {
    output[0] = global_id.x + local_id.x + local_index + wg_id.x + num_wgs.x;
}

struct Input1 {
    @builtin(vertex_index) index: u32,
}

struct Input2 {
    @builtin(instance_index) index: u32,
}

@vertex
fn vertex_two_structs(in1: Input1, in2: Input2) -> @builtin(position) @invariant vec4<f32> {
    var index = 2u;
    return vec4<f32>(f32(in1.index), f32(in2.index), f32(index), 0.0);
}
`

// refShaderCollatz implements the Collatz conjecture as a compute shader.
// Source: naga/tests/in/wgsl/collatz.wgsl
const refShaderCollatz = `
struct PrimeIndices {
    data: array<u32>
} // this is used as both input and output for convenience

@group(0) @binding(0)
var<storage,read_write> v_indices: PrimeIndices;

// The Collatz Conjecture states that for any integer n:
// If n is even, n = n/2
// If n is odd, n = 3n+1
// And repeat this process for each new n, you will always eventually reach 1.
fn collatz_iterations(n_base: u32) -> u32 {
    var n = n_base;
    var i: u32 = 0u;
    while n > 1u {
        if n % 2u == 0u {
            n = n / 2u;
        }
        else {
            n = 3u * n + 1u;
        }
        i = i + 1u;
    }
    return i;
}

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
    v_indices.data[global_id.x] = collatz_iterations(v_indices.data[global_id.x]);
}
`

// refShaderQuad tests vertex/fragment pipeline with textures, samplers, and discard.
// Source: naga/tests/in/wgsl/quad.wgsl
const refShaderQuad = `
// vertex
const c_scale: f32 = 1.2;

struct VertexOutput {
  @location(0) uv : vec2<f32>,
  @builtin(position) position : vec4<f32>,
}

@vertex
fn vert_main(
  @location(0) pos : vec2<f32>,
  @location(1) uv : vec2<f32>,
) -> VertexOutput {
  return VertexOutput(uv, vec4<f32>(c_scale * pos, 0.0, 1.0));
}

// fragment
@group(0) @binding(0) var u_texture : texture_2d<f32>;
@group(0) @binding(1) var u_sampler : sampler;

@fragment
fn frag_main(@location(0) uv : vec2<f32>) -> @location(0) vec4<f32> {
  let color = textureSample(u_texture, u_sampler, uv);
  if color.a == 0.0 {
    discard;
  }
  let premultiplied = color.a * color;
  return premultiplied;
}


// We need to make sure that backends are successfully handling multiple entry points for the same shader stage.
@fragment
fn fs_extra() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, 0.5, 0.0, 0.5);
}
`

// refShaderImage tests texture load/store with various texture types and coordinate formats.
// Source: naga/tests/in/wgsl/image.wgsl
const refShaderImage = `
@group(0) @binding(0)
var image_mipmapped_src: texture_2d<u32>;
@group(0) @binding(3)
var image_multisampled_src: texture_multisampled_2d<u32>;
@group(0) @binding(4)
var image_depth_multisampled_src: texture_depth_multisampled_2d;
@group(0) @binding(1)
var image_storage_src: texture_storage_2d<rgba8uint, read>;
@group(0) @binding(5)
var image_array_src: texture_2d_array<u32>;
@group(0) @binding(6)
var image_dup_src: texture_storage_1d<r32uint,read>; // for #1307
@group(0) @binding(7)
var image_1d_src: texture_1d<u32>;
@group(0) @binding(2)
var image_dst: texture_storage_1d<r32uint,write>;

@compute @workgroup_size(16)
fn main(@builtin(local_invocation_id) local_id: vec3<u32>) {
    let dim = textureDimensions(image_storage_src);
    let itc = vec2<i32>(dim * local_id.xy) % vec2<i32>(10, 20);
    // loads with ivec2 coords.
    let value1 = textureLoad(image_mipmapped_src, itc, i32(local_id.z));
    let value1_2 = textureLoad(image_mipmapped_src, itc, u32(local_id.z));
    let value2 = textureLoad(image_multisampled_src, itc, i32(local_id.z));
    let value3 = textureLoad(image_multisampled_src, itc, u32(local_id.z));
    let value4 = textureLoad(image_storage_src, itc);
    let value5 = textureLoad(image_array_src, itc, local_id.z, i32(local_id.z) + 1);
    let value6 = textureLoad(image_array_src, itc, i32(local_id.z), i32(local_id.z) + 1);
    let value7 = textureLoad(image_1d_src, i32(local_id.x), i32(local_id.z));
    let value8 = textureLoad(image_dup_src, i32(local_id.x));
    // loads with uvec2 coords.
    let value1u = textureLoad(image_mipmapped_src, vec2<u32>(itc), i32(local_id.z));
    let value2u = textureLoad(image_multisampled_src, vec2<u32>(itc), i32(local_id.z));
    let value3u = textureLoad(image_multisampled_src, vec2<u32>(itc), u32(local_id.z));
    let value4u = textureLoad(image_storage_src, vec2<u32>(itc));
    let value5u = textureLoad(image_array_src, vec2<u32>(itc), local_id.z, i32(local_id.z) + 1);
    let value6u = textureLoad(image_array_src, vec2<u32>(itc), i32(local_id.z), i32(local_id.z) + 1);
    let value7u = textureLoad(image_1d_src, u32(local_id.x), i32(local_id.z));
    // store with ivec2 coords.
    textureStore(image_dst, itc.x, value1 + value2 + value4 + value5 + value6);
    // store with uvec2 coords.
    textureStore(image_dst, u32(itc.x), value1u + value2u + value4u + value5u + value6u);
}

@compute @workgroup_size(16, 1, 1)
fn depth_load(@builtin(local_invocation_id) local_id: vec3<u32>) {
    let dim: vec2<u32> = textureDimensions(image_storage_src);
    let itc: vec2<i32> = (vec2<i32>(dim * local_id.xy) % vec2<i32>(10, 20));
    let val: f32 = textureLoad(image_depth_multisampled_src, itc, i32(local_id.z));
    textureStore(image_dst, itc.x, vec4<u32>(u32(val)));
    return;
}

@group(0) @binding(0)
var image_1d: texture_1d<f32>;
@group(0) @binding(1)
var image_2d: texture_2d<f32>;
@group(0) @binding(2)
var image_2d_u32: texture_2d<u32>;
@group(0) @binding(3)
var image_2d_i32: texture_2d<i32>;
@group(0) @binding(4)
var image_2d_array: texture_2d_array<f32>;
@group(0) @binding(5)
var image_cube: texture_cube<f32>;
@group(0) @binding(6)
var image_cube_array: texture_cube_array<f32>;
@group(0) @binding(7)
var image_3d: texture_3d<f32>;
@group(0) @binding(8)
var image_aa: texture_multisampled_2d<f32>;

@vertex
fn queries() -> @builtin(position) vec4<f32> {
    let dim_1d = textureDimensions(image_1d);
    let dim_1d_lod = textureDimensions(image_1d, i32(dim_1d));
    let dim_2d = textureDimensions(image_2d);
    let dim_2d_lod = textureDimensions(image_2d, 1);
    let dim_2d_array = textureDimensions(image_2d_array);
    let dim_2d_array_lod = textureDimensions(image_2d_array, 1);
    let dim_cube = textureDimensions(image_cube);
    let dim_cube_lod = textureDimensions(image_cube, 1);
    let dim_cube_array = textureDimensions(image_cube_array);
    let dim_cube_array_lod = textureDimensions(image_cube_array, 1);
    let dim_3d = textureDimensions(image_3d);
    let dim_3d_lod = textureDimensions(image_3d, 1);
    let dim_2s_ms = textureDimensions(image_aa);

    let sum = dim_1d + dim_2d.y + dim_2d_lod.y + dim_2d_array.y + dim_2d_array_lod.y +
        dim_cube.y + dim_cube_lod.y + dim_cube_array.y + dim_cube_array_lod.y +
        dim_3d.z + dim_3d_lod.z;
    return vec4<f32>(f32(sum));
}

@vertex
fn levels_queries() -> @builtin(position) vec4<f32> {
    let num_levels_2d = textureNumLevels(image_2d);
    let num_layers_2d = textureNumLayers(image_2d_array);
    let num_levels_2d_array = textureNumLevels(image_2d_array);
    let num_layers_2d_array = textureNumLayers(image_2d_array);
    let num_levels_cube = textureNumLevels(image_cube);
    let num_levels_cube_array = textureNumLevels(image_cube_array);
    let num_layers_cube = textureNumLayers(image_cube_array);
    let num_levels_3d = textureNumLevels(image_3d);
    let num_samples_aa = textureNumSamples(image_aa);

    let sum = num_layers_2d + num_layers_cube + num_samples_aa +
        num_levels_2d + num_levels_2d_array + num_levels_3d + num_levels_cube + num_levels_cube_array;
    return vec4<f32>(f32(sum));
}

@group(1) @binding(0)
var sampler_reg: sampler;

@fragment
fn texture_sample() -> @location(0) vec4<f32> {
    const tc = vec2<f32>(0.5);
    const tc3 = vec3<f32>(0.5);
    const offset = vec2<i32>(3, 1);
    let level = 2.3;
    var a: vec4<f32>;
    a += textureSample(image_1d, sampler_reg, tc.x);
    a += textureSample(image_2d, sampler_reg, tc);
    a += textureSample(image_2d, sampler_reg, tc, vec2<i32>(3, 1));
    a += textureSampleLevel(image_2d, sampler_reg, tc, level);
    a += textureSampleLevel(image_2d, sampler_reg, tc, level, offset);
    a += textureSampleBias(image_2d, sampler_reg, tc, 2.0, offset);
    a += textureSampleBaseClampToEdge(image_2d, sampler_reg, tc);
    a += textureSample(image_2d_array, sampler_reg, tc, 0u);
    a += textureSample(image_2d_array, sampler_reg, tc, 0u, offset);
    a += textureSampleLevel(image_2d_array, sampler_reg, tc, 0u, level);
    a += textureSampleLevel(image_2d_array, sampler_reg, tc, 0u, level, offset);
    a += textureSampleBias(image_2d_array, sampler_reg, tc, 0u, 2.0, offset);
    a += textureSample(image_2d_array, sampler_reg, tc, 0);
    a += textureSample(image_2d_array, sampler_reg, tc, 0, offset);
    a += textureSampleLevel(image_2d_array, sampler_reg, tc, 0, level);
    a += textureSampleLevel(image_2d_array, sampler_reg, tc, 0, level, offset);
    a += textureSampleBias(image_2d_array, sampler_reg, tc, 0, 2.0, offset);
    a += textureSample(image_cube_array, sampler_reg, tc3, 0u);
    a += textureSampleLevel(image_cube_array, sampler_reg, tc3, 0u, level);
    a += textureSampleBias(image_cube_array, sampler_reg, tc3, 0u, 2.0);
    a += textureSample(image_cube_array, sampler_reg, tc3, 0);
    a += textureSampleLevel(image_cube_array, sampler_reg, tc3, 0, level);
    a += textureSampleBias(image_cube_array, sampler_reg, tc3, 0, 2.0);
    return a;
}

@group(1) @binding(1)
var sampler_cmp: sampler_comparison;
@group(1) @binding(2)
var image_2d_depth: texture_depth_2d;
@group(1) @binding(3)
var image_2d_array_depth: texture_depth_2d_array;
@group(1) @binding(4)
var image_cube_depth: texture_depth_cube;

@fragment
fn texture_sample_comparison() -> @location(0) f32 {
    let tc = vec2<f32>(0.5);
    let tc3 = vec3<f32>(0.5);
    let dref = 0.5;
    var a: f32;
    a += textureSampleCompare(image_2d_depth, sampler_cmp, tc, dref);
    a += textureSampleCompare(image_2d_array_depth, sampler_cmp, tc, 0u, dref);
    a += textureSampleCompare(image_2d_array_depth, sampler_cmp, tc, 0, dref);
    a += textureSampleCompare(image_cube_depth, sampler_cmp, tc3, dref);
    a += textureSampleCompareLevel(image_2d_depth, sampler_cmp, tc, dref);
    a += textureSampleCompareLevel(image_2d_array_depth, sampler_cmp, tc, 0u, dref);
    a += textureSampleCompareLevel(image_2d_array_depth, sampler_cmp, tc, 0, dref);
    a += textureSampleCompareLevel(image_cube_depth, sampler_cmp, tc3, dref);
    return a;
}

@fragment
fn gather() -> @location(0) vec4<f32> {
    let tc = vec2<f32>(0.5);
    let dref = 0.5;
    let s2d = textureGather(1, image_2d, sampler_reg, tc);
    let s2d_offset = textureGather(3, image_2d, sampler_reg, tc, vec2<i32>(3, 1));
    let s2d_depth = textureGatherCompare(image_2d_depth, sampler_cmp, tc, dref);
    let s2d_depth_offset = textureGatherCompare(image_2d_depth, sampler_cmp, tc, dref, vec2<i32>(3, 1));

    let u = textureGather(0, image_2d_u32, sampler_reg, tc);
    let i = textureGather(0, image_2d_i32, sampler_reg, tc);
    let f = vec4<f32>(u) + vec4<f32>(i);

    return s2d + s2d_offset + s2d_depth + s2d_depth_offset + f;
}

@fragment
fn depth_no_comparison() -> @location(0) vec4<f32> {
    let tc = vec2<f32>(0.5);
    let level = 1;
    let s2d = textureSample(image_2d_depth, sampler_reg, tc);
    let s2d_gather = textureGather(image_2d_depth, sampler_reg, tc);
    let s2d_level = textureSampleLevel(image_2d_depth, sampler_reg, tc, level);
    return s2d + s2d_gather + s2d_level;
}
`

// refShaderShadow tests vertex/fragment with uniforms, storage buffers, shadow mapping,
// depth textures, comparison sampling, and for-loops.
// Source: naga/tests/in/wgsl/shadow.wgsl
const refShaderShadow = `
struct Globals {
    view_proj: mat4x4<f32>,
    num_lights: vec4<u32>,
}

@group(0)
@binding(0)
var<uniform> u_globals: Globals;

struct Entity {
    world: mat4x4<f32>,
    color: vec4<f32>,
}

@group(1)
@binding(0)
var<uniform> u_entity: Entity;

struct VertexOutput {
    @builtin(position) proj_position: vec4<f32>,
    @location(0) world_normal: vec3<f32>,
    @location(1) world_position: vec4<f32>,
}

@vertex
fn vs_main(
    @location(0) position: vec4<i32>,
    @location(1) normal: vec4<i32>,
) -> VertexOutput {
    let w = u_entity.world;
    let world_pos = u_entity.world * vec4<f32>(position);
    var out: VertexOutput;
    out.world_normal = mat3x3<f32>(w[0].xyz, w[1].xyz, w[2].xyz) * vec3<f32>(normal.xyz);
    out.world_position = world_pos;
    out.proj_position = u_globals.view_proj * world_pos;
    return out;
}

// fragment shader

struct Light {
    proj: mat4x4<f32>,
    pos: vec4<f32>,
    color: vec4<f32>,
}

@group(0)
@binding(1)
var<storage, read> s_lights: array<Light>;
@group(0)
@binding(1)
var<uniform> u_lights: array<Light, 10>; // Used when storage types are not supported
@group(0)
@binding(2)
var t_shadow: texture_depth_2d_array;
@group(0)
@binding(3)
var sampler_shadow: sampler_comparison;

fn fetch_shadow(light_id: u32, homogeneous_coords: vec4<f32>) -> f32 {
    if (homogeneous_coords.w <= 0.0) {
        return 1.0;
    }
    let flip_correction = vec2<f32>(0.5, -0.5);
    let proj_correction = 1.0 / homogeneous_coords.w;
    let light_local = homogeneous_coords.xy * flip_correction * proj_correction + vec2<f32>(0.5, 0.5);
    return textureSampleCompareLevel(t_shadow, sampler_shadow, light_local, i32(light_id), homogeneous_coords.z * proj_correction);
}

const c_ambient: vec3<f32> = vec3<f32>(0.05, 0.05, 0.05);
const c_max_lights: u32 = 10u;

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let normal = normalize(in.world_normal);
    var color: vec3<f32> = c_ambient;
    for(var i = 0u; i < min(u_globals.num_lights.x, c_max_lights); i++) {
        let light = s_lights[i];
        let shadow = fetch_shadow(i, light.proj * in.world_position);
        let light_dir = normalize(light.pos.xyz - in.world_position.xyz);
        let diffuse = max(0.0, dot(normal, light_dir));
        color += shadow * diffuse * light.color.xyz;
    }
    return vec4<f32>(color, 1.0) * u_entity.color;
}

// The fragment entrypoint used when storage buffers are not available for the lights
@fragment
fn fs_main_without_storage(in: VertexOutput) -> @location(0) vec4<f32> {
    let normal = normalize(in.world_normal);
    var color: vec3<f32> = c_ambient;
    for(var i = 0u; i < min(u_globals.num_lights.x, c_max_lights); i++) {
        let light = u_lights[i];
        let shadow = fetch_shadow(i, light.proj * in.world_position);
        let light_dir = normalize(light.pos.xyz - in.world_position.xyz);
        let diffuse = max(0.0, dot(normal, light_dir));
        color += shadow * diffuse * light.color.xyz;
    }
    return vec4<f32>(color, 1.0) * u_entity.color;
}
`

// refShaderBoids implements a flocking simulation compute shader.
// Source: naga/tests/in/wgsl/boids.wgsl
const refShaderBoids = `
const NUM_PARTICLES: u32 = 1500u;

struct Particle {
  pos : vec2<f32>,
  vel : vec2<f32>,
}

struct SimParams {
  deltaT : f32,
  rule1Distance : f32,
  rule2Distance : f32,
  rule3Distance : f32,
  rule1Scale : f32,
  rule2Scale : f32,
  rule3Scale : f32,
}

struct Particles {
  particles : array<Particle>
}

@group(0) @binding(0) var<uniform> params : SimParams;
@group(0) @binding(1) var<storage> particlesSrc : Particles;
@group(0) @binding(2) var<storage,read_write> particlesDst : Particles;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) global_invocation_id : vec3<u32>) {
  let index : u32 = global_invocation_id.x;
  if index >= NUM_PARTICLES {
    return;
  }

  var vPos = particlesSrc.particles[index].pos;
  var vVel = particlesSrc.particles[index].vel;

  var cMass = vec2<f32>(0.0, 0.0);
  var cVel = vec2<f32>(0.0, 0.0);
  var colVel = vec2<f32>(0.0, 0.0);
  var cMassCount : i32 = 0;
  var cVelCount : i32 = 0;

  var pos : vec2<f32>;
  var vel : vec2<f32>;
  var i : u32 = 0u;
  loop {
    if i >= NUM_PARTICLES {
      break;
    }
    if i == index {
      continue;
    }

    pos = particlesSrc.particles[i].pos;
    vel = particlesSrc.particles[i].vel;

    if distance(pos, vPos) < params.rule1Distance {
      cMass = cMass + pos;
      cMassCount = cMassCount + 1;
    }
    if distance(pos, vPos) < params.rule2Distance {
      colVel = colVel - (pos - vPos);
    }
    if distance(pos, vPos) < params.rule3Distance {
      cVel = cVel + vel;
      cVelCount = cVelCount + 1;
    }

    continuing {
      i = i + 1u;
    }
  }
  if cMassCount > 0 {
    cMass = cMass / f32(cMassCount) - vPos;
  }
  if cVelCount > 0 {
    cVel = cVel / f32(cVelCount);
  }

  vVel = vVel + (cMass * params.rule1Scale) +
      (colVel * params.rule2Scale) +
      (cVel * params.rule3Scale);

  // clamp velocity for a more pleasing simulation
  vVel = normalize(vVel) * clamp(length(vVel), 0.0, 0.1);

  // kinematic update
  vPos = vPos + (vVel * params.deltaT);

  // Wrap around boundary
  if vPos.x < -1.0 {
    vPos.x = 1.0;
  }
  if vPos.x > 1.0 {
    vPos.x = -1.0;
  }
  if vPos.y < -1.0 {
    vPos.y = 1.0;
  }
  if vPos.y > 1.0 {
    vPos.y = -1.0;
  }

  // Write back
  particlesDst.particles[index].pos = vPos;
  particlesDst.particles[index].vel = vVel;
}
`

// refShaderAccess tests pointer dereferencing, struct/array access, storage read/write,
// function pointers, and member access patterns.
// Source: naga/tests/in/wgsl/access.wgsl
const refShaderAccess = `
// This snapshot tests accessing various containers, dereferencing pointers.

struct GlobalConst {
    a: u32,
    b: vec3<u32>,
    c: i32,
}
// tests msl padding insertion for global constants
var<private> msl_padding_global_const: GlobalConst = GlobalConst(0u, vec3<u32>(0u, 0u, 0u), 0);

struct AlignedWrapper {
    @align(8) value: i32
}

struct Bar {
    _matrix: mat4x3<f32>,
    matrix_array: array<mat2x2<f32>, 2>,
    atom: atomic<i32>,
    atom_arr: array<atomic<i32>, 10>,
    arr: array<vec2<u32>, 2>,
    data: array<AlignedWrapper>,
}

@group(0) @binding(0)
var<storage,read_write> bar: Bar;

struct Baz {
    m: mat3x2<f32>,
}

@group(0) @binding(1)
var<uniform> baz: Baz;

@group(0) @binding(2)
var<storage,read_write> qux: vec2<i32>;

fn test_matrix_within_struct_accesses() {
    var idx = 1;

    idx--;

    // loads
    let l0 = baz.m;
    let l1 = baz.m[0];
    let l2 = baz.m[idx];
    let l3 = baz.m[0][1];
    let l4 = baz.m[0][idx];
    let l5 = baz.m[idx][1];
    let l6 = baz.m[idx][idx];

    var t = Baz(mat3x2<f32>(vec2<f32>(1.0), vec2<f32>(2.0), vec2<f32>(3.0)));

    idx++;

    // stores
    t.m = mat3x2<f32>(vec2<f32>(6.0), vec2<f32>(5.0), vec2<f32>(4.0));
    t.m[0] = vec2<f32>(9.0);
    t.m[idx] = vec2<f32>(90.0);
    t.m[0][1] = 10.0;
    t.m[0][idx] = 20.0;
    t.m[idx][1] = 30.0;
    t.m[idx][idx] = 40.0;
}

struct MatCx2InArray {
    am: array<mat4x2<f32>, 2>,
}

@group(0) @binding(3)
var<uniform> nested_mat_cx2: MatCx2InArray;

fn test_matrix_within_array_within_struct_accesses() {
    var idx = 1;

    idx--;

    // loads
    let l0 = nested_mat_cx2.am;
    let l1 = nested_mat_cx2.am[0];
    let l2 = nested_mat_cx2.am[0][0];
    let l3 = nested_mat_cx2.am[0][idx];
    let l4 = nested_mat_cx2.am[0][0][1];
    let l5 = nested_mat_cx2.am[0][0][idx];
    let l6 = nested_mat_cx2.am[0][idx][1];
    let l7 = nested_mat_cx2.am[0][idx][idx];

    var t = MatCx2InArray(array<mat4x2<f32>, 2>());

    idx++;

    // stores
    t.am = array<mat4x2<f32>, 2>();
    t.am[0] = mat4x2<f32>(vec2<f32>(8.0), vec2<f32>(7.0), vec2<f32>(6.0), vec2<f32>(5.0));
    t.am[0][0] = vec2<f32>(9.0);
    t.am[0][idx] = vec2<f32>(90.0);
    t.am[0][0][1] = 10.0;
    t.am[0][0][idx] = 20.0;
    t.am[0][idx][1] = 30.0;
    t.am[0][idx][idx] = 40.0;
}

fn read_from_private(foo: ptr<function, f32>) -> f32 {
    return *foo;
}

fn test_arr_as_arg(a: array<array<f32, 10>, 5>) -> f32 {
    return a[4][9];
}

@vertex
fn foo_vert(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
    var foo: f32 = 0.0;
    let baz: f32 = foo;
    foo = 1.0;

    _ = msl_padding_global_const;
    test_matrix_within_struct_accesses();
    test_matrix_within_array_within_struct_accesses();

    // test storage loads
    let _matrix = bar._matrix;
    let arr = bar.arr;
    let index = 3u;
    let b = bar._matrix[index].x;
    let a = bar.data[arrayLength(&bar.data) - 2u].value;
    let c = qux;

    // test pointer types
    let data_pointer: ptr<storage, i32, read_write> = &bar.data[0].value;
    let foo_value = read_from_private(&foo);

    // test array indexing
    var c2 = array<i32, 5>(a, i32(b), 3, 4, 5);
    c2[vi + 1u] = 42;
    let value = c2[vi];

    test_arr_as_arg(array<array<f32, 10>, 5>());

    return vec4<f32>(_matrix * vec4<f32>(vec4<i32>(value)), 2.0);
}

@fragment
fn foo_frag() -> @location(0) vec4<f32> {
    // test storage stores
    bar._matrix[1].z = 1.0;
    bar._matrix = mat4x3<f32>(vec3<f32>(0.0), vec3<f32>(1.0), vec3<f32>(2.0), vec3<f32>(3.0));
    bar.arr = array<vec2<u32>, 2>(vec2<u32>(0u), vec2<u32>(1u));
    bar.data[1].value = 1;
    qux = vec2<i32>();

    return vec4<f32>(0.0);
}

fn assign_through_ptr_fn(p: ptr<function, u32>) {
    *p = 42u;
}

fn assign_array_through_ptr_fn(foo: ptr<function, array<vec4<f32>, 2>>) {
    *foo = array<vec4<f32>, 2>(vec4(1.0), vec4(2.0));
}

fn assign_through_ptr() {
    var val = 33u;
    assign_through_ptr_fn(&val);

    var arr = array<vec4<f32>, 2>(vec4(6.0), vec4(7.0));
    assign_array_through_ptr_fn(&arr);
}

struct AssignToMember {
  x: u32,
}

fn fetch_arg_ptr_member(p: ptr<function, AssignToMember>) -> u32 {
  return (*p).x;
}

fn assign_to_arg_ptr_member(p: ptr<function, AssignToMember>) {
  (*p).x = 10u;
}

fn fetch_arg_ptr_array_element(p: ptr<function, array<u32, 4>>) -> u32 {
  return (*p)[1];
}

fn assign_to_arg_ptr_array_element(p: ptr<function, array<u32, 4>>) {
  (*p)[1] = 10u;
}

fn assign_to_ptr_components() {
   var s1: AssignToMember;
   assign_to_arg_ptr_member(&s1);
   fetch_arg_ptr_member(&s1);

   var a1: array<u32, 4>;
   assign_to_arg_ptr_array_element(&a1);
   fetch_arg_ptr_array_element(&a1);
}

fn index_ptr(value: bool) -> bool {
    var a = array<bool, 1>(value);
    let p = &a;
    return p[0];
}

struct S { m: i32 };

fn member_ptr() -> i32 {
    var s: S = S(42);
    let p = &s;
    return p.m;
}

struct Inner { delicious: i32 }

struct Outer { om_nom_nom: Inner, thing: u32 }

fn let_members_of_members() -> i32 {
    let thing = Outer();

    let inner = thing.om_nom_nom;
    let delishus = inner.delicious;

    if (thing.thing != u32(delishus)) {
        // LOL
    }

    return thing.om_nom_nom.delicious;
}

fn var_members_of_members() -> i32 {
    var thing = Outer();

    var inner = thing.om_nom_nom;
    var delishus = inner.delicious;

    if (thing.thing != u32(delishus)) {
        // LOL
    }

    return thing.om_nom_nom.delicious;
}

@compute @workgroup_size(1)
fn foo_compute() {
    assign_through_ptr();
    assign_to_ptr_components();
    index_ptr(true);
    member_ptr();
    let_members_of_members();
    var_members_of_members();
}
`

// refShaderMathFunctions tests math builtins: degrees, radians, saturate, sign,
// firstLeadingBit, firstTrailingBit, countTrailingZeros, countLeadingZeros, ldexp, modf, frexp, quantizeToF16.
// Source: naga/tests/in/wgsl/math-functions.wgsl
const refShaderMathFunctions = `
@fragment
fn main() {
    let f = 1.0;
    let v = vec4<f32>(0.0);
    let a = degrees(f);
    let b = radians(f);
    let c = degrees(v);
    let d = radians(v);
    let e = saturate(v);
    let g = refract(v, v, f);
    let sign_a = sign(-1);
    let sign_b = sign(vec4(-1));
    let sign_c = sign(-1.0);
    let sign_d = sign(vec4(-1.0));
    let const_dot = dot(vec2<i32>(), vec2<i32>());
    let first_leading_bit_abs = firstLeadingBit(abs(0u));
    let flb_a = firstLeadingBit(-1);
    let flb_b = firstLeadingBit(vec2(-1));
    let flb_c = firstLeadingBit(vec2(1u));
    let ftb_a = firstTrailingBit(-1);
    let ftb_b = firstTrailingBit(1u);
    let ftb_c = firstTrailingBit(vec2(-1));
    let ftb_d = firstTrailingBit(vec2(1u));
    let ctz_a = countTrailingZeros(0u);
    let ctz_b = countTrailingZeros(0);
    let ctz_c = countTrailingZeros(0xFFFFFFFFu);
    let ctz_d = countTrailingZeros(-1);
    let ctz_e = countTrailingZeros(vec2(0u));
    let ctz_f = countTrailingZeros(vec2(0));
    let ctz_g = countTrailingZeros(vec2(1u));
    let ctz_h = countTrailingZeros(vec2(1));
    let clz_a = countLeadingZeros(-1);
    let clz_b = countLeadingZeros(1u);
    let clz_c = countLeadingZeros(vec2(-1));
    let clz_d = countLeadingZeros(vec2(1u));
    let lde_a = ldexp(1.0, 2);
    let lde_b = ldexp(vec2(1.0, 2.0), vec2(3, 4));
    let modf_a = modf(1.5);
    let modf_b = modf(1.5).fract;
    let modf_c = modf(1.5).whole;
    let modf_d = modf(vec2(1.5, 1.5));
    let modf_e = modf(vec4(1.5, 1.5, 1.5, 1.5)).whole.x;
    let modf_f: f32 = modf(vec2(1.5, 1.5)).fract.y;
    let frexp_a = frexp(1.5);
    let frexp_b = frexp(1.5).fract;
    let frexp_c: i32 = frexp(1.5).exp;
    let frexp_d: i32 = frexp(vec4(1.5, 1.5, 1.5, 1.5)).exp.x;
    let quantizeToF16_a: f32 = quantizeToF16(1.0);
    let quantizeToF16_b: vec2<f32> = quantizeToF16(vec2(1.0, 1.0));
    let quantizeToF16_c: vec3<f32> = quantizeToF16(vec3(1.0, 1.0, 1.0));
    let quantizeToF16_d: vec4<f32> = quantizeToF16(vec4(1.0, 1.0, 1.0, 1.0));
}
`

// refShaderStructLayout tests struct alignment and padding in vertex, fragment, and compute stages.
// Source: naga/tests/in/wgsl/struct-layout.wgsl
const refShaderStructLayout = `
// Create several type definitions to test align and size layout.

struct NoPadding {
    @location(0)
    v3: vec3f, // align 16, size 12; no start padding needed
    @location(1)
    f3: f32, // align 4, size 4; no start padding needed
}
@fragment
fn no_padding_frag(input: NoPadding) -> @location(0) vec4f {
    _ = input;
    return vec4f(0.0);
}
@vertex
fn no_padding_vert(input: NoPadding) -> @builtin(position) vec4f {
    _ = input;
    return vec4f(0.0);
}
@group(0) @binding(0) var<uniform> no_padding_uniform: NoPadding;
@group(0) @binding(1) var<storage, read_write> no_padding_storage: NoPadding;
@compute @workgroup_size(16,1,1)
fn no_padding_comp() {
    var x: NoPadding;
    x = no_padding_uniform;
    x = no_padding_storage;
}

struct NeedsPadding {
    @location(0) f3_forces_padding: f32, // align 4, size 4; no start padding needed
    @location(1) v3_needs_padding: vec3f, // align 16, size 12; needs 12 bytes of padding
    @location(2) f3: f32, // align 4, size 4; no start padding needed
}
@fragment
fn needs_padding_frag(input: NeedsPadding) -> @location(0) vec4f {
    _ = input;
    return vec4f(0.0);
}
@vertex
fn needs_padding_vert(input: NeedsPadding) -> @builtin(position) vec4f {
    _ = input;
    return vec4f(0.0);
}
@group(0) @binding(2) var<uniform> needs_padding_uniform: NeedsPadding;
@group(0) @binding(3) var<storage, read_write> needs_padding_storage: NeedsPadding;
@compute @workgroup_size(16,1,1)
fn needs_padding_comp() {
    var x: NeedsPadding;
    x = needs_padding_uniform;
    x = needs_padding_storage;
}
`

// ---------------------------------------------------------------------------
// Bonus shader sources
// ---------------------------------------------------------------------------

// refShaderSkybox tests cube map sampling with matrix math (inverse projection, transpose).
// Source: naga/tests/in/wgsl/skybox.wgsl
const refShaderSkybox = `
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec3<f32>,
}

struct Data {
    proj_inv: mat4x4<f32>,
    view: mat4x4<f32>,
}
@group(0) @binding(0)
var<uniform> r_data: Data;

@vertex
fn vs_main(@builtin(vertex_index) vertex_index: u32) -> VertexOutput {
    // hacky way to draw a large triangle
    var tmp1 = i32(vertex_index) / 2;
    var tmp2 = i32(vertex_index) & 1;
    let pos = vec4<f32>(
        f32(tmp1) * 4.0 - 1.0,
        f32(tmp2) * 4.0 - 1.0,
        0.0,
        1.0,
    );

    let inv_model_view = transpose(mat3x3<f32>(r_data.view[0].xyz, r_data.view[1].xyz, r_data.view[2].xyz));
    let unprojected = r_data.proj_inv * pos;
    return VertexOutput(pos, inv_model_view * unprojected.xyz);
}

@group(0) @binding(1)
var r_texture: texture_cube<f32>;
@group(0) @binding(2)
var r_sampler: sampler;

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    return textureSample(r_texture, r_sampler, in.uv);
}
`

// refShaderWater implements a water simulation with simplex noise, Fresnel, and specular lighting.
// Source: wgpu/examples/features/src/water/water.wgsl
//
//nolint:misspell // Original Rust reference uses British English "colour" throughout
const refShaderWater = `
struct Uniforms {
    view: mat4x4<f32>,
    projection: mat4x4<f32>,
    time_size_width: vec4<f32>,
    viewport_height: f32,
};
@group(0) @binding(0) var<uniform> uniforms: Uniforms;

const light_point = vec3<f32>(150.0, 70.0, 0.0);
const light_colour = vec3<f32>(1.0, 0.98, 0.82);
const one = vec4<f32>(1.0, 1.0, 1.0, 1.0);

const Y_SCL: f32 = 0.86602540378443864676372317075294;
const CURVE_BIAS: f32 = -0.1;
const INV_1_CURVE_BIAS: f32 = 1.11111111111; //1.0 / (1.0 + CURVE_BIAS);

fn modf_polyfill_vec3(value: vec3<f32>, int_part: ptr<function, vec3<f32>>) -> vec3<f32> {
    *int_part = trunc(value);
    return value - *int_part;
}
fn modf_polyfill_vec4(value: vec4<f32>, int_part: ptr<function, vec4<f32>>) -> vec4<f32> {
    *int_part = trunc(value);
    return value - *int_part;
}

fn permute(x: vec4<f32>) -> vec4<f32> {
    var temp: vec4<f32> = 289.0 * one;
    return modf_polyfill_vec4(((x*34.0) + one) * x, &temp);
}

fn taylorInvSqrt(r: vec4<f32>) -> vec4<f32> {
    return 1.79284291400159 * one - 0.85373472095314 * r;
}

fn snoise(v: vec3<f32>) -> f32 {
    let C = vec2<f32>(1.0/6.0, 1.0/3.0);
    let D = vec4<f32>(0.0, 0.5, 1.0, 2.0);

    let vCy = dot(v, C.yyy);
    var i: vec3<f32> = floor(v + vec3<f32>(vCy, vCy, vCy));
    let iCx = dot(i, C.xxx);
    let x0 = v - i + vec3<f32>(iCx, iCx, iCx);

    let g = step(x0.yzx, x0.xyz);
    let l = (vec3<f32>(1.0, 1.0, 1.0) - g).zxy;
    let i1 = min(g, l);
    let i2 = max(g, l);

    let x1 = x0 - i1 + C.xxx;
    let x2 = x0 - i2 + C.yyy;
    let x3 = x0 - D.yyy;

    var temp: vec3<f32> = 289.0 * one.xyz;
    i = modf_polyfill_vec3(i, &temp);
    let p = permute(
        permute(
            permute(i.zzzz + vec4<f32>(0.0, i1.z, i2.z, 1.0))
            + i.yyyy + vec4<f32>(0.0, i1.y, i2.y, 1.0))
        + i.xxxx + vec4<f32>(0.0, i1.x, i2.x, 1.0));

    let n_ = 0.142857142857;
    let ns = n_ * D.wyz - D.xzx;

    let j = p - 49.0 * floor(p * ns.z * ns.z);

    let x_ = floor(j * ns.z);
    let y_ = floor(j - 7.0 * x_);

    var x: vec4<f32> = x_ *ns.x + ns.yyyy;
    var y: vec4<f32> = y_ *ns.x + ns.yyyy;
    let h = one - abs(x) - abs(y);

    let b0 = vec4<f32>(x.xy, y.xy);
    let b1 = vec4<f32>(x.zw, y.zw);

    let s0 = floor(b0)*2.0 + one;
    let s1 = floor(b1)*2.0 + one;
    let sh = -step(h, 0.0 * one);

    let a0 = b0.xzyw + s0.xzyw*sh.xxyy;
    let a1 = b1.xzyw + s1.xzyw*sh.zzww;

    var p0 = vec3<f32>(a0.xy, h.x);
    var p1 = vec3<f32>(a0.zw, h.y);
    var p2 = vec3<f32>(a1.xy, h.z);
    var p3 = vec3<f32>(a1.zw, h.w);

    let norm = taylorInvSqrt(vec4<f32>(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
    p0 *= norm.x;
    p1 *= norm.y;
    p2 *= norm.z;
    p3 *= norm.w;

    var m: vec4<f32> = max(0.6 * one - vec4<f32>(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0 * one);
    m *= m;
    return 9.0 * dot(m*m, vec4<f32>(dot(p0, x0), dot(p1, x1), dot(p2, x2), dot(p3, x3)));
}

fn apply_distortion(pos: vec3<f32>) -> vec3<f32> {
    var perlin_pos: vec3<f32> = pos;

    let sn = uniforms.time_size_width.x;
    let cs = uniforms.time_size_width.y;
    let size = uniforms.time_size_width.z;

    perlin_pos = vec3<f32>(perlin_pos.y - perlin_pos.x - size, perlin_pos.x, perlin_pos.z);

    let xcos = perlin_pos.x * cs;
    let xsin = perlin_pos.x * sn;
    let ycos = perlin_pos.y * cs;
    let ysin = perlin_pos.y * sn;
    let zcos = perlin_pos.z * cs;
    let zsin = perlin_pos.z * sn;

    let perlin_pos_y = vec3<f32>(xcos + zsin, perlin_pos.y, -xsin + xcos);
    let perlin_pos_z = vec3<f32>(xcos - ysin, xsin + ycos, perlin_pos.x);

    perlin_pos = vec3<f32>(perlin_pos.z - perlin_pos.x, perlin_pos.y, perlin_pos.x);

    let perlin_pos_x = vec3<f32>(perlin_pos.x, ycos - zsin, ysin + zcos);

    return vec3<f32>(
        pos.x + snoise(perlin_pos_x + 2.0*one.xxx) * 0.4,
        pos.y + snoise(perlin_pos_y - 2.0*one.xxx) * 1.8,
        pos.z + snoise(perlin_pos_z) * 0.4
    );
}

fn make_position(original: vec2<f32>) -> vec4<f32> {
    let interpreted = vec3<f32>(original.x * 0.5, 0.0, original.y * Y_SCL);
    return vec4<f32>(apply_distortion(interpreted), 1.0);
}

fn make_normal(a: vec3<f32>, b: vec3<f32>, c: vec3<f32>) -> vec3<f32> {
    let norm = normalize(cross(b - c, a - c));
    let center = (a + b + c) * (1.0 / 3.0);
    return (normalize(a - center) * CURVE_BIAS + norm) * INV_1_CURVE_BIAS;
}

fn calc_fresnel(view: vec3<f32>, normal: vec3<f32>) -> f32 {
    var refractive: f32 = abs(dot(view, normal));
    refractive = pow(refractive, 1.33333333333);
    return refractive;
}

fn calc_specular(eye: vec3<f32>, normal: vec3<f32>, light: vec3<f32>) -> f32 {
    let light_reflected = reflect(light, normal);
    var specular: f32 = max(dot(eye, light_reflected), 0.0);
    specular = pow(specular, 10.0);
    return specular;
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) f_WaterScreenPos: vec2<f32>,
    @location(1) f_Fresnel: f32,
    @location(2) f_Light: vec3<f32>,
};

@vertex
fn vs_main(
    @location(0) position: vec2<i32>,
    @location(1) offsets: vec4<i32>,
) -> VertexOutput {
    let p_pos = vec2<f32>(position);
    let b_pos = make_position(p_pos + vec2<f32>(offsets.xy));
    let c_pos = make_position(p_pos + vec2<f32>(offsets.zw));
    let a_pos = make_position(p_pos);
    let original_pos = vec4<f32>(p_pos.x * 0.5, 0.0, p_pos.y * Y_SCL, 1.0);

    let vm = uniforms.view;
    let transformed_pos = vm * a_pos;
    let water_pos = transformed_pos.xyz * (1.0 / transformed_pos.w);
    let normal = make_normal((vm * a_pos).xyz, (vm * b_pos).xyz, (vm * c_pos).xyz);
    let eye = normalize(-water_pos);
    let transformed_light = vm * vec4<f32>(light_point, 1.0);

    var result: VertexOutput;
    result.f_Light = light_colour * calc_specular(eye, normal, normalize(water_pos.xyz - (transformed_light.xyz * (1.0 / transformed_light.w))));
    result.f_Fresnel = calc_fresnel(eye, normal);

    let gridpos = uniforms.projection * vm * original_pos;
    result.f_WaterScreenPos = (0.5 * gridpos.xy * (1.0 / gridpos.w)) + vec2<f32>(0.5, 0.5);

    result.position = uniforms.projection * transformed_pos;
    return result;
}


const water_colour = vec3<f32>(0.0, 0.46, 0.95);
const zNear = 10.0;
const zFar = 400.0;

@group(0) @binding(1) var reflection: texture_2d<f32>;
@group(0) @binding(2) var terrain_depth_tex: texture_2d<f32>;
@group(0) @binding(3) var colour_sampler: sampler;
@group(0) @binding(4) var depth_sampler: sampler;

fn to_linear_depth(depth: f32) -> f32 {
    let z_n = 2.0 * depth - 1.0;
    let z_e = 2.0 * zNear * zFar / (zFar + zNear - z_n * (zFar - zNear));
    return z_e;
}

@fragment
fn fs_main(vertex: VertexOutput) -> @location(0) vec4<f32> {
    let reflection_colour = textureSample(reflection, colour_sampler, vertex.f_WaterScreenPos.xy).xyz;

    let pixel_depth = to_linear_depth(vertex.position.z);
    let normalized_coords = vertex.position.xy / vec2<f32>(uniforms.time_size_width.w, uniforms.viewport_height);
    let terrain_depth = to_linear_depth(textureSample(terrain_depth_tex, depth_sampler, normalized_coords).r);

    let dist = terrain_depth - pixel_depth;
    let clamped = pow(smoothstep(0.0, 1.5, dist), 4.8);

    let final_colour = vertex.f_Light + reflection_colour;
    let t = smoothstep(1.0, 5.0, dist) * 0.2;
    let depth_colour = mix(final_colour, water_colour, vec3<f32>(t, t, t));

    return vec4<f32>(depth_colour, clamped * (1.0 - vertex.f_Fresnel));
}
`
