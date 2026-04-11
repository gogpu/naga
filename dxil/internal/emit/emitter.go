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

	// Cached DXIL types for array local variables.
	// Used by emitAccess to emit GEP into array allocas.
	localVarArrayTypes map[uint32]*module.Type

	// Maps IR global variable handle to its alloca pointer value ID.
	// Used for non-resource globals (workgroup, private) that need pointer semantics.
	globalVarAllocas map[ir.GlobalVariableHandle]int

	// Cached DXIL types for global variable allocas.
	// Used by emitAccess to emit GEP into array/struct allocas.
	globalVarAllocaTypes map[ir.GlobalVariableHandle]*module.Type

	// Loop context stack for break/continue targets.
	loopStack []loopContext

	// dx.op function declarations (lazily created).
	dxOpFuncs map[dxOpKey]*module.Function

	// Constant cache: maps from constant key to emitter value ID.
	intConsts   map[int64]int  // int value -> emitter value ID
	floatConsts map[uint64]int // float bits -> emitter value ID
	undefID     int            // emitter value ID for undef, -1 if not created
	typedUndefs map[int]int    // type ID -> emitter value ID for typed undefs

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

	// Maps call result expression handles to their DXIL value IDs.
	// Set by emitStmtCall, read by ExprCallResult.
	callResultValues map[ir.ExpressionHandle]int

	// Maps call result expression handles to per-component value IDs.
	// For struct/vector returns from helper functions.
	callResultComponents map[ir.ExpressionHandle][]int

	// Maps IR function handles to their emitted DXIL function declarations.
	helperFunctions map[ir.FunctionHandle]*module.Function

	// Per-helper-function constant maps: maps emitter value IDs to module constants.
	// Each helper function gets its own value ID space for constants, stored here
	// so that finalizeHelperFunction can correctly remap them.
	helperConstMaps map[ir.FunctionHandle]map[int]*module.Constant

	// True when emitting a helper function body (not an entry point).
	// Affects return statement emission: helper functions use LLVM ret,
	// entry points use dx.op.storeOutput.
	emittingHelperFunction bool

	// Number of components in the current helper function's return type.
	// >1 means the return is packed into a struct {scalar, scalar, ...}.
	// Set before emitting helper function body, used by emitStmtReturn.
	helperReturnComps int

	// Mesh shader context: set when emitting a mesh shader entry point.
	meshCtx *meshContext
}

// meshContext holds state for mesh shader intrinsic emission.
type meshContext struct {
	// outputVar is the GlobalVariableHandle for the mesh output workgroup variable.
	outputVar ir.GlobalVariableHandle

	// meshInfo contains mesh shader metadata from the entry point.
	meshInfo *ir.MeshStageInfo

	// Signature element IDs (assigned during entry point setup).
	// vertexOutputSigIDs maps (builtin/location) to output signature element IDs.
	vertexOutputSigIDs map[meshOutputKey]int
	// primitiveOutputSigIDs maps (builtin/location) to primitive output signature element IDs.
	primitiveOutputSigIDs map[meshOutputKey]int

	// nextVertexSigID and nextPrimitiveSigID track the next available signature element IDs.
	nextVertexSigID    int
	nextPrimitiveSigID int

	// pendingVertexCount and pendingPrimitiveCount are value IDs for
	// buffered SetMeshOutputCounts arguments. Both must be set before emitting.
	pendingVertexCount    int
	pendingPrimitiveCount int
	hasVertexCount        bool
	hasPrimitiveCount     bool

	// taskPayloadVar is the GlobalVariableHandle for the task payload variable (if any).
	taskPayloadVar *ir.GlobalVariableHandle
}

// meshOutputKey identifies a mesh output by its type and location/builtin.
type meshOutputKey struct {
	isBuiltin bool
	builtin   ir.BuiltinValue
	location  uint32
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
		ir:                   irMod,
		mod:                  mod,
		opts:                 opts,
		exprValues:           make(map[ir.ExpressionHandle]int),
		dxOpFuncs:            make(map[dxOpKey]*module.Function),
		intConsts:            make(map[int64]int),
		floatConsts:          make(map[uint64]int),
		undefID:              -1,
		constMap:             make(map[int]*module.Constant),
		resourceHandles:      make(map[ir.GlobalVariableHandle]int),
		callResultValues:     make(map[ir.ExpressionHandle]int),
		callResultComponents: make(map[ir.ExpressionHandle][]int),
		helperFunctions:      make(map[ir.FunctionHandle]*module.Function),
		helperConstMaps:      make(map[ir.FunctionHandle]map[int]*module.Constant),
	}

	// Pre-scan the entry point to find which helper functions are actually called.
	calledFunctions := collectCalledFunctions(&ep.Function)
	if err := e.emitHelperFunctions(calledFunctions); err != nil {
		return nil, fmt.Errorf("dxil/emit: helper functions: %w", err)
	}

	if err := e.emitEntryPoint(ep); err != nil {
		return nil, fmt.Errorf("dxil/emit: entry point %q: %w", ep.Name, err)
	}

	// Finalize helper functions AFTER the entry point finalize
	// (which assigns final constant ValueIDs).
	for handle, helperFn := range e.helperFunctions {
		helperConstMap := e.helperConstMaps[handle]
		e.finalizeHelperFunction(helperFn, helperConstMap)
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
	case ir.StageMesh:
		return module.MeshShader
	case ir.StageTask:
		return module.AmplificationShader
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
	case module.MeshShader:
		return "ms"
	case module.AmplificationShader:
		return "as"
	default:
		return "vs"
	}
}

// emitHelperFunctions emits LLVM function definitions for all non-entry-point
// functions in the IR module. These are called via StmtCall from entry points.
//
// Each helper function is emitted as a real LLVM function with parameters and
// a return value. DXIL scalarizes all types, so vector/struct returns are
// returned as their scalar element type (the first component).
//
// collectCalledFunctions scans a function's statements for StmtCall that have
// a Result (non-void calls where the return value is used). Only these helpers
// need to be emitted to avoid ExprCallResult errors. Void calls can be safely
// skipped since their side effects are in the entry point body.
func collectCalledFunctions(fn *ir.Function) map[ir.FunctionHandle]bool {
	result := make(map[ir.FunctionHandle]bool)
	collectCallsFromBlock(fn.Body, result)
	return result
}

// functionHasComplexLocals returns true if any local variable has a type that
// cannot be correctly scalarized in DXIL helper functions (arrays, matrices, structs).
func functionHasComplexLocals(fn *ir.Function, irMod *ir.Module) bool {
	for i := range fn.LocalVars {
		irType := irMod.Types[fn.LocalVars[i].Type]
		switch irType.Inner.(type) {
		case ir.ArrayType, ir.MatrixType, ir.StructType:
			return true
		}
	}
	return false
}

// functionHasComplexExpressions returns true if the function body contains
// expressions that cannot yet be correctly emitted in DXIL helper functions.
// This includes: ExprCompose for arrays/structs, ExprSelect (uses i1 that
// may produce Invalid record in some contexts), etc.
func functionHasComplexExpressions(fn *ir.Function, irMod *ir.Module) bool {
	for i := range fn.Expressions {
		switch ek := fn.Expressions[i].Kind.(type) {
		case ir.ExprCompose:
			// Check if compose target is array or struct (not vector).
			if int(ek.Type) < len(irMod.Types) {
				ty := irMod.Types[ek.Type]
				switch ty.Inner.(type) {
				case ir.ArrayType, ir.StructType:
					return true
				}
			}
		}
	}
	return false
}

// functionAccessesGlobals returns true if any expression in the function is
// ExprGlobalVariable. Helper functions that access globals produce incorrect
// DXIL because global variable scalarization in alloca isn't properly handled.
func functionAccessesGlobals(fn *ir.Function) bool {
	for i := range fn.Expressions {
		if _, ok := fn.Expressions[i].Kind.(ir.ExprGlobalVariable); ok {
			return true
		}
	}
	return false
}

func collectCallsFromBlock(stmts ir.Block, result map[ir.FunctionHandle]bool) {
	for i := range stmts {
		switch sk := stmts[i].Kind.(type) {
		case ir.StmtCall:
			// Only include calls with a result (non-void functions whose
			// return value is used). This avoids emitting void helpers
			// whose side effects can't be captured anyway.
			if sk.Result != nil {
				result[sk.Function] = true
			}
		case ir.StmtBlock:
			collectCallsFromBlock(sk.Block, result)
		case ir.StmtIf:
			collectCallsFromBlock(sk.Accept, result)
			collectCallsFromBlock(sk.Reject, result)
		case ir.StmtLoop:
			collectCallsFromBlock(sk.Body, result)
			collectCallsFromBlock(sk.Continuing, result)
		case ir.StmtSwitch:
			for j := range sk.Cases {
				collectCallsFromBlock(sk.Cases[j].Body, result)
			}
		}
	}
}

//nolint:gocognit,gocyclo,funlen,cyclop,maintidx // helper function emission with context save/restore
func (e *Emitter) emitHelperFunctions(calledFunctions map[ir.FunctionHandle]bool) error {
	for i := range e.ir.Functions {
		fn := &e.ir.Functions[i]
		handle := ir.FunctionHandle(i)

		// Only emit helper functions that are actually called by the entry point.
		if !calledFunctions[handle] {
			continue
		}

		// Check if this helper function is supportable in DXIL:
		// 1. All parameters and return type must be scalarizable (no arrays/structs/pointers)
		// 2. Return type must not be bool (i1 returns cause Invalid record in DXC)
		// 3. Function body must not access global variables (not yet correctly scalarized)
		unsupported := false
		for _, arg := range fn.Arguments {
			argIRType := e.ir.Types[arg.Type]
			if !isScalarizableType(argIRType.Inner) {
				unsupported = true
				break
			}
		}
		if fn.Result != nil {
			resultIRType := e.ir.Types[fn.Result.Type]
			if !isScalarizableType(resultIRType.Inner) {
				unsupported = true
			}
			// Bool return type (i1) causes Invalid record in DXC.
			if st, ok := resultIRType.Inner.(ir.ScalarType); ok && st.Kind == ir.ScalarBool {
				unsupported = true
			}
		}
		if !unsupported && functionAccessesGlobals(fn) {
			unsupported = true
		}
		if !unsupported && functionHasComplexLocals(fn, e.ir) {
			unsupported = true
		}
		if !unsupported && functionHasComplexExpressions(fn, e.ir) {
			unsupported = true
		}
		if unsupported {
			continue
		}

		// Determine parameter types.
		paramTypes := make([]*module.Type, 0, len(fn.Arguments))
		for _, arg := range fn.Arguments {
			argIRType := e.ir.Types[arg.Type]
			dxilTy, err := typeToDXIL(e.mod, e.ir, argIRType.Inner)
			if err != nil {
				return fmt.Errorf("helper function %q arg %q: %w", fn.Name, arg.Name, err)
			}
			// For vector args, expand to N scalar params.
			numComps := componentCount(argIRType.Inner)
			for c := 0; c < numComps; c++ {
				paramTypes = append(paramTypes, dxilTy)
			}
		}

		// Determine return type.
		// For vector returns, use a struct {scalar, scalar, ...} since DXIL
		// functions can only return one value. The caller extracts components.
		retTy := e.mod.GetVoidType()
		var retNumComps int // >1 means vector return, needs struct packing
		if fn.Result != nil {
			resultIRType := e.ir.Types[fn.Result.Type]
			scalarTy, err := typeToDXIL(e.mod, e.ir, resultIRType.Inner)
			if err != nil {
				return fmt.Errorf("helper function %q return: %w", fn.Name, err)
			}
			retNumComps = componentCount(resultIRType.Inner)
			if retNumComps > 1 {
				// Build struct type {scalar, scalar, ...} for vector return.
				fields := make([]*module.Type, retNumComps)
				for c := 0; c < retNumComps; c++ {
					fields[c] = scalarTy
				}
				retTy = e.mod.GetStructType("", fields)
			} else {
				retTy = scalarTy
			}
		}

		funcTy := e.mod.GetFunctionType(retTy, paramTypes)
		name := fn.Name
		if name == "" {
			name = fmt.Sprintf("function_%d", i)
		}
		// Create function object but DON'T add to module yet.
		// We only add it if emission succeeds, to avoid orphan declarations.
		dxilFn := &module.Function{
			Name:     name,
			FuncType: funcTy,
		}

		// Save the per-function state, emit body, then restore.
		savedNextValue := e.nextValue
		savedBB := e.currentBB
		savedMainFn := e.mainFn
		savedExprValues := e.exprValues
		savedExprComponents := e.exprComponents
		savedLocalVarPtrs := e.localVarPtrs
		savedLocalVarComponentPtrs := e.localVarComponentPtrs
		savedLocalVarStructTypes := e.localVarStructTypes
		savedLocalVarArrayTypes := e.localVarArrayTypes
		savedLoopStack := e.loopStack

		e.mainFn = dxilFn
		e.exprValues = make(map[ir.ExpressionHandle]int)
		e.exprComponents = make(map[ir.ExpressionHandle][]int)
		e.localVarPtrs = make(map[uint32]int)
		e.localVarComponentPtrs = make(map[uint32][]int)
		e.localVarStructTypes = make(map[uint32]*module.Type)
		e.localVarArrayTypes = make(map[uint32]*module.Type)
		e.loopStack = nil
		e.emittingHelperFunction = true
		e.helperReturnComps = retNumComps
		// Each helper function body gets its own value numbering starting from 0.
		// Function parameters consume IDs first, then instructions.
		e.nextValue = 0

		// Save and reset constant caches — each helper function gets its own
		// emitter value ID space, so constant cache entries from other functions
		// would map to wrong IDs.
		savedIntConsts := e.intConsts
		savedFloatConsts := e.floatConsts
		savedConstMap := e.constMap
		savedUndefID := e.undefID
		savedTypedUndefs := e.typedUndefs
		e.intConsts = make(map[int64]int)
		e.floatConsts = make(map[uint64]int)
		e.constMap = make(map[int]*module.Constant)
		e.undefID = -1
		e.typedUndefs = nil

		// Helper functions don't have their own global var allocas.
		savedGlobalVarAllocas := e.globalVarAllocas
		savedGlobalVarAllocaTypes := e.globalVarAllocaTypes
		if e.globalVarAllocas == nil {
			e.globalVarAllocas = make(map[ir.GlobalVariableHandle]int)
		}
		if e.globalVarAllocaTypes == nil {
			e.globalVarAllocaTypes = make(map[ir.GlobalVariableHandle]*module.Type)
		}

		// Save constant count so we can roll back if emission fails.
		savedConstCount := len(e.mod.Constants)

		bb := dxilFn.AddBasicBlock("entry")
		e.currentBB = bb

		// Map function arguments to value IDs.
		paramIdx := 0
		for argIdx, arg := range fn.Arguments {
			argIRType := e.ir.Types[arg.Type]
			numComps := componentCount(argIRType.Inner)

			// Find the ExprFunctionArgument handle for this argument.
			exprHandle := e.findArgExprHandle(fn, argIdx)

			if numComps == 1 {
				valueID := e.allocValue()
				e.exprValues[exprHandle] = valueID
			} else {
				comps := make([]int, numComps)
				for c := 0; c < numComps; c++ {
					comps[c] = e.allocValue()
				}
				e.exprValues[exprHandle] = comps[0]
				e.exprComponents[exprHandle] = comps
			}
			paramIdx += numComps
		}

		// Emit function body. If it fails (unsupported features), skip this
		// helper — the entry point's emitStmtCall will fall back to no-op.
		helperErr := e.emitFunctionBody(fn)
		if helperErr == nil {
			if !e.currentBBHasTerminator() {
				e.currentBB.AddInstruction(module.NewRetVoidInstr())
			}
			// Save this helper's constant map for finalizeHelperFunction.
			e.helperConstMaps[handle] = e.constMap
		}

		// Restore entry-point context including constant caches.
		e.emittingHelperFunction = false
		e.helperReturnComps = 0
		e.nextValue = savedNextValue
		e.currentBB = savedBB
		e.mainFn = savedMainFn
		e.exprValues = savedExprValues
		e.exprComponents = savedExprComponents
		e.localVarPtrs = savedLocalVarPtrs
		e.localVarComponentPtrs = savedLocalVarComponentPtrs
		e.localVarStructTypes = savedLocalVarStructTypes
		e.localVarArrayTypes = savedLocalVarArrayTypes
		e.loopStack = savedLoopStack
		e.globalVarAllocas = savedGlobalVarAllocas
		e.globalVarAllocaTypes = savedGlobalVarAllocaTypes
		e.intConsts = savedIntConsts
		e.floatConsts = savedFloatConsts
		e.constMap = savedConstMap
		e.undefID = savedUndefID
		e.typedUndefs = savedTypedUndefs

		if helperErr != nil {
			// Helper failed (unsupported features) — don't add to module.
			// Roll back any constants created during partial emission.
			e.mod.Constants = e.mod.Constants[:savedConstCount]
			// The entry point's emitStmtCall will skip the call.
			continue
		}

		// Success — add function to module and register it.
		e.mod.Functions = append(e.mod.Functions, dxilFn)
		e.helperFunctions[handle] = dxilFn
	}
	return nil
}

// currentBBHasTerminator checks if the current basic block ends with
// a terminator instruction (br or ret).
func (e *Emitter) currentBBHasTerminator() bool {
	if e.currentBB == nil || len(e.currentBB.Instructions) == 0 {
		return false
	}
	last := e.currentBB.Instructions[len(e.currentBB.Instructions)-1]
	return last.Kind == module.InstrBr || last.Kind == module.InstrRet
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
	e.localVarArrayTypes = make(map[uint32]*module.Type)
	e.globalVarAllocas = make(map[ir.GlobalVariableHandle]int)
	e.globalVarAllocaTypes = make(map[ir.GlobalVariableHandle]*module.Type)
	e.loopStack = e.loopStack[:0]
	e.meshCtx = nil

	fn := &ep.Function

	// Set up mesh shader context if this is a mesh shader entry point.
	if ep.Stage == ir.StageMesh && ep.MeshInfo != nil {
		e.meshCtx = &meshContext{
			outputVar:             ep.MeshInfo.OutputVariable,
			meshInfo:              ep.MeshInfo,
			vertexOutputSigIDs:    make(map[meshOutputKey]int),
			primitiveOutputSigIDs: make(map[meshOutputKey]int),
			pendingVertexCount:    -1,
			pendingPrimitiveCount: -1,
			taskPayloadVar:        ep.TaskPayload,
		}
		e.buildMeshSignatureMapping(ep)
	}

	// Analyze and create resource bindings before function body emission.
	e.analyzeResources()

	// Emit I/O: load inputs into value map, then emit function body,
	// then store outputs. At this point we use temporary IDs starting
	// from 0. These will be adjusted in finalize().
	e.nextValue = 0

	e.emitResourceHandles()

	// For mesh shaders, input loads use compute-style builtin intrinsics
	// (threadIdInGroup, flattenedThreadIdInGroup, etc.).
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

	// Instruction results: process the entry point function only.
	// Helper functions are finalized separately in finalizeHelperFunction().
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
		_ = handle
		for i, id := range comps {
			if newID, ok := idMap[id]; ok {
				comps[i] = newID
			}
		}
	}
}

// finalizeHelperFunction remaps value IDs for a helper function body.
// Must be called AFTER finalize(mainFn) so that constants have their
// final ValueIDs assigned.
//
// Each helper function's instruction IDs start from globalValueCount()
// (same base as the entry point), because the serializer processes
// each function body independently with the same starting offset.
//
// The helperConstMap parameter is the per-helper constant mapping
// (emitter value ID -> *module.Constant) saved during helper emission.
//
//nolint:gocognit // value ID remapping across basic blocks
func (e *Emitter) finalizeHelperFunction(fn *module.Function, helperConstMap map[int]*module.Constant) {
	base := len(e.mod.GlobalVars) + len(e.mod.Functions) + len(e.mod.Constants)

	funcIDMap := make(map[int]int)

	// Map constants: emitter temp ID -> final constant ValueID.
	// Constants were finalized by the entry point's finalize() call,
	// so c.ValueID is already correct.
	for emitterID, c := range helperConstMap {
		funcIDMap[emitterID] = c.ValueID
	}

	// Function parameters consume value IDs after the base.
	nextID := base
	if fn.FuncType != nil {
		for i := range fn.FuncType.ParamTypes {
			funcIDMap[i] = nextID
			nextID++
		}
	}

	// Map instruction result IDs.
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.HasValue {
				oldID := instr.ValueID
				funcIDMap[oldID] = nextID
				instr.ValueID = nextID
				nextID++
			}
		}
	}

	// Rewrite operand references.
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			opIndices := valueOperandIndices(instr)
			for _, idx := range opIndices {
				if idx < len(instr.Operands) {
					if newID, ok := funcIDMap[instr.Operands[idx]]; ok {
						instr.Operands[idx] = newID
					}
				}
			}
			if instr.Kind == module.InstrRet && instr.ReturnValue >= 0 {
				if newID, ok := funcIDMap[instr.ReturnValue]; ok {
					instr.ReturnValue = newID
				}
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
//
//nolint:cyclop // one case per instruction kind — simple dispatch, not real complexity
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
	case module.InstrInsertVal:
		return []int{0, 1} // [aggregate, value, index]
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
	case module.InstrAtomicRMW:
		// AtomicRMW: [ptrValueID, valueID, atomicOp, isVolatile, ordering, synchscope]
		// Only operands 0 (ptr) and 1 (value) are value references.
		return []int{0, 1}
	case module.InstrCmpXchg:
		// CmpXchg: [ptrValueID, cmpValueID, newValueID, isVolatile, ordering, synchscope]
		// Only operands 0 (ptr), 1 (cmp), and 2 (new) are value references.
		return []int{0, 1, 2}
	default:
		// Phi, Switch, etc.: not yet used. Return all as safe fallback.
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

	// dx.viewIdState — required for mesh shaders.
	// Format: !{[3 x i32] [i32 0, i32 numOutputComps, i32 numPrimComps]}
	// numOutputComps = 4 * numOutputElements, numPrimComps = 4 * numPrimElements
	if kind == module.MeshShader {
		e.emitViewIDState(ep)
	}

	// Emit resource metadata if we have any resources.
	mdResources := e.emitResourceMetadata()

	// Build shader properties (tag-value pairs) for the entry point.
	var mdProperties *module.MetadataNode
	switch kind {
	case module.ComputeShader:
		mdProperties = e.emitComputeProperties(ep)
	case module.MeshShader:
		mdProperties = e.emitMeshProperties(ep)
	}

	// Build entry point signatures metadata (needed for mesh shaders).
	var mdSignatures *module.MetadataNode
	if kind == module.MeshShader {
		mdSignatures = e.emitMeshSignatureMetadata(ep)
	}

	// dx.entryPoints = !{!{null, !"main", !signatures, !resources, !properties}}
	mdName := e.mod.AddMetadataString("main")
	mdEntry := e.mod.AddMetadataTuple([]*module.MetadataNode{
		nil,          // function ref (simplified)
		mdName,       // entry point name
		mdSignatures, // signatures (nil for non-mesh)
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
	x := max(ep.Workgroup[0], 1)
	y := max(ep.Workgroup[1], 1)
	z := max(ep.Workgroup[2], 1)

	mdX := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(x)))
	mdY := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(y)))
	mdZ := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(z)))
	mdThreads := e.mod.AddMetadataTuple([]*module.MetadataNode{mdX, mdY, mdZ})

	// Properties = !{tag, value, ...}
	return e.mod.AddMetadataTuple([]*module.MetadataNode{mdTag, mdThreads})
}

// emitMeshProperties builds the shader properties metadata for a mesh shader
// entry point. Contains ONLY kDxilMSStateTag (tag 9) with numthreads nested inside.
//
// DXC reference format:
//
//	!{i32 9, !{!{i32 X, i32 Y, i32 Z}, i32 maxVert, i32 maxPrim, i32 topo, i32 payloadSize}}
//
// kDxilMSStateTag = 9. Numthreads is a nested tuple inside the mesh state.
// outputTopology: 1=line, 2=triangle
func (e *Emitter) emitMeshProperties(ep *ir.EntryPoint) *module.MetadataNode {
	if ep.MeshInfo == nil {
		return nil
	}

	i32Ty := e.mod.GetIntType(32)
	mi := ep.MeshInfo

	mdMSTag := e.mod.AddMetadataValue(i32Ty, e.getIntConst(9))

	x := max(ep.Workgroup[0], 1)
	y := max(ep.Workgroup[1], 1)
	z := max(ep.Workgroup[2], 1)

	// Numthreads as nested tuple.
	mdX := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(x)))
	mdY := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(y)))
	mdZ := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(z)))
	mdThreads := e.mod.AddMetadataTuple([]*module.MetadataNode{mdX, mdY, mdZ})

	maxVert := int64(mi.MaxVertices)
	maxPrim := int64(mi.MaxPrimitives)
	topo := meshTopologyToDXIL(mi.Topology)

	// Payload size: compute from task payload type if available.
	payloadSize := int64(0)
	if ep.TaskPayload != nil {
		payloadSize = e.computeTypeSize(e.ir.GlobalVariables[*ep.TaskPayload].Type)
	}

	mdMaxVert := e.mod.AddMetadataValue(i32Ty, e.getIntConst(maxVert))
	mdMaxPrim := e.mod.AddMetadataValue(i32Ty, e.getIntConst(maxPrim))
	mdTopo := e.mod.AddMetadataValue(i32Ty, e.getIntConst(topo))
	mdPayload := e.mod.AddMetadataValue(i32Ty, e.getIntConst(payloadSize))
	mdMSState := e.mod.AddMetadataTuple([]*module.MetadataNode{
		mdThreads, mdMaxVert, mdMaxPrim, mdTopo, mdPayload,
	})

	return e.mod.AddMetadataTuple([]*module.MetadataNode{mdMSTag, mdMSState})
}

// emitViewIDState emits the dx.viewIdState metadata for mesh shaders.
// Format: dx.viewIdState = !{[3 x i32] [i32 0, i32 numOutputComps, i32 numPrimComps]}
//
// numOutputComps = 4 * number of vertex output signature elements
// numPrimComps = 4 * number of primitive output signature elements (excluding indices)
func (e *Emitter) emitViewIDState(ep *ir.EntryPoint) {
	numOutputComps := int64(0)
	numPrimComps := int64(0)

	if ep.MeshInfo != nil {
		numOutputComps = e.countMeshOutputComps(ep.MeshInfo.VertexOutputType, false)
		numPrimComps = e.countMeshOutputComps(ep.MeshInfo.PrimitiveOutputType, true)
	}

	i32Ty := e.mod.GetIntType(32)
	arrTy := e.mod.GetArrayType(i32Ty, 3)

	c0 := e.getIntConst(0)
	c1 := e.getIntConst(numOutputComps)
	c2 := e.getIntConst(numPrimComps)

	aggConst := e.mod.AddAggregateConst(arrTy, []*module.Constant{c0, c1, c2})
	aggID := e.allocValue()
	e.constMap[aggID] = aggConst

	mdViewID := e.mod.AddMetadataValue(arrTy, aggConst)
	mdViewIDTuple := e.mod.AddMetadataTuple([]*module.MetadataNode{mdViewID})
	e.mod.AddNamedMetadata("dx.viewIdState", []*module.MetadataNode{mdViewIDTuple})
}

// countMeshOutputComps counts the total components in a mesh output struct type.
// For primitive outputs, index builtins (triangle_indices, line_indices, point_index) are excluded.
func (e *Emitter) countMeshOutputComps(typeHandle ir.TypeHandle, isPrimitive bool) int64 {
	if int(typeHandle) >= len(e.ir.Types) {
		return 0
	}
	st, ok := e.ir.Types[typeHandle].Inner.(ir.StructType)
	if !ok {
		return 0
	}
	total := int64(0)
	for _, member := range st.Members {
		if member.Binding == nil {
			continue
		}
		if isPrimitive {
			if bb, isBB := (*member.Binding).(ir.BuiltinBinding); isBB {
				if bb.Builtin == ir.BuiltinTriangleIndices ||
					bb.Builtin == ir.BuiltinLineIndices ||
					bb.Builtin == ir.BuiltinPointIndex {
					continue
				}
			}
		}
		total += e.memberComponentCount(member.Type)
	}
	return total
}

// memberComponentCount returns the number of components in a type (4 for vec4, 1 for scalar).
func (e *Emitter) memberComponentCount(th ir.TypeHandle) int64 {
	if int(th) >= len(e.ir.Types) {
		return 4
	}
	irType := e.ir.Types[th]
	switch ti := irType.Inner.(type) {
	case ir.VectorType:
		return int64(ti.Size)
	case ir.ScalarType:
		return 1
	default:
		return 4
	}
}

// emitMeshSignatureMetadata builds the signature metadata triplet for mesh shader
// entry points. DXC format:
//
//	!{null, !outputSigs, null}
//
// Where outputSigs contains per-element metadata nodes describing each vertex output.
// Each element: !{i32 sigID, !"SemanticName", i8 compType, i8 semKind, !{i32 semIndex}, i8 interpMode, i32 rows, i8 cols, i32 startRow, i8 startCol, !outputDeps}
//
//nolint:gocognit,nestif // signature metadata requires deep type inspection
func (e *Emitter) emitMeshSignatureMetadata(ep *ir.EntryPoint) *module.MetadataNode {
	if ep.MeshInfo == nil {
		return nil
	}

	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	var outputElements []*module.MetadataNode

	if int(ep.MeshInfo.VertexOutputType) < len(e.ir.Types) {
		vtxType := e.ir.Types[ep.MeshInfo.VertexOutputType]
		if st, ok := vtxType.Inner.(ir.StructType); ok {
			sigID := 0
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}

				// Determine semantic name and kind.
				semName := "TEXCOORD"
				semKind := int64(0) // arbitrary
				semIndex := int64(0)
				interpMode := int64(0) // undefined
				compType := int64(9)   // f32 = 9 in DXIL metadata

				if bb, isBB := (*member.Binding).(ir.BuiltinBinding); isBB {
					switch bb.Builtin {
					case ir.BuiltinPosition:
						semName = "SV_Position"
						semKind = 3    // SV_Position
						interpMode = 4 // noperspective
					}
				} else if lb, isLB := (*member.Binding).(ir.LocationBinding); isLB {
					semIndex = int64(lb.Location)
				}

				// Component type in metadata.
				cols := int64(4) // default vec4
				if int(member.Type) < len(e.ir.Types) {
					memberType := e.ir.Types[member.Type]
					switch ti := memberType.Inner.(type) {
					case ir.VectorType:
						cols = int64(ti.Size)
						switch ti.Scalar.Kind {
						case ir.ScalarUint:
							compType = 5 // u32
						case ir.ScalarSint:
							compType = 4 // i32
						}
					case ir.ScalarType:
						cols = 1
						switch ti.Kind {
						case ir.ScalarUint:
							compType = 5
						case ir.ScalarSint:
							compType = 4
						}
					}
				}

				// Build semantic index tuple.
				mdSemIdx := e.mod.AddMetadataTuple([]*module.MetadataNode{
					e.mod.AddMetadataValue(i32Ty, e.getIntConst(semIndex)),
				})

				// Element metadata: {sigID, "name", compType, semKind, semIndices, interpMode, rows, cols, startRow, startCol, deps_or_null}
				// DXC uses !{i32 depMask, i32 compMask} for viewId tracking.
				// null is used when there are no viewId dependencies.
				mdElem := e.mod.AddMetadataTuple([]*module.MetadataNode{
					e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(sigID))),
					e.mod.AddMetadataString(semName),
					e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(compType)),
					e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(semKind)),
					mdSemIdx,
					e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(interpMode)),
					e.mod.AddMetadataValue(i32Ty, e.getIntConst(1)),          // rows
					e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(cols)), // cols
					e.mod.AddMetadataValue(i32Ty, e.getIntConst(0)),          // startRow
					e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(0)),    // startCol
					nil, // output dependencies (null = no viewId dependency)
				})

				outputElements = append(outputElements, mdElem)
				sigID++
			}
		}
	}

	var mdOutputSigs *module.MetadataNode
	if len(outputElements) > 0 {
		mdOutputSigs = e.mod.AddMetadataTuple(outputElements)
	}

	// Signatures triplet: {inputSigs, outputSigs, primitiveSigs}
	return e.mod.AddMetadataTuple([]*module.MetadataNode{nil, mdOutputSigs, nil})
}

// meshTopologyToDXIL maps naga mesh output topology to DXIL output topology.
// DXIL: 0=undefined, 1=line, 2=triangle
func meshTopologyToDXIL(t ir.MeshOutputTopology) int64 {
	switch t {
	case ir.MeshTopologyLines:
		return 1
	case ir.MeshTopologyTriangles:
		return 2
	default:
		return 0
	}
}

// computeTypeSize estimates the byte size of a type for payload size metadata.
func (e *Emitter) computeTypeSize(th ir.TypeHandle) int64 {
	if int(th) >= len(e.ir.Types) {
		return 0
	}
	irType := e.ir.Types[th]
	return computeTypeSizeInner(e.ir, irType.Inner)
}

// computeTypeSizeInner estimates the byte size of a type inner.
func computeTypeSizeInner(irMod *ir.Module, inner ir.TypeInner) int64 {
	switch ti := inner.(type) {
	case ir.ScalarType:
		return int64(ti.Width)
	case ir.VectorType:
		return int64(ti.Size) * int64(ti.Scalar.Width)
	case ir.MatrixType:
		return int64(ti.Columns) * int64(ti.Rows) * int64(ti.Scalar.Width)
	case ir.ArrayType:
		if ti.Size.Constant == nil {
			return 0
		}
		baseSize := computeTypeSizeInner(irMod, irMod.Types[ti.Base].Inner)
		return int64(*ti.Size.Constant) * baseSize
	case ir.StructType:
		total := int64(0)
		for _, m := range ti.Members {
			total += computeTypeSizeInner(irMod, irMod.Types[m.Type].Inner)
		}
		return total
	default:
		return 0
	}
}

// buildMeshSignatureMapping analyzes the mesh shader output types and assigns
// signature element IDs for vertex and primitive outputs.
//
//nolint:nestif // mesh signature requires nested type checks
func (e *Emitter) buildMeshSignatureMapping(ep *ir.EntryPoint) {
	if e.meshCtx == nil || ep.MeshInfo == nil {
		return
	}

	// Analyze vertex output type.
	if int(ep.MeshInfo.VertexOutputType) < len(e.ir.Types) {
		vtxType := e.ir.Types[ep.MeshInfo.VertexOutputType]
		if st, ok := vtxType.Inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				key := bindingToMeshOutputKey(*member.Binding)
				e.meshCtx.vertexOutputSigIDs[key] = e.meshCtx.nextVertexSigID
				e.meshCtx.nextVertexSigID++
			}
		}
	}

	// Analyze primitive output type.
	if int(ep.MeshInfo.PrimitiveOutputType) < len(e.ir.Types) {
		primType := e.ir.Types[ep.MeshInfo.PrimitiveOutputType]
		if st, ok := primType.Inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				key := bindingToMeshOutputKey(*member.Binding)
				// Skip triangle_indices (handled by EmitIndices, not StorePrimitiveOutput).
				if key.isBuiltin && key.builtin == ir.BuiltinTriangleIndices {
					continue
				}
				e.meshCtx.primitiveOutputSigIDs[key] = e.meshCtx.nextPrimitiveSigID
				e.meshCtx.nextPrimitiveSigID++
			}
		}
	}
}

// bindingToMeshOutputKey converts a binding to a mesh output key for signature lookup.
func bindingToMeshOutputKey(b ir.Binding) meshOutputKey {
	switch bb := b.(type) {
	case ir.BuiltinBinding:
		return meshOutputKey{isBuiltin: true, builtin: bb.Builtin}
	case ir.LocationBinding:
		return meshOutputKey{isBuiltin: false, location: bb.Location}
	default:
		return meshOutputKey{}
	}
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

// getTypedUndefConstID returns the emitter value ID for an undef constant
// of the specified type. Used for bufferStore value channels where the undef
// must match the function's parameter type (e.g., undef f32 for float stores).
func (e *Emitter) getTypedUndefConstID(ty *module.Type) int {
	// For i32, reuse the standard undef.
	if ty.Kind == module.TypeInteger && ty.IntBits == 32 {
		return e.getUndefConstID()
	}
	// Check cache.
	if e.typedUndefs == nil {
		e.typedUndefs = make(map[int]int) // type ID -> emitter value ID
	}
	if id, ok := e.typedUndefs[ty.ID]; ok {
		return id
	}
	c := &module.Constant{
		ConstType: ty,
		IsUndef:   true,
	}
	e.mod.Constants = append(e.mod.Constants, c)
	id := e.allocValue()
	e.typedUndefs[ty.ID] = id
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

// getI8MetadataConst creates an i8 constant for metadata signature elements.
// These constants must be tracked in constMap so that finalize correctly
// numbers all constants when computing instruction value deltas.
func (e *Emitter) getI8MetadataConst(v int64) *module.Constant {
	c := e.mod.AddIntConst(e.mod.GetIntType(8), v)
	id := e.allocValue()
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
