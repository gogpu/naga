// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package hlsl implements HLSL entry point I/O handling with proper
// input/output structs and semantics for vertex, fragment, and compute shaders.
//
//nolint:nestif
package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// HLSL semantic constants.
const (
	semanticSVPosition = "SV_Position"
	hlslVoidType       = "void"
)

// =============================================================================
// Entry Point Input/Output Structs
// =============================================================================

// structArgEntry tracks entry point arguments that are structs with member bindings.
// When a WGSL entry point takes a struct argument like `input: VertexInput` where the
// struct members have @location or @builtin bindings, the HLSL backend must flatten
// those members into the input struct and reconstruct the original struct in the body.
type structArgEntry struct {
	argIdx        int
	structType    ir.StructType
	argTypeHandle ir.TypeHandle
}

// writeEntryPointInputStruct writes the input struct for an entry point.
// HLSL entry points typically use input structs with semantics for vertex/fragment stages.
// It returns the struct name, whether an input struct was written, and a list of struct
// arguments whose members were flattened into the input struct.
//
//nolint:gocognit // Entry point input handling requires checking multiple argument forms
func (w *Writer) writeEntryPointInputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) (string, bool, []structArgEntry) {
	// Check if we need an input struct (location or builtin bindings, or struct args with member bindings)
	hasInputs := false
	var structArgs []structArgEntry

	for i, arg := range fn.Arguments {
		if arg.Binding != nil {
			hasInputs = true
			continue
		}
		// Check if this is a struct argument with member bindings
		if int(arg.Type) < len(w.module.Types) {
			typeInfo := &w.module.Types[arg.Type]
			if st, ok := typeInfo.Inner.(ir.StructType); ok {
				hasMemberBindings := false
				for _, member := range st.Members {
					if member.Binding != nil {
						hasMemberBindings = true
						break
					}
				}
				if hasMemberBindings {
					hasInputs = true
					structArgs = append(structArgs, structArgEntry{
						argIdx:        i,
						structType:    st,
						argTypeHandle: arg.Type,
					})
				}
			}
		}
	}

	if !hasInputs {
		return "", false, nil
	}

	structName := fmt.Sprintf("%s_Input", w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}])

	w.writeLine("struct %s {", structName)
	w.pushIndent()

	for i, arg := range fn.Arguments {
		if arg.Binding != nil {
			// Direct binding on argument — existing behavior
			argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)}]
			argType, arraySuffix := w.getTypeNameWithArraySuffix(arg.Type)

			semantic := w.getSemanticFromBinding(*arg.Binding, i)

			interpMod := ""
			if ep.Stage == ir.StageFragment {
				interpMod = w.getInterpolationModifier(*arg.Binding)
				if interpMod != "" {
					interpMod += " "
				}
			}

			w.writeLine("%s%s %s%s : %s;", interpMod, argType, argName, arraySuffix, semantic)
			continue
		}

		// Check if this is a struct arg with member bindings
		for _, sa := range structArgs {
			if sa.argIdx != i {
				continue
			}
			// Flatten struct members with bindings into the input struct
			for memberIdx, member := range sa.structType.Members {
				if member.Binding == nil {
					continue
				}
				memberName := Escape(member.Name)
				memberType, arraySuffix := w.getTypeNameWithArraySuffix(member.Type)
				semantic := w.getSemanticFromBinding(*member.Binding, memberIdx)

				interpMod := ""
				if ep.Stage == ir.StageFragment {
					interpMod = w.getInterpolationModifier(*member.Binding)
					if interpMod != "" {
						interpMod += " "
					}
				}

				w.writeLine("%s%s %s%s : %s;", interpMod, memberType, memberName, arraySuffix, semantic)
			}
			break
		}
	}

	w.popIndent()
	w.writeLine("};")
	w.writeLine("")

	return structName, true, structArgs
}

// writeEntryPointOutputStruct writes the output struct for an entry point.
// Returns the struct name and whether an output struct was written.
func (w *Writer) writeEntryPointOutputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) (string, bool) {
	if fn.Result == nil {
		return "", false
	}

	resultType := fn.Result.Type
	if int(resultType) >= len(w.module.Types) {
		return "", false
	}

	typeInfo := &w.module.Types[resultType]
	st, ok := typeInfo.Inner.(ir.StructType)
	if !ok {
		// Simple return type - will be handled in signature
		return "", false
	}

	structName := fmt.Sprintf("%s_Output", w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}])

	w.writeLine("struct %s {", structName)
	w.pushIndent()

	for memberIdx, member := range st.Members {
		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(resultType), handle2: uint32(memberIdx)}]
		memberType, arraySuffix := w.getTypeNameWithArraySuffix(member.Type)

		// Determine semantic based on stage and position
		var semantic string
		switch ep.Stage {
		case ir.StageVertex:
			if memberIdx == 0 {
				semantic = "SV_Position"
			} else {
				semantic = fmt.Sprintf("TEXCOORD%d", memberIdx-1)
			}
		case ir.StageFragment:
			semantic = fmt.Sprintf("SV_Target%d", memberIdx)
		default:
			semantic = fmt.Sprintf("TEXCOORD%d", memberIdx)
		}

		w.writeLine("%s %s%s : %s;", memberType, memberName, arraySuffix, semantic)
	}

	w.popIndent()
	w.writeLine("};")
	w.writeLine("")

	return structName, true
}

// =============================================================================
// Entry Point Signature Generation
// =============================================================================

// writeEntryPointWithIO writes an entry point with proper input/output handling.
// This is the enhanced version that generates HLSL-style entry points with semantics.
func (w *Writer) writeEntryPointWithIO(epIdx int, ep *ir.EntryPoint) error {
	if int(ep.Function) >= len(w.module.Functions) {
		return fmt.Errorf("invalid entry point function handle: %d", ep.Function)
	}

	fn := &w.module.Functions[ep.Function]
	w.currentFunction = fn
	w.currentFuncHandle = ep.Function
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)

	defer func() {
		w.currentFunction = nil
		w.localNames = nil
	}()

	// Write input/output structs if needed
	inputStructName, hasInputStruct, structArgs := w.writeEntryPointInputStruct(epIdx, ep, fn)
	outputStructName, hasOutputStruct := w.writeEntryPointOutputStruct(epIdx, ep, fn)

	epName := w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}]

	// Write compute shader attributes
	if ep.Stage == ir.StageCompute {
		w.writeComputeAttributes(ep)
	}

	// Determine return type
	returnType := "void"
	if hasOutputStruct {
		returnType = outputStructName
	} else if fn.Result != nil {
		returnType = w.getTypeName(fn.Result.Type)
		// Add semantic for simple return types
		if fn.Result.Binding != nil {
			returnType = w.getTypeName(fn.Result.Type)
		}
	}

	// Write function signature
	w.writeEntryPointSignature(returnType, epName, ep, fn, inputStructName, hasInputStruct)

	w.writeReturnSemantic(ep, fn, hasOutputStruct)

	w.writeLine(" {")
	w.pushIndent()

	// Extract inputs from struct if needed
	if hasInputStruct {
		w.writeInputExtraction(ep, fn, structArgs)
	}

	outputLocalMapped, err := w.writeEntryPointLocalVars(fn, hasOutputStruct, outputStructName, hasInputStruct)
	if err != nil {
		w.popIndent()
		return err
	}

	// Create output struct if not already mapped from a local variable
	if hasOutputStruct && !outputLocalMapped {
		w.writeLine("%s _output;", outputStructName)
		w.writeLine("")
	}

	// Write function body statements
	if err := w.writeBlock(fn.Body); err != nil {
		w.popIndent()
		return err
	}

	// Return output struct if needed (fallback for control flow paths without explicit return)
	if hasOutputStruct {
		w.writeLine("return _output;")
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	return nil
}

// writeComputeAttributes writes [numthreads(x,y,z)] attribute for compute shaders.
func (w *Writer) writeComputeAttributes(ep *ir.EntryPoint) {
	x, y, z := ep.Workgroup[0], ep.Workgroup[1], ep.Workgroup[2]
	if x == 0 {
		x = 1
	}
	if y == 0 {
		y = 1
	}
	if z == 0 {
		z = 1
	}
	w.writeLine("[numthreads(%d, %d, %d)]", x, y, z)
}

// writeReturnSemantic adds HLSL return semantic for simple (non-struct) return types.
// Fragment shader @location(N) maps to SV_TargetN (not TEXCOORD).
func (w *Writer) writeReturnSemantic(ep *ir.EntryPoint, fn *ir.Function, hasOutputStruct bool) {
	if hasOutputStruct || fn.Result == nil || fn.Result.Binding == nil {
		return
	}
	var semantic string
	if ep.Stage == ir.StageFragment {
		if loc, ok := (*fn.Result.Binding).(ir.LocationBinding); ok {
			semantic = fmt.Sprintf("SV_Target%d", loc.Location)
		} else {
			semantic = w.getSemanticFromBinding(*fn.Result.Binding, 0)
		}
	} else {
		semantic = w.getSemanticFromBinding(*fn.Result.Binding, 0)
	}
	fmt.Fprintf(&w.out, " : %s", semantic)
}

// writeEntryPointLocalVars writes local variable declarations for an entry point.
// When a local variable has the same type as the entry point result, it IS the
// output variable — declared as _output with the output struct type so that HLSL
// semantics (SV_Position, TEXCOORD) are attached correctly.
// Returns whether an output local was mapped to _output.
func (w *Writer) writeEntryPointLocalVars(fn *ir.Function, hasOutputStruct bool, outputStructName string, hasInputStruct bool) (bool, error) {
	outputLocalMapped := false
	for localIdx, local := range fn.LocalVars {
		localName := w.namer.call(local.Name)
		localType, arraySuffix := w.getTypeNameWithArraySuffix(local.Type)

		if hasOutputStruct && !outputLocalMapped && fn.Result != nil && local.Type == fn.Result.Type {
			localName = "_output"
			localType = outputStructName
			outputLocalMapped = true
		}

		w.localNames[uint32(localIdx)] = localName

		if local.Init != nil {
			w.writeIndent()
			fmt.Fprintf(&w.out, "%s %s%s = ", localType, localName, arraySuffix)
			if err := w.writeExpression(*local.Init); err != nil {
				return false, fmt.Errorf("entry point local var init: %w", err)
			}
			w.out.WriteString(";\n")
		} else {
			w.writeLine("%s %s%s;", localType, localName, arraySuffix)
		}
	}

	if len(fn.LocalVars) > 0 || hasInputStruct {
		w.writeLine("")
	}
	return outputLocalMapped, nil
}

// writeEntryPointSignature writes the function signature for an entry point.
func (w *Writer) writeEntryPointSignature(returnType, epName string, ep *ir.EntryPoint, fn *ir.Function, inputStructName string, hasInputStruct bool) {
	w.writeIndent()
	fmt.Fprintf(&w.out, "%s %s(", returnType, epName)

	firstParam := true

	// Stage input struct
	if hasInputStruct {
		fmt.Fprintf(&w.out, "%s _input", inputStructName)
		firstParam = false
	}

	// Built-in inputs not in struct (compute shader specifics)
	if ep.Stage == ir.StageCompute {
		for i, arg := range fn.Arguments {
			if arg.Binding == nil {
				continue
			}
			if builtin, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				semantic := BuiltInToSemantic(builtin.Builtin)
				if !firstParam {
					w.out.WriteString(", ")
				}
				argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)}]
				argType := w.getTypeName(arg.Type)
				fmt.Fprintf(&w.out, "%s %s : %s", argType, argName, semantic)
				firstParam = false
			}
		}
	}

	w.out.WriteString(")")
}

// writeInputExtraction writes code to extract input values from the input struct.
// For vertex/fragment stages, ALL bindings (location and builtin) go through
// the input struct. For compute shaders, builtins are passed as direct parameters
// and only location bindings are extracted from the struct.
// Struct arguments with member bindings are reconstructed from the flattened input.
func (w *Writer) writeInputExtraction(ep *ir.EntryPoint, fn *ir.Function, structArgs []structArgEntry) {
	for i, arg := range fn.Arguments {
		// Check if this is a struct arg that was flattened
		if sa := findStructArg(structArgs, i); sa != nil {
			argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)}]
			structTypeName := w.getTypeName(sa.argTypeHandle)

			// Declare the struct variable
			w.writeLine("%s %s;", structTypeName, argName)

			// Assign each member from the flattened input struct
			for _, member := range sa.structType.Members {
				if member.Binding == nil {
					continue
				}
				memberName := Escape(member.Name)
				w.writeLine("%s.%s = _input.%s;", argName, memberName, memberName)
			}
			continue
		}

		if arg.Binding == nil {
			continue
		}
		// In compute shaders, builtins are direct parameters (not in input struct)
		if ep.Stage == ir.StageCompute {
			if _, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				continue
			}
		}
		argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)}]
		w.writeLine("%s %s = _input.%s;", w.getTypeName(arg.Type), argName, argName)
	}
}

// findStructArg returns the structArgEntry for the given argument index, or nil if not found.
func findStructArg(structArgs []structArgEntry, argIdx int) *structArgEntry {
	for idx := range structArgs {
		if structArgs[idx].argIdx == argIdx {
			return &structArgs[idx]
		}
	}
	return nil
}

// =============================================================================
// Extended Helper Functions
// =============================================================================

// writeModHelper writes the safe modulo helper function.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeModHelper() {
	w.writeLine("// Safe modulo helper (truncated division semantics)")
	w.writeLine("int %s(int a, int b) {", NagaModFunction)
	w.pushIndent()
	w.writeLine("return a - b * (a / b);")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	// Overload for uint
	w.writeLine("uint %s(uint a, uint b) {", NagaModFunction)
	w.pushIndent()
	w.writeLine("return a - b * (a / b);")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeDivHelper writes the safe division helper function.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeDivHelper() {
	w.writeLine("// Safe division helper (handles zero divisor)")
	w.writeLine("int %s(int a, int b) {", NagaDivFunction)
	w.pushIndent()
	w.writeLine("return b != 0 ? a / b : 0;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	// Overload for uint
	w.writeLine("uint %s(uint a, uint b) {", NagaDivFunction)
	w.pushIndent()
	w.writeLine("return b != 0u ? a / b : 0u;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeAbsHelper writes the safe abs helper function.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeAbsHelper() {
	w.writeLine("// Safe abs helper (handles INT_MIN)")
	w.writeLine("int %s(int v) {", NagaAbsFunction)
	w.pushIndent()
	w.writeLine("return v >= 0 ? v : (v == -2147483648 ? 2147483647 : -v);")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeNegHelper writes the safe negation helper function.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeNegHelper() {
	w.writeLine("// Safe negation helper (handles INT_MIN)")
	w.writeLine("int %s(int v) {", NagaNegFunction)
	w.pushIndent()
	w.writeLine("return v == -2147483648 ? 2147483647 : -v;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeModfHelper writes the modf wrapper to return result struct like WGSL.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeModfHelper() {
	w.writeLine("// modf wrapper returning struct like WGSL")
	w.writeLine("struct _naga_modf_result_f32 {")
	w.pushIndent()
	w.writeLine("float fract;")
	w.writeLine("float whole;")
	w.popIndent()
	w.writeLine("};")
	w.writeLine("")

	w.writeLine("_naga_modf_result_f32 %s(float x) {", NagaModfFunction)
	w.pushIndent()
	w.writeLine("_naga_modf_result_f32 result;")
	w.writeLine("result.fract = modf(x, result.whole);")
	w.writeLine("return result;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeFrexpHelper writes the frexp wrapper to return result struct like WGSL.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeFrexpHelper() {
	w.writeLine("// frexp wrapper returning struct like WGSL")
	w.writeLine("struct _naga_frexp_result_f32 {")
	w.pushIndent()
	w.writeLine("float fract;")
	w.writeLine("int exp;")
	w.popIndent()
	w.writeLine("};")
	w.writeLine("")

	w.writeLine("_naga_frexp_result_f32 %s(float x) {", NagaFrexpFunction)
	w.pushIndent()
	w.writeLine("_naga_frexp_result_f32 result;")
	w.writeLine("result.fract = frexp(x, result.exp);")
	w.writeLine("return result;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeExtractBitsHelper writes the extractBits helper for SM < 6.0.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeExtractBitsHelper() {
	w.writeLine("// extractBits helper for older shader models")
	w.writeLine("uint %s(uint e, uint offset, uint count) {", NagaExtractBitsFunction)
	w.pushIndent()
	w.writeLine("uint mask = count == 32u ? 0xffffffffu : ((1u << count) - 1u);")
	w.writeLine("return (e >> offset) & mask;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	// Signed version
	w.writeLine("int %s(int e, uint offset, uint count) {", NagaExtractBitsFunction)
	w.pushIndent()
	w.writeLine("uint bits = %s(uint(e), offset, count);", NagaExtractBitsFunction)
	w.writeLine("uint signBit = (bits >> (count - 1u)) & 1u;")
	w.writeLine("if (signBit != 0u && count < 32u) {")
	w.pushIndent()
	w.writeLine("uint signExtend = ~((1u << count) - 1u);")
	w.writeLine("bits |= signExtend;")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("return int(bits);")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeInsertBitsHelper writes the insertBits helper for SM < 6.0.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeInsertBitsHelper() {
	w.writeLine("// insertBits helper for older shader models")
	w.writeLine("uint %s(uint e, uint newbits, uint offset, uint count) {", NagaInsertBitsFunction)
	w.pushIndent()
	w.writeLine("uint mask = count == 32u ? 0xffffffffu : ((1u << count) - 1u);")
	w.writeLine("return (e & ~(mask << offset)) | ((newbits & mask) << offset);")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	// Signed version
	w.writeLine("int %s(int e, int newbits, uint offset, uint count) {", NagaInsertBitsFunction)
	w.pushIndent()
	w.writeLine("return int(%s(uint(e), uint(newbits), offset, count));", NagaInsertBitsFunction)
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeF2I32Helper writes the float-to-i32 conversion helper with clamping.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeF2I32Helper() {
	w.writeLine("// Float to i32 conversion with clamping (handles NaN, inf)")
	w.writeLine("int %s(float v) {", NagaF2I32Function)
	w.pushIndent()
	w.writeLine("return int(clamp(v, -2147483648.0, 2147483647.0));")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// writeF2U32Helper writes the float-to-u32 conversion helper with clamping.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeF2U32Helper() {
	w.writeLine("// Float to u32 conversion with clamping (handles NaN, inf)")
	w.writeLine("uint %s(float v) {", NagaF2U32Function)
	w.pushIndent()
	w.writeLine("return uint(clamp(v, 0.0, 4294967295.0));")
	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
}

// =============================================================================
// Function Argument Helpers
// =============================================================================

// isEntryPointFunction checks if a function is an entry point.
func (w *Writer) isEntryPointFunction(handle ir.FunctionHandle) bool {
	for _, ep := range w.module.EntryPoints {
		if ep.Function == handle {
			return true
		}
	}
	return false
}

// getArgumentSemantic returns the HLSL semantic for a function argument binding.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) getArgumentSemantic(arg ir.FunctionArgument, argIdx int) string {
	if arg.Binding == nil {
		return ""
	}
	return w.getSemanticFromBinding(*arg.Binding, argIdx)
}

// writeArgumentWithSemantic writes a function argument with its semantic.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeArgumentWithSemantic(arg ir.FunctionArgument, argIdx int, argName string) string {
	argType := w.getTypeName(arg.Type)
	semantic := w.getArgumentSemantic(arg, argIdx)

	if semantic != "" {
		return fmt.Sprintf("%s %s : %s", argType, argName, semantic)
	}
	return fmt.Sprintf("%s %s", argType, argName)
}

// =============================================================================
// Result/Output Helpers
// =============================================================================

// getResultSemantic returns the HLSL semantic for a function result binding.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) getResultSemantic(result *ir.FunctionResult) string {
	if result == nil || result.Binding == nil {
		return ""
	}
	return w.getSemanticFromBinding(*result.Binding, 0)
}

// writeResultType writes the return type with semantic if applicable.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeResultType(result *ir.FunctionResult) string {
	if result == nil {
		return "void"
	}

	typeName := w.getTypeName(result.Type)
	semantic := w.getResultSemantic(result)

	if semantic != "" {
		// HLSL doesn't support return semantics in the type declaration,
		// they're specified via output structs or SV_Target for fragments
		return typeName
	}
	return typeName
}
