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
//	binary, info, err := spirv.Compile(irModule, spirv.DefaultOptions())
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Binary Writer
//
// The package also provides a low-level binary writer for constructing
// SPIR-V modules programmatically using ModuleBuilder:
//
//	builder := spirv.NewModuleBuilder(spirv.Version1_3)
//	builder.AddCapability(spirv.CapabilityShader)
//	builder.SetMemoryModel(spirv.AddressingModelLogical, spirv.MemoryModelGLSL450)
//
//	// Add types
//	floatType := builder.AddTypeFloat(32)
//	vec4Type := builder.AddTypeVector(floatType, 4)
//
//	// Build binary
//	binary := builder.Build()
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
