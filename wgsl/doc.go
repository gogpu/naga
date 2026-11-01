// Package wgsl is the WGSL (WebGPU Shading Language) frontend: [Parse] turns
// WGSL source into an IR module, as Rust naga's front::wgsl.
//
// WGSL is the shader language for WebGPU, designed to be portable
// and map well to modern GPU APIs like Vulkan, Metal, and DX12.
//
// # Usage
//
//	source := `
//	@vertex
//	fn main() -> @builtin(position) vec4<f32> {
//	    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
//	}
//	`
//
//	module, info, err := wgsl.Parse(source, wgsl.ParseOptions{})
//	if err != nil {
//	    var pe *wgsl.ParseError
//	    if errors.As(err, &pe) {
//	        log.Fatal(pe.Format(source, "shader.wgsl"))
//	    }
//	    log.Fatal(err)
//	}
//	for _, w := range info.Warnings {
//	    log.Println(w.Message)
//	}
//
// The module is not validated: ir.Validate checks it against the
// capabilities the device has, and a backend (spirv, msl, glsl, hlsl, dxil)
// compiles it.
//
// # WGSL Specification
//
// This implementation follows the WGSL specification:
// https://www.w3.org/TR/WGSL/
//
// # Supported Features
//
//   - Full lexical analysis
//   - Type declarations (struct, alias)
//   - Function declarations
//   - Variable declarations (var, let, const)
//   - All standard types (scalars, vectors, matrices)
//   - Attributes (@vertex, @fragment, @compute, etc.)
//   - Control flow (if, for, while, loop, switch)
//   - All operators and expressions
//
// # Concurrency
//
// Parse is safe for concurrent use; each call returns a new module.
package wgsl
