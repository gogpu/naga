package emit

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// Emitter translates naga IR into a DXIL module.
type Emitter struct {
	ir   *ir.Module
	mod  *module.Module
	opts EmitOptions

	// Value numbering: maps naga expression handles to DXIL value IDs.
	// For scalar types, stores a single value ID.
	// For vector types, stores per-component IDs in exprComponents.
	exprValues map[ir.ExpressionHandle]int

	// Per-component value IDs for vector expressions.
	// Key is expression handle, value is slice of component IDs.
	exprComponents map[ir.ExpressionHandle][]int

	// Current function being emitted.
	currentBB *module.BasicBlock
	mainFn    *module.Function // the current function definition (for adding BBs)
	nextValue int              // next value ID to assign within a function

	// Maps local variable index to its alloca value ID (pointer).
	// For scalar types, this is the single alloca pointer.
	// For vector types, this is the pointer to the first component's alloca.
	localVarPtrs map[uint32]int

	// Per-component alloca pointers for vector local variables.
	// Maps local var index to a slice of alloca pointer IDs (one per component).
	localVarComponentPtrs map[uint32][]int

	// Cached DXIL types for struct local variables.
	// Used to ensure GEP source element type IDs match the alloca type IDs.
	localVarStructTypes map[uint32]*module.Type

	// Loop context stack for break/continue targets.
	loopStack []loopContext

	// dx.op function declarations (lazily created).
	dxOpFuncs map[dxOpKey]*module.Function

	// Constant cache: maps from constant key to emitter value ID.
	intConsts   map[int64]int  // int value -> emitter value ID
	floatConsts map[uint64]int // float bits -> emitter value ID
	undefID     int            // emitter value ID for undef, -1 if not created

	// constMap maps emitter value IDs to module constants.
	// Used during finalize to set proper Constant.ValueID.
	constMap map[int]*module.Constant

	// pendingComponents holds per-component IDs from the last
	// emitCompose call. Used by emitExpression to store in exprComponents.
	pendingComponents []int

	// Resource bindings: analyzed from GlobalVariables.
	resources       []resourceInfo
	resourceHandles map[ir.GlobalVariableHandle]int // global var handle -> index in resources

	// Cached DXIL resource types (lazily created).
	dxHandleType *module.Type
}

// loopContext holds the branch targets for the innermost loop,
// used by break and continue statements to emit correct branches.
type loopContext struct {
	continuingBBIndex int // basic block index for the continuing block
	mergeBBIndex      int // basic block index for the merge (after-loop) block
}

// dxOpKey uniquely identifies a dx.op function declaration.
type dxOpKey struct {
	name     string
	overload overloadType
}

// Overload suffix strings for dx.op function names.
const (
	suffixF32 = ".f32"
	suffixI32 = ".i32"
	suffixF64 = ".f64"
)

// overloadType indicates the type overload for a dx.op function.
type overloadType int

const (
	overloadVoid overloadType = iota
	overloadI1
	overloadI16
	overloadI32
	overloadI64
	overloadF16
	overloadF32
	overloadF64
)

// EmitOptions configures DXIL emission.
type EmitOptions struct {
	ShaderModelMajor uint32
	ShaderModelMinor uint32
}

// Emit translates a naga IR module into a DXIL module.Module.
//
// The caller is responsible for serializing the result to bitcode
// and wrapping it in a DXBC container.
func Emit(irMod *ir.Module, opts EmitOptions) (*module.Module, error) {
	if len(irMod.EntryPoints) == 0 {
		return nil, fmt.Errorf("dxil/emit: module has no entry points")
	}

	ep := &irMod.EntryPoints[0]
	shaderKind := stageToShaderKind(ep.Stage)

	mod := module.NewModule(shaderKind)
	mod.MajorVersion = 1
	mod.MinorVersion = opts.ShaderModelMinor

	e := &Emitter{
		ir:              irMod,
		mod:             mod,
		opts:            opts,
		exprValues:      make(map[ir.ExpressionHandle]int),
		dxOpFuncs:       make(map[dxOpKey]*module.Function),
		intConsts:       make(map[int64]int),
		floatConsts:     make(map[uint64]int),
		undefID:         -1,
		constMap:        make(map[int]*module.Constant),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}

	if err := e.emitEntryPoint(ep); err != nil {
		return nil, fmt.Errorf("dxil/emit: entry point %q: %w", ep.Name, err)
	}

	return mod, nil
}

// stageToShaderKind maps naga ShaderStage to DXIL ShaderKind.
func stageToShaderKind(stage ir.ShaderStage) module.ShaderKind {
	switch stage {
	case ir.StageVertex:
		return module.VertexShader
	case ir.StageFragment:
		return module.PixelShader
	case ir.StageCompute:
		return module.ComputeShader
	default:
		return module.VertexShader
	}
}

// shaderKindString returns the DXIL shader model prefix for metadata.
func shaderKindString(kind module.ShaderKind) string {
	switch kind {
	case module.VertexShader:
		return "vs"
	case module.PixelShader:
		return "ps"
	case module.ComputeShader:
		return "cs"
	case module.GeometryShader:
		return "gs"
	case module.HullShader:
		return "hs"
	case module.DomainShader:
		return "ds"
	default:
		return "vs"
	}
}

// emitEntryPoint emits a single entry point function.
func (e *Emitter) emitEntryPoint(ep *ir.EntryPoint) error {
	voidTy := e.mod.GetVoidType()
	funcTy := e.mod.GetFunctionType(voidTy, nil)

	mainFn := e.mod.AddFunction("main", funcTy, false)
	e.mainFn = mainFn
	bb := mainFn.AddBasicBlock("entry")
	e.currentBB = bb
	e.exprValues = make(map[ir.ExpressionHandle]int)
	e.exprComponents = make(map[ir.ExpressionHandle][]int)
	e.localVarPtrs = make(map[uint32]int)
	e.localVarComponentPtrs = make(map[uint32][]int)
	e.localVarStructTypes = make(map[uint32]*module.Type)
	e.loopStack = e.loopStack[:0]

	fn := &ep.Function

	// Analyze and create resource bindings before function body emission.
	e.analyzeResources()

	// Emit I/O: load inputs into value map, then emit function body,
	// then store outputs. At this point we use temporary IDs starting
	// from 0. These will be adjusted in finalize().
	e.nextValue = 0

	e.emitResourceHandles()

	if err := e.emitInputLoads(fn, ep.Stage); err != nil {
		return fmt.Errorf("input loads: %w", err)
	}

	if err := e.emitFunctionBody(fn); err != nil {
		return fmt.Errorf("function body: %w", err)
	}

	// Add void return at the end.
	bb = e.currentBB
	bb.AddInstruction(module.NewRetVoidInstr())

	// Emit metadata BEFORE finalize so all constants are known.
	e.emitMetadata(ep, stageToShaderKind(ep.Stage))

	// Finalize: assign proper value IDs that match serializer's numbering.
	e.finalize(mainFn)

	return nil
}

// finalize remaps the emitter's internal value IDs to match the
// serializer's global value numbering.
//
// The serializer assigns value IDs in this order:
//  1. Global vars: 0..gv_count-1
//  2. Functions: gv_count..gv_count+fn_count-1
//  3. Constants: in mod.Constants order (insertion order)
//  4. Instruction results: sequentially after constants
//
// This method builds a mapping from emitter IDs to global IDs and
// rewrites all instruction operands and value IDs.
//
//nolint:gocognit,gocyclo,cyclop // value ID remapping requires iterating multiple data structures
func (e *Emitter) finalize(mainFn *module.Function) {
	// Build the ID mapping table: emitterID -> globalID
	idMap := make(map[int]int)

	nextGlobalID := 0

	// Global vars.
	for _, gv := range e.mod.GlobalVars {
		gv.ValueID = nextGlobalID
		nextGlobalID++
	}

	// Functions get sequential IDs.
	for _, fn := range e.mod.Functions {
		fn.ValueID = nextGlobalID
		nextGlobalID++
	}

	// Constants: assign IDs in mod.Constants order (which matches
	// the serializer's order). Build a reverse lookup from
	// *module.Constant to emitter ID to find the mapping.
	constToEmitterID := make(map[*module.Constant]int)
	for emitterID, c := range e.constMap {
		constToEmitterID[c] = emitterID
	}
	for _, c := range e.mod.Constants {
		c.ValueID = nextGlobalID
		if emitterID, ok := constToEmitterID[c]; ok {
			idMap[emitterID] = nextGlobalID
		}
		nextGlobalID++
	}

	// Instruction results: process in order, assigning sequential IDs.
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.HasValue {
				oldID := instr.ValueID
				if _, isConst := e.constMap[oldID]; !isConst {
					idMap[oldID] = nextGlobalID
					instr.ValueID = nextGlobalID
					nextGlobalID++
				}
			}
		}
	}

	// Rewrite operand references, but ONLY for operands that are value IDs.
	// Some instructions mix value IDs with type IDs, literal opcodes,
	// alignment values, and basic block indices — those must not be remapped.
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			valueOpIndices := valueOperandIndices(instr)
			for _, idx := range valueOpIndices {
				if idx < len(instr.Operands) {
					if newID, ok := idMap[instr.Operands[idx]]; ok {
						instr.Operands[idx] = newID
					}
				}
			}
			if instr.Kind == module.InstrRet && instr.ReturnValue >= 0 {
				if newID, ok := idMap[instr.ReturnValue]; ok {
					instr.ReturnValue = newID
				}
			}
		}
	}

	// Also rewrite component tracking IDs.
	for handle, comps := range e.exprComponents {
		for i, id := range comps {
			if newID, ok := idMap[id]; ok {
				e.exprComponents[handle][i] = newID
			}
		}
	}
}

// valueOperandIndices returns the indices within Instruction.Operands that
// contain value IDs (and thus need remapping in finalize). Other operands
// hold type IDs, literal opcodes, alignment values, or basic block indices
// which must NOT be remapped.
//
// Instruction operand layouts:
//
//	InstrBinOp:      [lhs, rhs, opcode]           → values at [0, 1]
//	InstrCmp:        [lhs, rhs, predicate]         → values at [0, 1]
//	InstrSelect:     [cond, trueVal, falseVal]     → values at [0, 1, 2]
//	InstrCast:       [src, castOpcode]              → values at [0]
//	InstrExtractVal: [src, index]                   → values at [0]
//	InstrAlloca:     [allocTyID, sizeTyID, sizeValID, alignFlags] → values at [2]
//	InstrLoad:       [ptr, typeID, align, isVolatile] → values at [0]
//	InstrStore:      [ptr, value, align, isVolatile]  → values at [0, 1]
//	InstrCall:       [arg0, arg1, ...]              → all are values
//	InstrBr:         [bbIdx] or [trueBB, falseBB, cond] → values at [2] for cond branch
//	InstrRet:        uses ReturnValue field, not Operands → none
//	InstrGEP:        not yet used
//	InstrPhi:        not yet used
func valueOperandIndices(instr *module.Instruction) []int {
	switch instr.Kind {
	case module.InstrBinOp:
		return []int{0, 1} // [lhs, rhs, opcode]
	case module.InstrCmp:
		return []int{0, 1} // [lhs, rhs, pred]
	case module.InstrSelect:
		return []int{0, 1, 2} // [cond, true, false]
	case module.InstrCast:
		return []int{0} // [src, castOp]
	case module.InstrExtractVal:
		return []int{0} // [src, index]
	case module.InstrAlloca:
		return []int{2} // [allocTyID, sizeTyID, sizeValID, alignFlags]
	case module.InstrLoad:
		return []int{0} // [ptr, typeID, align, isVolatile]
	case module.InstrStore:
		return []int{0, 1} // [ptr, value, align, isVolatile]
	case module.InstrCall:
		// All call operands are value IDs (arguments to the called function).
		indices := make([]int, len(instr.Operands))
		for i := range indices {
			indices[i] = i
		}
		return indices
	case module.InstrBr:
		if len(instr.Operands) >= 3 {
			return []int{2} // [trueBB, falseBB, cond] — only cond is a value
		}
		return nil // unconditional branch: [bbIndex] — no value operands
	case module.InstrRet:
		return nil // uses ReturnValue field
	case module.InstrGEP:
		// GEP: [inbounds, sourceElemTypeID, ptrValueID, ...indexValueIDs]
		// Values at indices 2..N (ptr and all index operands).
		indices := make([]int, 0, len(instr.Operands)-2)
		for i := 2; i < len(instr.Operands); i++ {
			indices = append(indices, i)
		}
		return indices
	default:
		// GEP, Phi: not yet used. Return all as safe fallback.
		indices := make([]int, len(instr.Operands))
		for i := range indices {
			indices[i] = i
		}
		return indices
	}
}

// emitMetadata writes the required DXIL metadata nodes.
func (e *Emitter) emitMetadata(ep *ir.EntryPoint, kind module.ShaderKind) {
	i32Ty := e.mod.GetIntType(32)

	// dx.version = !{i32 1, i32 MINOR}
	mdMajor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(1))
	mdMinor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(e.opts.ShaderModelMinor)))
	mdVersion := e.mod.AddMetadataTuple([]*module.MetadataNode{mdMajor, mdMinor})
	e.mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersion})

	// dx.valver = !{i32 1, i32 7}
	mdValMajor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(1))
	mdValMinor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(7))
	mdValVer := e.mod.AddMetadataTuple([]*module.MetadataNode{mdValMajor, mdValMinor})
	e.mod.AddNamedMetadata("dx.valver", []*module.MetadataNode{mdValVer})

	// dx.shaderModel = !{!"vs", i32 6, i32 MINOR}
	mdKind := e.mod.AddMetadataString(shaderKindString(kind))
	mdSMMajor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(6))
	mdSMMinor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(e.opts.ShaderModelMinor)))
	mdSM := e.mod.AddMetadataTuple([]*module.MetadataNode{mdKind, mdSMMajor, mdSMMinor})
	e.mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})

	// Emit resource metadata if we have any resources.
	mdResources := e.emitResourceMetadata()

	// Build shader properties (tag-value pairs) for the entry point.
	// Compute shaders require kDxilNumThreadsTag (4) with workgroup dimensions.
	//
	// Reference: Mesa nir_to_dxil.c emit_metadata() ~2038,
	// DXIL.rst: kDxilNumThreadsTag(4) MD list: (i32, i32, i32)
	var mdProperties *module.MetadataNode
	if kind == module.ComputeShader {
		mdProperties = e.emitComputeProperties(ep)
	}

	// dx.entryPoints = !{!{null, !"main", null, !resources, !properties}}
	mdName := e.mod.AddMetadataString("main")
	mdEntry := e.mod.AddMetadataTuple([]*module.MetadataNode{
		nil,          // function ref (simplified)
		mdName,       // entry point name
		nil,          // signatures
		mdResources,  // resources (nil if none)
		mdProperties, // properties (nil if none)
	})
	e.mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	// llvm.ident = !{!"dxil-go-naga"}
	mdIdent := e.mod.AddMetadataString("dxil-go-naga")
	mdIdentTuple := e.mod.AddMetadataTuple([]*module.MetadataNode{mdIdent})
	e.mod.AddNamedMetadata("llvm.ident", []*module.MetadataNode{mdIdentTuple})
}

// emitComputeProperties builds the shader properties metadata for a compute
// shader entry point. Contains tag-value pairs for numthreads.
//
// Format: !{i32 kDxilNumThreadsTag, !{i32 X, i32 Y, i32 Z}}
// kDxilNumThreadsTag = 4
//
// Reference: Mesa nir_to_dxil.c emit_threads() ~1785, emit_tag() ~1967
func (e *Emitter) emitComputeProperties(ep *ir.EntryPoint) *module.MetadataNode {
	i32Ty := e.mod.GetIntType(32)

	// Tag = kDxilNumThreadsTag = 4.
	mdTag := e.mod.AddMetadataValue(i32Ty, e.getIntConst(4))

	// Value = !{i32 X, i32 Y, i32 Z} with minimum of 1 per axis.
	x := max32(ep.Workgroup[0], 1)
	y := max32(ep.Workgroup[1], 1)
	z := max32(ep.Workgroup[2], 1)

	mdX := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(x)))
	mdY := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(y)))
	mdZ := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(z)))
	mdThreads := e.mod.AddMetadataTuple([]*module.MetadataNode{mdX, mdY, mdZ})

	// Properties = !{tag, value, ...}
	return e.mod.AddMetadataTuple([]*module.MetadataNode{mdTag, mdThreads})
}

// max32 returns the larger of a and b.
func max32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

// getIntConstID returns the emitter value ID for a cached i32 constant.
func (e *Emitter) getIntConstID(v int64) int {
	if id, ok := e.intConsts[v]; ok {
		return id
	}
	c := e.mod.AddIntConst(e.mod.GetIntType(32), v)
	id := e.allocValue()
	e.intConsts[v] = id
	e.constMap[id] = c
	return id
}

// getI8ConstID returns the emitter value ID for a cached i8 constant.
// Used for dx.op call arguments that require i8 type (e.g., col index).
func (e *Emitter) getI8ConstID(v int64) int {
	// Use a distinct cache key to avoid colliding with i32 constants.
	key := v + (1 << 40) // offset to distinguish from i32
	if id, ok := e.intConsts[key]; ok {
		return id
	}
	c := e.mod.AddIntConst(e.mod.GetIntType(8), v)
	id := e.allocValue()
	e.intConsts[key] = id
	e.constMap[id] = c
	return id
}

// getFloatConstID returns the emitter value ID for a cached f32 constant.
func (e *Emitter) getFloatConstID(v float64) int {
	bits := math.Float64bits(v)
	if id, ok := e.floatConsts[bits]; ok {
		return id
	}
	c := e.addFloatConst(e.mod.GetFloatType(32), v)
	id := e.allocValue()
	e.floatConsts[bits] = id
	e.constMap[id] = c
	return id
}

// addFloatConstID adds a floating-point constant and returns its emitter value ID.
func (e *Emitter) addFloatConstID(ty *module.Type, v float64) int {
	c := e.addFloatConst(ty, v)
	id := e.allocValue()
	e.constMap[id] = c
	return id
}

// addFloatConst adds a floating-point constant to the module.
func (e *Emitter) addFloatConst(ty *module.Type, v float64) *module.Constant {
	c := &module.Constant{
		ConstType:  ty,
		FloatValue: v,
	}
	e.mod.Constants = append(e.mod.Constants, c)
	return c
}

// getUndefConstID returns the emitter value ID for an undef i32 constant.
func (e *Emitter) getUndefConstID() int {
	if e.undefID >= 0 {
		return e.undefID
	}
	c := &module.Constant{
		ConstType: e.mod.GetIntType(32),
		IsUndef:   true,
	}
	e.mod.Constants = append(e.mod.Constants, c)
	id := e.allocValue()
	e.undefID = id
	e.constMap[id] = c
	return id
}

// getIntConst is a compatibility helper that returns the module constant.
// Used only for metadata emission where we need the *module.Constant.
func (e *Emitter) getIntConst(v int64) *module.Constant {
	// Check if already created.
	if id, ok := e.intConsts[v]; ok {
		return e.constMap[id]
	}
	c := e.mod.AddIntConst(e.mod.GetIntType(32), v)
	id := e.allocValue()
	e.intConsts[v] = id
	e.constMap[id] = c
	return c
}

// getComponentID returns the value ID for a specific component of a
// vector expression. If per-component tracking exists, uses that.
// Otherwise falls back to baseID + component (legacy behavior).
func (e *Emitter) getComponentID(handle ir.ExpressionHandle, comp int) int {
	if comps, ok := e.exprComponents[handle]; ok && comp < len(comps) {
		return comps[comp]
	}
	// Fallback: assume sequential IDs from base.
	if base, ok := e.exprValues[handle]; ok {
		return base + comp
	}
	return 0
}

// allocValue assigns the next value ID and returns it.
func (e *Emitter) allocValue() int {
	v := e.nextValue
	e.nextValue++
	return v
}

// addCallInstr adds a call instruction to the current basic block and
// returns the assigned value ID.
func (e *Emitter) addCallInstr(fn *module.Function, resultTy *module.Type, operands []int) int {
	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrCall,
		HasValue:   resultTy.Kind != module.TypeVoid,
		ResultType: resultTy,
		CalledFunc: fn,
		Operands:   operands,
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// addBinOpInstr adds a binary operation instruction.
func (e *Emitter) addBinOpInstr(resultTy *module.Type, op BinOpKind, lhs, rhs int) int {
	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrBinOp,
		HasValue:   true,
		ResultType: resultTy,
		Operands:   []int{lhs, rhs, int(op)},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// addGEPInstr adds a getelementptr instruction and returns the result pointer value ID.
// sourceElemTy is the type of the element being pointed to (before indexing).
// resultTy is the pointer type of the result.
// ptrID is the base pointer value ID.
// indexIDs are the index value IDs (first is always the array-level index, then struct indices).
//
// Reference: Mesa dxil_module.c dxil_emit_gep_instr()
func (e *Emitter) addGEPInstr(sourceElemTy, resultTy *module.Type, ptrID int, indexIDs []int) int {
	valueID := e.allocValue()
	operands := make([]int, 3+len(indexIDs))
	operands[0] = 1 // inbounds = true
	operands[1] = sourceElemTy.ID
	operands[2] = ptrID
	for i, idx := range indexIDs {
		operands[3+i] = idx
	}
	instr := &module.Instruction{
		Kind:       module.InstrGEP,
		HasValue:   true,
		ResultType: resultTy,
		Operands:   operands,
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// addCmpInstr adds a comparison instruction.
func (e *Emitter) addCmpInstr(pred CmpPredicate, lhs, rhs int) int {
	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrCmp,
		HasValue:   true,
		ResultType: e.mod.GetIntType(1),
		Operands:   []int{lhs, rhs, int(pred)},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// getDxOpStoreFunc creates a dx.op.storeOutput function declaration.
// storeOutput: void @dx.op.storeOutput.TYPE(i32 opcode, i32 outputID, i32 row, i8 col, TYPE value)
func (e *Emitter) getDxOpStoreFunc(ol overloadType) *module.Function {
	name := "dx.op.storeOutput"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valueTy := e.overloadReturnType(ol)

	fullName := name
	switch ol {
	case overloadF32:
		fullName += suffixF32
	case overloadI32:
		fullName += suffixI32
	case overloadF64:
		fullName += suffixF64
	}

	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, valueTy}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpLoadFunc creates a dx.op.loadInput function declaration.
// loadInput: TYPE @dx.op.loadInput.TYPE(i32 opcode, i32 inputID, i32 row, i8 col, i32 vertexID)
func (e *Emitter) getDxOpLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.loadInput"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	retTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)

	fullName := name
	switch ol {
	case overloadF32:
		fullName += suffixF32
	case overloadI32:
		fullName += suffixI32
	case overloadF64:
		fullName += suffixF64
	}

	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// overloadSuffix returns the type suffix string for a given overload.
func overloadSuffix(ol overloadType) string {
	switch ol {
	case overloadF16:
		return ".f16"
	case overloadF32:
		return suffixF32
	case overloadF64:
		return suffixF64
	case overloadI16:
		return ".i16"
	case overloadI32:
		return suffixI32
	case overloadI64:
		return ".i64"
	case overloadI1:
		return ".i1"
	default:
		return ""
	}
}

// getDxOpUnaryFunc creates a dx.op unary function declaration.
// Signature: TYPE @dx.op.NAME.TYPE(i32 opcode, TYPE value)
func (e *Emitter) getDxOpUnaryFunc(name string, ol overloadType) *module.Function {
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	retTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)

	params := []*module.Type{i32Ty, retTy}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpBinaryFunc creates a dx.op binary function declaration.
// Signature: TYPE @dx.op.NAME.TYPE(i32 opcode, TYPE a, TYPE b)
func (e *Emitter) getDxOpBinaryFunc(name string, ol overloadType) *module.Function {
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	retTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)

	params := []*module.Type{i32Ty, retTy, retTy}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpTernaryFunc creates a dx.op ternary function declaration.
// Signature: TYPE @dx.op.NAME.TYPE(i32 opcode, TYPE a, TYPE b, TYPE c)
func (e *Emitter) getDxOpTernaryFunc(name string, ol overloadType) *module.Function {
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	retTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)

	params := []*module.Type{i32Ty, retTy, retTy, retTy}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpDotFunc creates a dx.op dot product function declaration.
// Dot products take 2*size scalar params and return a scalar.
// Signature: float @dx.op.dotN.f32(i32 opcode, float ax, float ay, ..., float bx, float by, ...)
func (e *Emitter) getDxOpDotFunc(size int, ol overloadType) *module.Function {
	name := fmt.Sprintf("dx.op.dot%d", size)
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	retTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)

	// params: i32 opcode, then 2*size scalar values
	params := make([]*module.Type, 1+2*size)
	params[0] = i32Ty
	for i := 1; i < len(params); i++ {
		params[i] = retTy
	}

	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// overloadReturnType returns the DXIL type for the given overload.
func (e *Emitter) overloadReturnType(ol overloadType) *module.Type {
	switch ol {
	case overloadVoid:
		return e.mod.GetVoidType()
	case overloadI1:
		return e.mod.GetIntType(1)
	case overloadI16:
		return e.mod.GetIntType(16)
	case overloadI32:
		return e.mod.GetIntType(32)
	case overloadI64:
		return e.mod.GetIntType(64)
	case overloadF16:
		return e.mod.GetFloatType(16)
	case overloadF32:
		return e.mod.GetFloatType(32)
	case overloadF64:
		return e.mod.GetFloatType(64)
	default:
		return e.mod.GetIntType(32)
	}
}

// overloadForScalar picks the overload type matching a naga scalar.
func overloadForScalar(s ir.ScalarType) overloadType {
	switch s.Kind {
	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return overloadF16
		case 8:
			return overloadF64
		default:
			return overloadF32
		}
	case ir.ScalarSint, ir.ScalarUint:
		switch s.Width {
		case 2:
			return overloadI16
		case 8:
			return overloadI64
		default:
			return overloadI32
		}
	case ir.ScalarBool:
		return overloadI1
	default:
		return overloadI32
	}
}
