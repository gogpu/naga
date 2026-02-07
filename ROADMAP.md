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

## Current State: v0.11.0

✅ **Production-ready** shader compiler (~19K LOC):
- Full WGSL frontend (lexer, parser, IR)
- 4 backend outputs (SPIR-V, MSL, GLSL, HLSL)
- 90+ WGSL built-in functions (math, geometric, bit manipulation, packing)
- Compute shaders (atomics, barriers, workgroups)
- Texture sampling and storage textures (50+ formats)
- Local const declarations and switch statements
- Correct SPIR-V structured control flow (`if/else`, `switch`, `loop`)
- Type inference and validation
- Development tools (nagac, spvdis)

---

## Upcoming

### v1.0.0 — Production Release
- [ ] Full WGSL specification compliance
- [ ] API stability guarantee
- [ ] Optimization passes (dead code elimination, constant folding)
- [ ] Source maps for debugging
- [ ] Comprehensive documentation

---

## Future Ideas

| Theme | Description |
|-------|-------------|
| **Optimization** | Dead code elimination, constant folding, inlining |
| **Source Maps** | Debug info mapping SPIR-V back to WGSL |
| **WGSL Extensions** | Pointer parameters, subgroups |
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
| **v0.11.0** | 2026-02 | SPIR-V if/else fix, 55 new built-in functions |
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
- **Ray tracing** — Beyond core WGSL scope
- **Mesh shaders** — Beyond core WGSL scope
- **Shader reflection** — Use external tools

---

## License

MIT License — see [LICENSE](LICENSE) for details.
