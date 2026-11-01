// Package naga provides a Pure Go shader compiler.
//
// naga compiles WGSL (WebGPU Shading Language) source code to multiple output formats:
//   - SPIR-V — Binary format for Vulkan
//   - MSL — Metal Shading Language for macOS/iOS
//   - GLSL — OpenGL Shading Language for OpenGL 3.3+, ES 3.0+
//   - HLSL — High Level Shading Language for DirectX
//   - DXIL — DirectX Intermediate Language for Direct3D 12
//
// This package is the one-call path from WGSL to SPIR-V, plus pipeline
// constants (ProcessOverrides) and module compaction (Compact). Each
// language has its own package: a frontend's Parse (wgsl.Parse) gives an IR
// module, ir.Validate checks it, and a backend's Compile (spirv, msl, glsl,
// hlsl, dxil) writes it.
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
// For MSL output:
//
//	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
//	...
//	if _, err := ir.Validate(module, ir.ValidateOptions{}); err != nil { ... }
//	mslCode, info, err := msl.Compile(module, msl.Options{})
//
// # Concurrency
//
// Every exported function is safe for concurrent use. ProcessOverrides never
// modifies its input module; its result may share unmodified parts of it.
// Compact is the one function that modifies the module it is given, in
// place: no other goroutine may use that module while it runs.
package naga

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// CompileOptions configures shader compilation.
type CompileOptions struct {
	// SPIRV is the SPIR-V backend's options, all of them. Its zero value is
	// the safe set: SPIR-V 1.0, Restrict bounds checks, bounded loops,
	// workgroup memory zeroed, every binding required. A shader that needs
	// a newer SPIR-V version (subgroup operations need 1.3) is an error
	// until SPIRV.Version gives one; the backend never raises it.
	SPIRV spirv.Options

	// Capabilities is what the IR validator allows beyond the core: the
	// zero value rejects f64, i64, u64 and f16, as Rust naga's default
	// does. Use [ir.CapFloat64], [ir.CapShaderInt64], [ir.CapShaderFloat16]
	// to enable one, or [ir.CapAll] to permit everything.
	Capabilities ir.Capabilities

	// SkipValidation compiles the module without validating it first. The
	// zero value validates, as Rust naga's CLI does.
	SkipValidation bool
}

// DefaultOptions returns the zero CompileOptions, the safe defaults.
func DefaultOptions() CompileOptions {
	return CompileOptions{}
}

// Compile compiles WGSL source code to SPIR-V binary using default options.
func Compile(source string) ([]byte, error) {
	return CompileWithOptions(source, DefaultOptions())
}

// CompileWithOptions compiles WGSL source code to SPIR-V binary with custom options.
//
// The compilation pipeline is:
//  1. Parse WGSL to IR (wgsl.Parse); a *wgsl.ParseError is returned wrapped,
//     with its line and column
//  2. Validate IR (if enabled)
//  3. Replace overrides by their defaults (ProcessOverrides with no pipeline
//     constants), as Rust naga's CLI does; an override without a default is
//     an error. For pipeline constants use wgsl.Parse, ProcessOverrides and a
//     backend.
//  4. Generate SPIR-V binary
func CompileWithOptions(source string, opts CompileOptions) ([]byte, error) {
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		var pe *wgsl.ParseError
		if errors.As(err, &pe) {
			if loc, ok := pe.Location(source); ok {
				return nil, fmt.Errorf("wgsl:%d:%d: %w", loc.Line, loc.Column, err)
			}
		}
		return nil, fmt.Errorf("wgsl: %w", err)
	}

	// Validate IR unless skipped (includes capability checks)
	if !opts.SkipValidation {
		if _, err := ir.Validate(module, ir.ValidateOptions{Capabilities: opts.Capabilities}); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}

	module, err = ProcessOverrides(module, OverrideOptions{})
	if err != nil {
		return nil, fmt.Errorf("pipeline constants: %w", err)
	}

	spirvBytes, _, err := spirv.Compile(module, opts.SPIRV)
	if err != nil {
		return nil, fmt.Errorf("SPIR-V generation error: %w", err)
	}
	return spirvBytes, nil
}
