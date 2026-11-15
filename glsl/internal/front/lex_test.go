package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// tn is a plain (non-combined) type name.
func tn(inner ir.TypeInner) typeName { return typeName{Inner: inner} }

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

func TestParseTypeNames(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	f64 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	i32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	boolean := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}

	tests := []struct {
		name string
		want typeName
	}{
		{"bool", tn(boolean)},
		{"int", tn(i32)},
		{"uint", tn(u32)},
		{"float", tn(f32)},
		{"double", tn(f64)},
		{"float16_t", tn(f16)},
		{"sampler", tn(ir.SamplerType{Comparison: false})},
		{"samplerShadow", tn(ir.SamplerType{Comparison: true})},

		{"vec3", tn(ir.VectorType{Size: ir.Vec3, Scalar: f32})},
		{"bvec2", tn(ir.VectorType{Size: ir.Vec2, Scalar: boolean})},
		{"ivec4", tn(ir.VectorType{Size: ir.Vec4, Scalar: i32})},
		{"uvec3", tn(ir.VectorType{Size: ir.Vec3, Scalar: u32})},
		{"dvec2", tn(ir.VectorType{Size: ir.Vec2, Scalar: f64})},
		{"f16vec2", tn(ir.VectorType{Size: ir.Vec2, Scalar: f16})},

		{"mat3", tn(ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32})},
		{"mat2x4", tn(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec4, Scalar: f32})},
		{"dmat2", tn(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f64})},
		{"f16mat2x3", tn(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f16})},

		{"texture2D", tn(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat})},
		{"texture2DMSArray", tn(ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, Multisampled: true, SampledKind: ir.ScalarFloat})},
		{"itexture2D", tn(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint})},
		{"utextureCubeArray", tn(ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarUint})},
		{"image2D", tn(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage, SampledKind: ir.ScalarFloat, StorageFormat: ir.StorageFormatR8Uint, StorageAccess: ir.StorageAccessLoad | ir.StorageAccessStore})},
		{"image1DArray", tn(ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassStorage, SampledKind: ir.ScalarFloat, StorageFormat: ir.StorageFormatR8Uint, StorageAccess: ir.StorageAccessLoad | ir.StorageAccessStore})},

		{"sampler2D", typeName{Combined: true, Kind: ir.ScalarFloat, Dim: ir.Dim2D}},
		{"usampler2DArray", typeName{Combined: true, Kind: ir.ScalarUint, Dim: ir.Dim2D, Arrayed: true}},
		{"isampler3D", typeName{Combined: true, Kind: ir.ScalarSint, Dim: ir.Dim3D}},
		{"sampler1DArray", typeName{Combined: true, Kind: ir.ScalarFloat, Dim: ir.Dim1D, Arrayed: true}},
		{"sampler2DMSArray", typeName{Combined: true, Kind: ir.ScalarFloat, Dim: ir.Dim2D, Arrayed: true, Multi: true}},
		{"sampler2DShadow", typeName{Combined: true, Comparison: true, Kind: ir.ScalarFloat, Dim: ir.Dim2D}},
		{"samplerCubeArrayShadow", typeName{Combined: true, Comparison: true, Kind: ir.ScalarFloat, Dim: ir.DimCube, Arrayed: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTypeName(tt.name)
			if !ok {
				t.Fatalf("parseTypeName(%q) = not a type", tt.name)
			}
			if got != tt.want {
				t.Fatalf("parseTypeName(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
			if tok := classify(ppToken{kind: ppIdent, text: tt.name}); tok.kind != tokTypeName || tok.typeName == nil {
				t.Errorf("classify(%q) = %v", tt.name, tok.kind)
			}
		})
	}
}

func TestParseTypeNamesReject(t *testing.T) {
	for _, name := range []string{
		"void", "struct", "float32", "vec5", "vec", "mat5", "mat2x5", "dvec", "f16vec",
		"imageCube", "image2DMS", "textureBuffer", "samplerBuffer", "sampler2DRect",
		"isampler2DShadow", "sampler3DShadow", "sampler2DMSShadow", "Sampler2D", "",
	} {
		if tn, ok := parseTypeName(name); ok {
			t.Errorf("parseTypeName(%q) = %+v, want not a type", name, tn)
		}
		if tok := classify(ppToken{kind: ppIdent, text: name}); tok.kind != tokIdent && name != "void" && name != "struct" {
			t.Errorf("classify(%q) = %v, want an identifier", name, tok.kind)
		}
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
