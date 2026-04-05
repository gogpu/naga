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
// Current status: Phase 0 — bitcode writer and container PoC.
// The Compile function is not yet implemented; it will be added in Phase 1.
//
// Reference implementations:
//   - Mesa's src/microsoft/compiler/ (C, MIT license)
//   - LLVM 3.7 Bitcode Format: https://releases.llvm.org/3.7.1/docs/BitCodeFormat.html
//   - INF-0004 Validator Hashing: https://github.com/microsoft/hlsl-specs
package dxil

import (
	"fmt"

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
// This function is not yet implemented (Phase 0). It will be available
// in Phase 1 when the naga IR to DXIL lowering is complete.
func Compile(_ *ir.Module, _ Options) ([]byte, error) {
	return nil, fmt.Errorf("dxil: backend not yet implemented (Phase 0)")
}
