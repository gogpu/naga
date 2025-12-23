// Package msl implements Metal Shading Language (MSL) code generation for naga.
//
// MSL is Apple's shader language for the Metal graphics API. It is based on C++14
// with extensions for GPU programming, including explicit address spaces, attribute-based
// parameter binding, and a metal:: namespace for standard library functions.
//
// # Usage
//
// To compile a WGSL shader to MSL:
//
//	module, err := wgsl.Parse(source)
//	if err != nil {
//	    return err
//	}
//
//	options := msl.Options{
//	    LangVersion: msl.Version{Major: 2, Minor: 1},
//	}
//
//	mslCode, err := msl.Compile(module, options)
//	if err != nil {
//	    return err
//	}
//
// # MSL Language Versions
//
// The backend supports MSL 1.2 through 3.0. Features used depend on the target version:
//   - MSL 1.2: Basic shaders, most texture operations
//   - MSL 2.0: Tessellation, indirect command buffers
//   - MSL 2.1: Improved array handling
//   - MSL 2.3: Ray tracing, 64-bit atomics
//   - MSL 3.0: Mesh shaders, extended features
//
// # Type Mapping
//
// WGSL types map to MSL as follows:
//
//	WGSL           MSL
//	----           ---
//	bool           bool
//	i32            int
//	u32            uint
//	f32            float
//	f16            half
//	vec2<T>        metal::T2
//	vec3<T>        metal::T3
//	vec4<T>        metal::T4
//	mat4x4<f32>    metal::float4x4
//	array<T, N>    array<T, N>  (wrapped in struct)
//	texture_2d     metal::texture2d<float>
//	sampler        metal::sampler
//
// # Address Spaces
//
// WGSL address spaces map to MSL as:
//
//	uniform    -> constant
//	storage    -> device
//	private    -> thread
//	workgroup  -> threadgroup
//	function   -> thread (stack)
//
// # Entry Points
//
// Entry points are generated with appropriate stage keywords:
//   - vertex: Vertex shaders with [[stage_in]], [[vertex_id]], etc.
//   - fragment: Fragment shaders with [[position]], [[color(N)]], etc.
//   - kernel: Compute shaders with [[thread_position_in_grid]], etc.
//
// # Helper Functions
//
// Some WGSL operations require polyfill functions in MSL:
//   - _naga_div: Safe integer division (handles zero)
//   - _naga_mod: Safe integer modulo (handles zero)
//   - _naga_modf: modf with WGSL-compatible result struct
//   - _naga_frexp: frexp with WGSL-compatible result struct
package msl
