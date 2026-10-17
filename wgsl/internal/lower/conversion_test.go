package lower

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

var (
	sAInt   = ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}
	sAFloat = ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}
	sBool   = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	sF16    = ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	sF64    = ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	sI64    = ir.ScalarType{Kind: ir.ScalarSint, Width: 8}
)

// The table of Rust naga's Scalar::automatic_conversion_combine
// (front/wgsl/lower/conversion.rs).
func TestAutomaticConversionCombine(t *testing.T) {
	none := ir.ScalarType{}
	tests := []struct{ a, b, want ir.ScalarType }{
		{sAInt, sAInt, sAInt},
		{sAFloat, sAFloat, sAFloat},
		{scalarI32, scalarI32, scalarI32},
		{sBool, sBool, sBool},
		{sAFloat, sAInt, sAFloat},
		{sAInt, sAFloat, sAFloat},
		{sAFloat, scalarF32, scalarF32},
		{sF16, sAFloat, sF16},
		{sAInt, scalarU32, scalarU32},
		{sI64, sAInt, sI64},
		{sAInt, sF64, sF64},
		{sAFloat, scalarI32, none},
		{scalarU32, sAFloat, none},
		{sBool, sAInt, none},
		{scalarF32, sBool, none},
		{scalarI32, scalarU32, none},
		{scalarF32, sF16, none}, // concrete never converts to concrete
		{scalarI32, sI64, none}, // same kind, other width
	}
	for _, tt := range tests {
		got, ok := automaticConversionCombine(tt.a, tt.b)
		if want := tt.want != none; ok != want || (ok && got != tt.want) {
			t.Errorf("combine(%s, %s) = %s, %v; want %s", wgslScalar(tt.a), wgslScalar(tt.b), wgslScalar(got), ok, wgslScalar(tt.want))
		}
	}
	if !automaticallyConvertsTo(sAInt, scalarF32) || automaticallyConvertsTo(scalarF32, sAFloat) || automaticallyConvertsTo(sAFloat, scalarU32) {
		t.Error("automaticallyConvertsTo disagrees with the combine table")
	}
}

func TestWGSLTypeNames(t *testing.T) {
	l := &Lowerer{module: &ir.Module{}}
	tests := []struct {
		inner ir.TypeInner
		want  string
	}{
		{sAInt, "{AbstractInt}"},
		{sAFloat, "{AbstractFloat}"},
		{sI64, "i64"},
		{sF16, "f16"},
		{ir.VectorType{Size: ir.Vec3, Scalar: sAFloat}, "vec3<{AbstractFloat}>"},
		{ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec4, Scalar: scalarF32}, "mat2x4<f32>"},
	}
	for _, tt := range tests {
		if got := l.wgslType(tt.inner); got != tt.want {
			t.Errorf("wgslType(%#v) = %q, want %q", tt.inner, got, tt.want)
		}
	}
}

// x++ / x-- (Rust StatementKind::Increment/Decrement): the typed 1 is
// appended before the load (Rust v30: Literal(0), LocalVariable, Literal(1),
// Load, Binary), unlike compound assignment, which loads first; only i32/u32
// references are allowed.
func TestIncrementDecrement(t *testing.T) {
	m, err := compileWGSL(t, "@compute @workgroup_size(1) fn main() {\n    var idx = 1;\n    idx--;\n}\n")
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	exprs := m.EntryPoints[0].Function.Expressions
	want := []string{"Literal", "ExprLocalVariable", "Literal", "ExprLoad", "ExprBinary"}
	if len(exprs) != len(want) {
		t.Fatalf("%d expressions, want %d (Rust)", len(exprs), len(want))
	}
	for i, e := range exprs {
		if got := reflect.TypeOf(e.Kind).Name(); got != want[i] {
			t.Errorf("expr[%d] = %s, want %s", i, got, want[i])
		}
	}
	if lit, ok := exprs[2].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralI32(1) {
		t.Errorf("expr[2] = %#v, want Literal(I32(1))", exprs[2].Kind)
	}
	_, err = compileWGSL(t, "@compute @workgroup_size(1) fn main() {\n    var f = 1.0;\n    f++;\n}\n")
	const msg = "increment/decrement operation requires reference type to be one of i32 or u32"
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Errorf("f32 increment: error = %v, want %q", err, msg)
	}
}

// workgroupUniformLoad of an atomic<T> yields a T, like Load (Rust
// WorkGroupUniformLoadResult: "loading through a pointer to atomic<T>
// produces a T"); `let v: u32 = workgroupUniformLoad(&a)` must type-check.
func TestWorkgroupUniformLoadOfAtomic(t *testing.T) {
	src := "var<workgroup> a: atomic<u32>;\n@compute @workgroup_size(1) fn main() {\n" +
		"    let v: u32 = workgroupUniformLoad(&a);\n    _ = v;\n}\n"
	m, err := compileWGSL(t, src)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	fn := &m.EntryPoints[0].Function
	for h, e := range fn.Expressions {
		if _, ok := e.Kind.(ir.ExprWorkGroupUniformLoadResult); !ok {
			continue
		}
		res, err := ir.ResolveExpressionType(m, fn, ir.ExpressionHandle(h))
		if err != nil {
			t.Fatal(err)
		}
		if got := ir.TypeResInner(m, res); got != (ir.ScalarType{Kind: ir.ScalarUint, Width: 4}) {
			t.Errorf("result type = %#v, want u32", got)
		}
		return
	}
	t.Fatal("no WorkGroupUniformLoadResult")
}

// The bitcast operand is concretized, as in Rust naga (front/wgsl/lower/mod.rs,
// "bitcast" arm: self.expression(), which lowers and concretizes). An abstract
// operand left in the function arena has no type a backend can write (MSL
// printed invalid_type_4294967295) and fails validation.
func TestBitcastConcretizesOperand(t *testing.T) {
	tests := []struct{ name, src string }{
		{"abstract int constant", "const A = 1;\nfn g() -> u32 { return bitcast<u32>(A); }"},
		{"abstract int literal", "fn g() -> u32 { return bitcast<u32>(1); }"},
		{"abstract float constant", "const F = 2.5;\nfn g() -> u32 { return bitcast<u32>(F); }"},
		{"abstract vector constant", "const V = vec2(1, 2);\nfn g() -> vec2<f32> { return bitcast<vec2<f32>>(V); }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := helperCompileWGSL(t, tt.src+"\n@compute @workgroup_size(1) fn main() { _ = g(); }\n")
			var fn *ir.Function
			for i := range m.Functions {
				if m.Functions[i].Name == "g" {
					fn = &m.Functions[i]
				}
			}
			if fn == nil {
				t.Fatal("function g not found")
			}
			found := false
			for _, e := range fn.Expressions {
				as, ok := e.Kind.(ir.ExprAs)
				if !ok || as.Convert != nil {
					continue
				}
				found = true
				res, err := ir.ResolveExpressionType(m, fn, as.Expr)
				if err != nil {
					t.Fatalf("resolve bitcast operand: %v", err)
				}
				if inner := ir.TypeResInner(m, res); inner == nil || ir.IsAbstractType(inner, m.Types) {
					t.Errorf("bitcast operand type = %v, want a concrete type", inner)
				}
			}
			if !found {
				t.Fatal("no bitcast (As without Convert) in g")
			}
			if errs, err := ir.Validate(m); err != nil || len(errs) > 0 {
				t.Errorf("validation: %v %v", err, errs)
			}
		})
	}
}
