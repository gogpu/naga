// Package naga provides a Pure Go shader compiler.
//
// naga compiles WGSL (WebGPU Shading Language) source code to multiple output formats:
//   - SPIR-V — Binary format for Vulkan
//   - MSL — Metal Shading Language for macOS/iOS
//   - GLSL — OpenGL Shading Language for OpenGL 3.3+, ES 3.0+
//
// The package provides a simple, high-level API for shader compilation as well as
// lower-level access to individual compilation stages.
//
// Example usage (SPIR-V):
//
//	source := `
//	@vertex
//	fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
//	    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
//	}
//	`
//	spirv, err := naga.Compile(source)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// For MSL output, use the msl package:
//
//	module, _ := naga.Lower(ast)
//	mslCode, info, err := msl.Compile(module, msl.DefaultOptions())
//
// For GLSL output, use the glsl package:
//
//	module, _ := naga.Lower(ast)
//	glslCode, info, err := glsl.Compile(module, glsl.DefaultOptions())
package naga

import (
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// CompileOptions configures shader compilation.
type CompileOptions struct {
	// SPIRVVersion is the target SPIR-V version (default: 1.3)
	SPIRVVersion spirv.Version

	// Debug enables debug info in output (OpName, OpLine, etc.)
	Debug bool

	// Validate enables IR validation before code generation
	Validate bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() CompileOptions {
	return CompileOptions{
		SPIRVVersion: spirv.Version1_3,
		Debug:        false,
		Validate:     true,
	}
}

// Compile compiles WGSL source code to SPIR-V binary using default options.
//
// This is the simplest way to compile a shader. For more control, use CompileWithOptions
// or the individual Parse/Lower/Generate functions.
func Compile(source string) ([]byte, error) {
	return CompileWithOptions(source, DefaultOptions())
}

// CompileWithOptions compiles WGSL source code to SPIR-V binary with custom options.
//
// The compilation pipeline is:
//  1. Parse WGSL source to AST
//  2. Lower AST to IR (intermediate representation)
//  3. Validate IR (if enabled)
//  4. Generate SPIR-V binary
func CompileWithOptions(source string, opts CompileOptions) ([]byte, error) {
	// Parse WGSL to AST
	ast, err := Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Lower AST to IR (pass source for error messages)
	module, err := LowerWithSource(ast, source)
	if err != nil {
		return nil, fmt.Errorf("lowering error: %w", err)
	}

	// Validate IR if requested
	if opts.Validate {
		validationErrors, err := Validate(module)
		if err != nil {
			return nil, fmt.Errorf("validation error: %w", err)
		}
		if len(validationErrors) > 0 {
			return nil, fmt.Errorf("validation failed: %w", &validationErrors[0])
		}
	}

	// Generate SPIR-V
	spirvOpts := spirv.Options{
		Version: opts.SPIRVVersion,
		Debug:   opts.Debug,
	}
	spirvBytes, err := GenerateSPIRV(module, spirvOpts)
	if err != nil {
		return nil, fmt.Errorf("SPIR-V generation error: %w", err)
	}

	return spirvBytes, nil
}

// Parse parses WGSL source code to AST (Abstract Syntax Tree).
//
// This is the first stage of compilation. The AST represents the syntactic
// structure of the shader but does not include semantic information like types.
func Parse(source string) (*wgsl.Module, error) {
	// Tokenize
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("tokenization error: %w", err)
	}

	// Parse to AST
	parser := wgsl.NewParser(tokens)
	module, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return module, nil
}

// Lower converts WGSL AST to IR (Intermediate Representation).
//
// The IR is a lower-level representation that includes type information,
// resolved identifiers, and a simpler structure suitable for code generation.
func Lower(ast *wgsl.Module) (*ir.Module, error) {
	return LowerWithSource(ast, "")
}

// LowerWithSource converts WGSL AST to IR, keeping source for error messages.
//
// When source is provided, errors will include line:column information
// and can show source context using ErrorList.FormatAll().
func LowerWithSource(ast *wgsl.Module, source string) (*ir.Module, error) {
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, err
	}
	return module, nil
}

// Validate validates an IR module for correctness.
//
// Validation checks include:
//   - Type consistency
//   - Reference validity (all handles point to valid objects)
//   - Control flow validity (structured control flow rules)
//   - Binding uniqueness (no duplicate @group/@binding)
//
// Returns a slice of validation errors. If the slice is empty, validation passed.
func Validate(module *ir.Module) ([]ir.ValidationError, error) {
	return ir.Validate(module)
}

// GenerateSPIRV generates SPIR-V binary from IR module.
//
// This is the final stage of compilation. The output is a binary blob
// that can be directly consumed by Vulkan or other SPIR-V consumers.
func GenerateSPIRV(module *ir.Module, opts spirv.Options) ([]byte, error) {
	backend := spirv.NewBackend(opts)
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		return nil, fmt.Errorf("SPIR-V generation error: %w", err)
	}
	return spirvBytes, nil
}
