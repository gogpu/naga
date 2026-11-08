package lower

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// swizzleChainSource reads a vector through n multi-component swizzles.
func swizzleChainSource(n int) string {
	return "@compute @workgroup_size(1) fn main() {\n var x: i32 = 0;\n var v = vec4<i32>(x);\n x = v" +
		strings.Repeat(".wzyx", n) + ".x;\n}\n"
}

// TestChainsLowerInLinearWork: the base of a multi-component swizzle
// was lowered twice, once by lowerMemberForRef and again by the lowerMember it
// fell back to, so a chain of n swizzles did 2^n work. 32 of them (about 200
// bytes of WGSL) ended the process with Go's fatal "out of memory". Each base
// is lowered once now, so doubling the chain about doubles the allocations.
// The other chains whose base the lowerer reaches through a reference path
// are checked the same way.
func TestChainsLowerInLinearWork(t *testing.T) {
	r := strings.Repeat
	fn := func(body string) string {
		return "@compute @workgroup_size(1) fn main() {\n var x: i32 = 0;\n var f: f32 = 1.0;\n" +
			" var v = vec4<f32>(f);\n var m = mat4x4<f32>(v, v, v, v);\n var a: array<array<array<f32, 2>, 2>, 2>;\n" +
			body + "\n}\n"
	}
	chains := []struct {
		name string
		src  func(n int) string
	}{
		{"swizzle", swizzleChainSource},
		{"swizzle then component", func(n int) string { return fn(" f = v" + r(".wzyx", n) + ".x;") }},
		{"nested modf", func(n int) string { return fn(" f = " + r("modf(", n) + "f" + r(").fract", n) + ";") }},
		{"nested frexp", func(n int) string { return fn(" f = " + r("frexp(", n) + "f" + r(").fract", n) + ";") }},
		{"dereference of address-of", func(n int) string { return fn(" f = " + r("(*&", n) + "f" + r(")", n) + ";") }},
		{"swizzle of a matrix column", func(n int) string { return fn(" v = m[0]" + r(".wzyx", n) + ";") }},
		{"local abstract consts that double", func(n int) string {
			var b strings.Builder
			b.WriteString(" const c0 = 1;")
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&b, " const c%d = c%d + c%d;", i, i-1, i-1)
			}
			fmt.Fprintf(&b, " x = i32(c%d);", n)
			return fn(b.String())
		}},
		{"swizzle through a pointer", func(n int) string {
			return fn(" let p = &v;\n f = (*p)" + r(".wzyx", n) + ".x;")
		}},
	}
	for _, c := range chains {
		t.Run(c.name, func(t *testing.T) {
			allocs := func(n int) float64 {
				src := c.src(n)
				return testing.AllocsPerRun(3, func() {
					if _, err := compileWGSL(t, src); err != nil {
						t.Fatalf("length %d: %v", n, err)
					}
				})
			}
			short, long := allocs(8), allocs(16)
			if long > 3*short {
				t.Fatalf("length 16 allocates %.0f times, 8 allocates %.0f: work grows faster than the chain", long, short)
			}
		})
	}
}

// TestSwizzleChainShape: a swizzle of a swizzle loads the variable once and
// swizzles the swizzled value, with no duplicate of the inner chain.
func TestSwizzleChainShape(t *testing.T) {
	m, err := compileWGSL(t, swizzleChainSource(3))
	if err != nil {
		t.Fatal(err)
	}
	ep := m.EntryPoints[0].Function
	var loads, swizzles int
	for _, e := range ep.Expressions {
		switch e.Kind.(type) {
		case ir.ExprLoad:
			loads++
		case ir.ExprSwizzle:
			swizzles++
		}
	}
	// x's initializer is a literal; v's is a Compose of a load of x. The
	// chain loads v once and swizzles three times; .x is an AccessIndex.
	if swizzles != 3 {
		t.Errorf("%d Swizzle expressions, want 3", swizzles)
	}
	if loads != 2 {
		t.Errorf("%d Load expressions, want 2 (x for v's initializer, then v)", loads)
	}
}
