package hlsl_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl"
)

// returnInSwitchInLoop is the helper of the readback case
// flow/return-in-switch-in-loop-rt: a return inside a switch inside a loop.
const returnInSwitchInLoop = `
@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

fn h(a: u32, b: u32) -> u32 {
    var acc = 0u;
    for (var i = 0u; i < 6u; i++) {
        switch (a + i) % 4u {
            case 2u: { if i >= b % 4u { return acc + 1000u; } }
            case 3u: { break; }
            default: { acc = acc + i; }
        }
        acc = acc + 10u;
    }
    return acc;
}

@compute @workgroup_size(1)
fn main() {
    dst[0] = h(dst[1], dst[2]);
}
`

// functionBody returns the text of the HLSL function whose header starts with
// header, up to its closing brace at the start of a line.
func functionBody(t *testing.T, code, header string) string {
	t.Helper()
	start := strings.Index(code, header)
	if start < 0 {
		t.Fatalf("no %q in\n%s", header, code)
	}
	end := strings.Index(code[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("no end of %q in\n%s", header, code)
	}
	return code[start : start+end+3]
}

// TestFXCHelperReturnInSwitchInLoop: FXC (shader models 5.x) inlines a helper
// and turns a return inside a switch inside a loop into the switch's break,
// dropping the returned value and the loop exit (fxc 10.0.26100, cs_5_1:
// `case l(2) … if_nz … mov r1.x, r2.x  break`; wgpu's GPU readback of
// flow/return-in-switch-in-loop-rt read 70 for 1021 on Intel and NVIDIA).
// For FXC such a helper gets one exit: each return stores the value and sets a
// flag, breaks leave the switch and the loop, and the one return is at the
// end — the shape FXC compiles right. Rust naga writes the return in place at
// every shader model (back/hlsl/writer.rs has no such pass), as we still do
// from shader model 6.0, where DXC compiles it.
func TestFXCHelperReturnInSwitchInLoop(t *testing.T) {
	fxc := functionBody(t, compileWGSLToHLSL(t, returnInSwitchInLoop), "uint h(")
	if n := len(regexp.MustCompile(`\breturn\b`).FindAllString(fxc, -1)); n != 1 {
		t.Fatalf("shader model 5.1: %d returns in h, want 1 at its end:\n%s", n, fxc)
	}
	lines := strings.Split(strings.TrimSpace(fxc), "\n")
	if last := strings.TrimSpace(lines[len(lines)-2]); !strings.HasPrefix(last, "return ") {
		t.Fatalf("shader model 5.1: h ends in %q, want its return:\n%s", last, fxc)
	}
	if !strings.Contains(fxc, "+ 1000u") {
		t.Fatalf("shader model 5.1: the returned value is gone:\n%s", fxc)
	}

	dxc := functionBody(t, compileWGSLToHLSLWithOpts(t, returnInSwitchInLoop,
		hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 0}}), "uint h(")
	if !strings.Contains(dxc, "return (_e17 + 1000u);") {
		t.Fatalf("shader model 6.0: want Rust's in-place return:\n%s", dxc)
	}
}

// TestFXCHelperOtherReturnsUnchanged: only the shape FXC miscompiles is
// rewritten. A return inside a loop with no switch, a return inside a switch
// with no loop, and an entry point's return (FXC writes `ret`, which is
// right) stay as Rust naga writes them.
func TestFXCHelperOtherReturnsUnchanged(t *testing.T) {
	for _, tt := range []struct {
		name, src, header, want string
	}{
		{
			name: "return in loop",
			src: `@group(0) @binding(0) var<storage, read_write> dst: array<u32>;
fn h(a: u32) -> u32 {
    for (var i = 0u; i < 6u; i++) { if i == a { return i + 7u; } }
    return 0u;
}
@compute @workgroup_size(1) fn main() { dst[0] = h(dst[1]); }`,
			header: "uint h(",
			want:   "return (_e",
		},
		{
			name: "return in switch",
			src: `@group(0) @binding(0) var<storage, read_write> dst: array<u32>;
fn h(a: u32) -> u32 {
    switch a { case 2u: { return 7u; } default: {} }
    return 0u;
}
@compute @workgroup_size(1) fn main() { dst[0] = h(dst[1]); }`,
			header: "uint h(",
			want:   "return 7u;",
		},
		{
			name: "entry point",
			src: `@group(0) @binding(0) var<storage, read_write> dst: array<u32>;
@compute @workgroup_size(1) fn main() {
    for (var i = 0u; i < 6u; i++) {
        switch i { case 2u: { dst[0] = i; return; } default: {} }
    }
}`,
			header: "void main(",
			want:   "return;",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := functionBody(t, compileWGSLToHLSL(t, tt.src), tt.header)
			if strings.Contains(body, "_returned") {
				t.Fatalf("rewritten:\n%s", body)
			}
			if !strings.Contains(body, tt.want) {
				t.Fatalf("want %q:\n%s", tt.want, body)
			}
		})
	}
}
