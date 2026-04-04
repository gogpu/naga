// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package hlsl implements HLSL entry point I/O handling with proper
// input/output structs and semantics for vertex, fragment, and compute shaders.
package hlsl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gogpu/naga/ir"
)

// HLSL semantic constants.
const (
	semanticSVPosition = "SV_Position"
	hlslVoidType       = "void"
)

// =============================================================================
// Interface Key for sorting EP struct members (matches Rust InterfaceKey)
// =============================================================================

// interfaceKeyKind orders members: locations first, then builtins, then other.
type interfaceKeyKind int

const (
	interfaceKeyLocation interfaceKeyKind = iota
	interfaceKeyBuiltIn
	interfaceKeyOther
)

// interfaceKey is used to sort entry point struct members.
// Location members come first (sorted by location), then builtins.
type interfaceKey struct {
	kind     interfaceKeyKind
	location uint32
	builtin  ir.BuiltinValue
}

func newInterfaceKey(binding *ir.Binding) interfaceKey {
	if binding == nil {
		return interfaceKey{kind: interfaceKeyOther}
	}
	switch b := (*binding).(type) {
	case ir.LocationBinding:
		return interfaceKey{kind: interfaceKeyLocation, location: b.Location}
	case ir.BuiltinBinding:
		return interfaceKey{kind: interfaceKeyBuiltIn, builtin: b.Builtin}
	default:
		return interfaceKey{kind: interfaceKeyOther}
	}
}

func interfaceKeyLess(a, b interfaceKey) bool {
	if a.kind != b.kind {
		return a.kind < b.kind
	}
	if a.kind == interfaceKeyLocation {
		return a.location < b.location
	}
	// For builtins, use numeric value for deterministic ordering
	return a.builtin < b.builtin
}

// isSubgroupBuiltinBinding returns true if the binding is a subgroup-related builtin.
// These builtins need special handling in HLSL (computed from wave intrinsics).
// Matches Rust naga is_subgroup_builtin_binding.
func isSubgroupBuiltinBinding(binding *ir.Binding) bool {
	if binding == nil {
		return false
	}
	bb, ok := (*binding).(ir.BuiltinBinding)
	if !ok {
		return false
	}
	switch bb.Builtin {
	case ir.BuiltinSubgroupSize, ir.BuiltinSubgroupInvocationID,
		ir.BuiltinNumSubgroups, ir.BuiltinSubgroupID:
		return true
	}
	return false
}

// hasSubgroupBuiltin checks if any argument (or struct member) has a subgroup builtin.
func (w *Writer) hasSubgroupBuiltin(fn *ir.Function) bool {
	for _, arg := range fn.Arguments {
		if isSubgroupBuiltinBinding(arg.Binding) {
			return true
		}
		// Check struct members
		if int(arg.Type) < len(w.module.Types) {
			if st, ok := w.module.Types[arg.Type].Inner.(ir.StructType); ok {
				for _, member := range st.Members {
					if isSubgroupBuiltinBinding(member.Binding) {
						return true
					}
				}
			}
		}
	}
	return false
}

// =============================================================================
// Entry Point Interface Struct Generation (matches Rust naga)
// =============================================================================

// writeEPInterface decides whether to create input/output interface structs
// for an entry point. Matches Rust naga's write_ep_interface logic:
//   - Input struct: created for Fragment stage, or if any arg has subgroup builtin
//   - Output struct: created when result binding is None AND stage is Vertex
func (w *Writer) writeEPInterface(
	epIdx int,
	fn *ir.Function,
	stage ir.ShaderStage,
	epName string,
) (*entryPointInterface, error) {
	epIO := &entryPointInterface{}

	// Input struct: Fragment stage always gets one, or if subgroup builtins present
	needsInputStruct := false
	if len(fn.Arguments) > 0 {
		if stage == ir.StageFragment {
			needsInputStruct = true
		}
		if w.hasSubgroupBuiltin(fn) {
			needsInputStruct = true
		}
	}

	if needsInputStruct {
		input, err := w.writeEPInputStruct(epIdx, fn, stage, epName)
		if err != nil {
			return nil, err
		}
		epIO.input = input
	}

	// Output struct: Vertex stage with struct result (no direct binding on result)
	if fn.Result != nil && fn.Result.Binding == nil && stage == ir.StageVertex {
		output, err := w.writeEPOutputStruct(fn, stage, epName, w.options.FragmentEntryPoint)
		if err != nil {
			return nil, err
		}
		epIO.output = output
	}

	return epIO, nil
}

// writeEPInputStruct flattens all entry point arguments into a single input struct.
// Matches Rust naga's write_ep_input_struct.
func (w *Writer) writeEPInputStruct(
	epIdx int,
	fn *ir.Function,
	stage ir.ShaderStage,
	epName string,
) (*entryPointBinding, error) {
	structName := fmt.Sprintf("%sInput_%s", stageName(stage), epName)

	var fakeMembers []epStructMember
	for i, arg := range fn.Arguments {
		// Check if argument is a struct with member bindings
		if int(arg.Type) < len(w.module.Types) {
			if st, ok := w.module.Types[arg.Type].Inner.(ir.StructType); ok {
				for _, member := range st.Members {
					memberName := w.namer.callOr(member.Name, "member")
					idx := uint32(len(fakeMembers))
					fakeMembers = append(fakeMembers, epStructMember{
						name:    memberName,
						ty:      member.Type,
						binding: member.Binding,
						index:   idx,
					})
				}
				continue
			}
		}

		// Non-struct argument
		argName := fn.Arguments[i].Name
		memberName := w.namer.callOr(argName, "member")
		idx := uint32(len(fakeMembers))
		fakeMembers = append(fakeMembers, epStructMember{
			name:    memberName,
			ty:      arg.Type,
			binding: arg.Binding,
			index:   idx,
		})
	}

	return w.writeInterfaceStruct(stage, IoInput, structName, fakeMembers)
}

// writeEPOutputStruct flattens the entry point result struct into an output struct.
// If a fragment entry point is provided and stage is Vertex, outputs not consumed
// by the fragment shader are stripped. Matches Rust naga's write_ep_output_struct.
func (w *Writer) writeEPOutputStruct(
	fn *ir.Function,
	stage ir.ShaderStage,
	epName string,
	fragEP *FragmentEntryPoint,
) (*entryPointBinding, error) {
	structName := fmt.Sprintf("%sOutput_%s", stageName(stage), epName)

	resultType := fn.Result.Type
	if int(resultType) >= len(w.module.Types) {
		return nil, fmt.Errorf("result type out of range")
	}

	st, ok := w.module.Types[resultType].Inner.(ir.StructType)
	if !ok {
		return nil, fmt.Errorf("EP output type is not a struct")
	}

	// Gather fragment input locations to filter vertex outputs.
	// Only applied when a fragment entry point is provided and stage is Vertex.
	var fsInputLocs []uint32
	if fragEP != nil && stage == ir.StageVertex {
		for _, arg := range fragEP.Function.Arguments {
			if int(arg.Type) < len(fragEP.Module.Types) {
				if fragSt, ok := fragEP.Module.Types[arg.Type].Inner.(ir.StructType); ok {
					for _, m := range fragSt.Members {
						if m.Binding != nil {
							if loc, ok := (*m.Binding).(ir.LocationBinding); ok {
								fsInputLocs = append(fsInputLocs, loc.Location)
							}
						}
					}
					continue
				}
			}
			if arg.Binding != nil {
				if loc, ok := (*arg.Binding).(ir.LocationBinding); ok {
					fsInputLocs = append(fsInputLocs, loc.Location)
				}
			}
		}
		sort.Slice(fsInputLocs, func(i, j int) bool { return fsInputLocs[i] < fsInputLocs[j] })
	}

	var fakeMembers []epStructMember
	for i, member := range st.Members {
		// Filter out vertex outputs not consumed by fragment shader
		if len(fsInputLocs) > 0 {
			if member.Binding != nil {
				if loc, ok := (*member.Binding).(ir.LocationBinding); ok {
					found := sort.Search(len(fsInputLocs), func(k int) bool {
						return fsInputLocs[k] >= loc.Location
					})
					if found >= len(fsInputLocs) || fsInputLocs[found] != loc.Location {
						continue // Skip — not consumed by fragment shader
					}
				}
			}
		}

		memberName := w.namer.callOr(member.Name, "member")
		fakeMembers = append(fakeMembers, epStructMember{
			name:    memberName,
			ty:      member.Type,
			binding: member.Binding,
			index:   uint32(i),
		})
	}

	return w.writeInterfaceStruct(stage, IoOutput, structName, fakeMembers)
}

// writeInterfaceStruct writes an interface struct with members sorted by binding.
// Locations come first (ascending), then builtins. Matches Rust's write_interface_struct.
func (w *Writer) writeInterfaceStruct(
	stage ir.ShaderStage,
	io Io,
	structName string,
	members []epStructMember,
) (*entryPointBinding, error) {
	// Sort members: locations first (ascending), then builtins
	sort.SliceStable(members, func(i, j int) bool {
		ki := newInterfaceKey(members[i].binding)
		kj := newInterfaceKey(members[j].binding)
		return interfaceKeyLess(ki, kj)
	})

	stageIO := &shaderStageIO{stage: stage, io: io}

	// Check if any member has SubgroupId (need __local_invocation_index)
	hasSubgroupId := false
	for _, m := range members {
		if m.binding != nil {
			if bb, ok := (*m.binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinSubgroupID {
				hasSubgroupId = true
				break
			}
		}
	}

	fmt.Fprintf(&w.out, "struct %s", structName)
	w.out.WriteString(" {\n")
	for _, m := range members {
		// Skip subgroup builtins — they are computed from wave intrinsics
		if isSubgroupBuiltinBinding(m.binding) {
			continue
		}
		w.out.WriteString("    ")
		if m.binding != nil {
			w.writeModifierForType(m.binding, m.ty)
		}
		typeName := w.getTypeName(m.ty)
		fmt.Fprintf(&w.out, "%s %s", typeName, m.name)
		if m.binding != nil {
			w.writeSemantic(m.binding, stageIO)
		}
		w.out.WriteString(";\n")
	}
	// Add __local_invocation_index if SubgroupId is used
	if hasSubgroupId {
		w.out.WriteString("    uint __local_invocation_index : SV_GroupIndex;\n")
	}
	w.out.WriteString("};\n\n")

	// For input structs, restore original order (by index) for initialization
	if io == IoInput {
		sort.SliceStable(members, func(i, j int) bool {
			return members[i].index < members[j].index
		})
	}

	// Generate arg name from struct name (Rust uses to_lowercase, not just first char)
	argName := w.namer.call(strings.ToLower(structName))

	return &entryPointBinding{
		tyName:  structName,
		argName: argName,
		members: members,
	}, nil
}

// writeModifierForType writes interpolation/invariant modifiers, applying default
// interpolation for int/uint types (Flat -> nointerpolation).
func (w *Writer) writeModifierForType(binding *ir.Binding, ty ir.TypeHandle) {
	if binding == nil {
		return
	}
	switch b := (*binding).(type) {
	case ir.BuiltinBinding:
		if b.Builtin == ir.BuiltinPosition && b.Invariant {
			fmt.Fprintf(&w.out, "precise ")
		}
	case ir.LocationBinding:
		if b.Interpolation != nil {
			if kindStr := InterpolationToHLSL(b.Interpolation.Kind); kindStr != "" {
				fmt.Fprintf(&w.out, "%s ", kindStr)
			}
			if sampStr := SamplingToHLSL(b.Interpolation.Sampling); sampStr != "" {
				fmt.Fprintf(&w.out, "%s ", sampStr)
			}
		} else {
			// Apply default interpolation: int/uint -> nointerpolation (matches Rust naga)
			if int(ty) < len(w.module.Types) {
				kind, hasKind := getScalarKind(w.module, ty)
				if hasKind && (kind == ir.ScalarSint || kind == ir.ScalarUint) {
					fmt.Fprintf(&w.out, "nointerpolation ")
				}
			}
		}
	}
}

// writeModifier writes interpolation/invariant modifiers for a binding.
// Matches Rust naga's write_modifier.
func (w *Writer) writeModifier(binding *ir.Binding) {
	if binding == nil {
		return
	}
	switch b := (*binding).(type) {
	case ir.BuiltinBinding:
		if b.Builtin == ir.BuiltinPosition {
			// Check for invariant - for now, we don't track this
		}
	case ir.LocationBinding:
		if b.Interpolation != nil {
			if kindStr := InterpolationToHLSL(b.Interpolation.Kind); kindStr != "" {
				fmt.Fprintf(&w.out, "%s ", kindStr)
			}
			if sampStr := SamplingToHLSL(b.Interpolation.Sampling); sampStr != "" {
				fmt.Fprintf(&w.out, "%s ", sampStr)
			}
		}
	}
}

// =============================================================================
// Entry Point Function Writing
// =============================================================================

// writeEntryPointWithIO writes an entry point with proper input/output handling.
// Uses entryPointIO (populated by writeAllEPInterfaces) to determine signature format.
func (w *Writer) writeEntryPointWithIO(epIdx int, ep *ir.EntryPoint) error {
	fn := &ep.Function
	// Write per-function wrapped helpers (matching Rust naga order)
	w.writePerFunctionWrappedHelpers(fn)

	w.currentFunction = fn
	w.currentFuncHandle = epFuncHandle(epIdx)
	w.currentEPIndex = epIdx
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.updateExpressionsToBake(fn)
	w.continueCtx.clear()

	defer func() {
		w.currentFunction = nil
		w.localNames = nil
	}()

	epName := w.names[nameKey{kind: nameKeyEntryPoint, handle1: uint32(epIdx)}]
	epIO := w.entryPointIO[epIdx]

	// Write compute shader attributes
	if ep.Stage == ir.StageCompute {
		w.writeComputeAttributes(ep)
	}

	// Determine return type
	returnType := "void"
	if epIO != nil && epIO.output != nil {
		returnType = epIO.output.tyName
	} else if fn.Result != nil {
		returnType = w.getTypeName(fn.Result.Type)
	}

	// Write function signature with precise modifier if needed
	w.writeIndent()
	// Check for invariant Position on result binding -> "precise" prefix
	if fn.Result != nil && fn.Result.Binding != nil {
		if bb, ok := (*fn.Result.Binding).(ir.BuiltinBinding); ok {
			if bb.Builtin == ir.BuiltinPosition && bb.Invariant {
				w.out.WriteString("precise ")
			}
		}
	}
	fmt.Fprintf(&w.out, "%s %s(", returnType, epName)

	// Determine if workgroup init is needed
	needWgInit := w.needWorkgroupInit(ep)

	// Write parameters
	hasParams := false
	if epIO != nil && epIO.input != nil {
		// Input struct parameter
		fmt.Fprintf(&w.out, "%s %s", epIO.input.tyName, epIO.input.argName)
		hasParams = true
	} else {
		// Flat parameters with semantics (vertex stage, compute stage)
		for i, arg := range fn.Arguments {
			if i > 0 {
				w.out.WriteString(", ")
			}
			argType := w.getTypeName(arg.Type)
			argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(i)}]
			fmt.Fprintf(&w.out, "%s %s", argType, argName)
			if arg.Binding != nil {
				stageIO := &shaderStageIO{stage: ep.Stage, io: IoInput}
				w.writeSemantic(arg.Binding, stageIO)
			}
			hasParams = true
		}
	}

	// Add __local_invocation_id parameter for workgroup init
	if needWgInit {
		if hasParams {
			w.out.WriteString(", ")
		}
		w.out.WriteString("uint3 __local_invocation_id : SV_GroupThreadID")
	}

	w.out.WriteString(")")

	// Write return semantic for non-struct results (e.g., fragment SV_Target)
	if epIO == nil || epIO.output == nil {
		if fn.Result != nil && fn.Result.Binding != nil {
			stageIO := &shaderStageIO{stage: ep.Stage, io: IoOutput}
			w.writeSemantic(fn.Result.Binding, stageIO)
		}
	}

	// Opening brace on next line (matches Rust naga HLSL format)
	w.out.WriteString("\n")
	w.writeLine("{")
	w.pushIndent()

	// Write workgroup variable zero-initialization if needed
	if needWgInit {
		w.writeWorkgroupInit()
	}

	// Write EP arguments initialization (extract from input struct)
	if epIO != nil && epIO.input != nil {
		w.writeEPArgumentsInit(epIdx, fn, ep, epIO.input)
	}

	// Write local variables (use pre-registered names from registerNames)
	for localIdx, local := range fn.LocalVars {
		localName := w.names[nameKey{kind: nameKeyLocal, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(localIdx)}]
		if localName == "" {
			localName = w.namer.callOr(local.Name, "local")
		}
		localType, arraySuffix := w.getTypeNameWithArraySuffix(local.Type)

		w.localNames[uint32(localIdx)] = localName

		// Rust naga always initializes locals: with init expression or (Type)0.
		// Exception: RayQuery variables are NOT zero-initialized.
		w.writeIndent()
		isRayQuery := strings.Contains(localType, "RayQuery")
		if !isRayQuery && int(local.Type) < len(w.module.Types) {
			_, isRayQuery = w.module.Types[local.Type].Inner.(ir.RayQueryType)
		}
		if isRayQuery {
			fmt.Fprintf(&w.out, "%s %s%s;\n", localType, localName, arraySuffix)
		} else {
			fmt.Fprintf(&w.out, "%s %s%s = ", localType, localName, arraySuffix)
			if local.Init != nil {
				if err := w.writeExpression(*local.Init); err != nil {
					w.popIndent()
					return fmt.Errorf("entry point local var init: %w", err)
				}
			} else {
				// Zero initialize: (Type)0 matching Rust naga's write_default_init
				fmt.Fprintf(&w.out, "(%s%s)0", localType, arraySuffix)
			}
			w.out.WriteString(";\n")
		}
	}

	if len(fn.LocalVars) > 0 {
		// Rust naga writes just a newline (no indentation) after locals
		w.out.WriteByte('\n')
	}

	// Write function body statements
	if err := w.writeBlock(fn.Body); err != nil {
		w.popIndent()
		return err
	}

	// Add implicit return for void entry points
	if fn.Result == nil && !hlslBlockEndsWithReturn(fn.Body) {
		w.writeLine("return;")
	}

	w.popIndent()
	w.writeLine("}")
	w.writeLine("")

	return nil
}

// writeEPArgumentsInit writes the argument initialization code when using an input struct.
// Matches Rust naga's write_ep_arguments_initialization.
func (w *Writer) writeEPArgumentsInit(epIdx int, fn *ir.Function, ep *ir.EntryPoint, epInput *entryPointBinding) {
	fakeIter := 0
	for i, arg := range fn.Arguments {
		argName := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(epFuncHandle(epIdx)), handle2: uint32(i)}]
		argType := w.getTypeName(arg.Type)

		// Check if this is a struct argument
		if int(arg.Type) < len(w.module.Types) {
			if st, ok := w.module.Types[arg.Type].Inner.(ir.StructType); ok {
				// Struct argument: initialize with { member1, member2, ... }
				w.writeIndent()
				fmt.Fprintf(&w.out, "%s %s = { ", argType, argName)
				for j, member := range st.Members {
					if j > 0 {
						w.out.WriteString(", ")
					}
					if fakeIter < len(epInput.members) {
						w.writeEPArgInit(ep, epInput, &epInput.members[fakeIter], member.Binding)
						fakeIter++
					}
				}
				w.out.WriteString(" };\n")
				continue
			}
		}

		// Simple argument
		w.writeIndent()
		fmt.Fprintf(&w.out, "%s %s = ", argType, argName)
		if fakeIter < len(epInput.members) {
			w.writeEPArgInit(ep, epInput, &epInput.members[fakeIter], arg.Binding)
			fakeIter++
		}
		w.out.WriteString(";\n")
	}
}

// writeEPArgInit writes the initialization expression for a single EP argument.
// Subgroup builtins are computed from wave intrinsics; others read from the input struct.
// Matches Rust naga write_ep_argument_initialization.
func (w *Writer) writeEPArgInit(ep *ir.EntryPoint, epInput *entryPointBinding, fakeMember *epStructMember, binding *ir.Binding) {
	if binding != nil {
		if bb, ok := (*binding).(ir.BuiltinBinding); ok {
			switch bb.Builtin {
			case ir.BuiltinSubgroupSize:
				w.out.WriteString("WaveGetLaneCount()")
				return
			case ir.BuiltinSubgroupInvocationID:
				w.out.WriteString("WaveGetLaneIndex()")
				return
			case ir.BuiltinNumSubgroups:
				total := uint32(1)
				if ep != nil {
					total = ep.Workgroup[0] * ep.Workgroup[1] * ep.Workgroup[2]
					if total == 0 {
						total = 1
					}
				}
				fmt.Fprintf(&w.out, "(%du + WaveGetLaneCount() - 1u) / WaveGetLaneCount()", total)
				return
			case ir.BuiltinSubgroupID:
				fmt.Fprintf(&w.out, "%s.__local_invocation_index / WaveGetLaneCount()", epInput.argName)
				return
			}
		}
	}
	// Default: read from input struct
	fmt.Fprintf(&w.out, "%s.%s", epInput.argName, fakeMember.name)
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

// =============================================================================
// Helper Functions
// =============================================================================

// stageName returns the debug name for a shader stage (matches Rust's {:?} formatting).
func stageName(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "Vertex"
	case ir.StageFragment:
		return "Fragment"
	case ir.StageCompute:
		return "Compute"
	default:
		return "Unknown"
	}
}

// toLowerFirst lowercases the first character of a string.
func toLowerFirst(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'A' && b[0] <= 'Z' {
		b[0] += 32
	}
	return string(b)
}

// writeModHelper writes the safe modulo helper function.
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

// writeExtractBitsOverload writes a single naga_extractBits overload for a type.
// Matches Rust naga's write_wrapped_math_functions for ExtractBits.
func (w *Writer) writeExtractBitsOverload(typeName string, scalarWidth uint8) {
	fmt.Fprintf(&w.out, "%s %s(\n", typeName, NagaExtractBitsFunction)
	fmt.Fprintf(&w.out, "    %s e,\n", typeName)
	fmt.Fprintf(&w.out, "    uint offset,\n")
	fmt.Fprintf(&w.out, "    uint count\n")
	fmt.Fprintf(&w.out, ") {\n")
	fmt.Fprintf(&w.out, "    uint w = %d;\n", scalarWidth*8)
	fmt.Fprintf(&w.out, "    uint o = min(offset, w);\n")
	fmt.Fprintf(&w.out, "    uint c = min(count, w - o);\n")
	fmt.Fprintf(&w.out, "    return (c == 0 ? 0 : (e << (w - c - o)) >> (w - c));\n")
	fmt.Fprintf(&w.out, "}\n")
}

// writeInsertBitsOverload writes a single naga_insertBits overload for a type.
// Matches Rust naga's write_wrapped_math_functions for InsertBits.
func (w *Writer) writeInsertBitsOverload(typeName string, scalarWidth uint8) {
	scalarBits := uint64(scalarWidth) * 8
	var scalarMax uint64
	switch scalarWidth {
	case 1:
		scalarMax = 0xFF
	case 2:
		scalarMax = 0xFFFF
	case 4:
		scalarMax = 0xFFFFFFFF
	case 8:
		scalarMax = 0xFFFFFFFFFFFFFFFF
	default:
		scalarMax = 0xFFFFFFFF
	}
	fmt.Fprintf(&w.out, "%s %s(\n", typeName, NagaInsertBitsFunction)
	fmt.Fprintf(&w.out, "    %s e,\n", typeName)
	fmt.Fprintf(&w.out, "    %s newbits,\n", typeName)
	fmt.Fprintf(&w.out, "    uint offset,\n")
	fmt.Fprintf(&w.out, "    uint count\n")
	fmt.Fprintf(&w.out, ") {\n")
	fmt.Fprintf(&w.out, "    uint w = %du;\n", scalarBits)
	fmt.Fprintf(&w.out, "    uint o = min(offset, w);\n")
	fmt.Fprintf(&w.out, "    uint c = min(count, w - o);\n")
	fmt.Fprintf(&w.out, "    uint mask = ((%du >> (%du - c)) << o);\n", scalarMax, scalarBits)
	fmt.Fprintf(&w.out, "    return (c == 0 ? e : ((e & ~mask) | ((newbits << o) & mask)));\n")
	fmt.Fprintf(&w.out, "}\n")
}

// writeF2I32Helper writes the float-to-i32 conversion helper with clamping.
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
// Since entry point functions are stored inline in EntryPoint.Function (not in
// Module.Functions[]), any FunctionHandle into Module.Functions[] is by definition
// NOT an entry point.
func (w *Writer) isEntryPointFunction(_ ir.FunctionHandle) bool {
	return false
}

// getArgumentSemantic returns the HLSL semantic for a function argument binding.
func (w *Writer) getArgumentSemantic(arg ir.FunctionArgument, argIdx int) string {
	if arg.Binding == nil {
		return ""
	}
	return w.getSemanticFromBinding(*arg.Binding, argIdx)
}

// writeArgumentWithSemantic writes a function argument with its semantic.
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
func (w *Writer) getResultSemantic(result *ir.FunctionResult) string {
	if result == nil || result.Binding == nil {
		return ""
	}
	return w.getSemanticFromBinding(*result.Binding, 0)
}

// writeResultType writes the return type with semantic if applicable.
func (w *Writer) writeResultType(result *ir.FunctionResult) string {
	if result == nil {
		return "void"
	}

	typeName := w.getTypeName(result.Type)
	return typeName
}

// needWorkgroupInit returns true if the entry point needs workgroup variable
// zero-initialization. Matches Rust naga's need_workgroup_variables_initialization.
func (w *Writer) needWorkgroupInit(ep *ir.EntryPoint) bool {
	if !w.options.ZeroInitializeWorkgroupMemory {
		return false
	}
	// Only compute-like entry points
	if ep.Stage != ir.StageCompute {
		return false
	}
	// Check if there are any workgroup variables
	for _, gv := range w.module.GlobalVariables {
		if gv.Space == ir.SpaceWorkGroup {
			return true
		}
	}
	return false
}

// writeWorkgroupInit writes the workgroup variable zero-initialization code.
// Matches Rust naga's write_workgroup_variables_initialization.
//
// For array types, generates per-element loops instead of (Type[N])0 to avoid
// FXC compilation hangs (e.g. (PathMonoid[256])0 hangs 22s, loop takes 68ms).
// Nested arrays produce nested loops.
func (w *Writer) writeWorkgroupInit() {
	w.writeLine("if (all(__local_invocation_id == uint3(0u, 0u, 0u))) {")
	w.pushIndent()
	for i, gv := range w.module.GlobalVariables {
		if gv.Space != ir.SpaceWorkGroup {
			continue
		}
		varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(i)}]
		w.writeWorkgroupZeroInit(varName, gv.Type, 0)
	}
	w.popIndent()
	w.writeLine("}")
	w.writeLine("GroupMemoryBarrierWithGroupSync();")
}

// workgroupZeroInitLoopThreshold is the minimum array size (in elements) above
// which we use a per-element for loop instead of bulk (Type[N])0.
// FXC hangs on large struct arrays (e.g., PathMonoid[256] = 5120 bytes → 22s).
// Small arrays (2, 10 elements) compile fine with bulk assign.
// Threshold 256 covers the known problematic case (PathMonoid[256] = 5120 bytes
// hangs FXC 22s) while keeping smaller arrays identical to Rust naga output.
// array<i32, 128> (512 bytes) compiles fine with bulk assign.
const workgroupZeroInitLoopThreshold = 256

// writeWorkgroupZeroInit writes zero-initialization for a workgroup variable.
// For large array types (>= threshold), generates a per-element for loop to avoid
// FXC compilation hangs. Small arrays and non-arrays use bulk (Type)0 assign
// (matching Rust naga output).
// The depth parameter controls the loop variable suffix for nested arrays.
func (w *Writer) writeWorkgroupZeroInit(varExpr string, typeHandle ir.TypeHandle, depth int) {
	if int(typeHandle) >= len(w.module.Types) {
		typeName := w.getTypeName(typeHandle)
		w.writeLine("%s = (%s)0;", varExpr, typeName)
		return
	}
	typ := w.module.Types[typeHandle]
	if arr, ok := typ.Inner.(ir.ArrayType); ok && arr.Size.Constant != nil {
		size := *arr.Size.Constant
		if size >= workgroupZeroInitLoopThreshold {
			// Large array: per-element loop to avoid FXC hang
			loopVar := fmt.Sprintf("_naga_zi_%d", depth)
			w.writeLine("for (uint %s = 0u; %s < %du; %s++) {", loopVar, loopVar, size, loopVar)
			w.pushIndent()
			elemExpr := fmt.Sprintf("%s[%s]", varExpr, loopVar)
			w.writeWorkgroupZeroInit(elemExpr, arr.Base, depth+1)
			w.popIndent()
			w.writeLine("}")
		} else {
			// Small array: bulk assign (matches Rust naga, FXC handles fine)
			typeName := w.getTypeName(typeHandle)
			w.writeLine("%s = (%s)0;", varExpr, typeName)
		}
	} else {
		typeName := w.getTypeName(typeHandle)
		w.writeLine("%s = (%s)0;", varExpr, typeName)
	}
}
