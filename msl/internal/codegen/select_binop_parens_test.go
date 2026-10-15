package codegen

import (
	"strings"
	"testing"
)

// Issue #95: a scalar select() used as an operand of a binary operator was
// emitted as a bare ternary. C++ gives ?: lower precedence than |, so
// `a | c ? x : y` parsed as `(a | c) ? x : y` and returned a wrong value on Metal.
//
// Rust naga (back/msl/writer.rs) writes both binop operands with
// is_scoped=false (put_binop) and a scalar Select wraps its whole ternary when
// not scoped. Expected lines below are Rust naga's output (naga-cli, reference
// wgpu 18da915b) for the same sources, identical under the Unchecked and
// ReadZeroSkipWrite bounds-check policies.
func TestSelectAsBinaryOperandIsParenthesized(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "issue 95",
			src: `@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@compute @workgroup_size(1)
fn main() {
    let v = out[0];
    out[1] = (v >> 24u) | select(0u, 0xffffff00u, (v & 0x80000000u) != 0u);
}
`,
			want: []string{
				"out[1] = (v >> 24u) | (((v & 2147483648u) != 0u) ? 4294967040u : 0u);",
			},
		},
		{
			name: "operand positions and non-ternary forms",
			src: `@group(0) @binding(0) var<storage, read_write> buf: array<u32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    buf[1] = select(0u, 1u, i > 2u) + i;
    buf[2] = i * select(3u, 4u, i == 0u);
    let vs = vec2<u32>(i) + select(vec2<u32>(1u), vec2<u32>(2u), vec2<bool>(i > 1u, i < 3u));
    buf[3] = vs.x;
    buf[6] = arrayLength(&buf) * 2u;
}
`,
			want: []string{
				// scalar select as left and as right operand
				"buf[1] = ((i > 2u) ? 1u : 0u) + i;",
				"buf[2] = i * ((i == 0u) ? 4u : 3u);",
				// vector select is a metal::select call — no extra parens
				"metal::uint2 vs = metal::uint2(i) + metal::select(metal::uint2(1u), metal::uint2(2u), metal::bool2(i > 1u, i < 3u));",
				// arrayLength expands to `1 + ...` and was already wrapped
				"buf[6] = (1 + (_buffer_sizes.size0 - 0 - 4) / 4) * 2u;",
			},
		},
		{
			// Same rule in a member-access context: the ternary is wrapped when
			// the condition is a scalar bool (even with vector values); a
			// vector-bool condition is a metal::select call and is not.
			name: "select in access context",
			src: `@group(0) @binding(0) var<storage, read_write> buf: array<u32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    buf[0] = select(vec2<u32>(1u), vec2<u32>(2u), vec2<bool>(i > 1u, i < 3u)).y;
    buf[1] = select(vec2<u32>(1u), vec2<u32>(2u), i > 1u).x;
}
`,
			want: []string{
				"buf[0] = metal::select(metal::uint2(1u), metal::uint2(2u), metal::bool2(i > 1u, i < 3u)).y;",
				"buf[1] = ((i > 1u) ? metal::uint2(2u) : metal::uint2(1u)).x;",
			},
		},
	}

	policies := []struct {
		name string
		p    BoundsCheckPolicies
	}{
		{"Unchecked", BoundsCheckPolicies{}},
		{"ReadZeroSkipWrite", DefaultBoundsCheckPolicies()},
	}

	for _, tt := range tests {
		for _, pol := range policies {
			t.Run(tt.name+"/"+pol.name, func(t *testing.T) {
				opts := DefaultOptions()
				opts.BoundsCheckPolicies = pol.p
				code := compileWGSLWithOpts(t, tt.src, opts)
				for _, want := range tt.want {
					if !containsLine(code, want) {
						t.Errorf("missing line (Rust naga output):\n  %s\ngot:\n%s", want, linesContaining(code, strings.SplitN(want, " = ", 2)[0]+" ="))
					}
				}
			})
		}
	}
}

// containsLine reports whether some line of code equals want after trimming
// indentation.
func containsLine(code, want string) bool {
	for _, l := range strings.Split(code, "\n") {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

// linesContaining returns the trimmed lines of code that contain substr.
func linesContaining(code, substr string) string {
	var out []string
	for _, l := range strings.Split(code, "\n") {
		if strings.Contains(l, substr) {
			out = append(out, "  "+strings.TrimSpace(l))
		}
	}
	return strings.Join(out, "\n")
}
