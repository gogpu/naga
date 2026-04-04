// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gogpu/naga/ir"
)

// nameKey identifies an IR entity for name lookup.
type nameKey struct {
	kind    nameKeyKind
	handle1 uint32
	handle2 uint32
}

// nameKeyKind identifies the type of IR entity.
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
)

// epFuncHandle returns a synthetic FunctionHandle for entry point index epIdx.
// Entry point functions are stored inline in EntryPoint.Function, not in Module.Functions[].
const entryPointHandleBase = ir.FunctionHandle(0x80000000)

func epFuncHandle(epIdx int) ir.FunctionHandle {
	return entryPointHandleBase + ir.FunctionHandle(epIdx)
}

// Io distinguishes input from output in entry point interfaces.
type Io int

const (
	// IoInput marks entry point inputs.
	IoInput Io = iota
	// IoOutput marks entry point outputs.
	IoOutput
)

// entryPointInterface stores the input/output struct bindings for an entry point.
type entryPointInterface struct {
	input  *entryPointBinding
	output *entryPointBinding
}

// entryPointBinding stores the struct name and argument name for an EP I/O struct.
type entryPointBinding struct {
	tyName  string
	argName string
	members []epStructMember
}

// epStructMember represents a flattened member of an EP I/O struct.
type epStructMember struct {
	name    string
	ty      ir.TypeHandle
	binding *ir.Binding
	index   uint32 // original index for re-ordering
}

// Writer generates HLSL source code from IR.
type Writer struct {
	module  *ir.Module
	options *Options

	// Output buffer
	out strings.Builder

	// Current indentation level
	indent int

	// Name management
	names map[nameKey]string
	namer *namer

	// Type tracking
	typeNames map[ir.TypeHandle]string

	// Tracks which struct types are used as EP results, and their stage.
	// Key: TypeHandle, Value: (stage, Io) pair for semantic writing.
	epResultTypes map[ir.TypeHandle]epResultInfo

	// Entry point I/O interface structs (indexed by EP index)
	entryPointIO map[int]*entryPointInterface

	// Function context (set during function writing)
	currentFunction     *ir.Function
	currentFuncHandle   ir.FunctionHandle
	currentEPIndex      int // -1 for regular functions, >= 0 for entry points
	localNames          map[uint32]string
	namedExpressions    map[ir.ExpressionHandle]string
	needBakeExpressions map[ir.ExpressionHandle]struct{}

	// Output tracking
	entryPointNames     map[string]string
	registerBindings    map[string]string
	helperFunctions     []string
	usedFeatures        FeatureFlags
	requiredShaderModel ShaderModel

	// Helper function flags
	needsModHelper          bool
	needsDivHelper          bool
	needsAbsHelper          bool
	needsStorageLoadHelpers map[string]bool // scalar type names needing LoadedStorageValueFrom

	// Struct constructor tracking
	structConstructors        map[ir.TypeHandle]struct{}
	structConstructorsWritten map[ir.TypeHandle]struct{}

	// Array constructor tracking
	arrayConstructorsWritten map[ir.TypeHandle]struct{}

	// ZeroValue wrapper function tracking
	// Maps type handle -> whether we've already written the wrapper
	wrappedZeroValues map[ir.TypeHandle]struct{}

	// Storage access chain for ByteAddressBuffer Load/Store.
	// Populated by fillAccessChain, consumed by writeStorageAddress/writeStorageLoad/writeStorageStore.
	tempAccessChain []subAccess

	// Tracks which wrapped binary op helpers (naga_div, naga_mod) have been written.
	wrappedBinaryOps map[wrappedBinaryOpKey]struct{}

	// Tracks which naga_neg helpers have been written (keyed by HLSL type string).
	wrappedNegOps map[string]struct{}

	// Tracks which naga_modf/naga_frexp helpers have been written (keyed by result struct name).
	wrappedMathHelpers map[string]struct{}

	// Float-to-int clamped cast helpers (naga_f2i32, naga_f2u32, naga_f2i64, naga_f2u64)
	needsF2ICast     bool
	f2iCastFunctions map[string]struct{}
	f2iCastWritten   map[f2iCastKey]struct{}

	// NagaBufferLength helper tracking (keyed by writable: true=RW, false=readonly)
	nagaBufferLengthWritten map[bool]struct{}

	// Clamp-to-edge helper tracking
	needsClampToEdgeHelper   bool
	clampToEdgeHelperWritten bool

	// External texture tracking
	// Maps GlobalVariable handle -> ExternalTextureBindTarget for decomposed globals
	externalTextureGlobals map[ir.GlobalVariableHandle]ExternalTextureBindTarget
	// Maps GlobalVariable handle -> [3]string{plane0Name, plane1Name, plane2Name} + paramsName
	externalTextureGlobalNames map[ir.GlobalVariableHandle][4]string // [plane0, plane1, plane2, params]
	// Maps (FunctionHandle, argIndex) -> [4]string{plane0, plane1, plane2, params}
	externalTextureFuncArgNames map[externalTextureFuncArgKey][4]string
	// External texture helper tracking
	externalTextureSampleHelperWritten bool
	externalTextureLoadHelperWritten   bool
	externalTextureDimensionsWritten   bool

	// Ray query helper tracking
	needsRayDescHelper                 bool
	needsCommittedIntersectionHelper   bool
	needsCandidateIntersectionHelper   bool
	rayDescHelperWritten               bool
	committedIntersectionHelperWritten bool
	candidateIntersectionHelperWritten bool

	// Continue forwarding context for switch-in-loop workaround.
	// Matches Rust naga's ContinueCtx.
	continueCtx continueCtx

	// Sampler heap tracking.
	samplerHeapsWritten bool
	samplerIndexBuffers map[uint32]string // group -> name of index buffer variable

	// MatCx2 decomposition tracking.
	// Matrices with rows=2 in uniform buffers need to be decomposed into structs.
	// See Rust naga back/hlsl/help.rs WrappedMatCx2.
	wrappedMatCx2 map[uint8]struct{} // columns -> written

	// Per-struct matCx2 access helper tracking.
	// Key: {typeHandle, memberIndex} -> written.
	// Tracks GetMat/SetMat/SetMatVec/SetMatScalar per struct member.
	wrappedStructMatrixAccess map[wrappedStructMatrixAccessKey]struct{}

	// Wrapped image query function tracking.
	// Matches Rust naga's WrappedImageQuery set.
	wrappedImageQueries map[wrappedImageQueryKey]struct{}
}

// wrappedStructMatrixAccessKey identifies a matCx2 struct member that needs Get/Set helpers.
type wrappedStructMatrixAccessKey struct {
	ty    ir.TypeHandle
	index uint32
}

// externalTextureFuncArgKey identifies an external texture function argument.
type externalTextureFuncArgKey struct {
	funcHandle ir.FunctionHandle
	argIndex   uint32
}

// subAccess represents one step in accessing a Storage global's component or element.
// Used to compute byte offsets for ByteAddressBuffer Load/Store operations.
type subAccess struct {
	kind subAccessKind

	// For subAccessOffset: the byte offset (constant)
	offset uint32

	// For subAccessIndex: the expression handle and stride
	value  ir.ExpressionHandle
	stride uint32

	// For subAccessBufferOffset: group and offset index into dynamic buffer offsets
	bufferGroup  uint32
	bufferOffset uint32
}

type subAccessKind int

const (
	subAccessOffset       subAccessKind = iota // constant byte offset
	subAccessIndex                             // runtime index * stride
	subAccessBufferOffset                      // dynamic buffer offset: __dynamic_buffer_offsets{group}._{offset}
)

// wrappedBinaryOpKey identifies a unique naga_div/naga_mod overload.
type wrappedBinaryOpKey struct {
	op       ir.BinaryOperator
	typeName string // e.g. "uint", "int", "uint2", etc.
}

// storeValue represents the value being stored in a storage buffer write.
type storeValue struct {
	kind      storeValueKind
	expr      ir.ExpressionHandle // for storeValueExpression
	depth     int                 // for storeValueTempIndex/storeValueTempAccess
	index     uint32              // for storeValueTempIndex
	base      ir.TypeHandle       // for storeValueTempAccess
	memberIdx uint32              // for storeValueTempAccess
}

type storeValueKind int

const (
	storeValueExpression storeValueKind = iota
	storeValueTempIndex
	storeValueTempAccess
)

// epResultInfo stores the stage for a type used as an EP result.
type epResultInfo struct {
	stage ir.ShaderStage
}

// newWriter creates a new HLSL writer.
func newWriter(module *ir.Module, options *Options) *Writer {
	return &Writer{
		module:                      module,
		options:                     options,
		names:                       make(map[nameKey]string),
		namer:                       newNamer(),
		typeNames:                   make(map[ir.TypeHandle]string),
		epResultTypes:               make(map[ir.TypeHandle]epResultInfo),
		entryPointIO:                make(map[int]*entryPointInterface),
		entryPointNames:             make(map[string]string),
		structConstructors:          make(map[ir.TypeHandle]struct{}),
		structConstructorsWritten:   make(map[ir.TypeHandle]struct{}),
		arrayConstructorsWritten:    make(map[ir.TypeHandle]struct{}),
		wrappedZeroValues:           make(map[ir.TypeHandle]struct{}),
		wrappedBinaryOps:            make(map[wrappedBinaryOpKey]struct{}),
		wrappedNegOps:               make(map[string]struct{}),
		wrappedMathHelpers:          make(map[string]struct{}),
		registerBindings:            make(map[string]string),
		namedExpressions:            make(map[ir.ExpressionHandle]string),
		requiredShaderModel:         options.ShaderModel,
		wrappedMatCx2:               make(map[uint8]struct{}),
		wrappedStructMatrixAccess:   make(map[wrappedStructMatrixAccessKey]struct{}),
		wrappedImageQueries:         make(map[wrappedImageQueryKey]struct{}),
		externalTextureGlobals:      make(map[ir.GlobalVariableHandle]ExternalTextureBindTarget),
		externalTextureGlobalNames:  make(map[ir.GlobalVariableHandle][4]string),
		externalTextureFuncArgNames: make(map[externalTextureFuncArgKey][4]string),
	}
}

// wrappedImageQueryKey identifies a unique image query wrapper function.
// Matches Rust naga's WrappedImageQuery.
type wrappedImageQueryKey struct {
	dim     ir.ImageDimension
	arrayed bool
	class   ir.ImageClass
	multi   bool // for sampled/depth multisampled
	query   imageQueryType
}

// imageQueryType identifies the type of image query.
type imageQueryType uint8

const (
	imageQuerySize       imageQueryType = iota // GetDimensions (no mip)
	imageQuerySizeLevel                        // GetDimensions with mip level
	imageQueryNumLevels                        // Get number of mip levels
	imageQueryNumLayers                        // Get number of array layers
	imageQueryNumSamples                       // Get number of samples
)

// String returns the generated HLSL source code.
// Output is trimmed to end with exactly one newline, matching Rust naga.
func (w *Writer) String() string {
	s := w.out.String()
	s = strings.TrimRight(s, "\n")
	return s + "\n"
}

// writeModule generates HLSL code for the entire module.
func (w *Writer) writeModule() error {
	// 1. Write header comment
	w.writeHeader()

	// 2. Register all names
	if err := w.registerNames(); err != nil {
		return err
	}

	// 2b. Collect EP result types (needed for semantic writing on user structs)
	w.collectEPResultTypes()

	// 2c. Write NagaConstants struct if special_constants_binding is set
	w.writeSpecialConstants()

	// 2c2. Write dynamic buffer offsets structs and constant buffers
	w.writeDynamicBufferOffsets()

	// 2d. Write __matCx2 typedefs and helper functions (before structs, matching Rust)
	w.writeAllMatCx2TypedefsAndFunctions()

	// 3. Write type definitions (structs)
	if err := w.writeTypes(); err != nil {
		return err
	}

	// 4. Scan for needed helpers
	w.scanForStructConstructors()
	w.scanStorageTextureHelpers()

	// 4b. Write special helper functions (naga_mod, naga_div, etc.)
	// Note: storage load helpers are written per-function, not here
	w.writeSpecialHelperFunctions()

	// 4b2. Write special functions (predeclared type helpers + RayDescFromRayDesc_)
	// Matches Rust naga's write_special_functions called at module level.
	w.writeModuleLevelSpecialFunctions()

	// 4c. Write wrapped expression functions from global expressions
	// (struct constructors, array constructors for global expressions)
	w.writeGlobalWrappedExpressionFunctions()

	// 4d. Write ZeroValue wrapper functions from global expressions
	// (Rust naga writes these before constants)
	w.writeZeroValueWrapperFunctions(w.module.GlobalExpressions)

	// 5. Write constants
	if err := w.writeConstants(); err != nil {
		return err
	}

	// 6. Write global variables (cbuffers, textures, etc.)
	if err := w.writeGlobalVariables(); err != nil {
		return err
	}

	// 7. Write entry point interface structs (before regular functions, matching Rust order)
	if err := w.writeAllEPInterfaces(); err != nil {
		return err
	}

	// 8. Write regular functions
	if err := w.writeFunctions(); err != nil {
		return err
	}

	// 9. Write entry points
	return w.writeEntryPoints()
}

// writeHeader writes an optional header comment.
// Rust naga does not emit any header comments, so this is a no-op to match.
func (w *Writer) writeHeader() {
	// No header — matches Rust naga HLSL output
}

// registerNames assigns unique names to all IR entities.
func (w *Writer) registerNames() error {
	// Register type names
	for handle := range w.module.Types {
		typ := &w.module.Types[handle]
		var baseName string
		if typ.Name != "" {
			baseName = typ.Name
		} else {
			baseName = fmt.Sprintf("type_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyType, handle1: uint32(handle)}] = name
		w.typeNames[ir.TypeHandle(handle)] = name

		// Register struct member names in a namespace scope (matches Rust naga)
		// Members only need to be unique among themselves, not globally
		if st, ok := typ.Inner.(ir.StructType); ok {
			h := handle // capture for closure
			w.namer.namespace(func() {
				for memberIdx, member := range st.Members {
					memberName := w.namer.callOr(member.Name, "member")
					w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(h), handle2: uint32(memberIdx)}] = memberName
				}
			})
		}
	}

	// Register type alias names so the namer detects collisions with variables.
	// In Rust naga, type aliases create a named type in the arena via
	// ensure_type_exists(Some(alias_name), inner), which the namer registers.
	// Our IR stores alias names separately in TypeAliasNames.
	for _, aliasName := range w.module.TypeAliasNames {
		w.namer.call(aliasName)
	}

	// Registration order matches Rust naga proc::Namer::reset():
	// 1. Types (already done above)
	// 2. Entry points + EP args + EP locals
	// 3. Functions + func args + func locals
	// 4. Global variables
	// 5. Constants

	// 2. Register entry point names + EP args + EP locals
	for epIdx := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[epIdx]
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}
		name := w.namer.call(ep.Name)
		w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}] = name
		w.entryPointNames[ep.Name] = name

		fn := &ep.Function
		for argIdx, arg := range fn.Arguments {
			argName := w.namer.callOr(arg.Name, "param")
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(argIdx)}] = argName
		}
		for localIdx, local := range fn.LocalVars {
			localName := w.namer.callOr(local.Name, "local")
			w.names[nameKey{kind: nameKeyLocal, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(localIdx)}] = localName
		}
	}

	// 3. Register function names + func args + func locals
	for handle := range w.module.Functions {
		fn := &w.module.Functions[handle]
		var baseName string
		if fn.Name != "" {
			baseName = fn.Name
		} else {
			baseName = fmt.Sprintf("function_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}] = name

		for argIdx, arg := range fn.Arguments {
			argName := w.namer.callOr(arg.Name, "param")
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}] = argName
		}
		for localIdx, local := range fn.LocalVars {
			localName := w.namer.callOr(local.Name, "local")
			w.names[nameKey{kind: nameKeyLocal, handle1: uint32(handle), handle2: uint32(localIdx)}] = localName
		}
	}

	// 4. Register global variable names
	for handle := range w.module.GlobalVariables {
		global := &w.module.GlobalVariables[handle]
		var baseName string
		if global.Name != "" {
			baseName = global.Name
		} else {
			baseName = fmt.Sprintf("global_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}] = name
	}

	// 5. Register constant names
	for handle := range w.module.Constants {
		constant := &w.module.Constants[handle]
		var baseName string
		if constant.Name != "" {
			baseName = constant.Name
		} else {
			typeName := w.typeNames[constant.Type]
			baseName = fmt.Sprintf("const_%s", typeName)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}] = name
	}

	return nil
}

// collectEPResultTypes scans entry points and records which struct types are
// used as entry point results. This is needed so writeStructDefinition can
// write semantics on struct members (matching Rust naga behavior).
func (w *Writer) collectEPResultTypes() {
	for epIdx := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[epIdx]
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}
		fn := &ep.Function
		if fn.Result != nil {
			w.epResultTypes[fn.Result.Type] = epResultInfo{stage: ep.Stage}
		}
	}
}

// writeAllEPInterfaces writes entry point interface structs for all active EPs.
// This is called between globals and entry point functions, matching Rust order.
func (w *Writer) writeAllEPInterfaces() error {
	for epIdx := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[epIdx]
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}
		epName := w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}]
		epIO, err := w.writeEPInterface(epIdx, &ep.Function, ep.Stage, epName)
		if err != nil {
			return err
		}
		w.entryPointIO[epIdx] = epIO
	}
	return nil
}

// NOTE: writeTypes, writeConstants, writeConstantValue, writeScalarValue,
// writeCompositeValue, writeGlobalVariables are implemented in types.go

// getBindTarget looks up or generates a bind target for a resource binding.
func (w *Writer) getBindTarget(binding *ir.ResourceBinding) BindTarget {
	if binding == nil {
		return DefaultBindTarget()
	}

	key := ResourceBinding{Group: binding.Group, Binding: binding.Binding}
	if target, ok := w.options.BindingMap[key]; ok {
		return target
	}

	// Auto-generate binding if allowed
	if w.options.FakeMissingBindings {
		space := uint8(0)
		if binding.Group <= 255 {
			space = uint8(binding.Group)
		}
		return BindTarget{
			Space:    space,
			Register: binding.Binding,
		}
	}

	return DefaultBindTarget()
}

// scanForStructConstructors scans all functions for struct Compose expressions
// and records which types need constructor functions.
func (w *Writer) scanForStructConstructors() {
	scanExprs := func(exprs []ir.Expression) {
		for _, expr := range exprs {
			if comp, ok := expr.Kind.(ir.ExprCompose); ok {
				if int(comp.Type) < len(w.module.Types) {
					if _, isStruct := w.module.Types[comp.Type].Inner.(ir.StructType); isStruct {
						w.structConstructors[comp.Type] = struct{}{}
					}
				}
			}
		}
	}

	// Scan global expressions
	scanExprs(w.module.GlobalExpressions)

	// Scan all functions
	for i := range w.module.Functions {
		scanExprs(w.module.Functions[i].Expressions)
	}

	// Scan all entry points
	for i := range w.module.EntryPoints {
		scanExprs(w.module.EntryPoints[i].Function.Expressions)
	}
}

// writeStructConstructors writes Construct{Type} helper functions for struct types.
// Matches Rust naga's wrapped constructor pattern:
//
//	TypeName ConstructTypeName(type1 arg0, type2 arg1, ...) {
//	    TypeName ret = (TypeName)0;
//	    ret.member0 = arg0;
//	    ret.member1 = arg1;
//	    ...
//	    return ret;
//	}
func (w *Writer) writeStructConstructors() {
	// Sort by type handle for deterministic output
	handles := make([]ir.TypeHandle, 0, len(w.structConstructors))
	for h := range w.structConstructors {
		handles = append(handles, h)
	}
	// Simple sort
	for i := 0; i < len(handles); i++ {
		for j := i + 1; j < len(handles); j++ {
			if handles[j] < handles[i] {
				handles[i], handles[j] = handles[j], handles[i]
			}
		}
	}

	for _, h := range handles {
		if int(h) >= len(w.module.Types) {
			continue
		}
		st, ok := w.module.Types[h].Inner.(ir.StructType)
		if !ok {
			continue
		}
		typeName := w.typeNames[h]
		if typeName == "" {
			continue
		}

		// Write constructor function
		w.writeLine("%s Construct%s(%s) {", typeName, typeName, w.structConstructorArgs(h, st))
		w.pushIndent()
		w.writeLine("%s ret = (%s)0;", typeName, typeName)
		for i := range st.Members {
			memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(h), handle2: uint32(i)}]
			if memberName == "" {
				memberName = fmt.Sprintf("member_%d", i)
			}
			w.writeLine("ret.%s = arg%d;", memberName, i)
		}
		w.writeLine("return ret;")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}
}

// writeArrayConstructor writes typedef + constructor for an array type.
// Matches Rust naga's WrappedConstructor for arrays:
//
//	typedef float ret_Constructarray4_float_[4];
//	ret_Constructarray4_float_ Constructarray4_float_(float arg0, float arg1, ...) {
//	    float ret[4] = { arg0, arg1, ... };
//	    return ret;
//	}
func (w *Writer) writeArrayConstructor(handle ir.TypeHandle) {
	if int(handle) >= len(w.module.Types) {
		return
	}
	arr, ok := w.module.Types[handle].Inner.(ir.ArrayType)
	if !ok || arr.Size.Constant == nil {
		return
	}

	typeId := w.hlslTypeId(handle)
	// Use getTypeNameWithArraySuffix for proper multi-dimensional array handling
	baseTypeName, fullArraySuffix := w.getTypeNameWithArraySuffix(handle)
	size := *arr.Size.Constant

	// Write typedef: typedef float ret_ConstructarrayN_type_[outerSize][innerSize];
	w.writeLine("typedef %s ret_Construct%s%s;", baseTypeName, typeId, fullArraySuffix)

	// Write constructor function
	args := make([]string, 0, int(size))
	for i := 0; i < int(size); i++ {
		argType, argSuffix := w.getTypeNameWithArraySuffix(arr.Base)
		args = append(args, fmt.Sprintf("%s arg%d%s", argType, i, argSuffix))
	}
	w.writeLine("ret_Construct%s Construct%s(%s) {", typeId, typeId, strings.Join(args, ", "))
	w.pushIndent()
	// Array initialization: float ret[outerSize][innerSize] = { ... };
	initArgs := make([]string, 0, int(size))
	for i := 0; i < int(size); i++ {
		initArgs = append(initArgs, fmt.Sprintf("arg%d", i))
	}
	w.writeLine("%s ret%s = { %s };", baseTypeName, fullArraySuffix, strings.Join(initArgs, ", "))
	w.writeLine("return ret;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeSingleStructConstructor writes a single struct constructor function.
func (w *Writer) writeSingleStructConstructor(h ir.TypeHandle) {
	if int(h) >= len(w.module.Types) {
		return
	}
	st, ok := w.module.Types[h].Inner.(ir.StructType)
	if !ok {
		return
	}
	typeName := w.typeNames[h]
	if typeName == "" {
		return
	}

	w.writeLine("%s Construct%s(%s) {", typeName, typeName, w.structConstructorArgs(h, st))
	w.pushIndent()
	w.writeLine("%s ret = (%s)0;", typeName, typeName)
	for i, member := range st.Members {
		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(h), handle2: uint32(i)}]
		if memberName == "" {
			memberName = fmt.Sprintf("member_%d", i)
		}
		// For matCx2 members (without binding), decompose into column assignments
		if member.Binding == nil && w.isMatCx2Type(member.Type) {
			mat := w.module.Types[member.Type].Inner.(ir.MatrixType)
			for col := uint8(0); col < uint8(mat.Columns); col++ {
				w.writeLine("ret.%s_%d = arg%d[%d];", memberName, col, i, col)
			}
		} else if member.Binding == nil && w.isArrayOfMatCx2Type(member.Type) {
			// Array-of-matCx2: cast arg to __matCx2[size] type
			m := getInnerMatrixData(w.module, member.Type)
			arr := w.module.Types[member.Type].Inner.(ir.ArrayType)
			size := ""
			if arr.Size.Constant != nil {
				size = fmt.Sprintf("[%d]", *arr.Size.Constant)
			}
			w.writeLine("ret.%s = (__mat%dx2%s)arg%d;", memberName, m.columns, size, i)
		} else {
			w.writeLine("ret.%s = arg%d;", memberName, i)
		}
	}
	w.writeLine("return ret;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// structConstructorArgs generates the argument list for a struct constructor.
func (w *Writer) structConstructorArgs(h ir.TypeHandle, st ir.StructType) string {
	args := make([]string, 0, len(st.Members))
	for i, member := range st.Members {
		typeName, arraySuffix := w.getTypeNameWithArraySuffix(member.Type)
		args = append(args, fmt.Sprintf("%s arg%d%s", typeName, i, arraySuffix))
	}
	return strings.Join(args, ", ")
}

// writeSpecialConstants writes the NagaConstants struct and ConstantBuffer
// when special_constants_binding is configured. This provides first_vertex,
// first_instance, and other values for vertex/instance index offsetting.
func (w *Writer) writeSpecialConstants() {
	bt := w.options.SpecialConstantsBinding
	if bt == nil {
		return
	}
	w.writeLine("struct NagaConstants {")
	w.pushIndent()
	w.writeLine("int first_vertex;")
	w.writeLine("int first_instance;")
	w.writeLine("uint other;")
	w.popIndent()
	w.writeLine("};")
	w.writeIndent()
	fmt.Fprintf(&w.out, "ConstantBuffer<NagaConstants> _NagaConstants: register(b%d", bt.Register)
	if bt.Space != 0 {
		fmt.Fprintf(&w.out, ", space%d", bt.Space)
	}
	w.out.WriteString(");\n\n")
}

// writeDynamicBufferOffsets writes __dynamic_buffer_offsetsTy structs and their
// ConstantBuffer declarations for dynamic storage buffer offsets.
// Matches Rust naga's writer.rs dynamic_storage_buffer_offsets_targets loop.
func (w *Writer) writeDynamicBufferOffsets() {
	if len(w.options.DynamicStorageBufferOffsetsTargets) == 0 {
		return
	}

	// Iterate in sorted order (BTreeMap in Rust = sorted by key)
	groups := make([]uint32, 0, len(w.options.DynamicStorageBufferOffsetsTargets))
	for g := range w.options.DynamicStorageBufferOffsetsTargets {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })

	for _, group := range groups {
		bt := w.options.DynamicStorageBufferOffsetsTargets[group]
		fmt.Fprintf(&w.out, "struct __dynamic_buffer_offsetsTy%d {\n", group)
		for i := uint32(0); i < bt.Size; i++ {
			fmt.Fprintf(&w.out, "    uint _%d;\n", i)
		}
		w.out.WriteString("};\n")
		fmt.Fprintf(&w.out, "ConstantBuffer<__dynamic_buffer_offsetsTy%d> __dynamic_buffer_offsets%d: register(b%d, space%d);\n",
			group, group, bt.Register, bt.Space)
		w.out.WriteByte('\n')
	}
}

// writeSpecialHelperFunctions writes naga_mod, naga_div, naga_abs polyfills.
// Storage load helpers are written per-function to match Rust naga ordering.
func (w *Writer) writeSpecialHelperFunctions() {
	if w.needsModHelper {
		w.writeLine("// Safe modulo helper (truncated division semantics)")
		w.writeLine("int %s(int a, int b) {", NagaModFunction)
		w.pushIndent()
		w.writeLine("return a - b * (a / b);")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
		w.helperFunctions = append(w.helperFunctions, NagaModFunction)
	}

	if w.needsDivHelper {
		w.writeLine("// Safe division helper (handles zero divisor)")
		w.writeLine("int %s(int a, int b) {", NagaDivFunction)
		w.pushIndent()
		w.writeLine("return b != 0 ? a / b : 0;")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
		w.helperFunctions = append(w.helperFunctions, NagaDivFunction)
	}

	if w.needsAbsHelper {
		w.writeLine("// Safe abs helper (handles INT_MIN)")
		w.writeLine("int %s(int v) {", NagaAbsFunction)
		w.pushIndent()
		w.writeLine("return v >= 0 ? v : (v == -2147483648 ? 2147483647 : -v);")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
		w.helperFunctions = append(w.helperFunctions, NagaAbsFunction)
	}
}

// writeModuleLevelSpecialFunctions writes module-level special functions like
// RayDescFromRayDesc_. Matches Rust naga's write_special_functions.
func (w *Writer) writeModuleLevelSpecialFunctions() {
	// Write RayDescFromRayDesc_ if the module has a ray_desc special type
	if w.module.SpecialTypes.RayIntersection != nil {
		// The module uses ray queries, write the RayDescFromRayDesc_ helper
		w.rayDescHelperWritten = true
		rayDescTypeName := "RayDesc_"
		w.out.WriteString("RayDesc RayDescFromRayDesc_(")
		w.out.WriteString(rayDescTypeName)
		w.out.WriteString(" arg0) {\n")
		w.out.WriteString("    RayDesc ret = (RayDesc)0;\n")
		w.out.WriteString("    ret.Origin = arg0.origin;\n")
		w.out.WriteString("    ret.TMin = arg0.tmin;\n")
		w.out.WriteString("    ret.Direction = arg0.dir;\n")
		w.out.WriteString("    ret.TMax = arg0.tmax;\n")
		w.out.WriteString("    return ret;\n")
		w.out.WriteString("}\n\n")
	}
}

// writeGlobalWrappedExpressionFunctions writes struct/array constructors that are
// used in global expressions. This matches Rust naga's
// write_wrapped_expression_functions(global_expressions).
func (w *Writer) writeGlobalWrappedExpressionFunctions() {
	for _, expr := range w.module.GlobalExpressions {
		if comp, ok := expr.Kind.(ir.ExprCompose); ok {
			if int(comp.Type) < len(w.module.Types) {
				switch w.module.Types[comp.Type].Inner.(type) {
				case ir.StructType:
					if _, written := w.structConstructorsWritten[comp.Type]; !written {
						w.structConstructorsWritten[comp.Type] = struct{}{}
						w.writeSingleStructConstructor(comp.Type)
					}
				case ir.ArrayType:
					if _, written := w.arrayConstructorsWritten[comp.Type]; !written {
						w.arrayConstructorsWritten[comp.Type] = struct{}{}
						w.writeArrayConstructor(comp.Type)
					}
				}
			}
		}
	}
}

// writePerFunctionWrappedHelpers writes all per-function wrapped helpers
// before the function body, matching Rust naga ordering.
func (w *Writer) writePerFunctionWrappedHelpers(fn *ir.Function) {
	// Write per-function wrapped helpers matching Rust naga write_wrapped_functions order:
	// 1. Math helpers (modf, frexp)
	w.writeWrappedMathHelpers(fn)
	// 2. Unary ops (naga_neg)
	w.writeWrappedUnaryOps(fn)
	// 3. Binary ops (naga_div, naga_mod)
	w.writeWrappedBinaryOps(fn)
	// 4. Expression functions: Compose-based constructors (struct/array from Compose expressions)
	w.writeComposeConstructors(fn)
	// 4b. Storage load helpers (LoadedStorageValueFrom)
	w.writeStorageLoadHelpers()
	// 5. ZeroValue wrapper functions
	w.writeZeroValueWrapperFunctions(fn.Expressions)
	// 6. Cast functions (naga_f2i32, naga_f2i64, etc.)
	w.writeWrappedCastFunctions(fn)
	// 6b. Struct matrix access helpers (GetMat/SetMat/SetMatVec/SetMatScalar)
	w.writeStructMatrixAccessHelpers(fn)
	// 7. Storage Load constructors, ArrayLength, ImageQuery, matrix access
	w.writeStorageLoadConstructors(fn)
	// 7b. Ray query helpers (RayDescFromRayDesc_, GetCommitted/CandidateIntersection)
	w.writeRayQueryHelpers(fn)
	// 8. NagaBufferLength helper for ArrayLength expressions
	w.writeNagaBufferLengthHelpers(fn)
	// 9. ClampToEdge / external sample helper for textureSampleBaseClampToEdge
	w.writeClampToEdgeHelper(fn)
	// 10. External texture load helper (nagaTextureLoadExternal)
	w.writeExternalTextureLoadHelperIfNeeded(fn)
	// 11. Image query wrapper functions (NagaDimensions2D, NagaNumLevels2D, etc.)
	w.writeWrappedImageQueryFunctions(fn)
}

// writeStructMatrixAccessHelpers scans a function for AccessIndex expressions
// that access matCx2 members of structs and writes Get/Set helpers if needed.
// Matches Rust naga help.rs write_wrapped_struct_matrix_* triggering.
func (w *Writer) writeStructMatrixAccessHelpers(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		ai, ok := expr.Kind.(ir.ExprAccessIndex)
		if !ok {
			continue
		}
		// Resolve the base type
		baseTyHandle := w.resolveExpressionTypeHandle(fn, ai.Base)
		if baseTyHandle == nil {
			continue
		}
		tyHandle := *baseTyHandle
		// Dereference pointer
		if ptr, ok := w.module.Types[tyHandle].Inner.(ir.PointerType); ok {
			tyHandle = ptr.Base
		}
		st, ok := w.module.Types[tyHandle].Inner.(ir.StructType)
		if !ok {
			continue
		}
		if int(ai.Index) >= len(st.Members) {
			continue
		}
		member := st.Members[ai.Index]
		if member.Binding != nil {
			continue
		}
		mat, ok := w.module.Types[member.Type].Inner.(ir.MatrixType)
		if !ok || mat.Rows != 2 {
			continue
		}
		w.writeWrappedStructMatrixAccessFunctions(tyHandle, ai.Index)
	}
}

// resolveExpressionTypeHandle resolves the TypeHandle for an expression in a function.
// Returns nil if the type cannot be determined.
func (w *Writer) resolveExpressionTypeHandle(fn *ir.Function, handle ir.ExpressionHandle) *ir.TypeHandle {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	expr := fn.Expressions[handle]
	switch e := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(e.Variable) < len(w.module.GlobalVariables) {
			ty := w.module.GlobalVariables[e.Variable].Type
			return &ty
		}
	case ir.ExprLocalVariable:
		if fn.LocalVars != nil && int(e.Variable) < len(fn.LocalVars) {
			ty := fn.LocalVars[e.Variable].Type
			return &ty
		}
	case ir.ExprFunctionArgument:
		if int(e.Index) < len(fn.Arguments) {
			ty := fn.Arguments[e.Index].Type
			return &ty
		}
	case ir.ExprAccessIndex:
		return w.resolveExpressionTypeHandle(fn, e.Base)
	case ir.ExprAccess:
		return w.resolveExpressionTypeHandle(fn, e.Base)
	case ir.ExprLoad:
		return w.resolveExpressionTypeHandle(fn, e.Pointer)
	}
	return nil
}

// writeWrappedImageQueryFunctions scans a function for ImageQuery expressions
// and writes wrapper functions that haven't been written yet.
// Matches Rust naga help.rs write_wrapped_image_query_function.
func (w *Writer) writeWrappedImageQueryFunctions(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		iq, ok := expr.Kind.(ir.ExprImageQuery)
		if !ok {
			continue
		}
		imgType := w.resolveImageTypeFromFn(fn, iq.Image)
		if imgType == nil {
			continue
		}

		var qt imageQueryType
		switch q := iq.Query.(type) {
		case ir.ImageQuerySize:
			if q.Level != nil {
				qt = imageQuerySizeLevel
			} else {
				qt = imageQuerySize
			}
		case ir.ImageQueryNumLevels:
			qt = imageQueryNumLevels
		case ir.ImageQueryNumLayers:
			qt = imageQueryNumLayers
		case ir.ImageQueryNumSamples:
			qt = imageQueryNumSamples
		default:
			continue
		}

		key := wrappedImageQueryKey{
			dim:     imgType.Dim,
			arrayed: imgType.Arrayed,
			class:   imgType.Class,
			multi:   imgType.Multisampled,
			query:   qt,
		}

		if _, exists := w.wrappedImageQueries[key]; exists {
			continue
		}
		w.wrappedImageQueries[key] = struct{}{}

		w.writeWrappedImageQueryFunction(key, imgType)
	}
}

// resolveImageTypeFromFn resolves the ImageType for an expression in a specific function.
// Used during per-function helper generation when w.currentFunction isn't set yet.
// Handles direct GlobalVariable, FunctionArgument, and Access/AccessIndex into BindingArrays.
func (w *Writer) resolveImageTypeFromFn(fn *ir.Function, handle ir.ExpressionHandle) *ir.ImageType {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	expr := fn.Expressions[handle]
	switch e := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(e.Variable) < len(w.module.GlobalVariables) {
			ty := w.module.GlobalVariables[e.Variable].Type
			if int(ty) < len(w.module.Types) {
				if img, ok := w.module.Types[ty].Inner.(ir.ImageType); ok {
					return &img
				}
				// Check for BindingArray of images
				if ba, ok := w.module.Types[ty].Inner.(ir.BindingArrayType); ok {
					if int(ba.Base) < len(w.module.Types) {
						if img, ok := w.module.Types[ba.Base].Inner.(ir.ImageType); ok {
							return &img
						}
					}
				}
			}
		}
	case ir.ExprFunctionArgument:
		if int(e.Index) < len(fn.Arguments) {
			ty := fn.Arguments[e.Index].Type
			if int(ty) < len(w.module.Types) {
				if img, ok := w.module.Types[ty].Inner.(ir.ImageType); ok {
					return &img
				}
			}
		}
	case ir.ExprAccess:
		// Access into a binding array of textures
		return w.resolveImageTypeFromFn(fn, e.Base)
	case ir.ExprAccessIndex:
		// AccessIndex into a binding array of textures
		return w.resolveImageTypeFromFn(fn, e.Base)
	}
	return nil
}

// writeWrappedImageQueryFunction writes a single NagaXxx wrapper function.
// Matches Rust naga help.rs write_wrapped_image_query_function.
func (w *Writer) writeWrappedImageQueryFunction(key wrappedImageQueryKey, imgType *ir.ImageType) {
	// External textures have their own special dimensions helper
	if key.class == ir.ImageClassExternal {
		w.writeExternalTextureDimensionsHelper()
		return
	}

	components := []string{"x", "y", "z", "w"}

	arrayCoords := 0
	if key.arrayed {
		arrayCoords = 1
	}
	// Extra coords for mip level count or sample count
	extraCoords := 0
	switch key.class {
	case ir.ImageClassStorage:
		extraCoords = 0
	case ir.ImageClassSampled, ir.ImageClassDepth:
		extraCoords = 1
	}

	// Determine return swizzle and number of GetDimensions parameters
	var retSwizzle string
	var numParams int

	switch key.query {
	case imageQuerySize, imageQuerySizeLevel:
		switch key.dim {
		case ir.Dim1D:
			retSwizzle = "x"
		case ir.Dim2D:
			retSwizzle = "xy"
		case ir.Dim3D:
			retSwizzle = "xyz"
		case ir.DimCube:
			retSwizzle = "xy"
		}
		numParams = len(retSwizzle) + arrayCoords + extraCoords
	case imageQueryNumLevels, imageQueryNumSamples, imageQueryNumLayers:
		if key.arrayed || key.dim == ir.Dim3D {
			retSwizzle = "w"
			numParams = 4
		} else {
			retSwizzle = "z"
			numParams = 3
		}
	}

	// Write return type
	retType := "uint"
	if len(retSwizzle) > 1 {
		retType = fmt.Sprintf("uint%d", len(retSwizzle))
	}

	// Write function signature
	fmt.Fprintf(&w.out, "%s ", retType)
	w.writeImageQueryFunctionName(key)
	w.out.WriteString("(")
	w.out.WriteString(w.imageTypeToHLSL(*imgType))
	w.out.WriteString(" tex")
	if key.query == imageQuerySizeLevel {
		w.out.WriteString(", uint mip_level")
	}
	w.out.WriteString(")\n")

	// Function body
	w.out.WriteString("{\n")
	w.out.WriteString("    uint4 ret;\n")

	// GetDimensions call
	w.out.WriteString("    tex.GetDimensions(")
	switch key.query {
	case imageQuerySizeLevel:
		w.out.WriteString("mip_level, ")
	default:
		switch key.class {
		case ir.ImageClassSampled:
			if !key.multi {
				w.out.WriteString("0, ")
			}
		case ir.ImageClassDepth:
			if !key.multi {
				w.out.WriteString("0, ")
			}
		case ir.ImageClassStorage:
			// No mip level for storage
		}
	}

	for i := 0; i < numParams-1; i++ {
		fmt.Fprintf(&w.out, "ret.%s, ", components[i])
	}
	fmt.Fprintf(&w.out, "ret.%s", components[numParams-1])
	w.out.WriteString(");\n")

	// Return
	fmt.Fprintf(&w.out, "    return ret.%s;\n", retSwizzle)
	w.out.WriteString("}\n\n")
}

// writeImageQueryFunctionNameDirect writes the NagaXxx function name directly to output.
// Delegates to writeImageQueryFunctionName which contains the shared implementation.
func (w *Writer) writeImageQueryFunctionNameDirect(key wrappedImageQueryKey) {
	w.writeImageQueryFunctionName(key)
}

// writeClampToEdgeHelper scans a function for ImageSample expressions with
// ClampToEdge=true and writes the nagaTextureSampleBaseClampToEdge helper if needed.
// For external textures, writes the multi-plane YUV version.
// Matches Rust naga help.rs write_wrapped_image_sample_function.
func (w *Writer) writeClampToEdgeHelper(fn *ir.Function) {
	if w.clampToEdgeHelperWritten {
		return
	}
	// Scan for ClampToEdge usage and determine if external texture is involved
	found := false
	isExternal := false
	for _, expr := range fn.Expressions {
		is, ok := expr.Kind.(ir.ExprImageSample)
		if !ok || !is.ClampToEdge {
			continue
		}
		found = true
		// Check if the image is an external texture
		imgType := w.resolveImageTypeFromFn(fn, is.Image)
		if imgType != nil && imgType.Class == ir.ImageClassExternal {
			isExternal = true
		}
	}
	if !found {
		return
	}

	w.clampToEdgeHelperWritten = true

	if isExternal {
		w.writeExternalTextureSampleHelper()
	} else {
		w.out.WriteString("float4 nagaTextureSampleBaseClampToEdge(Texture2D<float4> tex, SamplerState samp, float2 coords) {\n")
		w.out.WriteString("    float2 size;\n")
		w.out.WriteString("    tex.GetDimensions(size.x, size.y);\n")
		w.out.WriteString("    float2 half_texel = float2(0.5, 0.5) / size;\n")
		w.out.WriteString("    return tex.SampleLevel(samp, clamp(coords, half_texel, 1.0 - half_texel), 0.0);\n")
		w.out.WriteString("}\n\n")
	}
}

// writeWrappedUnaryOps scans function expressions for signed integer Negate
// and emits naga_neg helper overloads that haven't been written yet.
// Matches Rust naga's write_wrapped_unary_ops: naga_neg uses asint(-asuint(val))
// to avoid undefined behavior for INT_MIN negation in HLSL.
func (w *Writer) writeWrappedUnaryOps(fn *ir.Function) {
	for i, expr := range fn.Expressions {
		unaryExpr, ok := expr.Kind.(ir.ExprUnary)
		if !ok || unaryExpr.Op != ir.UnaryNegate {
			continue
		}

		// Resolve the operand type
		handle := ir.ExpressionHandle(i)
		resultInner := w.resolveExprTypeInner(fn, handle)
		if resultInner == nil {
			continue
		}

		var scalar *ir.ScalarType
		switch t := resultInner.(type) {
		case ir.ScalarType:
			scalar = &t
		case ir.VectorType:
			scalar = &t.Scalar
		default:
			continue
		}

		// Only I32 signed negation needs the helper
		if scalar.Kind != ir.ScalarSint || scalar.Width != 4 {
			continue
		}

		// Get the HLSL type string for deduplication
		typeStr := w.typeInnerToHLSLStr(resultInner)
		if _, written := w.wrappedNegOps[typeStr]; written {
			continue
		}
		w.wrappedNegOps[typeStr] = struct{}{}

		// Write the naga_neg helper
		fmt.Fprintf(&w.out, "%s naga_neg(%s val) {\n", typeStr, typeStr)
		fmt.Fprintf(&w.out, "    return asint(-asuint(val));\n")
		fmt.Fprintf(&w.out, "}\n\n")
	}
}

// writeWrappedMathHelpers scans function expressions for modf/frexp calls
// and emits naga_modf/naga_frexp overloads matching Rust naga's pattern.
func (w *Writer) writeWrappedMathHelpers(fn *ir.Function) {
	for i, expr := range fn.Expressions {
		mathExpr, ok := expr.Kind.(ir.ExprMath)
		if !ok {
			continue
		}

		isModf := mathExpr.Fun == ir.MathModf
		isFrexp := mathExpr.Fun == ir.MathFrexp
		if !isModf && !isFrexp {
			continue
		}

		// Resolve the result type to get the struct name
		handle := ir.ExpressionHandle(i)
		resultInner := w.resolveExprTypeInner(fn, handle)
		if resultInner == nil {
			continue
		}
		st, isSt := resultInner.(ir.StructType)
		if !isSt {
			continue
		}

		// Get the struct type handle to determine the struct name
		var resultStructName string
		if int(handle) < len(fn.ExpressionTypes) {
			res := &fn.ExpressionTypes[handle]
			if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
				resultStructName = w.getTypeName(*res.Handle)
			}
		}
		if resultStructName == "" {
			continue
		}

		// Check if already written
		if _, done := w.wrappedMathHelpers[resultStructName]; done {
			continue
		}
		w.wrappedMathHelpers[resultStructName] = struct{}{}

		// Determine the arg type from the first member's type
		if len(st.Members) == 0 {
			continue
		}
		argTypeHandle := st.Members[0].Type
		argTypeName := w.getTypeName(argTypeHandle)

		if isModf {
			fmt.Fprintf(&w.out, "%s naga_modf(%s arg) {\n", resultStructName, argTypeName)
			fmt.Fprintf(&w.out, "    %s other;\n", argTypeName)
			fmt.Fprintf(&w.out, "    %s result;\n", resultStructName)
			fmt.Fprintf(&w.out, "    result.fract = modf(arg, other);\n")
			fmt.Fprintf(&w.out, "    result.whole = other;\n")
			fmt.Fprintf(&w.out, "    return result;\n")
			fmt.Fprintf(&w.out, "}\n\n")
		} else {
			// frexp: result.fract = sign(arg) * frexp(arg, other)
			fmt.Fprintf(&w.out, "%s naga_frexp(%s arg) {\n", resultStructName, argTypeName)
			fmt.Fprintf(&w.out, "    %s other;\n", argTypeName)
			fmt.Fprintf(&w.out, "    %s result;\n", resultStructName)
			fmt.Fprintf(&w.out, "    result.fract = sign(arg) * frexp(arg, other);\n")
			fmt.Fprintf(&w.out, "    result.exp_ = other;\n")
			fmt.Fprintf(&w.out, "    return result;\n")
			fmt.Fprintf(&w.out, "}\n\n")
		}
	}

	// Scan for ExtractBits/InsertBits and generate per-type overloads.
	// Matches Rust naga's write_wrapped_math_functions for ExtractBits/InsertBits.
	type wrappedMathKey struct {
		fun    ir.MathFunction
		scalar ir.ScalarType
		vecStr string // "" for scalar, "2"/"3"/"4" for vectors
	}
	wrappedMath := make(map[wrappedMathKey]struct{})
	for _, expr := range fn.Expressions {
		mathExpr, ok := expr.Kind.(ir.ExprMath)
		if !ok {
			continue
		}
		if mathExpr.Fun != ir.MathExtractBits && mathExpr.Fun != ir.MathInsertBits {
			continue
		}
		argInner := w.resolveExprTypeInner(fn, mathExpr.Arg)
		if argInner == nil {
			continue
		}
		var scalar ir.ScalarType
		var vecStr string
		switch t := argInner.(type) {
		case ir.ScalarType:
			scalar = t
		case ir.VectorType:
			scalar = t.Scalar
			vecStr = fmt.Sprintf("%d", t.Size)
		default:
			continue
		}
		key := wrappedMathKey{fun: mathExpr.Fun, scalar: scalar, vecStr: vecStr}
		if _, done := wrappedMath[key]; done {
			continue
		}
		wrappedMath[key] = struct{}{}

		// Build HLSL type name
		var typeName string
		if vecStr == "" {
			if scalar.Kind == ir.ScalarSint {
				typeName = "int"
			} else {
				typeName = "uint"
			}
		} else {
			if scalar.Kind == ir.ScalarSint {
				typeName = "int" + vecStr
			} else {
				typeName = "uint" + vecStr
			}
		}

		if mathExpr.Fun == ir.MathExtractBits {
			w.writeExtractBitsOverload(typeName, scalar.Width)
		} else {
			w.writeInsertBitsOverload(typeName, scalar.Width)
		}
	}
}

// writeWrappedBinaryOps scans function expressions for integer Divide/Modulo
// and emits naga_div/naga_mod helper overloads that haven't been written yet.
// Matches Rust naga's write_wrapped_binary_ops pattern.
func (w *Writer) writeWrappedBinaryOps(fn *ir.Function) {
	for i, expr := range fn.Expressions {
		binExpr, ok := expr.Kind.(ir.ExprBinary)
		if !ok {
			continue
		}

		// Resolve the result scalar kind
		handle := ir.ExpressionHandle(i)
		resultInner := w.resolveExprTypeInner(fn, handle)
		if resultInner == nil {
			continue
		}
		var scalar *ir.ScalarType
		switch t := resultInner.(type) {
		case ir.ScalarType:
			scalar = &t
		case ir.VectorType:
			scalar = &t.Scalar
		default:
			continue
		}

		switch binExpr.Op {
		case ir.BinaryDivide:
			if scalar.Kind != ir.ScalarSint && scalar.Kind != ir.ScalarUint {
				continue
			}
		case ir.BinaryModulo:
			if scalar.Kind != ir.ScalarSint && scalar.Kind != ir.ScalarUint && scalar.Kind != ir.ScalarFloat {
				continue
			}
		default:
			continue
		}

		// Get the HLSL type name for the result
		typeName := w.typeInnerToHLSLStr(resultInner)
		key := wrappedBinaryOpKey{op: binExpr.Op, typeName: typeName}
		if _, written := w.wrappedBinaryOps[key]; written {
			continue
		}
		w.wrappedBinaryOps[key] = struct{}{}

		// Also get left/right type names (they may differ for mixed scalar/vector ops)
		leftInner := w.resolveExprTypeInner(fn, binExpr.Left)
		rightInner := w.resolveExprTypeInner(fn, binExpr.Right)
		leftTypeName := typeName
		rightTypeName := typeName
		if leftInner != nil {
			leftTypeName = w.typeInnerToHLSLStr(leftInner)
		}
		if rightInner != nil {
			rightTypeName = w.typeInnerToHLSLStr(rightInner)
		}

		switch binExpr.Op {
		case ir.BinaryDivide:
			w.writeNagaDivHelper(typeName, leftTypeName, rightTypeName, scalar)
		case ir.BinaryModulo:
			w.writeNagaModHelper(typeName, leftTypeName, rightTypeName, scalar)
		}
	}
}

// writeWrappedCastFunctions scans function expressions for float-to-int casts
// and emits clamped helper functions (naga_f2i32, naga_f2u32, naga_f2i64, naga_f2u64).
// Matches Rust naga's write_wrapped_cast_functions.
func (w *Writer) writeWrappedCastFunctions(fn *ir.Function) {
	if w.f2iCastWritten == nil {
		w.f2iCastWritten = make(map[f2iCastKey]struct{})
	}

	for _, expr := range fn.Expressions {
		asExpr, ok := expr.Kind.(ir.ExprAs)
		if !ok || asExpr.Convert == nil {
			continue
		}

		// Check if src is float — need the type of the *source* expression, not the result
		srcInner := w.resolveExprTypeInner(fn, asExpr.Expr)
		if srcInner == nil {
			continue
		}

		var srcScalar ir.ScalarType
		var vectorSize uint32
		switch t := srcInner.(type) {
		case ir.ScalarType:
			srcScalar = t
		case ir.VectorType:
			srcScalar = t.Scalar
			vectorSize = uint32(t.Size)
		default:
			continue
		}

		if srcScalar.Kind != ir.ScalarFloat {
			continue
		}
		dstWidth := *asExpr.Convert
		if asExpr.Kind != ir.ScalarSint && asExpr.Kind != ir.ScalarUint {
			continue
		}

		key := f2iCastKey{srcWidth: srcScalar.Width, dstKind: asExpr.Kind, dstWidth: dstWidth, vectorSize: vectorSize}
		if _, written := w.f2iCastWritten[key]; written {
			continue
		}
		w.f2iCastWritten[key] = struct{}{}

		dstScalarStr := scalarKindToHLSL(asExpr.Kind, dstWidth)
		srcScalarStr := scalarKindToHLSL(ir.ScalarFloat, srcScalar.Width)

		var dstTypeStr, srcTypeStr string
		if vectorSize > 0 {
			dstTypeStr = fmt.Sprintf("%s%d", dstScalarStr, vectorSize)
			srcTypeStr = fmt.Sprintf("%s%d", srcScalarStr, vectorSize)
		} else {
			dstTypeStr = dstScalarStr
			srcTypeStr = srcScalarStr
		}

		// Get min/max clamp values based on source float type and destination int type
		minVal, maxVal := f2iClampValues(srcScalar.Width, asExpr.Kind, dstWidth)

		fmt.Fprintf(&w.out, "%s %s(%s value) {\n", dstTypeStr, f2iCastFuncName(asExpr.Kind, dstWidth), srcTypeStr)
		fmt.Fprintf(&w.out, "    return %s(clamp(value, %s, %s));\n", dstTypeStr, minVal, maxVal)
		fmt.Fprintf(&w.out, "}\n\n")
	}
}

// f2iCastKey identifies a unique float-to-int cast helper overload.
type f2iCastKey struct {
	srcWidth   uint8
	dstKind    ir.ScalarKind
	dstWidth   uint8
	vectorSize uint32
}

// f2iCastFuncName returns the function name for a float-to-int cast helper.
func f2iCastFuncName(kind ir.ScalarKind, width uint8) string {
	switch {
	case kind == ir.ScalarSint && width == 4:
		return "naga_f2i32"
	case kind == ir.ScalarUint && width == 4:
		return "naga_f2u32"
	case kind == ir.ScalarSint && width == 8:
		return "naga_f2i64"
	case kind == ir.ScalarUint && width == 8:
		return "naga_f2u64"
	default:
		return fmt.Sprintf("naga_f2_%d_%d", kind, width)
	}
}

// f2iClampValues returns the min and max clamping value strings for a float-to-int conversion.
// These match the values from Rust's min_max_float_representable_by.
func f2iClampValues(srcWidth uint8, dstKind ir.ScalarKind, dstWidth uint8) (string, string) {
	switch {
	// f32 -> i32
	case srcWidth == 4 && dstKind == ir.ScalarSint && dstWidth == 4:
		return "-2147483600.0", "2147483500.0"
	// f32 -> u32
	case srcWidth == 4 && dstKind == ir.ScalarUint && dstWidth == 4:
		return "0.0", "4294967000.0"
	// f32 -> i64
	case srcWidth == 4 && dstKind == ir.ScalarSint && dstWidth == 8:
		return "-9.223372e18", "9.2233715e18"
	// f32 -> u64
	case srcWidth == 4 && dstKind == ir.ScalarUint && dstWidth == 8:
		return "0.0", "1.8446743e19"
	// f16 -> i32
	case srcWidth == 2 && dstKind == ir.ScalarSint && dstWidth == 4:
		return "-65504.0h", "65504.0h"
	// f16 -> u32
	case srcWidth == 2 && dstKind == ir.ScalarUint && dstWidth == 4:
		return "0.0h", "65504.0h"
	// f16 -> i64
	case srcWidth == 2 && dstKind == ir.ScalarSint && dstWidth == 8:
		return "-65504.0h", "65504.0h"
	// f16 -> u64
	case srcWidth == 2 && dstKind == ir.ScalarUint && dstWidth == 8:
		return "0.0h", "65504.0h"
	// f64 -> i32
	case srcWidth == 8 && dstKind == ir.ScalarSint && dstWidth == 4:
		return "-2147483648.0L", "2147483647.0L"
	// f64 -> u32
	case srcWidth == 8 && dstKind == ir.ScalarUint && dstWidth == 4:
		return "0.0L", "4294967295.0L"
	// f64 -> i64
	case srcWidth == 8 && dstKind == ir.ScalarSint && dstWidth == 8:
		return "-9.223372036854776e18L", "9.223372036854775e18L"
	// f64 -> u64
	case srcWidth == 8 && dstKind == ir.ScalarUint && dstWidth == 8:
		return "0.0L", "1.844674407370955e19L"
	default:
		return "0.0", "0.0"
	}
}

// writeNagaDivHelper writes a naga_div overload for a specific type.
// Matches Rust naga's write_wrapped_binary_ops for BinaryOperator::Divide.
func (w *Writer) writeNagaDivHelper(retType, leftType, rightType string, scalar *ir.ScalarType) {
	w.writeIndent()
	fmt.Fprintf(&w.out, "%s %s(%s lhs, %s rhs) {\n", retType, NagaDivFunction, leftType, rightType)
	switch scalar.Kind {
	case ir.ScalarUint:
		fmt.Fprintf(&w.out, "    return lhs / (rhs == 0u ? 1u : rhs);\n")
	case ir.ScalarSint:
		minVal := i32MinLiteral(scalar.Width)
		fmt.Fprintf(&w.out, "    return lhs / (((lhs == %s & rhs == -1) | (rhs == 0)) ? 1 : rhs);\n", minVal)
	}
	w.out.WriteString("}\n\n")
}

// writeNagaModHelper writes a naga_mod overload for a specific type.
// Matches Rust naga's write_wrapped_binary_ops for BinaryOperator::Modulo.
func (w *Writer) writeNagaModHelper(retType, leftType, rightType string, scalar *ir.ScalarType) {
	w.writeIndent()
	fmt.Fprintf(&w.out, "%s %s(%s lhs, %s rhs) {\n", retType, NagaModFunction, leftType, rightType)
	switch scalar.Kind {
	case ir.ScalarUint:
		fmt.Fprintf(&w.out, "    return lhs %% (rhs == 0u ? 1u : rhs);\n")
	case ir.ScalarSint:
		minVal := i32MinLiteral(scalar.Width)
		// Use right_type for divisor (e.g., int4 for vector ops)
		fmt.Fprintf(&w.out, "    %s divisor = ((lhs == %s & rhs == -1) | (rhs == 0)) ? 1 : rhs;\n", rightType, minVal)
		fmt.Fprintf(&w.out, "    return lhs - (lhs / divisor) * divisor;\n")
	case ir.ScalarFloat:
		fmt.Fprintf(&w.out, "    return lhs - rhs * trunc(lhs / rhs);\n")
	}
	w.out.WriteString("}\n\n")
}

// i32MinLiteral returns the HLSL representation of the minimum signed integer value.
// Uses int(-2147483647 - 1) to avoid compiler parsing issues with -2147483648.
// Matches Rust naga's write_literal for Literal::I32(i32::MIN).
func i32MinLiteral(width uint8) string {
	if width == 8 {
		return "(-9223372036854775807L - 1L)"
	}
	return "int(-2147483647 - 1)"
}

// resolveExprTypeInner resolves the TypeInner for an expression in a specific function.
func (w *Writer) resolveExprTypeInner(fn *ir.Function, handle ir.ExpressionHandle) ir.TypeInner {
	if int(handle) >= len(fn.ExpressionTypes) {
		return nil
	}
	resolution := &fn.ExpressionTypes[handle]
	if resolution.Handle != nil {
		h := *resolution.Handle
		if int(h) < len(w.module.Types) {
			return w.module.Types[h].Inner
		}
	}
	return resolution.Value
}

// typeInnerToHLSLStr returns the HLSL type name for a TypeInner.
func (w *Writer) typeInnerToHLSLStr(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarToHLSLStr(t)
	case ir.VectorType:
		return fmt.Sprintf("%s%d", scalarToHLSLStr(t.Scalar), t.Size)
	default:
		return "uint"
	}
}

// writePerFunctionConstructors writes struct and array constructors needed by
// this function's expressions that haven't been written yet.
// Kept for backward compatibility - calls both compose and storage load constructors.
func (w *Writer) writePerFunctionConstructors(fn *ir.Function) {
	w.writeComposeConstructors(fn)
	w.writeStorageLoadConstructors(fn)
}

// writeComposeConstructors writes struct/array constructors needed by Compose expressions.
// Matches Rust naga's write_wrapped_expression_functions (Compose handling).
func (w *Writer) writeComposeConstructors(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		composeExpr, ok := expr.Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		if int(composeExpr.Type) < len(w.module.Types) {
			switch w.module.Types[composeExpr.Type].Inner.(type) {
			case ir.StructType:
				if _, written := w.structConstructorsWritten[composeExpr.Type]; !written {
					w.structConstructorsWritten[composeExpr.Type] = struct{}{}
					w.writeSingleStructConstructor(composeExpr.Type)
				}
			case ir.ArrayType:
				if _, written := w.arrayConstructorsWritten[composeExpr.Type]; !written {
					w.arrayConstructorsWritten[composeExpr.Type] = struct{}{}
					w.writeArrayConstructor(composeExpr.Type)
				}
			}
		}
	}
}

// writeStorageLoadConstructors writes constructors needed by Load expressions from storage.
// Matches Rust naga's write_wrapped_functions loop for Expression::Load.
func (w *Writer) writeStorageLoadConstructors(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		loadExpr, ok := expr.Kind.(ir.ExprLoad)
		if !ok {
			continue
		}
		w.registerStorageLoadConstructors(fn, loadExpr.Pointer)
	}
}

// writeRayQueryHelpers writes ray query helper functions needed by this function.
// Scans for RayQueryGetIntersection expressions and RayQuery Initialize statements
// to determine which helpers are needed.
func (w *Writer) writeRayQueryHelpers(fn *ir.Function) {
	// Scan for ray query expressions/statements
	for _, expr := range fn.Expressions {
		if rqgi, ok := expr.Kind.(ir.ExprRayQueryGetIntersection); ok {
			if rqgi.Committed {
				w.needsCommittedIntersectionHelper = true
			} else {
				w.needsCandidateIntersectionHelper = true
			}
		}
	}
	// Check for Initialize statements (need RayDescFromRayDesc_)
	w.scanForRayQueryInit(fn.Body)

	// Write RayDescFromRayDesc_ helper
	if w.needsRayDescHelper && !w.rayDescHelperWritten {
		w.rayDescHelperWritten = true
		// Find the RayDesc_ type name
		rayDescTypeName := "RayDesc_"
		for _, t := range w.module.Types {
			if t.Name == "RayDesc" {
				// The WGSL RayDesc becomes RayDesc_ in HLSL
				break
			}
		}
		w.out.WriteString("RayDesc RayDescFromRayDesc_(")
		w.out.WriteString(rayDescTypeName)
		w.out.WriteString(" arg0) {\n")
		w.out.WriteString("    RayDesc ret = (RayDesc)0;\n")
		w.out.WriteString("    ret.Origin = arg0.origin;\n")
		w.out.WriteString("    ret.TMin = arg0.tmin;\n")
		w.out.WriteString("    ret.Direction = arg0.dir;\n")
		w.out.WriteString("    ret.TMax = arg0.tmax;\n")
		w.out.WriteString("    return ret;\n")
		w.out.WriteString("}\n\n")
	}

	// Write GetCommittedIntersection helper
	if w.needsCommittedIntersectionHelper && !w.committedIntersectionHelperWritten {
		w.committedIntersectionHelperWritten = true
		riName := w.getRayIntersectionTypeName()
		w.out.WriteString(riName + " GetCommittedIntersection(RayQuery<RAY_FLAG_NONE> rq) {\n")
		w.out.WriteString("    " + riName + " ret = (" + riName + ")0;\n")
		w.out.WriteString("    ret.kind = rq.CommittedStatus();\n")
		w.out.WriteString("    if( rq.CommittedStatus() == COMMITTED_NOTHING) {} else {\n")
		w.out.WriteString("        ret.t = rq.CommittedRayT();\n")
		w.out.WriteString("        ret.instance_custom_data = rq.CommittedInstanceID();\n")
		w.out.WriteString("        ret.instance_index = rq.CommittedInstanceIndex();\n")
		w.out.WriteString("        ret.sbt_record_offset = rq.CommittedInstanceContributionToHitGroupIndex();\n")
		w.out.WriteString("        ret.geometry_index = rq.CommittedGeometryIndex();\n")
		w.out.WriteString("        ret.primitive_index = rq.CommittedPrimitiveIndex();\n")
		w.out.WriteString("        if( rq.CommittedStatus() == COMMITTED_TRIANGLE_HIT ) {\n")
		w.out.WriteString("            ret.barycentrics = rq.CommittedTriangleBarycentrics();\n")
		w.out.WriteString("            ret.front_face = rq.CommittedTriangleFrontFace();\n")
		w.out.WriteString("        }\n")
		w.out.WriteString("        ret.object_to_world = rq.CommittedObjectToWorld4x3();\n")
		w.out.WriteString("        ret.world_to_object = rq.CommittedWorldToObject4x3();\n")
		w.out.WriteString("    }\n")
		w.out.WriteString("    return ret;\n")
		w.out.WriteString("}\n\n")
	}

	// Write GetCandidateIntersection helper
	if w.needsCandidateIntersectionHelper && !w.candidateIntersectionHelperWritten {
		w.candidateIntersectionHelperWritten = true
		riName := w.getRayIntersectionTypeName()
		w.out.WriteString(riName + " GetCandidateIntersection(RayQuery<RAY_FLAG_NONE> rq) {\n")
		w.out.WriteString("    " + riName + " ret = (" + riName + ")0;\n")
		w.out.WriteString("    CANDIDATE_TYPE kind = rq.CandidateType();\n")
		w.out.WriteString("    if (kind == CANDIDATE_NON_OPAQUE_TRIANGLE) {\n")
		w.out.WriteString("        ret.kind = 1;\n") // Triangle = 1
		w.out.WriteString("        ret.t = rq.CandidateTriangleRayT();\n")
		w.out.WriteString("        ret.barycentrics = rq.CandidateTriangleBarycentrics();\n")
		w.out.WriteString("        ret.front_face = rq.CandidateTriangleFrontFace();\n")
		w.out.WriteString("    } else {\n")
		w.out.WriteString("        ret.kind = 3;\n") // Aabb = 3
		w.out.WriteString("    }\n")
		w.out.WriteString("    ret.instance_custom_data = rq.CandidateInstanceID();\n")
		w.out.WriteString("    ret.instance_index = rq.CandidateInstanceIndex();\n")
		w.out.WriteString("    ret.sbt_record_offset = rq.CandidateInstanceContributionToHitGroupIndex();\n")
		w.out.WriteString("    ret.geometry_index = rq.CandidateGeometryIndex();\n")
		w.out.WriteString("    ret.primitive_index = rq.CandidatePrimitiveIndex();\n")
		w.out.WriteString("    ret.object_to_world = rq.CandidateObjectToWorld4x3();\n")
		w.out.WriteString("    ret.world_to_object = rq.CandidateWorldToObject4x3();\n")
		w.out.WriteString("    return ret;\n")
		w.out.WriteString("}\n\n")
	}
}

// scanForRayQueryInit recursively scans statements for RayQuery Initialize.
func (w *Writer) scanForRayQueryInit(stmts []ir.Statement) {
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case ir.StmtRayQuery:
			if _, ok := s.Fun.(ir.RayQueryInitialize); ok {
				w.needsRayDescHelper = true
			}
		case ir.StmtBlock:
			w.scanForRayQueryInit(s.Block)
		case ir.StmtIf:
			w.scanForRayQueryInit(s.Accept)
			w.scanForRayQueryInit(s.Reject)
		case ir.StmtLoop:
			w.scanForRayQueryInit(s.Body)
			w.scanForRayQueryInit(s.Continuing)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				w.scanForRayQueryInit(c.Body)
			}
		}
	}
}

// getRayIntersectionTypeName returns the HLSL type name for the RayIntersection struct.
func (w *Writer) getRayIntersectionTypeName() string {
	// Look for the special RayIntersection type in the module
	if w.module.SpecialTypes.RayIntersection != nil {
		return w.getTypeName(*w.module.SpecialTypes.RayIntersection)
	}
	return "RayIntersection"
}

// writeNagaBufferLengthHelpers scans function expressions for ArrayLength
// and emits NagaBufferLength/NagaBufferLengthRW helper functions.
// Matches Rust naga's write_wrapped_array_length_function.
func (w *Writer) writeNagaBufferLengthHelpers(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		alExpr, ok := expr.Kind.(ir.ExprArrayLength)
		if !ok {
			continue
		}

		// Determine if writable by finding the global variable
		writable := false
		if int(alExpr.Array) < len(fn.Expressions) {
			arrExpr := fn.Expressions[alExpr.Array]
			var gvHandle ir.GlobalVariableHandle
			switch kind := arrExpr.Kind.(type) {
			case ir.ExprGlobalVariable:
				gvHandle = kind.Variable
			case ir.ExprAccessIndex:
				if int(kind.Base) < len(fn.Expressions) {
					if gv, ok := fn.Expressions[kind.Base].Kind.(ir.ExprGlobalVariable); ok {
						gvHandle = gv.Variable
					}
				}
			}
			if int(gvHandle) < len(w.module.GlobalVariables) {
				gv := &w.module.GlobalVariables[gvHandle]
				writable = gv.Space == ir.SpaceStorage && gv.Access == ir.StorageReadWrite
			}
		}

		if w.nagaBufferLengthWritten == nil {
			w.nagaBufferLengthWritten = make(map[bool]struct{})
		}
		if _, written := w.nagaBufferLengthWritten[writable]; written {
			continue
		}
		w.nagaBufferLengthWritten[writable] = struct{}{}

		accessStr := ""
		if writable {
			accessStr = "RW"
		}
		fmt.Fprintf(&w.out, "uint NagaBufferLength%s(%sByteAddressBuffer buffer)\n", accessStr, accessStr)
		fmt.Fprintf(&w.out, "{\n")
		fmt.Fprintf(&w.out, "    uint ret;\n")
		fmt.Fprintf(&w.out, "    buffer.GetDimensions(ret);\n")
		fmt.Fprintf(&w.out, "    return ret;\n")
		fmt.Fprintf(&w.out, "}\n\n")
	}
}

// registerStorageLoadConstructors checks if a Load from storage will need
// struct/array constructors and writes them if not already written.
func (w *Writer) registerStorageLoadConstructors(fn *ir.Function, pointer ir.ExpressionHandle) {
	// Check if this is a storage pointer
	if !w.isStoragePointerInFunc(fn, pointer) {
		return
	}
	// Get the loaded value type (what the Load returns)
	if int(pointer) >= len(fn.ExpressionTypes) {
		return
	}
	// The pointer's type is a pointer to the value type
	resolution := &fn.ExpressionTypes[pointer]
	var inner ir.TypeInner
	var resolvedHandle *ir.TypeHandle
	if resolution.Handle != nil {
		h := *resolution.Handle
		if int(h) < len(w.module.Types) {
			inner = w.module.Types[h].Inner
			resolvedHandle = resolution.Handle
		}
	} else {
		inner = resolution.Value
	}
	// Dereference pointer
	if ptr, ok := inner.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			inner = w.module.Types[ptr.Base].Inner
			w.registerConstructorsForType(ptr.Base, inner)
		}
	} else if resolvedHandle != nil {
		// ExpressionTypes gave us a direct type handle (e.g., AccessIndex through pointer
		// returns the member type directly, not wrapped in a Pointer). Use the handle directly.
		w.registerConstructorsForType(*resolvedHandle, inner)
	} else {
		// ExpressionTypes has a Value (inline type), find the handle by searching
		w.registerConstructorsForInner(inner)
	}
}

// registerConstructorsForType registers and writes constructors for a type if needed.
func (w *Writer) registerConstructorsForType(handle ir.TypeHandle, inner ir.TypeInner) {
	switch inner.(type) {
	case ir.StructType:
		// Register member constructors FIRST (dependency order: array constructors
		// must be declared before the struct constructor that references them).
		if st, ok := inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if int(member.Type) < len(w.module.Types) {
					memberInner := w.module.Types[member.Type].Inner
					w.registerConstructorsForType(member.Type, memberInner)
				}
			}
		}
		if _, written := w.structConstructorsWritten[handle]; !written {
			w.structConstructorsWritten[handle] = struct{}{}
			w.writeSingleStructConstructor(handle)
		}
	case ir.ArrayType:
		if _, written := w.arrayConstructorsWritten[handle]; !written {
			w.arrayConstructorsWritten[handle] = struct{}{}
			w.writeArrayConstructor(handle)
		}
	}
}

// registerConstructorsForInner registers constructors by scanning module types for a match.
func (w *Writer) registerConstructorsForInner(inner ir.TypeInner) {
	if inner == nil {
		return
	}
	// For non-pointer types, find the matching type handle
	handle := w.getTypeHandleForInner(inner)
	if handle != nil {
		w.registerConstructorsForType(*handle, inner)
	}
}

// isStoragePointerInFunc checks if an expression in a function is a storage pointer.
// Used during the pre-function scanning phase before currentFunction is set.
func (w *Writer) isStoragePointerInFunc(fn *ir.Function, handle ir.ExpressionHandle) bool {
	// Walk the expression chain to find the root global variable
	cur := handle
	for {
		if int(cur) >= len(fn.Expressions) {
			return false
		}
		expr := fn.Expressions[cur]
		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			if int(e.Variable) < len(w.module.GlobalVariables) {
				return w.module.GlobalVariables[e.Variable].Space == ir.SpaceStorage
			}
			return false
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		default:
			return false
		}
	}
}

// writeStorageLoadHelpers writes storage texture scalar load helpers.
// Called per-function before the function body, matching Rust naga ordering.
func (w *Writer) writeStorageLoadHelpers() {
	for _, ty := range sortedKeys(w.needsStorageLoadHelpers) {
		zero, one := "0.0", "1.0"
		if ty == "uint" {
			zero, one = "0u", "1u"
		} else if ty == "int" {
			zero, one = "0", "1"
		}
		w.writeLine("%s4 LoadedStorageValueFrom%s(%s arg) {%s4 ret = %s4(arg, %s, %s, %s);return ret;}", ty, ty, ty, ty, ty, zero, zero, one)
	}
	// Clear so we only write once
	w.needsStorageLoadHelpers = nil
}

// scanStorageTextureHelpers scans for storage textures with scalar formats
// that need LoadedStorageValueFrom{type} helper functions.
// Matches Rust naga: only generates helpers when ExprImageLoad references
// a single-component storage texture, not for all storage textures globally.
func (w *Writer) scanStorageTextureHelpers() {
	w.needsStorageLoadHelpers = make(map[string]bool)
	// Scan all functions (including entry points) for ImageLoad expressions
	// on single-component storage textures.
	allFuncs := make([]*ir.Function, 0, len(w.module.Functions)+len(w.module.EntryPoints))
	for i := range w.module.Functions {
		allFuncs = append(allFuncs, &w.module.Functions[i])
	}
	for i := range w.module.EntryPoints {
		allFuncs = append(allFuncs, &w.module.EntryPoints[i].Function)
	}
	for _, fn := range allFuncs {
		for _, expr := range fn.Expressions {
			imgLoad, ok := expr.Kind.(ir.ExprImageLoad)
			if !ok {
				continue
			}
			// Resolve the image expression to find its type
			imgType := w.resolveImageType(fn, imgLoad.Image)
			if imgType == nil {
				continue
			}
			if imgType.Class != ir.ImageClassStorage {
				continue
			}
			scalarName := storageFormatScalarName(imgType.StorageFormat)
			if scalarName != "" {
				w.needsStorageLoadHelpers[scalarName] = true
			}
		}
	}
}

// resolveImageType resolves the ImageType for an expression handle in a function.
func (w *Writer) resolveImageType(fn *ir.Function, handle ir.ExpressionHandle) *ir.ImageType {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	expr := fn.Expressions[handle]
	gv, ok := expr.Kind.(ir.ExprGlobalVariable)
	if !ok {
		return nil
	}
	if int(gv.Variable) >= len(w.module.GlobalVariables) {
		return nil
	}
	tyHandle := w.module.GlobalVariables[gv.Variable].Type
	if int(tyHandle) >= len(w.module.Types) {
		return nil
	}
	img, ok := w.module.Types[tyHandle].Inner.(ir.ImageType)
	if !ok {
		return nil
	}
	return &img
}

// storageFormatScalarName returns the HLSL scalar type name for scalar storage formats.
// Returns "" for non-scalar (vec2/vec4) formats that don't need the helper.
func storageFormatScalarName(format ir.StorageFormat) string {
	switch format {
	case ir.StorageFormatR16Float, ir.StorageFormatR32Float:
		return "float"
	case ir.StorageFormatR8Unorm, ir.StorageFormatR16Unorm,
		ir.StorageFormatR8Snorm, ir.StorageFormatR16Snorm:
		return "float" // unorm/snorm still use float helper
	case ir.StorageFormatR8Uint, ir.StorageFormatR16Uint, ir.StorageFormatR32Uint:
		return "uint"
	case ir.StorageFormatR8Sint, ir.StorageFormatR16Sint, ir.StorageFormatR32Sint:
		return "int"
	default:
		return "" // multi-component formats don't need scalar wrapper
	}
}

// sortedKeys returns sorted keys from a map[string]bool.
func sortedKeys(m map[string]bool) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// hlslTypeId generates a type identifier string for use in wrapper function names.
// Matches Rust naga's TypeInner::hlsl_type_id.
func (w *Writer) hlslTypeId(handle ir.TypeHandle) string {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("type_%d", handle)
	}
	typ := &w.module.Types[handle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return scalarTypeToHLSL(inner)
	case ir.VectorType:
		return vectorTypeToHLSL(inner)
	case ir.MatrixType:
		return matrixTypeToHLSL(inner)
	case ir.ArrayType:
		if inner.Size.Constant != nil {
			return fmt.Sprintf("array%d_%s_", *inner.Size.Constant, w.hlslTypeId(inner.Base))
		}
		return fmt.Sprintf("array_%s_", w.hlslTypeId(inner.Base))
	case ir.StructType:
		if name, ok := w.typeNames[handle]; ok && name != "" {
			return name
		}
		return typ.Name
	default:
		return w.getTypeName(handle)
	}
}

// writeZeroValueWrapperFunctions scans expressions for ZeroValue and writes
// wrapper functions. Matches Rust naga's write_wrapped_zero_value_functions.
func (w *Writer) writeZeroValueWrapperFunctions(exprs []ir.Expression) {
	for _, expr := range exprs {
		if zv, ok := expr.Kind.(ir.ExprZeroValue); ok {
			if _, written := w.wrappedZeroValues[zv.Type]; !written {
				w.wrappedZeroValues[zv.Type] = struct{}{}
				w.writeZeroValueWrapperFunction(zv.Type)
			}
		}
	}
}

// writeZeroValueWrapperFunction writes a single ZeroValue wrapper function.
// Matches Rust naga's write_wrapped_zero_value_function.
func (w *Writer) writeZeroValueWrapperFunction(handle ir.TypeHandle) {
	typeId := w.hlslTypeId(handle)
	typeName := w.getTypeName(handle)

	// For arrays, need typedef + ret_ prefix
	if int(handle) < len(w.module.Types) {
		if arr, ok := w.module.Types[handle].Inner.(ir.ArrayType); ok {
			baseName, arraySuffix := w.getTypeNameWithArraySuffix(handle)
			_ = arr
			w.writeLine("typedef %s ret_ZeroValue%s%s;", baseName, typeId, arraySuffix)
			w.writeLine("ret_ZeroValue%s ZeroValue%s() {", typeId, typeId)
			w.pushIndent()
			w.writeLine("return (%s%s)0;", baseName, arraySuffix)
			w.popIndent()
			w.writeLine("}")
			w.writeLine("")
			return
		}
	}

	w.writeLine("%s ZeroValue%s() {", typeName, typeId)
	w.pushIndent()
	w.writeLine("return (%s)0;", typeName)
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeFunctions writes regular function definitions.
// Entry point functions are skipped here — they are written by writeEntryPoints
// with proper I/O structs and semantics.
func (w *Writer) writeFunctions() error {
	for handle := range w.module.Functions {
		if w.isEntryPointFunction(ir.FunctionHandle(handle)) {
			continue
		}
		fn := &w.module.Functions[handle]
		if err := w.writeFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
	}
	return nil
}

// writeFunction writes a single function definition.
func (w *Writer) writeFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	// Write per-function wrapped helpers (matching Rust naga order)
	w.writePerFunctionWrappedHelpers(fn)

	w.currentFunction = fn
	w.currentFuncHandle = handle
	w.currentEPIndex = -1
	w.localNames = make(map[uint32]string)
	w.updateExpressionsToBake(fn)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.continueCtx.clear()

	name := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}]

	// Return type — arrays need typedef
	var returnType string
	if fn.Result != nil {
		if int(fn.Result.Type) < len(w.module.Types) {
			if _, ok := w.module.Types[fn.Result.Type].Inner.(ir.ArrayType); ok {
				// Array return type needs typedef
				baseTypeName, arraySuffix := w.getTypeNameWithArraySuffix(fn.Result.Type)
				retTypeName := fmt.Sprintf("ret_%s", name)
				w.writeLine("typedef %s %s%s;", baseTypeName, retTypeName, arraySuffix)
				returnType = retTypeName
			} else {
				returnType = w.getTypeName(fn.Result.Type)
			}
		} else {
			returnType = w.getTypeName(fn.Result.Type)
		}
	} else {
		returnType = hlslVoidType
	}

	// Arguments — pointers become inout, arrays need suffix after param name
	// External texture arguments are expanded into 3 planes + params.
	args := make([]string, 0, len(fn.Arguments))
	for argIdx, arg := range fn.Arguments {
		argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}]

		// External texture arguments: expand into 3 planes + params
		if w.isExternalTexture(arg.Type) {
			paramsType := w.getExternalTextureParamsTypeName()
			var names [4]string
			for i := 0; i < 3; i++ {
				names[i] = w.namer.call(fmt.Sprintf("%s_plane%d_", argName, i))
			}
			names[3] = w.namer.call(fmt.Sprintf("%s_params", argName))
			w.externalTextureFuncArgNames[externalTextureFuncArgKey{
				funcHandle: handle,
				argIndex:   uint32(argIdx),
			}] = names
			args = append(args,
				fmt.Sprintf("Texture2D<float4> %s", names[0]),
				fmt.Sprintf("Texture2D<float4> %s", names[1]),
				fmt.Sprintf("Texture2D<float4> %s", names[2]),
				fmt.Sprintf("%s %s", paramsType, names[3]),
			)
			continue
		}

		argTypeHandle := arg.Type
		prefix := ""
		// Pointer arguments become inout (matching Rust naga)
		if int(argTypeHandle) < len(w.module.Types) {
			if ptr, ok := w.module.Types[argTypeHandle].Inner.(ir.PointerType); ok {
				prefix = "inout "
				argTypeHandle = ptr.Base
			}
		}
		argType, argSuffix := w.getTypeNameWithArraySuffix(argTypeHandle)
		args = append(args, fmt.Sprintf("%s%s %s%s", prefix, argType, argName, argSuffix))
	}

	// Rust naga puts the opening brace on the next line
	w.writeLine("%s %s(%s)", returnType, name, strings.Join(args, ", "))
	w.writeLine("{")
	w.pushIndent()

	// Write function body (local variables + statements)
	if err := w.writeFunctionBody(fn); err != nil {
		w.popIndent()
		w.currentFunction = nil
		return err
	}

	// Note: Rust naga does NOT add implicit return for void functions.
	// The IR's ensureBlockReturns handles adding returns where needed.
	// Void functions in HLSL don't need trailing return;.

	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	w.currentFunction = nil
	return nil
}

// writeEntryPoints writes entry point functions.
func (w *Writer) writeEntryPoints() error {
	for epIdx := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[epIdx]
		// Skip if not the selected entry point
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}

		if err := w.writeEntryPointWithIO(epIdx, ep); err != nil {
			return err
		}
	}
	return nil
}

// Output helpers

// writeLine writes a line with indentation and newline.
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

// hlslBlockEndsWithReturn checks if a block's last statement terminates
// all control flow paths with a return (or other terminator like kill).
// Matches Rust naga behavior: Switch/If that terminate all paths count.
func hlslBlockEndsWithReturn(block ir.Block) bool {
	if len(block) == 0 {
		return false
	}
	last := block[len(block)-1]
	switch s := last.Kind.(type) {
	case ir.StmtReturn, ir.StmtKill:
		return true
	case ir.StmtIf:
		return hlslBlockEndsWithReturn(s.Accept) && hlslBlockEndsWithReturn(s.Reject)
	case ir.StmtSwitch:
		for i := range s.Cases {
			if !s.Cases[i].FallThrough {
				if !hlslBlockEndsWithReturn(s.Cases[i].Body) {
					return false
				}
			}
		}
		return len(s.Cases) > 0 // at least one non-fall-through case returns
	case ir.StmtBlock:
		return hlslBlockEndsWithReturn(s.Block)
	default:
		return false
	}
}

// NOTE: getTypeName, typeToHLSL, float32FromBits are implemented in types.go
