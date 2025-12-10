# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned for v0.2.0
- Type inference for `let` bindings
- Array initialization syntax
- Texture sampling operations
- More complete validation

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

[Unreleased]: https://github.com/gogpu/naga/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/gogpu/naga/releases/tag/v0.1.0
