# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.1] - 2025-12-29

### Fixed

#### WGSL Lowering
- **clamp() built-in function** — Added missing `clamp` to math function map
  - Root cause: `getMathFunction()` was missing `clamp` → `ir.MathClamp` mapping
  - Caused "unknown function: clamp" error during shader compilation
  - Affected any WGSL shader using `clamp(value, min, max)`

### Added
- **Comprehensive math function tests** — `TestMathFunctions` covering all 12 WGSL built-in math functions
  - Tests: abs, min, max, clamp, sin, cos, tan, sqrt, length, normalize, dot, cross
  - Verifies correct IR generation for each function

## [0.8.0] - 2025-12-28

Code quality improvements and SPIR-V backend bug fixes.

### Fixed

#### SPIR-V Backend
- **sign() type checking** — Now correctly uses `SSign` for signed integers vs `FSign` for floats
- **atomicMin/Max signed vs unsigned** — Now correctly uses `OpAtomicSMin`/`OpAtomicSMax` for signed integers and `OpAtomicUMin`/`OpAtomicUMax` for unsigned

#### WGSL Frontend
- **Function resolution** — Added pre-registration pass for forward function references
- **Return type attributes** — Parser now correctly handles attributes on return types (e.g., `@builtin(position)`)

### Changed
- Removed dead `Write()` method from SPIR-V writer
- Removed unused `module` field from `spirv.Writer` struct
- Code cleanup in `hlsl/types.go` nolint directives

## [0.7.0] - 2025-12-28

HLSL backend for DirectX shader compilation (~8.8K new LOC).

### Added

#### HLSL Backend (DirectX)
- `hlsl/backend.go` — Public API: `Options`, `TranslationInfo`, `Compile()`
  - DXC-first strategy (Shader Model 6.0+)
  - FXC compatibility mode (Shader Model 5.1)
  - Vertex, fragment, and compute shader support
- `hlsl/writer.go` — HLSL code generation writer (~400 LOC)
- `hlsl/types.go` — Type generation (~500 LOC)
  - Scalars: float, half, double, int, uint, bool
  - Vectors: float2, float3, float4, int*, uint*
  - Matrices: float2x2, float3x3, float4x4
  - Structs with HLSL semantics
- `hlsl/expressions.go` — Expression code generation (~1100 LOC)
  - Literals, binary/unary operations
  - Access expressions (array, struct, swizzle)
  - 70+ HLSL intrinsic functions
  - Texture sampling: Sample, SampleLevel, SampleBias, SampleGrad, Gather
  - Derivatives: ddx, ddy, fwidth (coarse/fine variants)
- `hlsl/statements.go` — Statement code generation (~600 LOC)
  - Control flow (if, switch, loop, for)
  - GPU barriers (GroupMemoryBarrier, DeviceMemoryBarrier, AllMemoryBarrier)
  - Return, discard, break, continue
- `hlsl/storage.go` — Buffer and atomic operations (~500 LOC)
  - ByteAddressBuffer, RWByteAddressBuffer
  - StructuredBuffer<T>, RWStructuredBuffer<T>
  - cbuffer for uniforms
  - Atomics: InterlockedAdd, And, Or, Xor, Min, Max, Exchange, CompareExchange
- `hlsl/functions.go` — Entry point generation (~500 LOC)
  - Input/output structs with HLSL semantics (SV_Position, TEXCOORD, SV_Target)
  - `[numthreads(x,y,z)]` for compute shaders
  - Helper functions for safe math operations
- `hlsl/keywords.go` — HLSL reserved word escaping (200+ keywords)
- `hlsl/conv.go` — IR to HLSL type/semantic conversion
- `hlsl/namer.go` — Identifier mangling for HLSL compliance
- `hlsl/errors.go` — HLSL-specific error types
- `hlsl/shader_model.go` — Shader Model version handling
- `hlsl/bind_target.go` — Register binding management (b/t/s/u)

### Notes
- HLSL backend enables DirectX GPU rendering on Windows
- Supports DirectX 11 (SM 5.1) and DirectX 12 (SM 6.0+)
- Total: ~8800 lines of code

## [0.6.0] - 2025-12-25

GLSL backend for OpenGL shader compilation (~2.8K new LOC).

### Added

#### OpenGL Shading Language Backend
- `glsl/backend.go` — Public API: `Options`, `TranslationInfo`, `Compile()`
  - `GLSLVersion` configuration (GLSL 330, 400, 450, ES 300, ES 310)
  - Vertex, fragment, and compute shader support
- `glsl/writer.go` — GLSL code generation writer
- `glsl/types.go` — Type generation (~300 LOC)
  - Scalars: float, int, uint, bool
  - Vectors: vec2, vec3, vec4, ivec*, uvec*, bvec*
  - Matrices: mat2, mat3, mat4, mat2x3, etc.
  - Arrays with fixed size
  - Textures: sampler2D, sampler3D, samplerCube
- `glsl/expressions.go` — Expression code generation (~400 LOC)
  - Literals, binary/unary operations
  - Access expressions (array, struct, swizzle)
  - GLSL built-in function calls
- `glsl/statements.go` — Statement code generation (~300 LOC)
  - Variable declarations
  - Control flow (if, for, while, loop)
  - Assignments and function calls
- `glsl/functions.go` — Entry point generation (~400 LOC)
  - `void main()` with layout qualifiers
  - Vertex: `layout(location = N) in/out`
  - Fragment: `layout(location = N) out`
  - Compute: `layout(local_size_x/y/z)` workgroup size
- `glsl/keywords.go` — GLSL reserved word escaping (183 keywords)
- `glsl/backend_test.go` — Comprehensive unit tests (40+ tests)

### Changed
- README.md updated with GLSL backend documentation
- Architecture section now includes GLSL backend structure

### Notes
- GLSL backend enables OpenGL GPU rendering on all platforms
- Supports OpenGL 3.3+, OpenGL ES 3.0+
- Required by wgpu GLES backend for Linux/embedded platforms

## [0.5.0] - 2025-12-23

MSL backend for Metal shader compilation (~3.6K new LOC).

### Added

#### Metal Shading Language Backend
- `msl/backend.go` — Public API: `Options`, `TranslationInfo`, `Compile()`
- `msl/writer.go` — MSL code generation writer
- `msl/types.go` — Type generation (~400 LOC)
  - Scalars: float, half, int, uint, bool
  - Vectors: float2, float3, float4, etc.
  - Matrices: float2x2, float3x3, float4x4
  - Arrays with fixed size
  - Textures: texture2d, texture3d, texturecube
  - Samplers: sampler
- `msl/expressions.go` — Expression code generation (~600 LOC)
  - Literals, binary/unary operations
  - Access expressions (array, struct, swizzle)
  - Math function calls
- `msl/statements.go` — Statement code generation (~350 LOC)
  - Variable declarations
  - Control flow (if, for, while, loop)
  - Assignments and function calls
- `msl/functions.go` — Entry point generation (~500 LOC)
  - `[[vertex]]` for vertex shaders
  - `[[fragment]]` for fragment shaders
  - `[[kernel]]` for compute shaders
  - Stage input/output structs
- `msl/keywords.go` — MSL/C++ reserved word escaping
- `msl/backend_test.go` — Unit tests for MSL compilation

### Changed
- Pre-release check script now uses kolkov/racedetector (Pure Go, no CGO)
- Updated ecosystem: gogpu v0.5.0 (macOS Cocoa), wgpu v0.6.0 (Metal backend)

### Notes
- MSL backend enables Metal GPU rendering on macOS/iOS
- Required by wgpu v0.6.0 Metal backend

## [0.4.0] - 2025-12-12

Compute shader support with atomics, barriers, and developer experience improvements (~2K new LOC).

### Added

#### Compute Shader Infrastructure
- `wgsl/parser.go` — Access mode parsing for storage buffers
  - `var<storage, read>` — Read-only storage buffer
  - `var<storage, read_write>` — Read-write storage buffer
  - `var<workgroup>` — Workgroup shared memory
- `wgsl/lower.go` — Workgroup size extraction from `@workgroup_size` attribute
- `ir/ir.go` — `AtomicType` for `atomic<u32>` and `atomic<i32>`

#### Atomic Operations
- `wgsl/lower.go` — Atomic function lowering (~150 LOC)
  - `atomicAdd(&ptr, value)` — Atomic addition
  - `atomicSub(&ptr, value)` — Atomic subtraction
  - `atomicMin(&ptr, value)` — Atomic minimum
  - `atomicMax(&ptr, value)` — Atomic maximum
  - `atomicAnd(&ptr, value)` — Atomic bitwise AND
  - `atomicOr(&ptr, value)` — Atomic bitwise OR
  - `atomicXor(&ptr, value)` — Atomic bitwise XOR
  - `atomicExchange(&ptr, value)` — Atomic exchange
  - `atomicCompareExchangeWeak(&ptr, cmp, val)` — Compare and exchange
- `spirv/backend.go` — SPIR-V atomic emission (~100 LOC)
  - `OpAtomicIAdd`, `OpAtomicISub`, `OpAtomicAnd`, `OpAtomicOr`, `OpAtomicXor`
  - `OpAtomicUMin`, `OpAtomicUMax`, `OpAtomicExchange`, `OpAtomicCompareExch`
- `ir/expression.go` — `ExprAtomicResult` for atomic operation results

#### Workgroup Barriers
- `wgsl/lower.go` — Barrier function lowering
  - `workgroupBarrier()` — Synchronize workgroup threads
  - `storageBarrier()` — Memory barrier for storage buffers
  - `textureBarrier()` — Memory barrier for textures
- `spirv/backend.go` — `OpControlBarrier` emission with memory semantics

#### Address-of and Dereference Operators
- `wgsl/lower.go` — `&` and `*` operator handling
  - `&var` — Returns pointer (no-op for storage variables)
  - `*ptr` — Creates `ExprLoad` for dereferencing

#### Unused Variable Warnings
- `wgsl/lower.go` — Warning infrastructure (~50 LOC)
  - `Warning` type with message and source span
  - `LowerResult` struct containing module and warnings
  - `LowerWithWarnings()` API for accessing warnings
  - Variables prefixed with `_` are intentionally ignored
  - `checkUnusedVariables()` called after each function

#### Better Error Messages
- `wgsl/errors.go` — `SourceError` type with source location
- `wgsl/errors.go` — `FormatWithContext()` for pretty error display
- `wgsl/lower.go` — `LowerWithSource()` preserves source for errors

### Changed
- `spirv/spirv.go` — Added SPIR-V opcodes for atomics and barriers
- Total: 203 tests across all packages (+79 from v0.3.0)

### Fixed
- Type switch in `emitAtomic` now uses assignment form (gocritic fix)

## [0.3.0] - 2025-12-11

`let` type inference, array initialization, and texture sampling (~3K new LOC).

### Added

#### Type Inference for `let` Bindings
- `wgsl/lower.go` — `inferTypeFromExpression()` method (~80 LOC)
  - Supports inferring type from any expression
  - `let x = 1.0` → inferred f32
  - `let v = vec3(1.0)` → inferred vec3<f32>
  - `let n = normalize(v)` → inferred from function return type
- `wgsl/lower_type_inference_test.go` — 6 new tests

#### Array Initialization Syntax
- `wgsl/lower.go` — Array constructor handling (~50 LOC)
  - `array(1, 2, 3)` shorthand with inferred type and size
  - `array<f32, 3>(...)` explicit syntax
  - Element type inferred from first element
- Tests for array shorthand and vector arrays

#### Texture Sampling Operations
- `wgsl/lower.go` — Texture function lowering (~250 LOC)
  - `textureSample(t, s, coord)` — Basic sampling
  - `textureSampleBias(t, s, coord, bias)` — With LOD bias
  - `textureSampleLevel(t, s, coord, level)` — Specific mip level
  - `textureSampleGrad(t, s, coord, ddx, ddy)` — With derivatives
  - `textureLoad(t, coord, level)` — Direct texel load
  - `textureStore(t, coord, value)` — Write to storage texture
  - `textureDimensions(t)` — Get texture size
  - `textureNumLevels(t)` — Get mip count
  - `textureNumLayers(t)` — Get array layer count
- `spirv/backend.go` — SPIR-V image operations (~200 LOC)
  - `OpSampledImage` — Combine texture and sampler
  - `OpImageSampleImplicitLod` — textureSample
  - `OpImageSampleExplicitLod` — textureSampleLevel
  - `OpImageFetch` — textureLoad
  - `OpImageWrite` — textureStore
  - `OpImageQuerySize*` — textureDimensions
  - `OpImageQueryLevels` — textureNumLevels
  - Helper methods: `getSampledImageType()`, `emitVec4F32Type()`

### Changed
- `wgsl/lower.go` — `lowerLocalVar()` supports optional type with inference
- `wgsl/lower.go` — `isBuiltinConstructor()` includes "array"
- `wgsl/lower.go` — `lowerBuiltinConstructor()` handles array shorthand
- `naga_test.go` — Enabled `TestCompileWithMathFunctions` (was skipped)
- Total: 124 tests across all packages

### Fixed
- Array size now correctly uses pointer (`*uint32`) per IR definition
- SPIR-V OpImageFetch uses coordinate without sampler

## [0.2.0] - 2025-12-11

Type inference and SPIR-V backend improvements (~2K new LOC).

### Added

#### Type Inference System
- `ir/resolve.go` — Complete type inference engine (~500 LOC)
  - Resolves types for all 25+ expression kinds
  - Handles literals, constants, composites, binary/unary ops
  - Supports nested types (vectors, matrices, arrays, structs)
  - `TypeResolution` struct for dual handle/inline representation
- `ir/resolve_test.go` — 8 comprehensive unit tests

#### Type Deduplication
- `ir/registry.go` — Type registry for SPIR-V compliance (~100 LOC)
  - Ensures each unique type appears exactly once
  - Normalized type keys for structural equality
  - Supports all IR type kinds
- `ir/registry_test.go` — 18 unit tests

#### SPIR-V Backend Improvements
- Proper type resolution instead of placeholders
- Correct int/float/uint opcode selection:
  - `IAdd/ISub/IMul` vs `FAdd/FSub/FMul`
  - `SDiv/UDiv/FDiv` for signed/unsigned/float
  - `IEqual/SLessThan` vs `FOrdEqual/FOrdLessThan`
- `emitInlineType()` for temporary types
- Range-based iteration to avoid large struct copies

#### Testing
- `spirv/shader_test.go` — 10 end-to-end shader compilation tests
- `wgsl/lower_type_inference_test.go` — 3 integration tests
- `wgsl/deduplication_test.go` — Type deduplication tests
- Total: 67+ tests across all packages

### Changed
- `ir/ir.go` — Added `TypeResolution` struct and `ExpressionTypes` to `Function`
- `wgsl/lower.go` — Integrated type registry and expression type tracking
- `spirv/backend.go` — Uses real types from inference system (~350 lines changed)
- `ir/validate.go` — Range-based iteration for performance

### Fixed
- SPIR-V binary output now has correct type IDs for all expressions
- Comparison operators correctly return `bool` or `vec<bool>`
- Math functions select correct int vs float GLSL.std.450 instructions

## [0.1.0] - 2025-12-10

First stable release. Complete WGSL to SPIR-V compilation pipeline (~10K LOC).

### Added

#### Intermediate Representation (IR)
- `ir/expression.go` — 33 expression types (~520 LOC)
  - Literals (f32, f64, i32, u32, bool)
  - Binary/Unary operators (17 binary, 3 unary)
  - Access expressions (array, struct, swizzle)
  - Math functions (60+ supported)
  - Texture operations (sample, load, query)
- `ir/statement.go` — 16 statement types (~320 LOC)
  - Control flow (if, loop, switch, break, continue)
  - Memory operations (store, atomic)
  - Function calls
- `ir/validate.go` — Comprehensive IR validation (~750 LOC)
  - Type validation
  - Expression validation
  - Statement validation
  - Entry point validation

#### AST to IR Lowering
- `wgsl/lower.go` — AST → IR converter (~1050 LOC)
  - Type resolution (scalars, vectors, matrices, arrays, structs)
  - Built-in type recognition
  - Binding resolution (@builtin, @location, @group/@binding)
  - Expression lowering
  - Statement lowering

#### SPIR-V Backend
- `spirv/writer.go` — Binary module builder (~670 LOC)
  - SPIR-V header generation
  - Instruction encoding
  - String encoding with padding
- `spirv/backend.go` — IR → SPIR-V translator (~1500 LOC)
  - Type emission (all IR types)
  - Constant emission (scalars, composites)
  - Function emission
  - Expression emission (33 expression types)
  - Control flow (if, loop, break, continue)
  - 40+ built-in math functions via GLSL.std.450
  - Derivative functions (dpdx, dpdy, fwidth)
- `spirv/spirv.go` — SPIR-V constants and opcodes
  - 100+ opcodes
  - 81 GLSL.std.450 extended instructions

#### Public API
- `naga.go` — Public API (~160 LOC)
  - `Compile(source)` — One-function compilation
  - `CompileWithOptions(source, opts)` — Custom options
  - `Parse()`, `Lower()`, `Validate()`, `GenerateSPIRV()` — Individual stages

#### CLI Tool
- `cmd/nagac/main.go` — Command-line compiler
  - `-o` output file
  - `-debug` include debug names
  - `-validate` enable validation
  - `-version` show version

#### Tests
- `naga_test.go` — 7 integration tests
- `ir/validate_test.go` — 12 validation tests
- `spirv/backend_test.go` — Backend tests
- `spirv/writer_test.go` — Writer tests
- `wgsl/lower_test.go` — Lowering tests

### Changed
- Updated `.golangci.yml` with exclusions for compiler complexity
- Expanded `spirv/spirv.go` with full opcode set

---

[Unreleased]: https://github.com/gogpu/naga/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/gogpu/naga/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/gogpu/naga/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/gogpu/naga/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/gogpu/naga/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/gogpu/naga/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/gogpu/naga/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/gogpu/naga/releases/tag/v0.2.0
[0.1.0]: https://github.com/gogpu/naga/releases/tag/v0.1.0
