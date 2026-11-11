package codegen

import (
	"strings"
	"testing"
)

// BUG-MSL-MULTI-RETURN-001: every struct result return in an entry point
// declared `const auto _tmp`, so a fragment with two returns emitted a
// redefinition, invalid MSL.
//
// Reference: back/msl/writer.rs put_return_value names the temporary through
// the namer (`self.namer.call("_tmp")`), so the second return becomes _tmp_1
// (upstream 0892b384, #9684). Expected lines below are Rust naga's output for
// the same source.
func TestMultipleReturnsNameTheTemporary(t *testing.T) {
	src := `struct Outputs {
    @location(0) color : vec4<f32>,
}

@fragment
fn main() -> Outputs {
    return Outputs(vec4<f32>());
    return Outputs(vec4<f32>());
}
`
	code := compileWGSL(t, src)

	for _, want := range []string{
		"const auto _tmp = Outputs {metal::float4 {}};",
		"const auto _tmp_1 = Outputs {metal::float4 {}};",
		"return main_Output { _tmp_1.color };",
	} {
		if !containsLine(code, want) {
			t.Errorf("missing line (Rust naga output):\n  %s\ngot:\n%s", want, linesContaining(code, "_tmp"))
		}
	}

	// The same name twice is the defect under test.
	count := 0
	for _, l := range splitLines(code) {
		if l == "const auto _tmp = Outputs {metal::float4 {}};" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("`const auto _tmp` declared %d times (redefinition):\n%s", count, linesContaining(code, "const auto _tmp"))
	}
}

func splitLines(code string) []string {
	var out []string
	start := 0
	for i := 0; i < len(code); i++ {
		if code[i] == '\n' {
			out = append(out, strings.TrimSpace(code[start:i]))
			start = i + 1
		}
	}
	if start < len(code) {
		out = append(out, strings.TrimSpace(code[start:]))
	}
	return out
}
