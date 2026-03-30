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

## Current State: v0.15.0

✅ **Production-ready** shader compiler (~90K LOC) with **complete Rust naga parity**:
- Full WGSL frontend (lexer, parser, IR) — 164 test shaders (144 Rust reference + 20 custom)
- 4 backend outputs (SPIR-V, MSL, GLSL, HLSL) — all at 100% Rust naga parity
- Five-layer exact match: IR 144/144, SPIR-V 87/87, MSL 91/91, GLSL 68/68, HLSL 58/58
- 100+ WGSL built-in functions (math, geometric, bit manipulation, packing, derivatives, relational)
- Compute shaders (atomics, barriers, workgroups, runtime-sized storage buffers)
- Ray tracing (ray query types, acceleration structures, 7 ray query builtins)
- Subgroup operations (ballot, shuffle, broadcast, quad operations)
- f16/i64/u64/f64 scalar types with literal suffixes
- Texture sampling and storage textures (50+ formats)
- Override pipeline constants with ProcessOverrides
- SPIR-V backend with integer safety wrappers, image bounds checking, ray query helpers
- MSL backend with vertex pulling, external textures, pipeline constants
- GLSL backend with dead code elimination, ProcessOverrides, image bounds checking
- HLSL backend with row_major matrices, storage operations, semantic mapping
- 994 golden output files across 4 backends
- Development tools (nagac with SPIR-V 1.3, spvdis)

---

## Upcoming

### v1.0.0 — Production Release
- [x] Complete Rust naga parity (all 5 layers at 100%)
- [x] Compiler allocation optimization (−32% allocs, −34% bytes)
- [x] Ray tracing support (ray query types, acceleration structures)
- [x] Subgroup operations
- [x] Dead code elimination (GLSL entry-point reachability)
- [ ] Full WGSL specification compliance (remaining edge cases)
- [ ] API stability guarantee
- [ ] Optimization passes (constant folding, inlining)
- [ ] Source maps for debugging
- [ ] Comprehensive documentation

---

## Future Ideas

| Theme | Description |
|-------|-------------|
| **Optimization** | Constant folding, inlining, dead store elimination |
| **Source Maps** | Debug info mapping SPIR-V back to WGSL |
| **WGSL Extensions** | Pointer parameters, additional built-in functions |
| **Validation** | Full WGSL spec compliance checking |

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
| **v0.15.0** | 2026-03 | ALL 5 backends 100% Rust parity: IR 144/144, SPIR-V 87/87, MSL 91/91, GLSL 68/68, HLSL 58/58. ~90K LOC. |
| v0.14.8 | 2026-03 | GLSL bind group collision fix |
| v0.14.7 | 2026-03 | MSL multi-group binding index collision fix |
| v0.14.6 | 2026-03 | MSL pass-through globals for helper functions |
| v0.14.5 | 2026-03 | MSL buffer references instead of pointers |
| v0.14.4 | 2026-03 | MSL vertex stage_in, discard_fragment namespace |
| v0.14.3 | 2026-02 | SPIR-V deferred stores, prologue var init splitting |
| v0.14.2 | 2026-02 | Golden snapshot test system, 20 reference shaders |
| v0.14.1 | 2026-02 | HLSL row_major, mul() reversal, GLSL namedExpressions fix |
| v0.14.0 | 2026-02 | Major WGSL coverage: 15/15 Essential reference shaders |
| **v0.13.1** | 2026-02 | SPIR-V OpArrayLength fix, 68 benchmarks, −32% allocs |
| v0.13.0 | 2026-02 | GLSL backend, HLSL/SPIR-V fixes, all 93 WGSL builtins |
| v0.12.1 | 2026-02 | HLSL codegen wiring fix, all 93 WGSL builtins |
| v0.12.0 | 2026-02 | Function calls, compute shader codegen |
| v0.11.1 | 2026-02 | SPIR-V opcode fixes, compute shader improvements |
| v0.11.0 | 2026-02 | SPIR-V if/else fix, 55 new built-in functions |
| v0.10.0 | 2026-02 | Local const, switch statements, storage textures |
| v0.9.0 | 2026-01 | Sampler types, swizzle, dev tools |
| v0.8.x | 2026-01 | SPIR-V Intel fixes, MSL [[position]] fix |
| v0.7.0 | 2025-12 | HLSL backend (~8.8K LOC) |
| v0.6.0 | 2025-12 | GLSL backend (~2.8K LOC) |
| v0.5.0 | 2025-12 | MSL backend (~3.6K LOC) |
| v0.4.0 | 2025-12 | Compute shaders, atomics, barriers |
| v0.3.0 | 2025-12 | Texture sampling, array init |
| v0.2.0 | 2025-12 | Type inference, deduplication |
| v0.1.0 | 2025-12 | Initial release (~10K LOC) |

→ **See [CHANGELOG.md](CHANGELOG.md) for detailed release notes**

---

## Contributing

We welcome contributions! Priority areas:

1. **Test Cases** — Real-world shaders for testing
2. **WGSL Features** — Additional language features
3. **Optimization** — Backend optimization passes
4. **Documentation** — Improve docs and examples

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Non-Goals

- **Runtime compilation** — naga is compile-time only
- **Shader reflection** — Use external tools

---

## License

MIT License — see [LICENSE](LICENSE) for details.
