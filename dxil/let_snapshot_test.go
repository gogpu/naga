package dxil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// straightLineStores runs the instructions of a function without branches
// symbolically and returns the symbol of the value each dx.op.bufferStore
// writes, by its byte offset. A symbol names where a value comes from: a
// constant ("c5"), a buffer load ("load(c12)"), an operation on symbols. A
// local's memory is tracked per constant GEP path, so a load reads the last
// value stored to its place before it.
func straightLineStores(t *testing.T, mod *module.Module, fn *module.Function) map[int64]string {
	t.Helper()
	consts := map[int]int64{}
	for _, c := range mod.Constants {
		if c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			consts[c.ValueID] = c.IntValue
		}
	}
	sym := map[int]string{}
	symOf := func(id int) string {
		if s, ok := sym[id]; ok {
			return s
		}
		if v, ok := consts[id]; ok {
			return fmt.Sprintf("c%d", v)
		}
		return fmt.Sprintf("%%%d", id)
	}
	addr := map[int]string{} // GEP or alloca value → place in a local
	mem := map[string]string{}
	stores := map[int64]string{}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			switch in.Kind {
			case module.InstrBr, module.InstrPhi:
				t.Fatalf("straightLineStores: the function has control flow")
			case module.InstrAlloca:
				addr[in.ValueID] = fmt.Sprintf("a%d", in.ValueID)
			case module.InstrGEP:
				base, ok := addr[in.Operands[2]]
				if !ok {
					continue // not a local
				}
				path := base
				for _, idx := range in.Operands[3:] {
					v, isConst := consts[idx]
					if !isConst {
						t.Fatalf("straightLineStores: GEP with a run-time index")
					}
					path += fmt.Sprintf(".%d", v)
				}
				addr[in.ValueID] = path
			case module.InstrStore:
				if p, ok := addr[in.Operands[0]]; ok {
					mem[p] = symOf(in.Operands[1])
				}
			case module.InstrLoad:
				if p, ok := addr[in.Operands[0]]; ok {
					s, stored := mem[p]
					if !stored {
						s = "undef"
					}
					sym[in.ValueID] = s
				}
			case module.InstrSelect:
				sym[in.ValueID] = fmt.Sprintf("(%s ? %s : %s)", symOf(in.Operands[0]), symOf(in.Operands[1]), symOf(in.Operands[2]))
			case module.InstrCmp:
				sym[in.ValueID] = fmt.Sprintf("(%s cmp %s)", symOf(in.Operands[0]), symOf(in.Operands[1]))
			case module.InstrBinOp:
				sym[in.ValueID] = fmt.Sprintf("(%s op%d %s)", symOf(in.Operands[0]), in.Operands[2], symOf(in.Operands[1]))
			case module.InstrExtractVal:
				sym[in.ValueID] = fmt.Sprintf("%s[%d]", symOf(in.Operands[0]), in.Operands[1])
			case module.InstrCall:
				name := ""
				if in.CalledFunc != nil {
					name = in.CalledFunc.Name
				}
				switch {
				case strings.HasPrefix(name, "dx.op.bufferLoad"):
					sym[in.ValueID] = fmt.Sprintf("load(%s)", symOf(in.Operands[2]))
				case strings.HasPrefix(name, "dx.op.bufferStore"):
					off, ok := consts[in.Operands[2]]
					if !ok {
						t.Fatalf("straightLineStores: bufferStore at a run-time offset")
					}
					stores[off] = symOf(in.Operands[4])
				case in.HasValue:
					sym[in.ValueID] = fmt.Sprintf("%s#%d", name, in.ValueID)
				}
			}
		}
	}
	return stores
}

// A let of a local array, struct or vector is the value of the variable
// where the let is: a later store to the variable doesn't change it. The
// DXIL emitter read an element or member of such a let from the variable's
// alloca where the let was used, so it read the store made after it.
func TestLetSnapshotOfLocal(t *testing.T) {
	const prelude = `
struct Out { v: i32, seed: i32, i: u32 }
@group(0) @binding(0) var<storage, read_write> out: Out;
struct S { a: i32, b: vec2<i32> }
struct N { s: S, k: i32 }
const cs = S(7, vec2<i32>(8, 9));
fn bumpi(p: ptr<function, i32>) -> i32 { *p = *p + 1; return 0; }
fn fs(s: S, k: i32) -> i32 { return s.a + k; }
fn fa(arr: array<i32, 2>, k: i32) -> i32 { return arr[0] + k; }
`
	const seed = "load(c4)[0]"
	tests := []struct {
		name string
		body string
	}{
		{"array", `
    var arr = array<i32, 2>(out.seed, 20);
    let a = arr;
    arr[0] = arr[0] + 1;
    out.v = a[0];`},
		{"struct", `
    var s = S(out.seed, vec2<i32>(1, 2));
    let t = s;
    s.a = s.a + 1;
    out.v = t.a;`},
		{"struct vector member", `
    var s = S(1, vec2<i32>(out.seed, 2));
    let t = s;
    s.b.x = s.b.x + 1;
    out.v = t.b.x;`},
		{"nested struct", `
    var n = N(S(out.seed, vec2<i32>(1, 2)), 3);
    let m = n;
    n.s.a = n.s.a + 1;
    out.v = m.s.a;`},
		{"array of vectors", `
    var arr = array<vec2<i32>, 2>(vec2<i32>(out.seed, 1), vec2<i32>(2, 3));
    let a = arr;
    arr[0].x = arr[0].x + 1;
    out.v = a[0].x;`},
		{"struct without let", `
    var s = S(out.seed, vec2<i32>(1, 2));
    out.v = s.a;`},
		{"struct initialized from a constant", `
    var s = cs;
    out.v = s.a + out.seed;`},
		{"struct assigned whole", `
    var s: S;
    s = S(out.seed, vec2<i32>(1, 2));
    out.v = s.a;`},
		{"array by a run-time index", `
    var arr = array<i32, 2>(out.seed, out.seed);
    let a = arr;
    arr[0] = arr[0] + 1;
    arr[1] = arr[1] + 1;
    out.v = a[out.i];`},
		{"struct argument", `
    var s = S(out.seed, vec2<i32>(1, 2));
    out.v = fs(s, bumpi(&s.a));`},
		{"array argument", `
    var arr = array<i32, 2>(out.seed, 20);
    out.v = fa(arr, bumpi(&arr[0]));`},
		{"vector", `
    var acc = vec2<i32>(out.seed, 20);
    let c = acc;
    acc.x = acc.x + 1;
    out.v = c.x;`},
	}
	// The argument cases add bumpi's 0 to the seed; a run-time index picks
	// one of the two elements, both the seed.
	wants := map[string]func(string) bool{
		"struct argument":           func(s string) bool { return s == "("+seed+" op0 c0)" },
		"array argument":            func(s string) bool { return s == "("+seed+" op0 c0)" },
		"array by a run-time index": func(s string) bool { return strings.HasSuffix(s, " ? "+seed+" : "+seed+")") },
		// cs.a is 7: SROA dropped the constant initializer and read 0.
		"struct initialized from a constant": func(s string) bool { return s == "(c7 op0 "+seed+")" || s == "("+seed+" op0 c7)" },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, fn := emitMainModule(t, prelude+"@compute @workgroup_size(1) fn main() {"+tt.body+"\n}\n")
			ok := func(s string) bool { return s == seed }
			if w, has := wants[tt.name]; has {
				ok = w
			}
			if got := straightLineStores(t, mod, fn)[0]; !ok(got) {
				t.Errorf("out.v is %s, want it computed from the seed %s alone", got, seed)
			}
		})
	}
}
