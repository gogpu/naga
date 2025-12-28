// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import "fmt"

// ShaderModel represents a DirectX Shader Model version.
// Shader Models define the feature set available for shader compilation.
type ShaderModel uint8

// Supported Shader Model versions.
const (
	// ShaderModel5_0 is the base SM5 version (DirectX 11).
	ShaderModel5_0 ShaderModel = iota

	// ShaderModel5_1 provides improved resource binding (default).
	// This is the recommended minimum for maximum compatibility.
	ShaderModel5_1

	// ShaderModel6_0 introduces wave intrinsics and DXIL.
	ShaderModel6_0

	// ShaderModel6_1 adds SV_ViewID and barycentrics.
	ShaderModel6_1

	// ShaderModel6_2 adds float16 and denorm control.
	ShaderModel6_2

	// ShaderModel6_3 adds DirectX Raytracing (DXR).
	ShaderModel6_3

	// ShaderModel6_4 adds variable rate shading and library subobjects.
	ShaderModel6_4

	// ShaderModel6_5 adds mesh shaders and sampler feedback.
	ShaderModel6_5

	// ShaderModel6_6 adds 64-bit atomics and dynamic resources.
	ShaderModel6_6

	// ShaderModel6_7 adds advanced mesh shaders and work graphs.
	ShaderModel6_7
)

// String returns a human-readable representation of the shader model.
// Example: "SM 5.1", "SM 6.0"
func (sm ShaderModel) String() string {
	major, minor := sm.version()
	return fmt.Sprintf("SM %d.%d", major, minor)
}

// ProfileSuffix returns the shader profile suffix for this model.
// Example: "5_1", "6_0"
// Used to construct profiles like "vs_5_1", "ps_6_0".
func (sm ShaderModel) ProfileSuffix() string {
	major, minor := sm.version()
	return fmt.Sprintf("%d_%d", major, minor)
}

// version returns the major and minor version numbers.
func (sm ShaderModel) version() (major, minor uint8) {
	switch sm {
	case ShaderModel5_0:
		return 5, 0
	case ShaderModel5_1:
		return 5, 1
	case ShaderModel6_0:
		return 6, 0
	case ShaderModel6_1:
		return 6, 1
	case ShaderModel6_2:
		return 6, 2
	case ShaderModel6_3:
		return 6, 3
	case ShaderModel6_4:
		return 6, 4
	case ShaderModel6_5:
		return 6, 5
	case ShaderModel6_6:
		return 6, 6
	case ShaderModel6_7:
		return 6, 7
	default:
		return 5, 1 // Default to 5.1 for unknown
	}
}

// Major returns the major version number.
func (sm ShaderModel) Major() uint8 {
	major, _ := sm.version()
	return major
}

// Minor returns the minor version number.
func (sm ShaderModel) Minor() uint8 {
	_, minor := sm.version()
	return minor
}

// SupportsDXIL returns true if this shader model uses DXIL output.
// Shader Model 6.0+ uses DXIL (DirectX Intermediate Language).
// Earlier models use DXBC (DirectX Bytecode).
func (sm ShaderModel) SupportsDXIL() bool {
	return sm >= ShaderModel6_0
}

// SupportsWaveOps returns true if this shader model supports wave intrinsics.
// Wave operations were introduced in Shader Model 6.0.
func (sm ShaderModel) SupportsWaveOps() bool {
	return sm >= ShaderModel6_0
}

// SupportsMeshShaders returns true if this shader model supports mesh shaders.
// Mesh and amplification shaders were introduced in Shader Model 6.5.
func (sm ShaderModel) SupportsMeshShaders() bool {
	return sm >= ShaderModel6_5
}

// SupportsRayTracing returns true if this shader model supports ray tracing.
// DirectX Raytracing (DXR) was introduced in Shader Model 6.3.
func (sm ShaderModel) SupportsRayTracing() bool {
	return sm >= ShaderModel6_3
}

// Supports64BitAtomics returns true if this shader model supports 64-bit atomics.
// 64-bit atomics were introduced in Shader Model 6.6.
func (sm ShaderModel) Supports64BitAtomics() bool {
	return sm >= ShaderModel6_6
}

// SupportsFloat16 returns true if this shader model supports native float16.
// Native 16-bit floats were introduced in Shader Model 6.2.
func (sm ShaderModel) SupportsFloat16() bool {
	return sm >= ShaderModel6_2
}

// SupportsVariableRateShading returns true if this shader model supports VRS.
// Variable Rate Shading was introduced in Shader Model 6.4.
func (sm ShaderModel) SupportsVariableRateShading() bool {
	return sm >= ShaderModel6_4
}
