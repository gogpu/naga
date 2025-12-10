package spirv_test

import (
	"fmt"

	"github.com/gogpu/naga/spirv"
)

// ExampleModuleBuilder_minimal demonstrates creating a minimal SPIR-V module.
func ExampleModuleBuilder_minimal() {
	// Create a module builder targeting SPIR-V 1.3
	builder := spirv.NewModuleBuilder(spirv.Version1_3)

	// Add required capability
	builder.AddCapability(spirv.CapabilityShader)

	// Set memory model (required for all modules)
	builder.SetMemoryModel(spirv.AddressingModelLogical, spirv.MemoryModelGLSL450)

	// Build the binary
	binary := builder.Build()

	fmt.Printf("Generated SPIR-V module: %d bytes\n", len(binary))
	// Output: Generated SPIR-V module: 40 bytes
}

// ExampleModuleBuilder_withTypes demonstrates creating types.
func ExampleModuleBuilder_withTypes() {
	builder := spirv.NewModuleBuilder(spirv.Version1_3)
	builder.AddCapability(spirv.CapabilityShader)
	builder.SetMemoryModel(spirv.AddressingModelLogical, spirv.MemoryModelGLSL450)

	// Create basic types
	voidType := builder.AddTypeVoid()
	floatType := builder.AddTypeFloat(32)
	vec4Type := builder.AddTypeVector(floatType, 4)

	// Add debug names
	builder.AddName(floatType, "float")
	builder.AddName(vec4Type, "vec4")

	binary := builder.Build()

	fmt.Printf("void=%d float=%d vec4=%d size=%d\n", voidType, floatType, vec4Type, len(binary))
	// Output: void=1 float=2 vec4=3 size=108
}

// ExampleModuleBuilder_fragmentShader demonstrates a simple fragment shader structure.
func ExampleModuleBuilder_fragmentShader() {
	builder := spirv.NewModuleBuilder(spirv.Version1_3)

	// Capabilities and memory model
	builder.AddCapability(spirv.CapabilityShader)
	builder.SetMemoryModel(spirv.AddressingModelLogical, spirv.MemoryModelGLSL450)

	// Types
	voidType := builder.AddTypeVoid()
	vec4Type := builder.AddTypeVector(builder.AddTypeFloat(32), 4)
	vec4PtrOutput := builder.AddTypePointer(spirv.StorageClassOutput, vec4Type)
	funcType := builder.AddTypeFunction(voidType)

	// Output variable
	outputVar := builder.AddVariable(vec4PtrOutput, spirv.StorageClassOutput)
	builder.AddName(outputVar, "fragColor")
	builder.AddDecorate(outputVar, spirv.DecorationLocation, 0)

	// Main function
	mainFunc := builder.AddFunction(funcType, voidType, spirv.FunctionControlNone)
	builder.AddName(mainFunc, "main")
	builder.AddLabel()
	builder.AddReturn()
	builder.AddFunctionEnd()

	// Entry point
	builder.AddEntryPoint(spirv.ExecutionModelFragment, mainFunc, "main", []uint32{outputVar})
	builder.AddExecutionMode(mainFunc, spirv.ExecutionModeOriginUpperLeft)

	binary := builder.Build()

	fmt.Printf("Fragment shader: %d bytes\n", len(binary))
	// Output: Fragment shader: 244 bytes
}
