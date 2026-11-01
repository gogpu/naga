package codegen

import (
	"regexp"
	"strings"
	"testing"
)

// argAnd matches the test argument, a bitwise and of two loaded values.
var argAnd = regexp.MustCompile(`_e\d+ & _e\d+`)

// A function the MSL writer expands into code that reads one argument several
// times must have that argument computed once, into a temporary, as Rust
// naga's writer bakes it (back/msl/writer.rs, need_bake_expressions:
// FirstLeadingBit; Pack4x*/Unpack4x* below MSL 2.1; ExtractBits' offset;
// InsertBits' offset; the argument of an integer sign). Written inline, the
// argument expression is repeated, and the expansion's operators bind into it:
// unpack4xI8(a & b) below 2.1 became `a & b >> 8`, which is a & (b >> 8)
// (P1-MSL-098).
func TestReusedMathArgumentsAreBaked(t *testing.T) {
	const decls = `
@group(0) @binding(0) var<storage, read_write> buf: array<u32, 8>;
@group(0) @binding(1) var<storage, read_write> ibuf: array<i32, 8>;
`
	cases := []struct {
		name    string
		version Version
		body    string
	}{
		{"unpack4xI8 below 2.1", Version{Major: 1, Minor: 2}, `let v = unpack4xI8(buf[0] & buf[1]); ibuf[0] = v.y;`},
		{"unpack4xU8 below 2.1", Version{Major: 1, Minor: 2}, `let v = unpack4xU8(buf[0] & buf[1]); buf[2] = v.y;`},
		{"pack4xI8 below 2.1", Version{Major: 1, Minor: 2}, `buf[2] = pack4xI8(vec4<i32>(ibuf[0] & ibuf[1]));`},
		{"pack4xU8Clamp below 2.1", Version{Major: 1, Minor: 2}, `buf[2] = pack4xU8Clamp(vec4<u32>(buf[0] & buf[1]));`},
		{"firstLeadingBit", Version2_1, `ibuf[2] = firstLeadingBit(ibuf[0] & ibuf[1]);`},
		{"extractBits offset", Version2_1, `buf[2] = extractBits(buf[3], buf[0] & buf[1], 4u);`},
		{"insertBits offset", Version2_1, `buf[2] = insertBits(buf[3], buf[4], buf[0] & buf[1], 4u);`},
		{"integer sign", Version2_1, `ibuf[2] = sign(ibuf[0] & ibuf[1]);`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.LangVersion = c.version
			out := compileWGSLWithOpts(t, decls+"@compute @workgroup_size(1)\nfn main() {\n"+c.body+"\n}\n", opts)
			body := out[strings.Index(out, "kernel void main"):]
			// The argument is `a & b` of two loads; the expansions' own masks
			// are `& 0xFF` or `& <literal>`.
			if n := len(argAnd.FindAllString(body, -1)); n != 1 {
				t.Errorf("the argument `_eA & _eB` is written %d times, want once (baked):\n%s", n, body)
			}
		})
	}
}
