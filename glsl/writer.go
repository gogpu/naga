// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"
	"math"
	"strings"

	"github.com/gogpu/naga/ir"
)

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
)

// Writer generates GLSL source code from IR.
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

	// Texture-sampler pair tracking (WGSL separates, GLSL combines).
	// In WGSL, textures and samplers are separate globals. In GLSL, they must
	// be combined into e.g. "uniform sampler2D tex_sampler;".
	textureSamplerPairs []string

	// combinedSamplers maps a texture-sampler pair key to its combined GLSL name.
	// The key is "textureHandle:samplerHandle".
	combinedSamplers map[string]*combinedSamplerInfo

	// globalIsCombined tracks which global variable handles have been absorbed
	// into a combined sampler and should be skipped in writeGlobalVariables.
	globalIsCombined map[ir.GlobalVariableHandle]bool

	// Function context (set during function writing)
	currentFunction   *ir.Function
	currentFuncHandle ir.FunctionHandle
	localNames        map[uint32]string
	namedExpressions  map[ir.ExpressionHandle]string

	// Entry point context
	inEntryPoint     bool
	entryPointResult *ir.FunctionResult

	// Entry point struct IO flattening.
	// Maps argument index to the flattened struct member info.
	// When a vertex/fragment entry point argument is a struct type, the struct
	// members become individual layout(location=N) in declarations instead
	// of a single argument. This map tracks which arguments are flattened.
	epStructArgs map[uint32]*epStructInfo

	// Entry point struct output flattening.
	// When an entry point returns a struct type, this tracks the struct info
	// so that return statements can be expanded into individual assignments.
	epStructOutput *epStructInfo

	// Expression baking (expressions that need to be materialized to temporaries)
	needBakeExpression map[ir.ExpressionHandle]struct{}

	// Output tracking
	entryPointNames map[string]string
	extensions      []string
	requiredVersion Version

	// Helper function flags
	needsModHelper bool
	needsDivHelper bool
}

// epStructMemberInfo holds the GLSL variable name and binding for a flattened struct member.
type epStructMemberInfo struct {
	// glslName is the GLSL variable name emitted for this member.
	glslName string
	// isBuiltin is true if this member is bound to a GLSL built-in (gl_Position, etc.).
	isBuiltin bool
	// builtinName is the GLSL built-in name (only valid when isBuiltin is true).
	builtinName string
}

// epStructInfo tracks a flattened entry point struct argument or result.
type epStructInfo struct {
	// structType is the IR struct type handle.
	structType ir.TypeHandle
	// members maps struct member index to the resolved GLSL info.
	members []epStructMemberInfo
}

// combinedSamplerInfo holds information about a combined texture-sampler pair
// for GLSL output. WGSL has separate texture and sampler types, but GLSL
// requires them to be declared together as e.g. "uniform sampler2D name;".
type combinedSamplerInfo struct {
	// glslName is the combined GLSL variable name (e.g., "tex_texSampler").
	glslName string
	// textureHandle is the GlobalVariableHandle for the texture.
	textureHandle ir.GlobalVariableHandle
	// samplerHandle is the GlobalVariableHandle for the sampler.
	samplerHandle ir.GlobalVariableHandle
	// glslTypeName is the combined GLSL type (e.g., "sampler2D").
	glslTypeName string
	// binding is the binding index to use in the layout qualifier (from texture).
	binding *ir.ResourceBinding
}

// namer generates unique identifiers.
type namer struct {
	usedNames map[string]struct{}
	counter   uint32
}

func newNamer() *namer {
	return &namer{
		usedNames: make(map[string]struct{}),
	}
}

// call generates a unique name based on the given base.
func (n *namer) call(base string) string {
	// Escape reserved words
	escaped := escapeKeyword(base)

	// First try the base name directly
	if _, used := n.usedNames[escaped]; !used {
		n.usedNames[escaped] = struct{}{}
		return escaped
	}

	// Add numeric suffix
	for {
		n.counter++
		candidate := fmt.Sprintf("%s_%d", escaped, n.counter)
		if _, used := n.usedNames[candidate]; !used {
			n.usedNames[candidate] = struct{}{}
			return candidate
		}
	}
}

// newWriter creates a new GLSL writer.
func newWriter(module *ir.Module, options *Options) *Writer {
	return &Writer{
		module:             module,
		options:            options,
		names:              make(map[nameKey]string),
		namer:              newNamer(),
		typeNames:          make(map[ir.TypeHandle]string),
		entryPointNames:    make(map[string]string),
		namedExpressions:   make(map[ir.ExpressionHandle]string),
		needBakeExpression: make(map[ir.ExpressionHandle]struct{}),
		epStructArgs:       make(map[uint32]*epStructInfo),
		combinedSamplers:   make(map[string]*combinedSamplerInfo),
		globalIsCombined:   make(map[ir.GlobalVariableHandle]bool),
		requiredVersion:    options.LangVersion,
	}
}

// String returns the generated GLSL source code.
func (w *Writer) String() string {
	return w.out.String()
}

// writeModule generates GLSL code for the entire module.
func (w *Writer) writeModule() error {
	// 1. Write version directive
	w.writeVersionDirective()

	// 2. Write precision qualifiers (ES only)
	w.writePrecisionQualifiers()

	// 3. Register all names
	if err := w.registerNames(); err != nil {
		return err
	}

	// 3b. Scan for texture-sampler pairs (WGSL separate → GLSL combined)
	w.scanTextureSamplerPairs()

	// 4. Write type definitions (structs)
	if err := w.writeTypes(); err != nil {
		return err
	}

	// 5. Write constants
	if err := w.writeConstants(); err != nil {
		return err
	}

	// 6. Write global variables (uniforms, inputs, outputs)
	if err := w.writeGlobalVariables(); err != nil {
		return err
	}

	// 7. Write helper functions if needed
	w.writeHelperFunctions()

	// 8. Write regular functions
	if err := w.writeFunctions(); err != nil {
		return err
	}

	// 9. Write entry points
	return w.writeEntryPoints()
}

// writeVersionDirective writes the #version directive.
func (w *Writer) writeVersionDirective() {
	w.writeLine("#version %s", w.options.LangVersion.String())
	w.writeLine("")
}

// writePrecisionQualifiers writes precision qualifiers for ES.
func (w *Writer) writePrecisionQualifiers() {
	if !w.options.LangVersion.ES {
		return
	}

	// ES requires precision qualifiers
	w.writeLine("precision highp float;")
	w.writeLine("precision highp int;")
	w.writeLine("precision highp sampler2D;")
	w.writeLine("precision highp sampler3D;")
	w.writeLine("precision highp samplerCube;")
	w.writeLine("")
}

// registerNames assigns unique names to all IR entities.
//
//nolint:gocognit // Name registration requires handling all IR entity types
func (w *Writer) registerNames() error {
	// Register type names
	for handle, typ := range w.module.Types {
		var baseName string
		if typ.Name != "" {
			baseName = typ.Name
		} else {
			baseName = fmt.Sprintf("type_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyType, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index
		w.typeNames[ir.TypeHandle(handle)] = name                           //nolint:gosec // G115: handle is valid slice index

		// Register struct member names
		if st, ok := typ.Inner.(ir.StructType); ok {
			for memberIdx, member := range st.Members {
				memberName := member.Name
				if memberName == "" {
					memberName = fmt.Sprintf("member_%d", memberIdx)
				}
				w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}] = escapeKeyword(memberName) //nolint:gosec // G115: handle is valid slice index
			}
		}
	}

	// Register constant names
	for handle, constant := range w.module.Constants {
		var baseName string
		if constant.Name != "" {
			baseName = constant.Name
		} else {
			baseName = fmt.Sprintf("const_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index
	}

	// Register global variable names
	for handle, global := range w.module.GlobalVariables {
		var baseName string
		if global.Name != "" {
			baseName = global.Name
		} else {
			baseName = fmt.Sprintf("global_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index
	}

	// Register function names
	for handle := range w.module.Functions {
		fn := &w.module.Functions[handle]
		var baseName string
		if fn.Name != "" {
			baseName = fn.Name
		} else {
			baseName = fmt.Sprintf("function_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}] = name //nolint:gosec // G115: handle is valid slice index

		// Register function argument names
		for argIdx, arg := range fn.Arguments {
			argName := arg.Name
			if argName == "" {
				argName = fmt.Sprintf("arg_%d", argIdx)
			}
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}] = escapeKeyword(argName) //nolint:gosec // G115: handle is valid slice index
		}
	}

	// Register entry point names
	for epIdx, ep := range w.module.EntryPoints {
		// Entry point main function is always named "main" in GLSL
		name := "main"
		if len(w.module.EntryPoints) > 1 {
			// Multiple entry points not directly supported in single GLSL file
			// We'll emit the selected one as main
			if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
				continue
			}
		}
		w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}] = name //nolint:gosec // G115: epIdx is valid slice index
		w.entryPointNames[ep.Name] = name
	}

	return nil
}

// scanTextureSamplerPairs scans all functions and entry points for ExprImageSample
// expressions to discover which texture globals are paired with which sampler globals.
// GLSL has no separate texture/sampler types, so each pair must be emitted as a
// single "uniform sampler2D" declaration.
func (w *Writer) scanTextureSamplerPairs() {
	for i := range w.module.Functions {
		fn := &w.module.Functions[i]
		w.scanFunctionForPairs(fn)
	}
}

// scanFunctionForPairs scans a single function's expressions for ExprImageSample.
func (w *Writer) scanFunctionForPairs(fn *ir.Function) {
	for _, expr := range fn.Expressions {
		sample, ok := expr.Kind.(ir.ExprImageSample)
		if !ok {
			continue
		}
		w.registerTextureSamplerPair(fn, sample)
	}
}

// registerTextureSamplerPair resolves the texture and sampler expression handles
// back to their GlobalVariableHandles and creates a combined sampler entry.
func (w *Writer) registerTextureSamplerPair(fn *ir.Function, sample ir.ExprImageSample) {
	imageHandle := w.resolveGlobalVarHandle(fn, sample.Image)
	samplerHandle := w.resolveGlobalVarHandle(fn, sample.Sampler)
	if imageHandle == nil || samplerHandle == nil {
		return
	}

	pairKey := combinedPairKey(*imageHandle, *samplerHandle)
	if _, exists := w.combinedSamplers[pairKey]; exists {
		return // Already registered
	}

	texName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(*imageHandle)}]
	samplerName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(*samplerHandle)}]
	combinedName := texName + "_" + samplerName

	// Determine the GLSL combined sampler type from the texture's ImageType.
	glslType := "sampler2D" // default
	texGlobal := &w.module.GlobalVariables[*imageHandle]
	if int(texGlobal.Type) < len(w.module.Types) {
		if imgType, ok := w.module.Types[texGlobal.Type].Inner.(ir.ImageType); ok {
			glslType = w.imageToGLSL(imgType)
		}
	}

	info := &combinedSamplerInfo{
		glslName:      combinedName,
		textureHandle: *imageHandle,
		samplerHandle: *samplerHandle,
		glslTypeName:  glslType,
		binding:       texGlobal.Binding,
	}

	w.combinedSamplers[pairKey] = info
	w.globalIsCombined[*imageHandle] = true
	w.globalIsCombined[*samplerHandle] = true
}

// resolveGlobalVarHandle traces an expression handle back to its GlobalVariableHandle.
// It follows ExprLoad indirections since the IR may wrap globals in a Load.
func (w *Writer) resolveGlobalVarHandle(fn *ir.Function, exprHandle ir.ExpressionHandle) *ir.GlobalVariableHandle {
	if int(exprHandle) >= len(fn.Expressions) {
		return nil
	}
	expr := &fn.Expressions[exprHandle]

	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		h := k.Variable
		return &h
	case ir.ExprLoad:
		// Follow through Load to the pointer expression
		return w.resolveGlobalVarHandle(fn, k.Pointer)
	default:
		return nil
	}
}

// combinedPairKey returns a deterministic map key for a texture-sampler pair.
func combinedPairKey(textureHandle, samplerHandle ir.GlobalVariableHandle) string {
	return fmt.Sprintf("%d:%d", textureHandle, samplerHandle)
}

// getCombinedSamplerName returns the combined name for a texture-sampler pair,
// or empty string if not combined.
func (w *Writer) getCombinedSamplerName(textureHandle, samplerHandle ir.GlobalVariableHandle) string {
	key := combinedPairKey(textureHandle, samplerHandle)
	if info, ok := w.combinedSamplers[key]; ok {
		return info.glslName
	}
	return ""
}

// writeTypes writes struct type definitions.
func (w *Writer) writeTypes() error {
	for handle, typ := range w.module.Types {
		st, ok := typ.Inner.(ir.StructType)
		if !ok {
			continue
		}

		typeName := w.typeNames[ir.TypeHandle(handle)] //nolint:gosec // G115: handle is valid slice index
		w.writeLine("struct %s {", typeName)
		w.pushIndent()

		for memberIdx, member := range st.Members {
			baseType := w.getBaseTypeName(member.Type)
			arraySuffix := w.getArraySuffix(member.Type)
			memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}] //nolint:gosec // G115: handle is valid slice index
			w.writeLine("%s %s%s;", baseType, memberName, arraySuffix)
		}

		w.popIndent()
		w.writeLine("};")
		w.writeLine("")
	}
	return nil
}

// writeConstants writes constant definitions.
func (w *Writer) writeConstants() error {
	for handle, constant := range w.module.Constants {
		name := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}] //nolint:gosec // G115: handle is valid slice index
		baseType := w.getBaseTypeName(constant.Type)
		arraySuffix := w.getArraySuffix(constant.Type)
		value := w.writeConstantValue(constant)
		w.writeLine("const %s %s%s = %s;", baseType, name, arraySuffix, value)
	}
	if len(w.module.Constants) > 0 {
		w.writeLine("")
	}
	return nil
}

// writeConstantValue returns the GLSL representation of a constant value.
func (w *Writer) writeConstantValue(constant ir.Constant) string {
	switch v := constant.Value.(type) {
	case ir.ScalarValue:
		return w.writeScalarValue(v, constant.Type)
	case ir.CompositeValue:
		return w.writeCompositeValue(v, constant.Type)
	default:
		return "0" // Unknown value type
	}
}

// writeScalarValue returns the GLSL representation of a scalar value.
func (w *Writer) writeScalarValue(v ir.ScalarValue, typeHandle ir.TypeHandle) string {
	switch v.Kind {
	case ir.ScalarBool:
		if v.Bits != 0 {
			return "true"
		}
		return "false"
	case ir.ScalarSint:
		return fmt.Sprintf("%d", int32(v.Bits))
	case ir.ScalarUint:
		return fmt.Sprintf("%du", uint32(v.Bits))
	case ir.ScalarFloat:
		// Get width from type
		width := uint8(4) // Default to 32-bit
		if int(typeHandle) < len(w.module.Types) {
			if scalar, ok := w.module.Types[typeHandle].Inner.(ir.ScalarType); ok {
				width = scalar.Width
			}
		}
		if width == 4 {
			floatVal := math.Float32frombits(uint32(v.Bits))
			return formatFloat(floatVal)
		}
		// 64-bit float
		floatVal := math.Float64frombits(v.Bits)
		return formatFloat64(floatVal)
	default:
		return "0"
	}
}

// writeCompositeValue returns the GLSL representation of a composite value.
func (w *Writer) writeCompositeValue(v ir.CompositeValue, typeHandle ir.TypeHandle) string {
	typeName := w.getTypeName(typeHandle)
	var components []string
	for _, compHandle := range v.Components {
		if int(compHandle) < len(w.module.Constants) {
			constant := w.module.Constants[compHandle]
			components = append(components, w.writeConstantValue(constant))
		} else {
			components = append(components, "0")
		}
	}
	return fmt.Sprintf("%s(%s)", typeName, strings.Join(components, ", "))
}

// writeGlobalVariables writes uniform, input, and output declarations.
// Texture and sampler globals that are part of combined pairs are skipped here;
// their combined declarations are emitted by writeCombinedSamplerDeclarations.
func (w *Writer) writeGlobalVariables() error {
	// First, emit combined texture-sampler declarations.
	w.writeCombinedSamplerDeclarations()

	for handle, global := range w.module.GlobalVariables {
		// Skip globals that have been absorbed into combined samplers.
		if w.globalIsCombined[ir.GlobalVariableHandle(handle)] { //nolint:gosec // G115: handle is valid slice index
			continue
		}

		name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}] //nolint:gosec // G115: handle is valid slice index
		typeName := w.getTypeName(global.Type)

		switch global.Space {
		case ir.SpaceUniform:
			w.writeUniformVariable(name, typeName, global)
		case ir.SpaceStorage:
			w.writeStorageVariable(name, typeName, global)
		case ir.SpacePrivate:
			w.writeLine("%s %s;", typeName, name)
		case ir.SpaceWorkGroup:
			w.writeLine("shared %s %s;", typeName, name)
		default:
			w.writeLine("%s %s;", typeName, name)
		}
	}
	if len(w.module.GlobalVariables) > 0 || len(w.combinedSamplers) > 0 {
		w.writeLine("")
	}
	return nil
}

// writeCombinedSamplerDeclarations emits "uniform sampler2D name;" declarations
// for each discovered texture-sampler pair.
func (w *Writer) writeCombinedSamplerDeclarations() {
	// Collect entries for deterministic output order.
	pairs := make([]*combinedSamplerInfo, 0, len(w.combinedSamplers))
	for _, info := range w.combinedSamplers {
		pairs = append(pairs, info)
	}

	// Sort by texture handle then sampler handle for deterministic order.
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].textureHandle > pairs[j].textureHandle ||
				(pairs[i].textureHandle == pairs[j].textureHandle &&
					pairs[i].samplerHandle > pairs[j].samplerHandle) {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	for _, info := range pairs {
		w.textureSamplerPairs = append(w.textureSamplerPairs, info.glslName)

		if info.binding != nil {
			binding := info.binding.Binding + w.options.TextureBindingBase
			w.writeLine("layout(binding = %d) uniform %s %s;", binding, info.glslTypeName, info.glslName)
		} else {
			w.writeLine("uniform %s %s;", info.glslTypeName, info.glslName)
		}
	}
}

// writeUniformVariable writes a uniform declaration.
// For struct types in SpaceUniform, emits a GLSL uniform block (UBO) instead
// of a plain struct uniform. WebGPU var<uniform> maps to UBOs which require
// the block syntax: "uniform BlockName { members } instanceName;"
// Plain "uniform StructType varName;" is set via glUniform*, NOT via
// glBindBufferRange, so UBO data would never reach the shader.
func (w *Writer) writeUniformVariable(name, typeName string, global ir.GlobalVariable) {
	// Check if the type is a struct — if so, emit as a uniform block (UBO).
	if int(global.Type) < len(w.module.Types) {
		if st, ok := w.module.Types[global.Type].Inner.(ir.StructType); ok {
			w.writeUniformBlock(name, typeName, global, st)
			return
		}
	}

	// Non-struct uniform (plain uniform variable)
	if global.Binding != nil {
		binding := global.Binding.Binding + w.options.UniformBindingBase
		w.writeLine("layout(binding = %d) uniform %s %s;", binding, typeName, name)
	} else {
		w.writeLine("uniform %s %s;", typeName, name)
	}
}

// writeUniformBlock emits a GLSL uniform block (UBO) for a struct type.
// Uses std140 layout which matches WebGPU uniform buffer layout rules.
// Block name uses "_typeName_ubo" suffix to avoid conflict with the struct
// type definition (GLSL shares namespace between struct and block names).
func (w *Writer) writeUniformBlock(name, typeName string, global ir.GlobalVariable, st ir.StructType) {
	blockName := "_" + typeName + "_ubo"
	if global.Binding != nil {
		binding := global.Binding.Binding + w.options.UniformBindingBase
		w.writeLine("layout(std140, binding = %d) uniform %s {", binding, blockName)
	} else {
		w.writeLine("uniform %s {", blockName)
	}
	w.pushIndent()

	for memberIdx, member := range st.Members {
		baseType := w.getBaseTypeName(member.Type)
		arraySuffix := w.getArraySuffix(member.Type)
		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(global.Type), handle2: uint32(memberIdx)}] //nolint:gosec // G115: memberIdx bounded by slice
		w.writeLine("%s %s%s;", baseType, memberName, arraySuffix)
	}

	w.popIndent()
	w.writeLine("} %s;", name)
}

// writeStorageVariable writes a storage buffer declaration.
func (w *Writer) writeStorageVariable(name, typeName string, global ir.GlobalVariable) {
	if !w.options.LangVersion.SupportsStorageBuffers() {
		// Fall back to uniform for older versions
		w.writeUniformVariable(name, typeName, global)
		return
	}

	if global.Binding != nil {
		binding := global.Binding.Binding + w.options.StorageBindingBase
		w.writeLine("layout(std430, binding = %d) buffer %s_block { %s %s; };", binding, name, typeName, name)
	} else {
		w.writeLine("buffer %s_block { %s %s; };", name, typeName, name)
	}
}

// writeHelperFunctions writes any needed polyfill functions.
func (w *Writer) writeHelperFunctions() {
	if w.needsModHelper {
		w.writeLine("// Safe modulo helper (truncated division semantics)")
		w.writeLine("int _naga_mod(int a, int b) {")
		w.pushIndent()
		w.writeLine("return a - b * (a / b);")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}

	if w.needsDivHelper {
		w.writeLine("// Safe division helper (handles zero divisor)")
		w.writeLine("int _naga_div(int a, int b) {")
		w.pushIndent()
		w.writeLine("return b != 0 ? a / b : 0;")
		w.popIndent()
		w.writeLine("}")
		w.writeLine("")
	}
}

// writeFunctions writes regular function definitions.
// Entry point functions are skipped — they are emitted by writeEntryPoints as void main().
func (w *Writer) writeFunctions() error {
	// Collect entry point function handles to skip them
	epFunctions := make(map[ir.FunctionHandle]bool, len(w.module.EntryPoints))
	for _, ep := range w.module.EntryPoints {
		epFunctions[ep.Function] = true
	}

	for handle := range w.module.Functions {
		if epFunctions[ir.FunctionHandle(handle)] { //nolint:gosec // G115: handle is valid slice index
			continue
		}
		fn := &w.module.Functions[handle]
		if err := w.writeFunction(ir.FunctionHandle(handle), fn); err != nil { //nolint:gosec // G115: handle is valid slice index
			return err
		}
	}
	return nil
}

// writeFunction writes a single function definition.
func (w *Writer) writeFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	w.currentFunction = fn
	w.currentFuncHandle = handle
	w.localNames = make(map[uint32]string)

	name := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}]

	// Return type
	var returnType string
	if fn.Result != nil {
		returnType = w.getTypeName(fn.Result.Type)
	} else {
		returnType = "void"
	}

	// Arguments
	args := make([]string, 0, len(fn.Arguments))
	for argIdx, arg := range fn.Arguments {
		argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}] //nolint:gosec // G115: argIdx is bounded by slice length
		argType := w.getTypeName(arg.Type)
		args = append(args, fmt.Sprintf("%s %s", argType, argName))
	}

	w.writeLine("%s %s(%s) {", returnType, name, strings.Join(args, ", "))
	w.pushIndent()

	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	if err := w.writeBlock(ir.Block(fn.Body)); err != nil {
		return err
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	w.currentFunction = nil
	return nil
}

// writeEntryPoints writes entry point functions.
func (w *Writer) writeEntryPoints() error {
	for epIdx, ep := range w.module.EntryPoints {
		// Skip if not the selected entry point
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}

		if err := w.writeEntryPoint(epIdx, &ep); err != nil {
			return err
		}
	}
	return nil
}

// writeEntryPoint writes a single entry point.
func (w *Writer) writeEntryPoint(epIdx int, ep *ir.EntryPoint) error {
	// Look up actual function from handle
	fn := &w.module.Functions[ep.Function]
	w.currentFunction = fn
	w.currentFuncHandle = ep.Function
	w.localNames = make(map[uint32]string)
	w.inEntryPoint = true
	w.entryPointResult = fn.Result
	w.epStructArgs = make(map[uint32]*epStructInfo)
	w.epStructOutput = nil

	// Write input/output declarations for vertex/fragment
	switch ep.Stage {
	case ir.StageVertex:
		w.writeVertexIO(ep, fn)
	case ir.StageFragment:
		w.writeFragmentIO(ep, fn)
	case ir.StageCompute:
		w.writeComputeLayout(ep)
	}

	// Main function
	w.writeLine("void main() {")
	w.pushIndent()

	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	// Write function body
	if err := w.writeBlock(ir.Block(fn.Body)); err != nil {
		return err
	}

	w.popIndent()
	w.writeLine("}")

	w.currentFunction = nil
	w.inEntryPoint = false
	w.entryPointResult = nil
	w.epStructArgs = nil
	w.epStructOutput = nil
	_ = epIdx // Used for name lookup
	return nil
}

// writeVertexIO writes vertex shader input/output declarations.
func (w *Writer) writeVertexIO(_ *ir.EntryPoint, fn *ir.Function) {
	// Write input attributes
	for argIdx, arg := range fn.Arguments {
		if arg.Binding != nil {
			// Direct binding on argument (scalar/vector input)
			if loc, ok := (*arg.Binding).(ir.LocationBinding); ok {
				baseType := w.getBaseTypeName(arg.Type)
				arraySuffix := w.getArraySuffix(arg.Type)
				name := escapeKeyword(arg.Name)
				w.writeLine("layout(location = %d) in %s %s%s;", loc.Location, baseType, name, arraySuffix)
			}
			// BuiltinBinding: no declaration needed (gl_VertexID, gl_InstanceID are built-in)
		} else {
			// No direct binding — check if this is a struct with member bindings
			w.writeStructArgIO(uint32(argIdx), arg.Type, "in", false) //nolint:gosec // G115: argIdx bounded by slice
		}
	}

	// Write output varyings
	w.writeResultIO(fn, "out", true)
	w.writeLine("")
}

// writeFragmentIO writes fragment shader input/output declarations.
func (w *Writer) writeFragmentIO(_ *ir.EntryPoint, fn *ir.Function) {
	// Write input varyings from vertex shader
	for argIdx, arg := range fn.Arguments {
		if arg.Binding != nil {
			// Direct binding on argument
			if loc, ok := (*arg.Binding).(ir.LocationBinding); ok {
				baseType := w.getBaseTypeName(arg.Type)
				arraySuffix := w.getArraySuffix(arg.Type)
				name := escapeKeyword(arg.Name)
				w.writeLine("layout(location = %d) in %s %s%s;", loc.Location, baseType, name, arraySuffix)
			}
			// BuiltinBinding: no declaration needed (gl_FragCoord etc. are built-in)
		} else {
			// No direct binding — check if this is a struct with member bindings
			w.writeStructArgIO(uint32(argIdx), arg.Type, "in", false) //nolint:gosec // G115: argIdx bounded by slice
		}
	}

	// Write output colors
	w.writeResultIO(fn, "out", false)
	w.writeLine("")
}

// writeStructArgIO flattens a struct-typed entry point argument into individual IO declarations.
// It populates w.epStructArgs for later use by expression writers.
func (w *Writer) writeStructArgIO(argIdx uint32, typeHandle ir.TypeHandle, qualifier string, isOutput bool) {
	if int(typeHandle) >= len(w.module.Types) {
		return
	}
	st, ok := w.module.Types[typeHandle].Inner.(ir.StructType)
	if !ok {
		return
	}

	info := &epStructInfo{
		structType: typeHandle,
		members:    make([]epStructMemberInfo, len(st.Members)),
	}

	for memberIdx, member := range st.Members {
		if member.Binding == nil {
			info.members[memberIdx] = epStructMemberInfo{
				glslName: escapeKeyword(member.Name),
			}
			continue
		}
		switch b := (*member.Binding).(type) {
		case ir.LocationBinding:
			baseType := w.getBaseTypeName(member.Type)
			name := escapeKeyword(member.Name)
			w.writeLine("layout(location = %d) %s %s %s;", b.Location, qualifier, baseType, name)
			info.members[memberIdx] = epStructMemberInfo{
				glslName: name,
			}
		case ir.BuiltinBinding:
			builtinName := glslBuiltIn(b.Builtin, isOutput)
			info.members[memberIdx] = epStructMemberInfo{
				isBuiltin:   true,
				builtinName: builtinName,
				glslName:    builtinName,
			}
		}
	}

	w.epStructArgs[argIdx] = info
}

// writeResultIO writes output declarations for a function result.
// Handles both direct-binding results and struct results with member bindings.
func (w *Writer) writeResultIO(fn *ir.Function, qualifier string, isVertexOutput bool) {
	if fn.Result == nil {
		return
	}

	if fn.Result.Binding != nil {
		// Direct binding on result
		switch b := (*fn.Result.Binding).(type) {
		case ir.LocationBinding:
			baseType := w.getBaseTypeName(fn.Result.Type)
			arraySuffix := w.getArraySuffix(fn.Result.Type)
			outName := "fragColor"
			if isVertexOutput {
				outName = "_vs_out"
			}
			w.writeLine("layout(location = %d) %s %s %s%s;", b.Location, qualifier, baseType, outName, arraySuffix)
		// BuiltinBinding: uses gl_Position/gl_FragDepth, no declaration needed
		default:
			// No output declaration needed for builtins
		}
		return
	}

	// No direct binding — check if result type is a struct with member bindings
	if int(fn.Result.Type) >= len(w.module.Types) {
		return
	}
	st, ok := w.module.Types[fn.Result.Type].Inner.(ir.StructType)
	if !ok {
		// Non-struct without binding — use default location 0
		if !isVertexOutput {
			baseType := w.getBaseTypeName(fn.Result.Type)
			arraySuffix := w.getArraySuffix(fn.Result.Type)
			w.writeLine("layout(location = 0) %s %s fragColor%s;", qualifier, baseType, arraySuffix)
		}
		return
	}

	// Struct result — flatten members into individual out declarations.
	// For vertex outputs, prefix names with "v_" to avoid collisions with
	// input variables that may share the same member names (e.g., both
	// VertexInput and VertexOutput contain "local", "color", etc.).
	info := &epStructInfo{
		structType: fn.Result.Type,
		members:    make([]epStructMemberInfo, len(st.Members)),
	}

	for memberIdx, member := range st.Members {
		if member.Binding == nil {
			name := escapeKeyword(member.Name)
			if isVertexOutput {
				name = "v_" + name
			}
			info.members[memberIdx] = epStructMemberInfo{
				glslName: name,
			}
			continue
		}
		switch b := (*member.Binding).(type) {
		case ir.LocationBinding:
			baseType := w.getBaseTypeName(member.Type)
			name := escapeKeyword(member.Name)
			if isVertexOutput {
				name = "v_" + name
			}
			w.writeLine("layout(location = %d) %s %s %s;", b.Location, qualifier, baseType, name)
			info.members[memberIdx] = epStructMemberInfo{
				glslName: name,
			}
		case ir.BuiltinBinding:
			builtinName := glslBuiltIn(b.Builtin, true)
			info.members[memberIdx] = epStructMemberInfo{
				isBuiltin:   true,
				builtinName: builtinName,
				glslName:    builtinName,
			}
		}
	}

	w.epStructOutput = info
}

// NOTE: writeFragmentIO is defined alongside writeVertexIO above.

// writeComputeLayout writes compute shader layout declaration.
func (w *Writer) writeComputeLayout(ep *ir.EntryPoint) {
	if !w.options.LangVersion.SupportsCompute() {
		return
	}

	x := ep.Workgroup[0]
	y := ep.Workgroup[1]
	z := ep.Workgroup[2]

	if x == 0 {
		x = 1
	}
	if y == 0 {
		y = 1
	}
	if z == 0 {
		z = 1
	}

	w.writeLine("layout(local_size_x = %d, local_size_y = %d, local_size_z = %d) in;", x, y, z)
	w.writeLine("")
}

// writeLocalVars writes local variable declarations, including initializers if present.
func (w *Writer) writeLocalVars(fn *ir.Function) error {
	for localIdx, local := range fn.LocalVars {
		localName := w.namer.call(local.Name)
		w.localNames[uint32(localIdx)] = localName //nolint:gosec // G115: localIdx is valid slice index
		baseType := w.getBaseTypeName(local.Type)
		arraySuffix := w.getArraySuffix(local.Type)

		if local.Init != nil {
			// Has an initializer — emit "type name[size] = init_expr;"
			initStr, err := w.writeExpression(*local.Init)
			if err != nil {
				return err
			}
			w.writeLine("%s %s%s = %s;", baseType, localName, arraySuffix, initStr)
		} else {
			w.writeLine("%s %s%s;", baseType, localName, arraySuffix)
		}
	}
	return nil
}

// Note: writeBlock is defined in statements.go and takes ir.Block directly

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

// getTypeName returns the GLSL type name for a type handle.
// For arrays, this returns the full type including size (e.g., "vec2[3]").
// Use getBaseTypeName + getArraySuffix for variable declarations.
func (w *Writer) getTypeName(handle ir.TypeHandle) string {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("type_%d", handle)
	}

	typ := w.module.Types[handle]
	return w.typeToGLSL(typ)
}

// getBaseTypeName returns the base GLSL type name, unwrapping arrays.
// For "array<vec2, 3>" returns "vec2". For non-arrays, same as getTypeName.
func (w *Writer) getBaseTypeName(handle ir.TypeHandle) string {
	if int(handle) >= len(w.module.Types) {
		return fmt.Sprintf("type_%d", handle)
	}
	typ := w.module.Types[handle]
	if arr, ok := typ.Inner.(ir.ArrayType); ok {
		return w.getBaseTypeName(arr.Base)
	}
	return w.typeToGLSL(typ)
}

// getArraySuffix returns the array size suffix(es) for a type handle.
// For "array<vec2, 3>" returns "[3]". For non-arrays, returns "".
// Handles nested arrays: "array<array<float, 4>, 3>" returns "[3][4]".
func (w *Writer) getArraySuffix(handle ir.TypeHandle) string {
	if int(handle) >= len(w.module.Types) {
		return ""
	}
	typ := w.module.Types[handle]
	arr, ok := typ.Inner.(ir.ArrayType)
	if !ok {
		return ""
	}
	if arr.Size.Constant != nil {
		return fmt.Sprintf("[%d]", *arr.Size.Constant) + w.getArraySuffix(arr.Base)
	}
	return "[]" + w.getArraySuffix(arr.Base)
}

// glslBuiltIn returns the GLSL built-in variable name for a builtin value.
func glslBuiltIn(builtin ir.BuiltinValue, isOutput bool) string {
	switch builtin {
	case ir.BuiltinPosition:
		if isOutput {
			return "gl_Position"
		}
		return "gl_FragCoord"
	case ir.BuiltinVertexIndex:
		return "uint(gl_VertexID)"
	case ir.BuiltinInstanceIndex:
		return "uint(gl_InstanceID)"
	case ir.BuiltinFrontFacing:
		return "gl_FrontFacing"
	case ir.BuiltinFragDepth:
		return "gl_FragDepth"
	case ir.BuiltinSampleIndex:
		return "gl_SampleID"
	case ir.BuiltinSampleMask:
		return "gl_SampleMaskIn[0]"
	case ir.BuiltinLocalInvocationID:
		return "gl_LocalInvocationID"
	case ir.BuiltinLocalInvocationIndex:
		return "gl_LocalInvocationIndex"
	case ir.BuiltinGlobalInvocationID:
		return "gl_GlobalInvocationID"
	case ir.BuiltinWorkGroupID:
		return "gl_WorkGroupID"
	case ir.BuiltinNumWorkGroups:
		return "gl_NumWorkGroups"
	default:
		return "gl_UNKNOWN"
	}
}

// formatFloat formats a float32 for GLSL output.
func formatFloat(f float32) string {
	s := fmt.Sprintf("%g", f)
	// Ensure it has a decimal point or exponent
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// formatFloat64 formats a float64 for GLSL output.
func formatFloat64(f float64) string {
	s := fmt.Sprintf("%g", f)
	// Ensure it has a decimal point or exponent
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s + "lf" // double literal suffix
}
