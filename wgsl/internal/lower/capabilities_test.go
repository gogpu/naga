package lower

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// compileWithCaps parses and lowers WGSL source with the given capabilities.
func compileWithCaps(t *testing.T, src string, caps ir.Capabilities) (*ir.Module, error) {
	t.Helper()
	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		return nil, err
	}
	return LowerWithCapabilities(ast, src, caps)
}

func TestCapabilities_ScalarWidthValidation(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		caps      ir.Capabilities
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "f64 without CapFloat64 rejected",
			source:    `fn test() { let x: f64 = 1.0; }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "FLOAT64",
		},
		{
			name:   "f64 with CapFloat64 accepted",
			source: `fn test() { let x: f64 = 1.0; }`,
			caps:   ir.CapFloat64,
		},
		{
			name:      "i64 without CapShaderInt64 rejected",
			source:    `fn test() { let x: i64 = 1; }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_INT64",
		},
		{
			name:   "i64 with CapShaderInt64 accepted",
			source: `fn test() { let x: i64 = 1; }`,
			caps:   ir.CapShaderInt64,
		},
		{
			name:      "u64 without CapShaderInt64 rejected",
			source:    `fn test() { let x: u64 = 1; }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_INT64",
		},
		{
			name:   "u64 with CapShaderInt64 accepted",
			source: `fn test() { let x: u64 = 1; }`,
			caps:   ir.CapShaderInt64,
		},
		{
			name:      "f16 without CapShaderFloat16 rejected",
			source:    `fn test() { let x: f16 = 1.0h; }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_FLOAT16",
		},
		{
			name:   "f16 with CapShaderFloat16 accepted",
			source: `fn test() { let x: f16 = 1.0h; }`,
			caps:   ir.CapShaderFloat16,
		},
		{
			name:   "f32 without any caps accepted",
			source: `fn test() { let x: f32 = 1.0; }`,
			caps:   0,
		},
		{
			name:   "i32 without any caps accepted",
			source: `fn test() { let x: i32 = 1; }`,
			caps:   0,
		},
		{
			name:   "u32 without any caps accepted",
			source: `fn test() { let x: u32 = 1u; }`,
			caps:   0,
		},
		{
			name:   "bool without any caps accepted",
			source: `fn test() { let x: bool = true; }`,
			caps:   0,
		},
		{
			name:      "f64 in struct field rejected",
			source:    `struct S { x: f64, }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "FLOAT64",
		},
		{
			name:   "f64 in struct field with cap accepted",
			source: `struct S { x: f64, }`,
			caps:   ir.CapFloat64,
		},
		{
			name:      "f64 as function parameter rejected",
			source:    `fn test(x: f64) {}`,
			caps:      0,
			wantErr:   true,
			errSubstr: "FLOAT64",
		},
		{
			name:   "f64 as function parameter with cap accepted",
			source: `fn test(x: f64) {}`,
			caps:   ir.CapFloat64,
		},
		{
			name:      "i64 as function return type rejected",
			source:    `fn test() -> i64 { return 0; }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_INT64",
		},
		{
			name:   "i64 as function return type with cap accepted",
			source: `fn test() -> i64 { return 0; }`,
			caps:   ir.CapShaderInt64,
		},
		{
			name:      "u64 in vec2 rejected",
			source:    `fn test() { let v: vec2<u64> = vec2<u64>(0, 0); }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_INT64",
		},
		{
			name:   "u64 in vec2 with cap accepted",
			source: `fn test() { let v: vec2<u64> = vec2<u64>(0, 0); }`,
			caps:   ir.CapShaderInt64,
		},
		{
			name:      "f16 in vec4 rejected",
			source:    `fn test() { let v: vec4<f16> = vec4<f16>(0.0h, 0.0h, 0.0h, 0.0h); }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_FLOAT16",
		},
		{
			name:   "f16 in vec4 with cap accepted",
			source: `fn test() { let v: vec4<f16> = vec4<f16>(0.0h, 0.0h, 0.0h, 0.0h); }`,
			caps:   ir.CapShaderFloat16,
		},
		{
			name:   "CapAll permits everything",
			source: `fn test() { let a: f64 = 1.0; let b: i64 = 2; let c: u64 = 3; }`,
			caps:   ir.CapAll,
		},
		{
			name:   "multiple caps can be combined",
			source: `fn test() { let a: f64 = 1.0; let b: i64 = 2; }`,
			caps:   ir.CapFloat64 | ir.CapShaderInt64,
		},
		{
			name:      "vec4h short alias rejected without f16 cap",
			source:    `fn test() { let v: vec4h = vec4h(0.0h, 0.0h, 0.0h, 0.0h); }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_FLOAT16",
		},
		{
			name:   "vec4h short alias accepted with f16 cap",
			source: `fn test() { let v: vec4h = vec4h(0.0h, 0.0h, 0.0h, 0.0h); }`,
			caps:   ir.CapShaderFloat16,
		},
		{
			name:      "mat4x4h short alias rejected without f16 cap",
			source:    `fn test() { var m: mat4x4h; }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_FLOAT16",
		},
		{
			name:      "f64 constructor call rejected",
			source:    `fn test() { let x = f64(1.0); }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "FLOAT64",
		},
		{
			name:   "f64 constructor call with cap accepted",
			source: `fn test() { let x = f64(1.0); }`,
			caps:   ir.CapFloat64,
		},
		{
			name:      "i64 constructor call rejected",
			source:    `fn test() { let x = i64(1); }`,
			caps:      0,
			wantErr:   true,
			errSubstr: "SHADER_INT64",
		},
		{
			name:   "i64 constructor call with cap accepted",
			source: `fn test() { let x = i64(1); }`,
			caps:   ir.CapShaderInt64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileWithCaps(t, tt.source, tt.caps)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got success", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCapabilities_BackwardCompatibility(t *testing.T) {
	// LowerWithSource (no explicit caps) must accept everything (permissive mode).
	// This ensures existing callers are not broken.
	src := `fn test() { let a: f64 = 1.0; let b: i64 = 2; let c: u64 = 3; }`

	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}

	// LowerWithSource is permissive — should not error on 64-bit types.
	module, err := LowerWithSource(ast, src)
	if err != nil {
		t.Fatalf("LowerWithSource should be permissive but got error: %v", err)
	}
	if module == nil {
		t.Fatal("LowerWithSource returned nil module")
	}
}

func TestCapabilities_Contains(t *testing.T) {
	tests := []struct {
		name string
		c    ir.Capabilities
		flag ir.Capabilities
		want bool
	}{
		{"zero contains nothing", 0, ir.CapFloat64, false},
		{"CapFloat64 contains CapFloat64", ir.CapFloat64, ir.CapFloat64, true},
		{"CapFloat64 does not contain CapShaderInt64", ir.CapFloat64, ir.CapShaderInt64, false},
		{"CapAll contains CapFloat64", ir.CapAll, ir.CapFloat64, true},
		{"CapAll contains CapShaderInt64", ir.CapAll, ir.CapShaderInt64, true},
		{"CapAll contains CapShaderFloat16", ir.CapAll, ir.CapShaderFloat16, true},
		{"combined caps", ir.CapFloat64 | ir.CapShaderInt64, ir.CapFloat64, true},
		{"combined caps missing", ir.CapFloat64 | ir.CapShaderInt64, ir.CapShaderFloat16, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.Contains(tt.flag)
			if got != tt.want {
				t.Errorf("Capabilities(%d).Contains(%d) = %v, want %v", tt.c, tt.flag, got, tt.want)
			}
		})
	}
}

func TestCapabilities_AtomicI64(t *testing.T) {
	// atomic<i64> should be rejected without CapShaderInt64 since the
	// inner scalar type i64 requires the capability.
	src := `var<workgroup> counter: atomic<i64>;
	@compute @workgroup_size(1)
	fn main() {
		atomicStore(&counter, 0);
	}`

	_, err := compileWithCaps(t, src, 0)
	if err == nil {
		t.Fatal("expected error for atomic<i64> without SHADER_INT64 cap")
	}
	if !strings.Contains(err.Error(), "SHADER_INT64") {
		t.Errorf("error = %q, want containing SHADER_INT64", err.Error())
	}

	// With the capability, it should work.
	_, err = compileWithCaps(t, src, ir.CapShaderInt64)
	if err != nil {
		t.Fatalf("atomic<i64> with SHADER_INT64 cap should work: %v", err)
	}
}
