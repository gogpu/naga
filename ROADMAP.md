# naga Roadmap

> **Pure Go Shader Compiler**
>
> WGSL to SPIR-V, MSL, GLSL, and HLSL. Zero CGO.

---

## Vision

**naga** is a shader compiler written entirely in Go. It compiles WGSL (WebGPU Shading Language) to multiple backend formats without requiring CGO or external dependencies.

### Core Principles

1. **Pure Go** — No CGO, easy cross-compilation, single binary deployment
2. **Multi-Backend** — SPIR-V (Vulkan), MSL (Metal), GLSL (OpenGL), HLSL (DirectX)
3. **Spec Compliant** — Follow W3C WGSL and Khronos SPIR-V specifications
4. **Production-Ready** — Tested on real hardware (Intel, NVIDIA, AMD, Apple)

---

## Current State: v0.16.1 (2026-04-04)

✅ **Production-ready** shader compiler (~90K LOC) with **complete Rust naga parity**
and **100% SPIR-V binary validation**:

### What We Have

- **Full WGSL frontend** — Lexer (120+ tokens), parser, AST → IR lowerer
- **4 backend outputs** — SPIR-V, MSL, GLSL, HLSL — all at 100% Rust naga golden parity
- **164/164 SPIR-V binary validation** — every shader compiles and passes spirv-val (100%)
- **Five-layer exact match** — IR 144/144, SPIR-V 87/87, MSL 91/91, GLSL 68/68, HLSL 58/58
- **100+ WGSL built-in functions** — math, geometric, bit manipulation, packing, derivatives
- **Compute shaders** — atomics (int32/int64/float32), barriers, workgroups, runtime-sized arrays
- **Ray tracing** — ray query types, acceleration structures, 7 ray query builtins
- **Subgroup operations** — ballot, shuffle, broadcast, quad operations
- **Mesh shaders** — MeshEXT/TaskEXT execution models with SPV_EXT_mesh_shader
- **Pipeline overrides** — OpSpecConstant with ProcessOverrides pipeline
- **Image atomics** — OpImageTexelPointer + atomic ops on storage textures
- **Texture sampling** — sample, load, store, gather, dimensions (50+ formats)
- **f16/i64/u64/f64** scalar types with literal suffixes
- **Pack/Unpack** — 4x8 and 2x16 packing with signed/unsigned/clamped variants
- **SPIR-V integer safety** — naga_div/naga_mod wrappers prevent UB
- **Image bounds checking** — Restrict and ReadZeroSkipWrite policies
- **Pointer function arguments** — copy-in/copy-out spill pattern
- **994+ golden output files** across 4 backends
- **Development tools** — nagac CLI, spvdis disassembler

---

## Next Up

### v0.16.2 — Quality & Coverage

| Task | Priority | Effort | Description |
|------|----------|--------|-------------|
| **Test coverage 80%** | P2 | 8 | Per-package coverage: wgsl 40%→80%, msl 40%→80%, hlsl 50%→80% |
| **SPIR-V structural parity** | P3 | 3 | 90/93 → 93/93 Rust match (Int8 native types, extra Block decoration) |

### v1.0.0 — Stable Release

| Goal | Status | Notes |
|------|--------|-------|
| Complete Rust naga parity | ✅ Done | All 5 layers at 100% |
| SPIR-V binary validation | ✅ Done | 164/164 pass spirv-val |
| Compiler optimizations | ✅ Done | −32% allocs, −34% bytes |
| Ray tracing | ✅ Done | Ray query types, acceleration structures |
| Subgroup operations | ✅ Done | Ballot, shuffle, broadcast, quad |
| Mesh shaders | ✅ Done | MeshEXT/TaskEXT |
| Full WGSL spec compliance | In progress | Remaining edge cases |
| API stability guarantee | Planned | Semantic versioning contract |
| Test coverage 80%+ | Planned | awesome-go requirement |
| Comprehensive documentation | In progress | ARCHITECTURE.md, pkg.go.dev |

---

## Future Directions

### Frontends (New Input Formats)

| Frontend | Priority | Effort | Description |
|----------|----------|--------|-------------|
| **SPIR-V input** | Low | XL | SPIR-V → IR decompiler. Enables roundtrip testing and SPIR-V → MSL/GLSL/HLSL cross-compilation |
| **GLSL input** | Low | XL | GLSL → IR parser. Enables legacy OpenGL shader migration to modern backends |
| **WGSL output** | Low | L | IR → WGSL printer. Enables roundtrip testing (WGSL → IR → WGSL) and shader formatting/normalization |

### Optimization Passes

| Pass | Priority | Description |
|------|----------|-------------|
| Constant folding | Medium | Evaluate constant expressions at compile time |
| Dead code elimination | Medium | Remove unreachable functions and statements (GLSL already has entry-point reachability) |
| Inlining | Low | Inline small functions to reduce call overhead |
| Dead store elimination | Low | Remove stores that are never read |

### Tooling & DX

| Feature | Priority | Description |
|---------|----------|-------------|
| Source maps | Medium | Debug info mapping SPIR-V instructions back to WGSL source locations |
| Shader minification | Low | Remove debug names, compact identifiers for production builds |
| LSP integration | Low | Language server protocol for WGSL editor support |

### Known Limitations

| Limitation | Notes |
|------------|-------|
| Runtime residual prologue (SPV-008) | Workaround exists in gg (select/flat loops). Nested for-loops and if/else may produce incorrect runtime results despite valid SPIR-V |
| SPIR-V structural gaps (3 shaders) | Pack/unpack uses polyfill instead of Int8 native types; 1 extra Block decoration. Valid SPIR-V, just different from Rust output |

---

## Architecture

```
                      WGSL Source
                           │
                           ▼
                   ┌───────────────┐
                   │  wgsl/lexer   │
                   │  wgsl/parser  │
                   └───────┬───────┘
                           │ AST
                           ▼
                   ┌───────────────┐
                   │  wgsl/lower   │
                   └───────┬───────┘
                           │ IR
                           ▼
                   ┌───────────────┐
                   │  ir/validate  │
                   │  ir/resolve   │
                   └───────┬───────┘
                           │
        ┌──────────┬───────┴───────┬──────────┐
        ▼          ▼               ▼          ▼
   ┌─────────┐ ┌─────────┐   ┌─────────┐ ┌─────────┐
   │  spirv/ │ │   msl/  │   │  glsl/  │ │  hlsl/  │
   │ backend │ │ backend │   │ backend │ │ backend │
   └────┬────┘ └────┬────┘   └────┬────┘ └────┬────┘
        │          │             │          │
        ▼          ▼             ▼          ▼
     SPIR-V      MSL           GLSL       HLSL
    (Vulkan)   (Metal)       (OpenGL)  (DirectX)
```

---

## Released Versions

| Version | Date | Highlights |
|---------|------|------------|
| **v0.16.1** | 2026-04 | **164/164 spirv-val pass (100%).** 45 SPIR-V fixes: mesh shaders, overrides, image atomics, pack/unpack, pointer spill, integer ops, matrix decomposition, depth sampling. |
| **v0.16.0** | 2026-04 | GLSL TextureMappings + 34 SPIR-V validation fixes (119/164 pass) |
| **v0.15.0** | 2026-03 | ALL 5 backends 100% Rust parity: IR 144/144, SPIR-V 87/87, MSL 91/91, GLSL 68/68, HLSL 58/58. ~90K LOC |
| v0.14.8 | 2026-03 | GLSL bind group collision fix |
| v0.14.0 | 2026-02 | Major WGSL coverage: 15/15 Essential reference shaders |
| **v0.13.1** | 2026-02 | SPIR-V OpArrayLength fix, 68 benchmarks, −32% allocs |
| v0.13.0 | 2026-02 | GLSL backend, HLSL/SPIR-V fixes, all 93 WGSL builtins |
| v0.12.0 | 2026-02 | Function calls, compute shader codegen |
| v0.7.0 | 2025-12 | HLSL backend (~8.8K LOC) |
| v0.5.0 | 2025-12 | MSL backend (~3.6K LOC) |
| v0.1.0 | 2025-12 | Initial release (~10K LOC) |

→ **See [CHANGELOG.md](CHANGELOG.md) for detailed release notes**

---

## Contributing

We welcome contributions! Priority areas:

1. **Test Cases** — Real-world shaders for testing
2. **Test Coverage** — Help reach 80%+ per-package coverage
3. **Optimization** — Backend optimization passes
4. **Documentation** — Improve docs and examples

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Non-Goals

- **Runtime compilation** — naga is compile-time only (ahead-of-time shader compilation)
- **Shader reflection** — Use SPIR-V reflection tools (spirv-cross, spirv-reflect)
- **GLSL/HLSL as primary input** — WGSL is the primary input language; other frontends are future/optional

---

## License

MIT License — see [LICENSE](LICENSE) for details.
