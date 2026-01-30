package spirv_test

import (
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
)

// Example_backendCompile demonstrates compiling an IR module to SPIR-V.
func Example_backendCompile() {
	// Create a simple IR module with types and constants
	module := &ir.Module{
		Types: []ir.Type{
			// f32 scalar type
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// vec3<f32> vector type
			{
				Name: "vec3f",
				Inner: ir.VectorType{
					Size:   ir.Vec3,
					Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
				},
			},
		},
		Constants: []ir.Constant{
			// const x: f32 = 1.0;
			{
				Name:  "x",
				Type:  0,                                                      // f32
				Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}, // 1.0
			},
			// const y: f32 = 2.0;
			{
				Name:  "y",
				Type:  0,                                                      // f32
				Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40000000}, // 2.0
			},
			// const z: f32 = 3.0;
			{
				Name:  "z",
				Type:  0,                                                      // f32
				Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40400000}, // 3.0
			},
			// const position: vec3<f32> = vec3(1.0, 2.0, 3.0);
			{
				Name: "position",
				Type: 1, // vec3f
				Value: ir.CompositeValue{
					Components: []ir.ConstantHandle{0, 1, 2},
				},
			},
		},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	// Configure backend options
	options := spirv.Options{
		Version: spirv.Version1_3,
		Debug:   true, // Include debug names
	}

	// Create backend and compile
	backend := spirv.NewBackend(options)
	binary, err := backend.Compile(module)
	if err != nil {
		panic(err)
	}

	// Verify magic number
	magic := uint32(binary[0]) | uint32(binary[1])<<8 | uint32(binary[2])<<16 | uint32(binary[3])<<24
	fmt.Printf("SPIR-V magic: 0x%08x\n", magic)
	fmt.Printf("Binary size: %d bytes\n", len(binary))
	fmt.Println("Compilation successful!")

	// Output:
	// SPIR-V magic: 0x07230203
	// Binary size: 164 bytes
	// Compilation successful!
}
