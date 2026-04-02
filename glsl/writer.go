// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"
	"math"
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
	nameKeyEntryPointLocal
	nameKeyFunctionLocal
)

// maxBindingsPerGroup is the maximum number of bindings per bind group
// used when flattening (group, binding) to a single GL binding point.
// WebGPU allows up to 16 bindings per group across 4 groups (0..3).
// Flattened binding = group * maxBindingsPerGroup + binding.
const maxBindingsPerGroup = 16

// flattenBinding converts a WebGPU (group, binding) pair to a single GL binding
// index. GLSL has no concept of bind groups -- only flat binding indices.
// The formula is: group * maxBindingsPerGroup + binding + base.
func flattenBinding(group, binding, base uint32) uint32 {
	return group*maxBindingsPerGroup + binding + base
}

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

	// Block ID counter for unique interface block names (matches Rust naga's IdGenerator)
	blockIDCounter uint32

	// Varying counter for unique varying names (matches Rust naga's varying counter)
	varyingCounter int
	// Map from location key to varying name for EP IO setup
	varyingNameMap map[varyingLookupKey]string

	// Feature detection (matches Rust naga's FeaturesManager)
	features featuresManager

	// Continue forwarding for do-while switches inside loops.
	// In GLSL, only single-body switches (rendered as do-while) need this.
	continueCtx continueCtx

	// Cached block names for globals (computed once in registerNames, used in write*)
	globalBlockName    map[ir.GlobalVariableHandle]string // block name
	globalInstanceName map[ir.GlobalVariableHandle]string // instance variable name

	// Reachability set for dead code elimination.
	// When set, only reachable types, constants, globals, and functions
	// are emitted in the output. Built by collectReachable for the
	// selected entry point.
	reachable *reachableSet
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

// varyingLookupKey identifies a varying for name lookup.
type varyingLookupKey struct {
	location uint32
	blendSrc uint32
	isOutput bool
	stage    ir.ShaderStage
}

// namer generates unique identifiers, matching Rust naga's Namer.
// Uses per-name counters (not global) and adds '_' suffix when name ends with digit.
type namer struct {
	// unique maps base name → usage count (0 = first use, 1 = second, etc.)
	unique map[string]uint32
}

func newNamer() *namer {
	return &namer{
		unique: make(map[string]uint32),
	}
}

// call generates a unique name based on the given base.
// Matches Rust naga's Namer::call:
//   - First use of "foo" → "foo"
//   - Second use → "foo_1"
//   - Names ending in digit get trailing '_': "v3" → "v3_"
//   - Keywords get trailing '_': "main" → "main_"
func (n *namer) call(base string) string {
	escaped := sanitizeName(base)

	count, exists := n.unique[escaped]
	if exists {
		// Name already used — increment counter and suffix
		n.unique[escaped] = count + 1
		return fmt.Sprintf("%s_%d", escaped, count+1)
	}

	// First use — register it
	n.unique[escaped] = 0

	// Add '_' suffix if name ends with a digit or is a keyword
	result := escaped
	if len(result) > 0 && result[len(result)-1] >= '0' && result[len(result)-1] <= '9' {
		result += "_"
	} else if isKeyword(result) {
		result += "_"
	}

	return result
}

// sanitizeName cleans a name for use as a GLSL identifier.
// Matches Rust naga's Namer::sanitize:
//   - Drop leading digits
//   - Retain only ASCII alphanumeric and '_'
//   - Collapse consecutive '__' into single '_'
//   - Trim trailing '_'
func sanitizeName(name string) string {
	if name == "" {
		return "unnamed"
	}

	// Trim leading digits
	start := 0
	for start < len(name) && name[start] >= '0' && name[start] <= '9' {
		start++
	}
	name = name[start:]

	// Filter and collapse underscores — iterate RUNES (not bytes) for proper Unicode
	result := make([]byte, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (r >= '0' && r <= '9') {
			if r == '_' && len(result) > 0 && result[len(result)-1] == '_' {
				continue
			}
			result = append(result, byte(r))
		} else {
			switch r {
			case ':', '<', '>', ',', ' ':
				if len(result) == 0 || result[len(result)-1] != '_' {
					result = append(result, '_')
				}
			default:
				// Unicode codepoint escape: u{XXXX}_
				if len(result) > 0 && result[len(result)-1] != '_' {
					result = append(result, '_')
				}
				result = append(result, []byte(fmt.Sprintf("u%04x_", r))...)
			}
		}
	}

	// Trim trailing underscores
	for len(result) > 0 && result[len(result)-1] == '_' {
		result = result[:len(result)-1]
	}

	if len(result) == 0 {
		return "unnamed"
	}
	return string(result)
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
		globalBlockName:    make(map[ir.GlobalVariableHandle]string),
		globalInstanceName: make(map[ir.GlobalVariableHandle]string),
		varyingNameMap:     make(map[varyingLookupKey]string),
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
	// 0. Build reachability set for dead code elimination.
	// This determines which types, constants, globals, and functions are
	// actually used by the selected entry point, avoiding 150-320x output
	// bloat when modules contain many functions shared across entry points.
	w.buildReachableSet()

	// 1. Write version directive
	w.writeVersionDirective()

	// 1b. Collect required features and write extensions
	w.collectFeatures()
	w.features.writeExtensions(w)

	// 2. Write precision qualifiers (ES only)
	w.writePrecisionQualifiers()

	// 2b. Write compute layout (Rust naga emits this right after precision, before structs)
	w.writeComputeLayoutEarly()

	// 2b2. Write first instance uniform for vertex shaders using InstanceIndex
	w.writeFirstInstanceBinding()

	// 2c. Write early depth test layout (fragment shaders)
	w.writeEarlyDepthTest()

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

	// 4b. Write predeclared helper functions (naga_modf, naga_frexp)
	// Rust naga emits these right after type definitions, before constants.
	w.writePredeclaredHelpers()

	// 5. Write constants
	if err := w.writeConstants(); err != nil {
		return err
	}

	// 6. Write global variables (uniforms, inputs, outputs)
	if err := w.writeGlobalVariables(); err != nil {
		return err
	}

	// 7. Write entry point varying declarations (in/out at module level)
	// Rust naga writes these between globals and functions.
	w.writeVaryingDeclarations()

	// 7b. Write polyfill helper functions (mod, div) if needed
	w.writeHelperFunctions()

	// 7c. Separator between globals/varyings section and functions.
	// Rust naga always emits a blank line here (after write_varying, before functions).
	w.writeLine("")

	// 8. Write regular functions
	if err := w.writeFunctions(); err != nil {
		return err
	}

	// 9. Write entry points
	return w.writeEntryPoints()
}

// buildReachableSet constructs the reachability set for the selected entry point.
// If no specific entry point is selected, uses the first one.
func (w *Writer) buildReachableSet() {
	if len(w.module.EntryPoints) == 0 {
		return
	}

	// Find the target entry point
	for i := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[i]
		if w.options.EntryPoint == "" || ep.Name == w.options.EntryPoint {
			w.reachable = collectReachable(w.module, ep)
			return
		}
	}
}

// writeVersionDirective writes the #version directive.
func (w *Writer) writeVersionDirective() {
	w.writeLine("#version %s", w.options.LangVersion.String())
}

// getSelectedEntryPoint returns the entry point being compiled.
func (w *Writer) getSelectedEntryPoint() *ir.EntryPoint {
	for i := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[i]
		if w.options.EntryPoint == "" || ep.Name == w.options.EntryPoint {
			return ep
		}
	}
	return nil
}

// writePrecisionQualifiers writes precision qualifiers for ES.
// Matches Rust naga: blank line, then float and int, then blank line.
func (w *Writer) writePrecisionQualifiers() {
	if !w.options.LangVersion.ES {
		return
	}

	w.writeLine("")
	w.writeLine("precision highp float;")
	w.writeLine("precision highp int;")
	w.writeLine("")
}

// registerNames assigns unique names to all IR entities.
func (w *Writer) registerNames() error {
	// Register type names
	for handle, typ := range w.module.Types {
		var baseName string
		if typ.Name != "" {
			baseName = typ.Name
		} else {
			// Rust naga uses "type" as the default name for unnamed types
			baseName = "type"
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyType, handle1: uint32(handle)}] = name
		w.typeNames[ir.TypeHandle(handle)] = name

		// Register struct member names in a fresh namespace (per-struct).
		// Matches Rust naga: self.namespace(members.len(), |namer| { ... })
		if st, ok := typ.Inner.(ir.StructType); ok {
			memberNamer := newNamer()
			for memberIdx, member := range st.Members {
				memberName := member.Name
				if memberName == "" {
					memberName = "member"
				}
				w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}] = memberNamer.call(memberName)
			}
		}
	}

	// Register entry point names, arguments, and locals BEFORE constants/globals.
	// Matches Rust naga namer order: types → EP names+args+locals → functions → globals → constants.
	// Register ALL entry points (Rust namer is module-wide, not per-EP)
	for epIdx, ep := range w.module.EntryPoints {
		epName := w.namer.call(ep.Name)
		// The selected EP gets "main" as GLSL name
		if w.options.EntryPoint == "" || ep.Name == w.options.EntryPoint {
			w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}] = "main"
			w.entryPointNames[ep.Name] = "main"
		} else {
			w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}] = epName
			w.entryPointNames[ep.Name] = epName
		}

		epFuncHandle := uint32(len(w.module.Functions)) + uint32(epIdx)
		for argIdx, arg := range ep.Function.Arguments {
			argName := arg.Name
			if argName == "" {
				argName = fmt.Sprintf("arg_%d", argIdx)
			}
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: epFuncHandle, handle2: uint32(argIdx)}] = w.namer.call(argName)
		}

		// Register EP local variable names (reserve in global namer + store)
		for localIdx, local := range ep.Function.LocalVars {
			localName := local.Name
			if localName == "" {
				localName = "local"
			}
			w.names[nameKey{kind: nameKeyEntryPointLocal, handle1: uint32(epIdx), handle2: uint32(localIdx)}] = w.namer.call(localName)
		}
	}

	// Register function names, arguments, and locals
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
			argName := arg.Name
			if argName == "" {
				argName = fmt.Sprintf("arg_%d", argIdx)
			}
			w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}] = w.namer.call(argName)
		}

		for localIdx, local := range fn.LocalVars {
			localName := local.Name
			if localName == "" {
				localName = "local"
			}
			w.names[nameKey{kind: nameKeyFunctionLocal, handle1: uint32(handle), handle2: uint32(localIdx)}] = w.namer.call(localName)
		}
	}

	// Register constant names (AFTER EP locals and globals, matching Rust)
	for handle, constant := range w.module.Constants {
		var baseName string
		if constant.Name != "" {
			baseName = constant.Name
		} else {
			baseName = fmt.Sprintf("const_%d", handle)
		}
		name := w.namer.call(baseName)
		w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}] = name
	}

	// Register global variable names.
	// Rust naga's namer.reset calls namer.call_or(&var.name, "global") for EVERY global,
	// which reserves the name in the namer even if the GLSL name uses _group_G_binding_B_stage.
	// We must do the same to avoid collisions with local variables that shadow globals.
	for handle, global := range w.module.GlobalVariables {
		// Always call the namer to reserve the base name, matching Rust behavior.
		var baseName string
		if global.Name != "" {
			baseName = global.Name
		} else {
			baseName = fmt.Sprintf("global_%d", handle)
		}
		namerName := w.namer.call(baseName)

		var name string
		// Check if this global should get _group_G_binding_B_stage naming.
		isBlock := global.Binding != nil && (global.Space == ir.SpaceUniform || global.Space == ir.SpaceStorage)
		hasBindingName := global.Binding != nil

		if isBlock {
			stage := w.currentEntryPointStage()
			stageSuffix := "cs"
			switch stage {
			case ir.StageFragment:
				stageSuffix = "fs"
			case ir.StageVertex:
				stageSuffix = "vs"
			}
			instanceName := fmt.Sprintf("_group_%d_binding_%d_%s",
				global.Binding.Group, global.Binding.Binding, stageSuffix)
			w.globalInstanceName[ir.GlobalVariableHandle(handle)] = instanceName
			name = instanceName
		} else if global.Space == ir.SpaceImmediate {
			stage := w.currentEntryPointStage()
			stageSuffix := "cs"
			switch stage {
			case ir.StageFragment:
				stageSuffix = "fs"
			case ir.StageVertex:
				stageSuffix = "vs"
			}
			name = fmt.Sprintf("_immediates_binding_%s", stageSuffix)
		} else if hasBindingName {
			stage := w.currentEntryPointStage()
			stageSuffix := "cs"
			switch stage {
			case ir.StageFragment:
				stageSuffix = "fs"
			case ir.StageVertex:
				stageSuffix = "vs"
			}
			name = fmt.Sprintf("_group_%d_binding_%d_%s",
				global.Binding.Group, global.Binding.Binding, stageSuffix)
		} else {
			name = namerName
		}

		w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}] = name
	}

	return nil
}

// scanTextureSamplerPairs scans reachable functions and entry points for ExprImageSample
// expressions to discover which texture globals are paired with which sampler globals.
// GLSL has no separate texture/sampler types, so each pair must be emitted as a
// single "uniform sampler2D" declaration.
func (w *Writer) scanTextureSamplerPairs() {
	// Scan regular functions (skip unreachable ones).
	for i := range w.module.Functions {
		if w.reachable != nil && !w.reachable.hasFunction(ir.FunctionHandle(i)) {
			continue
		}
		fn := &w.module.Functions[i]
		w.scanFunctionForPairs(fn)
	}
	// Scan entry point functions (stored inline, not in Functions[]).
	for i := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[i]
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}
		w.scanFunctionForPairs(&ep.Function)
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
	// Only mark the sampler as combined (skipped in writeGlobalVariables).
	// The texture global stays visible so the combined declaration is emitted in-place.
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
// Rust naga writes ALL struct types regardless of entry point reachability.
// Only dynamically-sized structs are skipped (they become buffer blocks).
func (w *Writer) writeTypes() error {
	for handle, typ := range w.module.Types {
		st, ok := typ.Inner.(ir.StructType)
		if !ok {
			continue
		}

		// Skip structs ending with a runtime-sized array (dynamically sized).
		// These are only emitted as buffer blocks, not standalone structs.
		// Matches Rust naga: !is_dynamically_sized check.
		if len(st.Members) > 0 {
			lastMemberType := st.Members[len(st.Members)-1].Type
			if w.isDynamicallySized(lastMemberType) {
				continue
			}
		}

		typeName := w.typeNames[ir.TypeHandle(handle)]
		w.writeLine("struct %s {", typeName)
		w.pushIndent()

		for memberIdx, member := range st.Members {
			baseType := w.getBaseTypeName(member.Type)
			arraySuffix := w.getArraySuffix(member.Type)
			memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(handle), handle2: uint32(memberIdx)}]
			w.writeLine("%s %s%s;", baseType, memberName, arraySuffix)
		}

		w.popIndent()
		w.writeLine("};")
	}
	return nil
}

// writeConstants writes constant definitions.
// Rust naga writes ALL named constants (c.name.is_some()), regardless of
// whether they are used by the current entry point. No reachability filter.
func (w *Writer) writeConstants() error {
	wrote := false
	for handle, constant := range w.module.Constants {
		// Only write named constants (matches Rust: c.name.is_some())
		if constant.Name == "" {
			continue
		}

		name := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(handle)}]
		baseType := w.getBaseTypeName(constant.Type)
		arraySuffix := w.getArraySuffix(constant.Type)
		value := w.writeConstantValue(constant)
		w.writeLine("const %s %s%s = %s;", baseType, name, arraySuffix, value)
		wrote = true
	}
	if wrote {
		w.writeLine("")
	}
	return nil
}

// writeConstantValue returns the GLSL representation of a constant value.
func (w *Writer) writeConstantValue(constant ir.Constant) string {
	// First try the direct Value
	switch v := constant.Value.(type) {
	case ir.ScalarValue:
		return w.writeScalarValue(v, constant.Type)
	case ir.CompositeValue:
		return w.writeCompositeValue(v, constant.Type)
	case ir.ZeroConstantValue:
		zv := w.zeroInitValue(constant.Type)
		if zv != "" {
			return zv
		}
	}

	// If no direct value, try writing from GlobalExpressions init
	if int(constant.Init) < len(w.module.GlobalExpressions) {
		result := w.writeConstExpr(constant.Init)
		if result != "0" || constant.Value == nil {
			return result
		}
	}

	return "0" // Fallback
}

// writeConstExpr writes a constant expression from GlobalExpressions.
// This handles Compose, ZeroValue, Literal, and Constant references.
func (w *Writer) writeConstExpr(handle ir.ExpressionHandle) string {
	if int(handle) >= len(w.module.GlobalExpressions) {
		return "0"
	}
	expr := &w.module.GlobalExpressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal:
		return w.formatLiteral(k)
	case ir.ExprZeroValue:
		return w.zeroInitValue(k.Type)
	case ir.ExprConstant:
		if int(k.Constant) < len(w.module.Constants) {
			return w.writeConstantValue(w.module.Constants[k.Constant])
		}
		return "0"
	case ir.ExprCompose:
		typeName := w.getTypeName(k.Type)
		parts := make([]string, len(k.Components))
		for i, comp := range k.Components {
			parts[i] = w.writeConstExpr(comp)
		}
		return fmt.Sprintf("%s(%s)", typeName, strings.Join(parts, ", "))
	case ir.ExprSplat:
		value := w.writeConstExpr(k.Value)
		// Determine vector type from splat size + value scalar type
		prefix := ""
		if int(k.Value) < len(w.module.GlobalExpressions) {
			valExpr := &w.module.GlobalExpressions[k.Value]
			if lit, ok := valExpr.Kind.(ir.Literal); ok {
				switch lit.Value.(type) {
				case ir.LiteralI32:
					prefix = "i"
				case ir.LiteralU32:
					prefix = "u"
				case ir.LiteralBool:
					prefix = "b"
				}
			}
		}
		return fmt.Sprintf("%svec%d(%s)", prefix, k.Size, value)
	default:
		return "0"
	}
}

// formatLiteral formats an IR literal as a GLSL literal string.
func (w *Writer) formatLiteral(lit ir.Literal) string {
	switch v := lit.Value.(type) {
	case ir.LiteralBool:
		if bool(v) {
			return "true"
		}
		return "false"
	case ir.LiteralI32:
		return fmt.Sprintf("%d", int32(v))
	case ir.LiteralU32:
		return fmt.Sprintf("%du", uint32(v))
	case ir.LiteralF32:
		return formatFloat(float32(v))
	case ir.LiteralF64:
		return formatFloat64(float64(v)) // formatFloat64 already adds LF suffix
	case ir.LiteralI64:
		return fmt.Sprintf("%dl", int64(v))
	case ir.LiteralU64:
		return fmt.Sprintf("%dul", uint64(v))
	default:
		return "0"
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
	// Reset block ID counter — Rust generates IDs at write time, not registration
	w.blockIDCounter = 0

	// Build a map from texture handle to all combined sampler infos for that texture.
	// A depth texture may be paired with both a regular sampler and a comparison sampler,
	// producing two separate combined sampler entries for the same texture global.
	textureToCombined := make(map[ir.GlobalVariableHandle][]*combinedSamplerInfo)
	for _, info := range w.combinedSamplers {
		textureToCombined[info.textureHandle] = append(textureToCombined[info.textureHandle], info)
	}
	// Sort each texture's pairs so that the non-comparison sampler pair comes first.
	// The first pair gets the in-place declaration (texture-only name); additional pairs
	// are emitted as separate declarations with combined (texture__sampler) names.
	for _, infos := range textureToCombined {
		if len(infos) > 1 {
			sort.Slice(infos, func(i, j int) bool {
				iComp := w.isSamplerComparison(infos[i].samplerHandle)
				jComp := w.isSamplerComparison(infos[j].samplerHandle)
				if iComp != jComp {
					return !iComp // non-comparison first
				}
				return infos[i].samplerHandle < infos[j].samplerHandle
			})
		}
	}

	for handle, global := range w.module.GlobalVariables {
		// Skip unreachable globals (dead code elimination).
		if w.reachable != nil && !w.reachable.hasGlobal(ir.GlobalVariableHandle(handle)) {
			continue
		}

		// Skip sampler globals that have been absorbed into combined samplers.
		if w.globalIsCombined[ir.GlobalVariableHandle(handle)] {
			continue
		}

		name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}]
		typeName := w.getTypeName(global.Type)

		switch global.Space {
		case ir.SpaceUniform:
			w.writeUniformVariable(name, typeName, global)
		case ir.SpaceStorage:
			w.writeStorageVariable(name, typeName, global)
		case ir.SpacePrivate:
			baseType := w.getBaseTypeName(global.Type)
			arraySuffix := w.getArraySuffix(global.Type)
			// Add initializer from GlobalExpressions if available
			initStr := ""
			if global.InitExpr != nil {
				initStr = w.writeConstExpr(*global.InitExpr)
			}
			if initStr != "" && initStr != "0" {
				w.writeLine("%s %s%s = %s;", baseType, name, arraySuffix, initStr)
			} else if zv := w.zeroInitValue(global.Type); zv != "" {
				w.writeLine("%s %s%s = %s;", baseType, name, arraySuffix, zv)
			} else {
				w.writeLine("%s %s%s;", baseType, name, arraySuffix)
			}
		case ir.SpaceWorkGroup:
			baseType := w.getBaseTypeName(global.Type)
			arraySuffix := w.getArraySuffix(global.Type)
			w.writeLine("shared %s %s%s;", baseType, name, arraySuffix)
		case ir.SpacePushConstant:
			// Push constants emitted as uniform blocks
			w.writeUniformVariable(name, typeName, global)
		case ir.SpaceImmediate:
			// Immediate data (pipeline constants) — Rust uses special naming
			stage := w.currentEntryPointStage()
			stageSuffix := "cs"
			switch stage {
			case ir.StageFragment:
				stageSuffix = "fs"
			case ir.StageVertex:
				stageSuffix = "vs"
			}
			w.writeLine("uniform %s _immediates_binding_%s;", typeName, stageSuffix)
		default:
			// Handle texture/sampler globals — textures need "uniform", samplers are skipped
			if int(global.Type) < len(w.module.Types) {
				switch w.module.Types[global.Type].Inner.(type) {
				case ir.SamplerType:
					continue // GLSL has no standalone samplers
				case ir.ImageType:
					// Check if this texture has combined sampler pair(s) — if so, emit declarations.
					if infos, hasCombined := textureToCombined[ir.GlobalVariableHandle(handle)]; hasCombined {
						// First pair gets the in-place declaration (texture-only name).
						w.writeCombinedSamplerDecl(infos[0])
						// Additional pairs get separate declarations with combined names.
						for _, extra := range infos[1:] {
							w.writeLine("")
							w.writeExtraCombinedSamplerDecl(extra)
						}
					} else {
						w.writeImageGlobalDecl(global, name, typeName)
					}
				default:
					w.writeLine("%s %s;", typeName, name)
				}
			} else {
				w.writeLine("%s %s;", typeName, name)
			}
		}
		// Rust naga adds blank line after each global declaration
		w.writeLine("")
	}
	return nil
}

// writeImageGlobalDecl writes a standalone texture/image global declaration.
func (w *Writer) writeImageGlobalDecl(global ir.GlobalVariable, name, typeName string) {
	imgType := w.module.Types[global.Type].Inner.(ir.ImageType)
	highp := ""
	if w.options.LangVersion.ES {
		highp = "highp "
	}
	// Build layout qualifier parts
	var layoutParts []string
	if binding, ok := w.lookupBinding(global); ok {
		layoutParts = append(layoutParts, fmt.Sprintf("binding = %d", binding))
	}
	// Storage images need format layout and access qualifier
	if imgType.Class == ir.ImageClassStorage {
		layout := glslStorageFormat(imgType.StorageFormat)
		access := glslStorageAccess(imgType.StorageAccess)
		if layout != "" {
			layoutParts = append(layoutParts, layout)
		}
		if len(layoutParts) > 0 {
			w.writeLine("layout(%s) %suniform %s%s %s;", strings.Join(layoutParts, ", "), access, highp, typeName, name)
		} else {
			w.writeLine("%suniform %s%s %s;", access, highp, typeName, name)
		}
	} else {
		if len(layoutParts) > 0 {
			w.writeLine("layout(%s) uniform %s%s %s;", strings.Join(layoutParts, ", "), highp, typeName, name)
		} else {
			w.writeLine("uniform %s%s %s;", highp, typeName, name)
		}
	}
}

// writeCombinedSamplerDecl emits a single combined texture-sampler declaration.
func (w *Writer) writeCombinedSamplerDecl(info *combinedSamplerInfo) {
	// Use the texture global's registered name (which is _group_G_binding_B_stage for bound globals)
	varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(info.textureHandle)}]

	// Add highp qualifier for ES
	highp := ""
	if w.options.LangVersion.ES {
		highp = "highp "
	}

	// Look up the texture global's binding from the BindingMap
	layoutPrefix := ""
	if int(info.textureHandle) < len(w.module.GlobalVariables) {
		texGlobal := w.module.GlobalVariables[info.textureHandle]
		if binding, ok := w.lookupBinding(texGlobal); ok {
			layoutPrefix = fmt.Sprintf("layout(binding = %d) ", binding)
		}
	}
	w.writeLine("%suniform %s%s %s;", layoutPrefix, highp, info.glslTypeName, varName)

	// Update the combined sampler name so expression references use it
	info.glslName = varName

	// Track for TranslationInfo
	w.textureSamplerPairs = append(w.textureSamplerPairs, varName)
}

// writeExtraCombinedSamplerDecl emits an additional combined sampler declaration
// for a texture-sampler pair that is NOT the primary (in-place) pair for its texture.
// The combined name (texture__sampler) is kept as-is.
func (w *Writer) writeExtraCombinedSamplerDecl(info *combinedSamplerInfo) {
	highp := ""
	if w.options.LangVersion.ES {
		highp = "highp "
	}
	w.writeLine("uniform %s%s %s;", highp, info.glslTypeName, info.glslName)
	w.textureSamplerPairs = append(w.textureSamplerPairs, info.glslName)
}

// isSamplerComparison checks whether the global variable at the given handle
// is a comparison sampler (sampler_comparison in WGSL).
func (w *Writer) isSamplerComparison(handle ir.GlobalVariableHandle) bool {
	if int(handle) >= len(w.module.GlobalVariables) {
		return false
	}
	global := &w.module.GlobalVariables[handle]
	if int(global.Type) >= len(w.module.Types) {
		return false
	}
	if st, ok := w.module.Types[global.Type].Inner.(ir.SamplerType); ok {
		return st.Comparison
	}
	return false
}

// writeCombinedSamplerDeclarations emits "uniform sampler2D name;" declarations
// for each discovered texture-sampler pair.
// NOTE: This is only used as a fallback; normally combined samplers are emitted
// in-place by writeCombinedSamplerDecl during writeGlobalVariables.
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

		// Use the texture global's registered name (which is _group_G_binding_B_stage for bound globals)
		varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(info.textureHandle)}]

		// Add highp qualifier for ES
		highp := ""
		if w.options.LangVersion.ES {
			highp = "highp "
		}

		// Rust naga: layout(binding) only from binding_map
		// Look up the texture global's binding from the BindingMap
		layoutPrefix := ""
		if int(info.textureHandle) < len(w.module.GlobalVariables) {
			texGlobal := w.module.GlobalVariables[info.textureHandle]
			if binding, ok := w.lookupBinding(texGlobal); ok {
				layoutPrefix = fmt.Sprintf("layout(binding = %d) ", binding)
			}
		}
		w.writeLine("%suniform %s%s %s;", layoutPrefix, highp, info.glslTypeName, varName)
		w.writeLine("")

		// Update the combined sampler name so expression references use it
		info.glslName = varName
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

	// Non-struct uniform with binding — wrap in block (matches Rust naga)
	if global.Binding != nil {
		blockName, instanceName := w.getBlockNames(global)
		baseType := w.getBaseTypeName(global.Type)
		arraySuffix := w.getArraySuffix(global.Type)
		if binding, ok := w.lookupBinding(global); ok {
			w.writeLine("layout(std140, binding = %d) uniform %s { %s %s%s; };", binding, blockName, baseType, instanceName, arraySuffix)
		} else {
			w.writeLine("layout(std140) uniform %s { %s %s%s; };", blockName, baseType, instanceName, arraySuffix)
		}
	} else {
		w.writeLine("uniform %s %s;", typeName, name)
	}
}

// writeUniformBlock emits a GLSL uniform block (UBO) for a struct type.
// Matches Rust naga naming: TypeName_block_{binding}{Stage} { TypeName _group_G_binding_B_{stage}; }
func (w *Writer) writeUniformBlock(name, typeName string, global ir.GlobalVariable, st ir.StructType) {
	blockName, instanceName := w.getBlockNames(global)

	if global.Binding != nil {
		if binding, ok := w.lookupBinding(global); ok {
			w.writeLine("layout(std140, binding = %d) uniform %s { %s %s; };", binding, blockName, typeName, instanceName)
		} else {
			w.writeLine("layout(std140) uniform %s { %s %s; };", blockName, typeName, instanceName)
		}
	} else {
		w.writeLine("uniform %s {", blockName)
		w.pushIndent()
		for memberIdx, member := range st.Members {
			baseType := w.getBaseTypeName(member.Type)
			arraySuffix := w.getArraySuffix(member.Type)
			memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(global.Type), handle2: uint32(memberIdx)}]
			w.writeLine("%s %s%s;", baseType, memberName, arraySuffix)
		}
		w.popIndent()
		w.writeLine("} %s;", name)
	}
}

// globalBlockNames returns the block name and instance variable name for a
// uniform/storage global, matching Rust naga's naming convention:
//   - Block name: "{NamerTypeName}_block_{ID}{Stage}"
//   - Instance name: "_group_{group}_binding_{binding}_{stage_suffix}"
//
// Uses the namer-registered type name (not GLSL type name) for block naming.
// getBlockNames returns block name (generated at write time with incrementing ID)
// and instance name (pre-registered during registerNames).
func (w *Writer) getBlockNames(global ir.GlobalVariable) (string, string) {
	// Instance name from registration
	instanceName := ""
	for handle, g := range w.module.GlobalVariables {
		if g.Name == global.Name && g.Type == global.Type && g.Space == global.Space {
			instanceName = w.globalInstanceName[ir.GlobalVariableHandle(handle)]
			break
		}
	}
	if instanceName == "" {
		instanceName = "_unknown"
	}

	// Block name generated at WRITE time (Rust: IdGenerator during writing)
	typeName := w.typeNames[global.Type]
	typeName = strings.TrimRight(typeName, "_")
	stage := w.currentEntryPointStage()
	stageName := ""
	switch stage {
	case ir.StageCompute:
		stageName = "Compute"
	case ir.StageFragment:
		stageName = "Fragment"
	case ir.StageVertex:
		stageName = "Vertex"
	}
	blockID := w.blockIDCounter
	w.blockIDCounter++
	blockName := fmt.Sprintf("%s_block_%d%s", typeName, blockID, stageName)
	return blockName, instanceName
}

func (w *Writer) computeBlockNames(global ir.GlobalVariable) (string, string) {
	// Use the namer-registered type name, trimming trailing underscores
	typeName := w.typeNames[global.Type]
	typeName = strings.TrimRight(typeName, "_")
	stage := w.currentEntryPointStage()
	stageName := ""
	stageSuffix := ""
	switch stage {
	case ir.StageCompute:
		stageName = "Compute"
		stageSuffix = "cs"
	case ir.StageFragment:
		stageName = "Fragment"
		stageSuffix = "fs"
	case ir.StageVertex:
		stageName = "Vertex"
		stageSuffix = "vs"
	}

	group := uint32(0)
	binding := uint32(0)
	if global.Binding != nil {
		group = global.Binding.Group
		binding = global.Binding.Binding
	}

	blockID := w.blockIDCounter
	w.blockIDCounter++
	blockName := fmt.Sprintf("%s_block_%d%s", typeName, blockID, stageName)
	instanceName := fmt.Sprintf("_group_%d_binding_%d_%s", group, binding, stageSuffix)
	return blockName, instanceName
}

// currentEntryPointStage returns the shader stage of the selected entry point.
func (w *Writer) currentEntryPointStage() ir.ShaderStage {
	for i := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[i]
		if w.options.EntryPoint == "" || ep.Name == w.options.EntryPoint {
			return ep.Stage
		}
	}
	return ir.StageCompute // fallback
}

// lookupBinding returns the flat binding index for a global variable from the BindingMap.
// Returns (binding, true) if found, or (0, false) if not mapped.
// Matches Rust naga's binding_map.get(&resource_binding).
func (w *Writer) lookupBinding(global ir.GlobalVariable) (uint8, bool) {
	if global.Binding == nil || w.options.BindingMap == nil {
		return 0, false
	}
	if !w.options.LangVersion.supportsExplicitLocations() {
		return 0, false
	}
	key := BindingMapKey{Group: global.Binding.Group, Binding: global.Binding.Binding}
	binding, ok := w.options.BindingMap[key]
	return binding, ok
}

// writeStorageVariable writes a storage buffer declaration.
// Matches Rust naga: layout(std430) [readonly] buffer TypeName_block_NStage { ... } _group_G_binding_B_stage;
// storageLayoutPrefix returns "layout(std430, binding = N) " or "layout(std430) " depending on binding map.
func (w *Writer) storageLayoutPrefix(global ir.GlobalVariable) string {
	if binding, ok := w.lookupBinding(global); ok {
		return fmt.Sprintf("layout(std430, binding = %d) ", binding)
	}
	return "layout(std430) "
}

func (w *Writer) writeStorageVariable(name, typeName string, global ir.GlobalVariable) {
	if !w.options.LangVersion.SupportsStorageBuffers() {
		w.writeUniformVariable(name, typeName, global)
		return
	}

	blockName, instanceName := w.getBlockNames(global)
	layoutPrefix := w.storageLayoutPrefix(global)

	// Determine readonly from access mode
	readOnly := ""
	if global.Access == ir.StorageRead {
		readOnly = "readonly "
	}

	// Check if this is a struct type
	if int(global.Type) < len(w.module.Types) {
		if st, ok := w.module.Types[global.Type].Inner.(ir.StructType); ok {
			// Only expand dynamically-sized structs (last member is runtime array).
			// Non-dynamic structs: { StructType instance; }
			isDynamic := len(st.Members) > 0 && w.isDynamicallySized(st.Members[len(st.Members)-1].Type)
			if isDynamic {
				w.writeLine("%s%sbuffer %s {", layoutPrefix, readOnly, blockName)
				w.pushIndent()
				for memberIdx, member := range st.Members {
					baseType := w.getBaseTypeName(member.Type)
					arraySuffix := w.getArraySuffix(member.Type)
					memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(global.Type), handle2: uint32(memberIdx)}]
					w.writeLine("%s %s%s;", baseType, memberName, arraySuffix)
				}
				w.popIndent()
				w.writeLine("} %s;", instanceName)
			} else {
				w.writeLine("%s%sbuffer %s { %s %s; };", layoutPrefix, readOnly, blockName, typeName, instanceName)
			}
			return
		}
	}

	// Non-struct storage buffer — use C-style array declarations
	baseType := w.getBaseTypeName(global.Type)
	arraySuffix := w.getArraySuffix(global.Type)
	w.writeLine("%s%sbuffer %s { %s %s%s; };", layoutPrefix, readOnly, blockName, baseType, instanceName, arraySuffix)
}

// writeVaryingDeclarations writes entry point in/out declarations at module level.
// Matches Rust naga's write_varying which emits layout(location=N) in/out before functions.
func (w *Writer) writeVaryingDeclarations() {
	ep := w.getSelectedEntryPoint()
	if ep == nil {
		return
	}
	fn := &ep.Function

	// Write input varyings (from arguments)
	for _, arg := range fn.Arguments {
		w.writeVarying(arg.Binding, arg.Type, ep.Stage, false)
	}

	// Write output varyings (from result)
	if fn.Result != nil {
		w.writeVarying(fn.Result.Binding, fn.Result.Type, ep.Stage, true)
	}
}

// writeVarying writes a single varying (in/out) declaration.
// For struct types, expands into individual member declarations.
func (w *Writer) writeVarying(binding *ir.Binding, typeHandle ir.TypeHandle, stage ir.ShaderStage, isOutput bool) {
	if binding != nil {
		// Direct binding (scalar/vector)
		w.writeSingleVarying(binding, typeHandle, stage, isOutput)
		return
	}

	// Check if it's a struct — expand members
	if int(typeHandle) < len(w.module.Types) {
		if st, ok := w.module.Types[typeHandle].Inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if member.Binding != nil {
					w.writeSingleVarying(member.Binding, member.Type, stage, isOutput)
				}
			}
			return
		}
	}
}

// writeSingleVarying writes a single layout(location=N) in/out declaration.
// For builtins with invariant qualifier, writes "invariant gl_Position;" etc.
func (w *Writer) writeSingleVarying(binding *ir.Binding, typeHandle ir.TypeHandle, stage ir.ShaderStage, isOutput bool) {
	// Handle builtins — check for invariant and clip distance
	if b, ok := (*binding).(ir.BuiltinBinding); ok {
		if b.Invariant && isOutput {
			builtinName := glslBuiltIn(b.Builtin, isOutput)
			w.writeLine("invariant %s;", builtinName)
		}
		// ClipDistance: emit "out float gl_ClipDistance[N];" declaration.
		// Matches Rust naga: re-declare gl_ClipDistance with the array size from the type.
		if b.Builtin == ir.BuiltinClipDistance && isOutput {
			if int(typeHandle) < len(w.module.Types) {
				if arr, ok := w.module.Types[typeHandle].Inner.(ir.ArrayType); ok {
					if arr.Size.Constant != nil {
						w.writeLine("out float gl_ClipDistance[%d];", *arr.Size.Constant)
					}
				}
			}
		}
		return
	}

	loc, ok := (*binding).(ir.LocationBinding)
	if !ok {
		return
	}

	direction := "in"
	if isOutput {
		direction = "out"
	}

	typeName := w.getTypeName(typeHandle)
	// Use location for naming; for dual-source add blend_src offset
	nameIdx := int(loc.Location)
	if loc.BlendSrc != nil && *loc.BlendSrc > 0 {
		nameIdx = int(loc.Location) + int(*loc.BlendSrc)
	}
	varName := w.varyingName(nameIdx, stage, isOutput)

	// Cache for setupEntryPointIO lookup
	varyingKey := varyingLookupKey{location: loc.Location, isOutput: isOutput, stage: stage}
	if loc.BlendSrc != nil {
		varyingKey.blendSrc = *loc.BlendSrc
	}
	w.varyingNameMap[varyingKey] = varName

	// Rust naga: interpolation only for vertex output and fragment input
	emitInterp := (stage == ir.StageVertex && isOutput) || (stage == ir.StageFragment && !isOutput)

	interpQual := ""
	if emitInterp {
		if loc.Interpolation != nil {
			switch loc.Interpolation.Kind {
			case ir.InterpolationFlat:
				interpQual = "flat "
			case ir.InterpolationLinear:
				interpQual = "noperspective "
			case ir.InterpolationPerspective:
				interpQual = "smooth "
			}
			// Add sampling qualifier
			switch loc.Interpolation.Sampling {
			case ir.SamplingCentroid:
				interpQual += "centroid "
			case ir.SamplingSample:
				interpQual += "sample "
			}
		} else {
			interpQual = "smooth "
		}
	}

	// Rust naga: layout(location=N) when (supports_explicit_locations OR !emitInterp) AND supports_io_locations
	canWriteLayout := w.options.LangVersion.supportsExplicitLocations() || !emitInterp
	writeLayout := canWriteLayout && w.options.LangVersion.supportsIOLocations()

	if loc.BlendSrc != nil {
		w.writeLine("layout(location = %d, index = %d) %s%s %s;", loc.Location, *loc.BlendSrc, interpQual, direction, typeName+" "+varName)
	} else if writeLayout {
		w.writeLine("layout(location = %d) %s%s %s;", loc.Location, interpQual, direction, typeName+" "+varName)
	} else {
		w.writeLine("%s%s %s;", interpQual, direction, typeName+" "+varName)
	}
}

// lookupVaryingName finds the varying name from the cached map, falling back to generation.
func (w *Writer) lookupVaryingName(location uint32, stage ir.ShaderStage, isOutput bool) string {
	key := varyingLookupKey{location: location, isOutput: isOutput, stage: stage}
	if name, ok := w.varyingNameMap[key]; ok {
		return name
	}
	return w.varyingName(int(location), stage, isOutput)
}

// lookupVaryingNameWithBlend finds varying name with blend_src awareness.
func (w *Writer) lookupVaryingNameWithBlend(location uint32, blendSrc *uint32, stage ir.ShaderStage, isOutput bool) string {
	key := varyingLookupKey{location: location, isOutput: isOutput, stage: stage}
	if blendSrc != nil {
		key.blendSrc = *blendSrc
	}
	if name, ok := w.varyingNameMap[key]; ok {
		return name
	}
	idx := int(location)
	if blendSrc != nil {
		idx += int(*blendSrc)
	}
	return w.varyingName(idx, stage, isOutput)
}

// varyingName generates the Rust naga naming convention for varying variables.
func (w *Writer) varyingName(location int, stage ir.ShaderStage, isOutput bool) string {
	switch stage {
	case ir.StageVertex:
		if isOutput {
			return fmt.Sprintf("_vs2fs_location%d", location)
		}
		return fmt.Sprintf("_p2vs_location%d", location)
	case ir.StageFragment:
		if isOutput {
			return fmt.Sprintf("_fs2p_location%d", location)
		}
		return fmt.Sprintf("_vs2fs_location%d", location)
	default:
		return fmt.Sprintf("_location%d", location)
	}
}

// writePredeclaredHelpers writes naga_modf/naga_frexp helper functions.
// Detects predeclared result types by name pattern and generates the corresponding
// GLSL wrapper functions. Matches Rust naga's predeclared_types iteration.
func (w *Writer) writePredeclaredHelpers() {
	for handle, typ := range w.module.Types {
		name := typ.Name
		if name == "" {
			continue
		}

		// Detect modf result types: __modf_result_f32, __modf_result_vec2_f32, etc.
		// Detect frexp result types: __frexp_result_f32, __frexp_result_vec4_f32, etc.
		isModf := strings.HasPrefix(name, "__modf_result_")
		isFrexp := strings.HasPrefix(name, "__frexp_result_")
		if !isModf && !isFrexp {
			continue
		}

		structName := w.typeNames[ir.TypeHandle(handle)]

		// Parse the type suffix to determine argument type
		suffix := ""
		if isModf {
			suffix = strings.TrimPrefix(name, "__modf_result_")
		} else {
			suffix = strings.TrimPrefix(name, "__frexp_result_")
		}

		// Map suffix to GLSL type
		argType := ""
		otherType := "" // for frexp, the int type
		switch suffix {
		case "f32":
			argType = "float"
			otherType = "int"
		case "f64":
			argType = "double"
			otherType = "int"
		case "vec2_f32":
			argType = "vec2"
			otherType = "ivec2"
		case "vec3_f32":
			argType = "vec3"
			otherType = "ivec3"
		case "vec4_f32":
			argType = "vec4"
			otherType = "ivec4"
		case "vec2_f64":
			argType = "dvec2"
			otherType = "ivec2"
		case "vec3_f64":
			argType = "dvec3"
			otherType = "ivec3"
		case "vec4_f64":
			argType = "dvec4"
			otherType = "ivec4"
		default:
			continue
		}

		if isModf {
			w.writeLine("")
			w.writeLine("%s naga_modf(%s arg) {", structName, argType)
			w.pushIndent()
			w.writeLine("%s other;", argType)
			w.writeLine("%s fract = modf(arg, other);", argType)
			w.writeLine("return %s(fract, other);", structName)
			w.popIndent()
			w.writeLine("}")
		} else {
			w.writeLine("")
			w.writeLine("%s naga_frexp(%s arg) {", structName, argType)
			w.pushIndent()
			w.writeLine("%s other;", otherType)
			w.writeLine("%s fract = frexp(arg, other);", argType)
			w.writeLine("return %s(fract, other);", structName)
			w.popIndent()
			w.writeLine("}")
		}
	}
}

// scanNeedBakeExpressions scans function expressions for those that need to be
// baked into temporary variables. Matches Rust naga's need_bake_expressions scanning.
//
// Rust naga uses bake_ref_count(): Access/AccessIndex -> MAX (never bake),
// ImageSample/ImageLoad/Derivative/Load -> 1 (always bake, handled in shouldBakeExpression),
// everything else -> 2 (bake when used 2+ times).
// We count expression references and mark multi-use expressions for baking.
func (w *Writer) scanNeedBakeExpressions(fn *ir.Function) {
	// Phase 1: Count references to each expression handle
	refCount := make(map[ir.ExpressionHandle]int)
	countRef := func(h ir.ExpressionHandle) {
		refCount[h]++
	}
	countRefOpt := func(h *ir.ExpressionHandle) {
		if h != nil {
			refCount[*h]++
		}
	}

	// Count refs from expressions
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprAccess:
			countRef(e.Base)
			countRef(e.Index)
		case ir.ExprAccessIndex:
			countRef(e.Base)
		case ir.ExprSplat:
			countRef(e.Value)
		case ir.ExprSwizzle:
			countRef(e.Vector)
		case ir.ExprCompose:
			for _, c := range e.Components {
				countRef(c)
			}
		case ir.ExprLoad:
			countRef(e.Pointer)
		case ir.ExprUnary:
			countRef(e.Expr)
		case ir.ExprBinary:
			countRef(e.Left)
			countRef(e.Right)
		case ir.ExprSelect:
			countRef(e.Condition)
			countRef(e.Accept)
			countRef(e.Reject)
		case ir.ExprRelational:
			countRef(e.Argument)
		case ir.ExprMath:
			countRef(e.Arg)
			countRefOpt(e.Arg1)
			countRefOpt(e.Arg2)
		case ir.ExprAs:
			countRef(e.Expr)
		case ir.ExprImageSample:
			countRef(e.Image)
			countRef(e.Sampler)
			countRef(e.Coordinate)
			countRefOpt(e.ArrayIndex)
			// Count refs from SampleLevel variants
			switch lv := e.Level.(type) {
			case ir.SampleLevelExact:
				countRef(lv.Level)
			case ir.SampleLevelBias:
				countRef(lv.Bias)
			case ir.SampleLevelGradient:
				countRef(lv.X)
				countRef(lv.Y)
			}
			countRefOpt(e.DepthRef)
			countRefOpt(e.Offset)
		case ir.ExprImageLoad:
			countRef(e.Image)
			countRef(e.Coordinate)
			countRefOpt(e.ArrayIndex)
			countRefOpt(e.Sample)
			countRefOpt(e.Level)
		case ir.ExprImageQuery:
			countRef(e.Image)
		case ir.ExprDerivative:
			countRef(e.Expr)
		case ir.ExprArrayLength:
			countRef(e.Array)
		}
	}

	// Count refs from statements
	var countStmtRefs func(stmts []ir.Statement)
	countStmtRefs = func(stmts []ir.Statement) {
		for _, stmt := range stmts {
			switch s := stmt.Kind.(type) {
			case ir.StmtEmit:
				// Emit doesn't reference expressions directly
			case ir.StmtBlock:
				countStmtRefs(s.Block)
			case ir.StmtIf:
				countRef(s.Condition)
				countStmtRefs(s.Accept)
				countStmtRefs(s.Reject)
			case ir.StmtSwitch:
				countRef(s.Selector)
				for _, c := range s.Cases {
					countStmtRefs(c.Body)
				}
			case ir.StmtLoop:
				countStmtRefs(s.Body)
				countStmtRefs(s.Continuing)
				if s.BreakIf != nil {
					countRef(*s.BreakIf)
				}
			case ir.StmtReturn:
				countRefOpt(s.Value)
			case ir.StmtStore:
				countRef(s.Pointer)
				countRef(s.Value)
			case ir.StmtImageStore:
				countRef(s.Image)
				countRef(s.Coordinate)
				countRefOpt(s.ArrayIndex)
				countRef(s.Value)
			case ir.StmtCall:
				for _, arg := range s.Arguments {
					countRef(arg)
				}
			case ir.StmtAtomic:
				countRef(s.Pointer)
				countRef(s.Value)
			}
		}
	}
	countStmtRefs(fn.Body)

	// Phase 2: Mark expressions with refcount >= bake_ref_count for baking
	for i, expr := range fn.Expressions {
		handle := ir.ExpressionHandle(i)
		rc := refCount[handle]

		// Determine bake_ref_count (matches Rust)
		switch expr.Kind.(type) {
		case ir.ExprAccess, ir.ExprAccessIndex:
			// Never bake (threshold = MAX)
			continue
		case ir.ExprImageSample, ir.ExprImageLoad, ir.ExprDerivative, ir.ExprLoad:
			// Always bake (threshold = 1) -- handled in shouldBakeExpression already
			continue
		default:
			// Bake when used 2+ times
			if rc >= 2 {
				w.needBakeExpression[handle] = struct{}{}
			}
		}
	}

	// Phase 3: Math-specific forced baking (matches Rust's explicit inserts)
	for _, expr := range fn.Expressions {
		switch m := expr.Kind.(type) {
		case ir.ExprMath:
			switch m.Fun {
			case ir.MathDot4I8Packed, ir.MathDot4U8Packed:
				w.needBakeExpression[m.Arg] = struct{}{}
				if m.Arg1 != nil {
					w.needBakeExpression[*m.Arg1] = struct{}{}
				}
			case ir.MathQuantizeF16:
				w.needBakeExpression[m.Arg] = struct{}{}
			case ir.MathExtractBits:
				if m.Arg1 != nil {
					w.needBakeExpression[*m.Arg1] = struct{}{}
				}
			case ir.MathInsertBits:
				if m.Arg2 != nil {
					w.needBakeExpression[*m.Arg2] = struct{}{}
				}
			}
		}
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
// Since entry point functions are stored inline in EntryPoints[] (not in Functions[]),
// all functions in Functions[] are regular functions.
func (w *Writer) writeFunctions() error {
	for handle := range w.module.Functions {
		// Skip unreachable functions (dead code elimination).
		if w.reachable != nil && !w.reachable.hasFunction(ir.FunctionHandle(handle)) {
			continue
		}
		fn := &w.module.Functions[handle]
		if err := w.writeFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
		// Rust naga adds blank line after each function
		w.writeLine("")
	}
	return nil
}

// writeFunction writes a single function definition.
func (w *Writer) writeFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	w.currentFunction = fn
	w.currentFuncHandle = handle
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.scanNeedBakeExpressions(fn)

	name := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(handle)}]

	// Return type
	var returnType string
	if fn.Result != nil {
		returnType = w.getTypeName(fn.Result.Type)
	} else {
		returnType = "void"
	}

	// Arguments — use C-style array syntax: "type name[N]" not "type[N] name"
	// Pointer parameters become "inout type name" in GLSL.
	// Sampler parameters are filtered out — GLSL uses combined texture-sampler types.
	// Matches Rust naga: filter(|arg| !matches!(arg.ty.inner, TypeInner::Sampler { .. }))
	args := make([]string, 0, len(fn.Arguments))
	for argIdx, arg := range fn.Arguments {
		// Skip sampler arguments — GLSL combines them with texture args
		if int(arg.Type) < len(w.module.Types) {
			if _, isSampler := w.module.Types[arg.Type].Inner.(ir.SamplerType); isSampler {
				continue
			}
		}

		argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(argIdx)}]

		// Check if parameter is a pointer type — emit as "inout"
		qualifier := ""
		argType := arg.Type
		if int(argType) < len(w.module.Types) {
			if ptr, ok := w.module.Types[argType].Inner.(ir.PointerType); ok {
				qualifier = "inout "
				argType = ptr.Base // Unwrap pointer to get the base type
			}
		}

		// ES requires highp precision qualifier for sampler/image function parameters.
		// Matches Rust naga: write_type adds precision for Image types on ES.
		precision := ""
		if w.options.LangVersion.ES && int(argType) < len(w.module.Types) {
			if _, isImage := w.module.Types[argType].Inner.(ir.ImageType); isImage {
				precision = "highp "
			}
		}

		baseType := w.getBaseTypeName(argType)
		arraySuffix := w.getArraySuffix(argType)
		args = append(args, fmt.Sprintf("%s%s%s %s%s", qualifier, precision, baseType, argName, arraySuffix))
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
	// Entry point function is stored inline (not in Functions[])
	fn := &ep.Function
	w.currentFunction = fn
	// Use offset handle matching registerNames() so argument name lookups work
	w.currentFuncHandle = ir.FunctionHandle(len(w.module.Functions) + epIdx)
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.scanNeedBakeExpressions(fn)
	w.inEntryPoint = true
	w.entryPointResult = fn.Result
	w.epStructArgs = make(map[uint32]*epStructInfo)
	w.epStructOutput = nil

	// Set up struct IO mapping for entry point return values.
	// Varying declarations are at module level, but we need the name mapping
	// for return statement expansion (writeStructReturn).
	w.setupEntryPointIO(ep)

	// Main function
	w.writeLine("void main() {")
	w.pushIndent()

	// Workgroup variable zero initialization (compute shaders only).
	// Rust naga: if zero_initialize_workgroup_memory && compute stage
	if ep.Stage == ir.StageCompute {
		w.writeWorkgroupVarInit()
	}

	// Write entry point argument locals (before local vars).
	w.writeEntryPointArgLocals(ep)

	if err := w.writeLocalVars(fn); err != nil {
		return err
	}

	// Write function body
	if err := w.writeBlock(ir.Block(fn.Body)); err != nil {
		return err
	}

	// Note: coordinate space adjustment and point size for vertex shaders
	// are now emitted inside writeDirectReturn/writeStructReturn, matching Rust naga.

	w.popIndent()
	w.writeLine("}")
	// Rust naga adds blank line after entry point (end of file newline)
	w.writeLine("")

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
			w.writeStructArgIO(uint32(argIdx), arg.Type, "in", false)
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
			w.writeStructArgIO(uint32(argIdx), arg.Type, "in", false)
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

// writeWorkgroupVarInit emits zero-initialization guard for workgroup variables.
// Matches Rust naga: if (gl_LocalInvocationID == uvec3(0u)) { var = zero; } barrier();
func (w *Writer) writeWorkgroupVarInit() {
	// Collect workgroup globals that need initialization
	var workgroupVars []struct {
		handle ir.GlobalVariableHandle
		global ir.GlobalVariable
	}
	for handle, global := range w.module.GlobalVariables {
		if global.Space != ir.SpaceWorkGroup {
			continue
		}
		if w.reachable != nil && !w.reachable.hasGlobal(ir.GlobalVariableHandle(handle)) {
			continue
		}
		workgroupVars = append(workgroupVars, struct {
			handle ir.GlobalVariableHandle
			global ir.GlobalVariable
		}{ir.GlobalVariableHandle(handle), global})
	}

	if len(workgroupVars) == 0 {
		return
	}

	w.writeLine("if (gl_LocalInvocationID == uvec3(0u)) {")
	w.pushIndent()
	for _, wv := range workgroupVars {
		name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(wv.handle)}]
		zeroVal := w.zeroInitValue(wv.global.Type)
		if zeroVal == "" {
			zeroVal = "0"
		}
		w.writeLine("%s = %s;", name, zeroVal)
	}
	w.popIndent()
	w.writeLine("}")
	w.writeLine("memoryBarrierShared();")
	w.writeLine("barrier();")
}

// writeEntryPointArgLocals writes local variable declarations for entry point arguments.
// In GLSL, main() takes no arguments. Entry point parameters become locals initialized
// from builtins (gl_GlobalInvocationID, etc.) or varying inputs (_p2vs_location0, etc.).
func (w *Writer) writeEntryPointArgLocals(ep *ir.EntryPoint) {
	fn := &ep.Function
	for argIdx, arg := range fn.Arguments {
		// Find the FunctionArgument expression handle for this argument
		var exprHandle ir.ExpressionHandle = ir.ExpressionHandle(0)
		found := false
		for h, expr := range fn.Expressions {
			if fa, ok := expr.Kind.(ir.ExprFunctionArgument); ok && fa.Index == uint32(argIdx) {
				exprHandle = ir.ExpressionHandle(h)
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// Get the argument name from NamedExpressions or fall back to arg.Name
		name := ""
		if fn.NamedExpressions != nil {
			name = fn.NamedExpressions[exprHandle]
		}
		if name == "" {
			name = arg.Name
		}
		if name == "" {
			continue
		}

		// Determine the GLSL value to initialize from
		var initValue string
		if arg.Binding != nil {
			switch b := (*arg.Binding).(type) {
			case ir.BuiltinBinding:
				initValue = glslBuiltIn(b.Builtin, false)
				// ViewIndex: WebGL uses gl_ViewID_OVR, non-WebGL uses uint(gl_ViewIndex)
				if b.Builtin == ir.BuiltinViewIndex && !w.options.LangVersion.isWebGL() {
					initValue = "uint(gl_ViewIndex)"
				}
			case ir.LocationBinding:
				initValue = w.lookupVaryingName(b.Location, ep.Stage, false)
			}
		} else {
			// Struct argument — reconstruct from varying inputs
			// e.g., FragmentInput val = FragmentInput(gl_FragCoord, _vs2fs_location0, ...);
			info, hasInfo := w.epStructArgs[uint32(argIdx)]
			if !hasInfo {
				continue
			}
			structName := w.getTypeName(arg.Type)
			components := make([]string, len(info.members))
			for i, m := range info.members {
				if m.isBuiltin {
					components[i] = m.builtinName
				} else {
					components[i] = m.glslName
				}
			}
			varName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(w.currentFuncHandle), handle2: uint32(argIdx)}]
			w.writeLine("%s %s = %s(%s);", structName, varName, structName, strings.Join(components, ", "))
			w.namedExpressions[exprHandle] = varName
			continue
		}

		if initValue == "" {
			continue
		}

		// Write: "type name = initValue;"
		// Use the already-registered argument name from registerNames
		typeName := w.getTypeName(arg.Type)
		varName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(w.currentFuncHandle), handle2: uint32(argIdx)}]
		w.writeLine("%s %s = %s;", typeName, varName, initValue)

		// Register in namedExpressions so subsequent references use this name
		w.namedExpressions[exprHandle] = varName
	}
}

// setupEntryPointIO populates epStructOutput and epStructArgs with varying name
// mappings so that return statements and argument references work correctly.
// This does NOT write any declarations — those are at module level.
func (w *Writer) setupEntryPointIO(ep *ir.EntryPoint) {
	fn := &ep.Function

	// Set up struct argument mapping (for input varyings)
	for argIdx, arg := range fn.Arguments {
		if arg.Binding != nil {
			continue // Direct binding, not struct
		}
		if int(arg.Type) >= len(w.module.Types) {
			continue
		}
		st, ok := w.module.Types[arg.Type].Inner.(ir.StructType)
		if !ok {
			continue
		}
		info := &epStructInfo{
			structType: arg.Type,
			members:    make([]epStructMemberInfo, len(st.Members)),
		}
		for memberIdx, member := range st.Members {
			if member.Binding == nil {
				continue
			}
			switch b := (*member.Binding).(type) {
			case ir.LocationBinding:
				info.members[memberIdx] = epStructMemberInfo{
					glslName: w.lookupVaryingName(b.Location, ep.Stage, false),
				}
			case ir.BuiltinBinding:
				builtinName := glslBuiltIn(b.Builtin, false)
				info.members[memberIdx] = epStructMemberInfo{
					isBuiltin:   true,
					builtinName: builtinName,
					glslName:    builtinName,
				}
			}
		}
		w.epStructArgs[uint32(argIdx)] = info
	}

	// Set up struct output mapping (for return → varying assignment)
	if fn.Result == nil || fn.Result.Binding != nil {
		return // Direct binding or no result — handled by writeDirectReturn
	}
	if int(fn.Result.Type) >= len(w.module.Types) {
		return
	}
	st, ok := w.module.Types[fn.Result.Type].Inner.(ir.StructType)
	if !ok {
		return
	}
	info := &epStructInfo{
		structType: fn.Result.Type,
		members:    make([]epStructMemberInfo, len(st.Members)),
	}
	for memberIdx, member := range st.Members {
		if member.Binding == nil {
			continue
		}
		switch b := (*member.Binding).(type) {
		case ir.LocationBinding:
			info.members[memberIdx] = epStructMemberInfo{
				glslName: w.lookupVaryingNameWithBlend(b.Location, b.BlendSrc, ep.Stage, true),
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

// writeEarlyDepthTest writes early depth test layout if the entry point has it enabled.
func (w *Writer) writeEarlyDepthTest() {
	for i := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[i]
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}
		if ep.Stage == ir.StageFragment && ep.EarlyDepthTest != nil {
			switch ep.EarlyDepthTest.Conservative {
			case ir.ConservativeDepthUnchanged:
				w.writeLine("layout(early_fragment_tests) in;")
			case ir.ConservativeDepthGreaterEqual:
				w.writeLine("layout (depth_greater) out float gl_FragDepth;")
			case ir.ConservativeDepthLessEqual:
				w.writeLine("layout (depth_less) out float gl_FragDepth;")
			}
		}
		break
	}
}

// writeComputeLayoutEarly writes the compute layout declaration early in the output,
// matching Rust naga which emits it right after precision qualifiers.
func (w *Writer) writeComputeLayoutEarly() {
	// Find the target entry point
	for i := range w.module.EntryPoints {
		ep := &w.module.EntryPoints[i]
		if w.options.EntryPoint != "" && ep.Name != w.options.EntryPoint {
			continue
		}
		if ep.Stage == ir.StageCompute {
			w.writeComputeLayout(ep)
		}
		break
	}
}

// writeComputeLayout writes compute shader layout declaration.
// Note: no version check here — if we're compiling a compute shader,
// the extension (GL_ARB_compute_shader) has already been emitted if needed.
func (w *Writer) writeComputeLayout(ep *ir.EntryPoint) {
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

// writeFirstInstanceBinding writes "uniform uint naga_vs_first_instance;" for vertex shaders
// that use InstanceIndex built-in, when DRAW_PARAMETERS is not set.
// Matches Rust naga behavior.
func (w *Writer) writeFirstInstanceBinding() {
	ep := w.getSelectedEntryPoint()
	if ep == nil || ep.Stage != ir.StageVertex {
		return
	}
	if !w.features.contains(FeatureInstanceIndex) {
		return
	}
	// TODO: check for WriterFlagDrawParameters when added
	w.writeLine("uniform uint naga_vs_first_instance;")
	w.writeLine("")
}

// writeLocalVars writes local variable declarations, including initializers if present.
// Matches Rust naga: locals without explicit init get zero-initialized if the type supports it.
func (w *Writer) writeLocalVars(fn *ir.Function) error {
	for localIdx, local := range fn.LocalVars {
		// Use pre-registered name from registerNames (avoids double namer call)
		var localName string
		if w.inEntryPoint {
			// Find EP index
			for epIdx, ep := range w.module.EntryPoints {
				if &ep.Function == fn || (w.options.EntryPoint != "" && ep.Name == w.options.EntryPoint) {
					localName = w.names[nameKey{kind: nameKeyEntryPointLocal, handle1: uint32(epIdx), handle2: uint32(localIdx)}]
					break
				}
			}
		} else {
			localName = w.names[nameKey{kind: nameKeyFunctionLocal, handle1: uint32(w.currentFuncHandle), handle2: uint32(localIdx)}]
		}
		if localName == "" {
			// Fallback — shouldn't happen but safe
			localName = w.namer.call(local.Name)
		}
		w.localNames[uint32(localIdx)] = localName
		baseType := w.getBaseTypeName(local.Type)
		arraySuffix := w.getArraySuffix(local.Type)

		if local.Init != nil {
			// Has an initializer — emit "type name[size] = init_expr;"
			initStr, err := w.writeExpression(*local.Init)
			if err != nil {
				return err
			}
			w.writeLine("%s %s%s = %s;", baseType, localName, arraySuffix, initStr)
		} else if zeroInit := w.zeroInitValue(local.Type); zeroInit != "" {
			// No explicit init but type supports zero initialization.
			// Rust naga always zero-initializes supported types.
			w.writeLine("%s %s%s = %s;", baseType, localName, arraySuffix, zeroInit)
		} else {
			w.writeLine("%s %s%s;", baseType, localName, arraySuffix)
		}
	}
	return nil
}

// zeroInitValue returns the GLSL zero-init expression for a type, or "" if not supported.
// Matches Rust naga's is_value_init_supported + write_zero_init_value.
func (w *Writer) zeroInitValue(typeHandle ir.TypeHandle) string {
	if int(typeHandle) >= len(w.module.Types) {
		return ""
	}
	typ := &w.module.Types[typeHandle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return scalarZeroInit(inner.Kind)
	case ir.VectorType:
		return fmt.Sprintf("%s(%s)", w.getTypeName(typeHandle), scalarZeroInit(inner.Scalar.Kind))
	case ir.MatrixType:
		return fmt.Sprintf("%s(0.0)", w.getTypeName(typeHandle))
	case ir.ArrayType:
		if inner.Size.Constant == nil {
			return "" // Dynamic-sized array
		}
		elemZero := w.zeroInitValue(inner.Base)
		if elemZero == "" {
			return ""
		}
		typeName := w.getTypeName(typeHandle)
		count := *inner.Size.Constant
		if count == 0 {
			return ""
		}
		parts := make([]string, count)
		for i := range parts {
			parts[i] = elemZero
		}
		return fmt.Sprintf("%s(%s)", typeName, strings.Join(parts, ", "))
	case ir.StructType:
		structName := w.getTypeName(typeHandle)
		parts := make([]string, len(inner.Members))
		for i, m := range inner.Members {
			z := w.zeroInitValue(m.Type)
			if z == "" {
				return ""
			}
			parts[i] = z
		}
		return fmt.Sprintf("%s(%s)", structName, strings.Join(parts, ", "))
	case ir.AtomicType:
		return scalarZeroInit(inner.Scalar.Kind)
	default:
		return ""
	}
}

// scalarZeroInit returns the GLSL zero initialization literal for a scalar kind.
func scalarZeroInit(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarBool:
		return "false"
	case ir.ScalarSint:
		return "0"
	case ir.ScalarUint:
		return "0u"
	case ir.ScalarFloat:
		return "0.0"
	default:
		return "0"
	}
}

// isDynamicallySized returns true if a type is or contains a dynamic (runtime-sized) array.
func (w *Writer) isDynamicallySized(typeHandle ir.TypeHandle) bool {
	if int(typeHandle) >= len(w.module.Types) {
		return false
	}
	typ := &w.module.Types[typeHandle]
	switch inner := typ.Inner.(type) {
	case ir.ArrayType:
		return inner.Size.Constant == nil // nil size = dynamic
	case ir.StructType:
		if len(inner.Members) > 0 {
			return w.isDynamicallySized(inner.Members[len(inner.Members)-1].Type)
		}
		return false
	default:
		return false
	}
}

// resolveTypeInner attempts to extract a TypeInner from a TypeResolution.
// If the resolution is empty (both Handle and Value are nil), it falls back
// to dynamic type resolution using ir.ResolveExpressionType. This handles
// cases where the lowerer didn't populate ExpressionTypes for certain expressions
// (e.g., Splat, Compose) but the type can be inferred from the expression kind.
func (w *Writer) resolveTypeInner(res *ir.TypeResolution, handle ir.ExpressionHandle) ir.TypeInner {
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		return w.module.Types[*res.Handle].Inner
	}
	if res.Value != nil {
		return res.Value
	}
	// Fallback: resolve dynamically
	if w.module != nil && w.currentFunction != nil {
		resolved, err := ir.ResolveExpressionType(w.module, w.currentFunction, handle)
		if err == nil {
			if resolved.Handle != nil && int(*resolved.Handle) < len(w.module.Types) {
				return w.module.Types[*resolved.Handle].Inner
			}
			return resolved.Value
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
		// Matches Rust naga: (uint(gl_InstanceID) + naga_vs_first_instance)
		return "(uint(gl_InstanceID) + naga_vs_first_instance)"
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
	case ir.BuiltinViewIndex:
		return "gl_ViewID_OVR" // overridden per-version in writeEntryPointArgLocals
	case ir.BuiltinBarycentric:
		return "gl_BaryCoordEXT"
	case ir.BuiltinPointSize:
		return "gl_PointSize"
	case ir.BuiltinPrimitiveIndex:
		return "gl_PrimitiveID"
	case ir.BuiltinNumSubgroups:
		return "gl_NumSubgroups"
	case ir.BuiltinSubgroupID:
		return "gl_SubgroupID"
	case ir.BuiltinSubgroupSize:
		return "gl_SubgroupSize"
	case ir.BuiltinSubgroupInvocationID:
		return "gl_SubgroupInvocationID"
	case ir.BuiltinClipDistance:
		return "gl_ClipDistance"
	default:
		return "gl_UNKNOWN"
	}
}

// glslStorageFormat returns the GLSL format qualifier for a storage texture format.
func glslStorageFormat(format ir.StorageFormat) string {
	switch format {
	case ir.StorageFormatRgba8Unorm:
		return "rgba8"
	case ir.StorageFormatRgba8Snorm:
		return "rgba8_snorm"
	case ir.StorageFormatRgba8Uint:
		return "rgba8ui"
	case ir.StorageFormatRgba8Sint:
		return "rgba8i"
	case ir.StorageFormatRgba16Uint:
		return "rgba16ui"
	case ir.StorageFormatRgba16Sint:
		return "rgba16i"
	case ir.StorageFormatRgba16Float:
		return "rgba16f"
	case ir.StorageFormatRgba32Uint:
		return "rgba32ui"
	case ir.StorageFormatRgba32Sint:
		return "rgba32i"
	case ir.StorageFormatRgba32Float:
		return "rgba32f"
	case ir.StorageFormatR32Uint:
		return "r32ui"
	case ir.StorageFormatR32Sint:
		return "r32i"
	case ir.StorageFormatR32Float:
		return "r32f"
	default:
		return ""
	}
}

// glslStorageAccess returns the GLSL access qualifier for storage textures.
func glslStorageAccess(access ir.StorageAccess) string {
	switch access {
	case ir.StorageAccessRead:
		return "readonly "
	case ir.StorageAccessWrite:
		return "writeonly "
	case ir.StorageAccessReadWrite:
		return "" // both read+write = no qualifier
	default:
		return ""
	}
}

// formatFloat formats a float32 for GLSL output.
// Matches Rust Debug format: no '+' in exponent (3.4028235e38 not 3.4028235e+38).
func formatFloat(f float32) string {
	s := fmt.Sprintf("%g", f)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	s = strings.Replace(s, "e+", "e", 1)
	return s
}

// formatFloat64 formats a float64 for GLSL output.
func formatFloat64(f float64) string {
	s := fmt.Sprintf("%g", f)
	// Ensure it has a decimal point or exponent
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s + "LF" // double literal suffix (uppercase, matching Rust naga)
}
