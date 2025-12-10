# Naga Roadmap

> Pure Go Shader Compiler — WGSL to SPIR-V

## Current: v0.1.0

**Status:** Released

Complete WGSL to SPIR-V compilation pipeline (~10K LOC).

### Completed
- WGSL lexer (140+ tokens)
- WGSL parser (recursive descent)
- IR with 33 expression types, 16 statement types
- IR validation
- AST → IR lowering
- SPIR-V binary writer
- SPIR-V backend (types, constants, functions, expressions)
- Control flow (if, loop, break, continue)
- 40+ built-in math functions (GLSL.std.450)
- Public API: `naga.Compile()`, `CompileWithOptions()`
- CLI tool: `nagac`

---

## Next: v0.2.0

**Focus:** Language completeness

### Planned
- [ ] Type inference for `let` bindings
- [ ] Array initialization syntax (`array<f32, 3>(1.0, 2.0, 3.0)`)
- [ ] Texture sampling operations (`textureSample`, `textureLoad`)
- [ ] More complete validation (unused variables, unreachable code)
- [ ] Better error messages with source locations

---

## Future: v0.3.0

**Focus:** Multiple backends

### Planned
- [ ] GLSL backend output
- [ ] Source maps for debugging
- [ ] Optimization passes (dead code elimination, constant folding)

---

## Goal: v1.0.0

**Focus:** Production ready

### Requirements
- [ ] Full WGSL specification compliance
- [ ] Comprehensive test suite
- [ ] Stable public API
- [ ] Performance optimization
- [ ] HLSL backend (optional)
- [ ] MSL backend (optional)

---

## Non-Goals (for now)

- Ray tracing extensions
- Mesh shaders
- WebGPU-specific extensions beyond core WGSL

---

## Contributing

Help wanted on:
- Additional WGSL features
- Test cases from real shaders
- Backend implementations
- Documentation

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
