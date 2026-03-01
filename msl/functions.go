package msl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// MSL attribute constants
const (
	attrPosition  = "[[position]]"
	spaceConstant = "constant"
	spaceDevice   = "device"
)

// writeFunctions writes all non-entry-point function definitions.
func (w *Writer) writeFunctions() error {
	for handle := range w.module.Functions {
		fn := &w.module.Functions[handle]
		// Skip functions that are entry points (handled separately)
		if w.isEntryPointFunction(ir.FunctionHandle(handle)) {
			continue
		}

		if err := w.writeFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
	}
	return nil
}

// isEntryPointFunction checks if a function is an entry point.
func (w *Writer) isEntryPointFunction(handle ir.FunctionHandle) bool {
	for _, ep := range w.module.EntryPoints {
		if ep.Function == handle {
			return true
		}
	}
	return false
}

// writeFunction writes a regular function definition.
func (w *Writer) writeFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	// Set context
	w.currentFunction = fn
	w.currentFuncHandle = handle
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

	defer func() {
		w.currentFunction = nil
		w.localNames = nil
		w.namedExpressions = nil
		w.needBakeExpression = nil
	}()

	// Function name
	funcName := w.getName(nameKey{kind: nameKeyFunction, handle1: uint32(handle)})

	// Return type
	returnType := "void"
	if fn.Result != nil {
		returnType = w.writeTypeName(fn.Result.Type, StorageAccess(0))
	}

	// Function signature
	w.write("%s %s(", returnType, funcName)

	// Parameters
	for i, arg := range fn.Arguments {
		if i > 0 {
			w.write(", ")
		}
		argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(handle), handle2: uint32(i)})
		argType := w.writeTypeName(arg.Type, StorageAccess(0))
		w.write("%s %s", argType, argName)
	}

	w.write(") {\n")
	w.pushIndent()

	// Local variables
	for i, local := range fn.LocalVars {
		localName := escapeName(local.Name)
		if localName == "" {
			localName = fmt.Sprintf("local_%d", i)
		}
		w.localNames[uint32(i)] = localName

		localType := w.writeTypeName(local.Type, StorageAccess(0))
		w.writeIndent()
		w.write("%s %s", localType, localName)

		if local.Init != nil {
			w.write(" = ")
			if err := w.writeExpression(*local.Init); err != nil {
				return err
			}
		} else {
			w.write(" = %s()", localType)
		}
		w.write(";\n")
	}

	if len(fn.LocalVars) > 0 {
		w.writeLine("")
	}

	// Function body
	if err := w.writeBlock(fn.Body); err != nil {
		return err
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
	return nil
}

// writeEntryPoints writes all entry point functions.
func (w *Writer) writeEntryPoints() error {
	for epIdx, ep := range w.module.EntryPoints {
		// Check if we should skip this entry point
		if w.pipeline.EntryPoint != nil {
			if w.pipeline.EntryPoint.Name != ep.Name || w.pipeline.EntryPoint.Stage != ep.Stage {
				continue
			}
		}

		if err := w.writeEntryPoint(epIdx, &ep); err != nil {
			return err
		}
	}
	return nil
}

// writeEntryPoint writes a single entry point function.
//
//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // Entry point generation requires handling many input/output patterns
func (w *Writer) writeEntryPoint(epIdx int, ep *ir.EntryPoint) error {
	if int(ep.Function) >= len(w.module.Functions) {
		return fmt.Errorf("invalid entry point function handle: %d", ep.Function)
	}

	fn := &w.module.Functions[ep.Function]

	// Set context
	w.currentFunction = fn
	w.currentFuncHandle = ep.Function
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.entryPointOutputVar = ""
	w.entryPointOutputTypeActive = false
	w.entryPointInputStructArg = -1

	defer func() {
		w.currentFunction = nil
		w.localNames = nil
		w.namedExpressions = nil
		w.needBakeExpression = nil
		w.entryPointOutputVar = ""
		w.entryPointOutputTypeActive = false
		w.entryPointInputStructArg = -1
	}()

	// Write input/output structs if needed
	inputStructName, hasInputStruct := w.writeEntryPointInputStruct(epIdx, ep, fn)
	outputStructName, hasOutputStruct := w.writeEntryPointOutputStruct(epIdx, ep, fn)

	// Entry point name
	epName := w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)})

	// Stage keyword
	var stageKeyword string
	switch ep.Stage {
	case ir.StageVertex:
		stageKeyword = "vertex"
	case ir.StageFragment:
		stageKeyword = "fragment"
	case ir.StageCompute:
		stageKeyword = "kernel"
	}

	// Return type
	resolveReturnSignature := func() (string, string) {
		if hasOutputStruct {
			w.entryPointOutputVar = "_output"
			w.entryPointOutputType = fn.Result.Type
			w.entryPointOutputTypeActive = true
			return outputStructName, ""
		}
		if fn.Result == nil {
			return "void", ""
		}
		returnType := w.writeTypeName(fn.Result.Type, StorageAccess(0))
		if fn.Result.Binding == nil {
			return returnType, ""
		}
		if _, ok := (*fn.Result.Binding).(ir.BuiltinBinding); !ok {
			return returnType, ""
		}
		return returnType, w.writeBindingAttribute(*fn.Result.Binding)
	}
	returnType, returnAttr := resolveReturnSignature()

	// Function signature
	w.write("%s %s %s(", stageKeyword, returnType, epName)

	// Parameters
	firstParam := true

	// Stage input struct
	if hasInputStruct {
		w.write("%s _input [[stage_in]]", inputStructName)
		firstParam = false
	}

	// Built-in inputs not in struct
	for i, arg := range fn.Arguments {
		if arg.Binding != nil { //nolint:nestif // Binding type checks require nesting
			if builtin, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				attr := builtinInputAttribute(builtin.Builtin, ep.Stage)
				if attr != "" {
					if !firstParam {
						w.write(", ")
					}
					argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)})
					argType := w.writeTypeName(arg.Type, StorageAccess(0))
					w.write("%s %s %s", argType, argName, attr)
					firstParam = false
				}
			}
		}
	}

	// Resource bindings (textures, buffers, samplers)
	for i, global := range w.module.GlobalVariables {
		if global.Binding != nil {
			if !firstParam {
				w.write(",\n    ")
			}
			if err := w.writeGlobalResourceParam(uint32(i), &global); err != nil {
				return err
			}
			firstParam = false
		}
	}

	if returnAttr != "" {
		w.write(") %s {\n", returnAttr)
	} else {
		w.write(") {\n")
	}
	w.pushIndent()

	// Extract inputs from struct if needed
	emitInputAliases := func() {
		if !hasInputStruct {
			return
		}
		for i, arg := range fn.Arguments {
			if arg.Binding == nil {
				continue
			}
			if _, ok := (*arg.Binding).(ir.LocationBinding); !ok {
				continue
			}
			argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)})
			w.writeLine("auto %s = _input.%s;", argName, argName)
		}
		if w.entryPointInputStructArg < 0 {
			return
		}
		arg := fn.Arguments[w.entryPointInputStructArg]
		argName := arg.Name
		if argName == "" {
			argName = fmt.Sprintf("arg_%d", w.entryPointInputStructArg)
		}
		argName = escapeName(argName)
		w.writeLine("auto %s = _input;", argName)
	}
	emitInputAliases()

	// Local variables
	for i, local := range fn.LocalVars {
		localName := escapeName(local.Name)
		if localName == "" {
			localName = fmt.Sprintf("local_%d", i)
		}
		w.localNames[uint32(i)] = localName

		localType := w.writeTypeName(local.Type, StorageAccess(0))
		w.writeIndent()
		w.write("%s %s", localType, localName)

		if local.Init != nil {
			w.write(" = ")
			if err := w.writeExpression(*local.Init); err != nil {
				return err
			}
		} else {
			w.write(" = %s()", localType)
		}
		w.write(";\n")
	}

	if len(fn.LocalVars) > 0 || hasInputStruct {
		w.writeLine("")
	}

	// Create output struct if needed
	if hasOutputStruct {
		w.writeLine("%s _output;", outputStructName)
	}

	// Function body
	if err := w.writeBlock(fn.Body); err != nil {
		return err
	}

	// Return output struct if needed
	if hasOutputStruct {
		w.writeLine("return _output;")
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("")
	return nil
}

// writeEntryPointInputStruct writes the input struct for an entry point.
//
//nolint:gocognit,cyclop // Entry point struct generation requires handling many input/output patterns
func (w *Writer) writeEntryPointInputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) (string, bool) {
	// Check if we need an input struct (location bindings)
	hasLocationInputs := false
	for _, arg := range fn.Arguments {
		if arg.Binding == nil {
			continue
		}
		if _, ok := (*arg.Binding).(ir.LocationBinding); ok {
			hasLocationInputs = true
			break
		}
	}

	emitInputStruct := func(structName string, emitFields func()) {
		w.writeLine("struct %s {", structName)
		w.pushIndent()
		emitFields()
		w.popIndent()
		w.writeLine("};")
		w.writeLine("")
	}

	if hasLocationInputs {
		structName := fmt.Sprintf("%s_Input", w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}))

		emitInputStruct(structName, func() {
			for i, arg := range fn.Arguments {
				if arg.Binding == nil {
					continue
				}
				loc, ok := (*arg.Binding).(ir.LocationBinding)
				if !ok {
					continue
				}
				argName := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(ep.Function), handle2: uint32(i)})
				argType := w.writeTypeName(arg.Type, StorageAccess(0))

				attr := locationInputAttribute(loc, ep.Stage)
				w.writeLine("%s %s %s;", argType, argName, attr)
			}
		})

		return structName, true
	}
	for i, arg := range fn.Arguments {
		if arg.Binding != nil {
			continue
		}
		if int(arg.Type) >= len(w.module.Types) {
			continue
		}
		typeInfo := &w.module.Types[arg.Type]
		st, ok := typeInfo.Inner.(ir.StructType)
		if !ok {
			continue
		}

		structName := fmt.Sprintf("%s_Input", w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}))
		w.entryPointInputStructArg = i

		emitInputStruct(structName, func() {
			for memberIdx, member := range st.Members {
				memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(arg.Type), handle2: uint32(memberIdx)})
				memberType := w.writeTypeName(member.Type, StorageAccess(0))

				var attr string
				if member.Binding != nil {
					switch b := (*member.Binding).(type) {
					case ir.LocationBinding:
						attr = locationInputAttribute(b, ep.Stage)
					case ir.BuiltinBinding:
						attr = builtinInputAttribute(b.Builtin, ep.Stage)
					}
				}
				if attr == "" {
					// Fallback for struct members without explicit bindings
					switch {
					case ep.Stage == ir.StageFragment && memberIdx == 0:
						attr = attrPosition
					case ep.Stage == ir.StageFragment:
						attr = fmt.Sprintf("[[user(locn%d)]]", memberIdx-1)
					default:
						attr = fmt.Sprintf("[[attribute(%d)]]", memberIdx)
					}
				}

				w.writeLine("%s %s %s;", memberType, memberName, attr)
			}
		})

		return structName, true
	}
	return "", false
}

// writeEntryPointOutputStruct writes the output struct for an entry point.
func (w *Writer) writeEntryPointOutputStruct(epIdx int, ep *ir.EntryPoint, fn *ir.Function) (string, bool) {
	if fn.Result == nil {
		return "", false
	}

	// Check if result type is a struct with bindings
	resultType := fn.Result.Type
	if int(resultType) >= len(w.module.Types) {
		return "", false
	}

	typeInfo := &w.module.Types[resultType]
	st, ok := typeInfo.Inner.(ir.StructType)
	if !ok {
		// Simple return type - check if it has a builtin binding that requires output struct
		// In MSL, [[position]] must be on a struct member, not on function return type
		if fn.Result.Binding != nil {
			if _, isBuiltin := (*fn.Result.Binding).(ir.BuiltinBinding); isBuiltin {
				structName := fmt.Sprintf("%s_Output", w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}))
				returnType := w.writeTypeName(fn.Result.Type, StorageAccess(0))
				attr := w.writeBindingAttribute(*fn.Result.Binding)

				w.writeLine("struct %s {", structName)
				w.pushIndent()
				w.writeLine("%s member %s;", returnType, attr)
				w.popIndent()
				w.writeLine("};")
				w.writeLine("")

				return structName, true
			}
		}
		return "", false
	}

	structName := fmt.Sprintf("%s_Output", w.getName(nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}))

	w.writeLine("struct %s {", structName)
	w.pushIndent()

	for memberIdx, member := range st.Members {
		memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(resultType), handle2: uint32(memberIdx)})
		memberType := w.writeTypeName(member.Type, StorageAccess(0))

		// TODO: Get binding from struct member
		// For now, we'll use position-based attributes for vertex outputs
		var attr string
		switch ep.Stage {
		case ir.StageVertex:
			if memberIdx == 0 {
				attr = attrPosition
			} else {
				attr = fmt.Sprintf("[[user(locn%d)]]", memberIdx-1)
			}
		case ir.StageFragment:
			attr = fmt.Sprintf("[[color(%d)]]", memberIdx)
		}

		w.writeLine("%s %s %s;", memberType, memberName, attr)
	}

	w.popIndent()
	w.writeLine("};")
	w.writeLine("")

	return structName, true
}

// writeGlobalResourceParam writes a global resource as an entry point parameter.
func (w *Writer) writeGlobalResourceParam(handle uint32, global *ir.GlobalVariable) error {
	name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: handle})

	if int(global.Type) >= len(w.module.Types) {
		return fmt.Errorf("invalid type handle: %d", global.Type)
	}

	typeInfo := &w.module.Types[global.Type]

	// Determine binding slot
	var binding uint32
	if global.Binding != nil {
		binding = global.Binding.Binding
	}

	switch inner := typeInfo.Inner.(type) {
	case ir.SamplerType:
		w.write("%ssampler %s [[sampler(%d)]]", Namespace, name, binding)

	case ir.ImageType:
		typeName := w.imageTypeName(inner, StorageAccess(0))
		w.write("%s %s [[texture(%d)]]", typeName, name, binding)

	default:
		// Buffer types
		space := addressSpaceName(global.Space)
		typeName := w.writeTypeName(global.Type, StorageAccess(0))

		if space == spaceConstant || space == spaceDevice {
			w.write("%s %s* %s [[buffer(%d)]]", space, typeName, name, binding)
		} else {
			w.write("%s %s [[buffer(%d)]]", typeName, name, binding)
		}
	}

	return nil
}

// writeBindingAttribute writes the MSL attribute for a binding.
func (w *Writer) writeBindingAttribute(binding ir.Binding) string {
	switch b := binding.(type) {
	case ir.BuiltinBinding:
		return builtinOutputAttribute(b.Builtin)
	case ir.LocationBinding:
		return fmt.Sprintf("[[color(%d)]]", b.Location)
	}
	return ""
}

// builtinInputAttribute returns the MSL attribute for a built-in input.
func builtinInputAttribute(builtin ir.BuiltinValue, stage ir.ShaderStage) string {
	switch builtin {
	case ir.BuiltinPosition:
		if stage == ir.StageFragment {
			return attrPosition
		}
		return ""
	case ir.BuiltinVertexIndex:
		return "[[vertex_id]]"
	case ir.BuiltinInstanceIndex:
		return "[[instance_id]]"
	case ir.BuiltinFrontFacing:
		return "[[front_facing]]"
	case ir.BuiltinSampleIndex:
		return "[[sample_id]]"
	case ir.BuiltinSampleMask:
		return "[[sample_mask]]"
	case ir.BuiltinLocalInvocationID:
		return "[[thread_position_in_threadgroup]]"
	case ir.BuiltinLocalInvocationIndex:
		return "[[thread_index_in_threadgroup]]"
	case ir.BuiltinGlobalInvocationID:
		return "[[thread_position_in_grid]]"
	case ir.BuiltinWorkGroupID:
		return "[[threadgroup_position_in_grid]]"
	case ir.BuiltinNumWorkGroups:
		return "[[threadgroups_per_grid]]"
	}
	return ""
}

// builtinOutputAttribute returns the MSL attribute for a built-in output.
func builtinOutputAttribute(builtin ir.BuiltinValue) string {
	switch builtin {
	case ir.BuiltinPosition:
		return attrPosition
	case ir.BuiltinFragDepth:
		return "[[depth(any)]]"
	case ir.BuiltinSampleMask:
		return "[[sample_mask]]"
	}
	return ""
}

// locationInputAttribute returns the MSL attribute for a location input.
func locationInputAttribute(loc ir.LocationBinding, stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return fmt.Sprintf("[[attribute(%d)]]", loc.Location)
	case ir.StageFragment:
		// Fragment inputs use user() for custom varyings
		return fmt.Sprintf("[[user(locn%d)]]", loc.Location)
	}
	return ""
}
