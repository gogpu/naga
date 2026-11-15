package front

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// preprocessOK runs the preprocessor and fails the test on any diagnostic.
func preprocessOK(t *testing.T, source string, defines map[string]string) ppResult {
	t.Helper()
	res := preprocess(source, defines)
	if len(res.Errors) > 0 {
		t.Fatalf("preprocess(%q) reported %v", source, res.Errors)
	}
	return res
}

// wantError fails unless one diagnostic contains every substring.
func wantError(t *testing.T, res ppResult, substrings ...string) {
	t.Helper()
	for _, e := range res.Errors {
		matched := true
		for _, s := range substrings {
			if !strings.Contains(e.Message, s) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("errors %v do not contain %v", res.Errors, substrings)
}

// tokenStrings renders a token stream for comparisons.
func tokenStrings(tokens []ppToken) []string {
	out := make([]string, len(tokens))
	for i, t := range tokens {
		switch t.kind {
		case ppIdent:
			out[i] = "ident:" + t.text
		case ppInt:
			out[i] = "int:" + strconv.FormatUint(t.intValue, 10)
		case ppFloat:
			out[i] = "float:" + strconv.FormatFloat(float64(t.floatValue), 'g', -1, 32)
		case ppPunct:
			out[i] = "punct:" + t.text
		case ppHash:
			out[i] = "hash"
		case ppNewline:
			out[i] = "newline"
		default:
			out[i] = "invalid"
		}
	}
	return out
}

func checkTokens(t *testing.T, res ppResult, want ...string) {
	t.Helper()
	got := tokenStrings(res.Tokens)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("tokens = %v, want %v", got, want)
	}
}

func TestPreprocessTriviaAndSpans(t *testing.T) {
	source := "#version 450 core\r\nvoid main() {}\r\n"
	res := preprocessOK(t, source, nil)
	checkTokens(t, res, "ident:void", "ident:main", "punct:(", "punct:)", "punct:{", "punct:}")
	if !res.HasVersion || res.Version.Value != 450 || res.Version.Profile != ppProfileCore {
		t.Fatalf("version = %+v, want 450 core", res.Version)
	}
	main := res.Tokens[1]
	if main.span != (ir.Span{Start: 24, End: 28}) || main.line != 2 {
		t.Errorf("main span = %+v line %d, want bytes 24..28 on line 2", main.span, main.line)
	}
}

func TestPreprocessSplicesAndComments(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{"splice", "fo\\\no", []string{"ident:foo"}},
		{"crlf splice", "a\\\r\nb", []string{"ident:ab"}},
		{"block comment separates", "a/**/b", []string{"ident:a", "ident:b"}},
		{"line comment", "a // b\nc", []string{"ident:a", "ident:c"}},
		{"comment in comment", "a/* b /* c */d", []string{"ident:a", "ident:d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := preprocessOK(t, tt.source, nil)
			checkTokens(t, res, tt.want...)
		})
	}

	res := preprocess("a /* unterminated", nil)
	wantError(t, res, "unterminated comment")
}

func TestPreprocessNumbers(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"0", "int:0"},
		{"42", "int:42"},
		{"077", "int:63"},
		{"0x1f", "int:31"},
		{"42u", "int:42"},
		{"1.5", "float:1.5"},
		{".5", "float:0.5"},
		{"5.", "float:5"},
		{"1e3", "float:1000"},
		{"2.5f", "float:2.5"},
		{"1.", "float:1"},
		{"1.e3", "float:1000"},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			res := preprocessOK(t, tt.source, nil)
			checkTokens(t, res, tt.want)
		})
	}

	for _, source := range []string{"09", "0x", "1l", "1s", "1.5h", "1.5L"} {
		t.Run("invalid "+source, func(t *testing.T) {
			res := preprocess(source, nil)
			if len(res.Errors) == 0 {
				t.Fatalf("preprocess(%q) accepted the literal", source)
			}
		})
	}
}

func TestPreprocessIntegerSuffix(t *testing.T) {
	res := preprocessOK(t, "42 43u", nil)
	if !res.Tokens[0].intSigned || res.Tokens[1].intSigned {
		t.Errorf("signedness = %v %v, want true then false", res.Tokens[0].intSigned, res.Tokens[1].intSigned)
	}
}

func TestPreprocessPunctuation(t *testing.T) {
	want := []string{
		"<<=", ">>=", "++", "--", "&&", "||", "^^", "==", "!=", "<=", ">=", "<<", ">>",
		"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
		"(", ")", "{", "}", "[", "]", "<", ">", ",", ";", ":", "=", "!", "-", "~",
		"+", "*", "/", "%", "|", "^", "&", "?", ".",
	}
	source := strings.Join(want, " ")
	res := preprocessOK(t, source, nil)
	got := tokenStrings(res.Tokens)
	for i := range want {
		if got[i] != "punct:"+want[i] {
			t.Fatalf("token %d = %q, want %q", i, got[i], "punct:"+want[i])
		}
	}

	wantError(t, preprocess("a # b", nil), "unexpected '#'")
	wantError(t, preprocess("a @ b", nil), "unexpected character")
}

func TestPreprocessVersion(t *testing.T) {
	good := []struct {
		source  string
		value   uint16
		profile ppProfile
	}{
		{"#version 450\n", 450, ppProfileCore},
		{"#version 450 core\n", 450, ppProfileCore},
		{"#version 460\n", 460, ppProfileCore},
		{"#version 440 core\n", 440, ppProfileCore},
		{"#version 330 core\n", 330, ppProfileCore},
		{"#version 300 es\n", 300, ppProfileES},
	}
	for _, tt := range good {
		t.Run(strings.TrimSpace(tt.source), func(t *testing.T) {
			res := preprocessOK(t, tt.source, nil)
			if !res.HasVersion || res.Version.Value != tt.value || res.Version.Profile != tt.profile {
				t.Fatalf("version = %+v, want %d %s", res.Version, tt.value, tt.profile)
			}
		})
	}

	bad := []struct {
		source string
		want   string
	}{
		{"void main() {}\n#version 450\n", "#version must occur first"},
		{"#version 420 core\n", "invalid version: 420"},
		{"#version 330\n", "330 requires core"},
		{"#version 300\n", "300 requires es"},
		{"#version 450 es\n", "invalid profile: es"},
		{"#version 450 compatibility\n", "invalid profile: compatibility"},
		{"#version\n", "expected a version number"},
		{"#version 450 core extra\n", "unexpected token after #version"},
	}
	for _, tt := range bad {
		t.Run(tt.source, func(t *testing.T) {
			wantError(t, preprocess(tt.source, nil), tt.want)
		})
	}

	if res := preprocessOK(t, "void main() {}\n", nil); res.HasVersion {
		t.Error("an absent #version was recorded as present")
	}
}

func TestPreprocessExtensionsAndPragmas(t *testing.T) {
	res := preprocessOK(t, `#version 450 core
#extension GL_EXT_shader_explicit_arithmetic_types_float16 : require
#extension GL_KHR_shader_subgroup_basic : enable
#extension GL_KHR_shader_subgroup_basic : warn
#extension GL_KHR_shader_subgroup_basic : disable
#pragma shader_stage(vertex)
#pragma optimize(off)
`, nil)
	want := []struct {
		name     string
		behavior ppExtensionBehavior
	}{
		{"GL_EXT_shader_explicit_arithmetic_types_float16", ppExtensionRequire},
		{"GL_KHR_shader_subgroup_basic", ppExtensionEnable},
		{"GL_KHR_shader_subgroup_basic", ppExtensionWarn},
		{"GL_KHR_shader_subgroup_basic", ppExtensionDisable},
	}
	if len(res.Extensions) != len(want) {
		t.Fatalf("extensions = %v, want %d", res.Extensions, len(want))
	}
	for i, w := range want {
		got := res.Extensions[i]
		if got.Name != w.name || got.Behavior != w.behavior {
			t.Errorf("extension %d = %+v, want %s %s", i, got, w.name, w.behavior)
		}
	}
	if len(res.Pragmas) != 2 || res.Pragmas[0].Name != "shader_stage" {
		t.Fatalf("pragmas = %+v", res.Pragmas)
	}
	if got := tokenStrings(res.Pragmas[0].Args); strings.Join(got, " ") != "punct:( ident:vertex punct:)" {
		t.Errorf("shader_stage args = %v", got)
	}

	wantError(t, preprocess("#extension GL_EXT_x require\n", nil), "expected #extension name : behavior")
	wantError(t, preprocess("#extension GL_EXT_x : sometimes\n", nil), "invalid extension behavior")
	wantError(t, preprocess("#pragma\n", nil), "expected a pragma name")
}

func TestPreprocessDefines(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{"object", "#define A 1\nA", []string{"int:1"}},
		{"object body", "#define A 1 + 2\nA * 3", []string{"int:1", "punct:+", "int:2", "punct:*", "int:3"}},
		{"function", "#define A(x) x + 1\nA(2)", []string{"int:2", "punct:+", "int:1"}},
		{"two params", "#define F(x, y) x * y\nF(2, 3)", []string{"int:2", "punct:*", "int:3"}},
		{"zero params", "#define F() 7\nF()", []string{"int:7"}},
		{"nested macro", "#define B 2\n#define A B\nA", []string{"int:2"}},
		{"argument expanded", "#define F(x) x\nF(F(1))", []string{"int:1"}},
		{"self reference", "#define A A\nA", []string{"ident:A"}},
		{"empty body", "#define E\nE 1", []string{"int:1"}},
		{"undef", "#define A 1\n#undef A\nA", []string{"ident:A"}},
		{"space makes object-like", "#define F (x)\nF(1)", []string{"punct:(", "ident:x", "punct:)", "punct:(", "int:1", "punct:)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := preprocessOK(t, tt.source, nil)
			checkTokens(t, res, tt.want...)
		})
	}

	if res := preprocessOK(t, "FOO\n", map[string]string{"FOO": "1"}); strings.Join(tokenStrings(res.Tokens), " ") != "int:1" {
		t.Errorf("options define = %v, want int:1", tokenStrings(res.Tokens))
	}
	if res := preprocessOK(t, "EMPTY x\n", map[string]string{"EMPTY": ""}); strings.Join(tokenStrings(res.Tokens), " ") != "ident:x" {
		t.Errorf("empty options define = %v, want ident:x", tokenStrings(res.Tokens))
	}

	wantError(t, preprocess("#define A 1\n#define A 2\nA", nil), "macro redefined")
	preprocessOK(t, "#define A 1\n#define A 1\nA", nil)
	wantError(t, preprocess("#define F(x, x) x\n", nil), "duplicate macro parameter")
	wantError(t, preprocess("#define F(x)\nF(1, 2)", nil), "expects 1 argument(s), found 2")
	wantError(t, preprocess("#define F()\nF(1)", nil), "takes no arguments")
	wantError(t, preprocess("#define F(x\n", nil), "expected ',' or ')'")
	wantError(t, preprocess("#define F(x)\nF(1", nil), "unterminated macro arguments")
}

func TestPreprocessPredefinedMacros(t *testing.T) {
	res := preprocessOK(t, "#version 300 es\n__VERSION__ GL_ES __LINE__\n", nil)
	got := strings.Join(tokenStrings(res.Tokens), " ")
	if !strings.HasPrefix(got, "int:300 int:1 int:2") {
		t.Errorf("predefined macros = %q, want 300 1 and line 2", got)
	}

	res = preprocessOK(t, "#version 450 core\n__LINE__\n", nil)
	if got := tokenStrings(res.Tokens); strings.Join(got, " ") != "int:2" {
		t.Errorf("__LINE__ = %v, want int:2", got)
	}

	// In a macro body __LINE__ is the invocation's line.
	res = preprocessOK(t, "#version 450 core\n#define L __LINE__\n\nL\n", nil)
	if got := tokenStrings(res.Tokens); strings.Join(got, " ") != "int:4" {
		t.Errorf("__LINE__ in a macro = %v, want int:4", got)
	}

	// GL_ES is undefined outside ES.
	res = preprocessOK(t, "#version 450 core\nGL_ES\n", nil)
	if got := tokenStrings(res.Tokens); strings.Join(got, " ") != "ident:GL_ES" {
		t.Errorf("GL_ES in core = %v, want ident:GL_ES", got)
	}
}

func TestPreprocessLineDirective(t *testing.T) {
	res := preprocessOK(t, "#version 450 core\n#line 100\n__LINE__\n#line 7 3\n__LINE__\n", nil)
	want := "int:100 int:7"
	if got := strings.Join(tokenStrings(res.Tokens), " "); got != want {
		t.Fatalf("__LINE__ after #line = %q, want %q", got, want)
	}
	wantError(t, preprocess("#line\n", nil), "expected an expression")
	wantError(t, preprocess("#line 1 2 3\n", nil), "unexpected token after #line")
}

func TestPreprocessConditionals(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{"true", "#if 1\na\n#endif", []string{"ident:a"}},
		{"false", "#if 0\na\n#endif", nil},
		{"else", "#if 0\na\n#else\nb\n#endif", []string{"ident:b"}},
		{"elif", "#if 0\na\n#elif 1\nb\n#else\nc\n#endif", []string{"ident:b"}},
		{"arithmetic", "#if 2 > 1 && !0\nyes\n#endif", []string{"ident:yes"}},
		{"macro", "#define X 3\n#if X == 3\nyes\n#endif", []string{"ident:yes"}},
		{"defined", "#define X 3\n#if defined(X) && !defined(Y)\nyes\n#endif", []string{"ident:yes"}},
		{"undefined identifier is zero", "#if UNDEFINED_IDENT\nno\n#else\nyes\n#endif", []string{"ident:yes"}},
		{"predefined", "#version 300 es\n#if GL_ES && __VERSION__ == 300\nyes\n#endif", []string{"ident:yes"}},
		{"nested skipped", "#if 0\n#if garbage\nno\n#endif\nalso no\n#endif\nyes", []string{"ident:yes"}},
		{"directive inside skipped", "#if 0\n#unknown\ntokens\n#endif\nyes", []string{"ident:yes"}},
		{"ifdef", "#define X\n#ifdef X\nyes\n#endif", []string{"ident:yes"}},
		{"ifndef", "#ifndef X\nyes\n#endif", []string{"ident:yes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := preprocessOK(t, tt.source, nil)
			checkTokens(t, res, tt.want...)
		})
	}

	wantError(t, preprocess("#if 1/0\na\n#endif", nil), "division by zero")
	wantError(t, preprocess("#if 1 +\na\n#endif", nil), "unexpected end")
	wantError(t, preprocess("#if\na\n#endif", nil), "expected an expression")
	wantError(t, preprocess("#if 1.5\na\n#endif", nil), "floating-point value")
	wantError(t, preprocess("#if 9223372036854775807 + 1\na\n#endif", nil), "integer overflow")
	wantError(t, preprocess("#if 1 << 64\na\n#endif", nil), "shift count out of range")
	wantError(t, preprocess("#if 1\na\n", nil), "unterminated conditional")
	wantError(t, preprocess("#endif\n", nil), "#endif without #if")
	wantError(t, preprocess("#else\n", nil), "#else without #if")
	wantError(t, preprocess("#elif 1\n", nil), "#elif without #if")
	wantError(t, preprocess("#if 1\n#else\n#else\n#endif", nil), "#else after #else")
	wantError(t, preprocess("#if 1\n#else\n#elif 1\n#endif", nil), "#elif after #else")
	wantError(t, preprocess("#if 1\na\n#endif extra\n", nil), "unexpected token after #endif")
	wantError(t, preprocess("#error boom\n", nil), "error directive")
	wantError(t, preprocess("#version 450 core\n#nothing\n", nil), "unknown directive")
	wantError(t, preprocess("#version 450 core\n#\n", nil), "expected a directive name")
	wantError(t, preprocess("#ifdef\n#endif\n", nil), "expected an identifier")
}

func TestPreprocessTokenBudget(t *testing.T) {
	var b strings.Builder
	b.WriteString("#version 450 core\n")
	for i := 0; i < 20; i++ {
		next := i + 1
		if i == 19 {
			b.WriteString("#define A19 1 1\n")
			continue
		}
		b.WriteString("#define A" + strconv.Itoa(i) + " A" + strconv.Itoa(next) + " A" + strconv.Itoa(next) + "\n")
	}
	b.WriteString("A0\n")
	res := preprocess(b.String(), nil)
	wantError(t, res, "token limit exceeded")
}

func TestPreprocessConditionalNestingLimit(t *testing.T) {
	var b strings.Builder
	for i := 0; i <= maxPPNesting; i++ {
		b.WriteString("#if 1\n")
	}
	b.WriteString("x\n")
	for i := 0; i <= maxPPNesting; i++ {
		b.WriteString("#endif\n")
	}
	res := preprocess(b.String(), nil)
	wantError(t, res, "nest deeper than")
}

func TestPreprocessConditionOperators(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{"1 | 2", true},
		{"(1 | 2) == 3", true},
		{"(1 & 3) == 1", true},
		{"(3 ^ 1) == 2", true},
		{"(1 << 3) == 8", true},
		{"(16 >> 2) == 4", true},
		{"(1 < 2) && (2 > 1) && (1 <= 1) && (1 >= 1)", true},
		{"(1 != 2) == 1", true},
		{"(7 % 4) == 3", true},
		{"(+1) == 1", true},
		{"(-1) == -1", true},
		{"(~0) == -1", true},
		{"!0", true},
		{"0 && 1", false},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			res := preprocessOK(t, "#if "+tt.expr+"\nyes\n#else\nno\n#endif\n", nil)
			want := "ident:no"
			if tt.want {
				want = "ident:yes"
			}
			checkTokens(t, res, want)
		})
	}

	wantError(t, preprocess("#if 9223372036854775807 * 2\n#endif\n", nil), "integer overflow")
	wantError(t, preprocess("#if 1 % 0\n#endif\n", nil), "division by zero")
	wantError(t, preprocess("#if (1\n#endif\n", nil), "expected ')'")
	wantError(t, preprocess("#if defined\n#endif\n", nil), "expected a macro name after defined")
	wantError(t, preprocess("#if 1 2\n#endif\n", nil), "unexpected token")
}

func TestPreprocessDefinedPredefined(t *testing.T) {
	for _, source := range []string{
		"#version 450 core\n#if defined(__LINE__)\nyes\n#endif\n",
		"#version 450 core\n#if defined(__VERSION__)\nyes\n#endif\n",
		"#version 300 es\n#if defined(GL_ES)\nyes\n#endif\n",
	} {
		checkTokens(t, preprocessOK(t, source, nil), "ident:yes")
	}
	res := preprocessOK(t, "#version 450 core\n#if defined(GL_ES)\nyes\n#else\nno\n#endif\n", nil)
	checkTokens(t, res, "ident:no")
}

func TestPreprocessInvalidUTF8(t *testing.T) {
	res := preprocess("a "+string([]byte{0xff})+" b", nil)
	wantError(t, res, "unexpected character")
}

func TestPreprocessUndefAndRedefinitionErrors(t *testing.T) {
	wantError(t, preprocess("#undef\n", nil), "expected a macro name")
	wantError(t, preprocess("#undef A B\n", nil), "expected a macro name")
	wantError(t, preprocess("#define F(x) x\n#define F(x) y\n", nil), "macro redefined")
	wantError(t, preprocess("#define F(x) x\n#define F(y) y\n", nil), "macro redefined")
	wantError(t, preprocess("#define F(x) x\n#define F x\n", nil), "macro redefined")
}

func TestPreprocessDefineValueErrors(t *testing.T) {
	res := preprocess("A\n", map[string]string{"A": "@"})
	wantError(t, res, "in define A")
}

func TestPreprocessDirectiveWithoutNameWhileSkipping(t *testing.T) {
	res := preprocessOK(t, "#if 0\n#\n#endif\nyes\n", nil)
	checkTokens(t, res, "ident:yes")
}
