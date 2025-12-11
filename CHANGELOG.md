# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned for v0.4.0
- Better error messages with source locations
- Compute shader support
- Storage buffer operations
- Workgroup shared memory

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

[Unreleased]: https://github.com/gogpu/naga/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/gogpu/naga/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/gogpu/naga/releases/tag/v0.2.0
[0.1.0]: https://github.com/gogpu/naga/releases/tag/v0.1.0
