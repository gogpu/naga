package front

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// frontError is one diagnostic of the frontend: Rust naga's Error (error.rs:144-153),
// a message with the source span it refers to. The public API turns the list of
// them into one *ParseError whose first entry is the message and whose rest go
// into the notes (R1).
type frontError struct {
	Message string
	Span    ir.Span
}

func (e *frontError) Error() string { return e.Message }

// expectedKind is the kind of one entry of an "expected" list, Rust naga's
// ExpectedToken (error.rs:35-50).
type expectedKind uint8

const (
	expToken expectedKind = iota
	expTypeName
	expIdentifier
	expIntLiteral
	expFloatLiteral
	expBoolLiteral
	expEOF
)

// expected is one entry of an "expected" list.
type expected struct {
	Kind expectedKind
	Text string // the Rust name of the expected token, for expToken
}

// String renders the entry as Rust's ExpectedToken Display does (error.rs:56-68).
func (e expected) String() string {
	switch e.Kind {
	case expToken:
		return e.Text
	case expTypeName:
		return "a type"
	case expIdentifier:
		return "identifier"
	case expIntLiteral:
		return "integer literal"
	case expFloatLiteral:
		return "float literal"
	case expBoolLiteral:
		return "bool literal"
	case expEOF:
		return "end of file"
	default:
		return "unknown expected token"
	}
}

// wantPunct expects the punctuation token written s.
func wantPunct(s string) expected {
	name, ok := rustPunctNames[s]
	if !ok {
		// A punctuation token the table does not name is a bug in the table;
		// the spelling still identifies the token in the message.
		name = s
	}
	return expected{Kind: expToken, Text: name}
}

// wantKeyword expects the keyword of kind k.
func wantKeyword(k tokenKind) expected {
	return expected{Kind: expToken, Text: rustKeywordNames[k]}
}

// Commonly expected items.
var (
	wantIdentifier = expected{Kind: expIdentifier}
	wantTypeName   = expected{Kind: expTypeName}
	wantEOF        = expected{Kind: expEOF}
)

// joinExpected renders an expected list as Rust's join_with_comma does
// (error.rs:20-31): "a, b or c".
func joinExpected(list []expected) string {
	var b strings.Builder
	for i, e := range list {
		b.WriteString(e.String())
		switch i {
		case len(list) - 1:
		case len(list) - 2:
			b.WriteString(" or ")
		default:
			b.WriteString(", ")
		}
	}
	return b.String()
}

// invalidToken is Rust's ErrorKind::InvalidToken: the token found and what was
// expected instead (error.rs:86).
func invalidToken(found glslToken, want ...expected) *frontError {
	return &frontError{
		Message: "Expected " + joinExpected(want) + ", found " + tokenDebug(found),
		Span:    found.span,
	}
}

// errEOF is Rust's ErrorKind::EndOfFile (error.rs:74-76).
func errEOF(span ir.Span) *frontError {
	return &frontError{Message: "Unexpected end of file", Span: span}
}

// errSemantic is Rust's ErrorKind::SemanticError, whose Display is the message.
func errSemantic(span ir.Span, format string, args ...any) *frontError {
	return &frontError{Message: fmt.Sprintf(format, args...), Span: span}
}

// errMessage is a semantic error with a message that is already formatted; it
// keeps a variable message away from a printf-style call.
func errMessage(span ir.Span, msg string) *frontError {
	return &frontError{Message: msg, Span: span}
}

// errUnknownVariable is Rust's ErrorKind::UnknownVariable (error.rs:94-96).
func errUnknownVariable(name string, span ir.Span) *frontError {
	return &frontError{Message: "Unknown variable: " + name, Span: span}
}

// errUnknownType is Rust's ErrorKind::UnknownType (error.rs:98-99).
func errUnknownType(name string, span ir.Span) *frontError {
	return &frontError{Message: "Unknown type: " + name, Span: span}
}

// errVariableAlreadyDeclared is Rust's ErrorKind::VariableAlreadyDeclared.
func errVariableAlreadyDeclared(name string, span ir.Span) *frontError {
	return &frontError{Message: "Variable already declared: " + name, Span: span}
}

// errRedefinition refuses a second module-scope declaration of a name, with
// glslang's wording. The span is the second declaration's: the first one is
// what the name already denotes (module_names.go).
func errRedefinition(name string, span ir.Span) *frontError {
	return errSemantic(span, "'%s' : redefinition", name)
}

// errInternal is Rust's ErrorKind::InternalError: a bug in the frontend,
// surfaced as an error rather than a panic (error.rs:132-135).
func errInternal(msg string, span ir.Span) *frontError {
	return &frontError{Message: "Internal error: " + msg, Span: span}
}

// rustPunctNames maps a punctuation spelling to the name of its Rust naga
// TokenValue variant (token.rs:74-123), which is how a wrong token is named in
// an "expected" message.
var rustPunctNames = map[string]string{
	"=":   "Assign",
	"+=":  "AddAssign",
	"-=":  "SubAssign",
	"*=":  "MulAssign",
	"/=":  "DivAssign",
	"%=":  "ModAssign",
	"<<=": "LeftShiftAssign",
	">>=": "RightShiftAssign",
	"&=":  "AndAssign",
	"^=":  "XorAssign",
	"|=":  "OrAssign",

	"++": "Increment",
	"--": "Decrement",

	"||": "LogicalOr",
	"&&": "LogicalAnd",
	"^^": "LogicalXor",

	"<=": "LessEqual",
	">=": "GreaterEqual",
	"==": "Equal",
	"!=": "NotEqual",

	"<<": "LeftShift",
	">>": "RightShift",

	"{": "LeftBrace",
	"}": "RightBrace",
	"(": "LeftParen",
	")": "RightParen",
	"[": "LeftBracket",
	"]": "RightBracket",
	"<": "LeftAngle",
	">": "RightAngle",

	",": "Comma",
	";": "Semicolon",
	":": "Colon",
	".": "Dot",
	"!": "Bang",
	"-": "Dash",
	"~": "Tilde",
	"+": "Plus",
	"*": "Star",
	"/": "Slash",
	"%": "Percent",
	"|": "VerticalBar",
	"^": "Caret",
	"&": "Ampersand",
	"?": "Question",
}

// rustKeywordNames maps a keyword kind to the name of its Rust naga TokenValue
// variant (token.rs:33-73), which is how a wrong token is named in an
// "expected" message.
var rustKeywordNames = map[tokenKind]string{
	tokLayout:        "Layout",
	tokIn:            "In",
	tokOut:           "Out",
	tokInOut:         "InOut",
	tokUniform:       "Uniform",
	tokBuffer:        "Buffer",
	tokConst:         "Const",
	tokShared:        "Shared",
	tokRestrict:      "Restrict",
	tokReadOnly:      "MemoryQualifier(StorageAccess(LOAD))",
	tokWriteOnly:     "MemoryQualifier(StorageAccess(STORE))",
	tokInvariant:     "Invariant",
	tokFlat:          "Interpolation(Flat)",
	tokSmooth:        "Interpolation(Perspective)",
	tokNoPerspective: "Interpolation(Linear)",
	tokCentroid:      "Sampling(Centroid)",
	tokSample:        "Sampling(Sample)",
	tokPrecision:     "Precision",
	tokHighP:         "PrecisionQualifier(High)",
	tokMediumP:       "PrecisionQualifier(Medium)",
	tokLowP:          "PrecisionQualifier(Low)",
	tokContinue:      "Continue",
	tokBreak:         "Break",
	tokReturn:        "Return",
	tokDiscard:       "Discard",
	tokIf:            "If",
	tokElse:          "Else",
	tokSwitch:        "Switch",
	tokCase:          "Case",
	tokDefault:       "Default",
	tokWhile:         "While",
	tokDo:            "Do",
	tokFor:           "For",
	tokVoid:          "Void",
	tokStruct:        "Struct",
}

// tokenDebug names one token as Rust naga's `{found_token:?}` does: the Rust
// TokenValue variant name, with its value for literals and identifiers
// (token.rs:26-124).
func tokenDebug(t glslToken) string {
	switch t.kind {
	case tokIdent:
		return "Identifier(" + strconv.Quote(t.text) + ")"
	case tokInt:
		signed := "false"
		if t.intSigned {
			signed = "true"
		}
		return fmt.Sprintf("IntConstant(Integer { value: %d, signed: %s, width: %d })",
			t.intValue, signed, t.intWidth)
	case tokFloat:
		return fmt.Sprintf("FloatConstant(Float { value: %s, width: %d })",
			rustFloat(t.floatValue, t.floatWidth), t.floatWidth)
	case tokBool:
		return "BoolConstant(" + strconv.FormatBool(t.boolValue) + ")"
	case tokPunct:
		if name, ok := rustPunctNames[t.text]; ok {
			return name
		}
		return strconv.Quote(t.text)
	case tokTypeName:
		return "TypeName(Type { name: None, inner: " + typeInnerDebug(t.typeName) + " })"
	default:
		if name, ok := rustKeywordNames[t.kind]; ok {
			return name
		}
		return t.kind.String()
	}
}

// rustFloat renders a float literal as Rust's Debug of the f32 does: an
// exponent for a magnitude below 1e-4 or at least 1e16 — written without a plus
// sign and without leading zeros — and the shortest decimal with a trailing .0
// for an integral value otherwise.
func rustFloat(v float64, width int32) string {
	bits := 32
	if width == 64 {
		bits = 64
	}
	a := math.Abs(v)
	if a != 0 && (a < 1e-4 || a >= 1e16) {
		return rustExponent(strconv.FormatFloat(v, 'e', -1, bits))
	}
	s := strconv.FormatFloat(v, 'f', -1, bits)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}

// rustExponent rewrites Go's exponent form (1e+20, 1e-07) to Rust's (1e20,
// 1e-7): no plus sign and no leading zeros in the exponent.
func rustExponent(s string) string {
	i := strings.IndexByte(s, 'e')
	if i < 0 {
		return s
	}
	mantissa, exp := s[:i], s[i+1:]
	sign := ""
	if expire := len(exp) > 0; expire {
		switch exp[0] {
		case '+':
			exp = exp[1:]
		case '-':
			sign, exp = "-", exp[1:]
		}
	}
	exp = strings.TrimLeft(exp, "0")
	if exp == "" {
		exp = "0"
	}
	return mantissa + "e" + sign + exp
}

// vectorSizeNames maps a vector or matrix size to the Rust VectorSize name.
var vectorSizeNames = [...]string{
	ir.Vec2: "Bi",
	ir.Vec3: "Tri",
	ir.Vec4: "Quad",
}

// imageDimensionNames maps an image dimension to the Rust ImageDimension name.
var imageDimensionNames = [...]string{
	ir.Dim1D:   "D1",
	ir.Dim2D:   "D2",
	ir.Dim3D:   "D3",
	ir.DimCube: "Cube",
}

// scalarKindNames maps a scalar kind to the Rust ScalarKind name.
var scalarKindNames = [...]string{
	ir.ScalarSint:  "Sint",
	ir.ScalarUint:  "Uint",
	ir.ScalarFloat: "Float",
	ir.ScalarBool:  "Bool",
}

// scalarDebug renders a scalar type as Rust's Debug of Scalar does.
func scalarDebug(s ir.ScalarType) string {
	return fmt.Sprintf("Scalar { kind: %s, width: %d }", scalarKindNames[s.Kind], s.Width)
}

// typeInnerDebug renders a builtin type name as Rust's Debug of
// TypeInner does, which is what a TypeName token shows in an error message.
func typeInnerDebug(tn *typeName) string {
	if tn.Combined {
		// Rust naga has no combined sampler type (the #5994 difference), so
		// this shape is ours: an image and a sampler in one declaration. The
		// image half follows Rust's Shape of a sampled or depth image.
		dim, arrayed := imageDimensionNames[tn.Dim], "false"
		if tn.Arrayed {
			arrayed = "true"
		}
		class := fmt.Sprintf("Sampled { kind: %s, multi: %t }", scalarKindNames[tn.Kind], tn.Multi)
		if tn.Comparison {
			class = "Depth"
		}
		return fmt.Sprintf("Image { dim: %s, arrayed: %s, class: %s } and Sampler { comparison: %t }",
			dim, arrayed, class, tn.Comparison)
	}
	switch inner := tn.Inner.(type) {
	case ir.ScalarType:
		return "Scalar(" + scalarDebug(inner) + ")"
	case ir.VectorType:
		return fmt.Sprintf("Vector { size: %s, scalar: %s }",
			vectorSizeNames[inner.Size], scalarDebug(inner.Scalar))
	case ir.MatrixType:
		return fmt.Sprintf("Matrix { columns: %s, rows: %s, scalar: %s }",
			vectorSizeNames[inner.Columns], vectorSizeNames[inner.Rows], scalarDebug(inner.Scalar))
	case ir.ImageType:
		return fmt.Sprintf("Image { dim: %s, arrayed: %t, class: %s }",
			imageDimensionNames[inner.Dim], inner.Arrayed, imageClassDebug(inner))
	case ir.SamplerType:
		return fmt.Sprintf("Sampler { comparison: %t }", inner.Comparison)
	default:
		return fmt.Sprintf("TypeInner(%v)", tn.Inner)
	}
}

// imageClassDebug renders an image class as Rust's Debug of ImageClass does:
// the sampled class carries the value kind and the multisampled flag, where
// TypeInner::Image has no sibling of its own for it.
func imageClassDebug(image ir.ImageType) string {
	switch image.Class {
	case ir.ImageClassStorage:
		return fmt.Sprintf("Storage { format: %s, access: %s }",
			storageFormatDebug(image.StorageFormat), storageAccessDebug(image.StorageAccess))
	case ir.ImageClassDepth:
		return "Depth"
	case ir.ImageClassSampled:
		return fmt.Sprintf("Sampled { kind: %s, multi: %t }",
			scalarKindNames[image.SampledKind], image.Multisampled)
	default:
		return "External"
	}
}

// storageFormatNames are the Rust names of the storage formats the GLSL type
// names can carry. The parser only meets the placeholder an image type carries
// until its layout format applies; the rest keep Rust's Debug spelling.
var storageFormatNames = map[ir.StorageFormat]string{
	ir.StorageFormatR8Uint: "R8Uint",
}

// storageFormatDebug renders a storage format as Rust's Debug does.
func storageFormatDebug(f ir.StorageFormat) string {
	if name, ok := storageFormatNames[f]; ok {
		return name
	}
	return fmt.Sprintf("StorageFormat(%d)", uint8(f))
}

// storageAccessDebug renders a storage access as Rust's bitflags 2.x Debug
// does: the flags wrapped in the type's name, as in StorageAccess(LOAD | STORE).
func storageAccessDebug(a ir.StorageAccess) string {
	switch a {
	case ir.StorageAccessLoad:
		return "StorageAccess(LOAD)"
	case ir.StorageAccessStore:
		return "StorageAccess(STORE)"
	case ir.StorageAccessLoad | ir.StorageAccessStore:
		return "StorageAccess(LOAD | STORE)"
	default:
		return "StorageAccess(0x0)"
	}
}
