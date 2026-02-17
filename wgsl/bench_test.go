package wgsl

import (
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test shader sources for lexer/parser benchmarks
// ---------------------------------------------------------------------------

const benchShaderSmall = `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`

const benchShaderMedium = `
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var out: VertexOutput;
    var pos = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),
        vec2<f32>(-0.5, -0.5),
        vec2<f32>(0.5, -0.5)
    );
    out.position = vec4<f32>(pos[idx], 0.0, 1.0);
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`

const benchShaderLarge = `
struct Camera {
    view_proj: mat4x4<f32>,
}

struct Light {
    position: vec3<f32>,
    color: vec3<f32>,
}

@group(0) @binding(0) var<uniform> camera: Camera;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) world_pos: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
}

@vertex
fn vs_main(
    @location(0) pos: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
) -> VertexOutput {
    var out: VertexOutput;
    out.position = vec4<f32>(pos.x, pos.y, pos.z, 1.0);
    out.world_pos = pos;
    out.normal = normal;
    out.uv = uv;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let N = normalize(in.normal);
    let light_pos = vec3<f32>(10.0, 10.0, 10.0);
    let light_color = vec3<f32>(1.0, 1.0, 1.0);
    let L = normalize(light_pos - in.world_pos);
    let NdotL = max(dot(N, L), 0.0);
    let diffuse = light_color * NdotL;
    let view_dir = normalize(vec3<f32>(0.0, 0.0, 5.0) - in.world_pos);
    let half_dir = normalize(L + view_dir);
    let NdotH = max(dot(N, half_dir), 0.0);
    let shininess: f32 = 32.0;
    let spec_power = pow(NdotH, shininess);
    let specular = light_color * spec_power;
    let ambient = vec3<f32>(0.05, 0.05, 0.05);
    let base_color = vec3<f32>(0.8, 0.2, 0.2);
    let final_color = ambient + base_color * diffuse + specular * 0.5;
    let tone_mapped = final_color / (final_color + vec3<f32>(1.0, 1.0, 1.0));
    let gamma: f32 = 1.0 / 2.2;
    let corrected = vec3<f32>(
        pow(tone_mapped.x, gamma),
        pow(tone_mapped.y, gamma),
        pow(tone_mapped.z, gamma),
    );
    return vec4<f32>(corrected.x, corrected.y, corrected.z, 1.0);
}
`

type benchCase struct {
	name   string
	source string
}

var benchShaders = []benchCase{
	{"small", benchShaderSmall},
	{"medium", benchShaderMedium},
	{"large", benchShaderLarge},
}

// ---------------------------------------------------------------------------
// Lexer benchmarks
// ---------------------------------------------------------------------------

// BenchmarkLex benchmarks tokenization throughput for shaders of different sizes.
// Reports bytes/sec throughput for comparing across shader sizes.
func BenchmarkLex(b *testing.B) {
	for _, bc := range benchShaders {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				lexer := NewLexer(bc.source)
				tokens, err := lexer.Tokenize()
				if err != nil {
					b.Fatalf("tokenize failed: %v", err)
				}
				runtime.KeepAlive(tokens)
			}
		})
	}
}

// BenchmarkLexIdentifiers benchmarks identifier recognition throughput.
// Uses a synthetic source with many identifiers.
func BenchmarkLexIdentifiers(b *testing.B) {
	// Generate source with many identifiers separated by whitespace
	var sb strings.Builder
	idents := []string{
		"position", "color", "vertex_index", "normal", "world_pos",
		"view_proj", "camera", "light_color", "base_color", "final_val",
		"ambient", "diffuse", "specular", "NdotL", "NdotH",
		"half_dir", "tone_mapped", "corrected", "gamma", "shininess",
	}
	for j := 0; j < 50; j++ {
		for _, id := range idents {
			sb.WriteString(id)
			sb.WriteByte(' ')
		}
	}
	source := sb.String()

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lexer := NewLexer(source)
		tokens, err := lexer.Tokenize()
		if err != nil {
			b.Fatalf("tokenize failed: %v", err)
		}
		runtime.KeepAlive(tokens)
	}
}

// BenchmarkLexNumbers benchmarks number literal parsing throughput.
func BenchmarkLexNumbers(b *testing.B) {
	// Generate source with many number literals
	var sb strings.Builder
	numbers := []string{
		"0", "1", "42", "0u", "255u", "3.14", "0.5", "1.0e10",
		"2.5e-3", "100.0", "0.001", "99999", "0x1F", "0xFF",
		"3.14159265", "2.71828", "1.41421", "0.0", "1.0",
	}
	for j := 0; j < 50; j++ {
		for _, n := range numbers {
			sb.WriteString(n)
			sb.WriteByte(' ')
		}
	}
	source := sb.String()

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lexer := NewLexer(source)
		tokens, err := lexer.Tokenize()
		if err != nil {
			b.Fatalf("tokenize failed: %v", err)
		}
		runtime.KeepAlive(tokens)
	}
}

// BenchmarkLexKeywords benchmarks keyword recognition throughput.
func BenchmarkLexKeywords(b *testing.B) {
	// Generate source with many keywords
	var sb strings.Builder
	keywords := []string{
		"fn", "var", "let", "const", "struct", "return", "if", "else",
		"for", "while", "loop", "break", "continue", "switch", "case",
		"default", "true", "false", "alias", "discard",
	}
	for j := 0; j < 50; j++ {
		for _, kw := range keywords {
			sb.WriteString(kw)
			sb.WriteByte(' ')
		}
	}
	source := sb.String()

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lexer := NewLexer(source)
		tokens, err := lexer.Tokenize()
		if err != nil {
			b.Fatalf("tokenize failed: %v", err)
		}
		runtime.KeepAlive(tokens)
	}
}

// BenchmarkLexOperators benchmarks operator tokenization throughput.
func BenchmarkLexOperators(b *testing.B) {
	var sb strings.Builder
	operators := []string{
		"+ - * / % & | ^ ~ ! = < > . , : ; @",
		"-> ++ -- == != <= >= && || << >>",
		"+= -= *= /= %= &= |= ^= <<= >>=",
	}
	for j := 0; j < 100; j++ {
		for _, ops := range operators {
			sb.WriteString(ops)
			sb.WriteByte(' ')
		}
	}
	source := sb.String()

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lexer := NewLexer(source)
		tokens, err := lexer.Tokenize()
		if err != nil {
			b.Fatalf("tokenize failed: %v", err)
		}
		runtime.KeepAlive(tokens)
	}
}

// ---------------------------------------------------------------------------
// Parser benchmarks
// ---------------------------------------------------------------------------

// BenchmarkParse benchmarks parsing throughput (tokens to AST) for shaders
// of different sizes.
func BenchmarkParse(b *testing.B) {
	for _, bc := range benchShaders {
		b.Run(bc.name, func(b *testing.B) {
			// Pre-tokenize so we only measure parsing
			lexer := NewLexer(bc.source)
			tokens, err := lexer.Tokenize()
			if err != nil {
				b.Fatalf("tokenize failed: %v", err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				parser := NewParser(tokens)
				module, pErr := parser.Parse()
				if pErr != nil {
					b.Fatalf("parse failed: %v", pErr)
				}
				runtime.KeepAlive(module)
			}
		})
	}
}

// BenchmarkParseExpressions benchmarks expression parsing throughput using
// a shader with many arithmetic and function call expressions.
func BenchmarkParseExpressions(b *testing.B) {
	source := `
@fragment
fn main(@location(0) x: f32, @location(1) y: f32, @location(2) z: f32) -> @location(0) vec4<f32> {
    let a = x + y * z - x / y;
    let bb = sin(x) + cos(y) * sqrt(z);
    let c = normalize(vec3<f32>(x, y, z));
    let d = length(c) * dot(c, vec3<f32>(1.0, 0.0, 0.0));
    let e = clamp(a, 0.0, 1.0);
    let f = mix(a, d, 0.5);
    let g = pow(abs(bb), 2.0);
    let h = max(min(e, f), g);
    return vec4<f32>(h, h, h, 1.0);
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		b.Fatalf("tokenize failed: %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		parser := NewParser(tokens)
		module, pErr := parser.Parse()
		if pErr != nil {
			b.Fatalf("parse failed: %v", pErr)
		}
		runtime.KeepAlive(module)
	}
}

// BenchmarkParseStructs benchmarks struct declaration parsing throughput.
func BenchmarkParseStructs(b *testing.B) {
	source := `
struct Camera {
    view_proj: mat4x4<f32>,
    position: vec3<f32>,
    aspect: f32,
}

struct Light {
    position: vec3<f32>,
    color: vec3<f32>,
    intensity: f32,
    radius: f32,
}

struct Material {
    base_color: vec4<f32>,
    roughness: f32,
    metallic: f32,
    emissive: vec3<f32>,
    ao: f32,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) world_pos: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
    @location(3) tangent: vec3<f32>,
}

@vertex
fn main() -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		b.Fatalf("tokenize failed: %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		parser := NewParser(tokens)
		module, pErr := parser.Parse()
		if pErr != nil {
			b.Fatalf("parse failed: %v", pErr)
		}
		runtime.KeepAlive(module)
	}
}

// BenchmarkLexAndParse benchmarks the combined lex+parse pipeline
// to measure total frontend throughput.
func BenchmarkLexAndParse(b *testing.B) {
	for _, bc := range benchShaders {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				lexer := NewLexer(bc.source)
				tokens, err := lexer.Tokenize()
				if err != nil {
					b.Fatalf("tokenize failed: %v", err)
				}

				parser := NewParser(tokens)
				module, pErr := parser.Parse()
				if pErr != nil {
					b.Fatalf("parse failed: %v", pErr)
				}
				runtime.KeepAlive(module)
			}
		})
	}
}

// BenchmarkLower benchmarks AST-to-IR lowering for different shader sizes.
func BenchmarkLower(b *testing.B) {
	for _, bc := range benchShaders {
		b.Run(bc.name, func(b *testing.B) {
			lexer := NewLexer(bc.source)
			tokens, err := lexer.Tokenize()
			if err != nil {
				b.Fatalf("tokenize failed: %v", err)
			}
			parser := NewParser(tokens)
			ast, err := parser.Parse()
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				module, lErr := LowerWithSource(ast, bc.source)
				if lErr != nil {
					b.Fatalf("lower failed: %v", lErr)
				}
				runtime.KeepAlive(module)
			}
		})
	}
}
