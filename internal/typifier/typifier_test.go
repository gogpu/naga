package typifier

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// resolve types fn's expressions up to h and returns the type of h: the
// typifier's answer for one expression, as ir.ResolveExpressionType gave it.
func resolve(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) (ir.TypeResolution, error) {
	var t Typifier
	if err := t.Grow(m, fn, h+1); err != nil {
		return ir.TypeResolution{}, err
	}
	return t.Get(h), nil
}

var (
	tF32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	tI32  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	vec4f = ir.VectorType{Size: ir.Vec4, Scalar: tF32}
)

// TestGrowMemoizes: Grow types only what it has not, and a later Grow
// extends the table over expressions appended since, as a pass that appends
// expressions grows it.
func TestGrowMemoizes(t *testing.T) {
	m := &ir.Module{Types: []ir.Type{{Inner: tF32}}}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
		{Kind: ir.ExprSplat{Size: ir.Vec4, Value: 0}},
	}}
	var ty Typifier
	if err := ty.Grow(m, fn, 1); err != nil || ty.Len() != 1 {
		t.Fatalf("Grow(1): len %d, err %v", ty.Len(), err)
	}
	if err := ty.Grow(m, fn, 2); err != nil || ty.Len() != 2 {
		t.Fatalf("Grow(2): len %d, err %v", ty.Len(), err)
	}
	if got := ty.Inner(m, 1); got != vec4f {
		t.Errorf("splat: %#v, want vec4<f32>", got)
	}
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.ExprSwizzle{Size: ir.Vec2, Vector: 1}})
	if err := ty.Grow(m, fn, 3); err != nil {
		t.Fatal(err)
	}
	if got := ty.Inner(m, 2); got != (ir.VectorType{Size: ir.Vec2, Scalar: tF32}) {
		t.Errorf("appended swizzle: %#v, want vec2<f32>", got)
	}
	if err := ty.Grow(m, fn, 2); err != nil || ty.Len() != 3 {
		t.Errorf("Grow below Len: len %d, err %v; want no change", ty.Len(), err)
	}
	ty.Reset()
	if ty.Len() != 0 {
		t.Errorf("Reset: len %d", ty.Len())
	}
}

// TestGrowForwardReferences: the DXIL passes' IR refers forward (an inlined
// call result loads a later return slot, an alias stands for a later phi, a
// phi merges a later value); Grow types such an expression after what it
// reads, and a phi as its first typed incoming value.
func TestGrowForwardReferences(t *testing.T) {
	m := &ir.Module{Types: []ir.Type{{Inner: tF32}, {Inner: tI32}}}
	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{{Name: "slot", Type: 1}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLoad{Pointer: 3}},                                   // 0: loads the later slot
			{Kind: ssa.Alias{Source: 4}},                                      // 1: stands for the later phi
			{Kind: ir.Literal{Value: ir.LiteralF32(0)}},                       // 2
			{Kind: ir.ExprLocalVariable{Variable: 0}},                         // 3: the slot
			{Kind: ssa.Phi{Incoming: []ssa.Incoming{{Value: 5}, {Value: 2}}}}, // 4: loop phi
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 4, Right: 2}},        // 5: next iteration
		},
	}
	ty, err := Function(m, fn)
	if err != nil {
		t.Fatal(err)
	}
	want := []ir.TypeInner{tI32, tF32, tF32, ir.PointerType{Base: 1, Space: ir.SpaceFunction}, tF32, tF32}
	for h, w := range want {
		if got := ty.Inner(m, ir.ExpressionHandle(h)); got != w {
			t.Errorf("[%d] %T: %#v, want %#v", h, fn.Expressions[h].Kind, got, w)
		}
	}
}

// TestGrowError: an expression that cannot be typed fails Grow, which then
// keeps what the typifier had; so does a cycle with no typed way in.
func TestGrowError(t *testing.T) {
	m := &ir.Module{Types: []ir.Type{{Inner: tF32}}}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
		{Kind: ir.ExprSwizzle{Size: ir.Vec2, Vector: 0}}, // a swizzle of a scalar
	}}
	var ty Typifier
	if err := ty.Grow(m, fn, 1); err != nil {
		t.Fatal(err)
	}
	err := ty.Grow(m, fn, 2)
	if err == nil || !strings.Contains(err.Error(), "expression [1]") || !strings.Contains(err.Error(), "swizzle base must be vector") {
		t.Errorf("err = %v, want expression [1]'s swizzle error", err)
	}
	if ty.Len() != 1 {
		t.Errorf("after the error len %d, want 1", ty.Len())
	}
	var te *Error
	if !errors.As(err, &te) || te.Expression != 1 {
		t.Errorf("errors.As(*Error) = %+v, want the error of expression 1", te)
	}

	cycle := &ir.Function{Expressions: []ir.Expression{
		{Kind: ssa.Alias{Source: 1}},
		{Kind: ssa.Alias{Source: 0}},
	}}
	if _, err := Function(m, cycle); err == nil || !strings.Contains(err.Error(), "cannot be typed before it") {
		t.Errorf("alias cycle: err = %v, want the forward-reference error", err)
	}
	if err := ty.Grow(m, fn, 3); err == nil {
		t.Error("Grow beyond the arena: no error")
	}
}

// TestInvalidate: an expression a pass rewrote in place is typed again from
// what it is now (Rust's Typifier::invalidate).
func TestInvalidate(t *testing.T) {
	m := &ir.Module{}
	fn := &ir.Function{Expressions: []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralAbstractInt(1)}}}}
	ty, err := Function(m, fn)
	if err != nil {
		t.Fatal(err)
	}
	if got := ty.Inner(m, 0); got != (ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}) {
		t.Fatalf("abstract literal: %#v", got)
	}
	fn.Expressions[0].Kind = ir.Literal{Value: ir.LiteralI32(1)}
	if err := ty.Invalidate(m, fn, 0); err != nil {
		t.Fatal(err)
	}
	if got := ty.Inner(m, 0); got != tI32 {
		t.Errorf("after Invalidate: %#v, want i32", got)
	}
	if err := ty.Invalidate(m, fn, 1); err == nil {
		t.Error("Invalidate of an untyped expression: no error")
	}
}

// TestWorkGroupUniformLoadResult: the result carries the type the load
// gives (Rust), with no need of its statement.
func TestWorkGroupUniformLoadResult(t *testing.T) {
	m := &ir.Module{Types: []ir.Type{{Inner: tF32}}}
	fn := &ir.Function{Expressions: []ir.Expression{{Kind: ir.ExprWorkGroupUniformLoadResult{Ty: 0}}}}
	res, err := resolve(m, fn, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Handle == nil || *res.Handle != 0 {
		t.Errorf("got %#v, want handle 0", res)
	}
}

// TestModfFrexpResultTypes: modf and frexp are typed as the predeclared
// result struct of their argument's type, found by name; without it, as the
// argument.
func TestModfFrexpResultTypes(t *testing.T) {
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	m := &ir.Module{Types: []ir.Type{
		{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f16}},       // 0
		{Name: "__modf_result_vec3_f16", Inner: ir.StructType{}}, // 1
		{Name: "__frexp_result_f32", Inner: ir.StructType{}},     // 2
	}}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.ExprZeroValue{Type: 0}},
		{Kind: ir.ExprMath{Fun: ir.MathModf, Arg: 0}},
		{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
		{Kind: ir.ExprMath{Fun: ir.MathFrexp, Arg: 2}},
		{Kind: ir.ExprMath{Fun: ir.MathFrexp, Arg: 0}},
	}}
	ty, err := Function(m, fn)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		h    ir.ExpressionHandle
		want ir.TypeHandle
	}{{1, 1}, {3, 2}} {
		if res := ty.Get(c.h); res.Handle == nil || *res.Handle != c.want {
			t.Errorf("[%d]: %#v, want handle %d", c.h, res, c.want)
		}
	}
	if res := ty.Get(4); res.Handle == nil || *res.Handle != 0 {
		t.Errorf("frexp with no __frexp_result_vec3_f16: %#v, want the argument's type", res)
	}
}

// TestForModule types every function and entry point, and names the one
// that fails.
func TestForModule(t *testing.T) {
	ok := ir.Function{Name: "f", Expressions: []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralF32(0)}}}}
	m := &ir.Module{
		Functions:   []ir.Function{ok},
		EntryPoints: []ir.EntryPoint{{Name: "main", Function: ok}},
	}
	mt, err := ForModule(m)
	if err != nil {
		t.Fatal(err)
	}
	if mt.Function(&m.Functions[0]) == nil || mt.Function(&m.EntryPoints[0].Function) == nil {
		t.Error("a function of the module has no typifier")
	}
	if mt.Function(&ok) != nil {
		t.Error("a function outside the module has a typifier")
	}
	var none *Module
	if none.Function(&ok) != nil {
		t.Error("nil Module returned a typifier")
	}
	m.EntryPoints[0].Function.Expressions = append(m.EntryPoints[0].Function.Expressions,
		ir.Expression{Kind: ir.ExprFunctionArgument{Index: 3}})
	if _, err := ForModule(m); err == nil || !strings.Contains(err.Error(), `entry point "main"`) {
		t.Errorf("err = %v, want the entry point named", err)
	}
}
