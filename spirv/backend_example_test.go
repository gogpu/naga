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
		// A constant's value is its Init: an expression in GlobalExpressions.
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                               // 0
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},                               // 1
			{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},                               // 2
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1, 2}}}, // 3
		},
		Constants: []ir.Constant{
			{Name: "x", Type: 0, Init: 0},        // const x: f32 = 1.0;
			{Name: "y", Type: 0, Init: 1},        // const y: f32 = 2.0;
			{Name: "z", Type: 0, Init: 2},        // const z: f32 = 3.0;
			{Name: "position", Type: 1, Init: 3}, // const position: vec3<f32> = vec3(1.0, 2.0, 3.0);
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

	binary, _, err := spirv.Compile(module, options)
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
	// Binary size: 172 bytes
	// Compilation successful!
}
