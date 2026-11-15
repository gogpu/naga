package front

import "testing"

// These tests pin the error vocabulary to Rust naga's: a message names the
// expected tokens and the found one exactly as Rust's Display/Debug pair does
// (error.rs:20-68, token.rs:26-124), because the texts are behavior. The one
// shape Rust does not have is the combined sampler type, which the #5994
// difference makes a type here; its text is ours and is pinned below too.

func TestExpectedStrings(t *testing.T) {
	tests := []struct {
		want expected
		text string
	}{
		{wantPunct(";"), "Semicolon"},
		{wantKeyword(tokUniform), "Uniform"},
		{wantIdentifier, "identifier"},
		{wantTypeName, "a type"},
		{expected{Kind: expIntLiteral}, "integer literal"},
		{expected{Kind: expFloatLiteral}, "float literal"},
		{expected{Kind: expBoolLiteral}, "bool literal"},
		{wantEOF, "end of file"},
	}
	for _, tt := range tests {
		if got := tt.want.String(); got != tt.text {
			t.Errorf("%+v = %q, want %q", tt.want, got, tt.text)
		}
	}
}

func TestJoinExpected(t *testing.T) {
	tests := []struct {
		list []expected
		want string
	}{
		{[]expected{wantPunct(";")}, "Semicolon"},
		{[]expected{wantIdentifier, wantPunct(";")}, "identifier or Semicolon"},
		{[]expected{wantPunct(","), wantPunct(";"), wantEOF}, "Comma, Semicolon or end of file"},
	}
	for _, tt := range tests {
		if got := joinExpected(tt.list); got != tt.want {
			t.Errorf("joinExpected(%v) = %q, want %q", tt.list, got, tt.want)
		}
	}
}

func TestInvalidTokenMessage(t *testing.T) {
	found := glslToken{kind: tokPunct, text: ")"}
	err := invalidToken(found, wantPunct(","), wantPunct(";"))
	if want := "Expected Comma or Semicolon, found RightParen"; err.Message != want {
		t.Errorf("message = %q, want %q", err.Message, want)
	}
}

// TestTokenDebug pins the "found" representation of every token shape: Rust's
// Debug of TokenValue, including the value of a literal and the inner type of a
// type name.
func TestTokenDebug(t *testing.T) {
	vec3, ok := parseTypeName("vec3")
	if !ok {
		t.Fatal("vec3 is not a type name")
	}
	image, ok := parseTypeName("image2D")
	if !ok {
		t.Fatal("image2D is not a type name")
	}
	combined, ok := parseTypeName("sampler2D")
	if !ok {
		t.Fatal("sampler2D is not a combined sampler")
	}
	scalar, ok := parseTypeName("float")
	if !ok {
		t.Fatal("float is not a type name")
	}
	multisampled, ok := parseTypeName("texture2DMS")
	if !ok {
		t.Fatal("texture2DMS is not a type name")
	}

	tests := []struct {
		name string
		tok  glslToken
		want string
	}{
		{"identifier", glslToken{kind: tokIdent, text: "main"}, `Identifier("main")`},
		{"signed int", glslToken{kind: tokInt, intValue: 42, intSigned: true, intWidth: 32},
			"IntConstant(Integer { value: 42, signed: true, width: 32 })"},
		{"unsigned int", glslToken{kind: tokInt, intValue: 7, intSigned: false, intWidth: 32},
			"IntConstant(Integer { value: 7, signed: false, width: 32 })"},
		{"int with a point", glslToken{kind: tokFloat, floatValue: 1.5, floatWidth: 32},
			"FloatConstant(Float { value: 1.5, width: 32 })"},
		{"integral float", glslToken{kind: tokFloat, floatValue: 1, floatWidth: 32},
			"FloatConstant(Float { value: 1.0, width: 32 })"},
		// Rust's f32 Debug: an exponent for |x| < 1e-4 or >= 1e16, with no
		// sign or zero padding in it, and the shortest decimal otherwise.
		{"big exponent", glslToken{kind: tokFloat, floatValue: 1e20, floatWidth: 32},
			"FloatConstant(Float { value: 1e20, width: 32 })"},
		{"small exponent", glslToken{kind: tokFloat, floatValue: 1e-7, floatWidth: 32},
			"FloatConstant(Float { value: 1e-7, width: 32 })"},
		{"exponent threshold", glslToken{kind: tokFloat, floatValue: 1e16, floatWidth: 32},
			"FloatConstant(Float { value: 1e16, width: 32 })"},
		{"max float", glslToken{kind: tokFloat, floatValue: 3.4e38, floatWidth: 32},
			"FloatConstant(Float { value: 3.4e38, width: 32 })"},
		{"large decimal", glslToken{kind: tokFloat, floatValue: 123456789, floatWidth: 32},
			"FloatConstant(Float { value: 123456790.0, width: 32 })"},
		{"zero", glslToken{kind: tokFloat, floatValue: 0, floatWidth: 32},
			"FloatConstant(Float { value: 0.0, width: 32 })"},
		{"negative", glslToken{kind: tokFloat, floatValue: -1.5, floatWidth: 32},
			"FloatConstant(Float { value: -1.5, width: 32 })"},
		{"bool", glslToken{kind: tokBool, boolValue: true}, "BoolConstant(true)"},
		{"punctuation", glslToken{kind: tokPunct, text: ">>="}, "RightShiftAssign"},
		{"keyword", glslToken{kind: tokUniform}, "Uniform"},
		{"interpolation", glslToken{kind: tokSmooth}, "Interpolation(Perspective)"},
		{"noperspective", glslToken{kind: tokNoPerspective}, "Interpolation(Linear)"},
		{"precision qualifier", glslToken{kind: tokHighP}, "PrecisionQualifier(High)"},
		{"memory qualifier", glslToken{kind: tokReadOnly}, "MemoryQualifier(StorageAccess(LOAD))"},
		{"scalar type", glslToken{kind: tokTypeName, typeName: &scalar},
			"TypeName(Type { name: None, inner: Scalar(Scalar { kind: Float, width: 4 }) })"},
		{"vector type", glslToken{kind: tokTypeName, typeName: &vec3},
			"TypeName(Type { name: None, inner: Vector { size: Tri, scalar: Scalar { kind: Float, width: 4 } } })"},
		{"storage image type", glslToken{kind: tokTypeName, typeName: &image},
			"TypeName(Type { name: None, inner: Image { dim: D2, arrayed: false, class: Storage { format: R8Uint, access: StorageAccess(LOAD | STORE) } } })"},
		{"multisampled image type", glslToken{kind: tokTypeName, typeName: &multisampled},
			"TypeName(Type { name: None, inner: Image { dim: D2, arrayed: false, class: Sampled { kind: Float, multi: true } } })"},
		{"combined sampler type", glslToken{kind: tokTypeName, typeName: &combined},
			"TypeName(Type { name: None, inner: Image { dim: D2, arrayed: false, class: Sampled { kind: Float, multi: false } } and Sampler { comparison: false } })"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenDebug(tt.tok); got != tt.want {
				t.Errorf("tokenDebug = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRustNamesComplete checks that every punctuation the lexer can produce and
// every keyword has a Rust name: a missing entry would put an empty string into
// an error message.
func TestRustNamesComplete(t *testing.T) {
	all := append(append([]string{}, punctuations...), ".")
	for _, p := range all {
		if _, ok := rustPunctNames[p]; !ok {
			t.Errorf("punctuation %q has no Rust name", p)
		}
	}
	for word, kind := range keywords {
		if _, ok := rustKeywordNames[kind]; !ok {
			t.Errorf("keyword %q (%v) has no Rust name", word, kind)
		}
	}
	// The reverse, so a stale entry cannot hide a removed keyword.
	for kind := range rustKeywordNames {
		found := false
		for _, k := range keywords {
			if k == kind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Rust name %q belongs to no keyword kind", rustKeywordNames[kind])
		}
	}
}
