// Package spirv provides SPIR-V code generation from naga IR.
//
// SPIR-V is the standard intermediate language for GPU shaders,
// used by Vulkan, OpenCL, and other APIs.
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
// The backend currently supports:
//   - All scalar types (bool, f32, f64, i32, u32, etc.)
//   - Vector types (vec2, vec3, vec4)
//   - Matrix types (mat2x2, mat3x3, mat4x4, etc.)
//   - Array types (fixed-size)
//   - Struct types with member decorations
//   - Pointer types with storage classes
//   - Sampler and image types
//   - Scalar constants (bool, float, int)
//   - Composite constants (vectors, arrays, structs)
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
// # SPIR-V Structure
//
// SPIR-V modules consist of:
//   - Header (magic, version, generator, bound, schema)
//   - Capabilities (required features)
//   - Extensions (optional extensions)
//   - Extended instruction imports (GLSL.std.450, etc.)
//   - Memory model (addressing and memory model)
//   - Entry points (shader entry functions)
//   - Execution modes (shader configuration)
//   - Debug information (names, source info)
//   - Annotations (decorations)
//   - Types and constants
//   - Global variables
//   - Functions (code)
//
// # Implementation Status
//
// Current implementation:
//   - ✅ Types (scalar, vector, matrix, array, struct, pointer, sampler, image)
//   - ✅ Constants (scalar and composite)
//   - ✅ Type deduplication by handle
//   - ✅ Debug names
//   - ✅ Decorations (bindings, offsets)
//
// Next steps:
//   - ⏳ Global variables
//   - ⏳ Entry points with bindings
//   - ⏳ Functions and expressions
//   - ⏳ Control flow (if, loop, switch)
//   - ⏳ Built-in functions
//
// # References
//
// SPIR-V Specification: https://registry.khronos.org/SPIR-V/specs/unified1/SPIRV.html
package spirv
