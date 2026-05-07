package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// =============================================================================
// Helpers: WGSL → MSL integration test helper
// =============================================================================

// compileWGSL compiles a WGSL source to MSL with default options.
func compileWGSL(t *testing.T, src string) string {
	t.Helper()
	return compileWGSLWithOpts(t, src, DefaultOptions())
}

// compileWGSLWithOpts compiles WGSL source to MSL with custom options.
func compileWGSLWithOpts(t *testing.T, src string, opts Options) string {
	t.Helper()
	lexer := wgsl.NewLexer(src)
	tokens, lexErr := lexer.Tokenize()
	if lexErr != nil {
		t.Fatalf("Lex error: %v", lexErr)
	}
	parser := wgsl.NewParser(tokens)
	ast, parseErr := parser.Parse()
	if parseErr != nil {
		t.Fatalf("Parse error: %v", parseErr)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower error: %v", err)
	}
	code, _, compileErr := Compile(module, opts)
	if compileErr != nil {
		t.Fatalf("MSL compile error: %v", compileErr)
	}
	return code
}

// computeWrap wraps a WGSL expression in a compute shader with storage output
// to prevent dead-code elimination.
func computeWrap(expr string) string {
	return `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    ` + expr + `
}
`
}

// computeWrapIO wraps a WGSL expression with both storage input (to prevent
// const folding) and storage output (to prevent DCE).
func computeWrapIO(expr string) string {
	return `
struct In { a: f32, b: f32, c: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    ` + expr + `
}
`
}

// =============================================================================
// Test: Entry point stages — vertex, fragment, compute
// =============================================================================

func TestIntegration_VertexEntryPoint(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec4<f32>,
};
@vertex fn vs_main(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var o: VertexOutput;
    o.pos = vec4(0.0, 0.0, 0.0, 1.0);
    o.color = vec4(1.0, 0.0, 0.0, 1.0);
    return o;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "vertex")
	mustContainMSL(t, code, "vs_main")
	mustContainMSL(t, code, "[[position]]")
	mustContainMSL(t, code, "[[user(loc0)")
	mustContainMSL(t, code, "[[vertex_id]]")
}

func TestIntegration_FragmentEntryPoint(t *testing.T) {
	src := `
@fragment fn fs_main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "fragment")
	mustContainMSL(t, code, "fs_main")
	mustContainMSL(t, code, "[[color(0)]]")
}

func TestIntegration_ComputeEntryPoint(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(8, 8, 1)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    out.v = gid.x;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "kernel")
	mustContainMSL(t, code, "cs_main")
	mustContainMSL(t, code, "[[thread_position_in_grid]]")
}

// =============================================================================
// Test: Expressions via WGSL integration (storage output to prevent DCE)
// =============================================================================

func TestIntegration_ArithmeticExpressions(t *testing.T) {
	src := computeWrap(`
    let a = 10.0f;
    let b = 3.0f;
    out.v = a + b;
`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "+")
}

func TestIntegration_IntegerDivisionAndModulo(t *testing.T) {
	src := `
struct Out { d: i32, m: i32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    let a = 10i;
    let b = 3i;
    out.d = a / b;
    out.m = a % b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_div")
	mustContainMSL(t, code, "naga_mod")
}

func TestIntegration_UnsignedDivMod(t *testing.T) {
	src := `
struct Out { d: u32, m: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    let a = 10u;
    let b = 3u;
    out.d = a / b;
    out.m = a % b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_div")
	mustContainMSL(t, code, "naga_mod")
}

func TestIntegration_ComparisonOperators(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    let a = 1.0f;
    let b = 2.0f;
    if a == b { out.v = 1u; }
    if a != b { out.v = 2u; }
    if a < b  { out.v = 3u; }
    if a <= b { out.v = 4u; }
    if a > b  { out.v = 5u; }
    if a >= b { out.v = 6u; }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "==")
	mustContainMSL(t, code, "!=")
	mustContainMSL(t, code, "<")
	mustContainMSL(t, code, "<=")
}

func TestIntegration_BitwiseOperators(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    let a = 0xFFu;
    let b = 0x0Fu;
    out.v = a & b;
    out.v = a | b;
    out.v = a ^ b;
    out.v = a << 2u;
    out.v = a >> 2u;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "&")
	mustContainMSL(t, code, "|")
	mustContainMSL(t, code, "^")
	mustContainMSL(t, code, "<<")
	mustContainMSL(t, code, ">>")
}

func TestIntegration_LogicalOperators(t *testing.T) {
	// Note: WGSL && and || are lowered to if/else (short-circuit) not to && / ||.
	// We test logical not (!) and the if/else pattern.
	src := `
struct In { a: u32, b: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    let a = inp.a > 0u;
    if !a { out.v = 3u; }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "!")
	mustContainMSL(t, code, "if (")
}

func TestIntegration_UnaryNegate(t *testing.T) {
	src := computeWrap(`
    let a = 5.0f;
    out.v = -a;
`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "-(")
}

// =============================================================================
// Test: Math functions via WGSL integration
// =============================================================================

func TestIntegration_MathFunctions1Arg(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"abs_float", "out.v = abs(inp.a);", "metal::abs("},
		{"ceil", "out.v = ceil(inp.a);", "metal::ceil("},
		{"floor", "out.v = floor(inp.a);", "metal::floor("},
		{"round", "out.v = round(inp.a);", "metal::round("},
		{"trunc", "out.v = trunc(inp.a);", "metal::trunc("},
		{"sqrt", "out.v = sqrt(inp.a);", "metal::sqrt("},
		{"inverseSqrt", "out.v = inverseSqrt(inp.a);", "metal::rsqrt("},
		{"exp", "out.v = exp(inp.a);", "metal::exp("},
		{"exp2", "out.v = exp2(inp.a);", "metal::exp2("},
		{"log", "out.v = log(inp.a);", "metal::log("},
		{"log2", "out.v = log2(inp.a);", "metal::log2("},
		{"sin", "out.v = sin(inp.a);", "metal::sin("},
		{"cos", "out.v = cos(inp.a);", "metal::cos("},
		{"tan", "out.v = tan(inp.a);", "metal::tan("},
		{"asin", "out.v = asin(inp.a);", "metal::asin("},
		{"acos", "out.v = acos(inp.a);", "metal::acos("},
		{"atan", "out.v = atan(inp.a);", "metal::atan("},
		{"sinh", "out.v = sinh(inp.a);", "metal::sinh("},
		{"cosh", "out.v = cosh(inp.a);", "metal::cosh("},
		{"tanh", "out.v = tanh(inp.a);", "metal::tanh("},
		{"asinh", "out.v = asinh(inp.a);", "metal::asinh("},
		{"acosh", "out.v = acosh(inp.a);", "metal::acosh("},
		{"atanh", "out.v = atanh(inp.a);", "metal::atanh("},
		{"fract", "out.v = fract(inp.a);", "metal::fract("},
		{"saturate", "out.v = saturate(inp.a);", "metal::saturate("},
		{"sign_float", "out.v = sign(inp.a);", "metal::sign("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, computeWrapIO(tt.expr))
			mustContainMSL(t, code, tt.want)
		})
	}
}

func TestIntegration_MathFunctions2Arg(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"min_f32", "out.v = min(inp.a, inp.b);", "metal::min("},
		{"max_f32", "out.v = max(inp.a, inp.b);", "metal::max("},
		{"pow", "out.v = pow(inp.a, inp.b);", "metal::pow("},
		{"atan2", "out.v = atan2(inp.a, inp.b);", "metal::atan2("},
		{"step", "out.v = step(inp.a, inp.b);", "metal::step("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, computeWrapIO(tt.expr))
			mustContainMSL(t, code, tt.want)
		})
	}
}

func TestIntegration_MathFunctions3Arg(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"clamp", "out.v = clamp(inp.a, inp.b, inp.c);", "metal::clamp("},
		{"mix", "out.v = mix(inp.a, inp.b, inp.c);", "metal::mix("},
		{"smoothstep", "out.v = smoothstep(inp.a, inp.b, inp.c);", "metal::smoothstep("},
		{"fma", "out.v = fma(inp.a, inp.b, inp.c);", "metal::fma("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, computeWrapIO(tt.expr))
			mustContainMSL(t, code, tt.want)
		})
	}
}

func TestIntegration_IntegerMathFunctions(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"countOneBits", `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = countOneBits(inp.v); }
`, "metal::popcount("},
		{"reverseBits", `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = reverseBits(inp.v); }
`, "metal::reverse_bits("},
		{"countLeadingZeros", `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = countLeadingZeros(inp.v); }
`, "metal::clz("},
		{"countTrailingZeros", `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = countTrailingZeros(inp.v); }
`, "metal::ctz("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, tt.src)
			mustContainMSL(t, code, tt.want)
		})
	}
}

func TestIntegration_SignInteger(t *testing.T) {
	src := `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = sign(inp.v); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::select(metal::select(")
}

func TestIntegration_AbsInteger(t *testing.T) {
	src := `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = abs(inp.v); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_abs(")
}

// =============================================================================
// Test: Vector and matrix operations
// =============================================================================

func TestIntegration_VectorConstruction(t *testing.T) {
	src := `
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = vec4(1.0f, 2.0f, 3.0f, 4.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::float4(")
}

func TestIntegration_VectorSwizzle(t *testing.T) {
	src := `
struct Out { v: vec2<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let v = vec4(1.0f, 2.0f, 3.0f, 4.0f);
    out.v = v.zw;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".zw")
}

func TestIntegration_VectorSplat(t *testing.T) {
	src := `
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = vec4(1.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::float4(")
}

func TestIntegration_MatrixConstruction(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let m = mat2x2(1.0f, 0.0f, 0.0f, 1.0f);
    out.v = m[0][0];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::float2x2(")
}

func TestIntegration_MatrixTimesVector(t *testing.T) {
	src := `
struct Out { v: vec2<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let m = mat2x2(1.0f, 0.0f, 0.0f, 1.0f);
    let v = vec2(1.0f, 2.0f);
    out.v = m * v;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "*")
}

// =============================================================================
// Test: Type casts
// =============================================================================

func TestIntegration_TypeCasts(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"f32_to_i32", `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = i32(inp.v); }
`, "int"},
		{"i32_to_f32", `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = f32(inp.v); }
`, "static_cast<float>"},
		{"u32_to_i32", `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = i32(inp.v); }
`, "static_cast<int>"},
		{"i32_to_u32", `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = u32(inp.v); }
`, "static_cast<uint>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, tt.src)
			mustContainMSL(t, code, tt.want)
		})
	}
}

// =============================================================================
// Test: Control flow
// =============================================================================

func TestIntegration_IfElseChain(t *testing.T) {
	src := `
struct Out { v: i32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let x = 5i;
    if x > 10i { out.v = 1i; }
    else if x > 0i { out.v = 2i; }
    else { out.v = 3i; }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "if (")
	mustContainMSL(t, code, "} else")
}

func TestIntegration_ForLoop(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    for (var i = 0u; i < 10u; i = i + 1u) {
        out.v = i;
    }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "while(true)")
}

func TestIntegration_SwitchStatement(t *testing.T) {
	src := `
struct Out { v: i32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let x = 1i;
    switch x {
        case 0i: { out.v = 0i; }
        case 1i: { out.v = 1i; }
        default: { out.v = 2i; }
    }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "switch(")
	mustContainMSL(t, code, "case 0:")
	mustContainMSL(t, code, "case 1:")
	mustContainMSL(t, code, "default:")
}

func TestIntegration_LoopBreakContinue(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    var i = 0u;
    loop {
        if i >= 10u { break; }
        if i == 5u { i = i + 1u; continue; }
        out.v = i;
        i = i + 1u;
    }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "break;")
	mustContainMSL(t, code, "continue;")
}

// =============================================================================
// Test: Variables and stores
// =============================================================================

func TestIntegration_LocalVariables(t *testing.T) {
	src := `
@compute @workgroup_size(1)
fn main() {
    var x = 0i;
    x = 42i;
    let y = x;
    _ = y;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "int")
	mustContainMSL(t, code, "42")
}

func TestIntegration_PrivateGlobals(t *testing.T) {
	src := computeWrap(`
    global_val = 2.718;
    out.v = global_val;
`)
	src = strings.Replace(src, "struct Out { v: f32 };", "var<private> global_val: f32 = 3.14;\nstruct Out { v: f32 };", 1)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "global_val")
}

func TestIntegration_WorkgroupVariable(t *testing.T) {
	src := `
var<workgroup> shared_data: array<f32, 64>;

struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) {
    shared_data[idx] = f32(idx);
    out.v = shared_data[0];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "threadgroup")
}

// =============================================================================
// Test: Structs and member access
// =============================================================================

func TestIntegration_StructDefinition(t *testing.T) {
	src := `
struct Light {
    position: vec3<f32>,
    color: vec3<f32>,
    intensity: f32,
};
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    var light: Light;
    light.position = vec3(0.0f, 1.0f, 0.0f);
    light.color = vec3(1.0f, 1.0f, 1.0f);
    light.intensity = 1.0f;
    out.v = light.intensity;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "struct Light")
	mustContainMSL(t, code, ".position")
	mustContainMSL(t, code, ".intensity")
}

func TestIntegration_NestedStructs(t *testing.T) {
	src := `
struct Inner { value: f32 };
struct Outer { inner: Inner, scale: f32 };
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    var o: Outer;
    o.inner.value = 1.0f;
    o.scale = 2.0f;
    out.v = o.scale;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "struct Inner")
	mustContainMSL(t, code, "struct Outer")
}

// =============================================================================
// Test: Arrays
// =============================================================================

func TestIntegration_FixedArray(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn main() {
    var arr = array<f32, 4>(1.0f, 2.0f, 3.0f, 4.0f);
    out.v = arr[0];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "inner")
}

// =============================================================================
// Test: Uniform and storage buffers
// =============================================================================

func TestIntegration_UniformBuffer(t *testing.T) {
	src := `
struct Params { resolution: vec2<f32>, time: f32 };
@group(0) @binding(0) var<uniform> params: Params;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;

@compute @workgroup_size(1) fn main() { out.v = params.time; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "constant")
	mustContainMSL(t, code, "[[buffer(")
}

func TestIntegration_StorageBuffer(t *testing.T) {
	src := `
struct Data { values: array<f32, 256> };
@group(0) @binding(0) var<storage, read_write> data: Data;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    data.values[gid.x] = f32(gid.x);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "device")
	mustContainMSL(t, code, "[[buffer(")
}

func TestIntegration_ReadOnlyStorage(t *testing.T) {
	src := `
struct Input { values: array<f32, 256> };
@group(0) @binding(0) var<storage, read> input_data: Input;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;

@compute @workgroup_size(1) fn main() { out.v = input_data.values[0]; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "const")
}

// =============================================================================
// Test: Texture and sampler bindings
// =============================================================================

func TestIntegration_TextureSample(t *testing.T) {
	src := `
@group(0) @binding(0) var my_tex: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;

@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(my_tex, my_sampler, uv);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d<float")
	mustContainMSL(t, code, "sampler")
	mustContainMSL(t, code, ".sample(")
}

func TestIntegration_TextureSampleLevel(t *testing.T) {
	src := `
@group(0) @binding(0) var my_tex: texture_2d<f32>;
@group(0) @binding(1) var my_sampler: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;

@compute @workgroup_size(1) fn main() {
    out.v = textureSampleLevel(my_tex, my_sampler, vec2(0.5f, 0.5f), 0.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "level(")
}

// =============================================================================
// Test: Derivatives (fragment-only)
// =============================================================================

func TestIntegration_Derivatives(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"dpdx", "return vec4(dpdx(uv.x));", "metal::dfdx("},
		{"dpdy", "return vec4(dpdy(uv.y));", "metal::dfdy("},
		{"fwidth", "return vec4(fwidth(uv.x));", "metal::fwidth("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {\n    " + tt.expr + "\n}\n"
			code := compileWGSL(t, src)
			mustContainMSL(t, code, tt.want)
		})
	}
}

// =============================================================================
// Test: Relational functions
// =============================================================================

func TestIntegration_Relational(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"any", `
struct In { a: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    if any(inp.a > vec2(0.0f)) { out.v = 1u; }
}`, "metal::any("},
		{"all", `
struct In { a: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    if all(inp.a > vec2(0.0f)) { out.v = 1u; }
}`, "metal::all("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, tt.src)
			mustContainMSL(t, code, tt.want)
		})
	}
}

// =============================================================================
// Test: Select
// =============================================================================

func TestIntegration_SelectVector(t *testing.T) {
	src := `
struct In { a: vec2<f32>, b: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let cond = inp.a > inp.b;
    out.v = select(inp.a, inp.b, cond);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::select(")
}

func TestIntegration_SelectScalar(t *testing.T) {
	src := `
struct In { a: f32, b: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let cond = inp.a > inp.b;
    out.v = select(inp.a, inp.b, cond);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "?")
	mustContainMSL(t, code, ":")
}

// =============================================================================
// Test: Function calls
// =============================================================================

func TestIntegration_FunctionCall(t *testing.T) {
	src := `
fn add(a: f32, b: f32) -> f32 { return a + b; }
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = add(1.0f, 2.0f); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "add(")
}

func TestIntegration_FunctionCallWithGlobal(t *testing.T) {
	src := `
var<private> g_value: f32 = 1.0;
fn helper() -> f32 { return g_value; }
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = helper(); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "g_value")
}

// =============================================================================
// Test: Constants
// =============================================================================

func TestIntegration_ConstantDeclaration(t *testing.T) {
	src := `
const PI: f32 = 3.14159265;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = PI; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "constant")
	mustContainMSL(t, code, "3.14159")
}

// =============================================================================
// Test: Barriers
// =============================================================================

func TestIntegration_WorkgroupBarrier(t *testing.T) {
	src := `
var<workgroup> shared_val: f32;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) {
    shared_val = f32(idx);
    workgroupBarrier();
    out.v = shared_val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "threadgroup_barrier")
	mustContainMSL(t, code, "mem_threadgroup")
}

func TestIntegration_StorageBarrier(t *testing.T) {
	src := `
var<workgroup> shared_val: f32;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) {
    shared_val = f32(idx);
    storageBarrier();
    out.v = shared_val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "threadgroup_barrier")
	mustContainMSL(t, code, "mem_device")
}

// =============================================================================
// Test: Atomics
// =============================================================================

func TestIntegration_AtomicAdd(t *testing.T) {
	src := `
var<workgroup> counter: atomic<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) { atomicAdd(&counter, 1u); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_add_explicit")
	mustContainMSL(t, code, "memory_order_relaxed")
}

func TestIntegration_AtomicStore(t *testing.T) {
	src := `
var<workgroup> counter: atomic<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) { atomicStore(&counter, idx); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_store_explicit")
}

func TestIntegration_AtomicLoad(t *testing.T) {
	src := `
var<workgroup> counter: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) {
    out.v = atomicLoad(&counter);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_load_explicit")
}

func TestIntegration_AtomicCompareExchangeWeak(t *testing.T) {
	src := `
var<workgroup> counter: atomic<u32>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) idx: u32) {
    let result = atomicCompareExchangeWeak(&counter, 0u, 1u);
    out.v = result.old_value;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_compare_exchange")
}

// =============================================================================
// Test: Modf and frexp
// =============================================================================

func TestIntegration_Modf(t *testing.T) {
	src := computeWrap(`
    let r = modf(3.5f);
    out.v = r.fract;
`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_modf")
	mustContainMSL(t, code, "_modf_result")
}

func TestIntegration_Frexp(t *testing.T) {
	src := computeWrap(`
    let r = frexp(3.5f);
    out.v = r.fract;
`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_frexp")
	mustContainMSL(t, code, "_frexp_result")
}

// =============================================================================
// Test: QuantizeToF16
// =============================================================================

func TestIntegration_QuantizeToF16(t *testing.T) {
	src := computeWrap(`out.v = quantizeToF16(3.14f);`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float(half(")
}

func TestIntegration_QuantizeToF16_Vector(t *testing.T) {
	src := `
struct Out { v: vec2<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = quantizeToF16(vec2(3.14f, 2.71f)); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float2")
	mustContainMSL(t, code, "half2")
}

// =============================================================================
// Test: Integer dot product
// =============================================================================

func TestIntegration_IntDot(t *testing.T) {
	src := `
struct In { a: vec2<i32>, b: vec2<i32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = dot(inp.a, inp.b);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_dot")
}

// =============================================================================
// Test: WorkgroupZeroInit
// =============================================================================

func TestIntegration_WorkgroupZeroInit(t *testing.T) {
	src := `
var<workgroup> shared_arr: array<f32, 64>;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    shared_arr[lid] = 1.0f;
    out.v = shared_arr[0];
}
`
	opts := DefaultOptions()
	opts.ZeroInitializeWorkgroupMemory = true
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "threadgroup_barrier")
}

// =============================================================================
// Test: Header
// =============================================================================

func TestIntegration_HeaderContents(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = 1u; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "#include <metal_stdlib>")
	mustContainMSL(t, code, "#include <simd/simd.h>")
	mustContainMSL(t, code, "using metal::uint;")
}

// =============================================================================
// Test: Packed vec3
// =============================================================================

func TestIntegration_PackedVec3InStruct(t *testing.T) {
	src := `
struct Data { normal: vec3<f32>, extra: f32 };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;

@compute @workgroup_size(1) fn main() { out.v = data.normal; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "packed_float3")
}

// =============================================================================
// Test: Bitcast
// =============================================================================

func TestIntegration_Bitcast(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = bitcast<u32>(1.0f); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "as_type<")
}

// =============================================================================
// Test: Texture type variants
// =============================================================================

func TestIntegration_Texture3D(t *testing.T) {
	src := `
@group(0) @binding(0) var vol: texture_3d<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureSampleLevel(vol, samp, vec3(0.5f), 0.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture3d<float")
}

func TestIntegration_TextureCube(t *testing.T) {
	src := `
@group(0) @binding(0) var cube_tex: texture_cube<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureSampleLevel(cube_tex, samp, vec3(0.0f, 0.0f, 1.0f), 0.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texturecube<float")
}

func TestIntegration_Texture2DArray(t *testing.T) {
	src := `
@group(0) @binding(0) var arr_tex: texture_2d_array<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureSampleLevel(arr_tex, samp, vec2(0.5f), 0i, 0.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d_array<float")
}

func TestIntegration_TextureTypeSint(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<i32>;
struct Out { v: vec4<i32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d<int")
}

func TestIntegration_TextureTypeUint(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<u32>;
struct Out { v: vec4<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d<uint")
}

func TestIntegration_DepthTexture(t *testing.T) {
	src := `
@group(0) @binding(0) var depth: texture_depth_2d;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(depth, vec2(0i, 0i), 0i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "depth2d<float")
}

func TestIntegration_MultisampledTexture(t *testing.T) {
	src := `
@group(0) @binding(0) var ms_tex: texture_multisampled_2d<f32>;
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return textureLoad(ms_tex, vec2<i32>(pos.xy), 0i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d_ms<float")
}

// =============================================================================
// Test: Image storage read
// =============================================================================

func TestIntegration_ImageStorageRead(t *testing.T) {
	src := `
@group(0) @binding(0) var img: texture_storage_2d<rgba8unorm, read>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(img, vec2(0i, 0i));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d<float")
}

// =============================================================================
// Test: Texture queries
// =============================================================================

func TestIntegration_TextureDimensions(t *testing.T) {
	src := `
@group(0) @binding(0) var my_tex: texture_2d<f32>;
struct Out { v: vec2<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureDimensions(my_tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_width(")
}

func TestIntegration_TextureNumLevels(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureNumLevels(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_num_mip_levels(")
}

func TestIntegration_TextureNumSamples(t *testing.T) {
	src := `
@group(0) @binding(0) var ms_tex: texture_multisampled_2d<f32>;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureNumSamples(ms_tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_num_samples(")
}

// =============================================================================
// Test: Sampler comparison
// =============================================================================

func TestIntegration_SamplerComparison(t *testing.T) {
	src := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var shadow_sampler: sampler_comparison;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let shadow = textureSampleCompare(depth_tex, shadow_sampler, uv, 0.5f);
    return vec4(shadow, shadow, shadow, 1.0f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "depth2d<float")
	mustContainMSL(t, code, "sampler")
	mustContainMSL(t, code, ".sample_compare(")
}

// =============================================================================
// Test: Discard (kill)
// =============================================================================

func TestIntegration_Discard(t *testing.T) {
	src := `
@fragment fn fs(@location(0) alpha: f32) -> @location(0) vec4<f32> {
    if alpha < 0.5 { discard; }
    return vec4(1.0, 1.0, 1.0, alpha);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "discard_fragment()")
}

// =============================================================================
// Test: Named expressions
// =============================================================================

func TestIntegration_NamedExpressions(t *testing.T) {
	src := computeWrapIO(`
    let named_value = inp.a;
    out.v = named_value * inp.b;
`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "named_value")
}

// =============================================================================
// Test: Complex real-world shader
// =============================================================================

func TestIntegration_ComplexVertexShader(t *testing.T) {
	src := `
struct Camera { view_proj: mat4x4<f32>, position: vec3<f32> };
@group(0) @binding(0) var<uniform> camera: Camera;

struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
};
struct VertexOutput {
    @builtin(position) clip_pos: vec4<f32>,
    @location(0) world_normal: vec3<f32>,
    @location(1) uv: vec2<f32>,
};

@vertex fn vs_main(in: VertexInput) -> VertexOutput {
    var o: VertexOutput;
    o.clip_pos = camera.view_proj * vec4(in.position, 1.0);
    o.world_normal = in.normal;
    o.uv = in.uv;
    return o;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "vertex")
	mustContainMSL(t, code, "vs_main")
	mustContainMSL(t, code, "view_proj")
	mustContainMSL(t, code, "[[position]]")
	mustContainMSL(t, code, "[[buffer(")
}

// =============================================================================
// Test: Multiple entry points
// =============================================================================

func TestIntegration_MultipleEntryPoints(t *testing.T) {
	src := `
@vertex fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4(0.0, 0.0, 0.0, 1.0);
}
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "vertex")
	mustContainMSL(t, code, "fragment")
}

// =============================================================================
// Test: Vertex IO with interpolation
// =============================================================================

func TestIntegration_VertexFragmentIO(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec3<f32>,
    @location(1) @interpolate(flat) id: u32,
};
@vertex fn vs(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var o: VertexOutput;
    o.pos = vec4(0.0, 0.0, 0.0, 1.0);
    o.color = vec3(1.0, 0.0, 0.0);
    o.id = idx;
    return o;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[position]]")
	mustContainMSL(t, code, "[[user(loc0)")
	mustContainMSL(t, code, "[[user(loc1)")
	mustContainMSL(t, code, "flat")
}

// =============================================================================
// Test: Compute builtins
// =============================================================================

func TestIntegration_ComputeBuiltins(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(8, 8, 1)
fn main(
    @builtin(global_invocation_id) gid: vec3<u32>,
    @builtin(local_invocation_id) lid: vec3<u32>,
    @builtin(workgroup_id) wgid: vec3<u32>,
    @builtin(local_invocation_index) lidx: u32,
    @builtin(num_workgroups) nwg: vec3<u32>,
) {
    out.v = gid.x + lid.x + wgid.x + lidx + nwg.x;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[thread_position_in_grid]]")
	mustContainMSL(t, code, "[[thread_position_in_threadgroup]]")
	mustContainMSL(t, code, "[[threadgroup_position_in_grid]]")
	mustContainMSL(t, code, "[[thread_index_in_threadgroup]]")
	mustContainMSL(t, code, "[[threadgroups_per_grid]]")
}

// =============================================================================
// Test: Unpack/Pack functions
// =============================================================================

func TestIntegration_Unpack4x8Unorm(t *testing.T) {
	src := `
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = unpack4x8unorm(0xFFu); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "unpack_unorm4x8_to_float")
}

func TestIntegration_Unpack4x8Snorm(t *testing.T) {
	src := `
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = unpack4x8snorm(0xFFu); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "unpack_snorm4x8_to_float")
}

func TestIntegration_Pack4x8Unorm(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = pack4x8unorm(vec4(0.0f, 0.25f, 0.5f, 1.0f)); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "pack_float_to_unorm4x8")
}

func TestIntegration_Pack4x8Snorm(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = pack4x8snorm(vec4(-1.0f, -0.5f, 0.5f, 1.0f)); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "pack_float_to_snorm4x8")
}

func TestIntegration_Unpack2x16Float(t *testing.T) {
	src := `
struct Out { v: vec2<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = unpack2x16float(0xFFu); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float2(as_type<half2>(")
}

func TestIntegration_Pack2x16Float(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = pack2x16float(vec2(1.0f, 2.0f)); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "as_type<uint>(half2(")
}

// =============================================================================
// Test: Pipeline constants
// =============================================================================

func TestIntegration_PipelineConstantsBasic(t *testing.T) {
	src := `
override SCALE: f32 = 1.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = inp.v * SCALE; }
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"SCALE": 3.0}
	code := compileWGSLWithOpts(t, src, opts)
	// Override replaced: SCALE becomes 3.0 constant.
	mustContainMSL(t, code, "3.0")
}

// =============================================================================
// Test: Scalar types
// =============================================================================

func TestIntegration_ScalarTypes(t *testing.T) {
	src := `
struct Out { b: u32, i: i32, u: u32, f: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    if true { out.b = 1u; } else { out.b = 0u; }
    out.i = -1i;
    out.u = 1u;
    out.f = 1.0f;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "int")
	mustContainMSL(t, code, "uint")
	mustContainMSL(t, code, "float")
}

// =============================================================================
// Test: Atomic in struct
// =============================================================================

func TestIntegration_AtomicInStruct(t *testing.T) {
	src := `
struct Counter { count: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> counters: Counter;
@compute @workgroup_size(1) fn main() { atomicAdd(&counters.count, 1u); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_uint")
	mustContainMSL(t, code, "atomic_fetch_add_explicit")
}

// =============================================================================
// Test: WorkgroupUniformLoad
// =============================================================================

func TestIntegration_WorkgroupUniformLoad(t *testing.T) {
	src := `
var<workgroup> wg_val: u32;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    wg_val = lid;
    out.v = workgroupUniformLoad(&wg_val);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "threadgroup_barrier")
}

// =============================================================================
// Test: Address spaces
// =============================================================================

func TestIntegration_AddressSpaces(t *testing.T) {
	src := `
var<private> prv: f32;
struct Uniforms { val: f32 };
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
struct Storage { vals: array<f32, 4> };
@group(0) @binding(1) var<storage, read_write> store: Storage;
var<workgroup> wg: f32;

@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) lid: u32) {
    prv = uniforms.val;
    store.vals[0] = prv;
    wg = prv;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "constant")
	mustContainMSL(t, code, "device")
	mustContainMSL(t, code, "threadgroup")
}

// =============================================================================
// Test: F2I conversion
// =============================================================================

func TestIntegration_F2ISafeConversion(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = i32(inp.v); }
`
	code := compileWGSL(t, src)
	// Expect either naga_f2i helper or plain int() cast.
	if !strings.Contains(code, "naga_f2i") && !strings.Contains(code, "int(") {
		t.Error("Expected naga_f2i or int() cast in output")
	}
}

// =============================================================================
// Test: Version
// =============================================================================

func TestIntegration_Version10(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = 1.0f; }
`
	opts := DefaultOptions()
	opts.LangVersion = Version1_0
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "#include <metal_stdlib>")
}

// =============================================================================
// Test: Loop bounding
// =============================================================================

func TestIntegration_ForceLoopBounding(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    var i = 0u;
    loop {
        if i >= 10u { break; }
        out.v = i;
        i = i + 1u;
    }
}
`
	opts := DefaultOptions()
	opts.ForceLoopBounding = true
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "while(true)")
}

// =============================================================================
// Test: Literal types
// =============================================================================

func TestIntegration_LiteralTypes(t *testing.T) {
	src := `
struct Out { i: i32, u: u32, f: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.i = 42i;
    out.u = 100u;
    out.f = 3.14;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "42")
	mustContainMSL(t, code, "100u")
}

// =============================================================================
// Test: Keyword escaping
// =============================================================================

func TestIntegration_KeywordEscaping(t *testing.T) {
	src := `
struct vertex_ { value: f32 };
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    var v: vertex_;
    v.value = 1.0f;
    out.v = v.value;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "struct")
}
