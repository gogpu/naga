# naga Roadmap

> **Pure Go Shader Compiler — WGSL to SPIR-V, MSL, GLSL, HLSL, and DXIL. Zero CGO.**
>
> Current: **v0.17.15** (June 2026) · Target: **v1.0.0** (December 2026)

---

## Vision

**naga** is the shader compiler foundation for the [GoGPU](https://github.com/gogpu) ecosystem. It compiles WGSL to every major GPU backend — Vulkan, Metal, OpenGL, DirectX 11/12, and DirectX 12 SM 6.x (DXIL) — in Pure Go with zero CGO.

Our goal: **the most complete, most tested, most portable shader compiler available outside of browser engines.** Not a toy, not a wrapper — a production compiler that powers real GPU applications (2D graphics, GUI toolkits, ML training, game engines).

### Core Principles

1. **Pure Go** — No CGO, easy cross-compilation, single binary deployment
2. **Six Backends** — SPIR-V (Vulkan), MSL (Metal), GLSL (OpenGL), HLSL (DirectX 11/12), DXIL (DirectX 12 SM 6.x)
3. **Spec Compliant** — W3C WGSL, Khronos SPIR-V, Metal Shading Language, HLSL, DXIL specifications
4. **Production-Ready** — Tested on real hardware (Intel, AMD, Apple Silicon), verified by external contributors
5. **Rust Naga Parity** — Structural golden-file parity with [Rust naga](https://github.com/gfx-rs/naga) across all text backends

### Who Uses naga

| Consumer | Use Case |
|----------|----------|
| [gogpu/wgpu](https://github.com/gogpu/wgpu) | Pure Go WebGPU — Vulkan, Metal, DX12, GLES, Software backends |
| [gogpu/gg](https://github.com/gogpu/gg) | 2D graphics — GPU SDF acceleration, Vello compute shaders |
| [gogpu/ui](https://github.com/gogpu/ui) | GUI toolkit — 22+ widgets, 4 themes, GPU-accelerated rendering |
| [born-ml/born](https://github.com/born-ml/born) | ML framework — GPU compute training (migrated from go-webgpu) |
| Flutter embedding (@georgebuilds) | Flutter Impeller on Pure Go GPU stack |

---

## Where We Are (v0.17.15)

**~323K LOC, 6 backends, 100% Rust parity on all text backends, 172/172 SPIR-V validation.**

| Backend | Parity | Validation | Status |
|---------|--------|------------|--------|
| SPIR-V | 87/87 golden | 172/172 spirv-val | Production |
| MSL | 91/91 golden | M3 Max Metal 3.1 verified | Production |
| GLSL | 68/68 golden | Version-aware binding (GL 3.3–4.6, ES 3.0–3.2) | Production |
| HLSL | 72/72 golden | SM 5.0 / SM 6.0 | Production |
| DXIL | 161/170 IDxcValidator (94.7%) | 105/208 DXC golden (55.1%) | Experimental |
| IR | 144/144 golden | — | Production |

---

## Roadmap to v1.0.0 (December 2026)

### Phase 1: API Stability (July 2026)

**Goal: stable public API contract — no breaking changes after this phase.**

| Task | Priority | Status | Description |
|------|----------|--------|-------------|
| ARCH-001: Internal packages | P1 | In Progress | Move implementations to `internal/`, reduce public API 398→~118 symbols |
| API review + freeze | P1 | Planned | Review all public types/functions, document stability guarantees |
| Deprecation pass | P1 | Planned | Mark unstable APIs as deprecated, provide migration paths |
| `go doc` audit | P2 | Planned | Every exported symbol has clear godoc, every package has doc.go |

### Phase 2: Test Coverage (August–September 2026)

**Goal: 80%+ per-package coverage — awesome-go requirement.**

| Task | Priority | Status | Description |
|------|----------|--------|-------------|
| Coverage wave 5+ | P1 | Planned | wgsl 65%→80%, msl 64%→80%, hlsl 71%→80% |
| DXIL downstream corpus | P2 | Planned | Real-world shader corpus from gg/ui/born production |
| DXIL differential testing | P2 | Planned | naga WGSL→DXIL vs naga WGSL→HLSL→DXC→DXIL roundtrip |
| Native Go fuzzing | P2 | Planned | `testing/F` fuzzing for WGSL parser and all backends |
| WebGPU CTS integration | P3 | Planned | Test against WebGPU Conformance Test Suite shaders |

### Phase 3: DXIL Graduation (September–October 2026)

**Goal: DXIL backend from experimental to production — 100% IDxcValidator, 80%+ DXC golden parity.**

| Task | Priority | Status | Description |
|------|----------|--------|-------------|
| IDxcValidator 170/170 | P1 | 161/170 | Remaining 9: ray tracing, image atomics, subgroups (compile_fail) |
| DXC golden parity 80%+ | P1 | 105/208 (55%) | Structural match with DXC roundtrip output |
| DXIL optimization passes | P2 | Done (5 passes) | DCE, SROA, mem2reg, inlining, strength reduction |
| Microsoft IDxcValidator AV issue | P2 | Draft ready | File upstream issue for validator crash on malformed input |
| SM 7.0 SPIR-V readiness | P3 | Monitoring | Microsoft adopting SPIR-V for SM 7.0 — our SPIR-V backend (172/172) is the long-term path |

### Phase 4: Hardening & Polish (November 2026)

**Goal: fix remaining edge cases, finalize documentation.**

| Task | Priority | Status | Description |
|------|----------|--------|-------------|
| SPIR-V structural parity | P2 | 87/93 | 6 allow-listed (3 ahead of Rust: workgroup layout-free types) |
| SPV-008 runtime residual | P2 | Workaround in gg | Nested for-loops/if-else incorrect runtime results despite valid SPIR-V |
| Shared optimization passes | P3 | DXIL-only now | Lift DCE/SROA/mem2reg to IR level for all backends |
| Documentation finalization | P1 | In Progress | ARCHITECTURE.md, CONTRIBUTING.md, pkg.go.dev, migration guide |

### v1.0.0 Release (December 2026)

**Goal: stable, documented, 80%+ tested, production-proven shader compiler.**

| Milestone | Status | Notes |
|-----------|--------|-------|
| Complete Rust naga parity | ✅ Done | All 5 text backends at 100% |
| SPIR-V binary validation 100% | ✅ Done | 172/172 pass spirv-val |
| DXIL backend production-ready | In Progress | 94.7% IDxcValidator, target 100% |
| API stability guarantee | Planned | Semantic versioning contract, Phase 1 |
| Test coverage 80%+ | Planned | awesome-go requirement, Phase 2 |
| Performance benchmarks published | ✅ Done | 68 benchmarks, −32% allocs, −34% bytes |
| Comprehensive documentation | In Progress | ARCHITECTURE.md, CONTRIBUTING.md, pkg.go.dev |
| Real-world validation | ✅ Done | gg, ui, born-ml, Quake (ironwail-go), Flutter embedding |

---

## Future Directions (post-v1.0)

### New Frontends

| Frontend | Priority | Description |
|----------|----------|-------------|
| **SPIR-V input** | Medium | SPIR-V → IR decompiler. Enables roundtrip testing and SPIR-V → MSL/GLSL/HLSL cross-compilation |
| **GLSL input** | Low | GLSL → IR parser. Legacy OpenGL shader migration to modern backends |
| **WGSL output** | Medium | IR → WGSL printer. Roundtrip testing, shader formatting/normalization |

### Tooling & Developer Experience

| Feature | Priority | Description |
|---------|----------|-------------|
| **Source maps** | Medium | Debug info mapping SPIR-V/DXIL instructions back to WGSL source locations |
| **LSP integration** | Medium | Language server protocol for WGSL editor support (syntax, diagnostics, hover) |
| **Shader minification** | Low | Remove debug names, compact identifiers for production builds |
| **WGSL formatter** | Low | Canonical formatting (like `gofmt` for WGSL) |

### Ecosystem Integration

| Integration | Description |
|-------------|-------------|
| **gogpu/compute** | GPU compute abstraction (shader cache, pipeline cache, buffer pool) — naga as shader compiler |
| **gogpu/editor** | Code editor widget — WGSL syntax highlighting powered by naga lexer |
| **WebGPU CTS** | Conformance testing against W3C WebGPU test suite |

---

## Non-Goals

- **Runtime compilation** — naga is ahead-of-time only (compile-time shader compilation)
- **Shader reflection** — use SPIR-V reflection tools (spirv-cross, spirv-reflect)
- **GLSL/HLSL as primary input** — WGSL is the primary language; other frontends are future/optional
- **LLVM dependency** — DXIL backend generates LLVM 3.7 bitcode directly, no LLVM library needed

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
        ┌──────────┬───────┼───────┬──────────┐
        ▼          ▼       ▼       ▼          ▼
   ┌─────────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐
   │  spirv/ │ │ msl/ │ │ glsl/│ │ hlsl/│ │ dxil/│
   └────┬────┘ └───┬──┘ └───┬──┘ └───┬──┘ └───┬──┘
        │          │        │        │         │
        ▼          ▼        ▼        ▼         ▼
     SPIR-V      MSL      GLSL    HLSL      DXIL
    (Vulkan)   (Metal)  (OpenGL) (DX11/12) (DX12 SM6)
```

---

## Released Versions

| Version | Date | Highlights |
|---------|------|------------|
| **v0.17.15** | 2026-06 | MSL function-scope workgroup vars (PR #77, @georgebuilds), per-EP zero-init filtering |
| **v0.17.14** | 2026-06 | GLSL version-aware binding, UniformInfo reflection, Codecov OIDC (BUG-GLES-005) |
| **v0.17.13** | 2026-05 | DXIL PHI node ordering fix, coverage waves 3-4, ~60% overall |
| **v0.17.12** | 2026-05 | ARCH-001 internal packages refactor, 13 panics→errors |
| **v0.16.4** | 2026-04 | GLSL workgroup zero-init per-element loop |
| **v0.16.2** | 2026-04 | HLSL 72/72 parity (100%), ForceLoopBounding |
| **v0.16.1** | 2026-04 | 164/164 spirv-val pass (100%), +45 SPIR-V fixes |
| **v0.15.0** | 2026-03 | ALL 5 backends 100% Rust parity (IR/SPIR-V/MSL/GLSL/HLSL) |
| **v0.13.1** | 2026-02 | SPIR-V OpArrayLength fix, 68 benchmarks, −32% allocs |
| **v0.1.0** | 2025-12 | Initial release (~10K LOC) |

→ **See [CHANGELOG.md](CHANGELOG.md) for detailed release notes**

---

## Contributing

We welcome contributions! Current priority areas:

1. **MSL/Metal testing** — real hardware verification on Apple Silicon
2. **Test coverage** — help reach 80%+ per-package (see [CONTRIBUTING.md](CONTRIBUTING.md))
3. **DXIL parity** — DXC golden diff reduction
4. **Real-world shaders** — test cases from production applications
5. **Documentation** — improve docs, examples, tutorials

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

MIT License — see [LICENSE](LICENSE) for details.
