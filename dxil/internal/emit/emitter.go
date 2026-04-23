package emit

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/dxil/internal/viewid"
	"github.com/gogpu/naga/internal/backend"
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

	// exprUseCount tracks how many times each expression handle is
	// referenced as an operand of another expression. Computed once
	// per function by computeExprUseCount. Used by the LLVM-style
	// Reassociate pass to identify single-use chain intermediates.
	exprUseCount map[ir.ExpressionHandle]int

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

	// localConstVecArrays stores constant-folded per-lane tables for
	// local vars of type `array<vecN<f32>, M>` whose init is an
	// ExprCompose of vector literals. See tryRegisterLocalConstVecArray.
	// Maps local var idx → table[row][lane]. Non-nil entries bypass the
	// alloca+store path entirely — access code (getLocalConstVecLane)
	// synthesizes per-lane selects or constant lookups.
	localConstVecArrays map[uint32][][]float32

	// initOnlyLocals tracks non-scalar locals (vector, array, struct)
	// that have a non-nil Init expression and are never stored to in the
	// function body. For such locals, Load can resolve directly to the
	// Init value without emitting alloca+store+load. This implements a
	// targeted form of SROA: DXC's LLVM SROA + constant propagation
	// achieves the same elimination for `var x = const_expr; return x`.
	// Maps local variable index -> Init expression handle.
	initOnlyLocals map[uint32]ir.ExpressionHandle

	// currentFn is the IR function whose body is currently being
	// emitted. Set at the top of each entry-point/helper emit loop.
	// Read by localConstVecArrays helpers to walk function-local
	// expression handles (const folding needs access to the expr arena
	// without threading fn through every helper call).
	currentFn *ir.Function

	// Maps IR global variable handle to its alloca pointer value ID.
	// Used for non-resource globals (workgroup, private) that need pointer semantics.
	globalVarAllocas map[ir.GlobalVariableHandle]int

	// Cached DXIL types for global variable allocas.
	// Used by emitAccess to emit GEP into array/struct allocas.
	globalVarAllocaTypes map[ir.GlobalVariableHandle]*module.Type

	// globalVarModuleVars maps an emitter value ID (allocated for a
	// workgroup/groupshared variable) to the *module.GlobalVar registered
	// in m.GlobalVars. finalize() reads this map to remap the emitter ID
	// to the global var's bitcode ValueID after assignIDs runs. The
	// global var's bitcode value type is implicitly an addrspace(N)
	// pointer to its element type, so subsequent instruction operands
	// referencing the remapped ID get the right addrspace.
	globalVarModuleVars map[int]*module.GlobalVar

	// workgroupFieldPtrs tracks pointers obtained via a single-step
	// struct-field GEP off a workgroup addrspace(3) global. Keyed by the
	// emitter value ID of the field pointer, holds the info needed to
	// re-root any derived element GEP directly at the global so the
	// DXIL validator's TGSM-origin analysis can trace the pointer back
	// to an unambiguous global variable. Without this, `GEP-of-GEP`
	// chains off a workgroup global are rejected with
	// 'TGSM pointers must originate from an unambiguous TGSM global
	// variable'.
	workgroupFieldPtrs map[int]workgroupFieldOrigin

	// workgroupElemPtrs tracks pointers obtained via a single-step array
	// element GEP off a workgroup addrspace(3) global whose base type is
	// array<T>. Keyed by the emitter value ID of the element pointer, holds
	// the info needed for decomposed struct/array field loads/stores to
	// re-root each per-field GEP directly at the global with a 3-index
	// path [0, elemIdx, fieldN]. Same TGSM-origin rationale as
	// workgroupFieldPtrs but for arr<T>-rooted workgroup globals instead
	// of struct-rooted ones.
	workgroupElemPtrs map[int]workgroupElemOrigin

	// Loop context stack for break/continue targets.
	loopStack []loopContext

	// lastBranchBBs holds the BB indices recorded by the most-recently
	// completed StmtIf / StmtSwitch / (future) StmtLoop emit. Used by
	// emitExpression for ExprPhi to map each PhiIncoming.PredKey to a
	// concrete LLVM BB index when materializing FUNC_CODE_INST_PHI.
	// Cleared when emit hits any non-phi-bearing statement so a stale
	// snapshot from a prior construct cannot accidentally feed into a
	// later phi.
	//
	// Reference parity: LLVM mem2reg emits phi instructions at the
	// start of the merge BB after both branches' final instructions
	// have been emitted; the BB-of-incoming-value is exactly the
	// branch's terminator BB. We track that here.
	lastBranchBBs *branchBBs

	// dx.op function declarations (lazily created).
	dxOpFuncs map[dxOpKey]*module.Function

	// Constant cache: maps from constant key to emitter value ID.
	intConsts   map[int64]int  // i32 const value -> emitter value ID
	int64Consts map[int64]int  // i64 const value -> emitter value ID
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
	resources        []resourceInfo
	resourceHandles  map[ir.GlobalVariableHandle]int  // global var handle -> index in resources
	reachableGlobals map[ir.GlobalVariableHandle]bool // BUG-DXIL-016: globals transitively used by current entry point

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

	// inInlineExpansion is true when emitStmtCallInline is walking a callee
	// body inline inside the caller's basic block. Affects StmtReturn: instead
	// of emitting a ret instruction, the return value is captured into
	// inlineReturnValue / inlineReturnComponents so the caller's call.Result
	// can consume it. Matches DXC's AlwaysInliner semantics
	// (lib/HLSL/DxilLinker.cpp:1248) — DXIL forbids struct insertvalue/
	// extractvalue, so helper functions with aggregate args/returns CANNOT
	// exist as standalone LLVM functions and must be inlined at every call.
	inInlineExpansion      bool
	inlineReturnValue      int
	inlineReturnComponents []int
	inlineReturnCaptured   bool

	// Mesh shader context: set when emitting a mesh shader entry point.
	meshCtx *meshContext

	// Synthetic CBV handle for NumWorkGroups builtin (-1 if not created).
	numWGHandleID int

	// Wave ballot return type (struct {i32, i32, i32, i32}), lazily created.
	waveBallotRetTy *module.Type

	// Ray query handle map: expression handle → DXIL value ID for allocated ray query.
	rayQueryHandles map[ir.ExpressionHandle]int

	// currentEntryPoint is the entry point being emitted, used to scope
	// shader-flag computation to the reachable call graph only. Mirrors
	// DXC's ShaderFlags::CollectShaderFlags(Function*, ...) which is a
	// per-function (not per-module) operation — merging happens later via
	// AdjustMinimumShaderModelAndFlags after walking called functions.
	// See DxilShaderFlags.cpp:393. Without per-EP scoping, sibling EPs
	// in the same module (e.g. ray-query.wgsl's main + main_candidate)
	// leak each other's feature flags and trip 'Flags must match usage'.
	currentEntryPoint *ir.EntryPoint

	// currentReachableFunctions caches the set of ir.FunctionHandle
	// transitively called from currentEntryPoint.Function. Shared between
	// helper function emission (skip non-reachable helpers) and flag
	// computation (only consider reachable functions).
	currentReachableFunctions map[ir.FunctionHandle]bool

	// finalShaderFlags holds the last computed bitcode ShaderFlags value
	// (produced by computeBitcodeShaderFlags during entry-point metadata
	// emission). Emit() returns it to the caller so dxil.Compile can
	// regenerate a matching SFI0 container part — DXC's validator
	// memcmp's SFI0 against what it reconstructs from the bitcode
	// ShaderFlags, so the container and bitcode must agree exactly.
	finalShaderFlags uint64

	// samplerHeap tracks per-WGSL-sampler info needed to materialize
	// each sampler handle through the heap-indirection pattern shared
	// with the HLSL backend. Populated by analyzeResources when any
	// global is a SamplerType. nil → no samplers, fall back to the
	// pre-existing direct-binding path. See sampler_heap.go for the
	// model and BUG-DXIL-035 for the cross-backend rationale.
	samplerHeap *samplerHeapState

	// inputUsedMasks records per-input-signature-element the component
	// "used" bitmask derived from ViewID dependency analysis. A zero mask
	// means the input is never read by any output-contributing code path
	// and its metadata extended-properties field should be null (matching
	// DXC behavior). Non-zero = emit !{i32 3, i32 mask}. Populated by
	// emitGraphicsViewIDState, consumed by buildSignatureElementMD.
	// Indexed by input signature element ordinal in collectGraphicsSignatures
	// order. nil when no ViewID analysis has been performed (non-graphics
	// stages).
	inputUsedMasks []int64
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

// branchKind tags the kind of structured construct that produced the
// most recent BB snapshot, so phi materialization can pick the right
// PhiPredKey -> BBIndex mapping.
type branchKind uint8

const (
	branchKindIf branchKind = iota
	branchKindSwitch
)

// branchBBs holds the BB indices a just-completed structured construct
// produced, in the form needed to map each PhiIncoming.PredKey to a
// concrete LLVM BB index for FUNC_CODE_INST_PHI emission.
//
// Populated by emitIfStatement / emitSwitchStatement immediately
// before transitioning currentBB to the merge BB. Consumed by
// emitExpression for ExprPhi. Cleared on any non-phi statement.
type branchBBs struct {
	kind branchKind

	// Populated when kind == branchKindIf:
	//   acceptEndBB: BB at end of accept-body (where the br to merge lives)
	//   rejectEndBB: BB at end of reject-body (or acceptEndBB when no else)
	acceptEndBB int
	rejectEndBB int
	hasReject   bool

	// Populated when kind == branchKindSwitch:
	//   caseEndBBs[i] = BB at end of case i's body (where its terminator lives)
	caseEndBBs []int
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

// BindingLocation identifies a resource in the source shader.
// Mirror of dxil.BindingLocation kept in this internal package so that
// emit does not have to import the public dxil package (which would
// create an import cycle since dxil imports emit).
type BindingLocation struct {
	Group   uint32
	Binding uint32
}

// BindTarget specifies the DXIL register binding for a resource.
// Mirror of dxil.BindTarget.
type BindTarget struct {
	Space            uint32
	Register         uint32
	BindingArraySize *uint32
}

// BindingMap maps source-shader binding locations to DXIL register
// bindings. Mirror of dxil.BindingMap.
type BindingMap map[BindingLocation]BindTarget

// SamplerHeapBindTargets specifies binding targets for synthesized
// sampler heap arrays. Mirror of dxil.SamplerHeapBindTargets.
type SamplerHeapBindTargets struct {
	StandardSamplers   BindTarget
	ComparisonSamplers BindTarget
}

// EmitOptions configures DXIL emission.
type EmitOptions struct {
	ShaderModelMajor uint32
	ShaderModelMinor uint32

	// BindingMap remaps (group, binding) to (space, register) at emit
	// time. If nil or a binding is not present, raw WGSL numbers are
	// used. See dxil.Options.BindingMap for rationale.
	BindingMap BindingMap

	// SamplerHeapTargets overrides the (space, register) for synthesized
	// sampler heap arrays. When nil the emitter uses the defaults:
	// standard at (space=0, register=0), comparison at (space=1, register=0).
	SamplerHeapTargets *SamplerHeapBindTargets

	// SamplerBufferBindingMap overrides the (space, register) for per-group
	// sampler index buffer SRVs. Key is the bind group number. When nil
	// the emitter uses the defaults: (space=255, register=group).
	SamplerBufferBindingMap map[uint32]BindTarget

	// ReachableGlobals is the set of GlobalVariableHandles transitively
	// referenced by the entry point being compiled. analyzeResources
	// skips globals not in this set so multi-EP modules don't leak
	// unrelated bindings into the per-EP DXIL container. nil means
	// "include every global" (single-EP modules / tests). BUG-DXIL-016.
	ReachableGlobals map[ir.GlobalVariableHandle]bool

	// InputUsedMasks holds per-input-argument component usage masks
	// computed from IR body analysis. Used by the metadata builder to
	// emit null extended-properties for unused inputs (matching DXC's
	// MarkUsedSignatureElements). Key is InputUsageKey{argIdx, memberIdx},
	// value is a bitmask of used components (0 = unused). nil means
	// "assume all inputs used" (conservative).
	InputUsedMasks map[InputUsageKey]uint8
}

// InputUsageKey identifies a signature element by its argument and member.
type InputUsageKey struct {
	ArgIdx    int
	MemberIdx int // -1 for non-struct direct arg access
}

// Emit translates a naga IR module into a DXIL module.Module.
//
// The caller is responsible for serializing the result to bitcode
// and wrapping it in a DXBC container.
func Emit(irMod *ir.Module, opts EmitOptions) (*module.Module, error) {
	mod, _, err := EmitWithFlags(irMod, opts)
	return mod, err
}

// EmitWithFlags is like Emit but also returns the bitcode ShaderFlags
// value computed for the (single) entry point. Callers that build a
// DXBC container need this to regenerate a matching SFI0 (Feature Info)
// part — DXC's validator memcmp's that part against what it reconstructs
// from the bitcode ShaderFlags.
func EmitWithFlags(irMod *ir.Module, opts EmitOptions) (*module.Module, uint64, error) {
	if len(irMod.EntryPoints) == 0 {
		return nil, 0, fmt.Errorf("dxil/emit: module has no entry points")
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
		numWGHandleID:        -1,
		constMap:             make(map[int]*module.Constant),
		resourceHandles:      make(map[ir.GlobalVariableHandle]int),
		reachableGlobals:     opts.ReachableGlobals,
		callResultValues:     make(map[ir.ExpressionHandle]int),
		callResultComponents: make(map[ir.ExpressionHandle][]int),
		helperFunctions:      make(map[ir.FunctionHandle]*module.Function),
		helperConstMaps:      make(map[ir.FunctionHandle]map[int]*module.Constant),
	}

	// Pre-scan the entry point to find which helper functions are actually
	// called. Store on the Emitter so flag computation (computeBitcodeShaderFlags)
	// can scope ray-query / int64 / low-precision detection to the reachable
	// call graph, mirroring DXC's per-function ShaderFlags::CollectShaderFlags.
	e.currentEntryPoint = ep
	e.currentReachableFunctions = collectCalledFunctions(&ep.Function, e.ir.Functions)
	if err := e.emitHelperFunctions(e.currentReachableFunctions); err != nil {
		return nil, 0, fmt.Errorf("dxil/emit: helper functions: %w", err)
	}

	if err := e.emitEntryPoint(ep); err != nil {
		return nil, 0, fmt.Errorf("dxil/emit: entry point %q: %w", ep.Name, err)
	}

	// Finalize helper functions AFTER the entry point finalize
	// (which assigns final constant ValueIDs).
	for handle, helperFn := range e.helperFunctions {
		helperConstMap := e.helperConstMaps[handle]
		e.finalizeHelperFunction(helperFn, helperConstMap)
	}

	return mod, e.finalShaderFlags, nil
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
//
// The scan is transitive: if helper A calls helper B, both are collected.
func collectCalledFunctions(fn *ir.Function, allFunctions []ir.Function) map[ir.FunctionHandle]bool {
	result := make(map[ir.FunctionHandle]bool)
	collectCallsFromBlock(fn.Body, result)

	// Transitively collect helpers called by already-collected helpers.
	// Use a worklist approach to handle arbitrary call depth.
	changed := true
	for changed {
		changed = false
		for handle := range result {
			if int(handle) < len(allFunctions) {
				helperFn := &allFunctions[handle]
				before := len(result)
				collectCallsFromBlock(helperFn.Body, result)
				if len(result) > before {
					changed = true
				}
			}
		}
	}

	return result
}

// FunctionHasComplexLocals reports whether any local variable of fn has
// a type our standalone-helper emission cannot scalarize. Exported so
// dxil.Compile can mirror the same eligibility gate inside the IR inline
// policy (BUG-DXIL-029).
func FunctionHasComplexLocals(fn *ir.Function, irMod *ir.Module) bool {
	return functionHasComplexLocals(fn, irMod)
}

// FunctionHasComplexExpressions reports whether the function body uses
// expression kinds that cannot be emitted inside a standalone LLVM
// helper function. Exported for the inline policy mirror.
func FunctionHasComplexExpressions(fn *ir.Function, irMod *ir.Module) bool {
	return functionHasComplexExpressions(fn, irMod)
}

// FunctionAccessesGlobals reports whether the function reads any module
// global variable. Exported for the inline policy mirror.
func FunctionAccessesGlobals(fn *ir.Function) bool {
	return functionAccessesGlobals(fn)
}

// IsScalarizableType reports whether the IR type can be safely expanded
// into its scalar components for helper-function argument/return
// scalarization. Exported for the inline policy mirror.
func IsScalarizableType(inner ir.TypeInner) bool {
	return isScalarizableType(inner)
}

// ComponentCount returns the flat scalar component count of the given
// IR type inner. Exported for the inline policy mirror.
func ComponentCount(inner ir.TypeInner) int {
	return componentCount(inner)
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

		// Check if this helper function is supportable as a standalone
		// LLVM function in DXIL:
		//  1. All parameters and return types must be scalarizable
		//  2. Return type must not be bool (i1 returns cause Invalid record in DXC)
		//  3. Return type must not be a vector/aggregate: DXIL strictly forbids
		//     insertvalue/extractvalue on struct-typed values, so packing a
		//     vector return into {f32,f32,f32} produces an unvalidatable
		//     bitcode module. Vector-returning helpers are routed through
		//     emit-time inline expansion instead (see inline.go).
		//  4. Function body must not access global variables
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
			// Vector / aggregate return: always inline to avoid invalid
			// insertvalue/extractvalue on struct types. Matches DXC's
			// AlwaysInliner behavior (lib/HLSL/DxilLinker.cpp:1248).
			if componentCount(resultIRType.Inner) > 1 {
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
			e.emitHelperFallthroughTerminator(fn, retTy, retNumComps)
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

// emitHelperFallthroughTerminator closes out a helper function body when the
// current basic block still lacks a terminator. This happens for empty,
// unreachable merge blocks left behind by codegen of switch statements whose
// arms all return — LLVM still requires every block to have a terminator.
//
// The operand type of the emitted `ret` must match the function signature:
//   - `ret void` inside a non-void function is rejected by LLVM verifier
//     ("Function return type does not match operand type of return inst!").
//   - `ret <undef>` is rejected by the DXIL validator ("Instructions should
//     not read uninitialized value").
//
// A zero constant of the declared return type satisfies both. This mirrors
// what DXC's SimplifyCFG + CleanupInsts pipeline produces for the equivalent
// HLSL source.
//
// For vector returns packed into `{scalar, scalar, ...}` structs, the helper
// is already broken elsewhere by DXIL's insertvalue/extractvalue restriction;
// `ret void` is kept as a placeholder there — the blob fails earlier on those
// instructions first.
func (e *Emitter) emitHelperFallthroughTerminator(fn *ir.Function, retTy *module.Type, retNumComps int) {
	if e.currentBBHasTerminator() {
		return
	}
	if retTy == e.mod.GetVoidType() {
		e.currentBB.AddInstruction(module.NewRetVoidInstr())
		return
	}
	if fn.Result == nil || retNumComps > 1 {
		e.currentBB.AddInstruction(module.NewRetVoidInstr())
		return
	}
	resultIRType := e.ir.Types[fn.Result.Type]
	zeroID := e.getZeroForType(resultIRType.Inner)
	e.currentBB.AddInstruction(&module.Instruction{
		Kind:        module.InstrRet,
		HasValue:    false,
		ReturnValue: zeroID,
	})
}

// emitEntryPoint emits a single entry point function.
func (e *Emitter) emitEntryPoint(ep *ir.EntryPoint) error {
	voidTy := e.mod.GetVoidType()
	funcTy := e.mod.GetFunctionType(voidTy, nil)

	// The LLVM function symbol must match the entry-point name that's
	// also written into dx.entryPoints[0][1] and PSV0's EntryFunctionName
	// string. dxc mirrors the HLSL function name into the LLVM symbol;
	// we mirror ep.Name. BUG-DXIL-022 follow-up.
	mainFn := e.mod.AddFunction(ep.Name, funcTy, false)
	e.mainFn = mainFn
	bb := mainFn.AddBasicBlock("entry")
	e.currentBB = bb
	e.exprValues = make(map[ir.ExpressionHandle]int)
	e.exprComponents = make(map[ir.ExpressionHandle][]int)
	e.localVarPtrs = make(map[uint32]int)
	e.localVarComponentPtrs = make(map[uint32][]int)
	e.localVarStructTypes = make(map[uint32]*module.Type)
	e.localVarArrayTypes = make(map[uint32]*module.Type)
	e.localConstVecArrays = make(map[uint32][][]float32)
	e.initOnlyLocals = make(map[uint32]ir.ExpressionHandle)
	e.globalVarAllocas = make(map[ir.GlobalVariableHandle]int)
	e.globalVarAllocaTypes = make(map[ir.GlobalVariableHandle]*module.Type)
	e.workgroupFieldPtrs = make(map[int]workgroupFieldOrigin)
	e.workgroupElemPtrs = make(map[int]workgroupElemOrigin)
	e.loopStack = e.loopStack[:0]
	e.meshCtx = nil

	fn := &ep.Function
	e.currentFn = fn
	e.exprUseCount = computeExprUseCount(fn)

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
	// e.reachableGlobals (BUG-DXIL-016) was set from EmitOptions and
	// gates analyzeResources to only the globals used by THIS entry
	// point — multi-EP modules share the global arena and would
	// otherwise produce fake "Resource X overlap" errors.
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
	if err := e.emitMetadata(ep, stageToShaderKind(ep.Stage)); err != nil {
		return err
	}

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

	// Global vars. For workgroup/groupshared vars registered via
	// emitGlobalVarAlloca's addrspace-3 path, also map the emitter's
	// pre-allocated value ID to the global's final bitcode value ID
	// so later instruction operands resolve correctly.
	gvarToEmitterID := make(map[*module.GlobalVar]int)
	for emitterID, gv := range e.globalVarModuleVars {
		gvarToEmitterID[gv] = emitterID
	}
	for _, gv := range e.mod.GlobalVars {
		gv.ValueID = nextGlobalID
		if emitterID, ok := gvarToEmitterID[gv]; ok {
			idMap[emitterID] = nextGlobalID
		}
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
			// InstrPhi stores value IDs in PhiIncomings (not Operands),
			// so the standard valueOperandIndices loop above misses them.
			// Without this remap, phi operands stay as raw emitter IDs and
			// the bitcode signed-delta encoding references invalid value
			// positions, producing "Invalid record" at parser-time.
			if instr.Kind == module.InstrPhi {
				for i := range instr.PhiIncomings {
					if newID, ok := idMap[instr.PhiIncomings[i].ValueID]; ok {
						instr.PhiIncomings[i].ValueID = newID
					}
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
			// Phi value-IDs live in PhiIncomings (not Operands); see
			// the entry-point finalize() for the same remap call site.
			if instr.Kind == module.InstrPhi {
				for i := range instr.PhiIncomings {
					if newID, ok := funcIDMap[instr.PhiIncomings[i].ValueID]; ok {
						instr.PhiIncomings[i].ValueID = newID
					}
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
func (e *Emitter) emitMetadata(ep *ir.EntryPoint, kind module.ShaderKind) error {
	// Defensive guard: BUG-DXIL-012 documented that an unset e.mainFn here
	// causes us to emit !dx.entryPoints[0][0] = null, which makes
	// IDxcValidator AV at dxil.dll+0xe9da inside its entry-point walker.
	// We refuse to emit a container that would trigger that AV class —
	// even if downstream callers swallow the error, this turns a silent
	// post-emit crash into an immediate, attributable Go error.
	if e.mainFn == nil {
		return fmt.Errorf("dxil: emit %q: entry function not registered (would emit !dx.entryPoints[0][0] = null and trigger IDxcValidator AV — see BUG-DXIL-012)", ep.Name)
	}
	i32Ty := e.mod.GetIntType(32)

	// llvm.ident = !{!"dxil-go-naga"} — registered FIRST so the named-metadata
	// declaration order in the bitcode matches DXC's output (DXC emits
	// !llvm.ident before any !dx.* entries; see any DXC -dumpbin output).
	// Order doesn't affect semantics — the validator accepts either ordering —
	// but matching DXC byte-for-byte simplifies golden parity diffing
	// (TestDxilDxcGolden) since the metadata reference IDs (!M0, !M1, ...)
	// renumber in declaration order.
	mdIdent := e.mod.AddMetadataString("dxil-go-naga")
	mdIdentTuple := e.mod.AddMetadataTuple([]*module.MetadataNode{mdIdent})
	e.mod.AddNamedMetadata("llvm.ident", []*module.MetadataNode{mdIdentTuple})

	// dx.version = !{i32 1, i32 MINOR}
	mdMajor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(1))
	mdMinor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(e.opts.ShaderModelMinor)))
	mdVersion := e.mod.AddMetadataTuple([]*module.MetadataNode{mdMajor, mdMinor})
	e.mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersion})

	// dx.valver = !{i32 1, i32 8}
	//
	// Validator version determines the expected PSVRuntimeInfoN size that
	// dxil.dll cross-checks against the PSV0 part. validator 1.6 expects
	// PSVRuntimeInfo1 (36 B), 1.7 expects PSVRuntimeInfo2 (48 B), 1.8 expects
	// PSVRuntimeInfo3 (52 B). BUG-DXIL-009 emits PSVRuntimeInfo3 (52 B) in
	// PSV0 unconditionally, so we MUST declare validator 1.8+ here for the
	// two values to agree. Mismatch returns 0x80aa0013 with text:
	//   "DXIL container mismatch for 'PSVRuntimeInfoSize' between
	//    'PSV0' part:('52') and DXIL module:('48')"
	// dxc.exe's own output uses validator 1.8 unconditionally; we mirror.
	// BUG-DXIL-013.
	mdValMajor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(1))
	mdValMinor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(8))
	mdValVer := e.mod.AddMetadataTuple([]*module.MetadataNode{mdValMajor, mdValMinor})
	e.mod.AddNamedMetadata("dx.valver", []*module.MetadataNode{mdValVer})

	// dx.shaderModel = !{!"vs", i32 6, i32 MINOR}
	mdKind := e.mod.AddMetadataString(shaderKindString(kind))
	mdSMMajor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(6))
	mdSMMinor := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(e.opts.ShaderModelMinor)))
	mdSM := e.mod.AddMetadataTuple([]*module.MetadataNode{mdKind, mdSMMajor, mdSMMinor})
	e.mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})

	// Emit resource metadata before viewIdState — DXC outputs !dx.resources
	// before !dx.viewIdState in the named metadata section. Matching this
	// order ensures metadata IDs renumber identically after normalization.
	mdResources := e.emitResourceMetadata()

	// dx.viewIdState — required by the DXIL validator for graphics stages
	// AND by the D3D12 runtime format validator for CreateGraphicsPipelineState.
	// IDxcValidator does not demand it, so lack of emission was hidden until
	// end-to-end D3D12 gate proved 0/N graphics pipelines could create.
	//
	// Mesh shaders use their own variant (emitViewIDState). Every other
	// graphics stage (VS/PS/HS/DS/GS) must emit the conservative aggregate
	// form via emitGraphicsViewIDState.
	switch kind {
	case module.MeshShader:
		e.emitViewIDState(ep)
	case module.VertexShader, module.PixelShader, module.HullShader,
		module.DomainShader, module.GeometryShader:
		e.emitGraphicsViewIDState(ep)
	}

	// Build shader properties (tag-value pairs) for the entry point.
	// Tag 0 = ShaderFlags (i64). Tag 4 = NumThreads (compute/mesh).
	// Tag 9 = MSState (mesh). Tags must appear in ascending order; DXC
	// emits the ShaderFlags tag first when non-zero. BUG-DXIL-022
	// follow-up: previously we never emitted tag 0, so dxil.dll
	// reported "Flags must match usage. note: declared=0, actual=N"
	// for every shader that touched a raw/structured buffer.
	var propPairs []*module.MetadataNode
	flags := e.computeBitcodeShaderFlags()
	e.finalShaderFlags = flags
	if flags != 0 {
		i32Ty := e.mod.GetIntType(32)
		i64Ty := e.mod.GetIntType(64)
		propPairs = append(propPairs,
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(0)),
			e.mod.AddMetadataValue(i64Ty, e.getInt64Const(int64(flags))), //nolint:gosec // bit pattern, sign reinterpret OK
		)
	}
	switch kind {
	case module.ComputeShader:
		propPairs = append(propPairs, e.computePropertyPairs(ep)...)
	case module.MeshShader:
		propPairs = append(propPairs, e.meshPropertyPairs(ep)...)
	case module.AmplificationShader:
		// Task (amplification) shader: numthreads + payload size, like
		// mesh shader but without the mesh-output state. dxc tag layout
		// is the same kDxilNumThreadsTag(4) used by compute. Without
		// this our task shader had no NumThreads metadata and dxil.dll
		// read garbage values (0,0,184) from uninitialized memory.
		// BUG-DXIL-008 follow-up.
		propPairs = append(propPairs, e.computePropertyPairs(ep)...)
	}
	var mdProperties *module.MetadataNode
	if len(propPairs) > 0 {
		mdProperties = e.mod.AddMetadataTuple(propPairs)
	}

	// Build entry point signatures metadata.
	// DXC format: !{!input_sig, !output_sig, !patch_const_or_prim_sig}
	// where each component is null OR an MDNode listing per-element nodes.
	// The whole tuple is null only if all three signatures are empty.
	//
	// BUG-DXIL-021: compute and amplification shaders have no I/O signatures.
	// Their kernel arguments (WorkgroupID / LocalInvocationID / DispatchThreadID
	// / payload) are NOT I/O signature elements — they're read via dedicated
	// dx.op intrinsics (threadId, groupId, flattenedThreadIdInGroup, ...).
	// Previously we routed them through emitGraphicsSignatureMetadata, which
	// default-mapped every unknown builtin to Arbitrary "TEXCOORD" and made
	// dx.entryPoints[0][2] non-null. dxil.dll then read that as "module
	// declares an input signature" and rejected the container with
	// "Missing part 'Program Input Signature' required by module" (and
	// "Semantic 'TEXCOORD' is invalid as cs PatchConstant" for the same
	// shaders). Mirror dxc: nil signatures triplet for compute / amplification.
	var mdSignatures *module.MetadataNode
	switch kind {
	case module.MeshShader:
		mdSignatures = e.emitMeshSignatureMetadata(ep)
	case module.ComputeShader, module.AmplificationShader:
		mdSignatures = nil
	default:
		mdSignatures = e.emitGraphicsSignatureMetadata(ep, kind)
	}

	// dx.entryPoints = !{!{void()* @main, !"main", !signatures, !resources, !properties}}
	// The function pointer (operand 0) must reference the entry function — a null
	// here causes the DXIL validator to AV at dxil.dll+0xe9da when walking entry
	// points. See BUG-DXIL-012.
	var mdFunc *module.MetadataNode
	if e.mainFn != nil {
		mdFunc = e.mod.AddMetadataFunc(e.mainFn)
	}
	mdName := e.mod.AddMetadataString(ep.Name)
	mdEntry := e.mod.AddMetadataTuple([]*module.MetadataNode{
		mdFunc,       // function pointer to @main
		mdName,       // entry point name
		mdSignatures, // signatures triplet (or nil if all empty)
		mdResources,  // resources (nil if none)
		mdProperties, // properties (nil if none)
	})
	e.mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})
	// !llvm.ident now registered up-front so it precedes !dx.* entries — see
	// the comment near the top of this function.
	return nil
}

// computePropertyPairs returns the [tag, value] pairs that describe a
// compute entry point's NumThreads property. Caller wraps the slice in
// a metadata tuple together with any preceding ShaderFlags tag. Format:
//
//	[i32 kDxilNumThreadsTag, !{i32 X, i32 Y, i32 Z}]
//	kDxilNumThreadsTag = 4
//
// Reference: Mesa nir_to_dxil.c emit_threads() ~1785, emit_tag() ~1967
func (e *Emitter) computePropertyPairs(ep *ir.EntryPoint) []*module.MetadataNode {
	i32Ty := e.mod.GetIntType(32)
	mdTag := e.mod.AddMetadataValue(i32Ty, e.getIntConst(4))
	x := max(ep.Workgroup[0], 1)
	y := max(ep.Workgroup[1], 1)
	z := max(ep.Workgroup[2], 1)
	mdX := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(x)))
	mdY := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(y)))
	mdZ := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(z)))
	mdThreads := e.mod.AddMetadataTuple([]*module.MetadataNode{mdX, mdY, mdZ})
	return []*module.MetadataNode{mdTag, mdThreads}
}

// computeBitcodeShaderFlags computes the DXIL ShaderFlags bit field for
// the bitcode `dx.entryPoints[0][4]` properties tag 0. The validator
// computes the same value from actual opcode/type usage in DxilModule.
// CollectShaderFlagsForModule and reports "Flags must match usage" on
// any mismatch.
//
// Bit layout matches dxc DxilShaderFlags.h struct field order (verified
// by hand against actual=N hex values reported by dxilval --wgsl):
//
//	bit  4: EnableRawAndStructuredBuffers (0x10)
//	bit  5: LowPrecisionPresent           (0x20)
//	bit 16: UAVsAtEveryStage              (0x10000)
//	bit 20: Int64Ops                      (0x100000)
//	bit 23: UseNativeLowPrecision         (0x800000)
//	bit 25: RaytracingTier1_1             (0x2000000)
//
// We add more bits as we hit the next walls. BUG-DXIL-022 follow-up.
//
//nolint:gocognit,cyclop,gocyclo // single dispatch table over independent flag bits
func (e *Emitter) computeBitcodeShaderFlags() uint64 {
	var flags uint64
	hasNonImageBuffer := false
	hasUAV := false
	for i := range e.resources {
		r := &e.resources[i]
		switch r.class {
		case resourceClassSRV, resourceClassUAV:
			// Only non-image SRVs/UAVs (raw/structured/typed buffer)
			// trigger EnableRawAndStructuredBuffers. Textures don't,
			// and ray-tracing acceleration structures don't either —
			// they have their own ResourceKind (RTAccelerationStructure,
			// 16) and are tracked via a separate ShaderFlag. DXC's
			// CollectShaderFlagsForModule excludes AS explicitly.
			if int(r.typeHandle) < len(e.ir.Types) {
				inner := e.ir.Types[r.typeHandle].Inner
				if ba, ok := inner.(ir.BindingArrayType); ok {
					if int(ba.Base) < len(e.ir.Types) {
						inner = e.ir.Types[ba.Base].Inner
					}
				}
				switch inner.(type) {
				case ir.ImageType, ir.AccelerationStructureType:
					// Not a raw/structured buffer.
				default:
					hasNonImageBuffer = true
				}
			} else if r.kindOverride == 11 || r.kindOverride == 12 {
				// Synthetic raw/structured buffer (e.g., sampler-heap
				// index array) has no backing IR type; derive "non-image
				// buffer" from kindOverride so the shader flag still
				// matches the actual resource set.
				hasNonImageBuffer = true
			}
			if r.class == resourceClassUAV {
				hasUAV = true
			}
		}
	}
	if hasNonImageBuffer {
		flags |= 0x10 // EnableRawAndStructuredBuffers
	}
	// UAVsAtEveryStage (bit 16) — DXC's DxilModule.cpp:339-350 sets this
	// flag when ANY UAV is declared AND the shader stage is one of the
	// raster pipeline stages where UAV access is "unusual" enough that
	// the runtime needs to know.
	//
	// Pre-validator-1.8: NumUAVs > 0 && !(IsCS || IsPS)
	// 1.8+: NumUAVs > 0 && (IsVS || IsHS || IsDS || IsGS)
	//
	// Both forms exclude CS (compute always has UAV access) and PS
	// (always allowed). The 1.8+ form additionally excludes library /
	// mesh / amplification stages where the bit was redundant. We mirror
	// the 1.8+ form because it is narrower and matches the version of
	// dxil.dll our corpus walker links against.
	if hasUAV && e.currentEntryPoint != nil {
		switch e.currentEntryPoint.Stage {
		case ir.StageVertex:
			flags |= 0x10000
		}
	}
	if e.moduleUsesInt64() || e.moduleUsesInt64TypedAtomic() {
		// Int64Ops also implied by typed-resource int64 atomics: the
		// texture format itself carries the i64 element type and DXC
		// sets both bits.
		flags |= 0x100000 // Int64Ops
	}
	if e.moduleUsesDouble() {
		// DXC's CollectShaderFlagsForModule sets both bits together
		// whenever any f64 type appears (DxilShaderFlags.cpp). Bit 2 is
		// the historical D3D11 "doubles enabled" flag; bit 6 was added
		// for the double-extensions instructions (FMA on doubles, etc.)
		// and DXC always pairs them.
		flags |= 0x4  // EnableDoublePrecision (bit 2)
		flags |= 0x40 // EnableDoubleExtensions (bit 6)
	}
	if e.moduleUsesLowPrecision() {
		// WGSL f16 always maps to native half (SM 6.2+, -enable-16bit-types
		// semantics). Set both LowPrecisionPresent and UseNativeLowPrecision
		// — dxc's GetFeatureInfo uses the pair to pick NativeLowPrecision in
		// SFI0; setting only one of the two would mis-regenerate SFI0 and
		// trip 'Container part Feature Info does not match expected for
		// module'.
		flags |= 0x20     // LowPrecisionPresent
		flags |= 0x800000 // UseNativeLowPrecision
	}
	if e.moduleUsesRayQuery() {
		flags |= 0x2000000 // RaytracingTier1_1
	}
	if e.entryUsesViewIDInput() {
		flags |= 0x200000 // ViewID (bit 21)
	}
	if e.moduleUsesInt64TypedAtomic() {
		flags |= 0x8000000 // AtomicInt64OnTypedResource (bit 27)
	}
	if e.moduleUsesInt64GroupSharedAtomic() {
		flags |= 0x10000000 // AtomicInt64OnGroupShared (bit 28)
	}
	if e.moduleUsesWaveOps() {
		// WaveOps (bit 19, 0x80000) — DXC DxilShaderFlags.h /
		// CollectShaderFlagsForModule sets this when any wave-level
		// intrinsic is used: Wave* (WaveActiveAllTrue, WaveReadLane*,
		// WaveGetLaneIndex, WaveGetLaneCount, WavePrefix*), Quad* reads,
		// subgroup collectives, ballot, broadcast, shuffle*, etc.
		// subgroup-operations.wgsl trips 'Flags declared=0, actual=524288'
		// without it.
		flags |= 0x80000
	}
	return flags
}

// moduleUsesWaveOps reports whether the current entry point (or any
// reachable function) references a wave/subgroup/quad intrinsic or
// consumes a subgroup builtin. Used to set the WaveOps shader flag.
//
// The detection is IR-level so it stays in sync with the emit path that
// lowers these constructs to dx.op.wave*/dx.op.quad* calls.
func (e *Emitter) moduleUsesWaveOps() bool {
	if e.currentEntryPoint == nil {
		return false
	}
	ep := e.currentEntryPoint

	// Subgroup builtin arguments (flat).
	for i := range ep.Function.Arguments {
		arg := &ep.Function.Arguments[i]
		if arg.Binding == nil {
			continue
		}
		if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok && isSubgroupBuiltin(bb.Builtin) {
			return true
		}
	}
	// Subgroup builtin arguments carried via struct members.
	for i := range ep.Function.Arguments {
		arg := &ep.Function.Arguments[i]
		argTy := e.ir.Types[arg.Type]
		st, isSt := argTy.Inner.(ir.StructType)
		if !isSt {
			continue
		}
		for memberIdx := range st.Members {
			m := &st.Members[memberIdx]
			if m.Binding == nil {
				continue
			}
			if bb, ok := (*m.Binding).(ir.BuiltinBinding); ok && isSubgroupBuiltin(bb.Builtin) {
				return true
			}
		}
	}
	// Subgroup statements (barrier/ballot/collective/gather) in function body.
	if containsSubgroupStmt(ep.Function.Body) {
		return true
	}
	return false
}

// isSubgroupBuiltin reports whether b is a WGSL subgroup builtin that
// lowers to a wave intrinsic.
func isSubgroupBuiltin(b ir.BuiltinValue) bool {
	switch b {
	case ir.BuiltinSubgroupInvocationID,
		ir.BuiltinSubgroupSize,
		ir.BuiltinSubgroupID,
		ir.BuiltinNumSubgroups:
		return true
	}
	return false
}

// containsSubgroupStmt walks a statement block looking for any
// subgroup/quad statement. Recurses into block-structured statements.
func containsSubgroupStmt(body ir.Block) bool {
	for i := range body {
		s := &body[i]
		switch k := s.Kind.(type) {
		case ir.StmtSubgroupBallot, ir.StmtSubgroupCollectiveOperation, ir.StmtSubgroupGather:
			return true
		case ir.StmtBarrier:
			if k.Flags&ir.BarrierSubGroup != 0 {
				return true
			}
		case ir.StmtBlock:
			if containsSubgroupStmt(k.Block) {
				return true
			}
		case ir.StmtIf:
			if containsSubgroupStmt(k.Accept) || containsSubgroupStmt(k.Reject) {
				return true
			}
		case ir.StmtLoop:
			if containsSubgroupStmt(k.Body) || containsSubgroupStmt(k.Continuing) {
				return true
			}
		case ir.StmtSwitch:
			for ci := range k.Cases {
				if containsSubgroupStmt(k.Cases[ci].Body) {
					return true
				}
			}
		}
	}
	return false
}

// moduleUsesInt64GroupSharedAtomic reports whether any reachable workgroup
// global variable holds an int64 atomic. DXC sets
// m_bAtomicInt64OnGroupShared (bit 28) for these, with corresponding SFI0
// ShaderFeatureInfo_AtomicInt64OnGroupShared = 0x800000. atomicOps-int64.wgsl
// trips 'Flags must match usage' without it.
func (e *Emitter) moduleUsesInt64GroupSharedAtomic() bool {
	for i := range e.ir.GlobalVariables {
		gv := &e.ir.GlobalVariables[i]
		if gv.Space != ir.SpaceWorkGroup {
			continue
		}
		if e.reachableGlobals != nil && !e.reachableGlobals[ir.GlobalVariableHandle(i)] {
			continue
		}
		if globalVarHasInt64Atomic(e.ir, gv) {
			return true
		}
	}
	return false
}

// globalVarHasInt64Atomic walks a global var's type for an AtomicType
// wrapping i64/u64. Mirrors dxil.go globalUsesInt64Atomic but lives here
// because the emitter needs it during shader-flag computation.
func globalVarHasInt64Atomic(irMod *ir.Module, gv *ir.GlobalVariable) bool {
	if gv == nil || int(gv.Type) >= len(irMod.Types) {
		return false
	}
	visited := make(map[ir.TypeHandle]bool)
	var walk func(th ir.TypeHandle) bool
	walk = func(th ir.TypeHandle) bool {
		if visited[th] || int(th) >= len(irMod.Types) {
			return false
		}
		visited[th] = true
		switch t := irMod.Types[th].Inner.(type) {
		case ir.AtomicType:
			return (t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint) && t.Scalar.Width == 8
		case ir.ArrayType:
			return walk(t.Base)
		case ir.StructType:
			for _, m := range t.Members {
				if walk(m.Type) {
					return true
				}
			}
		}
		return false
	}
	return walk(gv.Type)
}

// moduleUsesInt64TypedAtomic reports whether any reachable global resource
// in the current module is a storage texture with R64Uint or R64Sint
// format — i.e. a typed UAV that supports textureAtomic{Min,Max,...}
// returning i64. DXC sets m_bAtomicInt64OnTypedResource (bit 27) for
// these, with a corresponding SFI0 ShaderFeatureInfo_AtomicInt64OnTypedResource
// (0x400000). atomicTexture-int64.wgsl trips
// 'Flags must match usage' without it.
func (e *Emitter) moduleUsesInt64TypedAtomic() bool {
	for i := range e.ir.GlobalVariables {
		gv := &e.ir.GlobalVariables[i]
		if e.reachableGlobals != nil && !e.reachableGlobals[ir.GlobalVariableHandle(i)] {
			continue
		}
		if int(gv.Type) >= len(e.ir.Types) {
			continue
		}
		img, ok := e.ir.Types[gv.Type].Inner.(ir.ImageType)
		if !ok || img.Class != ir.ImageClassStorage {
			continue
		}
		if img.StorageFormat == ir.StorageFormatR64Uint || img.StorageFormat == ir.StorageFormatR64Sint {
			return true
		}
	}
	return false
}

// entryUsesViewIDInput reports whether the current entry point binds
// BuiltinViewIndex on any argument. Used by computeBitcodeShaderFlags to
// set the ViewID feature bit, which the validator regenerates from
// instruction-level dx.op.viewID usage.
func (e *Emitter) entryUsesViewIDInput() bool {
	if e.currentEntryPoint == nil {
		return false
	}
	for i := range e.currentEntryPoint.Function.Arguments {
		arg := &e.currentEntryPoint.Function.Arguments[i]
		if arg.Binding == nil {
			continue
		}
		if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinViewIndex {
			return true
		}
	}
	return false
}

// moduleUsesLowPrecision returns true if any IR type or expression
// forces 16-bit float/int operations in the emitted bitcode. DXC's
// CollectShaderFlagsForModule sets m_bLowPrecisionPresent (bit 5,
// 0x20) on ANY f16/i16 materialization, and the validator rejects a
// mismatch with 'Flags must match usage. declared=0 actual=32'.
//
// Two trigger paths:
//
//  1. Explicit 16-bit types declared in the IR type arena — the
//     straightforward case (f16 attribute, i16 storage, etc.).
//
//  2. ir.MathQuantizeF16 used in any reachable function, even though
//     the surrounding shader is pure f32: the lowering injects
//     fptrunc f32→f16 / fpext f16→f32 casts, so the emitted bitcode
//     contains f16 values that the validator counts toward bit 5.
//     math-functions.wgsl declares ONLY f32 but uses quantizeToF16
//     on scalar/vec2/vec3/vec4 paths and tripped this gap.
func (e *Emitter) moduleUsesLowPrecision() bool {
	for i := range e.ir.Types {
		switch t := e.ir.Types[i].Inner.(type) {
		case ir.ScalarType:
			if t.Width == 2 {
				return true
			}
		case ir.VectorType:
			if t.Scalar.Width == 2 {
				return true
			}
		case ir.MatrixType:
			if t.Scalar.Width == 2 {
				return true
			}
		}
	}

	// Walk reachable function expressions for QuantizeF16 — even a
	// single use forces f16 into the emitted bitcode via fptrunc.
	if e.currentEntryPoint != nil {
		if functionUsesQuantizeF16(&e.currentEntryPoint.Function) {
			return true
		}
	}
	for i := range e.ir.Functions {
		if functionUsesQuantizeF16(&e.ir.Functions[i]) {
			return true
		}
	}
	return false
}

// functionUsesQuantizeF16 reports whether any expression in fn is an
// ir.ExprMath with Fun == ir.MathQuantizeF16.
func functionUsesQuantizeF16(fn *ir.Function) bool {
	for i := range fn.Expressions {
		if me, ok := fn.Expressions[i].Kind.(ir.ExprMath); ok && me.Fun == ir.MathQuantizeF16 {
			return true
		}
	}
	return false
}

// moduleUsesDouble returns true when the EMITTED bitcode actually contains
// f64 values (as opposed to merely declaring f64 types in the IR).
//
// DXC's CollectShaderFlagsForModule walks LLVM instructions and function
// signatures — anywhere a double materializes in EXECUTED code — not the
// type arena and not unused constants. Two corners caused this to be
// non-trivial:
//
//  1. f64.wgsl declares f64 constants but the only f64 helper is dropped
//     by the emit-time zero-fallback path (scalar return, accesses
//     globals, fails inliner gate). The leftover f64 literal constants
//     remain in mod.Constants but no instruction references them. DXC
//     considers unused constants as "not used" and reports actual=0;
//     scanning mod.Constants here would over-report and trip 'Flags
//     must match usage'.
//
//  2. conversion-float-to-int.wgsl inlines test_f64_to_*_vec helpers
//     whose bodies cast f64 to i32. The instruction RESULT is i32, but
//     the FuncType of the inlined helper or any dx.op.f64 declaration
//     still carries f64 in its signature. Scanning function signatures
//     (e.mod.Functions, declarations included) catches this.
//
// We therefore scan:
//   - Function declaration signatures (RetType + ParamTypes) — covers
//     dx.op.X.f64 intrinsics and helper functions.
//   - Defined function instruction result types — covers any fadd/fmul
//     producing an f64 SSA value.
//
// We DELIBERATELY DO NOT scan mod.Constants — see (1) above.
func (e *Emitter) moduleUsesDouble() bool {
	for _, fn := range e.mod.Functions {
		if fn.FuncType != nil {
			if isDoubleType(fn.FuncType.RetType) {
				return true
			}
			for _, p := range fn.FuncType.ParamTypes {
				if isDoubleType(p) {
					return true
				}
			}
		}
		if fn.IsDeclaration {
			continue
		}
		for _, bb := range fn.BasicBlocks {
			for _, instr := range bb.Instructions {
				if instr.ResultType != nil && isDoubleType(instr.ResultType) {
					return true
				}
			}
		}
	}
	return false
}

// isDoubleType reports whether a module.Type is the 64-bit float scalar.
func isDoubleType(t *module.Type) bool {
	return t != nil && t.Kind == module.TypeFloat && t.FloatBits == 64
}

// moduleUsesInt64 returns true if any IR type in the current module is
// a 64-bit integer scalar / vector. dxc's CollectShaderFlagsForModule
// sets m_bInt64Ops whenever int64 types appear anywhere.
func (e *Emitter) moduleUsesInt64() bool {
	for i := range e.ir.Types {
		switch t := e.ir.Types[i].Inner.(type) {
		case ir.ScalarType:
			if (t.Kind == ir.ScalarSint || t.Kind == ir.ScalarUint) && t.Width == 8 {
				return true
			}
		case ir.VectorType:
			if (t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint) && t.Scalar.Width == 8 {
				return true
			}
		}
	}
	return false
}

// moduleUsesRayQuery returns true if the current entry point's EMITTED
// bitcode contains ray query instructions. This scans the entry point
// body and every helper function that was successfully emitted into
// e.helperFunctions. Helpers that were filtered out (struct/vector
// returns, global variable access, or other unsupported features) are
// emitted as zero-value stubs — their ray query statements never reach
// the blob, so the validator computes actual=no-raytracing and the
// declared flag must match.
//
// Mirrors DXC's ShaderFlags::CollectShaderFlags which is per-function
// (DxilShaderFlags.cpp:393) and merged over the call graph by
// AdjustMinimumShaderModelAndFlags. dxil.go:moduleUsesRayQuery is still
// module-wide because it drives the SM 6.5 auto-upgrade which is
// correctly conservative (upgrade affects the whole module).
func (e *Emitter) moduleUsesRayQuery() bool {
	if e.currentEntryPoint != nil && blockHasRayQuery(e.currentEntryPoint.Function.Body) {
		return true
	}
	// Only emitted helpers contribute instructions to the blob; stubbed
	// helpers never reach bitcode.
	for handle := range e.helperFunctions {
		if int(handle) >= len(e.ir.Functions) {
			continue
		}
		if blockHasRayQuery(e.ir.Functions[handle].Body) {
			return true
		}
	}
	return false
}

// blockHasRayQuery scans a statement block tree for StmtRayQuery nodes.
func blockHasRayQuery(block ir.Block) bool {
	for i := range block {
		s := &block[i]
		if _, ok := s.Kind.(ir.StmtRayQuery); ok {
			return true
		}
		switch k := s.Kind.(type) {
		case ir.StmtBlock:
			if blockHasRayQuery(k.Block) {
				return true
			}
		case ir.StmtIf:
			if blockHasRayQuery(k.Accept) || blockHasRayQuery(k.Reject) {
				return true
			}
		case ir.StmtLoop:
			if blockHasRayQuery(k.Body) || blockHasRayQuery(k.Continuing) {
				return true
			}
		case ir.StmtSwitch:
			for _, c := range k.Cases {
				if blockHasRayQuery(c.Body) {
					return true
				}
			}
		}
	}
	return false
}

// meshPropertyPairs returns the [tag, value] pairs for a mesh entry
// point's MSState property (tag 9). Caller assembles the surrounding
// tuple and any preceding ShaderFlags pair.
//
// DXC reference format:
//
//	[i32 9, !{!{i32 X, i32 Y, i32 Z}, i32 maxVert, i32 maxPrim, i32 topo, i32 payloadSize}]
//
// outputTopology: 1=line, 2=triangle
func (e *Emitter) meshPropertyPairs(ep *ir.EntryPoint) []*module.MetadataNode {
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

	return []*module.MetadataNode{mdMSTag, mdMSState}
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
				semName := dxilArbitrarySemantic
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

// dxilArbitrarySemantic is the default DXIL semantic name used for
// WGSL @location(N) varyings that don't map to a system value. Sourced
// from backend.LocationSemantic so HLSL, DXIL, and wgpu/hal/dx12's
// input-layout SemanticName cannot drift (BUG-DXIL-028).
const dxilArbitrarySemantic = backend.LocationSemantic

// dxilSigInfo holds the per-element data needed to build a !dx.entryPoints
// signature element for graphics stages. Mirrors the relevant subset of
// DxilSignatureElement.
type dxilSigInfo struct {
	id          int64
	semName     string
	compType    int64 // DXIL ComponentType enum (1=I1, 4=I32, 5=U32, 8=F16, 9=F32 …)
	semKind     int64 // DXIL SemanticKind enum (0=Arbitrary, 1=VertexID, 3=Position …)
	semIndex    int64
	interpMode  int64 // DXIL InterpolationMode enum (0=Undefined, 2=Linear, 4=NoPerspective …)
	rows        int64
	cols        int64
	startRow    int64
	startCol    int64
	numChannels int64 // number of components actually used (1..4)
}

// isSystemManagedOutputSemantic reports whether a DXIL SemanticKind
// identifies a pixel-stage SIGNATURE element that the signature allocator
// refuses to pack into a real row/col — i.e. one that must carry
// StartRow = -1, StartCol = -1. Mirrors DXC's DxilSignatureAllocator
// behavior: the Depth family, Coverage, StencilRef (outputs), and
// SampleIndex (input) are system-managed and read/written via dedicated
// dx.op calls (storeDepth, storeCoverage, sampleIndex) even though a
// signature element is still emitted to declare them.
//
// The DXC validator rule (DxilValidation.cpp:4538) checks
// SemanticInterpretationKind. NotPacked and Shadow both imply
// ShouldBeAllocated=false. NotPacked: depth family, coverage on output.
// Shadow: SampleIndex on PS input. We unify both under the same helper
// because the IsAllocated rule is direction-agnostic.
//
// SemanticKind values from DxilConstants.h:281 enum SemanticKind:
//
//	12 = SampleIndex
//	14 = Coverage
//	15 = InnerCoverage
//	17 = Depth
//	18 = DepthLessEqual
//	19 = DepthGreaterEqual
//	20 = StencilRef
func isSystemManagedOutputSemantic(semKind int64) bool {
	switch semKind {
	case 12, // SampleIndex (SV_SampleIndex on PS input — Shadow interpretation)
		14, // Coverage (SV_Coverage on PS output — NotPacked)
		17, // Depth (SV_Depth)
		18, // DepthLessEqual
		19, // DepthGreaterEqual
		20: // StencilRef
		return true
	}
	return false
}

// builtinToDXILSemantic maps a naga BuiltinValue to (DXIL SemanticKind, semantic name).
func builtinToDXILSemantic(b ir.BuiltinValue) (int64, string) {
	switch b {
	case ir.BuiltinPosition:
		return 3, "SV_Position"
	case ir.BuiltinVertexIndex:
		return 1, "SV_VertexID"
	case ir.BuiltinInstanceIndex:
		return 2, "SV_InstanceID"
	case ir.BuiltinFrontFacing:
		return 13, "SV_IsFrontFace"
	case ir.BuiltinFragDepth:
		return 17, "SV_Depth"
	case ir.BuiltinSampleIndex:
		return 12, "SV_SampleIndex"
	case ir.BuiltinSampleMask:
		return 14, "SV_Coverage"
	case ir.BuiltinClipDistance:
		return 6, "SV_ClipDistance"
	case ir.BuiltinPrimitiveIndex:
		return 10, "SV_PrimitiveID"
	default:
		return 0, dxilArbitrarySemantic
	}
}

// dxilCompTypeForScalar maps a naga ScalarKind to DXIL ComponentType enum.
func dxilCompTypeForScalar(kind ir.ScalarKind) int64 {
	switch kind {
	case ir.ScalarFloat:
		return 9 // F32
	case ir.ScalarSint:
		return 4 // I32
	case ir.ScalarUint:
		return 5 // U32
	case ir.ScalarBool:
		return 1 // I1
	default:
		return 9 // default to F32
	}
}

// describeIRType returns (compType, cols) for an IR type.
// Vector types decompose to (scalar compType, vector size).
// Scalars are 1-wide. Anything else defaults to (F32, 4).
func describeIRType(irMod *ir.Module, th ir.TypeHandle) (int64, int64) {
	if int(th) >= len(irMod.Types) {
		return 9, 4
	}
	switch ti := irMod.Types[th].Inner.(type) {
	case ir.ScalarType:
		return dxilCompTypeForScalar(ti.Kind), 1
	case ir.VectorType:
		return dxilCompTypeForScalar(ti.Scalar.Kind), int64(ti.Size)
	case ir.ArrayType:
		// Array of scalar (e.g. SV_ClipDistance is array<f32, 1..8>):
		// element kind comes from the array's element scalar, and the
		// number of "channels" is the constant array size. Without this,
		// the default fallback returned (Float32, 4) which made our
		// signature declare SV_ClipDistance with a full xyzw mask while
		// the storeOutput sequence only wrote element 0 — producing
		// 'Not all elements of output SV_ClipDistance were written'.
		// SV_ClipDistance/SV_CullDistance are the canonical scalar-array
		// signature elements; arrays of vectors are not valid signature
		// elements and fall through to the default.
		if int(ti.Base) >= len(irMod.Types) {
			return 9, 4
		}
		baseInner := irMod.Types[ti.Base].Inner
		baseScalar, ok := scalarOfType(baseInner)
		if !ok {
			return 9, 4
		}
		count := int64(1)
		if ti.Size.Constant != nil {
			count = int64(*ti.Size.Constant)
		}
		if count < 1 {
			count = 1
		}
		if count > 4 {
			count = 4 // single-row signature element max
		}
		return dxilCompTypeForScalar(baseScalar.Kind), count
	}
	return 9, 4
}

// interpolationModeForBinding maps a (LocationBinding interpolation,
// stage, direction) triple to its DXIL InterpolationMode enum value.
// Mirrors dxil.go:psvInterpolationMode — direction-aware: only the
// CARRIER directions (vertex out, fragment in) emit a non-Undefined
// interp value; vertex in, fragment out, compute, etc. emit 0.
func interpolationModeForBinding(interp *ir.Interpolation, isFragment, isOutput bool) uint8 {
	isVertex := !isFragment
	carries := (isVertex && isOutput) || (isFragment && !isOutput)
	if !carries {
		return 0
	}
	if interp == nil {
		return 2 // Linear (perspective) — default for both carriers
	}
	switch interp.Kind {
	case ir.InterpolationFlat:
		return 1 // Constant
	case ir.InterpolationLinear:
		switch interp.Sampling {
		case ir.SamplingCentroid:
			return 5
		case ir.SamplingSample:
			return 7
		default:
			return 4
		}
	case ir.InterpolationPerspective:
		switch interp.Sampling {
		case ir.SamplingCentroid:
			return 3
		case ir.SamplingSample:
			return 6
		default:
			return 2
		}
	}
	return 2
}

// makeSigInfo builds a dxilSigInfo from a naga binding and IR type.
func makeSigInfo(irMod *ir.Module, id int, binding ir.Binding, th ir.TypeHandle, isOutput, isFragment bool) dxilSigInfo {
	info := dxilSigInfo{
		id:         int64(id),
		semIndex:   0,
		interpMode: 0,
		rows:       1,
		startRow:   0,
		startCol:   0,
	}
	info.compType, info.cols = describeIRType(irMod, th)
	info.numChannels = info.cols

	switch b := binding.(type) {
	case ir.BuiltinBinding:
		info.semKind, info.semName = builtinToDXILSemantic(b.Builtin)
		// SV_Position is always F32 vec4.
		if b.Builtin == ir.BuiltinPosition {
			info.compType = 9
			info.cols = 4
			info.numChannels = 4
		}
		// SV_IsFrontFace is always U32 in the signature metadata even
		// though the WGSL source declares it as `bool`. DXC validator
		// rejects 'SV_IsFrontFace must be uint' when the bitcode CompType
		// is I1 (the natural mapping from ScalarBool). The shader code
		// reads the loaded i32 and compares to 0 to produce its i1 SSA
		// value (see emitSingleInputLoad's bool-input path); the
		// SIGNATURE element here is independent and must match what dxc
		// declares for the SV_IsFrontFace built-in.
		if b.Builtin == ir.BuiltinFrontFacing {
			info.compType = 5 // U32
			info.cols = 1
			info.numChannels = 1
		}
		// Direction-aware interpolation. Verified against dxc reference
		// (vs_flat_out.hlsl). Carriers:
		//   stage    direction  carries
		//   vertex   in         no
		//   vertex   out        YES — to-rasterizer values
		//   fragment in         YES — from-rasterizer values
		//   fragment out        no — SV_Target / SV_Depth
		//
		// Vertex output and fragment input MUST emit identical interp
		// values for the same semantic.
		isVertex := !isFragment
		carries := (isVertex && isOutput) || (isFragment && !isOutput)
		if carries {
			if b.Builtin == ir.BuiltinPosition {
				info.interpMode = 4 // NoPerspective
			} else if b.Builtin == ir.BuiltinClipDistance {
				info.interpMode = 2 // Linear
			} else {
				info.interpMode = 0 // VertexID, InstanceID, etc — not actually emitted in carrier directions
			}
		}
	case ir.LocationBinding:
		info.semKind = 0 // Arbitrary
		info.semIndex = int64(b.Location)
		// Dual-source blending: @blend_src(N) overrides Location for the
		// SV_Target index. DXC HLSL writes dual source as SV_Target0 +
		// SV_Target1 (rows 0 and 1) and the runtime maps target 1 as the
		// dual-source alpha. Mirror the rewrite here so the bitcode
		// metadata side agrees with the OSG1 container side
		// (bindingToSignatureElements applies the same override).
		if b.BlendSrc != nil && isFragment && isOutput {
			info.semIndex = int64(*b.BlendSrc)
		}
		if isFragment && isOutput {
			info.semKind = 16 // Target (SV_Target)
			info.semName = "SV_Target"
			info.interpMode = 0 // Undefined for fragment outputs (SV_Target)
		} else {
			info.semName = dxilArbitrarySemantic
			// BUG-DXIL-019 follow-up: read the WGSL @interpolate
			// attribute instead of hard-coding. Mirrors PSV0 side
			// (dxil.go:psvInterpolationMode). Without this, vertex
			// outputs with @interpolate(flat) get bitcode interp=0
			// (Undefined) while PSV0 gets interp=1 (Constant), and
			// dxil.dll rejects with "SigOutputElement mismatch".
			info.interpMode = int64(interpolationModeForBinding(b.Interpolation, isFragment, isOutput))
		}
	default:
		info.semName = dxilArbitrarySemantic
	}
	return info
}

// collectGraphicsSignatures walks an entry point's args/result and produces
// per-element signature info for inputs and outputs.
//
// Each signature element gets a unique startRow that advances by its
// own Rows count. dxc's DxilSignatureAllocator assigns rows in this
// linear fashion for the simple case (one element per row); it can
// pack multiple elements into a single row only when their components
// don't overlap and their interpolation modes match. We currently
// assign one element per row for both inputs and outputs, mirroring
// what buildGraphicsPSVSigs does on the PSV0 side. BUG-DXIL-019.
//
// BuiltinViewIndex is skipped — SV_ViewID is read via dx.op.viewID()
// intrinsic, not via a signature element.
//
//nolint:gocognit,funlen,gocyclo,cyclop // signature element collection iterates two parallel walks
func (e *Emitter) collectGraphicsSignatures(ep *ir.EntryPoint, isFragment bool) (inputs, outputs []dxilSigInfo) {
	fn := &ep.Function

	skipBuiltin := func(b ir.Binding) bool {
		bb, ok := b.(ir.BuiltinBinding)
		if !ok {
			return false
		}
		switch bb.Builtin {
		case ir.BuiltinViewIndex:
			return true
		}
		return false
	}

	// skipInputBuiltin handles the additional input-only exclusions:
	// @builtin(sample_mask) on a fragment input maps to SV_Coverage,
	// which DXC declares as NotInSig for PS input (DxilSigPoint.inl:97-98
	// 'NotInSig _50'). The value is read via dx.op.coverage(91) instead.
	// The validator rejects 'Semantic SV_Coverage is invalid as ps Input'
	// when an SV_Coverage element appears in the PS input signature.
	skipInputBuiltin := func(b ir.Binding) bool {
		if skipBuiltin(b) {
			return true
		}
		bb, ok := b.(ir.BuiltinBinding)
		if !ok {
			return false
		}
		if bb.Builtin == ir.BuiltinSampleMask && isFragment {
			return true
		}
		return false
	}

	// addInputPacked / addOutputPacked replace the per-element row counter
	// with the (Register, StartCol) the packer assigned. SV_Position keeps
	// its own row, system-managed elements still carry (-1, -1) and consume
	// no row, locations within the same interp group share rows.
	addInputPacked := func(id *int, binding ir.Binding, th ir.TypeHandle, pe backend.PackedElement) {
		if skipInputBuiltin(binding) {
			return
		}
		info := makeSigInfo(e.ir, *id, binding, th, false, isFragment)
		if isSystemManagedOutputSemantic(info.semKind) {
			info.startRow = -1
			info.startCol = -1
			inputs = append(inputs, info)
			*id++
			return
		}
		info.startRow = int64(pe.Register)
		info.startCol = int64(pe.StartCol)
		inputs = append(inputs, info)
		*id++
	}
	addOutputPacked := func(id *int, binding ir.Binding, th ir.TypeHandle, pe backend.PackedElement) {
		if skipBuiltin(binding) {
			return
		}
		info := makeSigInfo(e.ir, *id, binding, th, true, isFragment)
		if isSystemManagedOutputSemantic(info.semKind) {
			info.startRow = -1
			info.startCol = -1
			outputs = append(outputs, info)
			*id++
			return
		}
		info.startRow = int64(pe.Register)
		info.startCol = int64(pe.StartCol)
		outputs = append(outputs, info)
		*id++
	}

	stage := ep.Stage
	isVSInput := stage == ir.StageVertex
	interpFnIn := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(interpolationModeForBinding(loc.Interpolation, isFragment, false))
	}
	interpFnOut := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(interpolationModeForBinding(loc.Interpolation, isFragment, true))
	}

	// Inputs: flatten args + struct members into a single binding list.
	{
		bindings, types := collectFlatArgBindings(e.ir, fn.Arguments)
		infos := make([]backend.SigElementInfo, len(bindings))
		for i, b := range bindings {
			infos[i] = backend.SigElementInfoForBinding(e.ir, b, types[i], stage, false, interpFnIn)
			if isVSInput && infos[i].Kind == backend.SigPackLocation {
				infos[i].Kind = backend.SigPackBuiltinSystemValue
			}
		}
		packed := backend.PackSignatureElements(infos, true)
		id := 0
		for i, b := range bindings {
			if packed[i].Rows == 0 {
				continue
			}
			addInputPacked(&id, b, types[i], packed[i])
		}
	}

	if fn.Result != nil {
		oid := 0
		resType := e.ir.Types[fn.Result.Type]
		if st, ok := resType.Inner.(ir.StructType); ok {
			packed := backend.PackStructMembers(e.ir, st.Members, stage, true, false, interpFnOut)
			for _, pm := range packed {
				if !pm.HasBinding || pm.Rows == 0 {
					continue
				}
				m := st.Members[pm.OrigIdx]
				addOutputPacked(&oid, *m.Binding, m.Type, pm.PackedElement)
			}
		} else if fn.Result.Binding != nil {
			pe := backend.PackedElement{Register: 0, StartCol: 0, ColCount: 4, Rows: 1}
			addOutputPacked(&oid, *fn.Result.Binding, fn.Result.Type, pe)
		}
	}
	return inputs, outputs
}

// collectFlatArgBindings flattens a function's arg list (and any struct-typed
// arg's members) into a sorted (binding, type) pair list using the shared
// interface-order convention. Mirrors dxil/dxil.go collectFlatBindings.
func collectFlatArgBindings(irMod *ir.Module, args []ir.FunctionArgument) ([]ir.Binding, []ir.TypeHandle) {
	var bindings []ir.Binding
	var types []ir.TypeHandle
	for _, arg := range args {
		if arg.Binding != nil {
			bindings = append(bindings, *arg.Binding)
			types = append(types, arg.Type)
			continue
		}
		argType := irMod.Types[arg.Type]
		st, ok := argType.Inner.(ir.StructType)
		if !ok {
			continue
		}
		order := backend.SortedMemberIndices(st.Members)
		for _, idx := range order {
			m := st.Members[idx]
			if m.Binding == nil {
				continue
			}
			bindings = append(bindings, *m.Binding)
			types = append(types, m.Type)
		}
	}
	return bindings, types
}

// buildSignatureElementMD constructs one !{...} signature element node.
//
// Layout (matches dxc DxilMDHelper::EmitSignatureElement):
//
//	[0]  i32 ID
//	[1]  !"SemanticName"
//	[2]  i8  CompType
//	[3]  i8  SemanticKind
//	[4]  !{i32 ...}  semantic index vector
//	[5]  i8  InterpolationMode
//	[6]  i32 Rows
//	[7]  i8  Cols
//	[8]  i32 StartRow
//	[9]  i8  StartCol
//	[10] !{i32 3, i32 mask} | null  extended properties — kDxilSignatureElement
//	                          UsageCompMaskTag(3) followed by the mask of
//	                          components actually used. null when unused.
//
// extendedUsedMask controls the [10] extended-properties field:
//   - < 0: emit !{i32 3, i32 fullMask} where fullMask = (1<<numChannels)-1
//     (output elements: all declared components are "written")
//   - == 0: emit null (input not used by any output-contributing code path)
//   - > 0: emit !{i32 3, i32 extendedUsedMask} (input partially/fully used)
func (e *Emitter) buildSignatureElementMD(info dxilSigInfo, extendedUsedMask int64) *module.MetadataNode {
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)

	mdSemIdx := e.mod.AddMetadataTuple([]*module.MetadataNode{
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(info.semIndex)),
	})

	// Determine the extended properties metadata node.
	mdExtended := e.buildExtendedPropertiesMD(i32Ty, info, extendedUsedMask)

	return e.mod.AddMetadataTuple([]*module.MetadataNode{
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(info.id)),
		e.mod.AddMetadataString(info.semName),
		e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(info.compType)),
		e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(info.semKind)),
		mdSemIdx,
		e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(info.interpMode)),
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(info.rows)),
		e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(info.cols)),
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(info.startRow)),
		e.mod.AddMetadataValue(i8Ty, e.getI8MetadataConst(info.startCol)),
		mdExtended,
	})
}

// buildExtendedPropertiesMD returns the [10] extended-properties metadata node
// for a signature element based on the extendedUsedMask policy value.
func (e *Emitter) buildExtendedPropertiesMD(i32Ty *module.Type, info dxilSigInfo, extendedUsedMask int64) *module.MetadataNode {
	switch {
	case extendedUsedMask < 0:
		// Output element: use full channel mask (all declared components written).
		channels := clampChannels(info.numChannels, info.cols)
		usageMask := int64(1)<<channels - 1
		return e.mod.AddMetadataTuple([]*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(3)), // kDxilSignatureElementUsageCompMaskTag
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(usageMask)),
		})
	case extendedUsedMask == 0:
		// Input not used: emit null (matches DXC for unused inputs).
		return nil
	default:
		// Input used: emit the specific used-component mask.
		return e.mod.AddMetadataTuple([]*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(3)), // kDxilSignatureElementUsageCompMaskTag
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(extendedUsedMask)),
		})
	}
}

// clampChannels resolves the effective channel count for extended properties,
// falling back to cols when numChannels is out of [1..4] range.
func clampChannels(numChannels, cols int64) int64 {
	ch := numChannels
	if ch <= 0 || ch > 4 {
		ch = cols
	}
	if ch < 0 {
		ch = 0
	} else if ch > 4 {
		ch = 4
	}
	return ch
}

// emitGraphicsSignatureMetadata builds the dx.entryPoints signatures triplet
// for non-mesh graphics stages: {inputs, outputs, patchConstOrPrim}.
//
// Returns nil when all three sub-signatures are empty (matches dxc behavior:
// the whole tuple is omitted instead of !{null, null, null}).
func (e *Emitter) emitGraphicsSignatureMetadata(ep *ir.EntryPoint, kind module.ShaderKind) *module.MetadataNode {
	isFragment := kind == module.PixelShader
	inputs, outputs := e.collectGraphicsSignatures(ep, isFragment)
	if len(inputs) == 0 && len(outputs) == 0 {
		return nil
	}

	var mdInputs, mdOutputs *module.MetadataNode
	if len(inputs) > 0 {
		nodes := make([]*module.MetadataNode, 0, len(inputs))
		for i := range inputs {
			// Use ViewID-derived used mask for inputs. When the input has
			// zero contribution to outputs, emit null extended properties
			// (matches DXC behavior for unused inputs).
			var usedMask int64
			if i < len(e.inputUsedMasks) {
				usedMask = e.inputUsedMasks[i]
			}
			nodes = append(nodes, e.buildSignatureElementMD(inputs[i], usedMask))
		}
		mdInputs = e.mod.AddMetadataTuple(nodes)
	}
	if len(outputs) > 0 {
		nodes := make([]*module.MetadataNode, 0, len(outputs))
		for i := range outputs {
			// Outputs always emit full channel mask (-1 = use default).
			nodes = append(nodes, e.buildSignatureElementMD(outputs[i], -1))
		}
		mdOutputs = e.mod.AddMetadataTuple(nodes)
	}

	return e.mod.AddMetadataTuple([]*module.MetadataNode{mdInputs, mdOutputs, nil})
}

// emitGraphicsViewIDState emits dx.viewIdState for non-mesh graphics stages.
//
// The serialized state is a flat uint32 array. dxc only emits the metadata
// when at least one entry is non-zero. We mirror that condition: skip when
// the shader has no signature elements at all.
//
// Conservative default (matches dxc trivial VS output):
//
//	!dx.viewIdState = !{!N}
//	!N = !{[K x i32] [i32 inputComps, i32 outputComps, i32 1]}
//
// where:
//
//	inputComps  = sum of per-input  component counts (or 1 if empty)
//	outputComps = sum of per-output component counts
//	trailing 1  = "no per-view dependencies, single view used"
//
// This is a minimal valid encoding that satisfies the validator without
// performing real per-component view-id dependency analysis.
//
// emitGraphicsViewIDState emits dx.viewIdState for non-mesh graphics stages.
// Called from emitMetadata for VS/PS/HS/DS/GS (mesh shaders use
// emitViewIDState which emits a different layout).
//
// Canonical DXC form (restored session 12, BUG-DXIL-018):
//
//	!dx.viewIdState = !{!N}
//	!N = !{[3 x i32] [i32 inComps, i32 outComps, i32 1]}
//
// where inComps = sum of per-input component counts (min 1),
//
//	outComps = sum of per-output component counts (min 1),
//	trailing 1 = "single view, no per-component view-id deps".
//
// The metadata operand is a ConstantAsMetadata wrapping a [3 x i32]
// aggregate constant. `DxilMDHelper::LoadDxilViewIdState`
// (dxc/lib/DXIL/DxilMetadataHelper.cpp:2211) validates exactly this
// shape — D3D12 runtime format validation calls `LoadDxilViewIdState`
// during CreateGraphicsPipelineState, so deviations (e.g. scalar tuple)
// are rejected with 0x80aa000f DXC_E_INCORRECT_DXIL_METADATA.
//
// Session 11 temporarily tried a scalar-tuple form (7e8ff33) to bypass
// a parse-gate regression on 34 shaders (0x80aa0018
// DXC_E_GENERAL_INTERNAL_ERROR). That form parses but fails the D3D12
// runtime gate on >50 shaders — strictly worse. Session 12 restores
// this canonical aggregate form; the 34-shader parse regression is a
// separate follow-up (likely related to our serializer emitting
// CST_CODE_AGGREGATE rather than CST_CODE_DATA for integer arrays;
// DXC expects ConstantDataArray).
func (e *Emitter) emitGraphicsViewIDState(ep *ir.EntryPoint) {
	isFragment := ep.Stage == ir.StageFragment
	inputs, outputs := e.collectGraphicsSignatures(ep, isFragment)
	if len(inputs) == 0 && len(outputs) == 0 {
		return
	}

	inElems := sigInfosToViewIDElems(inputs)
	outElems := sigInfosToViewIDElems(outputs)

	// Match DXC's DetermineMaxPackedLocation: the scalar count is
	// max(StartRow*4 + StartCol + NumChannels) over all non-system-managed
	// elements. See ComputeViewIdStateBuilder.cpp:279, GetLinearIndex.
	var inComps, outComps uint32
	for i := range inElems {
		if inElems[i].SystemManaged {
			continue
		}
		end := inElems[i].VectorRow*4 + inElems[i].StartCol + inElems[i].NumChannels
		if end > inComps {
			inComps = end
		}
	}
	for i := range outElems {
		if outElems[i].SystemManaged {
			continue
		}
		end := outElems[i].VectorRow*4 + outElems[i].StartCol + outElems[i].NumChannels
		if end > outComps {
			outComps = end
		}
	}

	// BUG-DXIL-018 Phase 3: emit the canonical DXC viewIdState payload
	// shape per DxilViewIdState::Serialize (ComputeViewIdState.cpp:252):
	//
	//   [NumInputScalars,
	//    NumOutputScalars,
	//    InputContrib[0..NumInputs-1] * outUINTs each]
	//
	// Per-input contribution masks come from the IR-level pass-through
	// dataflow analyzer (dxil/internal/viewid). For simple shaders like
	// the gogpu triangle FS the analyzer produces exact per-scalar
	// dependency bits; for constructs it can't trace precisely
	// (user-function calls, ray queries, etc.) it falls back to
	// "every input contributes to every output" — conservative but
	// still matches what D3D12 runtime expects for such shapes.
	const uintBits = 32
	outUINTs := (outComps + uintBits - 1) / uintBits
	// DXC's RoundUpToUINT(0) = 0 — when there are no output scalars the
	// contribution section is empty (NumInputs * 0 = 0 dwords). Do NOT
	// pad outUINTs to 1 here; that inflates the array from [2 x i32] to
	// [2+inComps x i32] with trailing zeros, mismatching the DXC golden.
	// Reference: ComputeViewIdState.cpp:208 RoundUpToUINT, line 264-268.
	totalUINTs := 2 + uint64(inComps)*uint64(outUINTs)
	values := make([]uint64, totalUINTs)
	values[0] = uint64(inComps)
	values[1] = uint64(outComps)

	// Run the dataflow analyzer when we have both inputs and outputs.
	var deps *viewid.Deps
	if len(inElems) > 0 && len(outElems) > 0 {
		deps = viewid.Analyze(e.ir, ep, inElems, outElems)
	}
	// Map pre-computed IR usage masks to per-input-element masks.
	e.inputUsedMasks = e.computeInputElementUsedMasks(ep, inElems)

	if deps != nil {
		// Copy per-input-scalar dwords (each scalar = outUINTs dwords).
		for i, v := range deps.InputScalarToOutputs {
			idx := uint64(2) + uint64(i)
			if idx < totalUINTs {
				values[idx] = uint64(v)
			}
		}
	}

	i32Ty := e.mod.GetIntType(32)
	arrTy := e.mod.GetArrayType(i32Ty, uint(totalUINTs))
	dataConst := e.mod.AddDataArrayConst(arrTy, values)
	dataID := e.allocValue()
	e.constMap[dataID] = dataConst

	mdViewID := e.mod.AddMetadataValue(arrTy, dataConst)
	mdViewIDTuple := e.mod.AddMetadataTuple([]*module.MetadataNode{mdViewID})
	e.mod.AddNamedMetadata("dx.viewIdState", []*module.MetadataNode{mdViewIDTuple})
}

// sigInfosToViewIDElems translates the emit-side dxilSigInfo list into
// the viewid package's SigElement list. The ScalarStart and VectorRow
// fields reflect packed (sum-of-NumChannels) and allocated (per-row)
// ordering that collectGraphicsSignatures assigned.
//
// SystemManaged elements are preserved with SystemManaged=true so the
// dataflow analyzer knows to skip them when computing input taint.
// computeInputElementUsedMasks maps pre-computed per-argument usage masks
// (from sig_usage.go analysis) to per-input-signature-element masks, clamped
// to the element's actual channel count. Returns nil-length slice when no
// usage data is available.
func (e *Emitter) computeInputElementUsedMasks(ep *ir.EntryPoint, inElems []viewid.SigElement) []int64 {
	masks := make([]int64, len(inElems))
	if e.opts.InputUsedMasks == nil {
		return masks
	}
	elemIdx := 0
	for argIdx := range ep.Function.Arguments {
		arg := &ep.Function.Arguments[argIdx]
		argType := e.ir.Types[arg.Type]
		if st, ok := argType.Inner.(ir.StructType); ok { //nolint:nestif // struct-member vs direct-binding dispatch
			for mi := range st.Members {
				if st.Members[mi].Binding == nil {
					continue
				}
				if elemIdx >= len(masks) {
					break
				}
				key := InputUsageKey{ArgIdx: argIdx, MemberIdx: mi}
				mask := e.opts.InputUsedMasks[key]
				channels := inElems[elemIdx].NumChannels
				if channels > 0 && channels <= 4 {
					mask &= uint8((1 << channels) - 1)
				}
				masks[elemIdx] = int64(mask)
				elemIdx++
			}
		} else if arg.Binding != nil {
			if elemIdx < len(masks) {
				key := InputUsageKey{ArgIdx: argIdx, MemberIdx: -1}
				mask := e.opts.InputUsedMasks[key]
				channels := inElems[elemIdx].NumChannels
				if channels > 0 && channels <= 4 {
					mask &= uint8((1 << channels) - 1)
				}
				masks[elemIdx] = int64(mask)
				elemIdx++
			}
		}
	}
	return masks
}

func sigInfosToViewIDElems(infos []dxilSigInfo) []viewid.SigElement {
	out := make([]viewid.SigElement, len(infos))
	var cumScalars uint32
	for i := range infos {
		info := &infos[i]
		if info.numChannels < 0 {
			continue
		}
		sysManaged := info.startRow < 0 || info.startCol < 0
		// VectorRow MUST mirror the actual signature register the packer
		// (PackSignatureElements) assigned, not arg/struct-member order.
		// DXC's GetLinearIndex(elem, row, col) = elem.StartRow*4 + col,
		// so output indices in `dx.viewIdState` and PSV0 dependency tables
		// are anchored to the register slot. After BUG-DXIL-029 reordered
		// outputs to match DXC (locations first, builtins last), the
		// arg-order counter diverged from the register, producing
		// "output 4" where DXC has "output 0".
		var row, col uint32
		if !sysManaged && info.startRow >= 0 {
			row = uint32(info.startRow) //nolint:gosec // startRow ≥ 0 after bound check
		}
		if !sysManaged && info.startCol >= 0 {
			col = uint32(info.startCol) //nolint:gosec // startCol ≥ 0 after bound check
		}
		elem := viewid.SigElement{
			ScalarStart:   cumScalars,
			NumChannels:   uint32(info.numChannels), //nolint:gosec // numChannels ≥ 0 after bound check
			VectorRow:     row,
			StartCol:      col,
			SystemManaged: sysManaged,
		}
		out[i] = elem
		cumScalars += elem.NumChannels
	}
	return out
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

// isConstValueID returns true if the given emitter value ID refers to a
// module-level constant (int or float). Used for LLVM operand canonicalization:
// commutative binops place constants on the right.
func (e *Emitter) isConstValueID(id int) bool {
	_, ok := e.constMap[id]
	return ok
}

// tryNegateFloatConst checks whether valueID refers to a non-zero float
// constant and, if so, returns a new constant with the negated value.
// Used for LLVM InstCombine canonicalization: fsub(a, +C) -> fadd(a, -C).
func (e *Emitter) tryNegateFloatConst(valueID int) (int, bool) {
	c, ok := e.constMap[valueID]
	if !ok || c.IsUndef || c.IsAggregate {
		return 0, false
	}
	// Only negate non-zero float constants (zero stays as fsub).
	if c.ConstType == nil || c.FloatValue == 0 {
		return 0, false
	}
	// Verify it's a float type.
	if c.ConstType.Kind != module.TypeFloat {
		return 0, false
	}
	negated := e.addFloatConstID(c.ConstType, -c.FloatValue)
	return negated, true
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

// getInt64Const creates (or reuses) an i64 LLVM constant with value v.
// Separate cache from getIntConst because i32/i64 constants are
// distinct module entries even when their integer value coincides;
// METADATA_VALUE references encode (type, constant) pairs and the type
// must match the wrapping metadata field.
func (e *Emitter) getInt64Const(v int64) *module.Constant {
	if id, ok := e.int64Consts[v]; ok {
		return e.constMap[id]
	}
	c := e.mod.AddIntConst(e.mod.GetIntType(64), v)
	id := e.allocValue()
	if e.int64Consts == nil {
		e.int64Consts = make(map[int64]int)
	}
	e.int64Consts[v] = id
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
// emitFRemLowered implements float remainder without using the LLVM FRem instruction.
// DXIL does not support FRem natively (DXC rejects it with "Invalid record").
// Lower to: a - b * floor(a / b), matching Mesa nir_to_dxil.c (lower_fmod = true).
func (e *Emitter) emitFRemLowered(resultTy *module.Type, lhs, rhs int) int {
	// floor function: dx.op.round_ni (opcode 27)
	floorFn := e.getDxOpUnaryFunc("dx.op.unary", overloadF32)
	floorOp := e.getIntConstID(int64(OpRoundNI))

	// Adjust overload for f16/f64 if needed.
	if resultTy.Kind == module.TypeFloat {
		switch resultTy.FloatBits {
		case 16:
			floorFn = e.getDxOpUnaryFunc("dx.op.unary", overloadF16)
		case 64:
			floorFn = e.getDxOpUnaryFunc("dx.op.unary", overloadF64)
		}
	}

	// Step 1: div = a / b
	div := e.addBinOpInstr(resultTy, BinOpFDiv, lhs, rhs)
	// Step 2: floorDiv = floor(div)
	floorDiv := e.addCallInstr(floorFn, resultTy, []int{floorOp, div})
	// Step 3: prod = b * floorDiv
	prod := e.addBinOpInstr(resultTy, BinOpFMul, rhs, floorDiv)
	// Step 4: result = a - prod
	return e.addBinOpInstr(resultTy, BinOpFSub, lhs, prod)
}

func (e *Emitter) addBinOpInstr(resultTy *module.Type, op BinOpKind, lhs, rhs int) int {
	valueID := e.allocValue()
	var flags uint32
	// DXC always sets the UnsafeAlgebra fast-math flag (bit 0) for
	// non-precise float binary operations. WGSL has no `precise`
	// qualifier, so all float ops get fast-math, matching DXC output.
	if resultTy != nil && resultTy.Kind == module.TypeFloat {
		flags = 1 // UnsafeAlgebra = "fast"
	}
	instr := &module.Instruction{
		Kind:       module.InstrBinOp,
		HasValue:   true,
		ResultType: resultTy,
		Operands:   []int{lhs, rhs, int(op)},
		Flags:      flags,
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

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, valueTy}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
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

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(retTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
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
	fn.AttrSetID = classifyDxOpAttr(fullName)
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
	fn.AttrSetID = classifyDxOpAttr(fullName)
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
	fn.AttrSetID = classifyDxOpAttr(fullName)
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
	fn.AttrSetID = classifyDxOpAttr(fullName)
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

// classifyDxOpAttr returns the AttrSet for a dx.op intrinsic by full name.
// Mirrors DXC's per-OpCode classification in lib/HLSL/DxilOperations.cpp
// OpFuncAttrType. Names checked by prefix because per-overload functions
// have suffixes (e.g., dx.op.bufferLoad.f32, dx.op.bufferLoad.i32).
//
// Pure (readnone) — no memory access:
//
//	threadId, threadIdInGroup, groupId, flattenedThreadIdInGroup,
//	loadInput, coverage, viewID, domainLocation,
//	unary/binary/tertiary/quaternary math, dot, isSpecialFloat,
//	makeDouble, splitDouble, etc.
//
// ReadOnly (readonly) — read memory, never write:
//
//	bufferLoad, rawBufferLoad, cbufferLoadLegacy, cbufferLoad,
//	sample/sampleLevel/sampleGrad/sampleCmp/gather*, textureLoad,
//	bufferUpdateCounter (read counter), atomicCompareExchange RESULT.
//
// Impure (nounwind only) — write memory or have control-flow side effects:
//
//	bufferStore, rawBufferStore, storeOutput, textureStore,
//	atomicBinOp, atomicCompareExchange (the call itself),
//	barrier, discard, emitStream, cutStream, etc.
//
// Default for unrecognized dx.op names = AttrSetNounwind (impure-safe).
// The entry point @main always carries AttrSetNounwind (set by AddFunction).
func classifyDxOpAttr(fullName string) uint32 {
	const prefix = "dx.op."
	if len(fullName) < len(prefix) || fullName[:len(prefix)] != prefix {
		return module.AttrSetNounwind
	}
	rest := fullName[len(prefix):]
	// Strip overload suffix like ".f32" / ".i32" — match by base name.
	if dot := indexByte(rest, '.'); dot >= 0 {
		rest = rest[:dot]
	}
	switch rest {
	// Pure intrinsics
	case "threadId", "threadIdInGroup", "groupId", "flattenedThreadIdInGroup",
		"loadInput", "coverage", "viewID", "domainLocation", "outputControlPointID",
		"primitiveID", "gsInstanceID", "stencilRef",
		"unary", "binary", "tertiary", "quaternary",
		"dot2", "dot3", "dot4", "dot",
		"makeDouble", "splitDouble", "isSpecialFloat",
		"isNaN", "isInf", "isFinite", "isNormal",
		"firstbitLo", "firstbitHi", "firstbitSHi", "countbits",
		"bfrev", "imad", "umad", "ibfe", "ubfe", "bfi",
		"saturate", "round_ne", "round_ni", "round_pi", "round_z":
		return module.AttrSetReadNone
	// ReadOnly intrinsics
	case "createHandle", "createHandleFromBinding", "annotateHandle",
		"bufferLoad", "rawBufferLoad", "rawBufferVectorLoad",
		"cbufferLoad", "cbufferLoadLegacy",
		"sample", "sampleBias", "sampleLevel", "sampleGrad", "sampleCmp", "sampleCmpLevelZero",
		"sampleCmpLevel", "sampleCmpGrad",
		"textureLoad", "textureGather", "textureGatherCmp", "textureGatherRaw",
		"calculateLOD", "getDimensions",
		"bufferUpdateCounter":
		return module.AttrSetReadOnly
		// Impure (default)
	}
	return module.AttrSetNounwind
}

// indexByte is strings.IndexByte without the import. Returns -1 if not found.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
