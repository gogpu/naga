package front

// Contracts of two remaining constructor and const-resolution shapes: the
// one-argument array and struct constructors of Rust's constructor_single
// (front/glsl/functions.rs:159-177), and the const context's lazy resolution of
// a module-scope name (context.rs:180-210).

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestConstructorArrayOfOneArgument is constructor_single's array arm
// (functions.rs:170-177): the single argument is converted to the element's
// scalar and composed as the array, as Rust does for the shape it accepts.
func TestConstructorArrayOfOneArgument(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float f; } u;
layout(location = 0) out vec4 o;
void main() {
    float a[2] = float[2](u.f);
    o = vec4(a[0]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	for _, expr := range main.Expressions {
		compose, ok := expr.Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		if _, isArray := mod.Types[compose.Type].Inner.(ir.ArrayType); !isArray {
			continue
		}
		if len(compose.Components) != 1 {
			t.Fatalf("the array compose has %d components, want the one argument", len(compose.Components))
		}
		return
	}
	t.Fatal("no array compose in the module")
}

// TestMaterializeConstRefOverrideIsRefused is Context::variable's const arm
// (context.rs:180-210): the materialized override reference is appended through
// the evaluator, which refuses an override inside a const-expression with Rust's
// override-in-const error. A name the frontend does not know stays unknown.
func TestMaterializeConstRefOverrideIsRefused(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	overrideSym := &glslSymbol{Kind: symGlobal, Name: "K"}
	fe.globalVars = append(fe.globalVars, namedLookup{
		name: "K",
		sym:  overrideSym,
		lookup: globalLookup{
			kind:     lookupOverride,
			override: 2,
			ty:       0,
			entryArg: -1,
		},
	})
	c, err := newContext(fe, true)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	ref, ferr := c.materializeConstRef(overrideSym, ir.Span{})
	if ferr == nil || ferr.Message != "Unexpected override-expression" {
		t.Fatalf("error = %v, %v; want Rust's override-in-const message", ref, ferr)
	}

	unknown := &glslSymbol{Kind: symGlobal, Name: "missing"}
	if ref, ferr := c.materializeConstRef(unknown, ir.Span{}); ferr != nil || ref != nil {
		t.Fatalf("an unknown name materialized: %v, %v", ref, ferr)
	}
}
