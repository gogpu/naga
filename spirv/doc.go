// Package spirv provides SPIR-V code generation from naga IR.
//
// SPIR-V is the standard intermediate language for GPU shaders,
// used by Vulkan, OpenCL, and other APIs.
//
// # Architecture
//
// This package is a thin public API layer. All codegen, builder, and spec
// logic lives in spirv/internal/codegen (following the DXIL pattern).
//
// # IR to SPIR-V Backend
//
// Compile translates naga IR modules to SPIR-V binary format:
//
//	binary, info, err := spirv.Compile(irModule, spirv.Options{})
//	if err != nil {
//		log.Fatal(err)
//	}
//
// The binary writer, opcodes and the other SPIR-V enumerations are internal,
// as in Rust naga (back::spv); the public surface is Compile, its Options and
// TranslationInfo, Version, Capability and the binding map types.
//
// # References
//
// SPIR-V Specification: https://registry.khronos.org/SPIR-V/specs/unified1/SPIRV.html
//
// # Concurrency
//
// Every exported function is safe for concurrent use. Compile never modifies
// its input module, so one module can be compiled for several targets at once.
package spirv
