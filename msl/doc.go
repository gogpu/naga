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
// Options.LangVersion is the version the output is written for (zero: 2.1).
// The backend never raises it: a shader that needs a newer MSL is refused
// with a *backend.VersionError, as Rust naga refuses it. The versions some
// features need:
//   - MSL 1.2: @builtin(instance_index), @blend_src, reverseBits,
//     extractBits, insertBits, writable storage buffers in fragment shaders,
//     writable storage textures in vertex and fragment shaders, read-write
//     storage textures
//   - MSL 2.0: binding arrays of samplers, textures and read-only or
//     write-only storage textures
//   - MSL 2.1: @invariant
//   - MSL 2.2: @builtin(view_index)
//   - MSL 2.3: @builtin(primitive_index), @builtin(barycentric)
//   - MSL 2.4: ray queries
//   - MSL 3.0: binding arrays of read-write storage textures, mesh shaders
//   - MSL 3.1: 64-bit storage textures (r64uint, r64sint)
//   - MSL 4.0: @interpolate(per_vertex)
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
//   - naga_div: Safe integer division (typed overloads, handles zero and INT_MIN/-1)
//   - naga_mod: Safe integer modulo (typed overloads, handles zero and INT_MIN/-1)
//   - naga_modf: modf with WGSL-compatible result struct
//   - naga_frexp: frexp with WGSL-compatible result struct
//   - _mslBufferSizes: Struct for runtime-sized array buffer sizes
//
// # Concurrency
//
// Every exported function is safe for concurrent use. Compile never modifies
// its input module, so one module can be compiled for several targets at once.
package msl
