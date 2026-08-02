// Package naga provides a Pure Go shader compiler.
//
// naga compiles WGSL (WebGPU Shading Language) source code to multiple output formats:
//   - SPIR-V — Binary format for Vulkan
//   - MSL — Metal Shading Language for macOS/iOS
//   - GLSL — OpenGL Shading Language for OpenGL 3.3+, ES 3.0+
//   - HLSL — High Level Shading Language for DirectX
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

// Capability constants re-exported from ir for convenience.
// See [ir.Capabilities] for full documentation.
const (
	// CapFloat64 enables f64 scalar type.
	CapFloat64 = ir.CapFloat64

	// CapShaderInt64 enables i64 and u64 scalar types.
	CapShaderInt64 = ir.CapShaderInt64

	// CapShaderFloat16 enables f16 scalar type.
	CapShaderFloat16 = ir.CapShaderFloat16

	// CapAll enables all capabilities.
	CapAll = ir.CapAll
)

// CompileOptions configures shader compilation.
type CompileOptions struct {
	// SPIRVVersion is the target SPIR-V version (default: 1.3)
	SPIRVVersion spirv.Version

	// Debug enables debug info in output (OpName, OpLine, etc.)
	Debug bool

	// Validate enables IR validation before code generation
	Validate bool

	// Capabilities controls which extended scalar types are allowed.
	// Zero value rejects f64/i64/u64/f16 (matching Rust naga defaults).
	// Use [CapFloat64], [CapShaderInt64], [CapShaderFloat16] to enable specific types,
	// or [CapAll] to permit everything.
	Capabilities ir.Capabilities
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

	// Lower AST to IR (always permissive — capability enforcement is in the validator)
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, fmt.Errorf("lowering error: %w", err)
	}

	// Validate IR if requested (includes capability checks)
	if opts.Validate {
		validationErrors, err := ir.ValidateWithCapabilities(module, opts.Capabilities)
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
// All capabilities are enabled (permissive mode for tools and tests).
// For strict capability validation, use [LowerWithCapabilities].
//
// The IR is a lower-level representation that includes type information,
// resolved identifiers, and a simpler structure suitable for code generation.
func Lower(ast *wgsl.Module) (*ir.Module, error) {
	return LowerWithSource(ast, "")
}

// LowerWithSource converts WGSL AST to IR, keeping source for error messages.
// All capabilities are enabled (permissive mode for tools and tests).
// For strict capability validation, use [LowerWithCapabilities].
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

// LowerWithCapabilities converts WGSL AST to IR with explicit capability control.
// The lowerer is always permissive; capability validation is performed by the
// validator after lowering. Types that require specific capabilities (f64, i64,
// u64, f16) produce validation errors unless the corresponding flag is set.
func LowerWithCapabilities(ast *wgsl.Module, source string, caps ir.Capabilities) (*ir.Module, error) {
	// Lower permissively — all types are accepted during lowering.
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, err
	}
	// Validate with capability restrictions.
	validationErrors, verr := ir.ValidateWithCapabilities(module, caps)
	if verr != nil {
		return nil, verr
	}
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: %w", &validationErrors[0])
	}
	return module, nil
}

// Validate validates an IR module for correctness with all capabilities enabled.
//
// Validation checks include:
//   - Type consistency
//   - Reference validity (all handles point to valid objects)
//   - Control flow validity (structured control flow rules)
//   - Binding uniqueness (no duplicate @group/@binding)
//
// For capability-restricted validation, use [ir.ValidateWithCapabilities] directly.
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
