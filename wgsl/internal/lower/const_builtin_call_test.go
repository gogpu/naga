package lower

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// flattenConstGE returns the scalar leaves of a fully evaluated global
// expression (literals, composes, splats and zero values), formatted so f32
// values compare at f32 precision.
func flattenConstGE(t *testing.T, m *ir.Module, h ir.ExpressionHandle) []string {
	t.Helper()
	if int(h) >= len(m.GlobalExpressions) {
		t.Fatalf("global expression %d out of range", h)
	}
	switch e := m.GlobalExpressions[h].Kind.(type) {
	case ir.Literal:
		switch v := e.Value.(type) {
		case ir.LiteralF32:
			return []string{fmt.Sprintf("%g", float32(v))}
		case ir.LiteralF64:
			return []string{fmt.Sprintf("%g", float64(v))}
		case ir.LiteralI32:
			return []string{fmt.Sprintf("%d", int32(v))}
		case ir.LiteralU32:
			return []string{fmt.Sprintf("%du", uint32(v))}
		case ir.LiteralBool:
			return []string{fmt.Sprintf("%t", bool(v))}
		default:
			t.Fatalf("global expression %d: unexpected literal %T", h, v)
		}
	case ir.ExprCompose:
		var out []string
		for _, c := range e.Components {
			out = append(out, flattenConstGE(t, m, c)...)
		}
		return out
	default:
		t.Fatalf("global expression %d is %T, want an evaluated constant", h, e)
	}
	return nil
}

func constByName(t *testing.T, m *ir.Module, name string) ir.Constant {
	t.Helper()
	for _, c := range m.Constants {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("constant %q not in module", name)
	return ir.Constant{}
}

func f32s(vs ...float64) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = fmt.Sprintf("%g", float32(v))
	}
	return out
}

// Issue #93: a module-scope const initializer may call any const builtin, and
// the call is evaluated at shader-creation time, as in a function body.
func TestModuleConstBuiltinCalls(t *testing.T) {
	tests := []struct {
		name string
		decl string // declarations; the constant under test is X
		want []string
	}{
		{"any", "const X: bool = any(vec2<bool>(true, false));", []string{"true"}},
		{"all", "const X: bool = all(vec2<bool>(true, false));", []string{"false"}},
		{"max", "const X: f32 = max(1.0, 2.0);", f32s(2)},
		{"abs", "const X: f32 = abs(-1.0);", f32s(1)},
		{"normalize", "const X: vec2<f32> = normalize(vec2<f32>(3.0, 4.0));", f32s(0.6, 0.8)},
		{"call as operand", "const X: f32 = 1.0 / sqrt(2.0);", f32s(1 / math.Sqrt(2))},
		{"nested calls", "const X: f32 = max(abs(-3.0), sqrt(4.0));", f32s(3)},
		{"argument is a constant", "const K: f32 = 16.0;\nconst X: f32 = sqrt(K);", f32s(4)},
		{"integer", "const X: i32 = clamp(5, 0, 3);", []string{"3"}},
		{"unsigned", "const X: u32 = countOneBits(7u);", []string{"3u"}},
		{"inferred concrete type", "const X = max(1.0f, 2.5f);", f32s(2.5)},
		{"abstract constant", "const A = clamp(5, 0, 3);\nconst X: i32 = A * 2;", []string{"6"}},
		{"vector arithmetic", "const X: vec2<f32> = 2.0 * normalize(vec2<f32>(0.0, 5.0));", f32s(0, 2)},
		{"select", "const X: f32 = select(1.0, 2.0, any(vec2<bool>(false, true)));", f32s(2)},
		{"dot", "const X: f32 = dot(vec3<f32>(1.0, 2.0, 3.0), vec3<f32>(4.0, 5.0, 6.0));", f32s(32)},
		{"length", "const X: f32 = length(vec2<f32>(3.0, 4.0));", f32s(5)},
		{"distance", "const X: f32 = distance(vec2<f32>(1.0, 1.0), vec2<f32>(4.0, 5.0));", f32s(5)},
		{"reflect", "const X: vec2<f32> = reflect(vec2<f32>(1.0, -1.0), vec2<f32>(0.0, 1.0));", f32s(1, 1)},
		// dot(e2, e3) >= 0 gives -e1 = -(0, 1): IEEE negation keeps the sign
		// of zero (Rust v30: F32(-0.0) F32(-1.0)).
		{"faceForward", "const X: vec2<f32> = faceForward(vec2<f32>(0.0, 1.0), vec2<f32>(0.0, 1.0), vec2<f32>(0.0, 1.0));", []string{"-0", "-1"}},
		{"extractBits", "const X: u32 = extractBits(0xF0u, 4u, 4u);", []string{"15u"}},
		{"insertBits", "const X: u32 = insertBits(0u, 0xFu, 4u, 4u);", []string{"240u"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.decl + "\n@compute @workgroup_size(1) fn main() { var x = X; }\n"
			m, err := compileWGSL(t, src)
			if err != nil {
				t.Fatalf("lowering failed: %v", err)
			}
			c := constByName(t, m, "X")
			got := flattenConstGE(t, m, c.Init)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("X = %v, want %v", got, tt.want)
			}
			if errs, err := ir.Validate(m); err != nil || len(errs) > 0 {
				t.Errorf("validation: %v %v", err, errs)
			}
		})
	}
}

// A const initializer must be a const-expression: builtins that are not
// const-functions, and override or runtime values, are errors.
func TestModuleConstBuiltinCallRejectsNonConst(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"override argument", "override O: f32 = 1.0;\nconst X: f32 = sqrt(O);"},
		{"not a const function", "@group(0) @binding(0) var t: texture_2d<f32>;\nconst X: vec2<u32> = textureDimensions(t);"},
		{"unknown function", "const X: f32 = nosuch(1.0);"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.src + "\n@compute @workgroup_size(1) fn main() { var x = X; }\n"
			if _, err := compileWGSL(t, src); err == nil {
				t.Fatal("lowering accepted a const initializer that is not a const-expression")
			} else if !strings.Contains(err.Error(), "'X'") {
				t.Errorf("error %q does not name the constant", err)
			}
		})
	}
}
