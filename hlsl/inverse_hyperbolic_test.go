package hlsl_test

import (
	"strings"
	"testing"
)

// HLSL has no asinh, acosh or atanh intrinsic. Rust naga writes their log
// forms (back/hlsl/writer.rs, Function::Asincosh and Function::Atanh) with
// the argument baked into a temporary, since it is written two or three
// times. The corpus has no WGSL shader that calls them (Rust tests them from
// SPIR-V input only), so TestRustReference never covered the WGSL path.
func TestInverseHyperbolicUseLogForms(t *testing.T) {
	src := `
@group(0) @binding(0) var<storage, read_write> b: array<f32, 4>;

@compute @workgroup_size(1)
fn main() {
    let x = b[0];
    b[1] = asinh(x);
    b[2] = acosh(x);
    b[3] = atanh(x);
}
`
	out := compileWGSLToHLSL(t, src)
	for _, name := range []string{"asinh(", "acosh(", "atanh("} {
		if strings.Contains(out, name) {
			t.Errorf("HLSL calls %s, which HLSL does not have:\n%s", name, out)
		}
	}
	for _, want := range []string{
		"log(x + sqrt(x * x + 1.0))",
		"log(x + sqrt(x * x - 1.0))",
		"0.5 * log((1.0 + x) / (1.0 - x))",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HLSL lacks %q:\n%s", want, out)
		}
	}
}
