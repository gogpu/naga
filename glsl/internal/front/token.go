package front

import (
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

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

// typeName is the type a GLSL type-name token denotes. A combined image
// sampler (sampler2D and friends) is not one IR type: lowering makes it an
// image global and a sampler global, so its shape is described by the fields
// instead of Inner.
type typeName struct {
	Inner ir.TypeInner // the IR type; nil for a combined sampler

	Combined   bool
	Comparison bool          // a *Shadow sampler
	Kind       ir.ScalarKind // the sampled value kind
	Dim        ir.ImageDimension
	Arrayed    bool
	Multi      bool
}

// parseTypeName classifies a GLSL type name, as Rust naga's types::parse_type
// does, with two differences the combined-sampler support requires: the
// sampler2D family (including i/u and *Shadow variants) is a type here, so
// `uniform sampler2D tex;` parses, and Rust's plain `sampler`/`samplerShadow`
// stay types.
func parseTypeName(name string) (typeName, bool) {
	switch name {
	case "bool":
		return typeName{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, true
	case "int":
		return typeName{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, true
	case "uint":
		return typeName{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, true
	case "float":
		return typeName{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, true
	case "double":
		return typeName{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, true
	case "float16_t":
		return typeName{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, true
	case "sampler":
		return typeName{Inner: ir.SamplerType{Comparison: false}}, true
	case "samplerShadow":
		return typeName{Inner: ir.SamplerType{Comparison: true}}, true
	}
	if tn, ok := parseCombinedSampler(name); ok {
		return tn, true
	}
	if tn, ok := parseVector(name); ok {
		return tn, true
	}
	if tn, ok := parseMatrix(name); ok {
		return tn, true
	}
	if tn, ok := parseTexture(name); ok {
		return tn, true
	}
	if tn, ok := parseImage(name); ok {
		return tn, true
	}
	return typeName{}, false
}

// vectorScalar maps a vector or matrix size-name prefix to its scalar type.
func vectorScalar(prefix string) (ir.ScalarType, bool) {
	switch prefix {
	case "":
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, true
	case "b":
		return ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, true
	case "i":
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, true
	case "u":
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, true
	case "d":
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, true
	case "f16":
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, true
	}
	return ir.ScalarType{}, false
}

// vectorSize parses "2", "3" or "4".
func vectorSize(size string) (ir.VectorSize, bool) {
	switch size {
	case "2":
		return ir.Vec2, true
	case "3":
		return ir.Vec3, true
	case "4":
		return ir.Vec4, true
	}
	return 0, false
}

// parseVector classifies vecN and its typed spellings (bvec, ivec, uvec, dvec,
// f16vec).
func parseVector(name string) (typeName, bool) {
	i := strings.Index(name, "vec")
	if i < 0 {
		return typeName{}, false
	}
	scalar, ok := vectorScalar(name[:i])
	if !ok {
		return typeName{}, false
	}
	size, ok := vectorSize(name[i+3:])
	if !ok {
		return typeName{}, false
	}
	return typeName{Inner: ir.VectorType{Size: size, Scalar: scalar}}, true
}

// parseMatrix classifies matN, matCxR, dmat* and f16mat*.
func parseMatrix(name string) (typeName, bool) {
	i := strings.Index(name, "mat")
	if i < 0 {
		return typeName{}, false
	}
	var scalar ir.ScalarType
	switch name[:i] {
	case "":
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	case "d":
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	case "f16":
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	default:
		return typeName{}, false
	}
	size := name[i+3:]
	columns, rows := ir.VectorSize(0), ir.VectorSize(0)
	if before, after, ok := strings.Cut(size, "x"); ok {
		var okColumns, okRows bool
		columns, okColumns = vectorSize(before)
		rows, okRows = vectorSize(after)
		if !okColumns || !okRows {
			return typeName{}, false
		}
	} else {
		square, ok := vectorSize(size)
		if !ok {
			return typeName{}, false
		}
		columns, rows = square, square
	}
	return typeName{Inner: ir.MatrixType{Columns: columns, Rows: rows, Scalar: scalar}}, true
}

// imageShape maps a texture or image size name to its dimension, array-ness and
// multisampling.
func imageShape(shape string) (ir.ImageDimension, bool, bool, bool) {
	switch shape {
	case "1D":
		return ir.Dim1D, false, false, true
	case "1DArray":
		return ir.Dim1D, true, false, true
	case "2D":
		return ir.Dim2D, false, false, true
	case "2DArray":
		return ir.Dim2D, true, false, true
	case "2DMS":
		return ir.Dim2D, false, true, true
	case "2DMSArray":
		return ir.Dim2D, true, true, true
	case "3D":
		return ir.Dim3D, false, false, true
	case "Cube":
		return ir.DimCube, false, false, true
	case "CubeArray":
		return ir.DimCube, true, false, true
	}
	return 0, false, false, false
}

// imageKind maps an i/u prefix of a texture or image name to the value kind.
func imageKind(prefix string) (ir.ScalarKind, bool) {
	switch prefix {
	case "":
		return ir.ScalarFloat, true
	case "i":
		return ir.ScalarSint, true
	case "u":
		return ir.ScalarUint, true
	}
	return 0, false
}

// parseTexture classifies texture1D and friends into a sampled image type.
func parseTexture(name string) (typeName, bool) {
	i := strings.Index(name, "texture")
	if i < 0 {
		return typeName{}, false
	}
	kind, ok := imageKind(name[:i])
	if !ok {
		return typeName{}, false
	}
	dim, arrayed, multi, ok := imageShape(name[i+len("texture"):])
	if !ok {
		return typeName{}, false
	}
	return typeName{Inner: ir.ImageType{
		Dim: dim, Arrayed: arrayed, Class: ir.ImageClassSampled,
		Multisampled: multi, SampledKind: kind,
	}}, true
}

// parseImage classifies image1D and friends into a storage image type. The
// storage format is a placeholder (R8Uint with load and store access), replaced
// when the declaration's layout(format=...) qualifier is applied; GLSL has no
// cube storage images here, matching Rust.
func parseImage(name string) (typeName, bool) {
	i := strings.Index(name, "image")
	if i < 0 {
		return typeName{}, false
	}
	kind, ok := imageKind(name[:i])
	if !ok {
		return typeName{}, false
	}
	dim, arrayed, multi, ok := imageShape(name[i+len("image"):])
	if !ok || dim == ir.DimCube || multi {
		return typeName{}, false
	}
	return typeName{Inner: ir.ImageType{
		Dim: dim, Arrayed: arrayed, Class: ir.ImageClassStorage,
		SampledKind:   kind,
		StorageFormat: ir.StorageFormatR8Uint,
		StorageAccess: ir.StorageAccessLoad | ir.StorageAccessStore,
	}}, true
}

// parseCombinedSampler classifies the sampler1D/2D/3D/Cube family, with i/u
// prefixes and *Shadow variants, as a combined image sampler.
func parseCombinedSampler(name string) (typeName, bool) {
	kind := ir.ScalarFloat
	rest := ""
	switch {
	case strings.HasPrefix(name, "sampler"):
		rest = name[len("sampler"):]
	case strings.HasPrefix(name, "usampler"):
		kind, rest = ir.ScalarUint, name[len("usampler"):]
	case strings.HasPrefix(name, "isampler"):
		kind, rest = ir.ScalarSint, name[len("isampler"):]
	default:
		return typeName{}, false
	}
	comparison := false
	if strings.HasSuffix(rest, "Shadow") {
		comparison = true
		rest = strings.TrimSuffix(rest, "Shadow")
	}
	dim, arrayed, multi, ok := imageShape(rest)
	if !ok {
		return typeName{}, false
	}
	// GLSL has no 3D and no multisampled shadow samplers.
	if comparison && (kind != ir.ScalarFloat || dim == ir.Dim3D || multi) {
		return typeName{}, false
	}
	return typeName{
		Combined: true, Comparison: comparison, Kind: kind,
		Dim: dim, Arrayed: arrayed, Multi: multi,
	}, true
}
