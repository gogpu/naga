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

## Current State: v0.17.4 (2026-04-21)

✅ **Production-ready** shader compiler (~192K LOC) with **complete Rust naga parity**,
**100% SPIR-V binary validation**, and **experimental DXIL backend**:

### What We Have

- **Full WGSL frontend** — Lexer (120+ tokens), parser, AST → IR lowerer
- **6 backend outputs — ALL at 100% validation:**
  - SPIR-V: 165/165 spirv-val (Vulkan)
  - MSL: 91/91 Rust naga parity (Metal)
  - GLSL: 68/68 Rust naga parity (OpenGL)
  - HLSL: 72/72 Rust naga parity (DirectX 11/12)
  - DXIL: **161/170 IDxcValidator (94.7%)**, 82/208 DXC golden parity (DirectX 12, SM 6.0-6.5) — world's first Pure Go DXIL generator
  - IR: 144/144 Rust naga parity
- **DXIL backend** (~48K LOC, 315 unit tests) — VS/PS/CS/MS, CBV/SRV/UAV (read-only storage → SRV, read-write → UAV), atomics (i32/i64/f32 + image), barriers, ray query (35 intrinsics), wave ops (13 intrinsics), mesh shaders (SM 6.5), texture sampling (8 variants), matrix scalarization, pack/unpack, helper functions. Optimization passes: DCE, SROA, mem2reg, function inlining. `Options.BindingMap` for wgpu root signature compatibility. Eliminates FXC/DXC dependency. Verified 2400+ frames at 60 FPS on D3D12. Renders circles + text in gg production integration. Rust naga has NOT implemented this (open issue since 2020)
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
- **Development tools** — nagac CLI, spvdis disassembler, dxilval CLI (Pure Go `IDxcValidator` wrapper with three-layer defensive pre-check)

---

## Next Up

### Next: Internal Packages Refactor (ARCH-001)

| Task | Priority | Effort | Description |
|------|----------|--------|-------------|
| **Internal packages refactor** | P2 | 13 | Move implementations to `internal/`, reduce public API 398→~118 symbols |
| **SPIR-V structural parity** | P3 | 3 | 87/93 match + 6 allow-listed (3 ahead of Rust: Workgroup layout-free types) |
| **Test coverage 80%** | P2 | 8 | After ARCH-001 — wgsl 40%→80%, msl 40%→80%, hlsl 48%→80% |

### Next: DXIL Backend (direct DXIL generation, no FXC)

| Task | Priority | Effort | Description |
|------|----------|--------|-------------|
| **DXIL Phase 0: Bitcode writer** | P1 | 8 | ✅ Done. LLVM 3.7 bitcode writer, module builder, DXBC container, BYPASS hash. |
| **DXIL Phase 1: Vertex+fragment** | P1 | 21 | ✅ Done. Full IR → DXIL lowering: math, casts, control flow, locals, resources, signatures. |
| **DXIL CBV loads** | P1 | 2 | ✅ Done. `dx.op.cbufferLoadLegacy` for uniform buffers. Register index + component extraction. |
| **DXIL Phase 2a: Compute foundation** | P2 | 5 | ✅ Done. Thread IDs, numthreads, UAV bufferLoad/bufferStore. |
| **DXIL Phase 2b: Atomics + barriers** | P2 | 3 | ✅ Done. atomicBinOp (8 ops), atomicCmpXchg, dx.op.barrier, workgroup atomicrmw/cmpxchg. |
| **DXIL Phase 2c: Mesh shaders** | P2 | 5 | ✅ Done. SM 6.5 intrinsics (168-172), PSG1 signatures, mesh metadata. |
| **DXIL DXC validation** | ✅ Done | — | **161/170 IDxcValidator (94.7%)**. 82/208 DXC golden parity. gg 58/59. All features: ray query, image atomics, wave ops. |
| **DXIL Phase 3: SM 6.x features** | ✅ Done | — | Ray query (SM 6.5), wave intrinsics (SM 6.0), mesh shaders (SM 6.5), image atomics. |
| **DXIL real validation — `IDxcValidator` wrapper** | ✅ Done | — | `cmd/dxilval` CLI + `internal/dxcvalidator` Pure Go wrapper around Microsoft `dxil.dll` (zero CGO). Three-layer defensive stack: (0) emitter assertion against null entry-point function refs, (1) `PreCheckContainer` fixed-offset DXBC structural check, (2) `bitcheck.Check` minimal LLVM 3.7 bitstream walker verifying `!dx.entryPoints[i][0]` is non-null. Prevents the `dxil.dll+0xe9da` AV class on any input (our own naga output, DXC output, third-party tool output, hand-crafted garbage). Closes BUG-DXIL-VALIDATOR-REAL. |

### v1.0.0 — Stable Release

| Goal | Status | Notes |
|------|--------|-------|
| Complete Rust naga parity | ✅ Done | All 5 layers at 100% |
| SPIR-V binary validation | ✅ Done | 164/164 pass spirv-val |
| Compiler optimizations | ✅ Done | −32% allocs, −34% bytes |
| Ray tracing | ✅ Done | Ray query types, acceleration structures |
| Subgroup operations | ✅ Done | Ballot, shuffle, broadcast, quad |
| Mesh shaders | ✅ Done | MeshEXT/TaskEXT |
| Internal packages | Planned | ARCH-001: `internal/` for all backends |
| DXIL backend | ✅ Done | Direct DXIL, no FXC dependency (~48K LOC, 161/170 IDxcValidator) |
| API stability guarantee | Planned | Semantic versioning contract |
| Test coverage 80%+ | Planned | awesome-go requirement, after ARCH-001 |

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
| Dead code elimination | ✅ Done | Mark-and-sweep DCE in DXIL pipeline (dead locals, control flow, pure calls, resources) |
| Inlining | ✅ Done | Two-tier inline policy in DXIL pipeline (alias aggregates, early-return wrapping) |
| SROA | ✅ Done | Struct locals → per-member locals in DXIL pipeline |
| mem2reg | ✅ Done | SSA promotion with phi insertion (Phase A + B) in DXIL pipeline |
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
| **v0.16.4** | 2026-04 | GLSL workgroup zero-init per-element loop (12KB → compact) |
| **v0.16.3** | 2026-04 | HLSL FXC workgroup zero-init fix (330× faster). First in industry. |
| **v0.16.2** | 2026-04 | HLSL 72/72 parity (100%). ForceLoopBounding architecture fix. +14 shaders. |
| **v0.16.1** | 2026-04 | **164/164 spirv-val pass (100%).** +45 SPIR-V fixes. |
| **v0.16.0** | 2026-04 | GLSL TextureMappings + 34 SPIR-V validation fixes (119/164 pass) |
| **v0.15.0** | 2026-03 | ALL 5 backends 100% Rust parity: IR 144/144, SPIR-V 87/87, MSL 91/91, GLSL 68/68, HLSL 72/72. ~90K LOC |
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
