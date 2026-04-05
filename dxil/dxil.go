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

	// Step 1: Emit naga IR -> DXIL module.
	emitOpts := emit.EmitOptions{
		ShaderModelMajor: opts.ShaderModel.Major,
		ShaderModelMinor: opts.ShaderModel.Minor,
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
	ep := &irModule.EntryPoints[0]
	shaderKind := stageToContainerKind(ep.Stage)

	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(shaderKind, 1, opts.ShaderModel.Minor, bitcodeData)

	// Step 4: Add I/O signatures and pipeline state.
	isFragment := ep.Stage == ir.StageFragment
	inputSig, outputSig := buildSignatures(irModule, ep, isFragment)
	if len(inputSig) > 0 {
		c.AddInputSignature(inputSig)
	}
	if len(outputSig) > 0 {
		c.AddOutputSignature(outputSig)
	}
	c.AddPSV0(buildPSV(ep, isFragment, len(inputSig), len(outputSig)))
	c.AddHashPart()

	containerData := c.Bytes()

	// Apply BYPASS hash if requested.
	if opts.UseBypassHash {
		container.SetBypassHash(containerData)
	}

	return containerData, nil
}

// buildSignatures extracts input/output signature elements from an entry point.
func buildSignatures(irMod *ir.Module, ep *ir.EntryPoint, isFragment bool) ([]container.SignatureElement, []container.SignatureElement) {
	var inputs, outputs []container.SignatureElement
	register := uint32(0)

	// Inputs from function arguments.
	for _, arg := range ep.Function.Arguments {
		if arg.Binding == nil {
			continue
		}
		elems := bindingToSignatureElements(irMod, *arg.Binding, arg.Type, register, false, isFragment)
		inputs = append(inputs, elems...)
		register += uint32(len(elems)) //nolint:gosec // bounded by argument count
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
func bindingToSignatureElements(_ *ir.Module, binding ir.Binding, typeHandle ir.TypeHandle, register uint32, isOutput, isFragment bool) []container.SignatureElement {
	var sem container.SemanticMapping
	if isOutput {
		sem = container.MapBindingToSemantic(binding, true, isFragment)
	} else {
		sem = container.MapBindingToSemantic(binding, false, isFragment)
	}

	// Default: float32, xyzw mask for vec4.
	elem := container.SignatureElement{
		SemanticName:  sem.SemanticName,
		SemanticIndex: sem.SemanticIndex,
		SystemValue:   sem.SystemValue,
		CompType:      container.CompTypeFloat32,
		Register:      register,
		Mask:          0x0F, // xyzw
		RWMask:        0x0F,
	}
	_ = typeHandle // TODO: refine mask/compType from actual type

	return []container.SignatureElement{elem}
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
	default:
		return uint32(module.VertexShader)
	}
}
