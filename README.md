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
| **Outputs** | SPIR-V (checked with spirv-val in CI), MSL, GLSL, HLSL; DXIL — 185 of 199 corpus shaders pass IDxcValidator (experimental) |
| **Compute** | Storage buffers, workgroups, atomics, barriers, subgroup operations |
| **Ray Tracing** | Ray query types, acceleration structures, 7 ray query builtins |
| **Compatibility** | Tested against Rust naga's WGSL snapshot corpus (172 shaders, gfx-rs/wgpu 14328f49): every output is compared, and each known difference is recorded with its reason — see [Rust Naga Compatibility](#rust-naga-compatibility) |
| **Build** | Zero CGO, single binary |

---

## Features

- **Pure Go** — No CGO, no external dependencies
- **WGSL Frontend** — Full lexer and parser (120+ tokens), 48 short type aliases (`vec3f`, `mat4x4f`, etc.), abstract constructors (`vec3(1,2,3)`)
- **Rust Naga Compatibility** — every IR, SPIR-V, MSL, GLSL and HLSL output is compared with Rust naga's on its 172-shader WGSL corpus; known differences are recorded per output with a reason. 29 own test shaders; 929 golden outputs
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
- **SPIR-V Backend** — Vulkan-compatible bytecode generation: integer div/mod safety wrappers, image bounds checking (Restrict/ReadZeroSkipWrite), ray query helpers, loop bounding (on by default), workgroup zero-init polyfill, NonUniform decorations, capability-aware instruction emission
- **MSL Backend** — Metal Shading Language output for macOS/iOS, vertex pulling transform, external textures, function-scope workgroup declarations (no host `setThreadgroupMemoryLength` needed)
- **GLSL Backend** — OpenGL Shading Language for OpenGL 3.3+, ES 3.0+, dead code elimination, image bounds checking, version-aware `layout(binding=N)` emission (`SupportsExplicitLocations` for GL 4.2+/ES 3.1+), `UniformInfo` reflection for runtime binding fallback on older GL drivers
- **HLSL Backend** — High-Level Shading Language for DirectX 11/12, shader models 5.0–6.7 (FXC and DXC); helpers get one exit where FXC would miscompile an early return
- **DXIL Backend** (experimental) — Direct DXIL generation from naga IR (**238 of 267 corpus entry points pass IDxcValidator**; **113 of 177 DXC roundtrip comparisons match exactly**; gg production shaders: every valid entry point passes; GPU readback corpus on D3D12 with Intel and NVIDIA). LLVM 3.7 bitcode with dx.op intrinsics, DXBC container. Vertex, fragment, compute, and mesh shaders (SM 6.0–6.9). CBV/SRV/UAV (read-only storage as SRV, read-write as UAV), atomics (i32/i64/f32 + image), barriers, ray query (35 intrinsics), wave/subgroup ops (13 intrinsics), texture sampling (8 variants), matrix scalarization, pack/unpack, helper functions. Optimization passes: DCE (liveness through control dependence, dead loops removed), SROA (struct decomposition, aggregate locals split per element), mem2reg (SSA promotion), single-store local promotion, loadInput DCE (per-member backwards reachability), workgroup struct decomposition, function inlining (early-return wrapping), strength reduction (mul→shl, urem→and, sub→add), constant folding. `Options.BindingMap` for WGSL→DXIL `(space, register)` remap (wgpu root signature compatibility). Eliminates FXC/DXC dependency. `dxil.Compile()` API. World's first Pure Go DXIL generator.
- **Type Conversions** — Scalar constructors `f32(x)`, `u32(y)`, `i32(z)` with correct SPIR-V opcodes
- **Bitcast** — `bitcast<T>(expr)` for reinterpreting bit patterns between types
- **Warnings** — Unused variable detection with `_` prefix exception
- **Validation** — Type checking, semantic validation, function call argument type/count verification, `@must_use` enforcement, `const_assert` evaluation, `@binding`/`@group` pairing, array size validation, swizzle namespace enforcement, mandatory semicolons
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

The zero value of every options struct is the safe one: SPIR-V 1.0, bounds checks on (`Restrict`), loops bounded,
workgroup memory zeroed, every binding required, no optional capability. Ask for more explicitly:

```go
opts := naga.CompileOptions{
    SPIRV:        spirv.Options{Version: spirv.Version1_3}, // subgroup operations need 1.3
    Capabilities: valid.CapShaderFloat16,                   // the device's features
}
spirvBytes, err := naga.CompileWithOptions(source, opts)
```

A backend never raises the target version a shader needs: it refuses with an error that names the feature and the
version (`errors.Is(err, backend.ErrUnsupported)`).

### CLI Tool

```bash
# Install
go install github.com/gogpu/naga/cmd/nagac@latest

# Compile shader
nagac -o shader.spv shader.wgsl

# With debug info
nagac -debug -o shader.spv shader.wgsl

# Show version
nagac -version
```

### Development Tools

```bash
# SPIR-V disassembler (debugging shader compilation)
go install github.com/gogpu/naga/cmd/spvdis@latest
spvdis shader.spv

# DXIL validator — Pure Go wrapper around Microsoft IDxcValidator (Windows)
# First Pure Go integration with dxil.dll, zero CGO. Runs a three-layer
# defensive pre-check (DXBC structural + LLVM bitcode metadata walker)
# before handing blobs to IDxcValidator::Validate.
go install github.com/gogpu/naga/cmd/dxilval@latest
dxilval shader.dxil                       # validate a single container
dxilval --wgsl shader.wgsl                # compile through naga, then validate
dxilval --corpus snapshot/testdata/rust/in/wgsl/  # walk a directory, typed-error summary

# Texture shader compile tool (testing)
go install github.com/gogpu/naga/cmd/texture_compile@latest
texture_compile shader.wgsl
```

### Parse, Validate, Write Any Backend

The same steps as Rust naga: parse WGSL to IR, validate it with the device's capabilities, then hand the module to
any backend. Every backend has the one contract `Compile(*ir.Module, Options) (output, TranslationInfo, error)`.

```go
module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
if err != nil {
    var pe *wgsl.ParseError
    if errors.As(err, &pe) {
        fmt.Println(pe.Format(source, "shader.wgsl")) // labelled source lines, as Rust naga prints them
    }
    return err
}
info, err := valid.Validate(module, valid.Options{Capabilities: valid.CapShaderFloat16})
if err != nil {
    return err // a valid.Errors: every problem, errors.As finds each *valid.Error
}
_ = info // per entry point: the globals it uses and how (bind group layouts, workgroup size limits)

spirvBytes, _, err := spirv.Compile(module, spirv.Options{})                            // Vulkan
mslCode, _, err := msl.Compile(module, msl.Options{})                                   // Metal
glslCode, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450})    // OpenGL; one entry point
hlslCode, _, err := hlsl.Compile(module, hlsl.Options{})                                // Direct3D through FXC/DXC
dxilBytes, _, err := dxil.Compile(module, dxil.Options{})                               // Direct3D 12, no compiler
```

GLSL and DXIL write one entry point: select it with `Options.EntryPoint = &backend.EntryPointSelector{Stage, Name}`
when the module has several.

### Pipeline Constants

`override` declarations get their values per pipeline. Parse and validate once, then process the module for each
pipeline before any backend, as wgpu does:

```go
pipelineModule, pipelineInfo, err := naga.ProcessOverrides(module, info, naga.OverrideOptions{
    Constants:  naga.PipelineConstants{"gain": 1.5, "13": 64}, // name, or @id
    EntryPoint: &backend.EntryPointSelector{Stage: ir.StageCompute, Name: "main"},
})
// errors.Is(err, naga.ErrOverrideMissingValue), *naga.PipelineConstantError, ...

code, _, err := msl.Compile(pipelineModule, msl.Options{})
```

`naga.Compile` applies the overrides' defaults. A backend given a module that still uses overrides returns an error
wrapping `ir.ErrUnresolvedOverride`.

---

## Architecture

```
naga/
├── naga.go, overrides.go, compact.go   # Compile, CompileWithOptions, ProcessOverrides, Compact
├── wgsl/        # WGSL frontend: Parse → *ir.Module (lexer, parser, lowerer in wgsl/internal)
├── ir/          # the IR data model (Rust naga's naga::ir): Module, types, expressions, statements, Layouter
├── valid/       # validation (Rust naga's valid): Validate, Options, ModuleInfo, Capabilities
├── backend/     # what every backend shares: bounds-check policies, entry point selector, shader model, errors
├── spirv/       # SPIR-V backend (Vulkan)
├── msl/         # Metal Shading Language backend
├── glsl/        # GLSL backend (OpenGL 3.3+, OpenGL ES 3.0+)
├── hlsl/        # HLSL backend (Direct3D through FXC or DXC)
├── dxil/        # DXIL backend (Direct3D 12 without a shader compiler)
│   └── internal/  # bitcode writer, DXIL module, DXBC container, IR → DXIL emitter, optimization passes
├── internal/    # shared implementation: typifier, constant evaluator, override processing, IR passes
├── testdata/    # GPU readback corpus (read by gogpu/wgpu) and vendored third-party shaders
├── snapshot/    # golden tests against Rust naga's outputs, DXC roundtrip, spirv-val and IDxcValidator
└── cmd/
    ├── nagac/            # CLI compiler
    ├── spvdis/           # SPIR-V disassembler
    ├── dxilval/          # DXIL validation through dxil.dll (Windows)
    └── texture_compile/  # texture shader testing
```

Each package's implementation lives in its `internal/` directory; the package itself is the public API that
pkg.go.dev shows.

## Supported WGSL Features

### Types
- Scalars: `f16`, `f32`, `f64`, `i16`, `u16` (`enable wgpu_int16;`), `i32`, `u32`, `i64`, `u64`, `bool`
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
- Subgroup: `subgroupBallot`, `subgroupAll`, `subgroupAny`, `subgroupAdd/Mul/Min/Max/And/Or/Xor`, `subgroupBroadcast/First`, `subgroupShuffle/XOR/Up/Down`, `quadSwap/Broadcast` (need `enable subgroups;`, as the WGSL spec requires)
- Directives: `enable` for `f16`, `clip_distances`, `dual_source_blending`, `subgroups`, `primitive_index`, `draw_index` and wgpu's extensions; a feature used without its `enable` is an error
- Ray Query: `rayQueryInitialize`, `rayQueryProceed`, `rayQueryGetCommittedIntersection`, `rayQueryGetCandidateIntersection`, `rayQueryTerminate`
- Uniform Load: `workgroupUniformLoad`
- Array: `arrayLength`

---

## Status

**Current Version:** See [CHANGELOG.md](CHANGELOG.md) for release history.

| Backend | Status | Target Platform |
|---------|--------|-----------------|
| SPIR-V | ✅ Stable, spirv-val in CI | Vulkan |
| MSL | ✅ Stable | Metal (macOS/iOS) |
| GLSL | ✅ Stable, version-aware binding | OpenGL 3.3+, ES 3.0+ |
| HLSL | ✅ Stable | DirectX 11/12 |
| DXIL | **238/267 entry points pass IDxcValidator** (Rust naga and own corpus), 113/177 DXC golden | DirectX 12 (SM 6.0-6.9, experimental) |

See [ROADMAP.md](ROADMAP.md) for detailed development plans.

### Test Coverage

Coverage per package and per commit is on [Codecov](https://codecov.io/gh/gogpu/naga). Tests verify output
(exact strings, spirv-val, IDxcValidator, DXC roundtrip), negative cases with Rust naga's error texts, and GPU
readback values; every fix ships with a test that failed before it.

### Architecture

All backends follow the DXIL internal package pattern — implementation in `internal/codegen/`, thin public API with real types (not aliases). Zero panics in error paths.

---

## References

- [WGSL Specification](https://www.w3.org/TR/WGSL/)
- [SPIR-V Specification](https://registry.khronos.org/SPIR-V/)
- [naga (Rust)](https://github.com/gfx-rs/wgpu/tree/trunk/naga) — the Rust implementation this project was ported from (see [UPSTREAM](UPSTREAM))

### Rust Naga Compatibility

naga is tested against the WGSL snapshot corpus of [Rust naga](https://github.com/gfx-rs/wgpu/tree/trunk/naga):
172 shaders with their test parameters and reference outputs, vendored from gfx-rs/wgpu at commit 14328f49
(`snapshot/testdata/rust/`, with a manifest of every file). Every output Rust's snapshot driver writes — IR,
SPIR-V, MSL, GLSL, HLSL — is compared with ours. An output that does not match is a test failure unless it is
recorded in `snapshot/rust_deviations_test.go` with its reason and a fingerprint of the difference; most records
name the upstream commits not ported yet, and some are bugs being fixed. SPIR-V is still compared by
instruction counts rather than exactly. Shaders using features we do not support yet (cooperative matrices,
`debugPrintf`, ray tracing pipelines) are listed with their expected error. In addition, 29 own test shaders and
the golden outputs of every backend (`snapshot/testdata/golden/`) guard against unintended changes.

---

## Ecosystem

**naga** is the shader compiler for the [GoGPU](https://github.com/gogpu) ecosystem.

| Project | Description |
|---------|-------------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | GPU framework with windowing and input |
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU implementation |
| **gogpu/naga** | **Shader compiler (this repo)** |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics library |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit (22 widgets, M3/Fluent/Cupertino) |

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

## Star History

<a href="https://starhistory.io">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.starhistory.io/png?repos=gogpu/naga&style=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.starhistory.io/png?repos=gogpu/naga&style=professional" />
   <img alt="Star History Chart" src="https://api.starhistory.io/png?repos=gogpu/naga" width="800" />
 </picture>
</a>

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>naga</b> — Shaders in Pure Go
</p>
