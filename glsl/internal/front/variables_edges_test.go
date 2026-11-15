package front

// Contracts of the pieces the module-scope tests do not reach: field selection
// and swizzles (Rust naga's Frontend::field_selection, front/glsl/variables.rs:
// 244-413), the braced initializer element types (parser/declarations.rs:31-66),
// the local storage qualifiers (variables.rs:670-681) and the switch
// diagnostics of parser/functions.rs:197-226.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// TestFieldSelectionErrors covers the refusals of field_selection
// (variables.rs:244-413).
func TestFieldSelectionErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown struct field",
			body: "    S s;\n    o = vec4(s.nope);\n",
			want: "Unknown field: nope",
		},
		{
			name: "invalid swizzle",
			body: "    vec2 v = vec2(1.0);\n    o = vec4(v.q);\n",
			want: `Invalid swizzle for vector "q"`,
		},
		{
			name: "swizzle too wide",
			body: "    vec4 v = vec4(1.0);\n    o = vec4(v.xyzwx);\n",
			want: `Bad swizzle size for ""xyzwx""`,
		},
		{
			name: "field on a scalar",
			body: "    float f = 1.0;\n    o = vec4(f.x);\n",
			want: `Can't lookup field on this type "x"`,
		},
		{
			name: "duplicate swizzle in lhs",
			body: "    vec4 v = vec4(1.0);\n    v.xx = vec2(2.0);\n    o = v;\n",
			want: "swizzle cannot have duplicate components in left-hand-side expression",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `#version 450
layout(location = 0) out vec4 o;
struct S { float a; };
void main() {
` + tt.body + `}
`
			_, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestNestedSwizzleFlattens is variables.rs:268-283: vec.zyx.xy is vec.z, so
// the inner pattern is applied to the outer one instead of nesting swizzles.
func TestNestedSwizzleFlattens(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { vec4 v; } u;
layout(location = 0) out vec4 o;
void main() {
    o = vec4(u.v.zyx.xy, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	// The flattened result is a vec2 swizzle over the vector itself; the inner
	// vec3 swizzle stays in the arena as a dead expression.
	found := false
	for _, expr := range main.Expressions {
		outer, ok := expr.Kind.(ir.ExprSwizzle)
		if !ok || outer.Size != ir.Vec2 {
			continue
		}
		found = true
		if _, nested := exprKindOf(t, main, outer.Vector).(ir.ExprSwizzle); nested {
			t.Fatal("the outer swizzle still points at the inner one, want the flattened pattern")
		}
	}
	if !found {
		t.Fatal("no flattened vec2 swizzle in the module")
	}
}

// TestSwizzleOfPointerLoads is the isPointer arm of the vector branch
// (variables.rs:290-330): the swizzle target of an Lhs field selection is a
// pointer, so it is loaded before the swizzle is built.
func TestSwizzleOfPointerLoads(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
struct S { vec4 v; };
void main() {
    S s;
    s.v.yx = vec2(1.0, 2.0);
    o = s.v;
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	for _, expr := range main.Expressions {
		swizzle, ok := expr.Kind.(ir.ExprSwizzle)
		if !ok {
			continue
		}
		load, ok := exprKindOf(t, main, swizzle.Vector).(ir.ExprLoad)
		if !ok {
			continue
		}
		if _, ok := exprKindOf(t, main, load.Pointer).(ir.ExprAccessIndex); !ok {
			t.Fatalf("the load reads %#v, want the member access", exprKindOf(t, main, load.Pointer))
		}
		return
	}
	t.Fatal("no swizzle over a load of the member pointer")
}

// TestOneComponentSwizzleOfPointerIsAnAccess is variables.rs:310-320: a
// one-component swizzle is an AccessIndex, and in an Rhs position the pointer
// is loaded before it.
func TestOneComponentSwizzleOfPointerIsAnAccess(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { vec4 v; } u;
layout(location = 0) out vec4 o;
void main() {
    o = vec4(u.v.z);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	for _, expr := range main.Expressions {
		if _, ok := expr.Kind.(ir.ExprSwizzle); ok {
			t.Fatal("a one-component swizzle lowered to ExprSwizzle, want an access")
		}
	}
	accesses := 0
	for _, expr := range main.Expressions {
		if _, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			accesses++
		}
	}
	if accesses == 0 {
		t.Fatal("no access index in the module")
	}
}

// TestBracedInitializersUseElementTypes is initializer_element_type
// (parser/declarations.rs:31-66): a braced initializer lowers one element per
// component, column, element, member or the type itself.
func TestBracedInitializersUseElementTypes(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
struct S { float a; float b; };
void main() {
    vec2 v = {1.0, 2.0};
    mat2 m = {vec2(1.0, 2.0), vec2(3.0, 4.0)};
    float a[2] = {5.0, 6.0};
    S s = {7.0, 8.0};
    float f = {9.0};
    o = vec4(v + m[0] + vec2(a[0], s.a) + vec2(f), 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	if len(main.LocalVars) < 5 {
		t.Fatalf("main declares %d locals, want the five braced ones", len(main.LocalVars))
	}
	// Every braced initializer is a Compose of the declared type.
	composed := map[string]bool{}
	for i, local := range main.LocalVars {
		if local.Init == nil {
			continue
		}
		if _, ok := exprKindOf(t, main, *local.Init).(ir.ExprCompose); ok {
			composed[local.Name] = true
		}
		_ = i
	}
	for _, name := range []string{"v", "m", "a", "s"} {
		if !composed[name] {
			t.Errorf("local %q has no compose initializer: %+v", name, main.LocalVars)
		}
	}
}

// TestLocalStorageQualifiers is local_mutability (variables.rs:670-681): a const
// local stays writable=false and any other storage class is reported.
func TestLocalStorageQualifiers(t *testing.T) {
	t.Run("const is immutable", func(t *testing.T) {
		source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    const float k = 2.0;
    o = vec4(k);
}
`
		mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
			t.Fatalf("the parsed module does not validate: %v", verr)
		}
	})
	t.Run("other storage classes are reported", func(t *testing.T) {
		source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    uniform float k;
    o = vec4(k);
}
`
		_, _, err := Parse(source, Options{Stage: ir.StageFragment})
		if err == nil || !strings.Contains(err.Error(), "Locals cannot have a storage qualifier") {
			t.Fatalf("error = %v, want the local storage qualifier error", err)
		}
	})
}

// TestInvalidLocalDeclarationIsIgnored is lower_declaration's recovery node
// (declarations.rs:400-416): a declaration the parser already reported does not
// stop lowering.
func TestInvalidLocalDeclarationIsIgnored(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(location = 0) out vec4 o;
void main() {
    void x;
    o = vec4(1.0);
}
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "Declaration cannot have void type") {
		t.Fatalf("error = %v, want the parser's void declaration error", err)
	}
}

// TestSwitchDiagnostics covers the switch checks of parser/functions.rs:197-226
// and the validator's duplicate-case texts of lower_stmt.go:628-660.
func TestSwitchDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "non-integer label",
			body: "    switch (u.k) { case 1.5: break; }\n",
			want: "Case values can only be integers",
		},
		{
			name: "duplicate case",
			body: "    switch (u.k) { case 1: break; case 1: break; }\n",
			want: "Multiple `switch` cases for I32(1) are present",
		},
		{
			name: "duplicate default",
			body: "    switch (u.k) { default: break; default: break; }\n",
			want: "Multiple `default` cases are present",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `#version 450
layout(set = 0, binding = 0) uniform U { int k; } u;
layout(location = 0) out vec4 o;
void main() {
` + tt.body + "    o = vec4(1.0);\n}\n"
			_, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestSwitchValueDebug is switchValueDebug's table, the Debug spelling Rust
// uses in the duplicate-case text.
func TestSwitchValueDebug(t *testing.T) {
	tests := []struct {
		value ir.SwitchValue
		want  string
	}{
		{ir.SwitchValueI32(-2), "I32(-2)"},
		{ir.SwitchValueU32(3), "U32(3)"},
		{ir.SwitchValueDefault{}, "Default"},
	}
	for _, tt := range tests {
		if got := switchValueDebug(tt.value); got != tt.want {
			t.Errorf("switchValueDebug(%T) = %q, want %q", tt.value, got, tt.want)
		}
	}
}
