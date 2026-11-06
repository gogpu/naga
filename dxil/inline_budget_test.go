package dxil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/wgsl"
)

// fanOutChain returns a compute shader with helpers g0..gn where g(k) calls
// g(k-1) twice: inlining every call gives 2^n copies of g0 in the entry
// point from source of size linear in n.
func fanOutChain(n int) string {
	var b strings.Builder
	b.WriteString("@group(0) @binding(0) var<storage, read_write> out: array<f32>;\n")
	b.WriteString("fn g0(x: f32) -> f32 { return x * 1.5 + 0.25; }\n")
	for k := 1; k <= n; k++ {
		fmt.Fprintf(&b, "fn g%d(x: f32) -> f32 { return g%d(x) + g%d(x + 1.0); }\n", k, k-1, k-1)
	}
	fmt.Fprintf(&b, "@compute @workgroup_size(1) fn main() { out[0] = g%d(out[1]); }\n", n)
	return b.String()
}

// A shader whose entry point full inlining would blow up exponentially is
// refused before the inlining starts. Without the bound Compile took time
// doubling with each level of the chain (16 s at n=16, 80 s at n=18), so at
// n=30 it would not return; the size estimate it is refused by grows
// linearly with the source.
func TestCompileRefusesExponentialInlining(t *testing.T) {
	src := fanOutChain(30)
	mod, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := irutil.InlinedEntryPointSizes(mod, maxInlinedEntryPointSize+1)[0]; got <= maxInlinedEntryPointSize {
		t.Fatalf("inlined size of a 2^30 fan-out = %d, want above %d", got, maxInlinedEntryPointSize)
	}
	_, _, err = Compile(mod, DefaultOptions())
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Compile(fan-out 30, %d bytes) = %v, want an error matching backend.ErrUnsupported", len(src), err)
	}
	if !strings.Contains(err.Error(), `entry point "main"`) {
		t.Errorf("error %q does not name the entry point", err)
	}
}

// A shallow chain stays well inside the bound and compiles to valid DXIL.
func TestCompileInlinesShallowFanOut(t *testing.T) {
	src := fanOutChain(4)
	mod, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := irutil.InlinedEntryPointSizes(mod, maxInlinedEntryPointSize+1)[0]; got > maxInlinedEntryPointSize/100 {
		t.Fatalf("inlined size of a 2^4 fan-out = %d, want a small count", got)
	}
	compileWGSLToDXIL(t, src)
}
