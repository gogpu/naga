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

	"github.com/gogpu/naga/dxil/internal/container"
	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/dxil/internal/module"
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

	// UseBypassHash uses the BYPASS sentinel hash instead of the
	// retail validator hash. Requires AgilitySDK 1.615+ (January 2025).
	// Default: true.
	UseBypassHash bool
}

// DefaultOptions returns the default DXIL compilation options.
func DefaultOptions() Options {
	return Options{
		ShaderModel:   SM6_0,
		UseBypassHash: true,
	}
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

	// Auto-upgrade shader model for features that require higher versions.
	// Mesh/amplification shaders require SM 6.5 minimum.
	ep := &irModule.EntryPoints[0]
	smMinor := opts.ShaderModel.Minor
	if ep.Stage == ir.StageMesh || ep.Stage == ir.StageTask {
		if smMinor < 5 {
			smMinor = 5
		}
	}

	// Step 1: Emit naga IR -> DXIL module.
	emitOpts := emit.EmitOptions{
		ShaderModelMajor: opts.ShaderModel.Major,
		ShaderModelMinor: smMinor,
	}
	mod, err := emit.Emit(irModule, emitOpts)
	if err != nil {
		return nil, fmt.Errorf("dxil: emit: %w", err)
	}

	// Step 2: Serialize DXIL module -> LLVM 3.7 bitcode.
	bitcodeData := module.Serialize(mod)
	if len(bitcodeData) == 0 {
		return nil, fmt.Errorf("dxil: serialization produced empty bitcode")
	}

	// Step 3: Wrap in DXBC container.
	// Parts are ordered to match DXC reference: SFI0, ISG1, OSG1, [PSG1], PSV0, HASH, DXIL.
	shaderKind := stageToContainerKind(ep.Stage)
	isFragment := ep.Stage == ir.StageFragment
	isMesh := ep.Stage == ir.StageMesh
	inputSig, outputSig, primSig := buildSignaturesEx(irModule, ep, isFragment, isMesh)

	c := container.New()
	c.AddFeaturesPart(0) // SFI0

	// ISG1, OSG1, PSG1 — mesh shaders always need these (even empty).
	if isMesh || len(inputSig) > 0 {
		c.AddInputSignature(inputSig)
	}
	if isMesh || len(outputSig) > 0 {
		c.AddOutputSignature(outputSig)
	}
	if len(primSig) > 0 {
		c.AddPrimitiveSignature(primSig)
	}

	c.AddPSV0(buildPSVEx(irModule, ep, isFragment, isMesh, len(inputSig), len(outputSig), len(primSig)))
	c.AddHashPart()
	c.AddDXILPart(shaderKind, opts.ShaderModel.Major, smMinor, bitcodeData) // DXIL last

	containerData := c.Bytes()

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
func buildSignatures(irMod *ir.Module, ep *ir.EntryPoint, isFragment bool) ([]container.SignatureElement, []container.SignatureElement) {
	var inputs, outputs []container.SignatureElement
	register := uint32(0)

	// Inputs from function arguments.
	for _, arg := range ep.Function.Arguments {
		if arg.Binding != nil {
			elems := bindingToSignatureElements(irMod, *arg.Binding, arg.Type, register, false, isFragment)
			inputs = append(inputs, elems...)
			register += uint32(len(elems)) //nolint:gosec // bounded by argument count
			continue
		}
		// Struct-typed arguments have per-member bindings.
		argType := irMod.Types[arg.Type]
		if st, ok := argType.Inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				elems := bindingToSignatureElements(irMod, *member.Binding, member.Type, register, false, isFragment)
				inputs = append(inputs, elems...)
				register += uint32(len(elems)) //nolint:gosec // bounded by struct members
			}
		}
	}

	// Outputs from function result.
	if ep.Function.Result != nil {
		resultType := irMod.Types[ep.Function.Result.Type]
		if st, ok := resultType.Inner.(ir.StructType); ok {
			// Struct result: each member is a separate output.
			outReg := uint32(0)
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				elems := bindingToSignatureElements(irMod, *member.Binding, member.Type, outReg, true, isFragment)
				outputs = append(outputs, elems...)
				outReg += uint32(len(elems)) //nolint:gosec // bounded by struct members
			}
		} else if ep.Function.Result.Binding != nil {
			// Scalar/vector result with binding.
			outputs = bindingToSignatureElements(irMod, *ep.Function.Result.Binding, ep.Function.Result.Type, 0, true, isFragment)
		}
	}

	return inputs, outputs
}

// bindingToSignatureElements converts a naga binding to signature elements.
func bindingToSignatureElements(irMod *ir.Module, binding ir.Binding, typeHandle ir.TypeHandle, register uint32, isOutput, isFragment bool) []container.SignatureElement {
	var sem container.SemanticMapping
	if isOutput {
		sem = container.MapBindingToSemantic(binding, true, isFragment)
	} else {
		sem = container.MapBindingToSemantic(binding, false, isFragment)
	}

	// Derive component type and mask from the IR type.
	compType := container.CompTypeFloat32
	mask := uint8(0x0F) // xyzw default (vec4)
	if irMod != nil && int(typeHandle) < len(irMod.Types) {
		t := irMod.Types[typeHandle]
		compType, mask = sigCompTypeAndMask(t.Inner)
	}
	// Fragment shader SV_Position is always float32 vec4 regardless of type declaration.
	if sem.SystemValue == container.SVPosition && isFragment && !isOutput {
		compType = container.CompTypeFloat32
		mask = 0x0F
	}

	elem := container.SignatureElement{
		SemanticName:  sem.SemanticName,
		SemanticIndex: sem.SemanticIndex,
		SystemValue:   sem.SystemValue,
		CompType:      compType,
		Register:      register,
		Mask:          mask,
		RWMask:        mask,
	}

	return []container.SignatureElement{elem}
}

// sigCompTypeAndMask returns the DXIL signature component type and mask for an IR type.
func sigCompTypeAndMask(inner ir.TypeInner) (container.ProgSigCompType, uint8) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return sigCompTypeForScalarKind(t.Kind), 0x01
	case ir.VectorType:
		ct := sigCompTypeForScalarKind(t.Scalar.Kind)
		mask := uint8((1 << t.Size) - 1)
		return ct, mask
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
func buildPSV(ep *ir.EntryPoint, isFragment bool, inputCount, outputCount int) container.PSVInfo {
	info := container.PSVInfo{
		ShaderStage:       container.PSVVertex,
		MinWaveLaneCount:  0,
		MaxWaveLaneCount:  0xFFFFFFFF,
		SigInputElements:  uint8(inputCount),  //nolint:gosec // bounded by entry point args
		SigOutputElements: uint8(outputCount), //nolint:gosec // bounded by entry point results
	}
	if isFragment {
		info.ShaderStage = container.PSVPixel
	} else {
		// Vertex shader: check if SV_Position is in outputs.
		if ep.Function.Result != nil {
			info.OutputPositionPresent = true
		}
	}
	return info
}

// buildSignaturesEx extracts input, output, and primitive output signature
// elements from an entry point. Mesh shaders have vertex outputs in OSG1
// and primitive outputs in PSG1.
//
//nolint:gocognit,nestif // mesh output signature extraction requires deep type inspection
func buildSignaturesEx(irMod *ir.Module, ep *ir.EntryPoint, isFragment, isMesh bool) ([]container.SignatureElement, []container.SignatureElement, []container.SignatureElement) {
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
func buildPSVEx(irMod *ir.Module, ep *ir.EntryPoint, isFragment, isMesh bool, inputCount, outputCount, primOutputCount int) container.PSVInfo {
	if !isMesh {
		return buildPSV(ep, isFragment, inputCount, outputCount)
	}

	info := container.PSVInfo{
		ShaderStage:                 container.PSVMesh,
		MinWaveLaneCount:            0,
		MaxWaveLaneCount:            0xFFFFFFFF,
		SigInputElements:            uint8(inputCount),      //nolint:gosec // bounded by entry point args
		SigOutputElements:           uint8(outputCount),     //nolint:gosec // bounded by mesh vertex outputs
		SigPatchConstOrPrimElements: uint8(primOutputCount), //nolint:gosec // bounded by mesh primitive outputs
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
