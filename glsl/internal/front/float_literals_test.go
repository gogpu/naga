package front

// GLSL 4.60 §4.1.4 gives a floating literal its width through its suffix:
// "lf" makes it a double, "hf" a half float (the float16 extension). The
// literal used to be refused outright ("unsupported 16- or 64-bit
// floating-point literal", card P1-GLSLIN-018); each width is now gated by the
// same dialect rule its type is, and reaches the IR as its own literal kind.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// literalWidths reports which float literal kinds the module's expressions
// hold, so a test can pin the IR the suffix produced.
func literalWidths(module *ir.Module) (f64, f16 bool) {
	scan := func(exprs []ir.Expression) {
		for _, e := range exprs {
			lit, ok := e.Kind.(ir.Literal)
			if !ok {
				continue
			}
			switch lit.Value.(type) {
			case ir.LiteralF64:
				f64 = true
			case ir.LiteralF16:
				f16 = true
			}
		}
	}
	scan(module.GlobalExpressions)
	for i := range module.Functions {
		scan(module.Functions[i].Expressions)
	}
	for i := range module.EntryPoints {
		scan(module.EntryPoints[i].Function.Expressions)
	}
	return f64, f16
}

const floatLiteralBody = "o = vec4(0.0);"

func TestFloatLiteralDouble(t *testing.T) {
	const src = "#version 450\nlayout(location = 0) out vec4 o;\nvoid main() { double d = 1.0lf; o = vec4(float(d)) + vec4(0.0); }\n"
	module, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("a 450 double literal did not parse: %v", err)
	}
	if f64, _ := literalWidths(module); !f64 {
		t.Fatal("the double literal did not reach the IR as ir.LiteralF64")
	}
	if _, verr := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); verr != nil {
		t.Fatalf("the module does not validate: %v", verr)
	}
}

func TestFloatLiteralHalf(t *testing.T) {
	const src = "#version 450\nlayout(location = 0) out vec4 o;\nvoid main() { float16_t h = 1.0hf; o = vec4(float(h)) + vec4(0.0); }\n"
	module, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("a 450 half literal did not parse: %v", err)
	}
	if _, f16 := literalWidths(module); !f16 {
		t.Fatal("the half literal did not reach the IR as ir.LiteralF16")
	}
}

func TestFloatLiteralDoubleRefusedInGL330(t *testing.T) {
	// glslang accepts the literal there and rejects the double type it needs;
	// the frontend refuses at the literal with the same double rule.
	const src = "#version 330 core\nout vec4 o;\nvoid main() { double d = 1.0lf; o = vec4(0.0); }\n"
	_, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("a 330 double literal parsed")
	}
	if !strings.Contains(err.Error(), "double") {
		t.Fatalf("error %q does not name the double rule", err.Error())
	}
}

func TestFloatLiteralHalfRefusedInGL330WithoutExtension(t *testing.T) {
	const src = "#version 330 core\nout vec4 o;\nvoid main() { float h = 1.0hf; o = vec4(h); }\n"
	_, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("a 330 half literal parsed without the float16 extension")
	}
	if !strings.Contains(err.Error(), "float16") {
		t.Fatalf("error %q does not name the float16 rule", err.Error())
	}
}

func TestFloatLiteralHalfAcceptedInGL330WithExtension(t *testing.T) {
	const src = "#version 330 core\n" +
		"#extension GL_EXT_shader_explicit_arithmetic_types_float16 : require\n" +
		"out vec4 o;\nvoid main() { float16_t h = 1.0hf; o = vec4(float(h)); }\n"
	module, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("a 330 half literal with the extension did not parse: %v", err)
	}
	if _, f16 := literalWidths(module); !f16 {
		t.Fatal("the half literal did not reach the IR as ir.LiteralF16")
	}
}

func TestFloatLiteralHalfRefusedInES300(t *testing.T) {
	const src = "#version 300 es\nprecision mediump float;\nout vec4 o;\nvoid main() { float h = 1.0hf; o = vec4(h); }\n"
	_, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("an ES 3.00 half literal parsed")
	}
	if !strings.Contains(err.Error(), "float16") {
		t.Fatalf("error %q does not name the float16 rule", err.Error())
	}
}

func TestFloatLiteralBareSuffixRefused(t *testing.T) {
	for _, src := range []string{
		"#version 450\nlayout(location = 0) out vec4 o;\nvoid main() { float f = 1.0l; o = vec4(f); }\n",
		"#version 450\nlayout(location = 0) out vec4 o;\nvoid main() { float f = 1.0h; o = vec4(f); }\n",
	} {
		_, _, err := Parse(src, Options{Stage: ir.StageFragment})
		if err == nil {
			t.Fatalf("a bare suffix literal parsed: %q", src)
		}
		if !strings.Contains(err.Error(), "must be followed by 'f'") {
			t.Fatalf("error %q does not name the suffix rule", err.Error())
		}
	}
}

var _ = floatLiteralBody
