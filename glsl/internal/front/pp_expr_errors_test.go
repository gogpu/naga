package front

// Contracts of the preprocessor's integer expression evaluator: a malformed,
// overflowing or non-integer conditional is a preprocessor error, and each
// operator level refuses to continue without its right operand. Rust naga has
// no preprocessor of its own; the messages follow the design's diagnostic style
// (design §3) and the operator table of GLSL's preprocessor grammar.

import "testing"

func TestPreprocessExpressionErrors(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"float", "1.5", "floating-point value in a preprocessor expression"},
		{"empty", "", "expected an expression"},
		{"unclosed parenthesis", "(1", "expected ')' in a preprocessor expression"},
		{"logical or without operand", "1 ||", "unexpected end of a preprocessor expression"},
		{"logical and without operand", "1 &&", "unexpected end of a preprocessor expression"},
		{"bit or without operand", "1 |", "unexpected end of a preprocessor expression"},
		{"bit xor without operand", "1 ^", "unexpected end of a preprocessor expression"},
		{"bit and without operand", "1 &", "unexpected end of a preprocessor expression"},
		{"equality without operand", "1 ==", "unexpected end of a preprocessor expression"},
		{"comparison without operand", "1 <", "unexpected end of a preprocessor expression"},
		{"shift without operand", "1 <<", "unexpected end of a preprocessor expression"},
		{"additive without operand", "1 +", "unexpected end of a preprocessor expression"},
		{"multiplicative without operand", "1 *", "unexpected end of a preprocessor expression"},
		{"unary without operand", "!", "unexpected end of a preprocessor expression"},
		{"defined without name", "defined(", "expected a macro name after defined"},
		{"division by zero", "1 / 0", "division by zero in a preprocessor expression"},
		{"remainder by zero", "1 % 0", "division by zero in a preprocessor expression"},
		{"shift out of range", "1 << 99", "shift count out of range in a preprocessor expression"},
		{"addition overflow", "9223372036854775807 + 1", "integer overflow in a preprocessor expression"},
		{"multiplication overflow", "9223372036854775807 * 2", "integer overflow in a preprocessor expression"},
		{"subtraction overflow", "-9223372036854775807 - 2", "integer overflow in a preprocessor expression"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "#version 450\n#if " + tt.expr + "\n#endif\nvoid main() {}\n"
			res := preprocess(source, nil)
			if len(res.Errors) == 0 {
				t.Fatalf("the conditional %q was accepted", tt.expr)
			}
			if res.Errors[0].Message != tt.want {
				t.Fatalf("error = %q, want %q", res.Errors[0].Message, tt.want)
			}
		})
	}
}

// TestPreprocessExpressionAccepts pins the forms the evaluator accepts so the
// error table above is not hiding a parser that refuses everything: an
// unknown macro name is 0, a signed division by -1 is fine, and octal, hex and
// unsigned suffixes are read.
func TestPreprocessExpressionAccepts(t *testing.T) {
	for _, expr := range []string{
		"UNKNOWN",
		"1 / -1",
		"010 + 1",
		"0x10 + 1",
		"1u + 1u",
		"defined(UNKNOWN)",
		"(1 + 2) * 3 == 9",
	} {
		source := "#version 450\n#if " + expr + "\n#endif\nvoid main() {}\n"
		if res := preprocess(source, nil); len(res.Errors) != 0 {
			t.Errorf("the conditional %q was refused: %v", expr, res.Errors[0].Message)
		}
	}
}
