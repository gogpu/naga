package lower

import (
	"fmt"
	"slices"
	"testing"

	"github.com/gogpu/naga/ir"
)

// geWant describes one expected global expression: its kind, the f32 value of a
// literal, and the type handle and components of a Compose/ZeroValue.
type geWant struct {
	kind  string // "Lit", "Compose", "ZeroValue"
	val   float32
	ty    ir.TypeHandle
	comps []ir.ExpressionHandle
}

func (g geWant) String() string {
	switch g.kind {
	case "Lit":
		return fmt.Sprintf("L%g", g.val)
	case "Compose":
		return fmt.Sprintf("C%d%v", g.ty, g.comps)
	case "ZeroValue":
		return fmt.Sprintf("Z%d", g.ty)
	}
	return g.kind
}

func lit(v float32) geWant { return geWant{kind: "Lit", val: v} }

func compose(ty ir.TypeHandle, comps ...ir.ExpressionHandle) geWant {
	return geWant{kind: "Compose", ty: ty, comps: comps}
}

func zeroValue(ty ir.TypeHandle) geWant { return geWant{kind: "ZeroValue", ty: ty} }

// geTree renders the expression tree rooted at h independent of arena order,
// e.g. "C0(C1(L1,L2),C1(L3,L4))".
func geTree(ges []geWant, h ir.ExpressionHandle) string {
	if int(h) >= len(ges) {
		return "?"
	}
	g := ges[h]
	if g.kind != "Compose" {
		return g.String()
	}
	s := fmt.Sprintf("C%d(", g.ty)
	for i, c := range g.comps {
		if i > 0 {
			s += ","
		}
		s += geTree(ges, c)
	}
	return s + ")"
}

// assertSameGEMultiset checks that two arenas differ only in order: the
// initializer (last expression) must be the same tree of values and types.
func assertSameGEMultiset(t *testing.T, got, want []geWant) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("global expression count = %d, want %d", len(got), len(want))
		return
	}
	last := ir.ExpressionHandle(len(want) - 1)
	if g, w := geTree(got, last), geTree(want, last); g != w {
		t.Errorf("initializer tree differs, not just order:\n got  %s\n want %s", g, w)
	}
}

func describeGE(e ir.ExpressionKind) geWant {
	switch k := e.(type) {
	case ir.Literal:
		if f, ok := k.Value.(ir.LiteralF32); ok {
			return lit(float32(f))
		}
		return geWant{kind: fmt.Sprintf("Lit(%T)", k.Value)}
	case ir.ExprCompose:
		return compose(k.Type, k.Components...)
	case ir.ExprZeroValue:
		return zeroValue(k.Type)
	default:
		return geWant{kind: fmt.Sprintf("%T", e)}
	}
}

// Module-scope matrix initializers must produce exactly Rust naga's IR: every
// type an initializer references survives compaction, types are registered in
// Rust's order, and the global expression arena has the same shape and order.
//
// Every expectation below is a Rust naga IR dump (front/wgsl lowering, which
// compacts with KeepUnused::Yes) of the same source, taken 2026-10-15 at
// gfx-rs/wgpu 18da915b.
//
// Before the fix the vec2<f32> column type was compacted away: a `const` was
// left with dangling column handles, and a `var<private>` (whose initializer
// was built after compaction) silently pointed its columns at the matrix type.
//
// Order of a matrix built from scalars follows construction.rs: concrete
// arguments first, then the conversions of abstract ones; a fully abstract
// partial matrix is concretized column by column.
func TestGlobalExpressionTypes_MatrixInitializersMatchRust(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	mat2 := ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32}
	vec2 := ir.VectorType{Size: ir.Vec2, Scalar: f32}
	vec4 := ir.VectorType{Size: ir.Vec4, Scalar: f32}
	withColumns := []ir.TypeInner{mat2, vec2, vec4}
	zeroTypes := []ir.TypeInner{mat2, vec4}

	// X is copied into a function-scope var so the body is not const-evaluated:
	// with `X[0]` on a const, Rust folds the access in the function arena
	// (registering extra types), which is outside what this test pins.
	const body = `

@fragment
fn fs() -> @location(0) vec4<f32> {
    var m = X;
    return vec4<f32>(m[0], m[1]);
}
`
	byColumn := []geWant{lit(1), lit(2), compose(1, 0, 1), lit(3), lit(4), compose(1, 3, 4), compose(0, 2, 5)}
	scalarsFirst := []geWant{lit(1), lit(2), lit(3), lit(4), compose(1, 0, 1), compose(1, 2, 3), compose(0, 4, 5)}

	tests := []struct {
		name      string
		decl      string
		wantTypes []ir.TypeInner
		wantGE    []geWant
		// knownGEOrderDiff, when set, tolerates a different global expression
		// order (types are always checked). The test fails once the order
		// matches, so a fixed divergence cannot linger here.
		knownGEOrderDiff string
	}{
		// Column arguments.
		{"const column args", `const X: mat2x2<f32> = mat2x2<f32>(vec2<f32>(1.0, 2.0), vec2<f32>(3.0, 4.0));`, withColumns, byColumn, ""},
		{"var column args", `var<private> X: mat2x2<f32> = mat2x2<f32>(vec2<f32>(1.0, 2.0), vec2<f32>(3.0, 4.0));`, withColumns, byColumn, ""},

		// Scalar arguments, concrete matrix (explicit type or a concrete argument).
		{"const explicit, abstract floats", `const X: mat2x2<f32> = mat2x2<f32>(1.0, 2.0, 3.0, 4.0);`, withColumns, scalarsFirst, ""},
		{"var explicit, abstract floats", `var<private> X: mat2x2<f32> = mat2x2<f32>(1.0, 2.0, 3.0, 4.0);`, withColumns, scalarsFirst, ""},
		{"const explicit, abstract ints", `const X: mat2x2<f32> = mat2x2<f32>(1, 2, 3, 4);`, withColumns, scalarsFirst, ""},
		{"var explicit, abstract ints", `var<private> X: mat2x2<f32> = mat2x2<f32>(1, 2, 3, 4);`, withColumns, scalarsFirst, ""},
		{"const partial, concrete args", `const X: mat2x2<f32> = mat2x2(1.0f, 2.0f, 3.0f, 4.0f);`, withColumns, scalarsFirst, ""},
		{"var partial, concrete args", `var<private> X: mat2x2<f32> = mat2x2(1.0f, 2.0f, 3.0f, 4.0f);`, withColumns, scalarsFirst, ""},
		{"var partial, first arg concrete", `var<private> X: mat2x2<f32> = mat2x2(1.0f, 2, 3, 4);`, withColumns, scalarsFirst, ""},
		{
			// The concrete 4.0f keeps its slot; the abstract 1, 2, 3 are
			// converted afterwards.
			"var partial, last arg concrete", `var<private> X: mat2x2<f32> = mat2x2(1, 2, 3, 4.0f);`, withColumns,
			[]geWant{lit(4), lit(1), lit(2), lit(3), compose(1, 1, 2), compose(1, 3, 0), compose(0, 4, 5)}, "",
		},

		// Scalar arguments, fully abstract partial matrix: column by column.
		{
			"const partial, abstract args", `const X: mat2x2<f32> = mat2x2(1, 2, 3, 4);`, withColumns, byColumn,
			// The const path (lowerCompositeConstant → evalConstantArgsAsGlobalExprs)
			// emits the four scalars first. Arena order only — same values and
			// types. Tracked in TECH-GE-002 (unify const and var initializer lowering).
			"const path emits scalars first",
		},
		{"var partial, abstract args", `var<private> X: mat2x2<f32> = mat2x2(1, 2, 3, 4);`, withColumns, byColumn, ""},
		{
			"var partial, negated abstract arg", `var<private> X: mat2x2<f32> = mat2x2(-1, 2, 3, 4);`, withColumns,
			[]geWant{lit(-1), lit(2), compose(1, 0, 1), lit(3), lit(4), compose(1, 3, 4), compose(0, 2, 5)}, "",
		},
		{"var partial, abstract const arg", "const A = 1;\nvar<private> X: mat2x2<f32> = mat2x2(A, 2, 3, 4);", withColumns, byColumn, ""},

		// Zero-value constructors with an explicit type stay ZeroValue (no column type).
		{"const zero-arg", `const X: mat2x2<f32> = mat2x2<f32>();`, zeroTypes, []geWant{zeroValue(0)}, ""},
		{"var zero-arg", `var<private> X: mat2x2<f32> = mat2x2<f32>();`, zeroTypes, []geWant{zeroValue(0)}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := helperCompileWGSL(t, tt.decl+body)

			gotTypes := make([]ir.TypeInner, len(m.Types))
			for i := range m.Types {
				gotTypes[i] = m.Types[i].Inner
			}
			if !slices.Equal(gotTypes, tt.wantTypes) {
				t.Errorf("types:\n got  %+v\n want %+v", gotTypes, tt.wantTypes)
			}

			gotGE := make([]geWant, len(m.GlobalExpressions))
			for i, e := range m.GlobalExpressions {
				gotGE[i] = describeGE(e.Kind)
			}
			matches := slices.EqualFunc(gotGE, tt.wantGE, func(a, b geWant) bool {
				return a.kind == b.kind && a.val == b.val && a.ty == b.ty && slices.Equal(a.comps, b.comps)
			})
			switch {
			case !matches && tt.knownGEOrderDiff == "":
				t.Errorf("global_expressions:\n got  %v\n want %v", gotGE, tt.wantGE)
			case !matches:
				t.Logf("known global expression order diff (%s):\n got  %v\n want %v", tt.knownGEOrderDiff, gotGE, tt.wantGE)
				assertSameGEMultiset(t, gotGE, tt.wantGE)
			case tt.knownGEOrderDiff != "":
				t.Errorf("global expressions now match Rust — remove knownGEOrderDiff %q", tt.knownGEOrderDiff)
			}

			// The initializer is the last global expression in every case.
			last := ir.ExpressionHandle(len(tt.wantGE) - 1)
			var init *ir.ExpressionHandle
			switch {
			case len(m.GlobalVariables) == 1:
				init = m.GlobalVariables[0].InitExpr
			case len(m.Constants) == 1:
				h := m.Constants[0].Init
				init = &h
			}
			if init == nil || *init != last {
				t.Errorf("initializer handle = %v, want %d", init, last)
			}
		})
	}
}
