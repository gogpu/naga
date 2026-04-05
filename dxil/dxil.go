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
	c.AddHashPart()

	containerData := c.Bytes()

	// Apply BYPASS hash if requested.
	if opts.UseBypassHash {
		container.SetBypassHash(containerData)
	}

	return containerData, nil
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
