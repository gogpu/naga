package lower

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/wgsl/internal/parser"
)

// WGSL §arithmetic-expr: division and remainder are shader-creation errors
// when the divisor is a const-expression that evaluates to zero, and a shift
// is one when the shift amount is a const-expression at or above the bit
// width of the shifted value. A let is a run-time expression, and an override
// is an override-expression: the spec makes those a pipeline-creation error,
// not a shader-creation one, so the frontend must not reject them.
//
// Rust naga checks both in its validator (valid/expression.rs
// validate_constant_divisor, validate_constant_shift_amounts), where a let
// bound to a constant is indistinguishable from the constant; there it
// rejects valid shaders. The check lives in the lowerer instead, which knows
// the evaluation stage of every expression (P1-WGSL-101).

const scalarStorage = `
@group(0) @binding(0) var<storage, read_write> o: array<u32, 4>;
`

const vectorStorage = `
@group(0) @binding(0) var<storage, read_write> o: array<vec2<u32>, 4>;
`

// entryWith wraps body in a compute entry point over decl.
func entryWith(decl, body string) string {
	return decl + `
@compute @workgroup_size(1)
fn main() {
` + body + `
}
`
}

// loweringMessages returns the messages of a lowering error: the first is the
// error, the rest are its notes.
func loweringMessages(t *testing.T, err error) []string {
	t.Helper()
	var list *parser.SourceErrors
	if !errors.As(err, &list) || list == nil {
		t.Fatalf("err = %T (%v), want *parser.SourceErrors", err, err)
	}
	messages := make([]string, 0, len(*list))
	for _, e := range *list {
		messages = append(messages, e.Message)
	}
	if len(messages) == 0 {
		t.Fatal("lowering error carries no messages")
	}
	return messages
}

// wantLoweringError asserts that the first message contains want and that a
// note cites the spec section.
func wantLoweringError(t *testing.T, err error, want string) {
	t.Helper()
	messages := loweringMessages(t, err)
	if !strings.Contains(messages[0], want) {
		t.Errorf("error = %q, want it to contain %q", messages[0], want)
	}
	for _, note := range messages[1:] {
		if strings.Contains(note, "WGSL spec §arithmetic-expr") {
			return
		}
	}
	t.Errorf("notes = %q, want one to cite WGSL spec §arithmetic-expr", messages[1:])
}

// TestConstDivisorZeroRejected covers the const-expression divisors: a
// literal, a const local, one zero component of a const vector, and a
// compound assignment, which WGSL defines through the same binary operation.
func TestConstDivisorZeroRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"divide literal", entryWith(scalarStorage, "o[0] = o[0] / 0u;")},
		{"modulo literal", entryWith(scalarStorage, "o[0] = o[0] % 0u;")},
		{"divide const local", entryWith(scalarStorage, "const z = 0u;\n    o[0] = o[0] / z;")},
		{"modulo const local", entryWith(scalarStorage, "const z = 0u;\n    o[0] = o[0] % z;")},
		{"divide signed zero", entryWith(scalarStorage, "o[0] = o[0] / 0i;")},
		{"vector component", entryWith(vectorStorage, "o[0] = o[0] / vec2<u32>(1u, 0u);")},
		{"compound divide", entryWith(scalarStorage, "o[0] /= 0u;")},
		{"compound modulo", entryWith(scalarStorage, "o[0] %= 0u;")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileWGSL(t, tc.src)
			if err == nil {
				t.Fatal("compiled, want Division by zero")
			}
			wantLoweringError(t, err, "Division by zero")
		})
	}
}

// TestConstShiftTooLargeRejected covers the const-expression shift amounts at
// and above the bit width of the shifted value, for both shift directions, a
// const local, a vector component and a compound assignment. The type text is
// Rust's: ShiftAmountTooLarge embeds the Debug of TypeInner, where a scalar
// type prints as Scalar(Scalar { ... }) and a vector as Vector { ... }.
func TestConstShiftTooLargeRejected(t *testing.T) {
	const scalarType = "Scalar(Scalar { kind: Uint, width: 4 })"
	const vectorType = "Vector { size: Bi, scalar: Scalar { kind: Uint, width: 4 } }"
	for _, tc := range []struct {
		name string
		src  string
		typ  string
	}{
		{"shift left literal", entryWith(scalarStorage, "o[0] = o[0] << 32u;"), scalarType},
		{"shift right literal", entryWith(scalarStorage, "o[0] = o[0] >> 32u;"), scalarType},
		{"shift left const local", entryWith(scalarStorage, "const s = 32u;\n    o[0] = o[0] << s;"), scalarType},
		{"shift left huge", entryWith(scalarStorage, "o[0] = o[0] << 4294967295u;"), scalarType},
		{"vector component", entryWith(vectorStorage, "o[0] = o[0] << vec2<u32>(32u, 1u);"), vectorType},
		{"compound shift", entryWith(scalarStorage, "o[0] <<= 32u;"), scalarType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileWGSL(t, tc.src)
			if err == nil {
				t.Fatal("compiled, want a shift amount error")
			}
			wantLoweringError(t, err, "Shift amount exceeds the bit width of "+tc.typ)
		})
	}
}

// TestShiftAmountBoundaryAccepted keeps the largest valid shift amount.
func TestShiftAmountBoundaryAccepted(t *testing.T) {
	if _, err := compileWGSL(t, entryWith(scalarStorage, "o[0] = o[0] << 31u;\n    o[0] = o[0] >> 31u;")); err != nil {
		t.Errorf("shift by 31 compiled with error: %v", err)
	}
}

// TestLetAndOverrideOperandsAreRuntime keeps the valid shaders: a let is a
// run-time expression whatever it is bound to, and an override-expression is a
// pipeline-creation error at most — the frontend has no pipeline constants
// and must compile it.
func TestLetAndOverrideOperandsAreRuntime(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"let zero divisor", entryWith(scalarStorage, "let z = 0u;\n    o[0] = o[0] / z;")},
		{"let zero modulo", entryWith(scalarStorage, "let z = 0u;\n    o[0] = o[0] % z;")},
		{"let large shift", entryWith(scalarStorage, "let s = 40u;\n    o[0] = o[0] << s;")},
		{"let compound", entryWith(scalarStorage, "let z = 0u;\n    o[0] /= z;")},
		{"override zero divisor", entryWith(scalarStorage+"\noverride d: u32 = 0u;", "o[0] = o[0] / d;")},
		{"override large shift", entryWith(scalarStorage+"\noverride sh: u32 = 40u;", "o[0] = o[0] << sh;")},
		{"let bound to a constant", entryWith(scalarStorage, "const z = 0u;\n    let lz = z;\n    o[0] = o[0] / lz;")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := compileWGSL(t, tc.src); err != nil {
				t.Errorf("compiled with error: %v", err)
			}
		})
	}
}
