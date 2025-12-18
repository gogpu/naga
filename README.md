<h1 align="center">naga</h1>

<p align="center">
  <strong>Pure Go Shader Compiler</strong><br>
  WGSL to SPIR-V. Zero CGO.
</p>

<p align="center">
  <a href="https://github.com/gogpu/naga/actions/workflows/ci.yml"><img src="https://github.com/gogpu/naga/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/gogpu/naga"><img src="https://codecov.io/gh/gogpu/naga/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://pkg.go.dev/github.com/gogpu/naga"><img src="https://pkg.go.dev/badge/github.com/gogpu/naga.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/gogpu/naga"><img src="https://goreportcard.com/badge/github.com/gogpu/naga" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/gogpu/naga"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://github.com/gogpu/naga"><img src="https://img.shields.io/badge/CGO-none-success" alt="Zero CGO"></a>
</p>

<p align="center">
  <sub>Part of the <a href="https://github.com/gogpu">GoGPU</a> ecosystem</sub>
</p>

> **v0.4.0** — Compute shaders, atomics, barriers, unused var warnings. ~17K lines of pure Go.

---

## Features

- **Pure Go** — No CGO, no external dependencies
- **WGSL Frontend** — Full lexer and parser (140+ tokens)
- **IR** — Complete intermediate representation (expressions, statements, types)
- **Compute Shaders** — Storage buffers, workgroup memory, `@workgroup_size`
- **Atomic Operations** — atomicAdd, atomicSub, atomicMin, atomicMax, atomicCompareExchangeWeak
- **Barriers** — workgroupBarrier, storageBarrier, textureBarrier
- **Type Inference** — Automatic type resolution for all expressions, including `let` bindings
- **Type Deduplication** — SPIR-V compliant unique type emission
- **Array Initialization** — `array(1, 2, 3)` shorthand with inferred type and size
- **Texture Sampling** — textureSample, textureLoad, textureStore, textureDimensions
- **SPIR-V Backend** — Vulkan-compatible bytecode generation with correct type handling
- **Warnings** — Unused variable detection with `_` prefix exception
- **Validation** — Type checking and semantic validation
- **CLI Tool** — `nagac` command-line compiler

## Installation

```bash
go get github.com/gogpu/naga
```

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

## Architecture

```
naga/
├── wgsl/              # WGSL frontend
│   ├── token.go       # Token types (140+)
│   ├── lexer.go       # Tokenizer
│   ├── ast.go         # AST types
│   ├── parser.go      # Recursive descent parser (~1400 LOC)
│   └── lower.go       # AST → IR converter (~1100 LOC)
├── ir/                # Intermediate representation
│   ├── ir.go          # Core types (Module, Type, Function)
│   ├── expression.go  # 33 expression types (~520 LOC)
│   ├── statement.go   # 16 statement types (~320 LOC)
│   ├── validate.go    # IR validation (~750 LOC)
│   ├── resolve.go     # Type inference (~500 LOC) ← NEW
│   └── registry.go    # Type deduplication (~100 LOC) ← NEW
├── spirv/             # SPIR-V backend
│   ├── spirv.go       # SPIR-V constants and opcodes
│   ├── writer.go      # Binary module builder (~670 LOC)
│   └── backend.go     # IR → SPIR-V translator (~1800 LOC)
├── naga.go            # Public API
└── cmd/nagac/         # CLI tool
```

## Supported WGSL Features

### Types
- Scalars: `f32`, `f64`, `i32`, `u32`, `bool`
- Vectors: `vec2<T>`, `vec3<T>`, `vec4<T>`
- Matrices: `mat2x2<f32>` ... `mat4x4<f32>`
- Arrays: `array<T, N>`
- Structs: `struct { ... }`
- Atomics: `atomic<u32>`, `atomic<i32>`
- Textures: `texture_2d<f32>`, `texture_3d<f32>`, `texture_cube<f32>`
- Samplers: `sampler`, `sampler_comparison`

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
- Variable declarations: `var`, `let`
- Control flow: `if`, `else`, `for`, `while`, `loop`
- Loop control: `break`, `continue`
- Functions: `return`, `discard`
- Assignment: `=`, `+=`, `-=`, `*=`, `/=`

### Built-in Functions (50+)
- Math: `sin`, `cos`, `tan`, `exp`, `log`, `pow`, `sqrt`, `abs`, `min`, `max`, `clamp`
- Geometric: `dot`, `cross`, `length`, `distance`, `normalize`, `reflect`
- Interpolation: `mix`, `step`, `smoothstep`
- Derivatives: `dpdx`, `dpdy`, `fwidth`
- Atomic: `atomicAdd`, `atomicSub`, `atomicMin`, `atomicMax`, `atomicAnd`, `atomicOr`, `atomicXor`, `atomicExchange`, `atomicCompareExchangeWeak`
- Barriers: `workgroupBarrier`, `storageBarrier`, `textureBarrier`

## Roadmap

### v0.1.0 ✅
- [x] WGSL lexer and parser
- [x] Complete IR (expressions, statements, types)
- [x] IR validation
- [x] SPIR-V binary writer
- [x] SPIR-V backend (types, constants, functions, expressions)
- [x] Control flow (if, loop, break, continue)
- [x] Built-in math functions (GLSL.std.450)
- [x] Public API and CLI tool

### v0.2.0 ✅
- [x] Type inference for all expressions
- [x] Type deduplication (SPIR-V compliant)
- [x] Correct int/float/uint opcode selection
- [x] SPIR-V backend with proper type handling
- [x] 67+ unit tests

### v0.3.0 ✅
- [x] Type inference for `let` bindings
- [x] Array initialization syntax (`array(1, 2, 3)`)
- [x] Texture sampling operations (textureSample, textureLoad, textureStore)
- [x] SPIR-V image operations (OpImageSample*, OpImageFetch, OpImageQuery*)
- [x] 124 unit tests

### v0.4.0 (Current) ✅
- [x] Better error messages with source locations
- [x] Storage buffers (`var<storage, read>`, `var<storage, read_write>`)
- [x] Workgroup shared memory (`var<workgroup>`)
- [x] Atomic operations (atomicAdd, atomicSub, atomicCompareExchangeWeak, etc.)
- [x] Workgroup barriers (workgroupBarrier, storageBarrier, textureBarrier)
- [x] Unused variable warnings
- [x] 203 unit tests

### v0.5.0 (Next)
- [ ] GLSL backend output
- [ ] Source maps for debugging
- [ ] Optimization passes

### v1.0.0 (Goal)
- [ ] Full WGSL specification compliance
- [ ] Production-ready stability
- [ ] HLSL/MSL backends

## References

- [WGSL Specification](https://www.w3.org/TR/WGSL/)
- [SPIR-V Specification](https://registry.khronos.org/SPIR-V/)
- [naga (Rust)](https://github.com/gfx-rs/naga) — Original implementation

## Related Projects

| Project | Description | Status |
|---------|-------------|--------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | Pure Go graphics framework | v0.3.0 |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU types and HAL | **v0.5.0** |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics with GPU backend, scene graph, SIMD | **v0.9.0** |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | WebGPU FFI bindings | Stable |

## Contributing

We welcome contributions! Areas where help is needed:
- Additional WGSL features (ray tracing, mesh shaders)
- Test cases from real shaders
- GLSL backend implementation
- Documentation improvements

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>naga</b> — Shaders in Pure Go
</p>
