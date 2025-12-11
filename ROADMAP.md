# Naga Roadmap

> Pure Go Shader Compiler — WGSL to SPIR-V

## Released: v0.1.0 ✅

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

## Released: v0.2.0 ✅

**Focus:** Type system improvements

### Completed
- [x] Type inference for all expressions (~500 LOC)
- [x] Type deduplication (SPIR-V compliant) (~100 LOC)
- [x] Correct int/float/uint opcode selection
- [x] SPIR-V backend with proper type handling
- [x] 67+ unit tests

---

## Released: v0.3.0 ✅

**Focus:** Language completeness (~15K LOC total)

### Completed
- [x] Type inference for `let` bindings
- [x] Array initialization syntax (`array(1, 2, 3)` shorthand)
- [x] Texture sampling operations (textureSample, textureLoad, textureStore)
- [x] Texture query operations (textureDimensions, textureNumLevels)
- [x] SPIR-V image operations (OpImageSample*, OpImageFetch, OpImageQuery*)
- [x] 124 unit tests

---

## Next: v0.4.0

**Focus:** Compute shaders & DX improvements

### Completed
- [x] Better error messages with source locations (SourceError, FormatWithContext)
- [x] Storage buffers (`var<storage, read>`, `var<storage, read_write>`)
- [x] Workgroup shared memory (`var<workgroup>`)
- [x] Access mode parsing (`read`, `write`, `read_write`)
- [x] Workgroup size extraction (@workgroup_size)
- [x] Atomic type support (`atomic<u32>`, `atomic<i32>`)

### Planned
- [ ] Atomic operations (atomicAdd, atomicMax, etc.)
- [ ] Workgroup barrier (workgroupBarrier)
- [ ] Unused variable warnings

---

## Future: v0.5.0

**Focus:** Multiple backends & optimization

### Planned
- [ ] GLSL backend output
- [ ] Source maps for debugging
- [ ] Optimization passes (dead code elimination, constant folding)
- [ ] Unreachable code detection

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
