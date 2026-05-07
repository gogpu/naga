// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// wgslToGLSL is a test helper that compiles WGSL source to GLSL with options.
func wgslToGLSL(t *testing.T, source string, opts Options) string {
	t.Helper()
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("WGSL tokenize error: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("WGSL parse error: %v", err)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("WGSL lower error: %v", err)
	}
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("GLSL compile error: %v", err)
	}
	return result
}

// glslMustContain checks that GLSL output contains expected substring.
func glslMustContain(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q.\nGot:\n%s", expected, output)
	}
}

// =============================================================================
// Vertex Shader Tests
// =============================================================================

func TestCompileWGSL_VertexShader(t *testing.T) {
	source := `
@vertex
fn vs_main(@location(0) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
    return pos;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace,
	})

	glslMustContain(t, output, "#version 330 core")
	glslMustContain(t, output, "void main()")
	glslMustContain(t, output, "gl_Position")
	glslMustContain(t, output, "gl_Position.yz")
}

func TestCompileWGSL_VertexShaderForcePointSize(t *testing.T) {
	source := `
@vertex
fn vs_main(@location(0) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
    return pos;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagForcePointSize,
	})

	glslMustContain(t, output, "gl_PointSize = 1.0;")
}

// =============================================================================
// Fragment Shader Tests
// =============================================================================

func TestCompileWGSL_FragmentShader(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})

	glslMustContain(t, output, "#version 330 core")
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Compute Shader Tests
// =============================================================================

func TestCompileWGSL_ComputeShader(t *testing.T) {
	source := `
@compute @workgroup_size(64)
fn cs_main(@builtin(global_invocation_id) id: vec3<u32>) {
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})

	glslMustContain(t, output, "#version 430 core")
	glslMustContain(t, output, "layout(local_size_x")
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Variable and Constant Tests
// =============================================================================

func TestCompileWGSL_Constants(t *testing.T) {
	source := `
const PI: f32 = 3.14159;
const MAX_SIZE: u32 = 256u;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let x = PI;
    return vec4<f32>(x, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "const")
}

func TestCompileWGSL_LocalVars(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var x: f32 = 1.0;
    var y: f32 = 2.0;
    x = x + y;
    return vec4<f32>(x, y, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "float")
	glslMustContain(t, output, "return")
}

// =============================================================================
// Control Flow Tests
// =============================================================================

func TestCompileWGSL_IfElse(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    var result: f32;
    if x > 0.5 {
        result = 1.0;
    } else {
        result = 0.0;
    }
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "if (")
	glslMustContain(t, output, "} else {")
}

func TestCompileWGSL_ForLoop(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        sum = sum + 1.0;
        continuing {
            i = i + 1;
        }
    }
    return vec4<f32>(sum, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "while(true)")
	glslMustContain(t, output, "loop_init")
	glslMustContain(t, output, "break;")
}

func TestCompileWGSL_Switch(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: i32) -> @location(0) vec4<f32> {
    var result: f32;
    switch x {
        case 0: {
            result = 0.0;
        }
        case 1: {
            result = 1.0;
        }
        default: {
            result = 0.5;
        }
    }
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "switch(")
	glslMustContain(t, output, "case 0:")
	glslMustContain(t, output, "case 1:")
	glslMustContain(t, output, "default:")
}

// =============================================================================
// Expression Tests
// =============================================================================

func TestCompileWGSL_BinaryOps(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a = 1.0 + 2.0;
    let b = 3.0 * 4.0;
    let c = 5.0 - 1.0;
    let d = 6.0 / 2.0;
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

func TestCompileWGSL_UnaryOps(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a = -1.0;
    let b = !true;
    return vec4<f32>(a, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

func TestCompileWGSL_MathFunctions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = sin(x);
    let b = cos(x);
    let c = abs(x);
    let d = max(a, b);
    let e = min(a, b);
    let f = clamp(x, 0.0, 1.0);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "sin(")
	glslMustContain(t, output, "cos(")
	glslMustContain(t, output, "abs(")
	glslMustContain(t, output, "max(")
	glslMustContain(t, output, "min(")
	glslMustContain(t, output, "clamp(")
}

func TestCompileWGSL_VectorOperations(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec3<f32>(1.0, 2.0, 3.0);
    let w = vec3<f32>(4.0, 5.0, 6.0);
    let d = dot(v, w);
    let c = cross(v, w);
    let n = normalize(v);
    let l = length(v);
    return vec4<f32>(d, l, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "dot(")
	glslMustContain(t, output, "cross(")
	glslMustContain(t, output, "normalize(")
	glslMustContain(t, output, "length(")
}

func TestCompileWGSL_Swizzle(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let xy = v.xy;
    let zw = v.zw;
    return vec4<f32>(xy, zw);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, ".xy")
	glslMustContain(t, output, ".zw")
}

func TestCompileWGSL_Select(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let result = select(0.0, 1.0, x > 0.5);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	// select maps to ternary or mix in GLSL
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Struct and Uniform Tests
// =============================================================================

func TestCompileWGSL_UniformStruct(t *testing.T) {
	source := `
struct Uniforms {
    model: mat4x4<f32>,
    color: vec4<f32>,
};

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(@location(0) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
    return uniforms.model * pos;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "uniform")
	glslMustContain(t, output, "mat4x4")
}

func TestCompileWGSL_StorageBuffer(t *testing.T) {
	source := `
struct Data {
    values: array<f32>,
};

@group(0) @binding(0) var<storage, read> data: Data;

@compute @workgroup_size(64)
fn cs_main(@builtin(global_invocation_id) id: vec3<u32>) {
    let x = data.values[id.x];
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "layout(std430)")
	glslMustContain(t, output, "buffer")
}

// =============================================================================
// Type Conversion Tests
// =============================================================================

func TestCompileWGSL_TypeCast(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let i: i32 = 42;
    let f = f32(i);
    let u: u32 = u32(i);
    return vec4<f32>(f, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "float(")
}

// =============================================================================
// ES Version Tests
// =============================================================================

func TestCompileWGSL_ES300_Precision(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion:        VersionES300,
		ForceHighPrecision: true,
	})
	glslMustContain(t, output, "#version 300 es")
	glslMustContain(t, output, "precision highp float;")
}

func TestCompileWGSL_ES310_Compute(t *testing.T) {
	source := `
@compute @workgroup_size(1)
fn cs_main() {
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: VersionES310})
	glslMustContain(t, output, "#version 310 es")
	glslMustContain(t, output, "layout(local_size_x")
}

// =============================================================================
// Function Call Tests
// =============================================================================

func TestCompileWGSL_FunctionCall(t *testing.T) {
	source := `
fn helper(x: f32) -> f32 {
    return x * 2.0;
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let result = helper(0.5);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "float helper")
	glslMustContain(t, output, "helper(")
}

// =============================================================================
// Derivative Tests
// =============================================================================

func TestCompileWGSL_Derivatives(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dx = dpdx(uv.x);
    let dy = dpdy(uv.y);
    return vec4<f32>(dx, dy, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "dFdx(")
	glslMustContain(t, output, "dFdy(")
}

// =============================================================================
// Discard Tests
// =============================================================================

func TestCompileWGSL_Discard(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) alpha: f32) -> @location(0) vec4<f32> {
    if alpha < 0.5 {
        discard;
    }
    return vec4<f32>(1.0, 1.0, 1.0, alpha);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "discard;")
}

// =============================================================================
// Array Tests
// =============================================================================

func TestCompileWGSL_FixedArray(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var arr: array<f32, 4>;
    arr[0] = 1.0;
    arr[1] = 2.0;
    arr[2] = 3.0;
    arr[3] = 4.0;
    return vec4<f32>(arr[0], arr[1], arr[2], arr[3]);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "float")
	glslMustContain(t, output, "[4]")
}

// =============================================================================
// Matrix Operations Tests
// =============================================================================

func TestCompileWGSL_MatrixMultiply(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let m = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    let v = vec2<f32>(1.0, 2.0);
    let result = m * v;
    return vec4<f32>(result, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "mat2x2")
}

// =============================================================================
// Splat Tests
// =============================================================================

func TestCompileWGSL_Splat(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec4<f32>(0.5);
    return v;
}
`
	// vec4(0.5) is a splat in WGSL — single value broadcast to all components
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "vec4(")
}

// =============================================================================
// Struct Return Tests
// =============================================================================

func TestCompileWGSL_StructReturn(t *testing.T) {
	source := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec4<f32>,
};

@vertex
fn vs_main(@location(0) pos: vec4<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = pos;
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    return out;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_Position")
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Struct Input Tests
// =============================================================================

func TestCompileWGSL_StructInput(t *testing.T) {
	source := `
struct FragInput {
    @location(0) color: vec4<f32>,
    @location(1) uv: vec2<f32>,
};

@fragment
fn fs_main(input: FragInput) -> @location(0) vec4<f32> {
    return input.color;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Workgroup Variable Tests
// =============================================================================

func TestCompileWGSL_WorkgroupVar(t *testing.T) {
	source := `
var<workgroup> shared_data: array<f32, 64>;

@compute @workgroup_size(64)
fn cs_main(@builtin(local_invocation_id) lid: vec3<u32>) {
    shared_data[lid.x] = f32(lid.x);
    workgroupBarrier();
    let val = shared_data[lid.x];
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "shared")
	glslMustContain(t, output, "barrier()")
}

// =============================================================================
// BindingMap Tests (end-to-end)
// =============================================================================

func TestCompileWGSL_BindingMap(t *testing.T) {
	source := `
@group(0) @binding(0) var<uniform> u_data: vec4<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return u_data;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version{Major: 4, Minor: 50},
		BindingMap: map[BindingMapKey]uint8{
			{Group: 0, Binding: 0}: 3,
		},
	})
	glslMustContain(t, output, "binding = 3")
}

// =============================================================================
// Relational Functions Tests
// =============================================================================

func TestCompileWGSL_RelationalOps(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    var result: f32 = 0.0;
    if x > 0.0 && x < 1.0 {
        result = 1.0;
    }
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "if (")
}

// =============================================================================
// Modulo/Division Helper Tests
// =============================================================================

func TestCompileWGSL_FloatModulo(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let result = x % 2.0;
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	// Float modulo needs naga_mod helper
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Literal Type Coverage Tests
// =============================================================================

func TestCompileWGSL_IntLiterals(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let i: i32 = -42;
    let u: u32 = 100u;
    let f: f32 = 3.14;
    let b: bool = true;
    return vec4<f32>(f32(i), f32(u), f, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Texture Sampling Tests
// =============================================================================

func TestCompileWGSL_TextureSample(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texture(")
	glslMustContain(t, output, "sampler2D")
}

// =============================================================================
// Reachability / Dead Code Tests
// =============================================================================

func TestCompileWGSL_UnusedFunction(t *testing.T) {
	source := `
fn unused_helper() -> f32 {
    return 999.0;
}

fn used_helper(x: f32) -> f32 {
    return x * 2.0;
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let r = used_helper(0.5);
    return vec4<f32>(r, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "used_helper")
	// unused_helper may or may not appear depending on DCE
}

// =============================================================================
// Modf/Frexp Helper Tests (end-to-end)
// =============================================================================

func TestCompileWGSL_Modf(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let result = modf(x);
    return vec4<f32>(result.fract, result.whole, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "naga_modf")
}

func TestCompileWGSL_Frexp(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let result = frexp(x);
    return vec4<f32>(result.fract, f32(result.exp), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "naga_frexp")
}

// =============================================================================
// Mix/Step/Smoothstep Tests
// =============================================================================

func TestCompileWGSL_MixStepSmoothstep(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = mix(0.0, 1.0, x);
    let b = step(0.5, x);
    let c = smoothstep(0.0, 1.0, x);
    return vec4<f32>(a, b, c, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "mix(")
	glslMustContain(t, output, "step(")
	glslMustContain(t, output, "smoothstep(")
}

// =============================================================================
// Pow/Exp/Log Tests
// =============================================================================

func TestCompileWGSL_PowExpLog(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = pow(x, 2.0);
    let b = exp(x);
    let c = log(x);
    let d = sqrt(x);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "pow(")
	glslMustContain(t, output, "exp(")
	glslMustContain(t, output, "log(")
	glslMustContain(t, output, "sqrt(")
}

// =============================================================================
// Floor/Ceil/Round/Trunc Tests
// =============================================================================

func TestCompileWGSL_FloorCeilRound(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = floor(x);
    let b = ceil(x);
    let c = round(x);
    let d = trunc(x);
    return vec4<f32>(a, b, c, d);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "floor(")
	glslMustContain(t, output, "ceil(")
	glslMustContain(t, output, "round(")
	glslMustContain(t, output, "trunc(")
}

// =============================================================================
// Struct Field Access Tests (writeAccessIndex)
// =============================================================================

func TestCompileWGSL_StructFieldAccess(t *testing.T) {
	source := `
struct MyData {
    x: f32,
    y: f32,
    z: f32,
};

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var data: MyData;
    data.x = 1.0;
    data.y = 2.0;
    data.z = 3.0;
    return vec4<f32>(data.x, data.y, data.z, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Vector Component Access Tests
// =============================================================================

func TestCompileWGSL_VectorAccess(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let x = v.x;
    let y = v.y;
    let z = v.z;
    let w = v.w;
    return vec4<f32>(x, y, z, w);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Array Dynamic Access Tests (writeAccess)
// =============================================================================

func TestCompileWGSL_DynamicArrayAccess(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) idx: i32) -> @location(0) vec4<f32> {
    var arr: array<f32, 8>;
    arr[0] = 1.0;
    arr[1] = 2.0;
    let val = arr[idx];
    return vec4<f32>(val, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Bitcast Tests (writeAs)
// =============================================================================

func TestCompileWGSL_Bitcast(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let i: i32 = 42;
    let u = bitcast<u32>(i);
    let f = bitcast<f32>(i);
    return vec4<f32>(f, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Fine/Coarse Derivative Tests
// =============================================================================

func TestCompileWGSL_DerivativesFineCoarse(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dx = dpdxFine(uv.x);
    let dy = dpdyFine(uv.y);
    let dxc = dpdxCoarse(uv.x);
    let dyc = dpdyCoarse(uv.y);
    return vec4<f32>(dx, dy, dxc, dyc);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version450})
	glslMustContain(t, output, "dFdxFine(")
	glslMustContain(t, output, "dFdyFine(")
	glslMustContain(t, output, "dFdxCoarse(")
	glslMustContain(t, output, "dFdyCoarse(")
}

// =============================================================================
// Integer Division Helper Tests
// =============================================================================

func TestCompileWGSL_IntegerDivision(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: i32 = 7;
    let b: i32 = 3;
    let div = a / b;
    return vec4<f32>(f32(div), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Comparison Operator Tests
// =============================================================================

func TestCompileWGSL_ComparisonOps(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = x > 0.0;
    let b = x < 1.0;
    let c = x >= 0.5;
    let d = x <= 0.5;
    let e = x == 0.0;
    let f = x != 1.0;
    var r: f32 = 0.0;
    if a {
        r = 1.0;
    }
    return vec4<f32>(r, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Bitwise Operation Tests
// =============================================================================

func TestCompileWGSL_BitwiseOps(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: u32 = 0xFFu;
    let b: u32 = 0x0Fu;
    let c = a & b;
    let d = a | b;
    let e = a ^ b;
    let f = ~a;
    let g = a << 2u;
    let h = a >> 2u;
    return vec4<f32>(f32(c), f32(d), f32(e), 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// CountOneBits / ReverseBits Tests
// =============================================================================

func TestCompileWGSL_BitOperations(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: u32 = 0xFFu;
    let bits = countOneBits(a);
    let rev = reverseBits(a);
    return vec4<f32>(f32(bits), f32(rev), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "bitCount(")
	glslMustContain(t, output, "bitfieldReverse(")
}

// =============================================================================
// firstLeadingBit / firstTrailingBit Tests
// =============================================================================

func TestCompileWGSL_FirstBitOps(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: u32 = 0xF0u;
    let fb = firstTrailingBit(a);
    return vec4<f32>(f32(fb), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "findLSB(")
}

// =============================================================================
// Function With Multiple Arguments Tests
// =============================================================================

func TestCompileWGSL_FunctionMultipleArgs(t *testing.T) {
	source := `
fn add3(a: f32, b: f32, c: f32) -> f32 {
    return a + b + c;
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let result = add3(1.0, 2.0, 3.0);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "float add3")
}

// =============================================================================
// ArrayLength Test
// =============================================================================

func TestCompileWGSL_ArrayLength(t *testing.T) {
	source := `
struct Buffer {
    data: array<f32>,
};

@group(0) @binding(0) var<storage, read> buf: Buffer;

@compute @workgroup_size(1)
fn cs_main() {
    let len = arrayLength(&buf.data);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Texture Dimension Query Tests
// =============================================================================

func TestCompileWGSL_TextureDimensions(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let size = textureDimensions(tex);
    return vec4<f32>(f32(size.x), f32(size.y), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "textureSize(")
}

// =============================================================================
// Early Depth Test
// =============================================================================

func TestCompileWGSL_EarlyDepthTest(t *testing.T) {
	source := `
@early_depth_test
@fragment
fn fs_main() -> @builtin(frag_depth) f32 {
    return 0.5;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version420})
	glslMustContain(t, output, "layout(early_fragment_tests)")
}

// =============================================================================
// Matrix Column Access Tests
// =============================================================================

func TestCompileWGSL_MatrixColumnAccess(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let m = mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        0.0, 0.0, 0.0, 1.0,
    );
    let col = m[0];
    return col;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Multiple Entry Points (reachability test)
// =============================================================================

func TestCompileWGSL_SelectNamedEntryPoint(t *testing.T) {
	source := `
@vertex
fn vs_main(@location(0) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
    return pos;
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	// Select fragment entry point
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		EntryPoint:  "fs_main",
	})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Inverse/Determinant/Transpose Tests
// =============================================================================

func TestCompileWGSL_MatrixOps(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let m = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    let det = determinant(m);
    let trans = transpose(m);
    return vec4<f32>(det, trans[0].x, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "determinant(")
	glslMustContain(t, output, "transpose(")
}

// =============================================================================
// Sign/Fract/Inversesqrt Tests
// =============================================================================

func TestCompileWGSL_MathMore(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = sign(x);
    let b = fract(x);
    let c = inverseSqrt(x);
    return vec4<f32>(a, b, c, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "sign(")
	glslMustContain(t, output, "fract(")
	glslMustContain(t, output, "inversesqrt(")
}

// =============================================================================
// Atan2 / Ldexp Tests
// =============================================================================

func TestCompileWGSL_Atan2Ldexp(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = atan2(x, 1.0);
    let b = ldexp(x, 2i);
    return vec4<f32>(a, b, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "atan(")
	glslMustContain(t, output, "ldexp(")
}

// =============================================================================
// Builtin Inputs Tests
// =============================================================================

func TestCompileWGSL_VertexIndex(t *testing.T) {
	source := `
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(vid), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_VertexID")
}

func TestCompileWGSL_FrontFacing(t *testing.T) {
	source := `
@fragment
fn fs_main(@builtin(front_facing) ff: bool) -> @location(0) vec4<f32> {
    if ff {
        return vec4<f32>(1.0, 0.0, 0.0, 1.0);
    }
    return vec4<f32>(0.0, 0.0, 1.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_FrontFacing")
}

// =============================================================================
// Let (Named Expression) Tests
// =============================================================================

func TestCompileWGSL_NamedExpressions(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let alpha = clamp(x, 0.0, 1.0);
    let doubled = alpha * 2.0;
    return vec4<f32>(doubled, doubled, doubled, alpha);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "alpha")
	glslMustContain(t, output, "doubled")
}

// =============================================================================
// Nested Struct Access Tests
// =============================================================================

func TestCompileWGSL_NestedStruct(t *testing.T) {
	source := `
struct Inner {
    value: f32,
};

struct Outer {
    inner: Inner,
    scale: f32,
};

@group(0) @binding(0) var<uniform> data: Outer;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = data.inner.value * data.scale;
    return vec4<f32>(v, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Fwidth Test
// =============================================================================

func TestCompileWGSL_Fwidth(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let fw = fwidth(x);
    return vec4<f32>(fw, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "fwidth(")
}

// =============================================================================
// Pack/Unpack Tests
// =============================================================================

func TestCompileWGSL_Pack4x8(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v = vec4<f32>(0.0, 0.25, 0.5, 1.0);
    let packed = pack4x8snorm(v);
    return vec4<f32>(f32(packed), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version400})
	glslMustContain(t, output, "packSnorm4x8(")
}

// =============================================================================
// Saturate / Distance / Reflect Tests
// =============================================================================

func TestCompileWGSL_DistanceReflect(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let d = distance(uv, vec2<f32>(0.5, 0.5));
    let n = vec2<f32>(0.0, 1.0);
    let r = reflect(uv, n);
    return vec4<f32>(d, r.x, r.y, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "distance(")
	glslMustContain(t, output, "reflect(")
}

// =============================================================================
// extractBits / insertBits Tests
// =============================================================================

func TestCompileWGSL_ExtractInsertBits(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let v: u32 = 0xFFu;
    let extracted = extractBits(v, 0u, 4u);
    let inserted = insertBits(v, 0xAu, 4u, 4u);
    return vec4<f32>(f32(extracted), f32(inserted), 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "bitfieldExtract(")
	glslMustContain(t, output, "bitfieldInsert(")
}

// =============================================================================
// textureLoad Tests (writeImageLoad)
// =============================================================================

func TestCompileWGSL_TextureLoad(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let color = textureLoad(tex, vec2<i32>(0, 0), 0);
    return color;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texelFetch(")
}

// =============================================================================
// Atomic Operations Tests
// =============================================================================

func TestCompileWGSL_AtomicOps(t *testing.T) {
	source := `
var<workgroup> counter: atomic<u32>;

@compute @workgroup_size(64)
fn cs_main(@builtin(local_invocation_index) idx: u32) {
    let old = atomicAdd(&counter, 1u);
    let val = atomicLoad(&counter);
    atomicStore(&counter, 0u);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "atomicAdd(")
}

// =============================================================================
// Texture Num Levels Tests
// =============================================================================

func TestCompileWGSL_TextureNumLevels(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let levels = textureNumLevels(tex);
    return vec4<f32>(f32(levels), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "textureQueryLevels(")
}

// =============================================================================
// Clamp/Min/Max Integer Tests
// =============================================================================

func TestCompileWGSL_IntegerMinMax(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: i32 = 5;
    let b: i32 = 10;
    let mn = min(a, b);
    let mx = max(a, b);
    let cl = clamp(a, 0, 20);
    return vec4<f32>(f32(mn), f32(mx), f32(cl), 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "min(")
	glslMustContain(t, output, "max(")
	glslMustContain(t, output, "clamp(")
}

// =============================================================================
// Relational IsNan/IsInf Tests
// =============================================================================

func TestCompileWGSL_RelationalFunctions(t *testing.T) {
	// Test relational operations that cannot be const-folded
	source := `
@fragment
fn fs_main(@location(0) x: f32, @location(1) y: f32) -> @location(0) vec4<f32> {
    var result: f32 = 0.0;
    let checks = vec2<bool>(x > 0.0, y > 0.0);
    let all_ok = all(checks);
    if all_ok {
        result = 1.0;
    }
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "all(")
}

// =============================================================================
// Multiple Location Outputs Tests
// =============================================================================

func TestCompileWGSL_MultipleOutputLocations(t *testing.T) {
	source := `
struct FragOutput {
    @location(0) color: vec4<f32>,
    @location(1) normal: vec4<f32>,
};

@fragment
fn fs_main() -> FragOutput {
    var out: FragOutput;
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    out.normal = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    return out;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Storage Image Tests
// =============================================================================

func TestCompileWGSL_StorageTexture(t *testing.T) {
	source := `
@group(0) @binding(0) var img: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(8, 8)
fn cs_main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(img, vec2<i32>(i32(id.x), i32(id.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "imageStore(")
	glslMustContain(t, output, "image2D")
}

// =============================================================================
// Depth Texture Tests
// =============================================================================

func TestCompileWGSL_DepthTextureSample(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_depth_2d;
@group(0) @binding(1) var samp: sampler_comparison;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(tex, samp, uv, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "Shadow")
	glslMustContain(t, output, "texture(")
}

// =============================================================================
// Saturate Test
// =============================================================================

func TestCompileWGSL_Saturate(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let s = saturate(x);
    return vec4<f32>(s, s, s, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "clamp(")
}

// =============================================================================
// While Loop with break_if Test
// =============================================================================

func TestCompileWGSL_LoopBreakIf(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    var i: i32 = 0;
    loop {
        i = i + 1;
        continuing {
            break if i >= 10;
        }
    }
    return vec4<f32>(f32(i), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "while(true)")
	glslMustContain(t, output, "break;")
}

// =============================================================================
// Pipeline Constants Tests
// =============================================================================

func TestCompileWGSL_PipelineOverrides(t *testing.T) {
	source := `
override width: f32 = 640.0;

@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(x / width, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion:       Version330,
		PipelineConstants: map[string]float64{"width": 1920.0},
	})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Nested If Tests
// =============================================================================

func TestCompileWGSL_NestedIf(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    var r: f32 = 0.0;
    if x > 0.0 {
        if x > 0.5 {
            r = 1.0;
        } else {
            r = 0.5;
        }
    } else {
        r = 0.0;
    }
    return vec4<f32>(r, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "if (")
	glslMustContain(t, output, "} else {")
}

// =============================================================================
// ES Version without highp precision
// =============================================================================

func TestCompileWGSL_ES300_NoHighP(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion:        VersionES300,
		ForceHighPrecision: false,
	})
	glslMustContain(t, output, "#version 300 es")
	// With ForceHighPrecision=false, should still have some precision qualifier
	glslMustContain(t, output, "precision")
}

// =============================================================================
// Multiple Uniform Buffers with BindingMap
// =============================================================================

// =============================================================================
// Uniform Block with Struct Tests (writeUniformBlock path)
// =============================================================================

func TestCompileWGSL_UniformBlockStruct(t *testing.T) {
	source := `
struct Camera {
    view: mat4x4<f32>,
    proj: mat4x4<f32>,
    position: vec3<f32>,
    fov: f32,
};

@group(0) @binding(0) var<uniform> camera: Camera;

@vertex
fn vs_main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    let clip = camera.proj * camera.view * vec4<f32>(pos, 1.0);
    return clip;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "uniform")
	glslMustContain(t, output, "mat4x4")
	glslMustContain(t, output, "gl_Position")
}

// =============================================================================
// Vertex/Fragment IO with Multiple Locations
// =============================================================================

func TestCompileWGSL_VertexFragmentIO(t *testing.T) {
	source := `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
};

struct VertexOutput {
    @builtin(position) clip_pos: vec4<f32>,
    @location(0) world_normal: vec3<f32>,
    @location(1) tex_coord: vec2<f32>,
};

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.clip_pos = vec4<f32>(input.position, 1.0);
    out.world_normal = input.normal;
    out.tex_coord = input.uv;
    return out;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_Position")
	glslMustContain(t, output, "layout(location = 0)")
	glslMustContain(t, output, "layout(location = 1)")
	glslMustContain(t, output, "layout(location = 2)")
}

func TestCompileWGSL_FragmentInputOutput(t *testing.T) {
	source := `
struct FragInput {
    @location(0) normal: vec3<f32>,
    @location(1) uv: vec2<f32>,
};

@fragment
fn fs_main(input: FragInput) -> @location(0) vec4<f32> {
    let light = dot(input.normal, vec3<f32>(0.0, 1.0, 0.0));
    return vec4<f32>(light, light, light, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "dot(")
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Compute with Workgroup Shared Memory
// =============================================================================

func TestCompileWGSL_ComputeWorkgroupInit(t *testing.T) {
	source := `
var<workgroup> tile: array<f32, 256>;

@compute @workgroup_size(16, 16)
fn cs_main(
    @builtin(local_invocation_id) lid: vec3<u32>,
    @builtin(workgroup_id) wid: vec3<u32>,
) {
    let idx = lid.y * 16u + lid.x;
    tile[idx] = f32(idx);
    workgroupBarrier();
    let val = tile[idx];
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "shared")
	glslMustContain(t, output, "barrier()")
	glslMustContain(t, output, "layout(local_size_x = 16, local_size_y = 16")
}

// =============================================================================
// Integer Division / Modulo Helper Tests
// =============================================================================

func TestCompileWGSL_IntegerModulo(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let a: i32 = 7;
    let b: i32 = 3;
    let rem = a % b;
    return vec4<f32>(f32(rem), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Exp2/Log2 Tests
// =============================================================================

func TestCompileWGSL_Exp2Log2(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = exp2(x);
    let b = log2(x);
    return vec4<f32>(a, b, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "exp2(")
	glslMustContain(t, output, "log2(")
}

// =============================================================================
// Unary Negation on Different Types
// =============================================================================

func TestCompileWGSL_UnaryNegation(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let neg_f = -x;
    let neg_v = -vec3<f32>(x, x, x);
    return vec4<f32>(neg_f, neg_v.x, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "-(")
}

// =============================================================================
// Multiple Textures + Samplers Test
// =============================================================================

func TestCompileWGSL_MultipleTextures(t *testing.T) {
	source := `
@group(0) @binding(0) var tex_a: texture_2d<f32>;
@group(0) @binding(1) var tex_b: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = textureSample(tex_a, samp, uv);
    let b = textureSample(tex_b, samp, uv);
    return a + b;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "texture(")
}

// =============================================================================
// Vertex Shader with Coordinate Space Adjust and PointSize
// =============================================================================

func TestCompileWGSL_VertexAdjustAndPointSize(t *testing.T) {
	source := `
struct VS_Out {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec3<f32>,
};

@vertex
fn vs_main(@location(0) pos: vec3<f32>, @location(1) col: vec3<f32>) -> VS_Out {
    var out: VS_Out;
    out.pos = vec4<f32>(pos, 1.0);
    out.color = col;
    return out;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace | WriterFlagForcePointSize,
	})
	glslMustContain(t, output, "gl_Position.yz")
	glslMustContain(t, output, "gl_PointSize = 1.0;")
}

func TestCompileWGSL_MultipleUniformsBindingMap(t *testing.T) {
	source := `
@group(0) @binding(0) var<uniform> a: vec4<f32>;
@group(0) @binding(1) var<uniform> b: vec4<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return a + b;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version{Major: 4, Minor: 50},
		BindingMap: map[BindingMapKey]uint8{
			{Group: 0, Binding: 0}: 0,
			{Group: 0, Binding: 1}: 1,
		},
	})
	glslMustContain(t, output, "binding = 0")
	glslMustContain(t, output, "binding = 1")
}

// =============================================================================
// Bounds Check Policy Tests (writeImageLoad paths)
// =============================================================================

func TestCompileWGSL_TextureLoadRestrict(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let color = textureLoad(tex, vec2<i32>(0, 0), 0);
    return color;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version430,
		BoundsCheckPolicies: BoundsCheckPolicies{
			ImageLoad: BoundsCheckRestrict,
		},
	})
	glslMustContain(t, output, "clamp(")
}

func TestCompileWGSL_TextureLoadReadZero(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    let color = textureLoad(tex, vec2<i32>(0, 0), 0);
    return color;
}
`
	output := wgslToGLSL(t, source, Options{
		LangVersion: Version430,
		BoundsCheckPolicies: BoundsCheckPolicies{
			ImageLoad: BoundsCheckReadZeroSkipWrite,
		},
	})
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// QuantizeToF16 Tests (bake forcing)
// =============================================================================

func TestCompileWGSL_QuantizeToF16(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let q = quantizeToF16(x);
    return vec4<f32>(q, 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	// quantizeToF16 maps to unpackHalf2x16(packHalf2x16(...))
	glslMustContain(t, output, "void main()")
}

// =============================================================================
// Asin/Acos/Atan Tests
// =============================================================================

func TestCompileWGSL_TrigInverse(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = asin(x);
    let b = acos(x);
    let c = atan(x);
    return vec4<f32>(a, b, c, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "asin(")
	glslMustContain(t, output, "acos(")
	glslMustContain(t, output, "atan(")
}

// =============================================================================
// Sinh/Cosh/Tanh Tests
// =============================================================================

func TestCompileWGSL_Hyperbolic(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = sinh(x);
    let b = cosh(x);
    let c = tanh(x);
    return vec4<f32>(a, b, c, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "sinh(")
	glslMustContain(t, output, "cosh(")
	glslMustContain(t, output, "tanh(")
}

// =============================================================================
// Acosh/Asinh/Atanh Tests
// =============================================================================

func TestCompileWGSL_HyperbolicInverse(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let a = asinh(x);
    let b = acosh(x + 2.0);
    let c = atanh(x * 0.5);
    return vec4<f32>(a, b, c, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "asinh(")
	glslMustContain(t, output, "acosh(")
	glslMustContain(t, output, "atanh(")
}

// =============================================================================
// Smoothstep / Refract Tests
// =============================================================================

func TestCompileWGSL_RefractFaceforward(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let i = vec3<f32>(1.0, 0.0, 0.0);
    let n = vec3<f32>(0.0, 1.0, 0.0);
    let r = refract(i, n, 1.5);
    let ff = faceForward(n, i, n);
    return vec4<f32>(r.x, ff.x, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "refract(")
	glslMustContain(t, output, "faceforward(")
}

// =============================================================================
// Texture with sample_index + sample_mask Tests
// =============================================================================

func TestCompileWGSL_SampleIndex(t *testing.T) {
	source := `
@fragment
fn fs_main(@builtin(sample_index) sid: u32) -> @location(0) vec4<f32> {
    return vec4<f32>(f32(sid), 0.0, 0.0, 1.0);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_SampleID")
}

// =============================================================================
// FragDepth Output Test
// =============================================================================

func TestCompileWGSL_FragDepthOutput(t *testing.T) {
	source := `
@fragment
fn fs_main() -> @builtin(frag_depth) f32 {
    return 0.5;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "gl_FragDepth")
}

// =============================================================================
// Mix with Boolean Condition Tests
// =============================================================================

func TestCompileWGSL_ConditionalAssign(t *testing.T) {
	source := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    var r = vec4<f32>(0.0);
    if x > 0.5 {
        r = vec4<f32>(1.0);
    }
    return r;
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version330})
	glslMustContain(t, output, "if (")
}
