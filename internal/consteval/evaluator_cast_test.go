package consteval

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// CastArray is Rust naga's cast_array: an array constructor is converted
// element by element into a new array of the converted element type.
func TestCastArray(t *testing.T) {
	e, a, exprs := newEval(BehaviorRuntime)
	aint := a.Type(ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8})
	size := uint32(2)
	arr := a.Array(aint, ir.ArraySize{Constant: &size})
	one, two := lit(t, e, ir.LiteralAbstractInt(1)), lit(t, e, ir.LiteralAbstractInt(2))
	h := mustEval(t, e, ir.ExprCompose{Type: arr, Components: []ir.ExpressionHandle{one, two}})

	r, err := e.CastArray(h, tF32)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := (*exprs)[r].Kind.(ir.ExprCompose)
	if !ok {
		t.Fatalf("result %T, want Compose", (*exprs)[r].Kind)
	}
	at, ok := e.Module.Types[c.Type].Inner.(ir.ArrayType)
	if !ok || !reflect.DeepEqual(e.Module.Types[at.Base].Inner, tF32) || *at.Size.Constant != 2 {
		t.Errorf("result type %#v, want array<f32, 2>", e.Module.Types[c.Type].Inner)
	}
	for i, comp := range c.Components {
		if got := (*exprs)[comp].Kind; !reflect.DeepEqual(got, ir.Literal{Value: ir.LiteralF32(float32(i + 1))}) {
			t.Errorf("element %d = %#v", i, got)
		}
	}
	if !e.IsConst(r) {
		t.Error("cast result is not const")
	}

	// Not an array: an ordinary conversion.
	s, err := e.CastArray(one, tF32)
	if err != nil || !reflect.DeepEqual((*exprs)[s].Kind, ir.Literal{Value: ir.LiteralF32(1)}) {
		t.Errorf("CastArray(1, f32) = %#v err=%v", (*exprs)[s].Kind, err)
	}

	// A conversion the value does not survive is an error (Rust text).
	big := lit(t, e, ir.LiteralAbstractInt(1<<40))
	if _, err := e.CastArray(big, tI32); !errors.Is(err, ErrNotRepresentable) {
		t.Errorf("CastArray(2^40, i32): %v", err)
	}

	// A run-time value cannot be cast here (Rust SubexpressionsAreNotConstant).
	*exprs = append(*exprs, ir.Expression{Kind: ir.ExprFunctionArgument{Index: 0}})
	if _, err := e.CastArray(ir.ExpressionHandle(len(*exprs)-1), tF32); !errors.Is(err, ErrNotConst) {
		t.Errorf("CastArray(arg): %v", err)
	}
}

func TestCastArrayZeroValue(t *testing.T) {
	e, a, exprs := newEval(BehaviorRuntime)
	afloat := a.Type(ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8})
	size := uint32(3)
	arr := a.Array(afloat, ir.ArraySize{Constant: &size})
	z := mustEval(t, e, ir.ExprZeroValue{Type: arr})
	r, err := e.CastArray(z, tF32)
	if err != nil {
		t.Fatal(err)
	}
	zv, ok := (*exprs)[r].Kind.(ir.ExprZeroValue)
	if !ok {
		t.Fatalf("result %T, want ZeroValue", (*exprs)[r].Kind)
	}
	at := e.Module.Types[zv.Type].Inner.(ir.ArrayType)
	if !reflect.DeepEqual(e.Module.Types[at.Base].Inner, tF32) {
		t.Errorf("zero array element type %#v, want f32", e.Module.Types[at.Base].Inner)
	}
}

// Casting keeps the shape of the value, as Rust naga's cast does: a Splat
// stays a Splat of the converted scalar, a Compose is converted component by
// component (nested constructors stay nested), and a ZeroValue is first
// expanded into constructors of one shared zero literal (eval_zero_value).
func TestCastKeepsShape(t *testing.T) {
	e, a, exprs := newEval(BehaviorRuntime)
	w := uint8(4)
	asF32 := func(h ir.ExpressionHandle) ir.ExpressionHandle {
		return mustEval(t, e, ir.ExprAs{Expr: h, Kind: ir.ScalarFloat, Convert: &w})
	}

	zero := lit(t, e, ir.LiteralAbstractFloat(0))
	sp := mustEval(t, e, ir.ExprSplat{Size: ir.Vec3, Value: zero})
	r := asF32(sp)
	s, ok := (*exprs)[r].Kind.(ir.ExprSplat)
	if !ok || s.Size != ir.Vec3 || !reflect.DeepEqual((*exprs)[s.Value].Kind, ir.Literal{Value: ir.LiteralF32(0)}) {
		t.Errorf("f32(splat(0)) = %#v, want Splat(Vec3, 0f)", (*exprs)[r].Kind)
	}

	vec2a := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}})
	vec3a := a.Type(ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}})
	inner := mustEval(t, e, ir.ExprCompose{Type: vec2a, Components: []ir.ExpressionHandle{
		lit(t, e, ir.LiteralAbstractInt(6)), lit(t, e, ir.LiteralAbstractInt(7))}})
	outer := mustEval(t, e, ir.ExprCompose{Type: vec3a, Components: []ir.ExpressionHandle{inner, lit(t, e, ir.LiteralAbstractInt(8))}})
	r = asF32(outer)
	c := (*exprs)[r].Kind.(ir.ExprCompose)
	if len(c.Components) != 2 {
		t.Fatalf("f32(vec3(vec2(6, 7), 8)) has %d components, want the nested 2", len(c.Components))
	}
	if _, nested := (*exprs)[c.Components[0]].Kind.(ir.ExprCompose); !nested {
		t.Errorf("first component %T, want the converted inner Compose", (*exprs)[c.Components[0]].Kind)
	}
	if got := valueAt(t, e, r); !reflect.DeepEqual(got, f32v(6, 7, 8).Components()) {
		t.Errorf("value %v", got)
	}

	vec2f := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: tF32})
	z := mustEval(t, e, ir.ExprZeroValue{Type: vec2f})
	wi := uint8(4)
	r = mustEval(t, e, ir.ExprAs{Expr: z, Kind: ir.ScalarSint, Convert: &wi})
	// Each component of the expanded zero (one shared literal) is converted
	// separately, as Rust casts component by component.
	if got := valueAt(t, e, r); !reflect.DeepEqual(got, []ir.LiteralValue{ir.LiteralI32(0), ir.LiteralI32(0)}) {
		t.Errorf("i32(vec2<f32>()) = %v", got)
	}
	zh, err := e.zeroValue(vec2f)
	if err != nil {
		t.Fatal(err)
	}
	if zc := (*exprs)[zh].Kind.(ir.ExprCompose); zc.Components[0] != zc.Components[1] {
		t.Errorf("zero vec2 = %#v, want one shared literal (Rust eval_zero_value)", zc)
	}
}
