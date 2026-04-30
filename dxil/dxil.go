// Package dxil implements a DXIL (DirectX Intermediate Language) backend
// for the naga shader compiler.
//
// DXIL is LLVM 3.7 bitcode with DirectX-specific metadata and dx.op
// intrinsic calls, wrapped in a DXBC container. This backend generates
// DXIL directly from naga IR, eliminating the need for external HLSL
// compilers (FXC/DXC).
//
// This package provides the public API surface. All implementation
// details (bitcode writer, module builder, container assembly) are in
// internal sub-packages.
//
// Current status: Phase 1 — vertex + fragment shader lowering (SM 6.0).
// The Compile function translates naga IR entry points to DXIL bytecode.
//
// Reference implementations:
//   - Mesa's src/microsoft/compiler/ (C, MIT license)
//   - LLVM 3.7 Bitcode Format: https://releases.llvm.org/3.7.1/docs/BitCodeFormat.html
//   - INF-0004 Validator Hashing: https://github.com/microsoft/hlsl-specs
package dxil

import (
	"fmt"
	"sort"

	"github.com/gogpu/naga/dxil/internal/container"
	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/dxil/internal/passes/dce"
	"github.com/gogpu/naga/dxil/internal/passes/mem2reg"
	"github.com/gogpu/naga/dxil/internal/passes/sroa"
	"github.com/gogpu/naga/dxil/internal/viewid"
	"github.com/gogpu/naga/internal/backend"
	"github.com/gogpu/naga/ir"
)

// ShaderModel represents a DXIL shader model version.
type ShaderModel struct {
	Major uint32
	Minor uint32
}

// Predefined shader model versions.
var (
	SM6_0 = ShaderModel{6, 0}
	SM6_1 = ShaderModel{6, 1}
	SM6_2 = ShaderModel{6, 2}
	SM6_3 = ShaderModel{6, 3}
	SM6_4 = ShaderModel{6, 4}
	SM6_5 = ShaderModel{6, 5}
	SM6_6 = ShaderModel{6, 6}
	SM6_7 = ShaderModel{6, 7}
	SM6_8 = ShaderModel{6, 8}
	SM6_9 = ShaderModel{6, 9}
)

// Options configures DXIL compilation.
type Options struct {
	// ShaderModel is the target shader model version.
	ShaderModel ShaderModel

	// UseBypassHash uses the BYPASS sentinel hash (16×0x01) instead of
	// the retail validator hash. The BYPASS form is accepted by
	// AgilitySDK 1.615+ (January 2025) ONLY with developer mode
	// enabled; on consumer systems it is rejected by the D3D12
	// runtime format validator with STATE_CREATION error id 67 /
	// 93 "Shader is corrupt or in an unrecognized format" even
	// though IDxcValidator accepts either hash form.
	//
	// Default: false (retail hash). Set to true explicitly only
	// when the target system is guaranteed to be in dev mode with
	// the right AgilitySDK version.
	UseBypassHash bool

	// BindingMap remaps WGSL @group(N) @binding(M) to DXIL (space, register)
	// at emit time. If a shader binding is not present in the map, its raw
	// WGSL numbers are used (current behavior, preserved for backward
	// compatibility when the map is nil).
	//
	// This is the DXIL analog of hlsl.Options.BindingMap. It is required
	// when the shader is consumed by a pipeline whose root signature uses
	// a different register scheme than the raw WGSL binding numbers —
	// notably wgpu/hal/dx12, which assigns registers via monotonic per-class
	// counters (SRV=t0,t1,... / UAV=u0,u1,... / CBV=b0,b1,...).
	BindingMap BindingMap

	// SamplerHeapTargets specifies binding targets for the synthesized
	// sampler heap arrays (nagaSamplerHeap / nagaComparisonSamplerHeap).
	// Mirrors hlsl.Options.SamplerHeapTargets. When nil, falls back to
	// the defaults: standard at (space=0, register=0), comparison at
	// (space=1, register=0).
	//
	// The D3D12 root signature allocates sampler heap arrays at specific
	// (space, register) positions; these targets must match or the PSO
	// creation will fail with E_INVALIDARG.
	SamplerHeapTargets *SamplerHeapBindTargets

	// SamplerBufferBindingMap maps bind group numbers to SRV binding
	// targets for per-group sampler index buffers
	// (StructuredBuffer<uint> nagaGroup<N>SamplerIndexArray). Mirrors
	// hlsl.Options.SamplerBufferBindingMap.
	//
	// When nil, falls back to the current defaults: each group's index
	// buffer at (space=255, register=group). The D3D12 HAL should
	// populate this map so that the generated DXIL matches the root
	// signature's SRV layout for sampler index buffers.
	SamplerBufferBindingMap map[uint32]BindTarget
}

// DefaultOptions returns the default DXIL compilation options.
func DefaultOptions() Options {
	return Options{
		ShaderModel:   SM6_0,
		UseBypassHash: false, // retail hash — portable, no dev-mode requirement
	}
}

// convertBindingMap converts the public dxil.BindingMap to the internal
// emit.BindingMap. Returns nil if the input is nil, preserving the
// backward-compatible "no remap" behavior downstream.
func convertBindingMap(m BindingMap) emit.BindingMap {
	if m == nil {
		return nil
	}
	out := make(emit.BindingMap, len(m))
	for k, v := range m {
		out[emit.BindingLocation{Group: k.Group, Binding: k.Binding}] = emit.BindTarget{
			Space:            v.Space,
			Register:         v.Register,
			BindingArraySize: v.BindingArraySize,
		}
	}
	return out
}

// convertSamplerHeapTargets converts the public SamplerHeapBindTargets to
// the internal emit mirror. Returns nil when the input is nil, preserving
// the backward-compatible default behavior downstream.
func convertSamplerHeapTargets(t *SamplerHeapBindTargets) *emit.SamplerHeapBindTargets {
	if t == nil {
		return nil
	}
	return &emit.SamplerHeapBindTargets{
		StandardSamplers: emit.BindTarget{
			Space:    t.StandardSamplers.Space,
			Register: t.StandardSamplers.Register,
		},
		ComparisonSamplers: emit.BindTarget{
			Space:    t.ComparisonSamplers.Space,
			Register: t.ComparisonSamplers.Register,
		},
	}
}

// convertSamplerBufferBindingMap converts the public sampler buffer binding
// map to the internal emit mirror. Returns nil when the input is nil.
func convertSamplerBufferBindingMap(m map[uint32]BindTarget) map[uint32]emit.BindTarget {
	if m == nil {
		return nil
	}
	out := make(map[uint32]emit.BindTarget, len(m))
	for group, bt := range m {
		out[group] = emit.BindTarget{
			Space:    bt.Space,
			Register: bt.Register,
		}
	}
	return out
}

// autoUpgradeSMPre returns the minimum SM minor version required by the
// module's features that can be determined before DCE (inlining/optimization).
// Post-DCE upgrades (rawBuffer, int64 buffer) are applied separately after
// reachableGlobals is computed.
func autoUpgradeSMPre(irModule *ir.Module, ep *ir.EntryPoint, smMinor uint32) uint32 {
	// Mesh/amplification shaders require SM 6.5 minimum.
	if (ep.Stage == ir.StageMesh || ep.Stage == ir.StageTask) && smMinor < 5 {
		smMinor = 5
	}
	// Ray query requires SM 6.5.
	if moduleUsesRayQuery(irModule) && smMinor < 5 {
		smMinor = 5
	}
	// 64-bit atomic operations require SM 6.6+ per dxc validator.
	if moduleUsesInt64Atomics(irModule) && smMinor < 6 {
		smMinor = 6
	}
	// Native low precision (WGSL f16 / i16) requires SM 6.2+.
	if moduleUsesLowPrecision(irModule) && smMinor < 2 {
		smMinor = 2
	}
	// dx.op.viewID (opcode 138) is valid from SM 6.1.
	if moduleUsesViewID(irModule) && smMinor < 1 {
		smMinor = 1
	}
	return smMinor
}

// prepareModule clones the IR module and inlines user helper functions.
// Returns the prepared module ready for optimization passes.
//
// BUG-DXIL-029: inline user helper functions at IR level so the DXIL
// emitter never sees a StmtCall to a helper with features it cannot
// emit as a standalone LLVM function (globals access, aggregate
// return, complex locals).
//
// Two-tier inline policy:
//  1. MUST inline: helpers that cannot be emitted standalone
//  2. ALSO inline: pure helpers (no globals access, no break/continue)
//     that CAN be emitted standalone but produce cleaner output inlined
//
// Helpers with side effects (UAV writes, globals) that CAN be emitted
// standalone stay as separate LLVM functions.
func prepareModule(irModule *ir.Module) (*ir.Module, error) {
	if len(irModule.Functions) == 0 {
		return ir.CloneModuleForOverrides(irModule), nil
	}
	irModule = ir.CloneModuleForOverrides(irModule)
	shouldInline := func(callee *ir.Function) bool {
		if helperNeedsInlining(irModule, callee) {
			return true
		}
		if functionHasBreakContinue(callee) {
			return false
		}
		if emit.FunctionAccessesGlobals(callee) {
			return false
		}
		if emit.FunctionHasComplexLocals(callee, irModule) {
			return false
		}
		if emit.FunctionHasComplexExpressions(callee, irModule) {
			return false
		}
		if functionHasControlFlow(callee) {
			return false
		}
		return true
	}
	if err := ir.InlineUserFunctions(irModule, shouldInline); err != nil {
		return nil, fmt.Errorf("dxil: inline user functions: %w", err)
	}
	return irModule, nil
}

// Compile translates a naga IR module to DXIL bytecode wrapped in
// a DXBC container.
//
// The compilation pipeline:
//  1. Emit: naga IR -> DXIL module (types, expressions, statements)
//  2. Serialize: DXIL module -> LLVM 3.7 bitcode
//  3. Container: wrap bitcode in DXBC with program header and hash
//
// The result is a valid DXBC container that can be loaded by D3D12
// or inspected with dxc.exe -dumpbin.
func Compile(irModule *ir.Module, opts Options) ([]byte, error) {
	if irModule == nil {
		return nil, fmt.Errorf("dxil: nil IR module")
	}
	if len(irModule.EntryPoints) == 0 {
		return nil, fmt.Errorf("dxil: module has no entry points")
	}

	ep := &irModule.EntryPoints[0]
	smMinor := autoUpgradeSMPre(irModule, ep, opts.ShaderModel.Minor)

	// Inline user helpers and clone the module so we never mutate the caller's IR.
	var inlineErr error
	irModule, inlineErr = prepareModule(irModule)
	if inlineErr != nil {
		return nil, inlineErr
	}
	ep = &irModule.EntryPoints[0]
	// Run mem2reg + DCE, mirroring the DXC pipeline order
	// (DxilLinker.cpp:1284: mem2reg -> SimplifyInst -> DCE).
	if err := runOptPasses(irModule); err != nil {
		return nil, err
	}

	// BUG-DXIL-016: compute the set of GlobalVariables transitively
	// reachable from this entry point's call graph ONCE up front and
	// thread it through both the bitcode emitter and the PSV0 builder.
	// Multi-EP modules share the global arena across all entry points;
	// without this filter unrelated bindings leak into the per-EP DXIL
	// container and produce fake "Resource X overlap" errors at
	// validation. Reference comparison: DXC achieves the same result
	// via an LLVM GlobalDCE pass after emission
	// (lib/HLSL/DxilLinker.cpp:1285); Mesa via nir_remove_dead_variables
	// pre-pass on the NIR shader (microsoft/compiler/nir_to_dxil.c:6683).
	// We use a single read-time reachability set shared between both
	// layers, which avoids mutating the source IR while keeping the two
	// layers byte-symmetric.
	reachableGlobals := reachableGlobalsForEntry(irModule, ep)

	// Post-DCE SM upgrades: only bump when reachable globals actually need
	// rawBufferLoad/Store (SM 6.2+) or 64-bit element access (SM 6.3+).
	// This mirrors DXC behavior: dead storage buffer accesses are optimized
	// away before SM requirements are applied.
	if reachableUsesRawBufferAccess(irModule, reachableGlobals) && smMinor < 2 {
		smMinor = 2
	}
	if reachableUsesInt64Buffer(irModule, reachableGlobals) && smMinor < 3 {
		smMinor = 3
	}

	// Pre-compute per-input-arg component usage masks from IR body analysis.
	// Passed to the emitter so metadata extended-properties on input signature
	// elements can use null (unused) vs !{i32 3, i32 mask} (used), matching
	// DXC's MarkUsedSignatureElements behavior.
	rawUsedMasks := computeInputUsedMasks(irModule, &ep.Function)
	var inputUsedMasks map[emit.InputUsageKey]uint8
	if len(rawUsedMasks) > 0 {
		inputUsedMasks = make(map[emit.InputUsageKey]uint8, len(rawUsedMasks))
		for k, v := range rawUsedMasks {
			inputUsedMasks[emit.InputUsageKey{ArgIdx: k.argIdx, MemberIdx: k.memberIdx}] = v
		}
	}

	// Step 1: Emit naga IR -> DXIL module.
	emitOpts := emit.EmitOptions{
		ShaderModelMajor:        opts.ShaderModel.Major,
		ShaderModelMinor:        smMinor,
		BindingMap:              convertBindingMap(opts.BindingMap),
		SamplerHeapTargets:      convertSamplerHeapTargets(opts.SamplerHeapTargets),
		SamplerBufferBindingMap: convertSamplerBufferBindingMap(opts.SamplerBufferBindingMap),
		ReachableGlobals:        reachableGlobals,
		InputUsedMasks:          inputUsedMasks,
	}
	mod, bitcodeShaderFlags, err := emit.EmitWithFlags(irModule, emitOpts)
	if err != nil {
		return nil, fmt.Errorf("dxil: emit: %w", err)
	}

	// Step 2: Serialize DXIL module -> LLVM 3.7 bitcode.
	bitcodeData := module.Serialize(mod)
	if len(bitcodeData) == 0 {
		return nil, fmt.Errorf("dxil: serialization produced empty bitcode")
	}

	// Step 3: Wrap in DXBC container.
	// Parts are ordered to match DXC reference: SFI0, ISG1, OSG1, [PSG1], PSV0, STAT, HASH, DXIL.
	shaderKind := stageToContainerKind(ep.Stage)
	isFragment := ep.Stage == ir.StageFragment
	isMesh := ep.Stage == ir.StageMesh
	inputSig, outputSig, primSig := buildSignaturesEx(irModule, ep, isFragment, isMesh)

	c := container.New()
	c.AddFeaturesPart(featureInfoFromShaderFlags(bitcodeShaderFlags))

	// ISG1 / OSG1 container parts are required for ALL stages, INCLUDING
	// compute and amplification. DXC emits empty 8-byte bodies
	// (count=0, offset=8) unconditionally and dxil.dll's
	// VerifyBlobPartMatches checks pWriter->size() which is always 8 for
	// an empty DxilProgramSignatureWriter — therefore a missing container
	// part always trips ContainerPartMissing. The prior exemption for
	// compute was BUG-DXIL-021: it masqueraded a PSV0/metadata leak as
	// "compute may omit", but DXC reference output proves otherwise.
	c.AddInputSignature(inputSig)
	c.AddOutputSignature(outputSig)
	if len(primSig) > 0 {
		c.AddPrimitiveSignature(primSig)
	}

	c.AddPSV0(buildPSVEx(irModule, ep, isFragment, isMesh, len(inputSig), len(outputSig), len(primSig), reachableGlobals, opts.BindingMap, opts.SamplerHeapTargets, opts.SamplerBufferBindingMap))

	// STAT (Shader Statistics) is required by the D3D12 runtime format
	// validator for graphics pipelines (BUG-DXIL-011). IDxcValidator
	// does not require it; omission produced VALID S_OK blobs that the
	// runtime then rejected at CreateGraphicsPipelineState with
	// STATE_CREATION id 67 "Vertex Shader is corrupt" / id 93 for
	// pixel shaders. DXC always emits STAT; we mirror that.
	//
	// Compute containers also benefit from STAT (DXC emits it for
	// compute too) but CreateComputePipelineState is more permissive
	// and empirically accepts our compute containers without it. We
	// emit STAT unconditionally anyway to match DXC's canonical part
	// list and avoid a second dark-corner divergence later.
	//
	// STAT must precede HASH so the runtime hash covers it — DXC's
	// part order is SFI0, ISG1, OSG1, PSV0, STAT, HASH, DXIL.
	c.AddSTATPart(shaderKind, opts.ShaderModel.Major, smMinor, bitcodeData)

	c.AddHashPart()
	c.AddDXILPart(shaderKind, opts.ShaderModel.Major, smMinor, bitcodeData) // DXIL last

	containerData := c.Bytes()

	// Fill the HASH part body with the shader hash (standard MD5 of the
	// DXIL bitcode). Must be set before ComputeRetailHash since the
	// container hash covers the HASH part content. Without this, D3D12
	// rejects graphics pipelines with HRESULT 0x80070057 "shader is
	// corrupt" — the runtime validates both hashes and compute-side
	// shaders are the only path that skips this verification.
	if err := container.WriteShaderHashPart(containerData); err != nil {
		return nil, fmt.Errorf("dxil: WriteShaderHashPart: %w", err)
	}

	// Apply hash: BYPASS sentinel (dev, AgilitySDK 1.615+) or retail (production).
	// Reference: INF-0004 Validator Hashing (microsoft/hlsl-specs).
	if opts.UseBypassHash {
		container.SetBypassHash(containerData)
	} else {
		container.ComputeRetailHash(containerData)
	}

	return containerData, nil
}

// buildSignatures extracts input/output signature elements from an entry point.
//
// Register packing follows DXC's PrefixStable algorithm: locations within the
// same interpolation group are greedy first-fit packed into 4-component
// register rows; SV_Position takes its own row of 4; system-managed PS-stage
// elements (SV_Depth family / SV_Coverage / SV_StencilRef on output,
// SV_SampleIndex on input) carry Register = 0xFFFFFFFF and consume no row.
// VS input is unpacked (PackingKind::InputAssembler — each element on its
// own row at column 0). See internal/backend/sig_pack.go.
func buildSignatures(irMod *ir.Module, ep *ir.EntryPoint, isFragment bool) ([]container.SignatureElement, []container.SignatureElement) {
	stage := ep.Stage
	isVSInput := stage == ir.StageVertex
	interpFn := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(psvInterpolationMode(loc.Interpolation, isFragment, false))
	}
	interpFnOut := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(psvInterpolationMode(loc.Interpolation, isFragment, true))
	}

	// Inputs from function arguments.
	//
	// Struct-member ordering must follow the same locations-first / builtins-
	// last convention as outputs. D3D12 CreateGraphicsPipelineState matches
	// VS output → FS input by (semantic name, register), so if VS outputs are
	// sorted but FS inputs aren't, register positions diverge and the runtime
	// rejects the PSO with E_INVALIDARG (IDxcValidator can't catch this — it
	// validates each stage independently).
	// Flatten args + struct members into a single binding list, sorted.
	bindings, types := collectFlatBindings(irMod, ep.Function.Arguments, nil, stage, false)
	packed := packBindingsForSignature(irMod, bindings, types, stage, false, isVSInput, interpFn)

	// Compute per-input component usage masks from IR body analysis.
	// This mirrors DXC's MarkUsedSignatureElements pass which sets
	// AlwaysReadsMask based on actual loadInput instructions in the bitcode.
	usedMasks := computeInputUsedMasks(irMod, &ep.Function)
	flatArgMapping := buildFlatArgMapping(irMod, ep.Function.Arguments, isVSInput)

	inputs := make([]container.SignatureElement, 0, len(bindings))
	for i, b := range bindings {
		pe := packed[i]
		if pe.Rows == 0 {
			// SigPackOther / no-binding placeholder — skip.
			continue
		}
		elem, ok := buildSigElementFromPacked(irMod, b, types[i], pe, false, isFragment)
		if !ok {
			continue
		}
		// Override RWMask (AlwaysReadsMask) with actual usage from IR analysis.
		if i < len(flatArgMapping) {
			elem.RWMask = resolveInputRWMask(usedMasks, flatArgMapping[i], elem.Mask)
		}
		inputs = append(inputs, elem)
	}
	sortSignatureElements(inputs)

	// Outputs from function result.
	outputs := buildOutputSigElements(irMod, ep.Function.Result, stage, isFragment, interpFnOut)

	return inputs, outputs
}

// buildOutputSigElements walks the entry point's result (struct or scalar) and
// produces the OSG1 signature elements in graphics-output order, with packed
// (Register, StartCol) layout. Empty result returns nil.
func buildOutputSigElements(
	irMod *ir.Module,
	res *ir.FunctionResult,
	stage ir.ShaderStage,
	isFragment bool,
	interpFnOut func(loc ir.LocationBinding) backend.SigPackInterp,
) []container.SignatureElement {
	if res == nil {
		return nil
	}
	resultType := irMod.Types[res.Type]
	st, ok := resultType.Inner.(ir.StructType)
	if !ok {
		if res.Binding == nil {
			return nil
		}
		// Scalar/vector result with binding — single element on row 0.
		return bindingToSignatureElements(irMod, *res.Binding, res.Type, 0, 0, true, isFragment)
	}
	// Walk members in graphics output order — locations first, then builtins
	// (SV_Position last). Same convention naga's HLSL backend applies via
	// interfaceKey/Less in hlsl/functions.go, so DXC's HLSL roundtrip
	// produces an identical OSG1 register layout.
	packed := backend.PackStructMembers(irMod, st.Members, stage, true, false, interpFnOut)
	out := make([]container.SignatureElement, 0, len(packed))
	for _, pm := range packed {
		if !pm.HasBinding || pm.Rows == 0 {
			continue
		}
		member := st.Members[pm.OrigIdx]
		elem, ok := buildSigElementFromPacked(irMod, *member.Binding, member.Type, pm.PackedElement, true, isFragment)
		if !ok {
			continue
		}
		out = append(out, elem)
	}
	// DXC's validator regenerates OSG1 from module metadata sorted by
	// register (ascending), then by start column within a register. The
	// binary OSG1 part must match this ordering or validation fails with
	// "Container part 'Program Output Signature' does not match expected".
	sortSignatureElements(out)
	return out
}

// sortSignatureElements sorts OSG1/ISG1 elements by (Register, StartCol)
// ascending. DXC's validator regenerates the container signature from module
// metadata in this order; if our binary doesn't match, validation fails.
// Elements with Register == 0xFFFFFFFF (system-managed, e.g. SV_Depth) sort
// to the end — matching DXC's convention.
func sortSignatureElements(elems []container.SignatureElement) {
	sort.SliceStable(elems, func(i, j int) bool {
		ri, rj := elems[i].Register, elems[j].Register
		if ri != rj {
			return ri < rj
		}
		// Within the same register, sort by lowest mask bit (start column).
		return maskStartCol(elems[i].Mask) < maskStartCol(elems[j].Mask)
	})
}

// maskStartCol returns the starting column index from a component mask.
func maskStartCol(mask uint8) uint8 {
	for col := uint8(0); col < 4; col++ {
		if mask&(1<<col) != 0 {
			return col
		}
	}
	return 0
}

// collectFlatBindings walks the function's arguments and (for struct args)
// their members, returning the bindings in graphics interface order
// (locations first, builtins last) along with their types.
//
// Top-level arg bindings keep the function-arg order. Struct members are
// sorted by SortedMemberIndices. The pair of slices is what the packer
// consumes.
func collectFlatBindings(
	irMod *ir.Module,
	args []ir.FunctionArgument,
	_ []ir.StructMember,
	stage ir.ShaderStage,
	_ bool,
) ([]ir.Binding, []ir.TypeHandle) {
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
	// Sort across all arguments so locations come before builtins.
	// Within-struct ordering is handled by SortedMemberIndices above,
	// but when multiple struct-typed arguments contribute members (e.g.,
	// fs_main(in: VertexOutput, note: NoteInstance)), the cross-argument
	// ordering must also be locations-first to match DXC's fragment input
	// register assignment. VS inputs use InputAssembler packing which
	// preserves declaration order.
	backend.SortFlatBindings(bindings, types, stage == ir.StageVertex)
	return bindings, types
}

// buildFlatArgMapping constructs a parallel mapping from the flat binding list
// (as produced by collectFlatBindings) back to (argIdx, memberIdx) keys for the
// input usage analysis. The iteration order MUST match collectFlatBindings exactly,
// including the cross-argument locations-first sort for non-VS inputs.
func buildFlatArgMapping(irMod *ir.Module, args []ir.FunctionArgument, isVSInput bool) []inputUsageKey {
	type keyedBinding struct {
		key     inputUsageKey
		binding ir.Binding
	}
	var items []keyedBinding
	for argIdx := range args {
		arg := &args[argIdx]
		if arg.Binding != nil {
			items = append(items, keyedBinding{
				key:     inputUsageKey{argIdx: argIdx, memberIdx: -1},
				binding: *arg.Binding,
			})
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
			items = append(items, keyedBinding{
				key:     inputUsageKey{argIdx: argIdx, memberIdx: idx},
				binding: *m.Binding,
			})
		}
	}
	// Apply the same cross-argument sort as collectFlatBindings.
	// VS inputs use InputAssembler packing which preserves declaration order.
	if !isVSInput {
		sort.SliceStable(items, func(i, j int) bool {
			ki := backend.NewMemberInterfaceKey(&items[i].binding)
			kj := backend.NewMemberInterfaceKey(&items[j].binding)
			return backend.MemberInterfaceLess(ki, kj)
		})
	}
	keys := make([]inputUsageKey, len(items))
	for i, item := range items {
		keys[i] = item.key
	}
	return keys
}

// resolveInputRWMask looks up the AlwaysReadsMask for one input element.
// The 0xFF sentinel from the usage analysis means "all declared components".
// A missing key means the input is not read by the shader body at all.
func resolveInputRWMask(usedMasks map[inputUsageKey]uint8, key inputUsageKey, declaredMask uint8) uint8 {
	usageMask, found := usedMasks[key]
	if !found {
		return 0
	}
	if usageMask == 0xFF {
		return declaredMask
	}
	return usageMask & declaredMask
}

// packBindingsForSignature builds the SigElementInfo list from a flat
// binding/type pair list and returns the packed register layout.
func packBindingsForSignature(
	irMod *ir.Module,
	bindings []ir.Binding,
	types []ir.TypeHandle,
	stage ir.ShaderStage,
	isOutput bool,
	isVSInput bool,
	interpFn func(loc ir.LocationBinding) backend.SigPackInterp,
) []backend.PackedElement {
	infos := make([]backend.SigElementInfo, len(bindings))
	for i, b := range bindings {
		infos[i] = backend.SigElementInfoForBinding(irMod, b, types[i], stage, isOutput, interpFn)
		if isVSInput && infos[i].Kind == backend.SigPackLocation {
			// VSIn uses PackingKind::InputAssembler — each element on its
			// own row at col 0. Treat as system-value-like for the packer.
			infos[i].Kind = backend.SigPackBuiltinSystemValue
		}
	}
	return backend.PackSignatureElements(infos, !isOutput)
}

// buildSigElementFromPacked builds one container.SignatureElement using a
// pre-computed packed (Register, StartCol). Returns ok=false if the binding
// is not actually represented in the signature (e.g. SV_ViewID).
func buildSigElementFromPacked(
	irMod *ir.Module,
	binding ir.Binding,
	typeHandle ir.TypeHandle,
	pe backend.PackedElement,
	isOutput, isFragment bool,
) (container.SignatureElement, bool) {
	elems := bindingToSignatureElements(irMod, binding, typeHandle, pe.Register, pe.StartCol, isOutput, isFragment)
	if len(elems) == 0 {
		return container.SignatureElement{}, false
	}
	return elems[0], true
}

// bindingToSignatureElements converts a naga binding to signature elements.
// Returns nil for bindings that are NOT signature elements (compute thread
// builtins, ViewID, etc.) — those are accessed via dedicated dx.op
// intrinsics, not loadInput/storeOutput.
//
// startCol is the packed starting component within the register row (0..3).
// The Mask byte encodes WHICH register lanes the element occupies, so a
// vec2 with startCol=2 produces Mask=0x0C (bits 2,3) instead of 0x03.
func bindingToSignatureElements(irMod *ir.Module, binding ir.Binding, typeHandle ir.TypeHandle, register uint32, startCol uint8, isOutput, isFragment bool) []container.SignatureElement {
	if bb, ok := binding.(ir.BuiltinBinding); ok {
		switch bb.Builtin {
		case ir.BuiltinViewIndex:
			// SV_ViewID lives in dx.viewIdState metadata, not ISG1.
			return nil
		case ir.BuiltinSampleMask:
			// SV_Coverage on PS INPUT is NotInSig (DxilSigPoint.inl:97).
			// DXC reads it via dx.op.coverage instead. SV_Coverage on
			// output remains a system-managed signature element.
			if !isOutput && isFragment {
				return nil
			}
		}
	}
	var sem container.SemanticMapping
	if isOutput {
		sem = container.MapBindingToSemantic(binding, true, isFragment)
	} else {
		sem = container.MapBindingToSemantic(binding, false, isFragment)
	}
	// Dual-source blending: WGSL encodes the second blend source as
	// @location(0) @blend_src(1). DXC HLSL uses SV_Target0 + SV_Target1
	// at register rows 0 and 1 — the runtime then maps target 1 as the
	// dual-source alpha. Translate by treating BlendSrc as the SV_Target
	// index when present, overriding the location-based mapping.
	if lb, ok := binding.(ir.LocationBinding); ok && lb.BlendSrc != nil && isFragment && isOutput {
		sem.SemanticIndex = *lb.BlendSrc
	}

	// Derive component type and mask from the IR type.
	compType := container.CompTypeFloat32
	mask := uint8(0x0F) // xyzw default (vec4)
	if irMod != nil && int(typeHandle) < len(irMod.Types) {
		t := irMod.Types[typeHandle]
		compType, mask = sigCompTypeAndMask(irMod, t.Inner)
	}
	// Fragment shader SV_Position is always float32 vec4 regardless of type declaration.
	if sem.SystemValue == container.SVPosition && isFragment && !isOutput {
		compType = container.CompTypeFloat32
		mask = 0x0F
	}
	// Shift the component mask to the packed start column. A vec2 with
	// startCol=2 occupies register lanes z,w (mask 0x0C), not x,y. The
	// signature `Mask` byte names the actual register lanes used, so packed
	// elements need this shift. Validator regenerates the mask the same way
	// from the bitcode metadata startCol+cols, and rejects with 'DXIL
	// container mismatch for SigOutputElement' if the OSG1 byte disagrees.
	if startCol > 0 {
		mask = (mask << startCol) & 0x0F
	}

	// RWMask interpretation depends on direction:
	//   input  → AlwaysReadsMask : bits set for components the shader
	//                              always reads. We set to `mask` —
	//                              every component covered by the
	//                              element type is read.
	//   output → NeverWritesMask : bits set for components the shader
	//                              NEVER writes. For a vec3 output
	//                              packed into a 4-component register
	//                              (mask=0x7), the 4th lane is unused
	//                              so NeverWrites = 0x8. Formula:
	//                              ~mask & 0xF (only bits inside the
	//                              4-component register, not the full
	//                              uint8).
	// dxc verified via dbgsimple.hlsl OSG1 dump.
	// BUG-DXIL-022 follow-up.
	rwMask := mask
	if isOutput {
		rwMask = (^mask) & 0x0F
	}
	// System-managed pixel outputs (SV_Depth family / SV_Coverage /
	// SV_StencilRef) carry Register = 0xFFFFFFFF in OSG1, matching the
	// kUndefinedRow convention from DXC DxilSemantic.h. The validator
	// rejects 'Output Semantic SV_X should have a packing location of -1'
	// when these elements claim a real register slot.
	regField := register
	if isSystemManagedSV(sem.SystemValue) {
		regField = 0xFFFFFFFF
	}
	elem := container.SignatureElement{
		SemanticName:  sem.SemanticName,
		SemanticIndex: sem.SemanticIndex,
		SystemValue:   sem.SystemValue,
		CompType:      compType,
		Register:      regField,
		Mask:          mask,
		RWMask:        rwMask,
	}

	return []container.SignatureElement{elem}
}

// isSystemManagedSV reports whether a container SystemValueKind is one of
// the pixel-stage signature elements that carry Register = -1. Mirrors
// DXC DxilSemantic::kUndefinedRow handling and the equivalent
// isSystemManagedOutputSemantic helper in the bitcode emitter. The list
// covers both PS outputs (Depth family / Coverage / StencilRef) and PS
// inputs (SampleIndex via Shadow interpretation).
func isSystemManagedSV(sv container.SystemValueKind) bool {
	switch sv {
	case container.SVSampleIndex,
		container.SVDepth,
		container.SVDepthGreaterEqual,
		container.SVDepthLessEqual,
		container.SVCoverage,
		container.SVStencilRef:
		return true
	}
	return false
}

// sigCompTypeAndMask returns the DXIL signature component type and mask for an IR type.
func sigCompTypeAndMask(irMod *ir.Module, inner ir.TypeInner) (container.ProgSigCompType, uint8) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return sigCompTypeForScalarKind(t.Kind), 0x01
	case ir.VectorType:
		ct := sigCompTypeForScalarKind(t.Scalar.Kind)
		mask := uint8((1 << t.Size) - 1)
		return ct, mask
	case ir.ArrayType:
		// Scalar-array signature elements (SV_ClipDistance, SV_CullDistance):
		// component type from base scalar, mask covers exactly the array
		// length up to 4 (single-row signature element max). Without this
		// the default below returned (Float32, 0xF) for array<f32, 1>
		// outputs, tripping the validator with 'Container part Program
		// Output Signature does not match expected for module' because
		// the OSG1 byte mask disagreed with the bitcode/PSV0 cols=1 set
		// elsewhere by describeIRType / makePSVSignatureElement.
		if irMod == nil || int(t.Base) >= len(irMod.Types) {
			return container.CompTypeFloat32, 0x0F
		}
		baseScalar, ok := irMod.Types[t.Base].Inner.(ir.ScalarType)
		if !ok {
			return container.CompTypeFloat32, 0x0F
		}
		n := uint8(1)
		if t.Size.Constant != nil {
			c := *t.Size.Constant
			switch {
			case c >= 1 && c <= 4:
				n = uint8(c)
			case c > 4:
				n = 4
			}
		}
		mask := uint8((1 << n) - 1)
		return sigCompTypeForScalarKind(baseScalar.Kind), mask
	default:
		return container.CompTypeFloat32, 0x0F
	}
}

// sigCompTypeForScalarKind maps an IR scalar kind to a DXIL signature component type.
func sigCompTypeForScalarKind(kind ir.ScalarKind) container.ProgSigCompType {
	switch kind {
	case ir.ScalarFloat:
		return container.CompTypeFloat32
	case ir.ScalarSint:
		return container.CompTypeSint32
	case ir.ScalarUint:
		return container.CompTypeUint32
	case ir.ScalarBool:
		return container.CompTypeUint32 // bool stored as u32 in signatures
	default:
		return container.CompTypeFloat32
	}
}

// buildPSV creates pipeline state validation info for an entry point.
//
// BUG-DXIL-019: populates PSVSigInputs / PSVSigOutputs for graphics stages
// (was previously left empty, triggering the BUG-DXIL-009 self-consistency
// clamp which forced sig counts to zero, which in turn produced a PSV0 vs
// bitcode metadata mismatch reported by IDxcValidator). Trivial vertex
// shaders with only `@builtin(position)` output now produce a self-consistent
// PSV0 part with one PSVSignatureElement entry, achieving the first-ever
// `S_OK` from the real validator.
func buildPSV(irMod *ir.Module, ep *ir.EntryPoint, isFragment bool, inputCount, outputCount int, reachable map[ir.GlobalVariableHandle]bool, bindingMap BindingMap, samplerHeap *SamplerHeapBindTargets, samplerBufMap map[uint32]BindTarget) container.PSVInfo {
	_ = isFragment // kept for caller compatibility; stage comes from ep.Stage
	info := container.PSVInfo{
		ShaderStage:       stageToPSVKind(ep.Stage),
		MinWaveLaneCount:  0,
		MaxWaveLaneCount:  0xFFFFFFFF,
		SigInputElements:  uint8(inputCount),  //nolint:gosec // bounded by entry point args
		SigOutputElements: uint8(outputCount), //nolint:gosec // bounded by entry point results
	}
	// OutputPositionPresent applies only to vertex shaders with a result.
	// Fragment/compute/task have no SV_Position output here; mesh goes
	// through the buildPSVEx path.
	if ep.Stage == ir.StageVertex && ep.Function.Result != nil {
		info.OutputPositionPresent = true
	}
	// Compute shaders carry their workgroup size in PSVRuntimeInfo (same
	// NumThreads slot as mesh/amplification). BUG-DXIL-008: without this
	// dxil.dll validator rejects compute PSV0 as thread-count mismatch.
	if ep.Stage == ir.StageCompute {
		info.NumThreadsX = max(ep.Workgroup[0], 1)
		info.NumThreadsY = max(ep.Workgroup[1], 1)
		info.NumThreadsZ = max(ep.Workgroup[2], 1)
	}

	// BUG-DXIL-022: populate resource bindings. dxil.dll's
	// VerifyPSVMatches cross-checks PSV0 resource count against the
	// bitcode !dx.resources list; a stale zero fails every shader that
	// touches buffers/textures/samplers ("DXIL container mismatch for
	// 'ResourceCount' between 'PSV0' part:('0') and DXIL module:('N')").
	// Reachability filter (BUG-DXIL-016) keeps multi-EP modules in sync
	// with emit/resources.go which applies the same filter.
	info.ResourceBindings = collectPSVResources(irMod, reachable, bindingMap, samplerHeap, samplerBufMap)
	if entryUsesViewIndex(ep) {
		info.UsesViewID = true
	}
	// Pixel-shader DepthOutput byte: set when the entry point writes any
	// of the SV_Depth family. The validator regenerates this field from
	// the bitcode signature (any output element with SemanticKind=Depth/
	// DepthGreaterEqual/DepthLessEqual) and reports 'PSVRuntimeInfo
	// mismatch' if the PSV0 byte disagrees.
	if isFragment && entryWritesDepthInModule(irMod, ep) {
		info.DepthOutput = true
	}
	// BUG-DXIL-NumWG-PSV: NumWorkGroups (@builtin(num_workgroups)) is
	// implemented in our emitter as a synthetic $Globals CBV at b0/space0
	// (see dxil/internal/emit/io.go getOrCreateNumWorkGroupsCBV). The
	// bitcode side declares a real CBV resource for it; the PSV0 side must
	// declare the same binding or the validator reports 'ResourceCount
	// mismatch'. Inject the synthetic binding when the entry point reads
	// the num_workgroups builtin.
	if entryUsesNumWorkGroups(ep) {
		info.ResourceBindings = append([]container.PSVResourceBinding{{
			ResType:    container.PSVResTypeCBV,
			ResKind:    container.PSVResKindCBuffer,
			Space:      0,
			LowerBound: 0,
			UpperBound: 0,
		}}, info.ResourceBindings...)
	}

	// BUG-DXIL-019: build PSVSignatureElement entries from the same data
	// that BUG-DXIL-012 uses for `!dx.entryPoints[0][2]` bitcode metadata.
	// The two must agree byte-for-byte or the validator reports
	// "PSVRuntimeInfo content mismatch".
	sigs := buildGraphicsPSVSigs(irMod, ep)
	// Pixel shader SampleFrequency bit is 1 when any input signature
	// element uses sample-frequency interpolation or is SV_SampleIndex.
	// Mirrors hlsl::SetShaderProps in
	// lib/DxilContainer/DxilPipelineStateValidation.cpp:216 — DXC walks
	// the input signature elements and sets PS.SampleFrequency = 1 when
	// IsAnySample() or SemanticKind::SampleIndex. We walk the same
	// PSVInputs we just built (they already carry the interp mode we
	// computed in psvInterpolationMode), so the container byte agrees
	// byte-for-byte with what the validator regenerates from the
	// equivalent bitcode signature metadata.
	//
	// DXIL InterpolationMode enum values for sample variants:
	//   6 = LinearSample                (perspective sample)
	//   7 = LinearNoperspectiveSample   (noperspective sample, WGSL 'linear, sample')
	// PSV SemanticKind for SV_SampleIndex is 12 (see dxil.go:676).
	if ep.Stage == ir.StageFragment {
		for _, elem := range sigs.PSVInputs {
			if elem.InterpolationMode == 6 || elem.InterpolationMode == 7 || elem.SemanticKind == 12 {
				info.SampleFrequency = true
				break
			}
		}
	}
	if len(sigs.PSVInputs) > 0 || len(sigs.PSVOutputs) > 0 {
		info.StringTable = sigs.StringTable
		info.SemanticIndexTable = sigs.SemIndices
		info.PSVSigInputs = sigs.PSVInputs
		info.PSVSigOutputs = sigs.PSVOutputs
		info.SigInputVectors = sigs.SigInputVectors
		info.SigOutputVectors = sigs.SigOutputVectorsStream0 // GS streams 1-3 are zero (no GS support yet)
		// EntryFunctionName: offset of entry name in the new string table.
		// buildGraphicsPSVSigs prepends "\0" then "<entryName>\0" so the
		// entry name lives at offset 1 (immediately after the empty string).
		info.EntryFunctionName = 1

		// BUG-DXIL-018 Phase 3: fill the per-input-component → output-
		// component dependency table. Must agree byte-for-byte with the
		// bitcode `dx.viewIdState` regenerated content or dxil.dll's
		// content-verifier reports 'ViewIDState mismatch'.
		info.InputToOutputTable = buildInputToOutputTable(irMod, ep, sigs)
		return info
	}

	// Fallback: no graphics signatures, use minimal string table with just
	// the entry function name (BUG-DXIL-009 path).
	info.StringTable, info.EntryFunctionName = container.BuildPSVStringTable(ep.Name)
	return info
}

// buildGraphicsPSVSigs walks an entry point's arguments and result and
// constructs PSV signature element entries + matching string/semantic-index
// tables for graphics stages (vertex, pixel, hull, domain, geometry).
//
// Mirror of buildMeshPSVSigOutputs but for non-mesh graphics stages.
//
// Returns:
//   - stringTable: leading "\0", entry name, semantic names (4-byte aligned)
//   - semIndices: per-element semantic index values (each is uint32)
//   - psvInputs / psvOutputs: signature elements for input args / result
//   - sigInputVectors / sigOutputVectors[0]: number of register rows used
//
// graphicsPSVSigs holds the result of walking an entry point's signatures
// for PSV0 emission. Returned as a struct (not multiple values) to satisfy
// gocritic's tooManyResultsChecker.
type graphicsPSVSigs struct {
	StringTable             []byte
	SemIndices              []uint32
	PSVInputs               []container.PSVSignatureElement
	PSVOutputs              []container.PSVSignatureElement
	SigInputVectors         uint8
	SigOutputVectorsStream0 uint8 // GS streams 1-3 not supported yet
}

//nolint:gocognit,nestif,gocyclo,cyclop,funlen // signature element construction requires deep type inspection
func buildGraphicsPSVSigs(irMod *ir.Module, ep *ir.EntryPoint) graphicsPSVSigs {
	var (
		stringTable             []byte
		semIndices              []uint32
		psvInputs               []container.PSVSignatureElement
		psvOutputs              []container.PSVSignatureElement
		sigInputVectors         uint8
		sigOutputVectorsStream0 uint8
		inputRow                uint8
		outputRow               uint8
	)
	// String table starts with "\0" (empty string at offset 0 used for
	// SV_* names) followed by the entry function name at offset 1.
	// BUG-DXIL-022 follow-up: do NOT 4-byte align here. Padding nulls
	// inserted in the middle of the table get tokenized by dxil.dll's
	// StringTableVerifier (see DxilContainerValidation.cpp:85-126) as
	// extra empty-string entries with use-count zero, triggering
	// "In 'StringTable', '<null>' is not used". Alignment is done once
	// at the very end of the table, after every appendName call.
	stringTable = append(stringTable, 0)
	stringTable = append(stringTable, []byte(ep.Name)...)
	stringTable = append(stringTable, 0)
	alignStringTable := func() {
		for len(stringTable)%4 != 0 {
			stringTable = append(stringTable, 0)
		}
	}

	// appendName adds a C-string to the string table and returns its
	// offset. Caller is responsible for re-aligning at the end of the
	// table — alignment matters only for the final on-disk layout.
	appendName := func(name string) uint32 {
		off := uint32(len(stringTable)) //nolint:gosec // string table grows monotonically with element count
		stringTable = append(stringTable, []byte(name)...)
		stringTable = append(stringTable, 0)
		return off
	}

	// addElement builds the PSV element using a pre-packed (Register, StartCol)
	// from the packer. The row counter is also tracked so SigInput/Output
	// Vectors covers the highest used row + element rows. System-managed
	// elements (Register=0xFFFFFFFF) do not contribute to the row counter.
	addElement := func(binding ir.Binding, typeHandle ir.TypeHandle, isOutput bool, pe backend.PackedElement, row *uint8) (container.PSVSignatureElement, bool) {
		sem, ok := psvSemanticForBinding(binding, ep.Stage, isOutput)
		if !ok {
			return container.PSVSignatureElement{}, false
		}
		// SV_* system-value semantics resolve via SemanticKind and use
		// SemanticNameOffset = 0 (empty string). Only ARBITRARY
		// semantics (TEXCOORD and friends, kind=0) need an actual
		// string in the PSV0 string table. Mirrors DXC's convention
		// in DxilContainerAssembler::WritePSVOData (the PSV string
		// table contains only user-defined names + the entry
		// function name). BUG-DXIL-011 Phase 2.
		nameOff := uint32(0)
		if sem.Name != "" && sem.Kind == 0 {
			nameOff = appendName(sem.Name)
		}
		// Deduplicate semantic index entries: DXC shares a single
		// SemanticIndexesOffset between every element whose index
		// list is equal (most commonly, all-zero for SigElement with
		// numRows=1). Our prior append-every-time form produced
		// len(semIndices) == numElements and let the validator's
		// byte-equality check on PSV0 disagree with DXC's canonical
		// shape even though the per-element semantics matched.
		semIdxOff := uint32(0)
		foundDup := false
		for scan := 0; scan+1 <= len(semIndices); scan++ {
			if semIndices[scan] == sem.Index {
				semIdxOff = uint32(scan)
				foundDup = true
				break
			}
		}
		if !foundDup {
			semIdxOff = uint32(len(semIndices)) //nolint:gosec // index bounded by total binding count
			semIndices = append(semIndices, sem.Index)
		}
		elem := makePSVSignatureElement(irMod, typeHandle, sem, nameOff, semIdxOff, uint8(pe.Register), pe.StartCol) //nolint:gosec // packed row fits uint8
		// System-managed pixel outputs do NOT consume an output register
		// row — they are written via dedicated dx.op calls (storeDepth /
		// storeCoverage / storeStencilRef) and the PSV0 SigOutputVectors
		// counter must skip over them. Otherwise PSV0 reports
		// SigOutputVectors[0]=1 while the bitcode regenerator computes 0
		// from the element's IsAllocated=false flag, tripping
		// 'PSVRuntimeInfo mismatch'.
		if !isPSVSemanticSystemManaged(sem.Kind) {
			top := uint8(pe.Register) + pe.Rows //nolint:gosec // packer caps to single byte for graphics signatures
			if top > *row {
				*row = top
			}
		}
		return elem, true
	}

	stage := ep.Stage
	isVSInput := stage == ir.StageVertex
	interpFnIn := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(psvInterpolationMode(loc.Interpolation, ep.Stage == ir.StageFragment, false))
	}
	interpFnOut := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(psvInterpolationMode(loc.Interpolation, ep.Stage == ir.StageFragment, true))
	}

	// Walk arguments (inputs).
	{
		bindings, types := collectFlatBindings(irMod, ep.Function.Arguments, nil, stage, false)
		packed := packBindingsForSignature(irMod, bindings, types, stage, false, isVSInput, interpFnIn)
		for i, b := range bindings {
			pe := packed[i]
			if pe.Rows == 0 {
				continue
			}
			elem, ok := addElement(b, types[i], false, pe, &inputRow)
			if !ok {
				continue
			}
			psvInputs = append(psvInputs, elem)
		}
	}

	// Walk result (outputs).
	if ep.Function.Result != nil {
		res := ep.Function.Result
		if res.Binding != nil {
			pe := backend.PackedElement{Register: 0, StartCol: 0, ColCount: 4, Rows: 1}
			elem, ok := addElement(*res.Binding, res.Type, true, pe, &outputRow)
			if ok {
				psvOutputs = append(psvOutputs, elem)
			}
		} else if int(res.Type) < len(irMod.Types) {
			if st, ok := irMod.Types[res.Type].Inner.(ir.StructType); ok {
				packed := backend.PackStructMembers(irMod, st.Members, stage, true, false, interpFnOut)
				for _, pm := range packed {
					if !pm.HasBinding || pm.Rows == 0 {
						continue
					}
					m := st.Members[pm.OrigIdx]
					elem, ok := addElement(*m.Binding, m.Type, true, pm.PackedElement, &outputRow)
					if !ok {
						continue
					}
					psvOutputs = append(psvOutputs, elem)
				}
			}
		}
	}

	// Re-align string table after all arbitrary names have been added.
	alignStringTable()

	// SigInputVectors / SigOutputVectors[0] = number of register rows used.
	// Packed elements may share rows; the row counter tracks the highest
	// (Register + Rows) of any non-system-managed element.
	sigInputVectors = inputRow
	sigOutputVectorsStream0 = outputRow
	return graphicsPSVSigs{
		StringTable:             stringTable,
		SemIndices:              semIndices,
		PSVInputs:               psvInputs,
		PSVOutputs:              psvOutputs,
		SigInputVectors:         sigInputVectors,
		SigOutputVectorsStream0: sigOutputVectorsStream0,
	}
}

// psvSemantic describes the semantic side of a PSV signature element:
// kind enum, optional semantic name (empty for SV_* system values),
// semantic index value, and interpolation mode. Pure data — no string
// table mutation, caller wires up name offset and semantic-index offset.
type psvSemantic struct {
	Kind          uint8  // DXC SemanticKind enum
	Name          string // empty for SV_* (kind-based dispatch); non-empty for arbitrary (TEXCOORD, etc.) and SV_Target
	Index         uint32 // value written at the semantic-index table slot
	Interpolation uint8  // DXIL InterpolationMode enum
}

// psvSemanticForBinding maps a (binding, stage, isOutput) triple to its
// PSV semantic encoding. Returns ok=false for bindings that are unrelated
// to the input/output signature (e.g. compute builtins like
// LocalInvocationID, which live outside the I/O signature entirely).
//
// SemanticKind values are from DXC DxilConstants.h `enum class SemanticKind`:
//
//	0=Arbitrary, 1=VertexID, 2=InstanceID, 3=Position, 6=ClipDistance,
//	7=CullDistance, 12=SampleIndex, 13=IsFrontFace, 14=Coverage,
//	16=Target, 17=Depth.
//
// InterpolationMode values are from DXC DxilConstants.h `enum class
// InterpolationMode`:
//
//	0=Undefined, 1=Constant, 2=Linear, 3=LinearCentroid,
//	4=LinearNoperspective, 5=LinearNoperspectiveCentroid,
//	6=LinearSample, 7=LinearNoperspectiveSample.
func psvSemanticForBinding(binding ir.Binding, stage ir.ShaderStage, isOutput bool) (psvSemantic, bool) {
	if bb, ok := binding.(ir.BuiltinBinding); ok {
		// SV_Coverage on PS input is NotInSig (DxilSigPoint.inl:97). Skip
		// from PSV0 sig element list to mirror the OSG1 / bitcode sides.
		if bb.Builtin == ir.BuiltinSampleMask && stage == ir.StageFragment && !isOutput {
			return psvSemantic{}, false
		}
		sem, ok := psvSemanticForBuiltin(bb.Builtin)
		if !ok {
			return sem, false
		}
		sem.Interpolation = sigInterpModeForBuiltin(bb.Builtin, stage, isOutput)
		return sem, true
	}
	if loc, ok := binding.(ir.LocationBinding); ok {
		return psvSemanticForLocation(loc, stage, isOutput), true
	}
	return psvSemantic{}, false
}

// sigInterpModeForBuiltin returns the DXIL InterpolationMode for a
// system-value sig element. Mirrored on the bitcode side as
// sigInterpForBuiltin in emit/emitter.go. Verified against dxc -dumpbin
// output for vs_flat_out.hlsl and sv_position_in_out.hlsl.
//
// Rule from dxc reference:
//
//	stage    direction  →  interp
//	------   ---------     -----------
//	vertex   in            0 (Undefined)        — vertex inputs come from buffer
//	vertex   out           per-builtin (4 for SV_Position, else 0/2)
//	fragment in            per-builtin (4 for SV_Position, else 0/2)
//	fragment out           0 (Undefined)        — SV_Target / SV_Depth etc.
//	compute  *             0
func sigInterpModeForBuiltin(b ir.BuiltinValue, stage ir.ShaderStage, isOutput bool) uint8 {
	isFragment := stage == ir.StageFragment
	isVertex := stage == ir.StageVertex
	carries := (isVertex && isOutput) || (isFragment && !isOutput)
	if !carries {
		return 0
	}
	switch b {
	case ir.BuiltinPosition:
		return 4 // NoPerspective
	case ir.BuiltinClipDistance:
		return 2 // Linear (perspective)
	default:
		// VertexID, InstanceID, FrontFacing, etc. don't actually appear
		// here for the carrier directions, but emit Undefined as a safe
		// default if they do.
		return 0
	}
}

// psvSemanticForBuiltin maps a WGSL builtin to its PSV semantic encoding.
// Returns ok=false for builtins that do not appear in the input/output
// signature (compute thread-id family, mesh-only builtins, etc.).
func psvSemanticForBuiltin(b ir.BuiltinValue) (psvSemantic, bool) {
	switch b {
	case ir.BuiltinPosition:
		// Interpolation defaults to 0 (Undefined) per dxc rule. Direction-
		// specific interp is applied by the caller for fragment inputs in
		// psvSemanticForLocation; system-value builtins always emit
		// Interpolation=0 here. SV_Position fragment input still maps to
		// noperspective in the bitcode-side mapping for SV_Position outputs
		// from vertex stages — but PSV0 sig element interp for SV_Position
		// is 0 (Undefined) per dxc reference (verified via -dumpbin on
		// trivial vertex shader).
		return psvSemantic{Kind: 3, Name: "SV_Position"}, true
	case ir.BuiltinVertexIndex:
		return psvSemantic{Kind: 1, Name: "SV_VertexID"}, true
	case ir.BuiltinInstanceIndex:
		return psvSemantic{Kind: 2, Name: "SV_InstanceID"}, true
	case ir.BuiltinFragDepth:
		return psvSemantic{Kind: 17, Name: "SV_Depth"}, true
	case ir.BuiltinFrontFacing:
		return psvSemantic{Kind: 13, Name: "SV_IsFrontFace"}, true
	case ir.BuiltinSampleIndex:
		return psvSemantic{Kind: 12, Name: "SV_SampleIndex"}, true
	case ir.BuiltinSampleMask:
		return psvSemantic{Kind: 14, Name: "SV_Coverage"}, true
	case ir.BuiltinClipDistance:
		return psvSemantic{Kind: 6, Name: "SV_ClipDistance"}, true
	case ir.BuiltinPrimitiveIndex:
		return psvSemantic{Kind: 10, Name: "SV_PrimitiveID"}, true
	case ir.BuiltinViewIndex:
		// SV_ViewID is NOT a signature element — read via dx.op.viewID()
		// intrinsic, not loadInput. Skip from ISG1/OSG1.
		return psvSemantic{}, false
	default:
		// Compute thread-id family (LocalInvocationID, GlobalInvocationID,
		// WorkGroupID, NumWorkGroups, etc.) and mesh-only builtins are
		// not signature elements.
		return psvSemantic{}, false
	}
}

// psvSemanticForLocation maps a WGSL @location binding to its PSV semantic
// encoding. Inputs and non-fragment outputs become arbitrary "TEXCOORD"
// semantics (the canonical user-defined name). Fragment outputs become
// "SV_Target" with kind=Target — DXC's signature validator special-cases
// this name.
func psvSemanticForLocation(loc ir.LocationBinding, stage ir.ShaderStage, isOutput bool) psvSemantic {
	if isOutput && stage == ir.StageFragment {
		// Fragment color outputs.
		idx := loc.Location
		// Dual-source blending: @blend_src(N) overrides @location for
		// the SV_Target index. Match the bitcode side rewrite in
		// makeSigInfo / bindingToSignatureElements.
		if loc.BlendSrc != nil {
			idx = *loc.BlendSrc
		}
		return psvSemantic{
			Kind:          16, // SemanticKind::Target
			Name:          "SV_Target",
			Index:         idx,
			Interpolation: 0, // Undefined for outputs
		}
	}
	// Vertex output / fragment input direction matters: vertex outputs
	// without an explicit @interpolate attribute default to Undefined
	// (matches dxc). Fragment inputs default to Linear perspective.
	isFragment := stage == ir.StageFragment
	return psvSemantic{
		Kind:          0, // SemanticKind::Arbitrary
		Name:          backend.LocationSemantic,
		Index:         loc.Location,
		Interpolation: psvInterpolationMode(loc.Interpolation, isFragment, isOutput),
	}
}

// psvInterpolationMode maps a naga Interpolation specification to the
// DXIL InterpolationMode enum value. Verified against dxc reference
// (vs_flat_out.hlsl + sv_position_in_out.hlsl):
//
//	stage    direction  →  carries interp?
//	------   ---------     ---------------
//	vertex   in            no (vertex buffer is not interpolated)
//	vertex   out           yes (next stage will interpolate)
//	fragment in            yes (raster output)
//	fragment out           no (SV_Target / SV_Depth)
//	compute  *             no
//
// When a side does NOT carry interp, the field MUST be Undefined (0)
// or the validator emits 'Interpolation mode for X is set but should
// be undefined'. Vertex output and fragment input MUST match — they
// describe the same data flowing across the rasterizer.
func psvInterpolationMode(interp *ir.Interpolation, isFragment, isOutput bool) uint8 {
	isVertex := !isFragment
	carries := (isVertex && isOutput) || (isFragment && !isOutput)
	if !carries {
		return 0
	}
	if interp == nil {
		return 2 // Linear (perspective center, default)
	}
	switch interp.Kind {
	case ir.InterpolationFlat:
		return 1 // Constant
	case ir.InterpolationLinear:
		// "linear" in WGSL == HLSL noperspective.
		switch interp.Sampling {
		case ir.SamplingCentroid:
			return 5 // LinearNoperspectiveCentroid
		case ir.SamplingSample:
			return 7 // LinearNoperspectiveSample
		default:
			return 4 // LinearNoperspective
		}
	case ir.InterpolationPerspective:
		switch interp.Sampling {
		case ir.SamplingCentroid:
			return 3 // LinearCentroid
		case ir.SamplingSample:
			return 6 // LinearSample
		default:
			return 2 // Linear
		}
	}
	return 2
}

// makePSVSignatureElement constructs a single container.PSVSignatureElement
// from its resolved type information and pre-computed semantic metadata.
//
// Component type encoding is the PSV-specific enum (NOT the bitcode
// metadata CompType). PSV uses:
//
//	0=Unknown, 1=UInt32, 2=SInt32, 3=Float32, ...
//
// while bitcode metadata uses:
//
//	0=Invalid, 1=I1, ..., 4=I32, 5=U32, ..., 9=F32, ...
//
// BUG-DXIL-012 emits the bitcode metadata version; this function emits
// the PSV version. Mismatching them looks like a bug but isn't.
//
//nolint:nestif // single dispatch table over IR type kinds for PSV cols
func makePSVSignatureElement(irMod *ir.Module, typeHandle ir.TypeHandle, sem psvSemantic, semNameOffset, semIdxOffset uint32, startRow, startCol uint8) container.PSVSignatureElement {
	cols := uint8(4)     // default vec4
	compType := uint8(3) // PSV Float32
	if int(typeHandle) < len(irMod.Types) {
		switch ti := irMod.Types[typeHandle].Inner.(type) {
		case ir.VectorType:
			cols = uint8(ti.Size)
			compType = psvComponentType(ti.Scalar.Kind)
		case ir.ScalarType:
			cols = 1
			compType = psvComponentType(ti.Kind)
		case ir.ArrayType:
			// Scalar-array signature elements (SV_ClipDistance,
			// SV_CullDistance) — element kind from base scalar, channel
			// count from constant array size, capped at 4 for a single
			// row. Mirrors describeIRType in dxil/internal/emit/emitter.go;
			// keeping the two paths in sync is required because the
			// validator memcmp's PSV0 cols against the bitcode metadata
			// regenerated cols. clip-distances.wgsl tripped 'Container
			// part Program Output Signature does not match expected for
			// module' when only one side knew about array elements.
			if int(ti.Base) < len(irMod.Types) {
				if baseScalar, ok := scalarOfPSVType(irMod.Types[ti.Base].Inner); ok {
					compType = psvComponentType(baseScalar.Kind)
					n := uint8(1)
					if ti.Size.Constant != nil {
						c := *ti.Size.Constant
						if c >= 1 && c <= 4 {
							n = uint8(c)
						} else if c > 4 {
							n = 4
						}
					}
					cols = n
				}
			}
		}
	}

	// SV_IsFrontFace is a special case: WGSL types it as bool but DXIL
	// materializes it as UInt32 (the DXC bool-input lowering emits
	// loadInput.i32 + icmp ne 0). The validator regenerates the PSV
	// ComponentType as UInt32 (=1 in the PSV enum: Float32=3, UInt32=1,
	// SInt32=2) from DXC's DxilSignatureElement defaulting rules; our
	// bool-derived default-branch value (3=Float32) trips 'DXIL
	// container mismatch for SigInputElement' even though every other
	// field matches. Mirrors the BuiltinFrontFacing override in
	// makeSigInfo (ISG1 side) — the two containers MUST agree.
	if sem.Kind == 13 { // IsFrontFace
		compType = 1 // UInt32 in the PSV ComponentType enum
	}

	// ColsAndStart: bits 0:4=Cols, bits 4:6=StartCol, bit 6=Allocated.
	// StartCol packs into 2 bits (max value 3 = column w). Caller must clamp.
	colsAndStart := cols | ((startCol & 0x03) << 4) | (1 << 6)
	row := startRow
	// System-managed pixel outputs (SV_Depth family / SV_Coverage /
	// SV_StencilRef) carry IsAllocated=0 in PSV0. DXC's
	// PSVSignatureElement::IsAllocated returns false for these, and the
	// validator regenerates the element with bit 6 cleared and StartRow
	// = 0xFF. Rows stays at 1 (the element STILL has 1 row's worth of
	// data, it's just not packed into the output register file). Mirror
	// that here so the PSV0 part matches the OSG1 container's
	// Register=0xFFFFFFFF and the bitcode metadata's startRow=-1.
	if isPSVSemanticSystemManaged(sem.Kind) {
		colsAndStart = cols // clear allocated bit
		row = 0xFF
	}

	return container.PSVSignatureElement{
		SemanticNameOffset:    semNameOffset,
		SemanticIndexesOffset: semIdxOffset,
		Rows:                  1,
		StartRow:              row,
		ColsAndStart:          colsAndStart,
		SemanticKind:          sem.Kind,
		ComponentType:         compType,
		InterpolationMode:     sem.Interpolation,
	}
}

// isPSVSemanticSystemManaged mirrors isSystemManagedSV but operates on the
// PSV-side SemanticKind enum (DXIL semantic kind, not D3D_NAME). PSV uses
// the ABI-locked enum from DxilConstants.h enum class SemanticKind:
//
//	12 = SampleIndex, 14 = Coverage, 17 = Depth, 18 = DepthLessEqual,
//	19 = DepthGreaterEqual, 20 = StencilRef
func isPSVSemanticSystemManaged(kind uint8) bool {
	switch kind {
	case 12, 14, 17, 18, 19, 20:
		return true
	}
	return false
}

// scalarOfPSVType extracts a ScalarType from an ArrayType element for the
// PSV0 signature element builder. Mirrors dxil/internal/emit/types.go
// scalarOfType but lives here to avoid cross-package import cycles.
func scalarOfPSVType(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	}
	return ir.ScalarType{}, false
}

// psvComponentType maps a naga ScalarKind to the PSV ComponentType enum
// value (NOT the bitcode metadata CompType — see makePSVSignatureElement).
func psvComponentType(k ir.ScalarKind) uint8 {
	switch k {
	case ir.ScalarFloat:
		return 3 // Float32
	case ir.ScalarUint:
		return 1 // UInt32
	case ir.ScalarSint:
		return 2 // SInt32
	default:
		return 3
	}
}

// buildSignaturesEx extracts input, output, and primitive output signature
// elements from an entry point. Mesh shaders have vertex outputs in OSG1
// and primitive outputs in PSG1.
//
//nolint:gocognit,nestif // mesh output signature extraction requires deep type inspection
func buildSignaturesEx(irMod *ir.Module, ep *ir.EntryPoint, isFragment, isMesh bool) ([]container.SignatureElement, []container.SignatureElement, []container.SignatureElement) {
	// Compute and amplification have no I/O signatures (BUG-DXIL-021
	// follow-up). Their kernel arguments (WorkgroupId / LocalInvocationID
	// / DispatchThreadID / payload) are accessed via dedicated dx.op
	// intrinsics inside the function body, not via ISG1/OSG1 signature
	// elements. dxc emits empty ISG1/OSG1 (count=0, offset=8) — we match.
	if ep.Stage == ir.StageCompute || ep.Stage == ir.StageTask {
		return nil, nil, nil
	}
	if !isMesh {
		inputs, outputs := buildSignatures(irMod, ep, isFragment)
		return inputs, outputs, nil
	}

	// Mesh shader: inputs from function arguments (compute-style builtins, no signature elements).
	var inputs []container.SignatureElement

	// Vertex outputs from MeshStageInfo.VertexOutputType.
	var outputs []container.SignatureElement
	if ep.MeshInfo != nil && int(ep.MeshInfo.VertexOutputType) < len(irMod.Types) {
		vtxType := irMod.Types[ep.MeshInfo.VertexOutputType]
		if st, ok := vtxType.Inner.(ir.StructType); ok {
			outReg := uint32(0)
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				elem := meshMemberToSignatureElement(irMod, *member.Binding, member.Type, outReg, false)
				outputs = append(outputs, elem)
				outReg++
			}
		}
	}

	// Primitive outputs from MeshStageInfo.PrimitiveOutputType.
	var primOutputs []container.SignatureElement
	if ep.MeshInfo != nil && int(ep.MeshInfo.PrimitiveOutputType) < len(irMod.Types) {
		primType := irMod.Types[ep.MeshInfo.PrimitiveOutputType]
		if st, ok := primType.Inner.(ir.StructType); ok {
			primReg := uint32(0)
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				// Skip triangle_indices — handled by EmitIndices intrinsic, not a signature element.
				if bb, isBB := (*member.Binding).(ir.BuiltinBinding); isBB {
					if bb.Builtin == ir.BuiltinTriangleIndices {
						continue
					}
				}
				elem := meshMemberToSignatureElement(irMod, *member.Binding, member.Type, primReg, true)
				primOutputs = append(primOutputs, elem)
				primReg++
			}
		}
	}

	return inputs, outputs, primOutputs
}

// meshMemberToSignatureElement converts a struct member binding to a signature element.
func meshMemberToSignatureElement(irMod *ir.Module, binding ir.Binding, typeHandle ir.TypeHandle, register uint32, isPrimitive bool) container.SignatureElement {
	_ = isPrimitive
	sem := container.MapBindingToSemantic(binding, true, false)

	// Determine component type and mask from the actual type.
	compType := container.CompTypeFloat32
	mask := uint8(0x0F) // xyzw default

	if int(typeHandle) < len(irMod.Types) {
		irType := irMod.Types[typeHandle]
		switch ti := irType.Inner.(type) {
		case ir.ScalarType:
			compType = scalarToCompType(ti)
			mask = 0x01
		case ir.VectorType:
			compType = scalarToCompType(ti.Scalar)
			mask = componentMask(int(ti.Size))
		}
	}

	// For output signatures, RWMask = NeverWritesMask (0 = all components may be written).
	// For input signatures, RWMask = AlwaysReadsMask (mask of always-read components).
	return container.SignatureElement{
		SemanticName:  sem.SemanticName,
		SemanticIndex: sem.SemanticIndex,
		SystemValue:   sem.SystemValue,
		CompType:      compType,
		Register:      register,
		Mask:          mask,
		RWMask:        0, // output: NeverWritesMask
	}
}

// scalarToCompType maps a scalar type to its signature component type.
func scalarToCompType(s ir.ScalarType) container.ProgSigCompType {
	switch s.Kind {
	case ir.ScalarFloat:
		return container.CompTypeFloat32
	case ir.ScalarUint, ir.ScalarBool:
		return container.CompTypeUint32
	case ir.ScalarSint:
		return container.CompTypeSint32
	default:
		return container.CompTypeFloat32
	}
}

// componentMask returns a component mask for the given number of components.
func componentMask(n int) uint8 {
	switch n {
	case 1:
		return 0x01
	case 2:
		return 0x03
	case 3:
		return 0x07
	case 4:
		return 0x0F
	default:
		return 0x0F
	}
}

// buildPSVEx creates PSV info for an entry point, including mesh shader support.
func buildPSVEx(irMod *ir.Module, ep *ir.EntryPoint, isFragment, isMesh bool, inputCount, outputCount, primOutputCount int, reachable map[ir.GlobalVariableHandle]bool, bindingMap BindingMap, samplerHeap *SamplerHeapBindTargets, samplerBufMap map[uint32]BindTarget) container.PSVInfo {
	if !isMesh {
		return buildPSV(irMod, ep, isFragment, inputCount, outputCount, reachable, bindingMap, samplerHeap, samplerBufMap)
	}

	info := container.PSVInfo{
		ShaderStage:                 stageToPSVKind(ep.Stage),
		MinWaveLaneCount:            0,
		MaxWaveLaneCount:            0xFFFFFFFF,
		SigInputElements:            uint8(inputCount),      //nolint:gosec // bounded by entry point args
		SigOutputElements:           uint8(outputCount),     //nolint:gosec // bounded by mesh vertex outputs
		SigPatchConstOrPrimElements: uint8(primOutputCount), //nolint:gosec // bounded by mesh primitive outputs
		ResourceBindings:            collectPSVResources(irMod, reachable, bindingMap, samplerHeap, samplerBufMap),
	}

	if ep.MeshInfo != nil {
		mi := ep.MeshInfo
		info.MaxOutputVertices = uint16(mi.MaxVertices)     //nolint:gosec // bounded by mesh spec
		info.MaxOutputPrimitives = uint16(mi.MaxPrimitives) //nolint:gosec // bounded by mesh spec
		switch mi.Topology {
		case ir.MeshTopologyLines:
			info.MeshOutputTopology = 1
		case ir.MeshTopologyTriangles:
			info.MeshOutputTopology = 2
		}
	}

	// NumThreads from workgroup size (minimum 1 per axis).
	info.NumThreadsX = max(ep.Workgroup[0], 1)
	info.NumThreadsY = max(ep.Workgroup[1], 1)
	info.NumThreadsZ = max(ep.Workgroup[2], 1)

	// SigOutputVectors: number of registers (rows) used by vertex outputs.
	// Each output element uses one register.
	if outputCount > 0 {
		info.SigOutputVectors = uint8(outputCount) //nolint:gosec // bounded by mesh outputs
	}

	// Build PSV string table and signature elements for outputs.
	// PSV requires matching PSVSignatureElement entries for each ISG1/OSG1 element.
	info.StringTable, info.SemanticIndexTable, info.PSVSigOutputs = buildMeshPSVSigOutputs(irMod, ep, outputCount)

	// EntryFunctionName: offset into string table where the entry point name starts.
	// String table starts with "\0" at offset 0, entry name at offset 1.
	if len(info.StringTable) > 1 {
		info.EntryFunctionName = 1 // offset of entry name after initial "\0"
	}

	return info
}

// buildMeshPSVSigOutputs builds PSV string table, semantic index table,
// and PSV signature element entries for mesh shader vertex outputs.
//
//nolint:gocognit,nestif // PSV signature element construction requires deep type inspection
func buildMeshPSVSigOutputs(irMod *ir.Module, ep *ir.EntryPoint, outputCount int) ([]byte, []uint32, []container.PSVSignatureElement) {
	if ep.MeshInfo == nil || outputCount == 0 {
		return nil, nil, nil
	}

	// Build string table: starts with "\0" (empty for system value names), then entry point name.
	var stringTable []byte
	stringTable = append(stringTable, 0) // offset 0: empty string for SV_ names
	epNameOffset := len(stringTable)
	stringTable = append(stringTable, []byte(ep.Name)...)
	stringTable = append(stringTable, 0) // null terminator
	_ = epNameOffset

	// 4-byte align
	for len(stringTable)%4 != 0 {
		stringTable = append(stringTable, 0)
	}

	// Build semantic index table: one entry per output element.
	semIndices := make([]uint32, outputCount)
	// All indices are 0 for now (single-element semantics).

	// Build PSV signature elements for vertex outputs.
	var psvOutputs []container.PSVSignatureElement

	if int(ep.MeshInfo.VertexOutputType) < len(irMod.Types) {
		vtxType := irMod.Types[ep.MeshInfo.VertexOutputType]
		if st, ok := vtxType.Inner.(ir.StructType); ok {
			semIdxOffset := uint32(0)
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}

				// Determine PSV semantic kind and component info.
				semKind := uint8(0) // arbitrary
				interpMode := uint8(0)
				if bb, isBB := (*member.Binding).(ir.BuiltinBinding); isBB {
					switch bb.Builtin {
					case ir.BuiltinPosition:
						semKind = 3    // PSV SV_Position
						interpMode = 4 // noperspective
					}
				}

				cols := uint8(4)     // default vec4
				compType := uint8(3) // float32
				if int(member.Type) < len(irMod.Types) {
					memberType := irMod.Types[member.Type]
					switch ti := memberType.Inner.(type) {
					case ir.VectorType:
						cols = uint8(ti.Size)
						switch ti.Scalar.Kind {
						case ir.ScalarUint:
							compType = 1
						case ir.ScalarSint:
							compType = 2
						}
					case ir.ScalarType:
						cols = 1
						switch ti.Kind {
						case ir.ScalarUint:
							compType = 1
						case ir.ScalarSint:
							compType = 2
						}
					}
				}

				// ColsAndStart: bits 0-3=cols, bits 4-5=startCol, bit 6=allocated
				colsAndStart := cols | (1 << 6) // allocated=1

				psvOutputs = append(psvOutputs, container.PSVSignatureElement{
					SemanticNameOffset:    0,            // empty string for SV_ names
					SemanticIndexesOffset: semIdxOffset, // index into semantic index table
					Rows:                  1,
					StartRow:              0,
					ColsAndStart:          colsAndStart,
					SemanticKind:          semKind,
					ComponentType:         compType,
					InterpolationMode:     interpMode,
				})
				semIdxOffset++
			}
		}
	}

	return stringTable, semIndices, psvOutputs
}

// stageToContainerKind maps naga ShaderStage to the DXIL shader kind
// value used in the DXBC container program header.
func stageToContainerKind(stage ir.ShaderStage) uint32 {
	switch stage {
	case ir.StageVertex:
		return uint32(module.VertexShader)
	case ir.StageFragment:
		return uint32(module.PixelShader)
	case ir.StageCompute:
		return uint32(module.ComputeShader)
	case ir.StageMesh:
		return uint32(module.MeshShader)
	case ir.StageTask:
		return uint32(module.AmplificationShader)
	default:
		return uint32(module.VertexShader)
	}
}

// entryWritesDepth reports whether any output binding of the entry point
// (top-level result or struct member) carries one of the SV_Depth family
// builtins. Used to set PSVInfo.DepthOutput so the validator's regenerated
// PSVRuntimeInfo matches the bitcode signature.
func entryWritesDepth(ep *ir.EntryPoint) bool {
	if ep == nil || ep.Function.Result == nil {
		return false
	}
	check := func(b ir.Binding) bool {
		bb, ok := b.(ir.BuiltinBinding)
		if !ok {
			return false
		}
		switch bb.Builtin {
		case ir.BuiltinFragDepth:
			return true
		}
		return false
	}
	return entryResultBindingMatches(ep, check)
}

// entryResultBindingMatches walks an entry point's result binding (top-
// level OR every struct member) looking for any binding that satisfies
// `match`. Used by entryWritesDepth and any future PSV0 flag detector
// that needs to recognize a builtin output regardless of struct nesting.
func entryResultBindingMatches(ep *ir.EntryPoint, match func(ir.Binding) bool) bool {
	if ep == nil || ep.Function.Result == nil {
		return false
	}
	res := ep.Function.Result
	if res.Binding != nil && match(*res.Binding) {
		return true
	}
	// Struct return: check every member binding.
	// We rely on the entry point's own type table (no irMod here) by
	// walking the result type via a helper threaded from the caller; for
	// the depth check the caller passes ep so we look up via a closure
	// that knows the module — but only buildPSV has access. Instead,
	// expose a variant taking irMod.
	return false
}

// entryWritesDepthInModule extends entryWritesDepth to also walk a
// struct-typed return's members. The callers always have access to
// the *ir.Module so we pass it in directly.
func entryWritesDepthInModule(irMod *ir.Module, ep *ir.EntryPoint) bool {
	if entryWritesDepth(ep) {
		return true
	}
	if ep == nil || ep.Function.Result == nil || irMod == nil {
		return false
	}
	resType := irMod.Types[ep.Function.Result.Type]
	st, ok := resType.Inner.(ir.StructType)
	if !ok {
		return false
	}
	for _, m := range st.Members {
		if m.Binding == nil {
			continue
		}
		if bb, isBB := (*m.Binding).(ir.BuiltinBinding); isBB && bb.Builtin == ir.BuiltinFragDepth {
			return true
		}
	}
	return false
}

// entryUsesNumWorkGroups reports whether any argument of the entry point
// (or any nested struct member of an argument) carries the
// BuiltinNumWorkGroups binding. The emitter creates a synthetic CBV
// resource on demand for this builtin (io.go getOrCreateNumWorkGroupsCBV);
// the PSV0 builder must mirror the declaration to keep ResourceCount in
// sync with the bitcode module.
func entryUsesNumWorkGroups(ep *ir.EntryPoint) bool {
	if ep == nil {
		return false
	}
	check := func(b ir.Binding) bool {
		bb, ok := b.(ir.BuiltinBinding)
		return ok && bb.Builtin == ir.BuiltinNumWorkGroups
	}
	for i := range ep.Function.Arguments {
		arg := &ep.Function.Arguments[i]
		if arg.Binding != nil && check(*arg.Binding) {
			return true
		}
	}
	return false
}

// globalUsesInt64Atomic walks a global variable's type to determine whether
// the resource is the destination of int64 atomic operations. The detection
// is conservative — any AtomicType wrapping an i64/u64 scalar anywhere in
// the type graph counts (storage buffer arrays, struct members), and any
// storage texture with an R64Uint/R64Sint format counts. Mirrors the
// shader-flag side moduleUsesInt64Atomics so PSV0 ResFlags and SFI0 stay
// in sync.
func globalUsesInt64Atomic(irMod *ir.Module, gv *ir.GlobalVariable) bool {
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
		case ir.ImageType:
			if t.Class == ir.ImageClassStorage {
				return t.StorageFormat == ir.StorageFormatR64Uint || t.StorageFormat == ir.StorageFormatR64Sint
			}
		}
		return false
	}
	return walk(gv.Type)
}

// collectPSVResources walks the IR's global variables and produces one
// PSVResourceBinding per resource. BUG-DXIL-022.
//
// Ordering MUST match dxc's PSV0 layout: CBuffers, Samplers, SRVs, UAVs
// (see reference/dxil/dxc/lib/DxilContainer/DxilContainerAssembler.cpp
// line 816-855). The validator builds its own list from the same DxilModule
// resource accessors and memcmp's element-by-element, so any reordering
// here breaks 'ResourceBindInfo mismatch' even when the records are
// individually correct.
//
// reachable is the set of globals actually used by the entry point being
// compiled. Multi-EP modules carry all globals in one arena (see
// BUG-DXIL-016); without this filter the PSV0 resource list disagrees
// with the bitcode metadata that emit/resources.go produces (which
// applies the same filter).
//
//nolint:gocognit,cyclop,gocyclo,funlen // single dispatch table over IR resource categories
func collectPSVResources(irMod *ir.Module, reachable map[ir.GlobalVariableHandle]bool, bindingMap BindingMap, samplerHeap *SamplerHeapBindTargets, samplerBufMap map[uint32]BindTarget) []container.PSVResourceBinding {
	if irMod == nil {
		return nil
	}
	const (
		classCBV     = 0
		classSampler = 1
		classSRV     = 2
		classUAV     = 3
	)
	// Sampler-heap rewrite: mirror emit/sampler_heap.go's detection so
	// that the PSV0 resource list and the DXIL bitcode resource metadata
	// agree (the validator memcmp's the two and reports 'DXIL container
	// mismatch for ResourceCount/ResourceBindInfo' on any drift).
	samplerGroups, hasStdSampler, hasCmpSampler := detectPSVSamplerHeap(irMod, reachable)
	samplerHeapMode := hasStdSampler || hasCmpSampler

	var buckets [4][]container.PSVResourceBinding
	for i := range irMod.GlobalVariables {
		gv := &irMod.GlobalVariables[i]
		// Reachability filter — must mirror emit/resources.go.
		if reachable != nil && !reachable[ir.GlobalVariableHandle(i)] {
			continue
		}
		if gv.Binding == nil {
			// Push constants / immediate data get a synthetic CBV binding
			// inside emit.analyzeResources but the index is allocated
			// there — we cannot reproduce it here without sharing state.
			// Tracked as a follow-up; the affected shaders fall through
			// to the next architectural wall.
			continue
		}
		// In sampler-heap mode, skip per-WGSL sampler globals. The heap
		// and per-group index buffer entries are appended below.
		if samplerHeapMode && gv.Space == ir.SpaceHandle {
			if _, isSampler, _ := psvSamplerKindOfGlobal(irMod, gv); isSampler {
				continue
			}
		}

		var class int
		var resType container.PSVResourceType
		var resKind container.PSVResourceKind
		switch gv.Space {
		case ir.SpaceUniform, ir.SpacePushConstant, ir.SpaceImmediate:
			class = classCBV
			resType = container.PSVResTypeCBV
			resKind = container.PSVResKindCBuffer
		case ir.SpaceStorage:
			if gv.Access == ir.StorageRead {
				class = classSRV
				resType = container.PSVResTypeSRVRaw
				resKind = container.PSVResKindRawBuffer
			} else {
				class = classUAV
				resType = container.PSVResTypeUAVRaw
				resKind = container.PSVResKindRawBuffer
			}
		case ir.SpaceHandle:
			rt, rk, ok := classifyHandleResource(irMod, gv.Type)
			if !ok {
				continue
			}
			resType = rt
			resKind = rk
			switch {
			case rt == container.PSVResTypeSampler:
				class = classSampler
			case rt == container.PSVResTypeUAVTyped, rt == container.PSVResTypeUAVRaw,
				rt == container.PSVResTypeUAVStructured, rt == container.PSVResTypeUAVStructuredWithCounter:
				class = classUAV
			default:
				class = classSRV
			}
		default:
			continue
		}

		// Resolve (space, register, arraySize) through the binding map
		// when present — must stay consistent with emit/resources.go,
		// which applies the same map to DXIL bitcode resource metadata.
		// Any disagreement trips 'DXIL container mismatch for
		// ResourceBindInfo'.
		space := gv.Binding.Group
		lower := gv.Binding.Binding
		var explicitArraySize *uint32
		if bindingMap != nil {
			loc := BindingLocation{Group: gv.Binding.Group, Binding: gv.Binding.Binding}
			if tgt, ok := bindingMap[loc]; ok {
				space = tgt.Space
				lower = tgt.Register
				explicitArraySize = tgt.BindingArraySize
			}
		}

		upper := lower
		if int(gv.Type) < len(irMod.Types) {
			if ba, ok := irMod.Types[gv.Type].Inner.(ir.BindingArrayType); ok {
				switch {
				case ba.Size != nil:
					upper = lower + *ba.Size - 1
				case explicitArraySize != nil:
					// Unbounded IR array with a concrete TOML override
					// (e.g. binding_array_size = 10). Honor it so the
					// PSV0 entry does not claim the entire register
					// space and collide with neighboring resources.
					upper = lower + *explicitArraySize - 1
				default:
					upper = ^uint32(0)
				}
			}
		}

		// PSVResourceFlag::UsedByAtomic64 (0x1) marks a UAV that is the
		// destination of an int64 atomic op. The validator regenerates
		// this bit from instruction-level dx.op.atomicBinOp.i64 usage
		// and reports 'ResourceBindInfo mismatch' when it disagrees with
		// the PSV0 byte. Detect the flag by walking the resource type
		// for atomic<i64> / atomic<u64> wrappers (storage buffers) or
		// the r64uint / r64sint storage texture format (textures).
		var resFlags uint32
		if class == classUAV && globalUsesInt64Atomic(irMod, gv) {
			resFlags = 0x1 // PSVResourceFlag::UsedByAtomic64
		}
		buckets[class] = append(buckets[class], container.PSVResourceBinding{
			ResType:    resType,
			Space:      space,
			LowerBound: lower,
			UpperBound: upper,
			ResKind:    resKind,
			ResFlags:   resFlags,
		})
	}

	// Append synthesized sampler heap + per-group index buffer entries.
	// Must stay byte-aligned with emit/sampler_heap.go.appendSamplerHeapResources.
	//
	// Resolve (space, register) from caller-provided SamplerHeapTargets /
	// SamplerBufferBindingMap, falling back to the same defaults the
	// emitter uses when those fields are nil.
	stdSpace, stdReg := psvSamplerHeapTarget(samplerHeap, false)
	cmpSpace, cmpReg := psvSamplerHeapTarget(samplerHeap, true)

	if hasStdSampler {
		buckets[classSampler] = append(buckets[classSampler], container.PSVResourceBinding{
			ResType:    container.PSVResTypeSampler,
			Space:      stdSpace,
			LowerBound: stdReg,
			UpperBound: stdReg + 2048 - 1, // 2048-slot array
			ResKind:    container.PSVResKindSampler,
		})
	}
	if hasCmpSampler {
		buckets[classSampler] = append(buckets[classSampler], container.PSVResourceBinding{
			ResType:    container.PSVResTypeSampler,
			Space:      cmpSpace,
			LowerBound: cmpReg,
			UpperBound: cmpReg + 2048 - 1,
			ResKind:    container.PSVResKindSampler,
		})
	}
	for _, group := range samplerGroups {
		idxSpace, idxReg := psvSamplerIndexBufferTarget(samplerBufMap, group)
		buckets[classSRV] = append(buckets[classSRV], container.PSVResourceBinding{
			ResType:    container.PSVResTypeSRVStructured,
			Space:      idxSpace,
			LowerBound: idxReg,
			UpperBound: idxReg, // single buffer view
			ResKind:    container.PSVResKindStructuredBuffer,
		})
	}

	out := make([]container.PSVResourceBinding, 0, len(buckets[0])+len(buckets[1])+len(buckets[2])+len(buckets[3]))
	out = append(out, buckets[classCBV]...)
	out = append(out, buckets[classSampler]...)
	out = append(out, buckets[classSRV]...)
	out = append(out, buckets[classUAV]...)
	return out
}

// psvSamplerHeapTarget resolves (space, register) for a standard or
// comparison sampler heap PSV0 entry, consulting the caller-provided
// SamplerHeapBindTargets override and falling back to the same defaults
// that emit/sampler_heap.go uses.
func psvSamplerHeapTarget(t *SamplerHeapBindTargets, comparison bool) (space, register uint32) {
	if t != nil {
		if comparison {
			return t.ComparisonSamplers.Space, t.ComparisonSamplers.Register
		}
		return t.StandardSamplers.Space, t.StandardSamplers.Register
	}
	if comparison {
		return 1, 0 // comparisonSamplerHeapDefaultSpace, comparisonSamplerHeapDefaultRegister
	}
	return 0, 0 // samplerHeapDefaultSpace, samplerHeapDefaultRegister
}

// psvSamplerIndexBufferTarget resolves (space, register) for a per-group
// sampler index buffer SRV PSV0 entry.
func psvSamplerIndexBufferTarget(m map[uint32]BindTarget, group uint32) (space, register uint32) {
	if m != nil {
		if tgt, ok := m[group]; ok {
			return tgt.Space, tgt.Register
		}
	}
	return 255, group // samplerIndexBufferDefaultSpace, group
}

// detectPSVSamplerHeap is the PSV0-side counterpart of
// emit/sampler_heap.go:detectSamplerHeapNeeded. Must stay byte-aligned
// with the emit logic (same filter: reachability + SpaceHandle + SamplerType
// + skip binding_array<sampler>). Returns the sorted list of bind groups
// that contain at least one sampler, plus two flags for whether any
// standard / comparison sampler is present.
func detectPSVSamplerHeap(irMod *ir.Module, reachable map[ir.GlobalVariableHandle]bool) (groups []uint32, hasStd, hasCmp bool) {
	groupSet := make(map[uint32]bool)
	for i := range irMod.GlobalVariables {
		gv := &irMod.GlobalVariables[i]
		if reachable != nil && !reachable[ir.GlobalVariableHandle(i)] {
			continue
		}
		if gv.Space != ir.SpaceHandle || gv.Binding == nil {
			continue
		}
		cmp, isSampler, inBindingArray := psvSamplerKindOfGlobal(irMod, gv)
		if !isSampler || inBindingArray {
			continue
		}
		if cmp {
			hasCmp = true
		} else {
			hasStd = true
		}
		if !groupSet[gv.Binding.Group] {
			groupSet[gv.Binding.Group] = true
			groups = append(groups, gv.Binding.Group)
		}
	}
	if !hasStd && !hasCmp {
		return nil, false, false
	}
	// Sort ascending — deterministic register allocation.
	for i := 1; i < len(groups); i++ {
		for j := i; j > 0 && groups[j-1] > groups[j]; j-- {
			groups[j-1], groups[j] = groups[j], groups[j-1]
		}
	}
	return groups, hasStd, hasCmp
}

// psvSamplerKindOfGlobal returns (comparison, isSampler, isBindingArray).
// isBindingArray is true when the type is BindingArrayType wrapping a
// sampler — the PSV0 side keeps the direct-binding path for those since
// the emit side does too (see sampler_heap.go detectSamplerHeapNeeded).
func psvSamplerKindOfGlobal(irMod *ir.Module, gv *ir.GlobalVariable) (cmp, isSampler, isBindingArray bool) {
	if int(gv.Type) >= len(irMod.Types) {
		return false, false, false
	}
	inner := irMod.Types[gv.Type].Inner
	if ba, ok := inner.(ir.BindingArrayType); ok {
		if int(ba.Base) >= len(irMod.Types) {
			return false, false, false
		}
		if st, ok := irMod.Types[ba.Base].Inner.(ir.SamplerType); ok {
			return st.Comparison, true, true
		}
		return false, false, false
	}
	if st, ok := inner.(ir.SamplerType); ok {
		return st.Comparison, true, false
	}
	return false, false, false
}

// classifyHandleResource peels a (possibly binding-array-wrapped) handle
// type and returns its PSV resource type/kind tuple.
func classifyHandleResource(irMod *ir.Module, th ir.TypeHandle) (container.PSVResourceType, container.PSVResourceKind, bool) {
	if int(th) >= len(irMod.Types) {
		return 0, 0, false
	}
	inner := irMod.Types[th].Inner
	if ba, ok := inner.(ir.BindingArrayType); ok {
		if int(ba.Base) >= len(irMod.Types) {
			return 0, 0, false
		}
		inner = irMod.Types[ba.Base].Inner
	}
	switch t := inner.(type) {
	case ir.ImageType:
		isStorage := t.Class == ir.ImageClassStorage
		switch t.Dim {
		case ir.Dim1D:
			if t.Arrayed {
				return uavOrSrvTyped(isStorage), container.PSVResKindTexture1DArray, true
			}
			return uavOrSrvTyped(isStorage), container.PSVResKindTexture1D, true
		case ir.Dim2D:
			if t.Multisampled {
				if t.Arrayed {
					return uavOrSrvTyped(isStorage), container.PSVResKindTexture2DMSArray, true
				}
				return uavOrSrvTyped(isStorage), container.PSVResKindTexture2DMS, true
			}
			if t.Arrayed {
				return uavOrSrvTyped(isStorage), container.PSVResKindTexture2DArray, true
			}
			return uavOrSrvTyped(isStorage), container.PSVResKindTexture2D, true
		case ir.Dim3D:
			return uavOrSrvTyped(isStorage), container.PSVResKindTexture3D, true
		case ir.DimCube:
			if t.Arrayed {
				return uavOrSrvTyped(isStorage), container.PSVResKindTextureCubeArray, true
			}
			return uavOrSrvTyped(isStorage), container.PSVResKindTextureCube, true
		}
	case ir.SamplerType:
		return container.PSVResTypeSampler, container.PSVResKindSampler, true
	case ir.AccelerationStructureType:
		return container.PSVResTypeSRVRaw, container.PSVResKindRTAccelerationStructure, true
	}
	return 0, 0, false
}

func uavOrSrvTyped(isStorage bool) container.PSVResourceType {
	if isStorage {
		return container.PSVResTypeUAVTyped
	}
	return container.PSVResTypeSRVTyped
}

// reachableGlobalsForEntry walks the entry point function body and every
// function transitively callable from it, returning the set of
// GlobalVariableHandles referenced by live expressions.
//
// After DCE runs (runOptPasses), the entry point's statement body only
// contains live StmtEmit ranges and live statement operands. Dead
// expressions still exist in the expression arena but are not referenced
// by any statement. Scanning the raw expression arena (the old approach)
// would incorrectly mark globals referenced only by dead code as
// reachable, causing naga to emit createHandle calls and resource
// metadata entries that DXC's GlobalDCE would strip. This function
// mirrors DXC's behavior by only following expression handles that are
// transitively reachable from the post-DCE statement body.
//
// For helper functions (reached via StmtCall), the full expression arena
// is scanned because those functions are not DCE'd individually and
// being called at all implies their entire body is live.
func reachableGlobalsForEntry(irMod *ir.Module, ep *ir.EntryPoint) map[ir.GlobalVariableHandle]bool {
	if irMod == nil || ep == nil {
		return nil
	}
	reachable := make(map[ir.GlobalVariableHandle]bool)
	visited := make(map[*ir.Function]bool)

	// collectFromLiveExprs marks globals reachable from a set of live
	// expression handles within a function, following sub-expression
	// references transitively.
	seen := make(map[ir.ExpressionHandle]bool)
	var followExpr func(fn *ir.Function, h ir.ExpressionHandle)
	followExpr = func(fn *ir.Function, h ir.ExpressionHandle) {
		if int(h) >= len(fn.Expressions) || seen[h] {
			return
		}
		seen[h] = true
		expr := fn.Expressions[h]
		switch ex := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			reachable[ex.Variable] = true
		case ir.ExprCallResult:
			if int(ex.Function) < len(irMod.Functions) {
				walkHelperFunction(&irMod.Functions[ex.Function], irMod, visited, reachable)
			}
		}
		// Follow sub-expression references.
		visitExprSubHandles(expr.Kind, func(sub ir.ExpressionHandle) {
			followExpr(fn, sub)
		})
	}

	// Walk the entry point's statement body, collecting expression
	// handles referenced by live statements.
	fn := &ep.Function
	visited[fn] = true
	walkBlockExprs(fn, fn.Body, followExpr)

	// Also follow StmtCall targets into helper functions.
	// Call arguments and result are already followed by walkStmtExprs;
	// this only needs to walk into callees.
	walkBlockForCalls(fn.Body, func(call ir.StmtCall) {
		if int(call.Function) < len(irMod.Functions) {
			walkHelperFunction(&irMod.Functions[call.Function], irMod, visited, reachable)
		}
	})

	return reachable
}

// walkHelperFunction scans all expressions in a helper function (not the
// entry point). Helper functions reached via StmtCall are assumed fully
// live — their entire expression arena is scanned. This matches DXC
// behavior: if LLVM doesn't inline a callee, all its code is live.
func walkHelperFunction(
	fn *ir.Function,
	irMod *ir.Module,
	visited map[*ir.Function]bool,
	reachable map[ir.GlobalVariableHandle]bool,
) {
	if fn == nil || visited[fn] {
		return
	}
	visited[fn] = true
	for _, expr := range fn.Expressions {
		switch ex := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			reachable[ex.Variable] = true
		case ir.ExprCallResult:
			if int(ex.Function) < len(irMod.Functions) {
				walkHelperFunction(&irMod.Functions[ex.Function], irMod, visited, reachable)
			}
		}
	}
	walkBlockForCalls(fn.Body, func(call ir.StmtCall) {
		if int(call.Function) < len(irMod.Functions) {
			walkHelperFunction(&irMod.Functions[call.Function], irMod, visited, reachable)
		}
	})
}

// walkBlockExprs walks a statement block and calls followExpr for every
// expression handle referenced by statements (StmtEmit ranges, store
// targets/values, return values, control-flow conditions, etc.).
func walkBlockExprs(fn *ir.Function, block ir.Block, followExpr func(*ir.Function, ir.ExpressionHandle)) {
	for i := range block {
		walkStmtExprs(fn, block[i].Kind, followExpr)
	}
}

// walkStmtExprs dispatches a single statement, calling followExpr for
// every expression handle it references and recursing into sub-blocks.
//
//nolint:gocyclo,cyclop // exhaustive statement-kind dispatch
func walkStmtExprs(fn *ir.Function, kind ir.StatementKind, followExpr func(*ir.Function, ir.ExpressionHandle)) {
	switch sk := kind.(type) {
	case ir.StmtEmit:
		for h := sk.Range.Start; h < sk.Range.End; h++ {
			followExpr(fn, h)
		}
	case ir.StmtStore:
		followExpr(fn, sk.Pointer)
		followExpr(fn, sk.Value)
	case ir.StmtReturn:
		if sk.Value != nil {
			followExpr(fn, *sk.Value)
		}
	case ir.StmtImageStore:
		followImageStoreExprs(fn, sk, followExpr)
	case ir.StmtImageAtomic:
		followImageAtomicExprs(fn, sk, followExpr)
	case ir.StmtAtomic:
		followExpr(fn, sk.Pointer)
		followExpr(fn, sk.Value)
		if sk.Result != nil {
			followExpr(fn, *sk.Result)
		}
	case ir.StmtCall:
		for _, a := range sk.Arguments {
			followExpr(fn, a)
		}
		if sk.Result != nil {
			followExpr(fn, *sk.Result)
		}
	case ir.StmtWorkGroupUniformLoad:
		followExpr(fn, sk.Pointer)
		followExpr(fn, sk.Result)
	case ir.StmtRayQuery:
		followExpr(fn, sk.Query)
		followRayQueryFunctionExprs(fn, sk.Fun, followExpr)
	case ir.StmtSubgroupBallot:
		followExpr(fn, sk.Result)
		if sk.Predicate != nil {
			followExpr(fn, *sk.Predicate)
		}
	case ir.StmtSubgroupCollectiveOperation:
		followExpr(fn, sk.Argument)
		followExpr(fn, sk.Result)
	case ir.StmtSubgroupGather:
		followExpr(fn, sk.Argument)
		followExpr(fn, sk.Result)
	case ir.StmtIf:
		followExpr(fn, sk.Condition)
		walkBlockExprs(fn, sk.Accept, followExpr)
		walkBlockExprs(fn, sk.Reject, followExpr)
	case ir.StmtLoop:
		walkBlockExprs(fn, sk.Body, followExpr)
		walkBlockExprs(fn, sk.Continuing, followExpr)
		if sk.BreakIf != nil {
			followExpr(fn, *sk.BreakIf)
		}
	case ir.StmtSwitch:
		followExpr(fn, sk.Selector)
		for ci := range sk.Cases {
			walkBlockExprs(fn, sk.Cases[ci].Body, followExpr)
		}
	case ir.StmtBlock:
		walkBlockExprs(fn, sk.Block, followExpr)
	case ir.StmtBarrier, ir.StmtKill:
		// No expression operands.
	}
}

func followImageStoreExprs(fn *ir.Function, sk ir.StmtImageStore, followExpr func(*ir.Function, ir.ExpressionHandle)) {
	followExpr(fn, sk.Image)
	followExpr(fn, sk.Coordinate)
	followExpr(fn, sk.Value)
	if sk.ArrayIndex != nil {
		followExpr(fn, *sk.ArrayIndex)
	}
}

func followImageAtomicExprs(fn *ir.Function, sk ir.StmtImageAtomic, followExpr func(*ir.Function, ir.ExpressionHandle)) {
	followExpr(fn, sk.Image)
	followExpr(fn, sk.Coordinate)
	followExpr(fn, sk.Value)
	if sk.ArrayIndex != nil {
		followExpr(fn, *sk.ArrayIndex)
	}
}

// visitExprSubHandles calls f for every expression handle directly
// referenced by the given expression kind. This mirrors DCE's
// visitExprHandles but lives in the dxil package to avoid a
// cross-package dependency.
//
//nolint:gocyclo,cyclop // exhaustive expression-kind dispatch
func visitExprSubHandles(kind ir.ExpressionKind, f func(ir.ExpressionHandle)) {
	switch k := kind.(type) {
	case ir.ExprLoad:
		f(k.Pointer)
	case ir.ExprAlias:
		f(k.Source)
	case ir.ExprAccess:
		f(k.Base)
		f(k.Index)
	case ir.ExprAccessIndex:
		f(k.Base)
	case ir.ExprBinary:
		f(k.Left)
		f(k.Right)
	case ir.ExprUnary:
		f(k.Expr)
	case ir.ExprSelect:
		f(k.Condition)
		f(k.Accept)
		f(k.Reject)
	case ir.ExprSplat:
		f(k.Value)
	case ir.ExprSwizzle:
		f(k.Vector)
	case ir.ExprCompose:
		for _, c := range k.Components {
			f(c)
		}
	case ir.ExprAs:
		f(k.Expr)
	case ir.ExprDerivative:
		f(k.Expr)
	case ir.ExprMath:
		visitMathExprHandles(k, f)
	case ir.ExprRelational:
		f(k.Argument)
	case ir.ExprArrayLength:
		f(k.Array)
	case ir.ExprImageSample:
		visitImageSampleHandles(k, f)
	case ir.ExprImageLoad:
		visitImageLoadHandles(k, f)
	case ir.ExprImageQuery:
		f(k.Image)
	case ir.ExprPhi:
		for _, inc := range k.Incoming {
			f(inc.Value)
		}
	}
}

func visitMathExprHandles(k ir.ExprMath, f func(ir.ExpressionHandle)) {
	f(k.Arg)
	if k.Arg1 != nil {
		f(*k.Arg1)
	}
	if k.Arg2 != nil {
		f(*k.Arg2)
	}
	if k.Arg3 != nil {
		f(*k.Arg3)
	}
}

func visitImageSampleHandles(k ir.ExprImageSample, f func(ir.ExpressionHandle)) {
	f(k.Image)
	f(k.Sampler)
	f(k.Coordinate)
	if k.ArrayIndex != nil {
		f(*k.ArrayIndex)
	}
	if k.Offset != nil {
		f(*k.Offset)
	}
	if k.DepthRef != nil {
		f(*k.DepthRef)
	}
}

func visitImageLoadHandles(k ir.ExprImageLoad, f func(ir.ExpressionHandle)) {
	f(k.Image)
	f(k.Coordinate)
	if k.ArrayIndex != nil {
		f(*k.ArrayIndex)
	}
	if k.Sample != nil {
		f(*k.Sample)
	}
	if k.Level != nil {
		f(*k.Level)
	}
}

// followRayQueryFunctionExprs calls followExpr for every expression
// handle embedded in a RayQueryFunction variant.
func followRayQueryFunctionExprs(fn *ir.Function, fun ir.RayQueryFunction, followExpr func(*ir.Function, ir.ExpressionHandle)) {
	switch f := fun.(type) {
	case ir.RayQueryInitialize:
		followExpr(fn, f.AccelerationStructure)
		followExpr(fn, f.Descriptor)
	case ir.RayQueryProceed:
		followExpr(fn, f.Result)
	case ir.RayQueryGenerateIntersection:
		followExpr(fn, f.HitT)
	case ir.RayQueryTerminate, ir.RayQueryConfirmIntersection:
		// No expression operands.
	}
}

// walkBlockForCalls visits every StmtCall in a block tree, descending
// into nested StmtBlock/StmtIf/StmtSwitch/StmtLoop bodies.
func walkBlockForCalls(block ir.Block, visit func(ir.StmtCall)) {
	for i := range block {
		s := &block[i]
		switch k := s.Kind.(type) {
		case ir.StmtCall:
			visit(k)
		case ir.StmtBlock:
			walkBlockForCalls(k.Block, visit)
		case ir.StmtIf:
			walkBlockForCalls(k.Accept, visit)
			walkBlockForCalls(k.Reject, visit)
		case ir.StmtLoop:
			walkBlockForCalls(k.Body, visit)
			walkBlockForCalls(k.Continuing, visit)
		case ir.StmtSwitch:
			for _, c := range k.Cases {
				walkBlockForCalls(c.Body, visit)
			}
		}
	}
}

// stageToPSVKind maps naga ShaderStage to the PSV0 ShaderStage byte.
// Single source of truth for PSV0 stage dispatch; see BUG-DXIL-008.
// ABI values locked by TestPSVShaderKindMatchesMicrosoftABI.
func stageToPSVKind(stage ir.ShaderStage) container.PSVShaderKind {
	switch stage {
	case ir.StageVertex:
		return container.PSVVertex
	case ir.StageFragment:
		return container.PSVPixel
	case ir.StageCompute:
		return container.PSVCompute
	case ir.StageMesh:
		return container.PSVMesh
	case ir.StageTask:
		return container.PSVAmplification
	default:
		return container.PSVInvalid
	}
}

// featureInfoFromShaderFlags maps bitcode ShaderFlags into SFI0 (Feature
// Info) container bits, mirroring DXC's ShaderFlags::GetFeatureInfo at
// lib/DXIL/DxilShaderFlags.cpp:55. The validator memcmp's the container's
// SFI0 part against a value it regenerates from bitcode metadata via this
// same mapping, so the two sides MUST agree exactly — passing a
// module-wide approximation (as we did before) trips 'Container part
// Feature Info does not match expected for module' whenever the per-EP
// bitcode ShaderFlags differ from what a module-wide walk would produce
// (e.g. a multi-EP module where only one entry point uses ray queries).
//
// Bit layouts:
//   - bit  2 (0x4)  of ShaderFlags         = EnableDoublePrecision
//     →   0x0001 of SFI0                 = Doubles
//   - bit  5 (0x20) of ShaderFlags         = LowPrecisionPresent
//   - bit  6 (0x40) of ShaderFlags         = EnableDoubleExtensions
//     →   0x0020 of SFI0                 = 11_1_DoubleExtensions
//   - bit 23 (0x800000) of ShaderFlags     = UseNativeLowPrecision
//     →     0x10 of SFI0                 = MinimumPrecision (non-native)
//     →  0x40000 of SFI0                 = NativeLowPrecision
//   - bit 16 (0x10000) of ShaderFlags      = UAVsAtEveryStage
//     →   0x0004 of SFI0                 = UAVsAtEveryStage
//   - bit 20 (0x100000) of ShaderFlags     = Int64Ops
//     →   0x8000 of SFI0                 = Int64Ops
//   - bit 25 (0x2000000) of ShaderFlags    = RaytracingTier1_1
//     → 0x100000 of SFI0                 = Raytracing_Tier_1_1
//
// SFI0 constants from DxilConstants.h:2299+.
func featureInfoFromShaderFlags(sf uint64) uint64 {
	var bits uint64
	const (
		flagDoublePrec      = uint64(0x4)        // bit 2
		flagLowPrecision    = uint64(0x20)       // bit 5
		flagDoubleExt       = uint64(0x40)       // bit 6
		flagUAVsAtEvery     = uint64(0x10000)    // bit 16
		flagWaveOps         = uint64(0x80000)    // bit 19
		flagInt64Ops        = uint64(0x100000)   // bit 20
		flagViewID          = uint64(0x200000)   // bit 21
		flagNativeLowPrec   = uint64(0x800000)   // bit 23
		flagRaytracingTier1 = uint64(0x2000000)  // bit 25
		flagAtomic64Typed   = uint64(0x8000000)  // bit 27
		flagAtomic64GS      = uint64(0x10000000) // bit 28
	)
	if sf&flagDoublePrec != 0 {
		bits |= 0x0001 // Doubles
	}
	if sf&flagDoubleExt != 0 {
		bits |= 0x0020 // 11_1_DoubleExtensions
	}
	if sf&flagLowPrecision != 0 {
		if sf&flagNativeLowPrec != 0 {
			bits |= 0x40000 // NativeLowPrecision
		} else {
			bits |= 0x10 // MinimumPrecision
		}
	}
	if sf&flagUAVsAtEvery != 0 {
		bits |= 0x0004 // UAVsAtEveryStage
	}
	if sf&flagWaveOps != 0 {
		// DxilConstants.h:2317 ShaderFeatureInfo_WaveOps = 0x4000. NOT
		// the same as the bitcode WaveOps bit (0x80000 = bit 19); the
		// two spaces happen to share the name but not the value. 0x80000
		// in SFI0 is ShaderFeatureInfo_ShadingRate, which would cause
		// dxc -dumpbin to report 'shader requires Shading Rate' and the
		// validator to reject Feature Info as non-matching for a shader
		// that in fact only uses wave intrinsics.
		bits |= 0x4000 // WaveOps
	}
	if sf&flagInt64Ops != 0 {
		bits |= 0x8000 // Int64Ops
	}
	if sf&flagViewID != 0 {
		bits |= 0x10000 // ViewID
	}
	if sf&flagAtomic64Typed != 0 {
		bits |= 0x400000 // AtomicInt64OnTypedResource
	}
	if sf&flagAtomic64GS != 0 {
		bits |= 0x800000 // AtomicInt64OnGroupShared
	}
	if sf&flagRaytracingTier1 != 0 {
		bits |= 0x100000 // Raytracing_Tier_1_1
	}
	return bits
}

// typeContains64BitScalar walks the type graph for any 64-bit scalar leaf
// (int64, uint64, float64). Conservative — bails out at recursive struct
// depth via a visited set.
func typeContains64BitScalar(m *ir.Module, th ir.TypeHandle) bool {
	visited := make(map[ir.TypeHandle]bool)
	var walk func(th ir.TypeHandle) bool
	walk = func(th ir.TypeHandle) bool {
		if visited[th] || int(th) >= len(m.Types) {
			return false
		}
		visited[th] = true
		switch t := m.Types[th].Inner.(type) {
		case ir.ScalarType:
			return t.Width == 8
		case ir.VectorType:
			return t.Scalar.Width == 8
		case ir.MatrixType:
			return t.Scalar.Width == 8
		case ir.AtomicType:
			return t.Scalar.Width == 8
		case ir.ArrayType:
			return walk(t.Base)
		case ir.StructType:
			for _, mb := range t.Members {
				if walk(mb.Type) {
					return true
				}
			}
		}
		return false
	}
	return walk(th)
}

// moduleUsesViewID returns true when any entry point reads
// @builtin(view_index). DXC's CollectShaderFlagsForModule sets ViewID in
// CollectShaderFlagsForFunction whenever it sees a dx.op.viewID call;
// we walk the IR up front instead since the compile is per-EP and we
// need to know in advance for the SM auto-upgrade.
func moduleUsesViewID(m *ir.Module) bool {
	for i := range m.EntryPoints {
		if entryUsesViewIndex(&m.EntryPoints[i]) {
			return true
		}
	}
	return false
}

// entryUsesViewIndex reports whether any argument of the entry point
// (or struct member of an argument) carries the BuiltinViewIndex binding.
func entryUsesViewIndex(ep *ir.EntryPoint) bool {
	if ep == nil {
		return false
	}
	check := func(b ir.Binding) bool {
		bb, ok := b.(ir.BuiltinBinding)
		return ok && bb.Builtin == ir.BuiltinViewIndex
	}
	for i := range ep.Function.Arguments {
		if ep.Function.Arguments[i].Binding != nil && check(*ep.Function.Arguments[i].Binding) {
			return true
		}
	}
	return false
}

// reachableUsesRawBufferAccess is the post-DCE version of the SM 6.2
// upgrade check. It only considers storage-buffer globals that survived
// dead-code elimination (present in reachableGlobals). When DCE removes
// all accesses to a >4-component storage buffer, no rawBufferLoad/Store
// ops are emitted and the SM 6.2 requirement does not apply.
func reachableUsesRawBufferAccess(m *ir.Module, reachable map[ir.GlobalVariableHandle]bool) bool {
	for i := range m.GlobalVariables {
		h := ir.GlobalVariableHandle(i)
		if !reachable[h] {
			continue
		}
		gv := &m.GlobalVariables[i]
		if gv.Space != ir.SpaceStorage {
			continue
		}
		if int(gv.Type) >= len(m.Types) {
			continue
		}
		if compositeHasMoreThanFour(m, m.Types[gv.Type].Inner) {
			return true
		}
	}
	return false
}

// reachableUsesInt64Buffer is the post-DCE version of the SM 6.3 upgrade
// check. Only considers storage buffers reachable after dead-code elimination.
func reachableUsesInt64Buffer(m *ir.Module, reachable map[ir.GlobalVariableHandle]bool) bool {
	for i := range m.GlobalVariables {
		h := ir.GlobalVariableHandle(i)
		if !reachable[h] {
			continue
		}
		gv := &m.GlobalVariables[i]
		if gv.Space != ir.SpaceStorage {
			continue
		}
		if int(gv.Type) >= len(m.Types) {
			continue
		}
		if typeContains64BitScalar(m, gv.Type) {
			return true
		}
	}
	return false
}

// compositeHasMoreThanFour returns true when an individual element access on
// this type would require more than 4 scalar components — the threshold
// above which emitUAVLoad switches from bufferLoad to rawBufferLoad.
//
// The check considers per-element component counts, not total array extent,
// because storage buffer accesses are per-element (arr[i] loads one element,
// not the whole array). This matches DXC behavior: a runtime array<u32> uses
// bufferLoad at SM 6.0, not rawBufferLoad at SM 6.2.
func compositeHasMoreThanFour(m *ir.Module, inner ir.TypeInner) bool {
	switch t := inner.(type) {
	case ir.VectorType:
		return int(t.Size) > 4
	case ir.MatrixType:
		return int(t.Columns)*int(t.Rows) > 4
	case ir.ArrayType:
		// Per-element access: check the base element type, not array extent.
		return compositeHasMoreThanFour(m, m.Types[t.Base].Inner)
	case ir.StructType:
		total := 0
		for _, mb := range t.Members {
			total += memberScalarCount(m, m.Types[mb.Type].Inner)
			if total > 4 {
				return true
			}
		}
		return total > 4
	}
	return false
}

// memberScalarCount returns the flattened scalar count of a struct member's
// type for the purpose of determining whether individual element accesses
// will require rawBufferLoad (> 4 components per access).
//
// For runtime-sized arrays, what matters is the per-element component count
// (since the emitter accesses elements individually), not the array extent.
// A runtime array<u32> is accessed one u32 at a time (1 component),
// so bufferLoad suffices and SM 6.0 is adequate. DXC uses the same logic:
// the pointers.wgsl shader with DynamicArray{arr: array<u32>} compiles
// to SM 6.0 with bufferLoad/bufferStore, not rawBuffer variants.
func memberScalarCount(m *ir.Module, inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	case ir.ArrayType:
		// For both fixed and runtime arrays, the SM upgrade decision
		// depends on the per-element component count, because the emitter
		// accesses array elements individually via bufferLoad/bufferStore.
		return memberScalarCount(m, m.Types[t.Base].Inner)
	case ir.StructType:
		total := 0
		for _, mb := range t.Members {
			total += memberScalarCount(m, m.Types[mb.Type].Inner)
		}
		return total
	}
	return 1
}

// moduleUsesLowPrecision reports whether any type in the module is a
// 16-bit float, 16-bit integer, or a vector/matrix thereof.
func moduleUsesLowPrecision(m *ir.Module) bool {
	for i := range m.Types {
		switch t := m.Types[i].Inner.(type) {
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
	// ir.MathQuantizeF16 lowers to fptrunc f32→f16 + fpext f16→f32, so
	// it materializes f16 in the emitted bitcode even when no f16 type
	// is declared in the IR type arena. The validator still counts
	// that toward m_bLowPrecisionPresent and SM compatibility checks,
	// so the auto-upgrade must see it here as well. math-functions.wgsl
	// trips this: the declared arena is pure f32 but quantizeToF16 is
	// used on scalar/vec2/vec3/vec4 paths.
	for i := range m.Functions {
		if functionContainsQuantizeF16(&m.Functions[i]) {
			return true
		}
	}
	for i := range m.EntryPoints {
		if functionContainsQuantizeF16(&m.EntryPoints[i].Function) {
			return true
		}
	}
	return false
}

// functionContainsQuantizeF16 reports whether fn contains any
// ir.ExprMath with Fun == ir.MathQuantizeF16.
func functionContainsQuantizeF16(fn *ir.Function) bool {
	for i := range fn.Expressions {
		if me, ok := fn.Expressions[i].Kind.(ir.ExprMath); ok && me.Fun == ir.MathQuantizeF16 {
			return true
		}
	}
	return false
}

// functionHasControlFlow reports whether the function body contains
// any structured control flow (if/loop/switch). Functions with control
// flow produce loops/branches that our DCE cannot fully eliminate when
// the call result is unused, diverging from DXC which eliminates dead
// calls before inlining.
func functionHasControlFlow(fn *ir.Function) bool {
	return blockHasControlFlow(fn.Body)
}

func blockHasControlFlow(block ir.Block) bool {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtIf, ir.StmtLoop, ir.StmtSwitch:
			return true
		case ir.StmtBlock:
			if blockHasControlFlow(sk.Block) {
				return true
			}
		}
	}
	return false
}

// functionHasBreakContinue reports whether any statement in the function
// body is a StmtBreak or StmtContinue. Such functions cannot be safely
// inlined because the break/continue would escape its enclosing loop.
func functionHasBreakContinue(fn *ir.Function) bool {
	return blockHasBreakContinue(fn.Body)
}

func blockHasBreakContinue(block ir.Block) bool {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtBreak, ir.StmtContinue:
			return true
		case ir.StmtIf:
			if blockHasBreakContinue(sk.Accept) || blockHasBreakContinue(sk.Reject) {
				return true
			}
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				if blockHasBreakContinue(sk.Cases[ci].Body) {
					return true
				}
			}
		case ir.StmtLoop:
			// break/continue INSIDE a loop is self-contained — safe to inline.
			// Don't recurse into loop body.
		case ir.StmtBlock:
			if blockHasBreakContinue(sk.Block) {
				return true
			}
		}
	}
	return false
}

// helperNeedsInlining mirrors emitHelperFunctions' eligibility gate and
// returns true when the callee CANNOT be emitted as a standalone LLVM
// function and would therefore fall through to emitStmtCall's
// zero-valued fallback. The IR inline pass consults this predicate to
// expand only the helpers that would otherwise silently return zero
// (BUG-DXIL-029).
//
// Must stay in lockstep with emitHelperFunctions (emitter.go:538).
func helperNeedsInlining(irMod *ir.Module, callee *ir.Function) bool {
	for _, arg := range callee.Arguments {
		if int(arg.Type) >= len(irMod.Types) {
			return true
		}
		if !emit.IsScalarizableType(irMod.Types[arg.Type].Inner) {
			return true
		}
	}
	if callee.Result != nil {
		if int(callee.Result.Type) >= len(irMod.Types) {
			return true
		}
		resultIR := irMod.Types[callee.Result.Type]
		if !emit.IsScalarizableType(resultIR.Inner) {
			return true
		}
		if st, ok := resultIR.Inner.(ir.ScalarType); ok && st.Kind == ir.ScalarBool {
			return true
		}
		if emit.ComponentCount(resultIR.Inner) > 1 {
			return true
		}
	}
	if emit.FunctionAccessesGlobals(callee) {
		return true
	}
	if emit.FunctionHasComplexLocals(callee, irMod) {
		return true
	}
	if emit.FunctionHasComplexExpressions(callee, irMod) {
		return true
	}
	return false
}

func moduleUsesInt64Atomics(m *ir.Module) bool {
	for i := range m.Types {
		switch t := m.Types[i].Inner.(type) {
		case ir.AtomicType:
			// Plain atomic<i64> / atomic<u64> on storage buffers.
			if (t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint) && t.Scalar.Width == 8 {
				return true
			}
		case ir.ImageType:
			// 64-bit storage texture formats (R64Uint / R64Sint) trigger
			// int64 texture atomics — textureAtomicMin/Max etc. in WGSL
			// lower to dx.op.atomicBinOp.i64 which the validator rejects
			// with 'opcode 64-bit atomic operations should only be used
			// in Shader Model 6.6+'. Detecting the storage format here
			// lets the SM auto-upgrade fire even though the type arena
			// has no AtomicType<i64> entry — atomic-ness is encoded in
			// the texture, not a wrapper type.
			if t.Class == ir.ImageClassStorage {
				if t.StorageFormat == ir.StorageFormatR64Uint || t.StorageFormat == ir.StorageFormatR64Sint {
					return true
				}
			}
		}
	}
	return false
}

// runOptPasses runs the optimization pass pipeline on all functions
// in the module. This mirrors DXC's post-emit pipeline order:
// SROA -> mem2reg -> SimplifyInst -> DCE (DxilLinker.cpp:1284).
//
// SROA decomposes struct-typed locals into per-member locals so that
// mem2reg can promote the resulting scalar/vector locals to SSA form.
// mem2reg promotes scalar locals to SSA values, eliminating
// alloca/load/store triples. DCE then removes stores to locals whose
// values never reach an observable side effect.
func runOptPasses(irModule *ir.Module) error {
	// SROA: decompose struct locals into per-member scalar/vector locals.
	for i := range irModule.EntryPoints {
		sroa.Run(irModule, &irModule.EntryPoints[i].Function)
	}
	for i := range irModule.Functions {
		sroa.Run(irModule, &irModule.Functions[i])
	}
	// mem2reg: promote scalar locals to SSA values.
	for i := range irModule.EntryPoints {
		if err := mem2reg.Run(irModule, &irModule.EntryPoints[i].Function); err != nil {
			return fmt.Errorf("dxil: mem2reg: %w", err)
		}
	}
	for i := range irModule.Functions {
		if err := mem2reg.Run(irModule, &irModule.Functions[i]); err != nil {
			return fmt.Errorf("dxil: mem2reg: %w", err)
		}
	}
	for i := range irModule.EntryPoints {
		dce.Run(irModule, &irModule.EntryPoints[i].Function)
	}
	for i := range irModule.Functions {
		dce.Run(irModule, &irModule.Functions[i])
	}
	return nil
}

func moduleUsesRayQuery(m *ir.Module) bool {
	for i := range m.Functions {
		if blockUsesRayQuery(m.Functions[i].Body) {
			return true
		}
	}
	for i := range m.EntryPoints {
		if blockUsesRayQuery(m.EntryPoints[i].Function.Body) {
			return true
		}
	}
	return false
}

func blockUsesRayQuery(block ir.Block) bool {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtRayQuery:
			return true
		case ir.StmtBlock:
			if blockUsesRayQuery(sk.Block) {
				return true
			}
		case ir.StmtIf:
			if blockUsesRayQuery(sk.Accept) || blockUsesRayQuery(sk.Reject) {
				return true
			}
		case ir.StmtLoop:
			if blockUsesRayQuery(sk.Body) || blockUsesRayQuery(sk.Continuing) {
				return true
			}
		case ir.StmtSwitch:
			for j := range sk.Cases {
				if blockUsesRayQuery(sk.Cases[j].Body) {
					return true
				}
			}
		}
	}
	return false
}

// buildInputToOutputTable computes the PSV0 dependency table for an
// entry point via the IR-level dxil/internal/viewid dataflow analyzer.
//
// The analyzer walks the IR (no DXIL bitcode dependency) and returns
// per-input-component contribution bitmaps in the exact layout
// PSVDependencyTable expects:
//
//	pData[inputComp * MaskDwords(SigOutputVectors)] = bitmap
//	  where inputComp = inputVectorRow * 4 + col
//
// For simple pass-through shaders (e.g. triangle FS: vec4(input.color, 1))
// this produces exactly the bits the DXC validator regenerates from the
// bitcode. For constructs it can't trace (user calls, ray queries) it
// conservatively reports "every input contributes to every output".
//
// Returns nil when the entry point has no signature — the PSV0
// dep-table region is zero-sized in that case.
func buildInputToOutputTable(irMod *ir.Module, ep *ir.EntryPoint, sigs graphicsPSVSigs) []uint32 {
	inElems := psvSigsToViewIDElems(sigs.PSVInputs)
	outElems := psvSigsToViewIDElems(sigs.PSVOutputs)
	if len(inElems) == 0 || len(outElems) == 0 {
		return nil
	}
	deps := viewid.Analyze(irMod, ep, inElems, outElems)
	if deps == nil {
		return nil
	}
	return deps.InputCompToOutputComps
}

// psvSigsToViewIDElems translates the PSV-side PSVSignatureElement list
// into the viewid package's SigElement list. ScalarStart is computed by
// accumulating each element's Cols (low 4 bits of ColsAndStart).
// SystemManaged reflects the Allocated bit (ColsAndStart bit 6) —
// elements with Allocated=0 (SV_Depth, SV_Coverage, SV_SampleIndex on
// input) do not occupy a vector row.
func psvSigsToViewIDElems(elems []container.PSVSignatureElement) []viewid.SigElement {
	out := make([]viewid.SigElement, len(elems))
	var cumScalars uint32
	for i := range elems {
		e := &elems[i]
		cols := uint32(e.ColsAndStart & 0x0F)
		allocated := (e.ColsAndStart>>6)&0x01 != 0
		sysManaged := !allocated
		// VectorRow MUST mirror the actual PSV0 register slot (StartRow)
		// the packer assigned, NOT arg-order. DXC's GetLinearIndex
		// (output `dx.viewIdState` / PSV0 dep table indexing) uses
		// elem.StartRow*4 + col. After BUG-DXIL-029 reordered locations
		// before builtins, an arg-order counter diverged.
		var row, startCol uint32
		if !sysManaged {
			row = uint32(e.StartRow)
			startCol = uint32((e.ColsAndStart >> 4) & 0x03)
		}
		out[i] = viewid.SigElement{
			ScalarStart:   cumScalars,
			NumChannels:   cols,
			VectorRow:     row,
			StartCol:      startCol,
			SystemManaged: sysManaged,
		}
		cumScalars += cols
	}
	return out
}
