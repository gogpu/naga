package front

// Contracts of the unsized array constructor, Rust naga's parse_postfix
// (front/glsl/parser/expressions.rs:130-160): the source writes a type with
// ArraySize::Dynamic, and the constructor takes the size of its argument list.
// The empty form has no size to take and is rejected.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// arrayComposeOf returns the first ExprCompose of an array type in a
// function, and that array's type inner.
func arrayComposeOf(t *testing.T, mod *ir.Module, function ir.Function) (ir.ExprCompose, ir.ArrayType) {
	t.Helper()
	for _, expr := range function.Expressions {
		compose, ok := expr.Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		array, ok := mod.Types[compose.Type].Inner.(ir.ArrayType)
		if !ok {
			continue
		}
		return compose, array
	}
	t.Fatalf("function %q has no array constructor", function.Name)
	return ir.ExprCompose{}, ir.ArrayType{}
}

// constantSize returns the constant element count of an array size, or 0 for a
// dynamic-sized array.
func constantSize(size ir.ArraySize) uint32 {
	if size.Constant == nil {
		return 0
	}
	return *size.Constant
}

// TestConstructorUnsizedArrayTakesArgumentCount is the three-argument form of
// `float[](4.0, 5.0, 6.0)`: the constructor's type is a sized array of three
// floats, not the dynamic-sized one the source writes
// (expressions.rs:136-152), so the module both validates and gives every
// backend a constructible composite type.
func TestConstructorUnsizedArrayTakesArgumentCount(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    float arr[3] = float[](4.0, 5.0, 6.0);
    o = vec4(arr[0], arr[1], arr[2], 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	compose, array := arrayComposeOf(t, mod, main)
	if got := constantSize(array.Size); got != 3 {
		t.Fatalf("array constructor size = %d, want the argument count 3 (size %+v)", got, array.Size)
	}
	if _, ok := mod.Types[array.Base].Inner.(ir.ScalarType); !ok {
		t.Fatalf("array base = %T, want a scalar", mod.Types[array.Base].Inner)
	}
	if len(compose.Components) != 3 {
		t.Fatalf("array constructor has %d components, want 3", len(compose.Components))
	}
}

// TestConstructorUnsizedArrayOneArgument is the one-argument form: the size is
// one, which is also the smallest size the source can name.
func TestConstructorUnsizedArrayOneArgument(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    float arr[1] = float[](7.0);
    o = vec4(arr[0]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	_, array := arrayComposeOf(t, mod, main)
	if got := constantSize(array.Size); got != 1 {
		t.Fatalf("array constructor size = %d, want 1 (size %+v)", got, array.Size)
	}
}

// TestConstructorUnsizedArrayEmptyIsAnError is the empty form: Rust refuses it
// with "There must be at least one argument" (expressions.rs:146-152), because
// an array of zero elements has no size to build (P0-GLSLIN-004).
func TestConstructorUnsizedArrayEmptyIsAnError(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    float arr[1] = float[]();
    o = vec4(arr[0]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatalf("Parse succeeded with %v globals, want the empty constructor refused", len(mod.GlobalVariables))
	}
	if want := "There must be at least one argument"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

// TestConstructorUnsizedArrayNested is the same rewrite inside a larger
// expression: the constructor is indexed, so the array type is met through
// postfix parsing rather than an initializer.
func TestConstructorUnsizedArrayNested(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    float x = float[](4.0, 5.0, 6.0)[1];
    o = vec4(x);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	main := functionNamed(t, mod, "main")
	compose, array := arrayComposeOf(t, mod, main)
	if got := constantSize(array.Size); got != 3 {
		t.Fatalf("array constructor size = %d, want the argument count 3 (size %+v)", got, array.Size)
	}
	if len(compose.Components) != 3 {
		t.Fatalf("array constructor has %d components, want 3", len(compose.Components))
	}
}
