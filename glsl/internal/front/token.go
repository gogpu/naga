package front

import "strconv"

// tokenKind is the kind of a classified GLSL token: a literal, punctuation, a
// keyword, or a type name.
type tokenKind uint8

const (
	tokIdent tokenKind = iota
	tokInt
	tokFloat
	tokBool
	tokPunct
	tokTypeName

	tokLayout
	tokIn
	tokOut
	tokInOut
	tokUniform
	tokBuffer
	tokConst
	tokShared
	tokRestrict
	tokReadOnly
	tokWriteOnly
	tokInvariant
	tokFlat
	tokSmooth
	tokNoPerspective
	tokCentroid
	tokSample
	tokPrecision
	tokHighP
	tokMediumP
	tokLowP
	tokContinue
	tokBreak
	tokReturn
	tokDiscard
	tokIf
	tokElse
	tokSwitch
	tokCase
	tokDefault
	tokWhile
	tokDo
	tokFor
	tokVoid
	tokStruct
)

// tokenKindNames are the names tokenKind.String reports, indexed by value. The
// order must match the constants above; TestTokenKindNames checks it.
var tokenKindNames = [...]string{
	"identifier", "int", "float", "bool", "punct", "type name",
	"layout", "in", "out", "inout", "uniform", "buffer", "const", "shared",
	"restrict", "readonly", "writeonly", "invariant",
	"flat", "smooth", "noperspective", "centroid", "sample",
	"precision", "highp", "mediump", "lowp",
	"continue", "break", "return", "discard",
	"if", "else", "switch", "case", "default",
	"while", "do", "for", "void", "struct",
}

// String returns the kind's name for diagnostics.
func (k tokenKind) String() string {
	if int(k) < len(tokenKindNames) {
		return tokenKindNames[k]
	}
	return "tokenKind(" + strconv.Itoa(int(k)) + ")"
}

// keywords maps the GLSL keywords that are not type names to their kinds.
// true and false carry a value and are classified separately.
var keywords = map[string]tokenKind{
	"layout":        tokLayout,
	"in":            tokIn,
	"out":           tokOut,
	"inout":         tokInOut,
	"uniform":       tokUniform,
	"buffer":        tokBuffer,
	"const":         tokConst,
	"shared":        tokShared,
	"restrict":      tokRestrict,
	"readonly":      tokReadOnly,
	"writeonly":     tokWriteOnly,
	"invariant":     tokInvariant,
	"flat":          tokFlat,
	"smooth":        tokSmooth,
	"noperspective": tokNoPerspective,
	"centroid":      tokCentroid,
	"sample":        tokSample,
	"precision":     tokPrecision,
	"highp":         tokHighP,
	"mediump":       tokMediumP,
	"lowp":          tokLowP,
	"continue":      tokContinue,
	"break":         tokBreak,
	"return":        tokReturn,
	"discard":       tokDiscard,
	"if":            tokIf,
	"else":          tokElse,
	"switch":        tokSwitch,
	"case":          tokCase,
	"default":       tokDefault,
	"while":         tokWhile,
	"do":            tokDo,
	"for":           tokFor,
	"void":          tokVoid,
	"struct":        tokStruct,
}
