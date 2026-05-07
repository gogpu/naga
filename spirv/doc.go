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
// The Backend translates naga IR modules to SPIR-V binary format:
//
//	backend := spirv.NewBackend(spirv.DefaultOptions())
//	binary, err := backend.Compile(irModule)
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
package spirv
