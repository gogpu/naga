package msl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// dotWrapper tracks an integer dot product wrapper function.
// MSL metal::dot doesn't support int/uint vectors, so we emit manual wrappers:
//
//	int naga_dot_int2(metal::int2 a, metal::int2 b) {
//	    return ( + a.x * b.x + a.y * b.y);
//	}
type dotWrapper struct {
	scalar ir.ScalarType
	size   ir.VectorSize
	name   string
}

// divModOverload identifies a typed div/mod helper function overload.
// Rust naga emits per-type overloads (not C++ templates) for integer div/mod.
// Overloads are recorded in first-use order to match Rust naga's output
// (which emits helpers via write_wrapped_binary_op as expressions are encountered).
type divModOverload struct {
	kind       ir.ScalarKind // ScalarSint or ScalarUint
	width      uint8         // scalar width in bytes (4 or 8)
	vectorSize ir.VectorSize // 0 for scalar, 2/3/4 for vector
	isDiv      bool          // true = naga_div, false = naga_mod
}

// mslTypeName returns the MSL type name for this overload (e.g., "uint", "metal::int2").
func (o divModOverload) mslTypeName() string {
	scalarName := scalarTypeName(ir.ScalarType{Kind: o.kind, Width: o.width})
	if o.vectorSize == 0 {
		return scalarName
	}
	return fmt.Sprintf("%s%s%d", Namespace, scalarName, o.vectorSize)
}

// f2iOverload identifies a float-to-int conversion helper function overload.
// Rust naga emits separate overloads for each (srcScalar, vectorSize, dstScalar) combo,
// e.g., naga_f2i32(half), naga_f2i32(float), naga_f2i32(metal::half2), naga_f2i32(metal::float2).
type f2iOverload struct {
	srcScalar  ir.ScalarType // float source scalar (Float16, Float32, Float64)
	vectorSize ir.VectorSize // 0 for scalar, 2/3/4 for vector
	dstScalar  ir.ScalarType // int destination scalar (Sint32, Uint32, Sint64, Uint64)
}

// wrappedMathResult describes a modf/frexp result struct variant.
// Rust naga emits structs like _modf_result_f32_, _modf_result_vec2_f32_, etc.
type wrappedMathResult struct {
	scalar     ir.ScalarType
	vectorSize ir.VectorSize // 0 for scalar
}

// modfStructName returns the MSL struct name for this modf result type.
func (r wrappedMathResult) modfStructName() string {
	return "_modf_result_" + wrappedMathSuffix(r) + "_"
}

// frexpStructName returns the MSL struct name for this frexp result type.
func (r wrappedMathResult) frexpStructName() string {
	return "_frexp_result_" + wrappedMathSuffix(r) + "_"
}

// wrappedMathSuffix returns the type suffix like "f32", "vec2_f32", "vec4_f32".
func wrappedMathSuffix(r wrappedMathResult) string {
	scalarSuffix := wrappedScalarSuffix(r.scalar)
	if r.vectorSize == 0 {
		return scalarSuffix
	}
	return fmt.Sprintf("vec%d_%s", r.vectorSize, scalarSuffix)
}

// wrappedScalarSuffix returns the Rust naga scalar suffix for wrapped math structs.
func wrappedScalarSuffix(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return "f16"
		case 4:
			return "f32"
		case 8:
			return "f64"
		}
	case ir.ScalarSint:
		switch s.Width {
		case 4:
			return "i32"
		}
	case ir.ScalarUint:
		switch s.Width {
		case 4:
			return "u32"
		}
	}
	return fmt.Sprintf("unknown_%d_%d", s.Kind, s.Width)
}

// atomicCompareExchangeVariant describes an _atomic_compare_exchange_result_* struct.
type atomicCompareExchangeVariant struct {
	scalar ir.ScalarType
}

// atomicExchangeStructName returns the struct name like _atomic_compare_exchange_result_Uint_4_.
func (v atomicCompareExchangeVariant) atomicExchangeStructName() string {
	kindName := "Uint"
	if v.scalar.Kind == ir.ScalarSint {
		kindName = "Sint"
	}
	return fmt.Sprintf("_atomic_compare_exchange_result_%s_%d_", kindName, v.scalar.Width)
}

// nameKey identifies an IR entity for name lookup.
type nameKey struct {
	kind    nameKeyKind
	handle1 uint32
	handle2 uint32
}

type nameKeyKind uint8

const (
	nameKeyType nameKeyKind = iota
	nameKeyStructMember
	nameKeyConstant
	nameKeyGlobalVariable
	nameKeyFunction
	nameKeyFunctionArgument
	nameKeyEntryPoint
	nameKeyLocal
	nameKeyExternalTexturePlane0
	nameKeyExternalTexturePlane1
	nameKeyExternalTexturePlane2
	nameKeyExternalTextureParams
	nameKeyOverride
)

// epFuncHandle returns a synthetic FunctionHandle for entry point index epIdx.
// Since entry point functions are no longer in Module.Functions[], we use a
// high-bit offset to avoid collisions with real function handles in nameKey maps.
const entryPointHandleBase = ir.FunctionHandle(0x80000000)

func epFuncHandle(epIdx int) ir.FunctionHandle {
	return entryPointHandleBase + ir.FunctionHandle(epIdx)
}

// Writer generates MSL source code from IR.
type Writer struct {
	module   *ir.Module
	options  *Options
	pipeline *PipelineOptions

	// Output buffer
	out strings.Builder

	// Current indentation level
	indent int

	// Name management
	names      map[nameKey]string
	namer      *namer
	structPads map[nameKey]struct{} // Tracks struct members that need padding

	// Type tracking
	typeNames     map[ir.TypeHandle]string
	arrayWrappers map[ir.TypeHandle]string // Array types wrapped in structs

	// Function context (set during function writing)
	currentFunction    *ir.Function
	currentFuncHandle  ir.FunctionHandle // For regular functions; for entry points, use epFuncHandle(currentEPIndex)
	currentEPIndex     int               // Entry point index when writing an entry point; -1 for regular functions
	localNames         map[uint32]string
	namedExpressions   map[ir.ExpressionHandle]string
	needBakeExpression map[ir.ExpressionHandle]struct{}

	// guardedIndices tracks expression handles used as indices in RZSW-policy accesses.
	// These need to be baked into temporaries to avoid double-evaluation in the
	// ternary condition + access. Matches Rust naga's guarded_indices / find_checked_indexes.
	guardedIndices map[ir.ExpressionHandle]struct{}

	// oobLocals maps type handles to variable names for out-of-bounds pointer locals.
	// When RZSW bounds checking needs a pointer fallback, we can't use DefaultConstructible
	// because we need an actual addressable local. Matches Rust naga's oob_local_for_type.
	oobLocals map[ir.TypeHandle]string

	// Output tracking
	entryPointNames                   map[string]string
	needsSizesBuffer                  bool
	needsDefaultConstructible         bool
	needsTextureSampleBaseClampToEdge bool
	clampToEdgeEmitted                bool
	argumentBufferWrapperEmitted      bool
	externalTextureWrapperEmitted     bool
	needsExternalTextureLoad          bool
	needsExternalTextureSample        bool
	needsExternalTextureDimensions    bool
	externalTextureLoadEmitted        bool
	externalTextureSampleEmitted      bool
	externalTextureDimensionsEmitted  bool

	// Typed div/mod overloads: tracks which (scalar kind, vector size) combos are needed.
	// Vector size 0 means scalar. Single merged slice in first-use order to match
	// Rust naga's output (which emits helpers as expressions are encountered).
	helperOverloads []divModOverload

	// Integer dot product wrapper functions: naga_dot_{type}{size}
	// MSL metal::dot doesn't support integer vectors, so we emit manual wrappers.
	dotWrappers []dotWrapper

	// absHelpers tracks signed integer abs helper functions to emit.
	// Rust naga emits naga_abs() using metal::select + as_type for signed integers.
	absHelpers []absHelper
	negHelpers []absHelper

	// needsRayQuery is set to true when the module contains RayQuery types,
	// triggering emission of the _RayQuery struct and _map_intersection_type helper.
	needsRayQuery bool

	// modfResultTypes tracks the modf result struct variants needed (by scalar/vector type).
	// Each entry describes a _modf_result_* struct to emit.
	modfResultTypes []wrappedMathResult

	// frexpResultTypes tracks the frexp result struct variants needed (by scalar/vector type).
	// Each entry describes a _frexp_result_* struct to emit.
	frexpResultTypes []wrappedMathResult

	// atomicCompareExchangeTypes tracks the _atomic_compare_exchange_result_* struct
	// variants needed (by scalar kind and width). Each entry describes a struct and
	// a pair of template helper functions to emit.
	atomicCompareExchangeTypes []atomicCompareExchangeVariant

	// f2iHelpers tracks the float-to-int helper functions needed (naga_f2i32, naga_f2u32, etc.)
	// Each entry describes a unique (srcScalar, vectorSize, dstScalar) overload.
	// Rust naga emits separate overloads for half/float/half2/float2 etc.
	f2iHelpers []f2iOverload

	// Global variable handles that need runtime array sizes in _mslBufferSizes struct.
	bufferSizeGlobals []uint32

	entryPointOutputVar        string
	entryPointOutputType       ir.TypeHandle
	entryPointOutputTypeActive bool
	entryPointOutputStructName string // MSL struct name for entry point output
	entryPointInputStructArg   int
	entryPointStage            ir.ShaderStage // current entry point's stage

	// flattenedMemberNames maps (struct type, member index) to the MSL name
	// used for that member when it appears as a flattened entry point parameter.
	// Matches Rust naga's flattened_member_names HashMap.
	flattenedMemberNames map[nameKey]string
	// hasVaryings indicates whether the current entry point has a stage_in struct
	// with at least one location-bound member (i.e., actual varyings to pass).
	hasVaryings bool

	// currentAccessPolicy is the bounds check policy active for the current access chain.
	// Set by writeLoad before traversing the pointer access chain, reset after.
	// BoundsCheckUnchecked (zero value) means no special handling needed.
	currentAccessPolicy BoundsCheckPolicy

	// insideRZSW is true when we're inside a RZSW ternary or if-check.
	// Prevents nested RZSW wrapping for inner access chain expressions.
	insideRZSW bool

	// Pass-through globals: for each function, the list of global variable handles
	// that need to be passed as extra parameters (textures, samplers, buffers, etc.)
	// MSL requires these because helper functions can't access entry point bindings directly.
	funcPassThroughGlobals map[ir.FunctionHandle][]uint32

	// currentResourceMap maps (group, binding) to Metal bind targets for the current entry point.
	// Computed per entry point to assign sequential buffer/texture/sampler indices that avoid
	// collisions across bind groups.
	currentResourceMap map[ir.ResourceBinding]BindTarget

	// globalWriteUsage tracks which global variables are written to by any function (module-wide).
	// Used to determine if storage buffers need the `const` qualifier in MSL.
	globalWriteUsage map[uint32]struct{}

	// perFuncWriteUsage tracks which global variables are written to per function.
	// Key is the function handle, value is a set of global variable handles.
	// Matches Rust naga's per-function GlobalUse::WRITE analysis.
	perFuncWriteUsage map[int]map[uint32]struct{}

	// minRequiredVersion tracks the minimum Metal version required by features used
	// in the shader. Bumped when builtins like barycentric_coord or amplification_id
	// are encountered. The final header version is max(options.LangVersion, minRequiredVersion).
	minRequiredVersion Version

	// unnamedCount tracks the number of unnamed variables allocated for subgroup operations
	// and other unnamed results. Used by allocateUnnamedVar().
	unnamedCount int

	// Vertex Pulling Transform (VPT) state.
	// vptUnpackingFunctions maps VertexFormat to the generated unpacking function info.
	vptUnpackingFunctions map[VertexFormat]vptUnpackFunc
	// vptBufferMappings holds resolved VPT buffer mapping info (names, etc.)
	vptBufferMappings []vptBufferMappingResolved
	// vptNeedsVertexID / vptNeedsInstanceID track whether any buffer uses ByVertex/ByInstance step mode.
	vptNeedsVertexID   bool
	vptNeedsInstanceID bool
	// vptVertexIDName / vptInstanceIDName are the generated names for [[vertex_id]] / [[instance_id]].
	vptVertexIDName   string
	vptInstanceIDName string
}

// vptUnpackFunc holds info about a generated unpacking function for VPT.
type vptUnpackFunc struct {
	name      string
	byteCount uint32
	dimension uint32
}

// vptBufferMappingResolved holds resolved names for a vertex buffer mapping.
type vptBufferMappingResolved struct {
	id         uint32
	stride     uint32
	stepMode   VertexBufferStepMode
	tyName     string
	paramName  string
	elemName   string
	attributes []AttributeMapping
}

// vptAttributeResolved holds resolved info about an attribute for VPT.
type vptAttributeResolved struct {
	tyName    string
	dimension uint32
	tyIsInt   bool
	name      string
}

// namer generates unique identifiers.
// It matches Rust naga's Namer behavior: per-base counters, trailing underscore
// for names ending in a digit or matching a keyword.
type namer struct {
	usedNames map[string]struct{}
	// perBase tracks the last numeric suffix used for each sanitized base name.
	// Zero means "no suffix has been appended yet".
	perBase map[string]uint32
}

func newNamer() *namer {
	return &namer{
		usedNames: make(map[string]struct{}),
		perBase:   make(map[string]uint32),
	}
}

// fallbackName is used when sanitization produces an empty string.
const fallbackName = "unnamed"

// call generates a unique name based on the given base.
// Matches Rust naga Namer::call behavior:
//   - sanitize base name
//   - if first use: return base (with trailing _ if ends with digit or is keyword)
//   - if already used: append _{N} suffix with per-base counter
func (n *namer) call(base string) string {
	sanitized := sanitizeName(base)
	if sanitized == "" {
		sanitized = fallbackName
	}

	count, exists := n.perBase[sanitized]
	if exists {
		// Already used this base — append suffix with incremented counter
		count++
		n.perBase[sanitized] = count
		candidate := fmt.Sprintf("%s_%d", sanitized, count)
		n.usedNames[candidate] = struct{}{}
		return candidate
	}

	// First use of this base
	n.perBase[sanitized] = 0

	result := sanitized
	// Add trailing underscore if name ends with digit or is a reserved keyword
	if result != "" && result[len(result)-1] >= '0' && result[len(result)-1] <= '9' {
		result += "_"
	} else if isReserved(result) {
		result += "_"
	}

	n.usedNames[result] = struct{}{}
	return result
}

// sanitizeName cleans a raw label for use as an identifier base.
// Matches Rust naga Namer::sanitize behavior:
//   - drops leading digits
//   - retains only ASCII alphanumeric and '_'
//   - converts non-ASCII characters to u{04hex}_ format
//   - converts C++-ish type separators (:, <, >, ,) to underscores
//   - collapses consecutive underscores
//   - trims trailing underscores
func sanitizeName(s string) string {
	if s == "" {
		return fallbackName
	}

	// Fast path: if already valid (no leading digits, no special chars, no __)
	valid := true
	for i, c := range s {
		if i == 0 && c >= '0' && c <= '9' {
			valid = false
			break
		}
		if !isAlphanumericOrUnderscore(c) {
			valid = false
			break
		}
	}
	if valid && !strings.Contains(s, "__") {
		return strings.TrimRight(s, "_")
	}

	// Slow path: filter character by character, matching Rust naga Namer::sanitize
	var buf strings.Builder
	for _, c := range s {
		// Convert C++-ish type separators to underscores
		switch c {
		case ':', '<', '>', ',':
			c = '_'
		}

		hadUnderscoreAtEnd := buf.Len() > 0 && buf.String()[buf.Len()-1] == '_'
		if hadUnderscoreAtEnd && c == '_' {
			continue // collapse consecutive underscores
		}

		if isAlphanumericOrUnderscore(c) {
			if buf.Len() == 0 && c >= '0' && c <= '9' {
				continue // drop leading digits
			}
			buf.WriteRune(c)
		} else {
			// Non-ASCII or special character: convert to u{04hex}_ format
			if buf.Len() > 0 && !hadUnderscoreAtEnd {
				buf.WriteByte('_')
			}
			fmt.Fprintf(&buf, "u%04x_", c)
		}
	}

	result := strings.TrimRight(buf.String(), "_")
	if result == "" {
		return fallbackName
	}
	return result
}

func isAlphanumericOrUnderscore(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// newWriter creates a new MSL writer.
func newWriter(module *ir.Module, options *Options, pipeline *PipelineOptions) *Writer {
	return &Writer{
		module:                   module,
		options:                  options,
		pipeline:                 pipeline,
		names:                    make(map[nameKey]string),
		namer:                    newNamer(),
		structPads:               make(map[nameKey]struct{}),
		typeNames:                make(map[ir.TypeHandle]string),
		arrayWrappers:            make(map[ir.TypeHandle]string),
		entryPointNames:          make(map[string]string),
		namedExpressions:         make(map[ir.ExpressionHandle]string),
		entryPointInputStructArg: -1,
		funcPassThroughGlobals:   make(map[ir.FunctionHandle][]uint32),
	}
}

// String returns the generated MSL source code.
// The output is trimmed to end with exactly one newline, matching Rust naga.
func (w *Writer) String() string {
	s := w.out.String()
	s = strings.TrimRight(s, "\n")
	return s + "\n"
}

// requireVersion bumps the minimum required Metal version if the given
// version is higher than the current minimum. This is called when features
// requiring specific Metal versions are encountered (e.g., barycentric_coord
// requires Metal 2.3).
func (w *Writer) requireVersion(v Version) {
	if w.minRequiredVersion.Less(v) {
		w.minRequiredVersion = v
	}
}

// effectiveVersion returns the Metal version to emit in the header.
// It is the maximum of the user-requested version and the minimum
// required version based on shader features.
func (w *Writer) effectiveVersion() Version {
	if w.options.LangVersion.Less(w.minRequiredVersion) {
		return w.minRequiredVersion
	}
	return w.options.LangVersion
}

// writeModule generates MSL code for the entire module.
func (w *Writer) writeModule() error {
	// 1. Register all names
	if err := w.registerNames(); err != nil {
		return err
	}

	// 1b. Determine if DefaultConstructible helper struct is needed.
	// Matches Rust naga: emitted when any bounds check policy uses ReadZeroSkipWrite.
	w.needsDefaultConstructible = w.options.BoundsCheckPolicies.Contains(BoundsCheckReadZeroSkipWrite)

	// 1c. Scan globals for runtime-sized arrays → _mslBufferSizes struct members
	w.scanBufferSizeGlobals()

	// 1d. Scan types for RayQuery → _RayQuery struct and _map_intersection_type helper
	w.scanRayQueryTypes()

	// 1e. Pre-scan for ClampToEdge texture sampling.
	w.scanClampToEdge()

	// 1f. Pre-scan all expressions for modf/frexp/atomic-compare-exchange types.
	// This ensures all wrapped struct variants are registered even when the
	// expression writer optimizes away the wrapper call.
	w.scanWrappedMathTypes()

	// 1g. Initialize Vertex Pulling Transform (VPT) state.
	// This generates namer names for VPT buffers and writes unpacking functions.
	// Must happen after registerNames but before types are written.
	var vptUnpackCode string
	if w.options.VertexPullingTransform && len(w.options.VertexBufferMappings) > 0 {
		savedOut := w.out
		w.out = strings.Builder{}
		w.initVertexPulling()
		vptUnpackCode = w.out.String()
		w.out = savedOut
	}

	// 2. Write _mslBufferSizes struct + type definitions to a temporary buffer.
	// The sizes struct must appear before type definitions (matches Rust naga order).
	typesOut := w.out
	w.out = strings.Builder{}
	w.writeBufferSizesStruct()
	if err := w.writeTypes(); err != nil {
		return err
	}
	typesCode := w.out.String()

	// 3. Write constants
	w.out = strings.Builder{}
	if err := w.writeConstants(); err != nil {
		return err
	}
	constantsCode := w.out.String()

	// 4. Analyze pass-through globals for helper functions
	w.analyzeFuncPassThroughGlobals()

	// 4b. Analyze which globals are written to (for const qualifier on storage buffers)
	w.analyzeGlobalWriteUsage()

	// 5. Write functions and entry points to per-function buffers, tracking
	//    which helpers are needed. Helpers are emitted demand-driven between
	//    functions, matching Rust naga's output order.
	type funcOutput struct {
		code              string
		helpersAfter      int // count of helperOverloads after writing this function
		dotWrappersAfter  int // count of dotWrappers after writing this function
		f2iHelpersAfter   int // count of f2iHelpers after writing this function
		absHelpersAfter   int // count of absHelpers after writing this function
		negHelpersAfter   int // count of negHelpers after writing this function
		isRegularFunction bool
	}
	var funcOutputs []funcOutput

	// Write regular functions
	for handle := range w.module.Functions {
		fn := &w.module.Functions[handle]
		if w.isEntryPointFunction(ir.FunctionHandle(handle)) {
			continue
		}
		w.out = strings.Builder{}
		if err := w.writeFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
		funcOutputs = append(funcOutputs, funcOutput{
			code:              w.out.String(),
			helpersAfter:      len(w.helperOverloads),
			dotWrappersAfter:  len(w.dotWrappers),
			f2iHelpersAfter:   len(w.f2iHelpers),
			absHelpersAfter:   len(w.absHelpers),
			negHelpersAfter:   len(w.negHelpers),
			isRegularFunction: true,
		})
	}

	// Write entry points — each as a separate funcOutput so that helpers
	// (naga_f2i32, etc.) can be interleaved between entry points, matching Rust naga.
	for epIdx, ep := range w.module.EntryPoints {
		if w.pipeline.EntryPoint != nil {
			if w.pipeline.EntryPoint.Name != ep.Name || w.pipeline.EntryPoint.Stage != ep.Stage {
				continue
			}
		}
		w.out = strings.Builder{}
		if err := w.writeEntryPoint(epIdx, &ep); err != nil {
			return err
		}
		funcOutputs = append(funcOutputs, funcOutput{
			code:              w.out.String(),
			helpersAfter:      len(w.helperOverloads),
			dotWrappersAfter:  len(w.dotWrappers),
			f2iHelpersAfter:   len(w.f2iHelpers),
			absHelpersAfter:   len(w.absHelpers),
			negHelpersAfter:   len(w.negHelpers),
			isRegularFunction: false,
		})
	}

	// 5b. Write modf/frexp structs and atomic compare-exchange structs to a buffer.
	// These are discovered during function/entry point writing (step 5) and must
	// appear before the struct definitions in Rust naga output order.
	// Order matches Rust naga's FastIndexMap insertion order (first encounter during lowering).
	w.out = strings.Builder{}
	w.writeModfFrexpStructs()
	w.writeAtomicCompareExchangeStructs()
	wrappedStructsCode := w.out.String()

	// 5c. Write modf/frexp functions, atomic compare-exchange template functions,
	// and float-to-int helper functions.
	w.out = strings.Builder{}
	w.writeModfFrexpFunctions()
	w.writeAtomicCompareExchangeFunctions()
	w.writeF2IHelpers()
	wrappedFuncsCode := w.out.String()

	// 6. Now we know the effective Metal version and which helpers are needed.
	//    Assemble final output: header, DefaultConstructible, RayQuery,
	//    types, wrapped structs, wrapped funcs, constants, then functions
	//    with demand-driven helper interleaving.
	w.out = typesOut
	w.writeHeader()
	w.writeDefaultConstructible()
	w.writeRayQueryStruct()
	w.out.WriteString(typesCode)
	w.out.WriteString(wrappedStructsCode)

	hasWrappedFuncs := wrappedFuncsCode != ""

	if hasWrappedFuncs {
		if wrappedStructsCode != "" || typesCode != "" {
			w.writeLine("")
		}
		w.out.WriteString(wrappedFuncsCode)
	}

	// Constants come after wrapped functions.
	w.out.WriteString(constantsCode)

	// VPT unpacking functions come after constants, before function outputs.
	if vptUnpackCode != "" {
		w.out.WriteString(vptUnpackCode)
	}

	// Emit functions with demand-driven helper interleaving.
	// Before each function, emit any new helpers that were registered during
	// that function's writing. This matches Rust naga's order.
	prevHelpers := 0
	prevDotWrappers := 0
	prevF2I := 0
	prevAbs := 0
	prevNeg := 0
	for _, fo := range funcOutputs {
		// Emit new helpers needed by this function
		newHelpers := w.helperOverloads[prevHelpers:fo.helpersAfter]
		newDotWrappers := w.dotWrappers[prevDotWrappers:fo.dotWrappersAfter]
		newF2I := w.f2iHelpers[prevF2I:fo.f2iHelpersAfter]
		newAbs := w.absHelpers[prevAbs:fo.absHelpersAfter]
		newNeg := w.negHelpers[prevNeg:fo.negHelpersAfter]

		// Check if this function uses ClampToEdge
		needsClampToEdge := false
		if w.needsTextureSampleBaseClampToEdge && !w.clampToEdgeEmitted {
			if strings.Contains(fo.code, "nagaTextureSampleBaseClampToEdge") {
				needsClampToEdge = true
				w.clampToEdgeEmitted = true
			}
		}

		// Check if this function uses external texture helpers
		needsExtSample := false
		if w.needsExternalTextureSample && !w.externalTextureSampleEmitted {
			if strings.Contains(fo.code, "nagaTextureSampleBaseClampToEdge") {
				needsExtSample = true
				w.externalTextureSampleEmitted = true
			}
		}
		needsExtLoad := false
		if w.needsExternalTextureLoad && !w.externalTextureLoadEmitted {
			if strings.Contains(fo.code, "nagaTextureLoadExternal") {
				needsExtLoad = true
				w.externalTextureLoadEmitted = true
			}
		}
		needsExtDimensions := false
		if w.needsExternalTextureDimensions && !w.externalTextureDimensionsEmitted {
			if strings.Contains(fo.code, "nagaTextureDimensionsExternal") {
				needsExtDimensions = true
				w.externalTextureDimensionsEmitted = true
			}
		}
		needsExternalHelpers := needsExtSample || needsExtLoad || needsExtDimensions

		if len(newHelpers) > 0 || len(newDotWrappers) > 0 || len(newF2I) > 0 || len(newAbs) > 0 || len(newNeg) > 0 || needsClampToEdge || needsExternalHelpers {
			// Blank line before helpers if there are regular functions before them
			if fo.isRegularFunction {
				w.writeLine("")
			}
			savedOut := w.out
			w.out = strings.Builder{}
			// Emit f2i helpers before dot wrappers to match Rust naga's
			// write_wrapped_functions ordering (casts come before binary ops).
			for _, ovl := range newF2I {
				w.writeF2IHelper(ovl)
			}
			// Emit neg helpers before div/mod (matches Rust naga ordering)
			for _, neg := range newNeg {
				w.writeNegHelper(neg)
			}
			w.writeHelperSubsetWithAbs(newHelpers, newAbs, newDotWrappers)
			if needsClampToEdge && !needsExtSample {
				w.writeTextureSampleBaseClampToEdge()
			}
			if needsExtSample {
				w.writeExternalTextureSampleBaseClampToEdge()
			}
			if needsExtLoad {
				w.writeExternalTextureLoadHelper()
			}
			if needsExtDimensions {
				w.writeExternalTextureDimensionsHelper()
			}
			helpersStr := w.out.String()
			w.out = savedOut

			// Trim trailing blank when followed by a function (which has its own blank)
			if fo.isRegularFunction && helpersStr != "" {
				helpersStr = strings.TrimRight(helpersStr, "\n") + "\n"
			}
			w.out.WriteString(helpersStr)
		}

		w.out.WriteString(fo.code)
		prevHelpers = fo.helpersAfter
		prevDotWrappers = fo.dotWrappersAfter
		prevF2I = fo.f2iHelpersAfter
		prevAbs = fo.absHelpersAfter
		prevNeg = fo.negHelpersAfter
	}

	return nil
}

// needsArrayLength returns true if the type contains a runtime-sized array,
// either directly or as the last member of a struct.
// Matches Rust naga's needs_array_length function.
func (w *Writer) needsArrayLength(typeHandle ir.TypeHandle) bool {
	if int(typeHandle) >= len(w.module.Types) {
		return false
	}
	typ := &w.module.Types[typeHandle]
	switch inner := typ.Inner.(type) {
	case ir.ArrayType:
		return inner.Size.Constant == nil // Dynamic array
	case ir.StructType:
		if len(inner.Members) == 0 {
			return false
		}
		lastMember := inner.Members[len(inner.Members)-1]
		return w.needsArrayLength(lastMember.Type)
	default:
		return false
	}
}

// scanBufferSizeGlobals populates bufferSizeGlobals with handles of global
// variables that contain runtime-sized arrays. These need entries in
// the _mslBufferSizes struct.
func (w *Writer) scanBufferSizeGlobals() {
	w.bufferSizeGlobals = nil
	for i, global := range w.module.GlobalVariables {
		if w.needsArrayLength(global.Type) {
			w.bufferSizeGlobals = append(w.bufferSizeGlobals, uint32(i))
		}
	}
	w.needsSizesBuffer = len(w.bufferSizeGlobals) > 0
}

// scanRayQueryTypes scans the module for RayQuery types.
// If any are found, needsRayQuery is set to true so the _RayQuery struct
// and _map_intersection_type helper are emitted.
func (w *Writer) scanRayQueryTypes() {
	for _, typ := range w.module.Types {
		if _, ok := typ.Inner.(ir.RayQueryType); ok {
			w.needsRayQuery = true
			w.requireVersion(Version2_4)
			return
		}
	}
}

// writeRayQueryStruct emits the _RayQuery struct and _map_intersection_type helper.
// Matches Rust naga output for Metal 2.4+ ray tracing shaders.
// NOTE: requireVersion(Version2_4) is called in scanRayQueryTypes, not here,
// so the header version is correct when writeHeader is called before this method.
func (w *Writer) writeRayQueryStruct() {
	if !w.needsRayQuery {
		return
	}
	w.writeLine("struct _RayQuery {")
	w.pushIndent()
	w.writeLine("metal::raytracing::intersector<metal::raytracing::instancing, metal::raytracing::triangle_data, metal::raytracing::world_space_data> intersector;")
	w.writeLine("metal::raytracing::intersector<metal::raytracing::instancing, metal::raytracing::triangle_data, metal::raytracing::world_space_data>::result_type intersection;")
	w.writeLine("bool ready = false;")
	w.popIndent()
	w.writeLine("};")
	w.writeLine("constexpr metal::uint _map_intersection_type(const metal::raytracing::intersection_type ty) {")
	w.pushIndent()
	w.writeLine("return ty==metal::raytracing::intersection_type::triangle ? 1 : ")
	w.writeLine("    ty==metal::raytracing::intersection_type::bounding_box ? 4 : 0;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeModfFrexpStructs emits _modf_result_* and _frexp_result_* struct definitions
// and their corresponding naga_modf/naga_frexp wrapper functions.
// Matches Rust naga output.
func (w *Writer) writeModfFrexpStructs() {
	for _, r := range w.modfResultTypes {
		name := r.modfStructName()
		valType := wrappedMathMSLType(r.scalar, r.vectorSize)
		w.writeLine("struct %s {", name)
		w.pushIndent()
		w.writeLine("%s fract;", valType)
		w.writeLine("%s whole;", valType)
		w.popIndent()
		w.writeLine("};")
	}
	for _, r := range w.frexpResultTypes {
		name := r.frexpStructName()
		valType := wrappedMathMSLType(r.scalar, r.vectorSize)
		// frexp exp field is always int (or int vector)
		expType := wrappedMathMSLType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, r.vectorSize)
		w.writeLine("struct %s {", name)
		w.pushIndent()
		w.writeLine("%s fract;", valType)
		w.writeLine("%s exp;", expType)
		w.popIndent()
		w.writeLine("};")
	}
}

// writeModfFrexpFunctions emits the naga_modf and naga_frexp wrapper functions.
// These must appear after the struct definitions and before entry points.
func (w *Writer) writeModfFrexpFunctions() {
	for i, r := range w.modfResultTypes {
		structName := r.modfStructName()
		argType := wrappedMathMSLType(r.scalar, r.vectorSize)
		if i > 0 {
			w.writeLine("")
		}
		w.writeLine("%s naga_modf(%s arg) {", structName, argType)
		w.pushIndent()
		w.writeLine("%s other;", argType)
		w.writeLine("%s fract = %smodf(arg, other);", argType, Namespace)
		w.writeLine("return %s{ fract, other };", structName)
		w.popIndent()
		w.writeLine("}")
	}
	for i, r := range w.frexpResultTypes {
		structName := r.frexpStructName()
		argType := wrappedMathMSLType(r.scalar, r.vectorSize)
		// frexp uses int (or int vector) for the exponent output
		expScalar := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
		expType := wrappedMathMSLType(expScalar, r.vectorSize)
		// Rust naga uses non-namespaced type for the local variable when it's a vector
		expLocalType := expType
		if r.vectorSize > 0 {
			// Rust naga: "int4 other;" (without metal:: prefix)
			expLocalType = fmt.Sprintf("%s%d", scalarTypeName(expScalar), r.vectorSize)
		}
		if i > 0 || len(w.modfResultTypes) > 0 {
			w.writeLine("")
		}
		w.writeLine("%s naga_frexp(%s arg) {", structName, argType)
		w.pushIndent()
		w.writeLine("%s other;", expLocalType)
		w.writeLine("%s fract = %sfrexp(arg, other);", argType, Namespace)
		w.writeLine("return %s{ fract, other };", structName)
		w.popIndent()
		w.writeLine("}")
	}
}

// writeAtomicCompareExchangeStructs emits _atomic_compare_exchange_result_* struct
// definitions. Matches Rust naga output.
func (w *Writer) writeAtomicCompareExchangeStructs() {
	for _, v := range w.atomicCompareExchangeTypes {
		name := v.atomicExchangeStructName()
		scalarName := scalarTypeName(v.scalar)
		w.writeLine("struct %s {", name)
		w.pushIndent()
		w.writeLine("%s old_value;", scalarName)
		w.writeLine("bool exchanged;")
		// Padding: bool is 1 byte, need to pad to align next field or end of struct.
		// Rust naga emits char _pad2[3] for 4-byte scalars, char _pad2[7] for 8-byte.
		padSize := int(v.scalar.Width) - 1
		if padSize > 0 {
			w.writeLine("char _pad2[%d];", padSize)
		}
		w.popIndent()
		w.writeLine("};")
	}
}

// writeAtomicCompareExchangeFunctions emits naga_atomic_compare_exchange_weak_explicit
// template helper functions. Rust naga emits two overloads per scalar type:
// one for device address space and one for threadgroup.
func (w *Writer) writeAtomicCompareExchangeFunctions() {
	for i, v := range w.atomicCompareExchangeTypes {
		structName := v.atomicExchangeStructName()
		scalarName := scalarTypeName(v.scalar)

		// Blank line between variants (not before the first one).
		// Matches Rust naga output.
		if i > 0 {
			w.writeLine("")
		}

		// Device overload
		w.writeLine("template <typename A>")
		w.writeLine("%s naga_atomic_compare_exchange_weak_explicit(", structName)
		w.pushIndent()
		w.writeLine("device A *atomic_ptr,")
		w.writeLine("%s cmp,", scalarName)
		w.writeLine("%s v", scalarName)
		w.popIndent()
		w.writeLine(") {")
		w.pushIndent()
		w.writeLine("bool swapped = %satomic_compare_exchange_weak_explicit(", Namespace)
		w.pushIndent()
		w.writeLine("atomic_ptr, &cmp, v,")
		w.writeLine("%smemory_order_relaxed, %smemory_order_relaxed", Namespace, Namespace)
		w.popIndent()
		w.writeLine(");")
		w.writeLine("return %s{cmp, swapped};", structName)
		w.popIndent()
		w.writeLine("}")

		// Threadgroup overload
		w.writeLine("template <typename A>")
		w.writeLine("%s naga_atomic_compare_exchange_weak_explicit(", structName)
		w.pushIndent()
		w.writeLine("threadgroup A *atomic_ptr,")
		w.writeLine("%s cmp,", scalarName)
		w.writeLine("%s v", scalarName)
		w.popIndent()
		w.writeLine(") {")
		w.pushIndent()
		w.writeLine("bool swapped = %satomic_compare_exchange_weak_explicit(", Namespace)
		w.pushIndent()
		w.writeLine("atomic_ptr, &cmp, v,")
		w.writeLine("%smemory_order_relaxed, %smemory_order_relaxed", Namespace, Namespace)
		w.popIndent()
		w.writeLine(");")
		w.writeLine("return %s{cmp, swapped};", structName)
		w.popIndent()
		w.writeLine("}")
	}
}

// wrappedMathMSLType returns the MSL type name for a scalar or vector type used
// in modf/frexp result structs.
func wrappedMathMSLType(scalar ir.ScalarType, vectorSize ir.VectorSize) string {
	name := scalarTypeName(scalar)
	if vectorSize == 0 {
		return name
	}
	return fmt.Sprintf("%s%s%d", Namespace, name, vectorSize)
}

// registerModfResult registers a modf result struct variant if not already registered.
func (w *Writer) registerModfResult(scalar ir.ScalarType, vectorSize ir.VectorSize) {
	r := wrappedMathResult{scalar: scalar, vectorSize: vectorSize}
	for _, existing := range w.modfResultTypes {
		if existing == r {
			return
		}
	}
	w.modfResultTypes = append(w.modfResultTypes, r)
}

// registerFrexpResult registers a frexp result struct variant if not already registered.
func (w *Writer) registerFrexpResult(scalar ir.ScalarType, vectorSize ir.VectorSize) {
	r := wrappedMathResult{scalar: scalar, vectorSize: vectorSize}
	for _, existing := range w.frexpResultTypes {
		if existing == r {
			return
		}
	}
	w.frexpResultTypes = append(w.frexpResultTypes, r)
}

// registerAtomicCompareExchange registers an atomic compare-exchange result variant.
func (w *Writer) registerAtomicCompareExchange(scalar ir.ScalarType) {
	v := atomicCompareExchangeVariant{scalar: scalar}
	for _, existing := range w.atomicCompareExchangeTypes {
		if existing == v {
			return
		}
	}
	w.atomicCompareExchangeTypes = append(w.atomicCompareExchangeTypes, v)
}

// registerF2IHelper registers a float-to-int helper function for the given overload.
func (w *Writer) registerF2IHelper(srcScalar ir.ScalarType, vectorSize ir.VectorSize, dstScalar ir.ScalarType) {
	ovl := f2iOverload{srcScalar: srcScalar, vectorSize: vectorSize, dstScalar: dstScalar}
	for _, existing := range w.f2iHelpers {
		if existing == ovl {
			return
		}
	}
	w.f2iHelpers = append(w.f2iHelpers, ovl)
}

// f2iFunctionName returns the helper function name for a float-to-int cast.
func f2iFunctionName(dst ir.ScalarType) string {
	switch {
	case dst.Kind == ir.ScalarSint && dst.Width == 4:
		return "naga_f2i32"
	case dst.Kind == ir.ScalarUint && dst.Width == 4:
		return "naga_f2u32"
	case dst.Kind == ir.ScalarSint && dst.Width == 8:
		return "naga_f2i64"
	case dst.Kind == ir.ScalarUint && dst.Width == 8:
		return "naga_f2u64"
	default:
		return "naga_f2i32"
	}
}

// writeF2IHelpers emits ALL float-to-int helper function definitions.
// Used during the wrappedFuncsCode phase for helpers not interleaved.
func (w *Writer) writeF2IHelpers() {
	// No-op: f2i helpers are now interleaved demand-driven via writeF2IHelper.
}

// writeF2IHelper emits a single float-to-int helper function overload.
// Rust naga emits per-(src, vector, dst) overloads with type-specific clamp bounds.
func (w *Writer) writeF2IHelper(ovl f2iOverload) {
	funName := f2iFunctionName(ovl.dstScalar)

	// Build source type name
	var srcTypeName string
	if ovl.vectorSize == 0 {
		srcTypeName = scalarTypeName(ovl.srcScalar)
	} else {
		srcTypeName = fmt.Sprintf("%s%s%d", Namespace, scalarTypeName(ovl.srcScalar), ovl.vectorSize)
	}

	// Build destination type name
	var dstTypeName string
	if ovl.vectorSize == 0 {
		dstTypeName = scalarTypeName(ovl.dstScalar)
	} else {
		dstTypeName = fmt.Sprintf("%s%s%d", Namespace, scalarTypeName(ovl.dstScalar), ovl.vectorSize)
	}

	// Determine clamping range based on (srcScalar, dstScalar).
	// For f16, the clamp bounds are limited by f16 range (65504).
	// For f32/f64, they're limited by the int type's representable range.
	minVal, maxVal := f2iClampBounds(ovl.srcScalar, ovl.dstScalar)

	w.writeLine("%s %s(%s value) {", dstTypeName, funName, srcTypeName)
	w.pushIndent()
	w.writeLine("return static_cast<%s>(%sclamp(value, %s, %s));", dstTypeName, Namespace, minVal, maxVal)
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// f2iClampBounds returns the (min, max) clamp literal strings for a float-to-int conversion.
// Matches Rust naga's proc::min_max_float_representable_by.
func f2iClampBounds(src, dst ir.ScalarType) (string, string) {
	isF16 := src.Width == 2
	suffix := ""
	if isF16 {
		suffix = "h"
	}

	switch {
	case isF16 && dst.Kind == ir.ScalarSint:
		return "-65504.0h", "65504.0h"
	case isF16 && dst.Kind == ir.ScalarUint:
		return "0.0h", "65504.0h"
	case dst.Kind == ir.ScalarSint && dst.Width == 4:
		return "-2147483600.0" + suffix, "2147483500.0" + suffix
	case dst.Kind == ir.ScalarUint && dst.Width == 4:
		return "0.0" + suffix, "4294967000.0" + suffix
	case dst.Kind == ir.ScalarSint && dst.Width == 8:
		return "-9223372000000000000.0" + suffix, "9223371500000000000.0" + suffix
	case dst.Kind == ir.ScalarUint && dst.Width == 8:
		return "0.0" + suffix, "18446743000000000000.0" + suffix
	default:
		return "-2147483600.0", "2147483500.0"
	}
}

// scanWrappedMathTypes scans all functions and entry points for MathModf/MathFrexp/
// AtomicExchange(compare) expressions and pre-registers the wrapped struct types.
// This ensures all structs are emitted even when the expression writer optimizes
// away the wrapper call (e.g., modf(v).whole → trunc(v)).
func (w *Writer) scanWrappedMathTypes() {
	scanFunc := func(fn *ir.Function) {
		for _, expr := range fn.Expressions {
			switch k := expr.Kind.(type) {
			case ir.ExprMath:
				if k.Fun == ir.MathModf || k.Fun == ir.MathFrexp {
					// Resolve the argument type
					if int(k.Arg) < len(fn.ExpressionTypes) {
						res := &fn.ExpressionTypes[k.Arg]
						var inner ir.TypeInner
						if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
							inner = w.module.Types[*res.Handle].Inner
						} else if res.Value != nil {
							inner = res.Value
						}
						if inner != nil {
							scalar, vecSize := typeInnerScalarAndVec(inner)
							if scalar != nil {
								if k.Fun == ir.MathModf {
									w.registerModfResult(*scalar, vecSize)
								} else {
									w.registerFrexpResult(*scalar, vecSize)
								}
							}
						}
					}
				}
			}
		}
		// Scan statements for atomic compare-exchange
		w.scanBlockForAtomicExchange(fn, fn.Body)
	}

	for i := range w.module.Functions {
		scanFunc(&w.module.Functions[i])
	}
	for i := range w.module.EntryPoints {
		fn := w.getEntryPointFunction(i)
		if fn != nil {
			scanFunc(fn)
		}
	}
}

// getEntryPointFunction returns the Function for an entry point index.
func (w *Writer) getEntryPointFunction(epIdx int) *ir.Function {
	if epIdx >= len(w.module.EntryPoints) {
		return nil
	}
	return &w.module.EntryPoints[epIdx].Function
}

// scanBlockForAtomicExchange recursively scans a block of statements for atomic compare-exchange.
func (w *Writer) scanBlockForAtomicExchange(fn *ir.Function, block ir.Block) {
	for _, stmt := range block {
		switch s := stmt.Kind.(type) {
		case ir.StmtAtomic:
			if exchange, ok := s.Fun.(ir.AtomicExchange); ok && exchange.Compare != nil {
				// Determine scalar type from the pointer expression's atomic type,
				// or fall back to the value expression's type.
				scalar := w.resolveAtomicScalarFromPointer(fn, s.Pointer)
				if scalar == nil {
					scalar = w.resolveScalarFromExprType(fn, s.Value)
				}
				if scalar != nil {
					w.registerAtomicCompareExchange(*scalar)
				}
			}
		case ir.StmtBlock:
			w.scanBlockForAtomicExchange(fn, s.Block)
		case ir.StmtIf:
			w.scanBlockForAtomicExchange(fn, s.Accept)
			w.scanBlockForAtomicExchange(fn, s.Reject)
		case ir.StmtSwitch:
			for _, sc := range s.Cases {
				w.scanBlockForAtomicExchange(fn, sc.Body)
			}
		case ir.StmtLoop:
			w.scanBlockForAtomicExchange(fn, s.Body)
			w.scanBlockForAtomicExchange(fn, s.Continuing)
		}
	}
}

// typeInnerScalarAndVec extracts scalar and vector size from a TypeInner.
func typeInnerScalarAndVec(inner ir.TypeInner) (*ir.ScalarType, ir.VectorSize) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return &t, 0
	case ir.VectorType:
		return &t.Scalar, t.Size
	default:
		return nil, 0
	}
}

// resolveAtomicScalarFromPointer walks the pointer expression chain to find the
// atomic type's scalar. Returns nil if not found.
func (w *Writer) resolveAtomicScalarFromPointer(fn *ir.Function, ptrHandle ir.ExpressionHandle) *ir.ScalarType {
	if int(ptrHandle) >= len(fn.ExpressionTypes) {
		return nil
	}
	res := &fn.ExpressionTypes[ptrHandle]
	var inner ir.TypeInner
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner = w.module.Types[*res.Handle].Inner
	} else if res.Value != nil {
		inner = res.Value
	}
	if inner == nil {
		return nil
	}
	// The pointer expression should resolve to the pointed-to type.
	// For atomics, the type is Atomic { base: scalar }.
	if at, ok := inner.(ir.AtomicType); ok {
		return &at.Scalar
	}
	// For pointer types, resolve the base
	if pt, ok := inner.(ir.PointerType); ok {
		if int(pt.Base) < len(w.module.Types) {
			if at, ok := w.module.Types[pt.Base].Inner.(ir.AtomicType); ok {
				return &at.Scalar
			}
		}
	}
	return nil
}

// resolveScalarFromExprType resolves a scalar type from an expression's type resolution.
func (w *Writer) resolveScalarFromExprType(fn *ir.Function, handle ir.ExpressionHandle) *ir.ScalarType {
	if int(handle) >= len(fn.ExpressionTypes) {
		return nil
	}
	res := &fn.ExpressionTypes[handle]
	var inner ir.TypeInner
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner = w.module.Types[*res.Handle].Inner
	} else if res.Value != nil {
		inner = res.Value
	}
	if inner == nil {
		return nil
	}
	if scalar, ok := inner.(ir.ScalarType); ok {
		return &scalar
	}
	return nil
}

// writeBufferSizesStruct emits the _mslBufferSizes struct if any global
// variables contain runtime-sized arrays. Matches Rust naga output:
//
//	struct _mslBufferSizes {
//	    uint size0;
//	};
func (w *Writer) writeBufferSizesStruct() {
	vptIDs := w.vptBufferSizeMembers()
	if len(w.bufferSizeGlobals) == 0 && len(vptIDs) == 0 {
		return
	}
	w.writeLine("struct _mslBufferSizes {")
	w.pushIndent()
	for _, handle := range w.bufferSizeGlobals {
		w.writeLine("uint size%d;", handle)
	}
	for _, id := range vptIDs {
		w.writeLine("uint buffer_size%d;", id)
	}
	w.popIndent()
	w.writeLine("};")
	w.writeLine("")
}

// resolveBufferSizesBinding returns the MSL binding attribute for the _buffer_sizes
// parameter. If an explicit SizesBuffer slot is configured for the entry point,
// uses [[buffer(N)]]. If FakeMissingBindings is enabled, uses [[user(fake0)]].
// Otherwise returns an empty attribute (should not happen in valid configurations).
func (w *Writer) resolveBufferSizesBinding(epName string) string {
	if w.options.PerEntryPointMap != nil {
		if epRes, ok := w.options.PerEntryPointMap[epName]; ok {
			if epRes.SizesBuffer != nil {
				return fmt.Sprintf("[[buffer(%d)]]", *epRes.SizesBuffer)
			}
		}
	}
	if w.options.FakeMissingBindings {
		return "[[user(fake0)]]"
	}
	return ""
}

// resolveImmediatesBufferBinding returns the Metal attribute string for the immediates buffer.
func (w *Writer) resolveImmediatesBufferBinding(epName string) string {
	if w.options.PerEntryPointMap != nil {
		if epRes, ok := w.options.PerEntryPointMap[epName]; ok {
			if epRes.ImmediatesBuffer != nil {
				return fmt.Sprintf("[[buffer(%d)]]", *epRes.ImmediatesBuffer)
			}
		}
	}
	if w.options.FakeMissingBindings {
		return "[[user(fake0)]]"
	}
	return ""
}

// funcNeedsBufferSizes returns true if a helper function references any global
// variable that contains a runtime-sized array, requiring _buffer_sizes pass-through.
func (w *Writer) funcNeedsBufferSizes(handle ir.FunctionHandle) bool {
	if len(w.bufferSizeGlobals) == 0 {
		return false
	}
	globals, ok := w.funcPassThroughGlobals[handle]
	if !ok {
		return false
	}
	bufferSizeSet := make(map[uint32]struct{}, len(w.bufferSizeGlobals))
	for _, h := range w.bufferSizeGlobals {
		bufferSizeSet[h] = struct{}{}
	}
	for _, h := range globals {
		if _, needs := bufferSizeSet[h]; needs {
			return true
		}
	}
	return false
}

// writeHeader writes the MSL file header.
// Must be called after functions/entry points are processed so that
// effectiveVersion() reflects any features that require a higher Metal version.
func (w *Writer) writeHeader() {
	v := w.effectiveVersion()
	w.writeLine("// language: metal%d.%d", v.Major, v.Minor)
	w.writeLine("#include <metal_stdlib>")
	w.writeLine("#include <simd/simd.h>")
	w.writeLine("")
	w.writeLine("using metal::uint;")
	// Trailing blank line is omitted when DefaultConstructible or _RayQuery follows
	// immediately. Matches Rust naga output where the struct starts right after
	// "using metal::uint;".
	if !w.needsDefaultConstructible && !w.needsRayQuery {
		w.writeLine("")
	}
}

// writeDefaultConstructible emits the DefaultConstructible helper struct
// used by the ReadZeroSkipWrite bounds check policy. This C++14 struct can be
// implicitly converted to any default-constructible type via template parameter
// inference, allowing "DefaultConstructible()" to produce zero values for any
// type without knowing the type name. Matches Rust naga's put_default_constructible.
func (w *Writer) writeDefaultConstructible() {
	if !w.needsDefaultConstructible {
		return
	}
	w.writeLine("struct DefaultConstructible {")
	w.pushIndent()
	w.writeLine("template<typename T>")
	w.writeLine("operator T() && {")
	w.pushIndent()
	w.writeLine("return T {};")
	w.popIndent()
	w.writeLine("}")
	w.popIndent()
	w.writeLine("};")
	w.writeLine("")
}

// scanClampToEdge scans all functions for ImageSample expressions with ClampToEdge
// and external texture operations (load, sample, dimensions).
func (w *Writer) scanClampToEdge() {
	scan := func(fn *ir.Function) {
		for _, expr := range fn.Expressions {
			if sample, ok := expr.Kind.(ir.ExprImageSample); ok && sample.ClampToEdge {
				w.needsTextureSampleBaseClampToEdge = true
				// Check if the image is an external texture
				if imgType := w.resolveImageTypeFromFunc(fn, sample.Image); imgType != nil && imgType.Class == ir.ImageClassExternal {
					w.needsExternalTextureSample = true
				}
			}
			if load, ok := expr.Kind.(ir.ExprImageLoad); ok {
				if imgType := w.resolveImageTypeFromFunc(fn, load.Image); imgType != nil && imgType.Class == ir.ImageClassExternal {
					w.needsExternalTextureLoad = true
				}
			}
			if query, ok := expr.Kind.(ir.ExprImageQuery); ok {
				if _, isSizeQuery := query.Query.(ir.ImageQuerySize); isSizeQuery {
					if imgType := w.resolveImageTypeFromFunc(fn, query.Image); imgType != nil && imgType.Class == ir.ImageClassExternal {
						w.needsExternalTextureDimensions = true
					}
				}
			}
		}
	}
	for i := range w.module.Functions {
		scan(&w.module.Functions[i])
	}
	for i := range w.module.EntryPoints {
		scan(&w.module.EntryPoints[i].Function)
	}
}

// resolveImageTypeFromFunc resolves the image type of an expression within a specific function.
func (w *Writer) resolveImageTypeFromFunc(fn *ir.Function, handle ir.ExpressionHandle) *ir.ImageType {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	expr := &fn.Expressions[handle]

	// Follow FunctionArgument -> function argument type
	if arg, ok := expr.Kind.(ir.ExprFunctionArgument); ok {
		if int(arg.Index) < len(fn.Arguments) {
			tyHandle := fn.Arguments[arg.Index].Type
			if int(tyHandle) < len(w.module.Types) {
				if img, ok := w.module.Types[tyHandle].Inner.(ir.ImageType); ok {
					return &img
				}
			}
		}
		return nil
	}

	// Follow GlobalVariable -> global var type
	if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
		if int(gv.Variable) < len(w.module.GlobalVariables) {
			gvar := &w.module.GlobalVariables[gv.Variable]
			if int(gvar.Type) < len(w.module.Types) {
				if img, ok := w.module.Types[gvar.Type].Inner.(ir.ImageType); ok {
					return &img
				}
			}
		}
		return nil
	}

	return nil
}

// writeTextureSampleBaseClampToEdge emits the helper function for clamped texture sampling.
// Matches Rust naga's nagaTextureSampleBaseClampToEdge.
func (w *Writer) writeTextureSampleBaseClampToEdge() {
	if !w.needsTextureSampleBaseClampToEdge {
		return
	}
	w.writeLine("metal::float4 nagaTextureSampleBaseClampToEdge(metal::texture2d<float, metal::access::sample> tex, metal::sampler samp, metal::float2 coords) {")
	w.pushIndent()
	w.writeLine("metal::float2 half_texel = 0.5 / metal::float2(tex.get_width(0u), tex.get_height(0u));")
	w.writeLine("return tex.sample(samp, metal::clamp(coords, half_texel, 1.0 - half_texel), metal::level(0.0));")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeExternalTextureSampleBaseClampToEdge emits the external texture version of
// nagaTextureSampleBaseClampToEdge. Handles multi-plane YUV sampling with transfer functions.
func (w *Writer) writeExternalTextureSampleBaseClampToEdge() {
	w.write("float4 nagaTextureSampleBaseClampToEdge(NagaExternalTextureWrapper tex, %ssampler samp, float2 coords) {\n", Namespace)
	l1, l2, l3 := "    ", "        ", "            "
	w.write("%suint2 plane0_size = uint2(tex.plane0.get_width(), tex.plane0.get_height());\n", l1)
	w.write("%scoords = tex.params.sample_transform * float3(coords, 1.0);\n", l1)
	w.write("%sfloat2 bounds_min = tex.params.sample_transform * float3(0.0, 0.0, 1.0);\n", l1)
	w.write("%sfloat2 bounds_max = tex.params.sample_transform * float3(1.0, 1.0, 1.0);\n", l1)
	w.write("%sfloat4 bounds = float4(%smin(bounds_min, bounds_max), %smax(bounds_min, bounds_max));\n", l1, Namespace, Namespace)
	w.write("%sfloat2 plane0_half_texel = float2(0.5, 0.5) / float2(plane0_size);\n", l1)
	w.write("%sfloat2 plane0_coords = %sclamp(coords, bounds.xy + plane0_half_texel, bounds.zw - plane0_half_texel);\n", l1, Namespace)
	w.write("%sif (tex.params.num_planes == 1u) {\n", l1)
	w.write("%sreturn tex.plane0.sample(samp, plane0_coords, %slevel(0.0f));\n", l2, Namespace)
	w.write("%s} else {\n", l1)
	w.write("%suint2 plane1_size = uint2(tex.plane1.get_width(), tex.plane1.get_height());\n", l2)
	w.write("%sfloat2 plane1_half_texel = float2(0.5, 0.5) / float2(plane1_size);\n", l2)
	w.write("%sfloat2 plane1_coords = %sclamp(coords, bounds.xy + plane1_half_texel, bounds.zw - plane1_half_texel);\n", l2, Namespace)
	w.write("%sfloat y = tex.plane0.sample(samp, plane0_coords, %slevel(0.0f)).r;\n", l2, Namespace)
	w.write("%sfloat2 uv = float2(0.0, 0.0);\n", l2)
	w.write("%sif (tex.params.num_planes == 2u) {\n", l2)
	w.write("%suv = tex.plane1.sample(samp, plane1_coords, %slevel(0.0f)).xy;\n", l3, Namespace)
	w.write("%s} else {\n", l2)
	w.write("%suint2 plane2_size = uint2(tex.plane2.get_width(), tex.plane2.get_height());\n", l3)
	w.write("%sfloat2 plane2_half_texel = float2(0.5, 0.5) / float2(plane2_size);\n", l3)
	w.write("%sfloat2 plane2_coords = %sclamp(coords, bounds.xy + plane2_half_texel, bounds.zw - plane1_half_texel);\n", l3, Namespace)
	w.write("%suv.x = tex.plane1.sample(samp, plane1_coords, %slevel(0.0f)).x;\n", l3, Namespace)
	w.write("%suv.y = tex.plane2.sample(samp, plane2_coords, %slevel(0.0f)).x;\n", l3, Namespace)
	w.write("%s}\n", l2)
	w.writeConvertYuvToRgbAndReturn(l2)
	w.write("%s}\n", l1)
	w.write("}\n\n")
}

// writeExternalTextureLoadHelper emits the nagaTextureLoadExternal helper function.
func (w *Writer) writeExternalTextureLoadHelper() {
	w.write("float4 nagaTextureLoadExternal(NagaExternalTextureWrapper tex, uint2 coords) {\n")
	l1, l2, l3 := "    ", "        ", "            "
	w.write("%suint2 plane0_size = uint2(tex.plane0.get_width(), tex.plane0.get_height());\n", l1)
	w.write("%suint2 cropped_size = %sany(tex.params.size != 0) ? tex.params.size : plane0_size;\n", l1, Namespace)
	w.write("%scoords = %smin(coords, cropped_size - 1);\n", l1, Namespace)
	w.write("%suint2 plane0_coords = uint2(%sround(tex.params.load_transform * float3(float2(coords), 1.0)));\n", l1, Namespace)
	w.write("%sif (tex.params.num_planes == 1u) {\n", l1)
	w.write("%sreturn tex.plane0.read(plane0_coords);\n", l2)
	w.write("%s} else {\n", l1)
	w.write("%suint2 plane1_size = uint2(tex.plane1.get_width(), tex.plane1.get_height());\n", l2)
	w.write("%suint2 plane1_coords = uint2(%sfloor(float2(plane0_coords) * float2(plane1_size) / float2(plane0_size)));\n", l2, Namespace)
	w.write("%sfloat y = tex.plane0.read(plane0_coords).x;\n", l2)
	w.write("%sfloat2 uv;\n", l2)
	w.write("%sif (tex.params.num_planes == 2u) {\n", l2)
	w.write("%suv = tex.plane1.read(plane1_coords).xy;\n", l3)
	w.write("%s} else {\n", l2)
	w.write("%suint2 plane2_size = uint2(tex.plane2.get_width(), tex.plane2.get_height());\n", l2)
	w.write("%suint2 plane2_coords = uint2(%sfloor(float2(plane0_coords) * float2(plane2_size) / float2(plane0_size)));\n", l2, Namespace)
	w.write("%suv = float2(tex.plane1.read(plane1_coords).x, tex.plane2.read(plane2_coords).x);\n", l3)
	w.write("%s}\n", l2)
	w.writeConvertYuvToRgbAndReturn(l2)
	w.write("%s}\n", l1)
	w.write("}\n\n")
}

// writeExternalTextureDimensionsHelper emits the nagaTextureDimensionsExternal helper.
func (w *Writer) writeExternalTextureDimensionsHelper() {
	w.write("uint2 nagaTextureDimensionsExternal(NagaExternalTextureWrapper tex) {\n")
	l1, l2 := "    ", "        "
	w.write("%sif (%sany(tex.params.size != uint2(0u))) {\n", l1, Namespace)
	w.write("%sreturn tex.params.size;\n", l2)
	w.write("%s} else {\n", l1)
	w.write("%sreturn uint2(tex.plane0.get_width(), tex.plane0.get_height());\n", l2)
	w.write("%s}\n", l1)
	w.write("}\n\n")
}

// writeConvertYuvToRgbAndReturn writes the YUV-to-RGB conversion and return statement.
func (w *Writer) writeConvertYuvToRgbAndReturn(l1 string) {
	l2 := l1 + "    "
	w.write("%sfloat3 srcGammaRgb = (tex.params.yuv_conversion_matrix * float4(y, uv, 1.0)).rgb;\n", l1)
	w.write("%sfloat3 srcLinearRgb = %sselect(\n", l1, Namespace)
	w.write("%s%spow((srcGammaRgb + tex.params.src_tf.a - 1.0) / tex.params.src_tf.a, tex.params.src_tf.g),\n", l2, Namespace)
	w.write("%ssrcGammaRgb / tex.params.src_tf.k,\n", l2)
	w.write("%ssrcGammaRgb < tex.params.src_tf.k * tex.params.src_tf.b);\n", l2)
	w.write("%sfloat3 dstLinearRgb = tex.params.gamut_conversion_matrix * srcLinearRgb;\n", l1)
	w.write("%sfloat3 dstGammaRgb = %sselect(\n", l1, Namespace)
	w.write("%stex.params.dst_tf.a * %spow(dstLinearRgb, 1.0 / tex.params.dst_tf.g) - (tex.params.dst_tf.a - 1),\n", l2, Namespace)
	w.write("%stex.params.dst_tf.k * dstLinearRgb,\n", l2)
	w.write("%sdstLinearRgb < tex.params.dst_tf.b);\n", l2)
	w.write("%sreturn float4(dstGammaRgb, 1.0);\n", l1)
}

// registerNames assigns unique names to all IR entities.
//
// registerNames pre-registers ALL names for the entire module before any code
// generation, matching Rust naga's namer.reset() behavior (namer.rs).
//
// Order matches Rust exactly:
//  1. Types (with struct member namespaces)
//  2. Entry points (name + arguments + locals)
//  3. Functions (name + arguments + locals)
//  4. Global variables
//  5. Constants (with const_{type_name} fallback for unnamed)
//
// Pre-registering all names ensures the namer's unique counter sees every name
// before output structs or other generated names are created. This prevents
// suffix ordering mismatches (e.g., a local var "vsOutput" and output struct
// "vsOutput" getting different _1 suffixes depending on registration order).
func (w *Writer) registerNames() error {
	// Entry point functions are stored inline in EntryPoints, not in Functions[].
	// No need for an entryPointFuncNames map to skip them.

	// 1. Register type names.
	// Types with built-in MSL names (scalars, vectors, matrices) use "type" as
	// their base name, matching Rust naga where these types are unnamed in the
	// arena. Named struct/array types use their WGSL name.
	for handle, typ := range w.module.Types {
		var baseName string
		if typ.Name != "" {
			baseName = typ.Name
		} else {
			baseName = "type"
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyType, handle1: uint32(handle)}] = name
		w.typeNames[ir.TypeHandle(handle)] = name

		// Register struct member names using a fresh namespace scope.
		// Struct members only need unique names among themselves (not globally),
		// matching Rust naga's namer.namespace() + call_or() behavior.
		if st, ok := typ.Inner.(ir.StructType); ok {
			memberNamer := newNamer()
			for memberIdx, member := range st.Members {
				memberName := member.Name
				if memberName == "" {
					memberName = "member"
				}
				mname := memberNamer.call(memberName)
				w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}] = mname
			}
		}
	}

	// 1b. Register type alias names so the namer detects collisions.
	// In Rust naga, type aliases create type entries with their name,
	// which the namer processes along with other types.
	for _, aliasName := range w.module.TypeAliasNames {
		w.namer.call(aliasName)
	}

	// 2. Register entry point names, arguments, and locals.
	// Rust naga registers entry points BEFORE regular functions.
	for epIdx, ep := range w.module.EntryPoints {
		epName := w.namer.call(ep.Name)
		w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}] = epName
		w.entryPointNames[ep.Name] = epName

		// Also register as function name for lookup via nameKeyFunction
		// using synthetic handle (entry points are not in Functions[]).
		syntheticHandle := epFuncHandle(epIdx)
		w.names[nameKey{kind: nameKeyFunction, handle1: uint32(syntheticHandle)}] = epName

		fn := &ep.Function

		// Register entry point argument names via namer.call to match Rust
		// namer.call_or(&arg.name, "param") behavior.
		for argIdx, arg := range fn.Arguments {
			argBase := arg.Name
			if argBase == "" {
				argBase = "param"
			}
			argName := w.namer.call(argBase)
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(syntheticHandle), handle2: uint32(argIdx)}] = argName
		}

		// Register entry point local variable names.
		// This is critical: locals must be registered before output struct
		// naming so the namer knows about conflicting names.
		for localIdx, local := range fn.LocalVars {
			localBase := local.Name
			if localBase == "" {
				localBase = "local"
			}
			localName := w.namer.call(localBase)
			w.names[nameKey{kind: nameKeyLocal, handle1: uint32(syntheticHandle), handle2: uint32(localIdx)}] = localName
		}
	}

	// 3. Register regular (non-entry-point) function names, arguments, and locals.
	// All functions in Module.Functions[] are regular functions (entry points
	// are stored inline in EntryPoints[]).
	for idx := range w.module.Functions {
		handle := ir.FunctionHandle(idx)

		fn := &w.module.Functions[idx]
		baseName := fn.Name
		if baseName == "" {
			baseName = "function"
		}
		funcName := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}] = funcName

		// Register argument names via namer.call to match Rust.
		for argIdx, arg := range fn.Arguments {
			argBase := arg.Name
			if argBase == "" {
				argBase = "param"
			}
			argName := w.namer.call(argBase)
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}] = argName
		}

		// Register local variable names.
		for localIdx, local := range fn.LocalVars {
			localBase := local.Name
			if localBase == "" {
				localBase = "local"
			}
			localName := w.namer.call(localBase)
			w.names[nameKey{kind: nameKeyLocal, handle1: uint32(handle), handle2: uint32(localIdx)}] = localName
		}
	}

	// 4. Register global variable names.
	for handle, global := range w.module.GlobalVariables {
		var baseName string
		if global.Name != "" {
			baseName = global.Name
		} else {
			baseName = "global"
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}] = name

		// For external texture globals, register plane and params names.
		// Matches Rust naga namer.rs: format!("{base}_{suffix}") where
		// suffix is _plane0, _plane1, _plane2, _params.
		if int(global.Type) < len(w.module.Types) {
			if imgType, isImg := w.module.Types[global.Type].Inner.(ir.ImageType); isImg {
				if imgType.Class == ir.ImageClassExternal {
					base := global.Name
					if base == "" {
						base = "global"
					}
					// Rust: self.call(&format!("{base}_{suffix}"))
					// suffix includes leading underscore, e.g. "_plane0"
					// So we get "tex__plane0" which sanitizeName collapses to "tex_plane0"
					// then namer.call adds trailing _ for digit-ending names.
					w.names[nameKey{kind: nameKeyExternalTexturePlane0, handle1: uint32(handle)}] = w.namer.call(base + "__plane0")
					w.names[nameKey{kind: nameKeyExternalTexturePlane1, handle1: uint32(handle)}] = w.namer.call(base + "__plane1")
					w.names[nameKey{kind: nameKeyExternalTexturePlane2, handle1: uint32(handle)}] = w.namer.call(base + "__plane2")
					w.names[nameKey{kind: nameKeyExternalTextureParams, handle1: uint32(handle)}] = w.namer.call(base + "__params")
				}
			}
		}
	}

	// 5. Register constant names.
	// Rust uses const_{type_name} as fallback for unnamed constants.
	for handle, constant := range w.module.Constants {
		var baseName string
		if constant.Name != "" {
			baseName = constant.Name
		} else {
			typeName := w.names[nameKey{kind: nameKeyType, handle1: uint32(constant.Type)}]
			baseName = fmt.Sprintf("const_%s", typeName)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}] = name
	}

	// 6. Register override names.
	// Overrides are written as MSL constants after process_overrides.
	for handle, ov := range w.module.Overrides {
		baseName := ov.Name
		if baseName == "" {
			baseName = fmt.Sprintf("override_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyOverride, handle1: uint32(handle)}] = name
	}

	return nil
}

// Output helpers

// write writes text to the output. If args are provided, uses fmt.Fprintf.
//
//nolint:goprintffuncname
func (w *Writer) write(format string, args ...any) {
	if len(args) == 0 {
		w.out.WriteString(format)
	} else {
		fmt.Fprintf(&w.out, format, args...)
	}
}

// writeLine writes a line with optional format args and a newline.
//
//nolint:goprintffuncname
func (w *Writer) writeLine(format string, args ...any) {
	w.writeIndent()
	if len(args) == 0 {
		w.out.WriteString(format)
	} else {
		fmt.Fprintf(&w.out, format, args...)
	}
	w.out.WriteByte('\n')
}

// writeIndent writes the current indentation.
func (w *Writer) writeIndent() {
	for i := 0; i < w.indent; i++ {
		w.out.WriteString("    ")
	}
}

// pushIndent increases indentation.
func (w *Writer) pushIndent() {
	w.indent++
}

// popIndent decreases indentation.
func (w *Writer) popIndent() {
	if w.indent > 0 {
		w.indent--
	}
}

// writeHelperFunctions writes typed naga_div/naga_mod overloads in first-use order.
// Rust naga emits per-type overloads using metal::select, not C++ templates.
// Overloads are written in the order they were first encountered during expression
// writing, matching Rust naga's write_wrapped_binary_op behavior.
func (w *Writer) writeHelperFunctions() {
	if len(w.helperOverloads) == 0 && len(w.dotWrappers) == 0 && len(w.absHelpers) == 0 {
		return
	}

	for _, o := range w.helperOverloads {
		typeName := o.mslTypeName()
		if o.isDiv {
			w.writeLine("%s naga_div(%s lhs, %s rhs) {", typeName, typeName, typeName)
			w.pushIndent()
			switch o.kind {
			case ir.ScalarSint:
				minVal := sintMinLiteral(o.width)
				w.writeLine("return lhs / metal::select(rhs, 1, (lhs == %s & rhs == -1) | (rhs == 0));", minVal)
			case ir.ScalarUint:
				w.writeLine("return lhs / metal::select(rhs, 1u, rhs == 0u);")
			}
			w.popIndent()
		} else {
			w.writeLine("%s naga_mod(%s lhs, %s rhs) {", typeName, typeName, typeName)
			w.pushIndent()
			switch o.kind {
			case ir.ScalarSint:
				minVal := sintMinLiteral(o.width)
				w.writeLine("%s divisor = metal::select(rhs, 1, (lhs == %s & rhs == -1) | (rhs == 0));", typeName, minVal)
				w.writeLine("return lhs - (lhs / divisor) * divisor;")
			case ir.ScalarUint:
				w.writeLine("return lhs %s metal::select(rhs, 1u, rhs == 0u);", "%")
			}
			w.popIndent()
		}
		w.writeLine("}")
		// Rust naga emits a trailing blank line after each helper function.
		w.writeLine("")
	}

	// Abs helpers for signed integers (emitted before dot wrappers to match Rust ordering).
	// Rust naga emits: T naga_abs(T val) { return metal::select(as_type<T>(-as_type<U>(val)), val, val >= 0); }
	for _, a := range w.absHelpers {
		typeName := scalarTypeName(a.scalar)
		unsignedScalar := ir.ScalarType{Kind: ir.ScalarUint, Width: a.scalar.Width}
		unsignedName := scalarTypeName(unsignedScalar)
		if a.vecSize > 0 {
			typeName = fmt.Sprintf("%s%s%d", Namespace, typeName, a.vecSize)
			unsignedName = fmt.Sprintf("%s%s%d", Namespace, unsignedName, a.vecSize)
		}
		w.writeLine("%s naga_abs(%s val) {", typeName, typeName)
		w.pushIndent()
		w.writeLine("return %sselect(as_type<%s>(-as_type<%s>(val)), val, val >= 0);", Namespace, typeName, unsignedName)
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}

	// Integer dot product wrappers: naga_dot_{type}{size}
	// MSL metal::dot doesn't support int/uint vectors, so we emit manual wrappers.
	components := [4]string{"x", "y", "z", "w"}
	for _, d := range w.dotWrappers {
		retType := scalarTypeName(d.scalar)
		vecType := fmt.Sprintf("%s%s%d", Namespace, retType, d.size)
		w.writeLine("%s %s(%s a, %s b) {", retType, d.name, vecType, vecType)
		w.pushIndent()
		w.writeIndent()
		w.write("return (")
		for i := ir.VectorSize(0); i < d.size; i++ {
			w.write(" + a.%s * b.%s", components[i], components[i])
		}
		w.write(");\n")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}
}

// writeHelperSubset writes a subset of helper overloads and dot wrappers.
// Used for demand-driven helper emission between functions.
func (w *Writer) writeHelperSubset(overloads []divModOverload, dots []dotWrapper) {
	for _, o := range overloads {
		typeName := o.mslTypeName()
		if o.isDiv {
			w.writeLine("%s naga_div(%s lhs, %s rhs) {", typeName, typeName, typeName)
			w.pushIndent()
			switch o.kind {
			case ir.ScalarSint:
				minVal := sintMinLiteral(o.width)
				w.writeLine("return lhs / metal::select(rhs, 1, (lhs == %s & rhs == -1) | (rhs == 0));", minVal)
			case ir.ScalarUint:
				w.writeLine("return lhs / metal::select(rhs, 1u, rhs == 0u);")
			}
			w.popIndent()
		} else {
			w.writeLine("%s naga_mod(%s lhs, %s rhs) {", typeName, typeName, typeName)
			w.pushIndent()
			switch o.kind {
			case ir.ScalarSint:
				minVal := sintMinLiteral(o.width)
				w.writeLine("%s divisor = metal::select(rhs, 1, (lhs == %s & rhs == -1) | (rhs == 0));", typeName, minVal)
				w.writeLine("return lhs - (lhs / divisor) * divisor;")
			case ir.ScalarUint:
				w.writeLine("return lhs %s metal::select(rhs, 1u, rhs == 0u);", "%")
			}
			w.popIndent()
		}
		w.writeLine("}")
		w.writeLine("")
	}

	components := [4]string{"x", "y", "z", "w"}
	for _, d := range dots {
		retType := scalarTypeName(d.scalar)
		vecType := fmt.Sprintf("%s%s%d", Namespace, retType, d.size)
		w.writeLine("%s %s(%s a, %s b) {", retType, d.name, vecType, vecType)
		w.pushIndent()
		w.writeIndent()
		w.write("return (")
		for i := ir.VectorSize(0); i < d.size; i++ {
			w.write(" + a.%s * b.%s", components[i], components[i])
		}
		w.write(");\n")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}
}

// writeHelperSubsetWithAbs writes helpers in the Rust naga order: divmod, abs, dot.
func (w *Writer) writeHelperSubsetWithAbs(overloads []divModOverload, absHelpers []absHelper, dots []dotWrapper) {
	w.writeHelperSubsetDivMod(overloads)
	w.writeHelperSubsetAbs(absHelpers)
	w.writeHelperSubsetDot(dots)
}

func (w *Writer) writeHelperSubsetDivMod(overloads []divModOverload) {
	for _, o := range overloads {
		typeName := o.mslTypeName()
		if o.isDiv {
			w.writeLine("%s naga_div(%s lhs, %s rhs) {", typeName, typeName, typeName)
			w.pushIndent()
			switch o.kind {
			case ir.ScalarSint:
				minVal := sintMinLiteral(o.width)
				w.writeLine("return lhs / metal::select(rhs, 1, (lhs == %s & rhs == -1) | (rhs == 0));", minVal)
			case ir.ScalarUint:
				w.writeLine("return lhs / metal::select(rhs, 1u, rhs == 0u);")
			}
			w.popIndent()
		} else {
			w.writeLine("%s naga_mod(%s lhs, %s rhs) {", typeName, typeName, typeName)
			w.pushIndent()
			switch o.kind {
			case ir.ScalarSint:
				minVal := sintMinLiteral(o.width)
				w.writeLine("%s divisor = metal::select(rhs, 1, (lhs == %s & rhs == -1) | (rhs == 0));", typeName, minVal)
				w.writeLine("return lhs - (lhs / divisor) * divisor;")
			case ir.ScalarUint:
				w.writeLine("return lhs %s metal::select(rhs, 1u, rhs == 0u);", "%")
			}
			w.popIndent()
		}
		w.writeLine("}")
		w.writeLine("")
	}
}

func (w *Writer) writeHelperSubsetAbs(helpers []absHelper) {
	for _, a := range helpers {
		typeName := scalarTypeName(a.scalar)
		unsignedScalar := ir.ScalarType{Kind: ir.ScalarUint, Width: a.scalar.Width}
		unsignedName := scalarTypeName(unsignedScalar)
		if a.vecSize > 0 {
			typeName = fmt.Sprintf("%s%s%d", Namespace, typeName, a.vecSize)
			unsignedName = fmt.Sprintf("%s%s%d", Namespace, unsignedName, a.vecSize)
		}
		w.writeLine("%s naga_abs(%s val) {", typeName, typeName)
		w.pushIndent()
		w.writeLine("return metal::select(as_type<%s>(-as_type<%s>(val)), val, val >= 0);", typeName, unsignedName)
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}
}

func (w *Writer) writeHelperSubsetDot(dots []dotWrapper) {
	components := [4]string{"x", "y", "z", "w"}
	for _, d := range dots {
		retType := scalarTypeName(d.scalar)
		vecType := fmt.Sprintf("%s%s%d", Namespace, retType, d.size)
		w.writeLine("%s %s(%s a, %s b) {", retType, d.name, vecType, vecType)
		w.pushIndent()
		w.writeIndent()
		w.write("return (")
		for i := ir.VectorSize(0); i < d.size; i++ {
			w.write(" + a.%s * b.%s", components[i], components[i])
		}
		w.write(");\n")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}
}

// sintMinLiteral returns the MSL literal for the minimum value of a signed integer.
// Uses the (-MAX - 1) pattern to avoid literal overflow warnings in Metal compilers.
func sintMinLiteral(width uint8) string {
	switch width {
	case 4:
		return "(-2147483647 - 1)"
	case 8:
		return "(-9223372036854775807L - 1L)"
	default:
		return "(-2147483647 - 1)"
	}
}

// addDivOverload registers a typed div helper overload if not already registered.
func (w *Writer) addDivOverload(o divModOverload) {
	o.isDiv = true
	for _, existing := range w.helperOverloads {
		if existing == o {
			return
		}
	}
	w.helperOverloads = append(w.helperOverloads, o)
}

// addModOverload registers a typed mod helper overload if not already registered.
func (w *Writer) addModOverload(o divModOverload) {
	o.isDiv = false
	for _, existing := range w.helperOverloads {
		if existing == o {
			return
		}
	}
	w.helperOverloads = append(w.helperOverloads, o)
}

// registerDotWrapper registers an integer dot product wrapper function and returns its name.
// Format: naga_dot_{type}{size} (e.g., naga_dot_int2, naga_dot_uint3).
func (w *Writer) registerDotWrapper(scalar ir.ScalarType, size ir.VectorSize) string {
	typeName := scalarTypeName(scalar)
	name := fmt.Sprintf("naga_dot_%s%d", typeName, size)
	for _, existing := range w.dotWrappers {
		if existing.name == name {
			return name
		}
	}
	w.dotWrappers = append(w.dotWrappers, dotWrapper{scalar: scalar, size: size, name: name})
	return name
}

// absHelper tracks a signed integer abs helper function variant.
type absHelper struct {
	scalar  ir.ScalarType
	vecSize ir.VectorSize // 0 for scalar
}

// registerAbsHelper registers a naga_abs helper function for the given scalar/vector type.
func (w *Writer) registerAbsHelper(scalar ir.ScalarType, vecSize ir.VectorSize) {
	for _, existing := range w.absHelpers {
		if existing.scalar == scalar && existing.vecSize == vecSize {
			return
		}
	}
	w.absHelpers = append(w.absHelpers, absHelper{scalar: scalar, vecSize: vecSize})
}

// writeNegHelper emits a naga_neg function for signed integer negation.
// Matches Rust: T naga_neg(T val) { return as_type<T>(-as_type<unsigned_T>(val)); }
func (w *Writer) writeNegHelper(h absHelper) {
	typeName := scalarTypeName(h.scalar)
	unsignedName := scalarTypeName(ir.ScalarType{Kind: ir.ScalarUint, Width: h.scalar.Width})
	if h.vecSize > 0 {
		typeName = fmt.Sprintf("%s%s%d", Namespace, typeName, h.vecSize)
		unsignedName = fmt.Sprintf("%s%s%d", Namespace, unsignedName, h.vecSize)
	}
	w.writeLine("%s naga_neg(%s val) {", typeName, typeName)
	w.pushIndent()
	w.writeLine("return as_type<%s>(-as_type<%s>(val));", typeName, unsignedName)
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// registerNegHelper registers a naga_neg helper function for signed integer negation.
// Avoids UB on INT_MIN by casting to unsigned, negating, then casting back.
func (w *Writer) registerNegHelper(scalar ir.ScalarType, vecSize ir.VectorSize) {
	for _, existing := range w.negHelpers {
		if existing.scalar == scalar && existing.vecSize == vecSize {
			return
		}
	}
	w.negHelpers = append(w.negHelpers, absHelper{scalar: scalar, vecSize: vecSize})
}

// getTypeName returns the MSL type name for a type handle.
func (w *Writer) getTypeName(handle ir.TypeHandle) string {
	if name, ok := w.typeNames[handle]; ok {
		return name
	}
	return fmt.Sprintf("type_%d", handle)
}

// analyzeGlobalWriteUsage scans all functions and entry points to determine
// which global variables are written to (via Store/Atomic statements and
// transitive calls). Builds both module-wide and per-function write usage maps.
// Matches Rust naga's per-function GlobalUse::WRITE analysis for const qualifier.
func (w *Writer) analyzeGlobalWriteUsage() {
	w.globalWriteUsage = make(map[uint32]struct{})
	w.perFuncWriteUsage = make(map[int]map[uint32]struct{})

	// Phase 1: Direct writes per function
	funcCalls := make(map[int][]int) // function -> called functions
	for i := range w.module.Functions {
		fn := &w.module.Functions[i]
		funcWrites := make(map[uint32]struct{})
		w.scanBlockForWritesPerFunc(fn.Body, fn.Expressions, funcWrites)
		w.perFuncWriteUsage[i] = funcWrites
		// Also collect call graph for propagation
		calls := w.collectCalls(fn.Body, fn.Expressions)
		if len(calls) > 0 {
			funcCalls[i] = calls
		}
	}

	// Entry point functions (stored inline, not in Functions[])
	for epIdx := range w.module.EntryPoints {
		fn := &w.module.EntryPoints[epIdx].Function
		key := int(epFuncHandle(epIdx))
		funcWrites := make(map[uint32]struct{})
		w.scanBlockForWritesPerFunc(fn.Body, fn.Expressions, funcWrites)
		w.perFuncWriteUsage[key] = funcWrites
		calls := w.collectCalls(fn.Body, fn.Expressions)
		if len(calls) > 0 {
			funcCalls[key] = calls
		}
	}

	// Phase 2: Propagate writes from callees to callers (fixed-point).
	// A function that calls another function inherits all of its callee's writes.
	changed := true
	for changed {
		changed = false
		for caller, callees := range funcCalls {
			for _, callee := range callees {
				if calleeWrites, ok := w.perFuncWriteUsage[callee]; ok {
					for globalHandle := range calleeWrites {
						if _, exists := w.perFuncWriteUsage[caller][globalHandle]; !exists {
							w.perFuncWriteUsage[caller][globalHandle] = struct{}{}
							changed = true
						}
					}
				}
			}
		}
	}

	// Phase 3: Build module-wide map
	for _, funcWrites := range w.perFuncWriteUsage {
		for k := range funcWrites {
			w.globalWriteUsage[k] = struct{}{}
		}
	}
}

// collectCalls collects all function handles called from a block.
func (w *Writer) collectCalls(block ir.Block, expressions []ir.Expression) []int {
	var calls []int
	w.collectCallsFromBlock(block, expressions, &calls)
	return calls
}

func (w *Writer) collectCallsFromBlock(block ir.Block, expressions []ir.Expression, calls *[]int) {
	for _, stmt := range block {
		switch s := stmt.Kind.(type) {
		case ir.StmtCall:
			*calls = append(*calls, int(s.Function))
		case ir.StmtBlock:
			w.collectCallsFromBlock(s.Block, expressions, calls)
		case ir.StmtIf:
			w.collectCallsFromBlock(s.Accept, expressions, calls)
			w.collectCallsFromBlock(s.Reject, expressions, calls)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				w.collectCallsFromBlock(c.Body, expressions, calls)
			}
		case ir.StmtLoop:
			w.collectCallsFromBlock(s.Body, expressions, calls)
			w.collectCallsFromBlock(s.Continuing, expressions, calls)
		}
	}
}

// scanBlockForWritesPerFunc recursively scans a block for Store statements that
// write to global variables, recording into the given funcWrites map.
func (w *Writer) scanBlockForWritesPerFunc(block ir.Block, expressions []ir.Expression, funcWrites map[uint32]struct{}) {
	for _, stmt := range block {
		switch s := stmt.Kind.(type) {
		case ir.StmtStore:
			w.markGlobalWritePerFunc(s.Pointer, expressions, funcWrites)
		case ir.StmtAtomic:
			w.markGlobalWritePerFunc(s.Pointer, expressions, funcWrites)
		case ir.StmtBlock:
			w.scanBlockForWritesPerFunc(s.Block, expressions, funcWrites)
		case ir.StmtIf:
			w.scanBlockForWritesPerFunc(s.Accept, expressions, funcWrites)
			w.scanBlockForWritesPerFunc(s.Reject, expressions, funcWrites)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				w.scanBlockForWritesPerFunc(c.Body, expressions, funcWrites)
			}
		case ir.StmtLoop:
			w.scanBlockForWritesPerFunc(s.Body, expressions, funcWrites)
			w.scanBlockForWritesPerFunc(s.Continuing, expressions, funcWrites)
		}
	}
}

// markGlobalWritePerFunc checks if an expression ultimately refers to a global
// variable and marks it as written in the given funcWrites map.
func (w *Writer) markGlobalWritePerFunc(handle ir.ExpressionHandle, expressions []ir.Expression, funcWrites map[uint32]struct{}) {
	if int(handle) >= len(expressions) {
		return
	}
	expr := &expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		funcWrites[uint32(k.Variable)] = struct{}{}
	case ir.ExprAccess:
		w.markGlobalWritePerFunc(k.Base, expressions, funcWrites)
	case ir.ExprAccessIndex:
		w.markGlobalWritePerFunc(k.Base, expressions, funcWrites)
	}
}

// isStorageGlobalReadOnly returns true if a storage global variable is not
// written to by the current function. Matches Rust naga's per-function
// GlobalUse::WRITE analysis: each function parameter gets const based on
// whether THAT function writes to the global, not module-wide.
func (w *Writer) isStorageGlobalReadOnly(handle uint32) bool {
	global := &w.module.GlobalVariables[handle]
	if global.Space != ir.SpaceStorage {
		return false
	}
	// Check per-function write usage (matches Rust naga's per-function analysis).
	if funcWrites, ok := w.perFuncWriteUsage[int(w.currentFuncHandle)]; ok {
		_, isWritten := funcWrites[handle]
		return !isWritten
	}
	// Fallback to module-wide analysis if per-function data not available.
	_, isWritten := w.globalWriteUsage[handle]
	return !isWritten
}

// getOutputStructName returns the MSL struct name for the current entry point's output.
func (w *Writer) getOutputStructName() string {
	return w.entryPointOutputStructName
}

// getName returns the registered name for a name key.
func (w *Writer) getName(key nameKey) string {
	if name, ok := w.names[key]; ok {
		return name
	}
	return fmt.Sprintf("unnamed_%d_%d", key.kind, key.handle1)
}
