package codegen

import (
	"regexp"
	"strings"
	"testing"
)

// argAnd matches the test argument, a bitwise and of two loaded values.
var argAnd = regexp.MustCompile(`_e\d+ & _e\d+`)

// A function the GLSL writer expands into code that reads one argument
// several times must have that argument computed once, into a temporary, as
// Rust naga's GLSL writer bakes it (back/glsl/writer.rs
// update_expressions_to_bake: Pack4x*/Unpack4x*, an integer dot, the
// fallbacks of unpack4x8snorm/unorm and unpack2x16snorm/unorm where the
// version lacks them, countLeadingZeros of a signed integer). Written inline,
// a compound argument is computed once per use.
func TestReusedMathArgumentsAreBaked(t *testing.T) {
	const decls = `
@group(0) @binding(0) var<storage, read_write> buf: array<u32, 8>;
@group(0) @binding(1) var<storage, read_write> ibuf: array<i32, 8>;
@group(0) @binding(2) var<storage, read_write> fbuf: array<f32, 8>;
`
	cases := []struct {
		name    string
		version Version
		body    string
	}{
		{"pack4xI8", Version450, `buf[2] = pack4xI8(vec4<i32>(ibuf[0] & ibuf[1]));`},
		{"unpack4xU8", Version450, `buf[2] = unpack4xU8(buf[0] & buf[1]).y;`},
		{"integer dot", Version450, `ibuf[2] = dot(vec2<i32>(ibuf[0] & ibuf[1]), vec2<i32>(ibuf[3]));`},
		{"countLeadingZeros i32", Version450, `ibuf[2] = countLeadingZeros(ibuf[0] & ibuf[1]);`},
		// GLSL 3.30 has neither storage buffers nor the native unpacks.
		{"unpack4x8snorm without native", Version330, `return vec4(unpack4x8snorm(u.a & u.b).y);`},
		{"unpack2x16unorm without native", Version330, `return vec4(unpack2x16unorm(u.a & u.b).y);`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := decls + "@compute @workgroup_size(1)\nfn main() {\n" + c.body + "\n}\n"
			if c.version == Version330 {
				src = "struct U { a: u32, b: u32 }\n@group(0) @binding(0) var<uniform> u: U;\n" +
					"@fragment\nfn main() -> @location(0) vec4<f32> {\n" + c.body + "\n}\n"
			}
			out, _, err := compileWGSLHelper(src, Options{LangVersion: c.version})
			if err != nil {
				t.Fatal(err)
			}
			body := out[strings.Index(out, "void main()"):]
			if n := len(argAnd.FindAllString(body, -1)); n != 1 {
				t.Errorf("the argument `_eA & _eB` is written %d times, want once (baked):\n%s", n, body)
			}
		})
	}
}
