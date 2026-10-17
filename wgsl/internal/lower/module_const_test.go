package lower

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// geShape prints a global expression tree compactly: literals with their
// type, Compose/Splat with their operands. Handles are not printed, so two
// trees compare by structure.
func geShape(t *testing.T, m *ir.Module, h ir.ExpressionHandle) string {
	t.Helper()
	if int(h) >= len(m.GlobalExpressions) {
		t.Fatalf("global expression %d out of range (%d)", h, len(m.GlobalExpressions))
	}
	switch e := m.GlobalExpressions[h].Kind.(type) {
	case ir.Literal:
		return reflect.TypeOf(e.Value).Name() + "(" + fmtLiteral(e.Value) + ")"
	case ir.ExprCompose:
		parts := make([]string, len(e.Components))
		for i, c := range e.Components {
			parts[i] = geShape(t, m, c)
		}
		return "Compose[" + strings.Join(parts, " ") + "]"
	case ir.ExprSplat:
		return "Splat" + string('0'+rune(e.Size)) + "[" + geShape(t, m, e.Value) + "]"
	case ir.ExprZeroValue:
		return "ZeroValue"
	default:
		t.Fatalf("global expression %d is %T, want an evaluated constant", h, e)
	}
	return ""
}

func fmtLiteral(v ir.LiteralValue) string {
	switch x := v.(type) {
	case ir.LiteralF32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case ir.LiteralF64:
		return strconv.FormatFloat(float64(x), 'g', -1, 64)
	case ir.LiteralI32:
		return strconv.FormatInt(int64(x), 10)
	case ir.LiteralU32:
		return strconv.FormatUint(uint64(x), 10)
	case ir.LiteralBool:
		return strconv.FormatBool(bool(x))
	}
	return "?"
}

// Module constants are lowered like Rust naga's GlobalDeclKind::Const: the
// initializer is evaluated in Module.GlobalExpressions, abstract constants
// are real constants with abstract types that compact removes at the end,
// and a typed declaration converts the abstract value (f64 precision for
// AbstractFloat, i64 for AbstractInt). Expected IR: Rust naga v30.
func TestModuleConstFromAbstract(t *testing.T) {
	tests := []struct {
		name  string
		decls string
		want  map[string]string // surviving constant -> init shape
	}{
		// 1.79e308 is not an f32: an AbstractFloat stored as f32 would be +Inf.
		{"abstract float keeps f64 precision",
			"const A = 1.79e308; const B = A / 1.0e300; const X: f32 = B;",
			map[string]string{"X": "LiteralF32(1.79e+08)"}},
		{"abstract int to u32", "const A = 3; const X: u32 = A;",
			map[string]string{"X": "LiteralU32(3)"}},
		{"abstract int + float", "const A = 3; const Y = A + 1.5; const X: f32 = Y;",
			map[string]string{"X": "LiteralF32(4.5)"}},
		{"abstract array index", "const ARR = array(1, 2, 3); const X: u32 = ARR[1];",
			map[string]string{"X": "LiteralU32(2)"}},
		{"abstract matrix column", "const M = mat2x2(1.0, 2.0, 3.0, 4.0); const X: f32 = M[1][0];",
			map[string]string{"X": "LiteralF32(3)"}},
		{"abstract vector arithmetic", "const V = vec3(1, 2, 3) * 2; const X: vec3<f32> = V;",
			map[string]string{"X": "Compose[LiteralF32(2) LiteralF32(4) LiteralF32(6)]"}},
		{"splat stays a splat", "const V = vec2(7); const X: vec2<u32> = V;",
			map[string]string{"X": "Splat2[LiteralU32(7)]"}},
		{"struct member", "struct S { a: f32, b: u32 } const S0 = S(1.5, 2); const X = S0.b;",
			map[string]string{"S0": "Compose[LiteralF32(1.5) LiteralU32(2)]", "X": "LiteralU32(2)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.decls + "\n@compute @workgroup_size(1) fn main() { var x = X; }\n"
			m, err := compileWGSL(t, src)
			if err != nil {
				t.Fatalf("lowering failed: %v", err)
			}
			got := map[string]string{}
			for _, c := range m.Constants {
				got[c.Name] = geShape(t, m, c.Init)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("constants = %v, want %v", got, tt.want)
			}
			for i, ty := range m.Types {
				if ir.IsAbstractType(ty.Inner, m.Types) {
					t.Errorf("type [%d] %v is abstract; compact must remove abstract types", i, ty.Inner)
				}
			}
			if errs, err := ir.Validate(m); err != nil || len(errs) > 0 {
				t.Errorf("validation: %v %v", err, errs)
			}
		})
	}
}

// X = S0.b refers to S0's component: the evaluator sees through the constant
// and shares the literal (Rust: X.init == S0's second component).
func TestModuleConstAccessSharesComponent(t *testing.T) {
	src := "struct S { a: f32, b: u32 }\nconst S0 = S(1.5, 2);\nconst X = S0.b;\n" +
		"@compute @workgroup_size(1) fn main() { var x = X; }\n"
	m, err := compileWGSL(t, src)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	s0, x := constByName(t, m, "S0"), constByName(t, m, "X")
	comp, ok := m.GlobalExpressions[s0.Init].Kind.(ir.ExprCompose)
	if !ok || len(comp.Components) != 2 {
		t.Fatalf("S0 init is %T, want a 2-component Compose", m.GlobalExpressions[s0.Init].Kind)
	}
	if x.Init != comp.Components[1] {
		t.Errorf("X.Init = %d, want S0's component %d (shared, as Rust)", x.Init, comp.Components[1])
	}
	if len(m.GlobalExpressions) != 3 {
		t.Errorf("%d global expressions, want 3 (Rust: F32, U32, Compose)", len(m.GlobalExpressions))
	}
}

// An abstract constant used in a function is converted where it is used; the
// constant itself is not in the module (Rust compacts it away).
func TestModuleConstAbstractUsedInFunction(t *testing.T) {
	tests := []struct {
		name  string
		decls string
		want  ir.LiteralValue
	}{
		{"int", "const A = 1; const X = A;", ir.LiteralI32(1)},
		{"float alias chain", "const A = 2.5; const B = A; const X = B * 2;", ir.LiteralF32(5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.decls + "\n@compute @workgroup_size(1) fn main() { var x = X; }\n"
			m, err := compileWGSL(t, src)
			if err != nil {
				t.Fatalf("lowering failed: %v", err)
			}
			if len(m.Constants) != 0 || len(m.GlobalExpressions) != 0 {
				t.Errorf("module keeps %d constants, %d global expressions; Rust keeps none",
					len(m.Constants), len(m.GlobalExpressions))
			}
			fn := &m.EntryPoints[0].Function
			if len(fn.Expressions) != 1 {
				t.Fatalf("main has %d expressions, want 1 (Rust: the converted literal)", len(fn.Expressions))
			}
			lit, ok := fn.Expressions[0].Kind.(ir.Literal)
			if !ok || lit.Value != tt.want {
				t.Errorf("main expression = %#v, want Literal(%#v)", fn.Expressions[0].Kind, tt.want)
			}
		})
	}
}

func TestModuleConstAbstractNotRepresentable(t *testing.T) {
	src := "const A = 3000000000;\nconst X: i32 = A;\n@compute @workgroup_size(1) fn main() { var x = X; }\n"
	_, err := compileWGSL(t, src)
	if err == nil {
		t.Fatal("lowering accepted 3000000000 as an i32")
	}
	const want = "the concrete type `i32` cannot represent the abstract value `3000000000` accurately"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q, want it to contain Rust's %q", err, want)
	}
}

// A phony assignment converts its value (Rust expression()), which reads the
// value's type: types registered for builtin results must be in Module.Types
// at once (subgroupBallot's vec4<u32> was only in the registry).
func TestPhonyOfBuiltinResult(t *testing.T) {
	src := "enable subgroups;\n@compute @workgroup_size(1) fn main() {\n" +
		"    _ = subgroupBallot();\n    _ = subgroupBallot(true);\n}\n"
	if _, err := compileWGSL(t, src); err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
}
