package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestTokenKindNames(t *testing.T) {
	if len(tokenKindNames) != int(tokStruct)+1 {
		t.Fatalf("%d names for %d kinds", len(tokenKindNames), int(tokStruct)+1)
	}
	seen := make(map[string]bool, len(tokenKindNames))
	for i, name := range tokenKindNames {
		if name == "" || seen[name] {
			t.Errorf("name %d = %q, want a unique non-empty name", i, name)
		}
		seen[name] = true
	}
}

func TestClassifyKeywords(t *testing.T) {
	want := map[string]tokenKind{
		"layout": tokLayout, "in": tokIn, "out": tokOut, "inout": tokInOut,
		"uniform": tokUniform, "buffer": tokBuffer, "const": tokConst, "shared": tokShared,
		"restrict": tokRestrict, "readonly": tokReadOnly, "writeonly": tokWriteOnly,
		"invariant": tokInvariant, "flat": tokFlat, "smooth": tokSmooth,
		"noperspective": tokNoPerspective, "centroid": tokCentroid, "sample": tokSample,
		"precision": tokPrecision, "highp": tokHighP, "mediump": tokMediumP, "lowp": tokLowP,
		"continue": tokContinue, "break": tokBreak, "return": tokReturn, "discard": tokDiscard,
		"if": tokIf, "else": tokElse, "switch": tokSwitch, "case": tokCase, "default": tokDefault,
		"while": tokWhile, "do": tokDo, "for": tokFor, "void": tokVoid, "struct": tokStruct,
	}
	if len(keywords) != len(want) {
		t.Errorf("keywords has %d entries, the test lists %d", len(keywords), len(want))
	}
	for word, kind := range want {
		if got := classify(ppToken{kind: ppIdent, text: word}); got.kind != kind {
			t.Errorf("classify(%q) = %v, want %v", word, got.kind, kind)
		}
	}
	if got := classify(ppToken{kind: ppIdent, text: "true"}); got.kind != tokBool || !got.boolValue {
		t.Errorf("true = %v %v, want a true bool", got.kind, got.boolValue)
	}
	if got := classify(ppToken{kind: ppIdent, text: "false"}); got.kind != tokBool || got.boolValue {
		t.Errorf("false = %v %v, want a false bool", got.kind, got.boolValue)
	}
}

func TestClassifyLiterals(t *testing.T) {
	span := ir.Span{Start: 4, End: 6}
	tok := classify(ppToken{kind: ppInt, intValue: 42, intSigned: true, intWidth: 32, span: span, line: 3})
	if tok.kind != tokInt || tok.intValue != 42 || !tok.intSigned || tok.span != span || tok.line != 3 {
		t.Errorf("int token = %+v", tok)
	}

	tok = classify(ppToken{kind: ppFloat, floatValue: 1.5, floatWidth: 32})
	if tok.kind != tokFloat || tok.floatValue != 1.5 {
		t.Errorf("float token = %+v", tok)
	}

	tok = classify(ppToken{kind: ppPunct, text: "+="})
	if tok.kind != tokPunct || !tok.isPunct("+=") || tok.isPunct("+") {
		t.Errorf("punct token = %+v", tok)
	}
}

func TestLexEndToEnd(t *testing.T) {
	source := "#version 450 core\nlayout(location = 0) in vec3 pos;\nvoid main() { gl_Position = vec4(pos, 1.0); }\n"
	res := preprocessOK(t, source, nil)
	tokens := lex(res.Tokens)

	find := func(match func(glslToken) bool) int {
		for i, tok := range tokens {
			if match(tok) {
				return i
			}
		}
		return -1
	}
	if i := find(func(t glslToken) bool { return t.kind == tokLayout }); i < 0 {
		t.Fatal("layout keyword not classified")
	}
	vec3 := find(func(t glslToken) bool { return t.kind == tokTypeName && t.typeName != nil && t.typeName.Inner != nil })
	if vec3 < 0 {
		t.Fatal("vec3 not classified as a type name")
	}
	if got := tokens[vec3].typeName.Inner; got != (ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}) {
		t.Errorf("vec3 = %+v", got)
	}
	if find(func(t glslToken) bool { return t.kind == tokIdent && t.text == "main" }) < 0 {
		t.Error("main not classified as an identifier")
	}
	if find(func(t glslToken) bool { return t.kind == tokIdent && t.text == "gl_Position" }) < 0 {
		t.Error("gl_Position not classified as an identifier")
	}
	if find(func(t glslToken) bool { return t.kind == tokFloat && t.floatValue == 1 }) < 0 {
		t.Error("1.0 not classified as a float")
	}
	if find(func(t glslToken) bool { return t.isPunct("=") }) < 0 {
		t.Error("= not classified as punctuation")
	}
}
