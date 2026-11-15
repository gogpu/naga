package front

// Contracts of the three declaration shapes the module walk does not otherwise
// meet: an invariant redeclaration of a name that does not resolve (Rust's
// make_variable_invariant is silent, variables.rs:226-242), an explicit IO
// location that is not a constant (reserveExplicitIOLocations skips it and the
// declaration reports it, entry.go:65-89), and a precision statement inside a
// function body (lower_stmt.go:216-222).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestInvariantOfUnknownNameIsSilent is entry.go:106-113: the parser admits a
// name it cannot resolve, and lowering does nothing.
func TestInvariantOfUnknownNameIsSilent(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
invariant unknownName;
void main() { o = vec4(1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(mod.GlobalVariables) != 1 {
		t.Fatalf("module has %d globals, want only the output", len(mod.GlobalVariables))
	}
}

// TestNonConstantIOLocationIsReportedOnce is entry.go:75-83: a location whose
// constant expression cannot be evaluated is left unreserved, and the
// declaration's own qualifier evaluation reports it.
func TestNonConstantIOLocationIsReportedOnce(t *testing.T) {
	source := `#version 450
layout(constant_id = 0) const int K = 1;
layout(location = K) out vec4 o;
void main() { o = vec4(1.0); }
`
	_, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("Parse accepted a non-constant location")
	}
	if strings.Count(err.Error(), "override-expression") > 1 {
		t.Fatalf("the location was reported more than once: %v", err)
	}
}

// TestInParameterIsCopiedIntoALocal is add_function_arg (context.rs:316-403):
// an `in` parameter the body may write is copied into a local at entry, so a
// write does not reach the caller.
func TestInParameterIsCopiedIntoALocal(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void bump(in float x) { x = x + 1.0; }
void main() {
    float v = 1.0;
    bump(v);
    o = vec4(v);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	bump := functionNamed(t, mod, "bump")
	if len(bump.Arguments) != 1 {
		t.Fatalf("bump has %d arguments, want 1", len(bump.Arguments))
	}
	if len(bump.LocalVars) != 1 {
		t.Fatalf("bump has %d locals, want the parameter copy", len(bump.LocalVars))
	}
	if len(bump.Body) == 0 {
		t.Fatal("bump has an empty body")
	}
	store, ok := bump.Body[0].Kind.(ir.StmtStore)
	if !ok {
		t.Fatalf("bump's body starts with %T, want the parameter copy store", bump.Body[0].Kind)
	}
	if _, isArg := bump.Expressions[store.Value].Kind.(ir.ExprFunctionArgument); !isArg {
		t.Fatalf("the copy reads %T, want the function argument", bump.Expressions[store.Value].Kind)
	}
}

// TestPrecisionStatementInAFunctionIsRejected pins parity with the pinned Rust
// naga oracle (naga.exe --input-kind glsl on the same shader): a precision
// statement inside a function body is refused with the identical message, so
// this frontend is never stricter nor laxer than the reference here. The
// statement-level arm of lowerDeclaration therefore stays unreachable from the
// parser.
func TestPrecisionStatementInAFunctionIsRejected(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
void main() {
    precision highp float;
    o = vec4(1.0);
}
`
	_, _, err := Parse(source, Options{Stage: ir.StageFragment})
	want := "Expected LeftParen, integer literal, float literal or bool literal, found Precision"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want the reference's %q", err, want)
	}
}
