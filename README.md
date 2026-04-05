<h1 align="center">naga</h1>

<p align="center">
  <strong>Pure Go Shader Compiler</strong><br>
  WGSL to SPIR-V, MSL, GLSL, HLSL, and DXIL. Zero CGO.
</p>

<p align="center">
  <a href="https://github.com/gogpu/naga/actions/workflows/ci.yml"><img src="https://github.com/gogpu/naga/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/gogpu/naga"><img src="https://codecov.io/gh/gogpu/naga/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://pkg.go.dev/github.com/gogpu/naga"><img src="https://pkg.go.dev/badge/github.com/gogpu/naga.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/gogpu/naga"><img src="https://goreportcard.com/badge/github.com/gogpu/naga" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/gogpu/naga/releases"><img src="https://img.shields.io/github/v/release/gogpu/naga" alt="Latest Release"></a>
  <a href="https://github.com/gogpu/naga"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://github.com/gogpu/naga"><img src="https://img.shields.io/badge/CGO-none-success" alt="Zero CGO"></a>
  <a href="https://github.com/gogpu/gogpu/stargazers"><img src="https://img.shields.io/github/stars/gogpu/gogpu?style=flat&labelColor=555&color=yellow" alt="Stars"></a>
  <a href="https://github.com/gogpu/gogpu/discussions"><img src="https://img.shields.io/github/discussions/gogpu/gogpu?style=flat&labelColor=555&color=blue" alt="Discussions"></a>
</p>

<p align="center">
  <sub>Part of the <a href="https://github.com/gogpu">GoGPU</a> ecosystem</sub>
</p>



---

## Overview

**naga** is a shader compiler written entirely in Go. It compiles WGSL (WebGPU Shading Language) to multiple backend formats without requiring CGO or external dependencies.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Input** | Full WGSL parser (120+ tokens), 48 short type aliases (`vec3f`, `mat4x4f`...), abstract constructors |
| **Outputs** | SPIR-V, MSL, GLSL, HLSL, DXIL (experimental) |
| **Compute** | Storage buffers, workgroups, atomics, barriers, subgroup operations |
| **Ray Tracing** | Ray query types, acceleration structures, 7 ray query builtins |
| **Compatibility** | **144/144 (100%)** reference shaders compile. Five-layer exact match: **IR 144/144**, **SPIR-V 87/87**, **MSL 91/91**, **GLSL 68/68**, **HLSL 58/58** — complete Rust naga parity on all backends |
| **Build** | Zero CGO, single binary |

---

## Features

- **Pure Go** — No CGO, no external dependencies
- **WGSL Frontend** — Full lexer and parser (120+ tokens), 48 short type aliases (`vec3f`, `mat4x4f`, etc.), abstract constructors (`vec3(1,2,3)`)
- **Rust Naga Compatibility** — **144/144 (100%)** reference shaders compile. Five-layer exact match: **IR 144/144**, **SPIR-V 87/87**, **MSL 91/91**, **GLSL 68/68**, **HLSL 58/58** — complete Rust naga parity on all backends. 164 snapshot tests with 994 golden outputs
- **IR** — Complete intermediate representation (expressions, statements, types)
- **Compute Shaders** — Storage buffers, workgroup memory, `@workgroup_size`
- **Atomic Operations** — atomicAdd, atomicSub, atomicMin, atomicMax, atomicCompareExchangeWeak
- **Barriers** — workgroupBarrier, storageBarrier, textureBarrier
- **Type Inference** — Automatic type resolution for all expressions, including `let` bindings
- **Type Deduplication** — SPIR-V compliant unique type emission
- **Array Initialization** — `array(1, 2, 3)` shorthand with inferred type and size
- **Texture Sampling** — textureSample, textureLoad, textureStore, textureDimensions, textureGather, textureSampleCompare
- **Swizzle Operations** — Full vector swizzle support (`.xyz`, `.rgba`, `.xxyy`, etc.)
- **Function Calls** — `OpFunctionCall` support for modular WGSL shaders with helper functions
- **SPIR-V Backend** — Vulkan-compatible bytecode generation (**87/87 exact Rust naga parity**): integer div/mod safety wrappers, image bounds checking (Restrict/ReadZeroSkipWrite), ray query helpers, force loop bounding, workgroup zero-init polyfill, NonUniform decorations, capability-aware instruction emission
- **MSL Backend** — Metal Shading Language output for macOS/iOS (**91/91 exact Rust naga parity**), vertex pulling transform, external textures, override pipeline constants
- **GLSL Backend** — OpenGL Shading Language for OpenGL 3.3+, ES 3.0+ (**68/68 exact Rust naga parity**), dead code elimination, ProcessOverrides, image bounds checking
- **HLSL Backend** — High-Level Shading Language for DirectX 11/12 (**58/58 exact Rust naga parity**)
- **DXIL Backend** (experimental) — Direct DXIL generation from naga IR. LLVM 3.7 bitcode with dx.op intrinsics, DXBC container with BYPASS hash. Vertex + fragment shaders, SM 6.0. Eliminates FXC/DXC dependency. `dxil.Compile()` API. ~12K LOC, 190 tests.
- **Type Conversions** — Scalar constructors `f32(x)`, `u32(y)`, `i32(z)` with correct SPIR-V opcodes
- **Bitcast** — `bitcast<T>(expr)` for reinterpreting bit patterns between types
- **Warnings** — Unused variable detection with `_` prefix exception
- **Validation** — Type checking and semantic validation
- **CLI Tool** — `nagac` command-line compiler

---

## Installation

```bash
go get github.com/gogpu/naga
```

**Requirements:** Go 1.25+

---

## Usage

### As Library

```go
package main

import (
    "fmt"
    "log"

    "github.com/gogpu/naga"
)

func main() {
    source := `
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
    // Simple compilation
    spirv, err := naga.Compile(source)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Generated %d bytes of SPIR-V\n", len(spirv))
}
```

### With Options

```go
opts := naga.CompileOptions{
    SPIRVVersion: spirv.Version1_3,
    Debug:        true,   // Include debug names
    Validate:     true,   // Enable IR validation
}
spirv, err := naga.CompileWithOptions(source, opts)
```

### CLI Tool

```bash
# Install
go install github.com/gogpu/naga/cmd/nagac@latest

# Compile shader
nagac shader.wgsl -o shader.spv

# With debug info
nagac -debug shader.wgsl -o shader.spv

# Show version
nagac -version
```

### Development Tools

```bash
# SPIR-V disassembler (debugging shader compilation)
go install github.com/gogpu/naga/cmd/spvdis@latest
spvdis shader.spv

# Texture shader compile tool (testing)
go install github.com/gogpu/naga/cmd/texture_compile@latest
texture_compile shader.wgsl
```

### Multiple Backends

```go
// Parse and lower WGSL to IR (shared across all backends)
ast, _ := naga.Parse(source)
module, _ := naga.Lower(ast)

// Generate SPIR-V (Vulkan)
spirvBytes, _ := naga.GenerateSPIRV(module, spirv.Options{})

// Generate MSL (Metal)
mslCode, _, _ := msl.Compile(module, msl.DefaultOptions())

// Generate GLSL (OpenGL)
glslCode, _, _ := glsl.Compile(module, glsl.DefaultOptions())

// Generate HLSL (DirectX)
hlslCode, _, _ := hlsl.Compile(module, hlsl.DefaultOptions())
```

### Individual Stages

```go
// Parse WGSL to AST
ast, err := naga.Parse(source)

// Lower AST to IR
module, err := naga.Lower(ast)

// Validate IR
errors, err := naga.Validate(module)

// Generate SPIR-V
spirvOpts := spirv.Options{Version: spirv.Version1_3, Debug: true}
spirvBytes, err := naga.GenerateSPIRV(module, spirvOpts)
```

---

## Architecture

```
naga/                              ~90K LOC total
├── wgsl/              # WGSL frontend (~19.5K LOC)
│   ├── token.go       # Token types (120+)
│   ├── lexer.go       # Tokenizer
│   ├── ast.go         # AST types
│   ├── parser.go      # Recursive descent parser
│   └── lower.go       # AST → IR converter
├── ir/                # Intermediate representation (~6.5K LOC)
│   ├── ir.go          # Core types (Module, Type, Function)
│   ├── expression.go  # 30+ expression kinds
│   ├── statement.go   # 20+ statement kinds
│   ├── validate.go    # IR validation
│   ├── resolve.go     # Type inference
│   └── registry.go    # Type deduplication
├── spirv/             # SPIR-V backend (~10.8K LOC)
│   ├── spirv.go       # SPIR-V constants and opcodes
│   ├── block.go       # Block ownership model (Rust naga pattern)
│   ├── writer.go      # Binary module builder
│   ├── backend.go     # IR → SPIR-V translator
│   └── ray_query.go   # Ray query helper functions
├── msl/               # MSL backend (~14.2K LOC)
│   ├── backend.go     # Public API, Options, Compile()
│   ├── writer.go      # MSL code writer
│   ├── types.go       # Type generation
│   ├── expressions.go # Expression codegen
│   ├── statements.go  # Statement codegen
│   ├── functions.go   # Entry points and functions
│   └── keywords.go    # MSL/C++ reserved words
├── glsl/              # GLSL backend (~7.8K LOC)
│   ├── backend.go     # Public API, version targeting
│   ├── writer.go      # GLSL code writer
│   ├── types.go       # Type generation
│   ├── expressions.go # Expression codegen
│   ├── statements.go  # Statement codegen
│   └── keywords.go    # Reserved word escaping
├── hlsl/              # HLSL backend (~13.6K LOC)
│   ├── backend.go     # Public API, Options, Compile()
│   ├── writer.go      # HLSL code writer
│   ├── types.go       # Type generation
│   ├── expressions.go # Expression codegen
│   ├── statements.go  # Statement codegen
│   ├── storage.go     # Buffer/atomic operations
│   ├── functions.go   # Entry points with semantics
│   └── keywords.go    # HLSL reserved words
├── naga.go            # Public API
└── cmd/
    ├── nagac/         # CLI compiler
    ├── spvdis/        # SPIR-V disassembler
    └── texture_compile/ # Texture shader testing
```

## Supported WGSL Features

### Types
- Scalars: `f16`, `f32`, `f64`, `i32`, `u32`, `i64`, `u64`, `bool`
- Vectors: `vec2<T>`, `vec3<T>`, `vec4<T>` (and short aliases: `vec2f`, `vec3i`, `vec4u`, etc.)
- Matrices: `mat2x2<f32>` ... `mat4x4<f32>` (and short aliases: `mat2x2f`, `mat4x4f`, etc.)
- Arrays: `array<T, N>`, `array<T>` (runtime-sized, storage buffers)
- Structs: `struct { ... }` (with constructor syntax: `StructName(field1, field2)`)
- Atomics: `atomic<u32>`, `atomic<i32>`
- Textures: `texture_2d<f32>`, `texture_3d<f32>`, `texture_cube<f32>`, `texture_depth_2d_array`
- Samplers: `sampler`, `sampler_comparison`
- Binding arrays: `binding_array<T, N>`
- Ray tracing: `acceleration_structure`, `ray_query`
- Abstract constructors: `vec3(1,2,3)`, `mat2x2(...)`, `array(...)` (without explicit template parameters)
- Type aliases: `alias FVec3 = vec3<f32>;`

### Shader Stages
- `@vertex` — Vertex shaders with `@builtin(position)` output
- `@fragment` — Fragment shaders with `@location(N)` outputs
- `@compute` — Compute shaders with `@workgroup_size(X, Y, Z)`

### Bindings
- `@builtin(position)`, `@builtin(vertex_index)`, `@builtin(instance_index)`
- `@builtin(global_invocation_id)` — Compute shader invocation ID
- `@location(N)` — Vertex attributes and fragment outputs
- `@group(G) @binding(B)` — Resource bindings

### Address Spaces
- `var<uniform>` — Uniform buffer
- `var<storage, read>` — Read-only storage buffer
- `var<storage, read_write>` — Read-write storage buffer
- `var<workgroup>` — Workgroup shared memory

### Statements
- Variable declarations: `var`, `let`, `const`
- Override declarations: `@id(N) override name: type = default;`
- Compile-time assertions: `const_assert expr;`
- Control flow: `if`, `else`, `for`, `while`, `loop`, `switch`, `case`, `default`
- Loop control: `break`, `continue`, `break if` (continuing blocks)
- Functions: `return`, `discard`
- Assignment: `=`, `+=`, `-=`, `*=`, `/=`

### Built-in Functions (100+)
- Math: `abs`, `min`, `max`, `clamp`, `saturate`, `sign`, `fma`, `modf`, `frexp`, `ldexp`, `quantizeToF16`
- Trigonometric: `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2`, `sinh`, `cosh`, `tanh`, `asinh`, `acosh`, `atanh`
- Angle: `radians`, `degrees`
- Exponential: `exp`, `exp2`, `log`, `log2`, `pow`, `sqrt`, `inverseSqrt`
- Decomposition: `ceil`, `floor`, `round`, `fract`, `trunc`
- Geometric: `dot`, `cross`, `length`, `distance`, `normalize`, `faceForward`, `reflect`, `refract`, `outerProduct`
- Interpolation: `mix`, `step`, `smoothstep`
- Matrix: `transpose`, `determinant`, `inverse`
- Relational: `all`, `any`, `isnan`, `isinf`
- Bit: `countTrailingZeros`, `countLeadingZeros`, `countOneBits`, `reverseBits`, `extractBits`, `insertBits`, `firstTrailingBit`, `firstLeadingBit`
- Packing: `pack4x8snorm`, `pack4x8unorm`, `pack2x16snorm`, `pack2x16unorm`, `pack2x16float`, `pack4xI8`, `pack4xU8`, `pack4xI8Clamp`, `pack4xU8Clamp`, `unpack4x8snorm`, `unpack4x8unorm`, `unpack2x16snorm`, `unpack2x16unorm`, `unpack2x16float`, `unpack4xI8`, `unpack4xU8`
- Selection: `select`
- Derivatives: `dpdx`, `dpdy`, `fwidth`, `dpdxCoarse`, `dpdyCoarse`, `fwidthCoarse`, `dpdxFine`, `dpdyFine`, `fwidthFine`
- Atomic: `atomicAdd`, `atomicSub`, `atomicMin`, `atomicMax`, `atomicAnd`, `atomicOr`, `atomicXor`, `atomicExchange`, `atomicCompareExchangeWeak`
- Barriers: `workgroupBarrier`, `storageBarrier`, `textureBarrier`, `subgroupBarrier`
- Subgroup: `subgroupBallot`, `subgroupAll`, `subgroupAny`, `subgroupAdd/Mul/Min/Max/And/Or/Xor`, `subgroupBroadcast/First`, `subgroupShuffle/XOR/Up/Down`, `quadSwap/Broadcast`
- Ray Query: `rayQueryInitialize`, `rayQueryProceed`, `rayQueryGetCommittedIntersection`, `rayQueryGetCandidateIntersection`, `rayQueryTerminate`
- Uniform Load: `workgroupUniformLoad`
- Array: `arrayLength`

---

## Status

**Current Version:** See [CHANGELOG.md](CHANGELOG.md) for release history.

| Backend | Status | Target Platform |
|---------|--------|-----------------|
| SPIR-V | ✅ **87/87 Rust parity** | Vulkan |
| MSL | ✅ **91/91 Rust parity** | Metal (macOS/iOS) |
| GLSL | ✅ **68/68 Rust parity** | OpenGL 3.3+, ES 3.0+ |
| HLSL | ✅ **58/58 Rust parity** | DirectX 11/12 |

See [ROADMAP.md](ROADMAP.md) for detailed development plans.

---

## References

- [WGSL Specification](https://www.w3.org/TR/WGSL/)
- [SPIR-V Specification](https://registry.khronos.org/SPIR-V/)
- [naga (Rust)](https://github.com/gfx-rs/naga) — Original implementation

### Rust Naga Compatibility

naga is tested against **all 144 reference WGSL shaders** from the [Rust naga](https://github.com/gfx-rs/naga) test suite — **100% compatibility** across all five layers: **IR 144/144**, **SPIR-V 87/87**, **MSL 91/91**, **GLSL 68/68**, **HLSL 58/58** exact output match. Total: 164 test shaders with 994 golden outputs.

---

## Ecosystem

**naga** is the shader compiler for the [GoGPU](https://github.com/gogpu) ecosystem.

| Project | Description |
|---------|-------------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | GPU framework with windowing and input |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation |
| **gogpu/naga** | **Shader compiler (this repo)** |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics library |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit (planned) |

---

## Documentation

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** — Compiler architecture, pipeline, IR design
- **[ROADMAP.md](ROADMAP.md)** — Development milestones
- **[CHANGELOG.md](CHANGELOG.md)** — Release notes
- **[pkg.go.dev](https://pkg.go.dev/github.com/gogpu/naga)** — API reference

---

## Contributing

We welcome contributions! Areas where help is needed:
- Additional WGSL features
- Test cases from real shaders
- Backend optimizations
- Documentation improvements

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>naga</b> — Shaders in Pure Go
</p>
