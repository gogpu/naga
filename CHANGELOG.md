# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.17.3] - 2026-04-10

### Added

- **DXIL: CBV (Constant Buffer) loads** — `dx.op.cbufferLoadLegacy` for `var<uniform>`.
  Register index calculation (byteOffset/16), component extraction via `extractvalue`.
  Supports f32/i32/f64/i64/f16 overloads, struct member access at arbitrary offsets.

- **DXIL: Compute shader support (Phase 2)** — `@compute` entry points now compile to DXIL:
  - Thread ID builtins: `dx.op.threadId`, `dx.op.groupId`, `dx.op.threadIdInGroup`,
    `dx.op.flattenedThreadIdInGroup`
  - `numthreads` metadata from `@workgroup_size(X,Y,Z)`
  - UAV storage buffers: `dx.op.bufferLoad`/`dx.op.bufferStore` for `var<storage, read_write>`
  - Atomic operations: `dx.op.atomicBinOp` (add, subtract, and, or, xor, min, max, exchange),
    `dx.op.atomicCompareExchange`, atomic load/store
  - Barriers: `dx.op.barrier` with storage/workgroup/subgroup flag mapping
  - Reference: Mesa `nir_to_dxil.c`

### Fixed

- **DXIL: bitcode binary operation opcodes** — `BinOpKind` constants used LLVM IR enum
  numbering (FAdd=1) instead of bitcode unified opcodes (Add/FAdd=0). DXC decoded our
  FAdd as FSub. Fixed to match Mesa `dxil_module.h` encoding.

- **DXIL: finalize() operand remapping** — `finalize()` remapped ALL instruction operands
  as value IDs, corrupting type IDs, opcodes, alignment values, and basic block indices.
  Added `valueOperandIndices()` that precisely identifies which operands are values per
  instruction type.

- **DXIL: alloca alignment encoding** — Was `log2(bytes)`, should be `log2(bytes)+1` per
  LLVM 3.7 / Mesa `dxil_emit_alloca()`.

- **DXIL: vector local variable scalarization** — Single alloca for `vec4<f32>` replaced
  with per-component allocas. Store/Load now operate on correct components.

- **DXIL: GEP struct access** — Added `getelementptr` (FUNC_CODE_INST_GEP=43) for struct
  member access from local variable pointers. Nested struct access with flat offset
  computation. Struct store decomposed into per-scalar-field GEP + store.

- **DXIL: retail hash wired up** — `ComputeRetailHash()` (INF-0004 modified MD5) was
  implemented but never called. Now used when `UseBypassHash=false`.

- **DXIL: push constants as CBV** — `SpacePushConstant` and `SpaceImmediate` globals
  classified as CBV resources with synthetic bindings.

- **DXIL: resource metadata rewrite** — Per-class metadata matching Mesa exactly. CBV
  fields[6] = buffer size (was resource kind), SRV/UAV 9-11 fields with element type tags,
  fields[1] = undef pointer (was null). Fixes 23 DXC validator crashes.

- **DXIL: struct return decomposition** — Multi-output shaders (struct returns with multiple
  @location fields) now decompose into per-field GEP + scalar load + storeOutput.

- **DXIL: binary op vector scalarization** — Vector binary operations decomposed into
  per-component scalar ops with scalar-vector broadcast.

- **DXIL: global variable allocas** — Non-resource globals (workgroup, private) get proper
  alloca pointers instead of placeholder values.

- **DXIL: array/matrix CBV loads, dynamic GEP, UAV constant-index fix** — Matrix
  CBV loads (one cbufferLoadLegacy per column), array local variable allocas,
  dynamic Access with GEP, UAV constant-index access fix. 3 more shaders pass DXC.

- **DXIL: SRV/UAV direct loads, dynamic CBV index, array output decomposition** —
  SRV/UAV loads routed to bufferLoad (not LLVM load), dynamic CBV index with stride
  arithmetic, ZeroValue/Compose dynamic array access, array-typed builtin outputs.

- **DXIL: typed undef for bufferStore, deep UAV chains, entry-block allocas** —
  Float bufferStore uses typed undef (f32, was i32). Deep UAV pointer chains
  (struct-wrapped arrays, nested Access). All allocas in entry block (was lazy).

- **DXIL: mesh shader intrinsics (SM 6.5)** — SetMeshOutputCounts (168),
  StoreVertexOutput (171), StorePrimitiveOutput (172), EmitIndices (169).
  All 4 mesh shaders pass DXC (9 entry points). PSG1 primitive signatures.

- **DXIL: struct-typed entry point arguments** — Fragment/vertex shaders with struct
  inputs (e.g., `vertex: VertexOutput`) now correctly load per-member with row tracking.
  Fixes 14 additional shaders.

- **DXIL: helper function emission with per-function value ID isolation** — Each helper
  function gets independent value ID space. `collectCalledFunctions()` pre-scans entry point.

- **DXIL: switch statements** — Cascading `icmp eq` + conditional branches with merge block.

- **DXIL: 17 pack/unpack math functions** — pack4x8snorm/unorm, unpack4x8snorm/unorm,
  pack2x16float/snorm/unorm, unpack variants, pack4xI8/U8/clamp, unpack4xI8/U8.

- **DXIL: matrix operations** — mat*vec, vec*mat, mat*mat, mat+/-mat, mat*scalar, transpose.
  All scalarized to component-wise DXIL instructions (dot products for multiply).

- **DXIL: workgroup atomics** — LLVM `atomicrmw`/`cmpxchg` for workgroup variables
  (add, sub, and, or, xor, min, max, xchg, compare-exchange).

- **DXIL: vector select scalarization, math broadcast, ExprArrayLength, helper vector returns,
  struct member component stores, atomic compare-exchange result struct, abstract literals,
  matrix alloca, refract/modf/frexp/quantizeF16** — systematic fixes across emitter.

- **DXIL: atomic type width support (i32/i64/f32)** — workgroup atomics now use correct
  type width instead of hardcoded i32. Fixes f32/i64 atomic shaders.

- **DXIL: ExprOverride + ProcessOverrides** — pipeline override constants now compile.
  DXIL test harness processes overrides (same as SPIR-V/HLSL).

- **DXIL: matrix column extraction, f16 constant encoding** — AccessIndex on matrix
  returns full column vector (was single scalar). F16 constants use IEEE 754 half-precision.

- **DXIL: typed zero constants, CBV/UAV resource pass-through, struct field loads** —
  f16/i64 zero constants use correct types. CBV resource AccessIndex pass-through.
  UAV struct field byte offset + scalar type resolution. Multi-register struct loads.

- **DXIL: FRem lowering** — LLVM FRem lowered to `a - b * floor(a/b)` (DXC rejects FRem).

- **DXIL: texture intrinsics** — `dx.op.getDimensions` (72) for textureDimensions/numLevels/numSamples/numLayers,
  `dx.op.textureLoad` (66) for imageLoad, `dx.op.textureStore` (67) for imageStore.

- **DXIL: complex UAV access chains, array load/store decomposition, workgroup uniform load** —
  Matrix column+component UAV access, multi-member struct arrays, 512-element array copy,
  StmtWorkGroupUniformLoad as barrier+load+barrier pattern.

- **DXIL: 8 texture sampling intrinsics** — OpSample(60), OpSampleBias(61),
  OpSampleLevel(62), OpSampleGrad(63), OpSampleCmp(64), OpSampleCmpLevelZero(65),
  OpTextureGather(73), OpTextureGatherCmp(74). Previously only OpSample.

- **DXIL: binding array dynamic handles** — `dx.op.createHandle` with dynamic index
  for `binding_array<T>` resources. Both ExprAccess and ExprAccessIndex paths.

- **DXIL: NumWorkGroups via synthetic CBV** — `$Globals` CBV with cbufferLoadLegacy
  for compute dispatch dimensions (DXIL has no intrinsic, matches DXC approach).

- **DXIL: DXC dumpbin validation** — **155/165 shaders pass DXC dumpbin (93.9%)**.
  1 val_fail (binding-buffer-arrays), 9 compile_fail (ray tracing 4, image atomics 2,
  subgroups 1, no entry points 2).
  Added `TestDxilValSummary` test (analogous to `TestSpirvValBinarySummary`).

### Fixed (other backends)

- **SPIR-V: OpIMul for vec4\<f32\>*f32 after unpack4x8unorm (BUG-SPIRV-003)** —
  `resolveMathType()` missing return types for 10 pack/unpack functions. `unpack4x8unorm(u32)`
  returned `u32` instead of `vec4<f32>`, causing `OpIMul` instead of `OpVectorTimesScalar`.
  Fixes gg#252, naga#61.

### Changed

- **SPIR-V Rust reference: allow-list for intentional divergences** — Unified allow-list
  across all backends (SPIR-V/MSL/HLSL/GLSL) for shaders where our output intentionally
  differs from Rust naga (Workgroup layout-free types per VUID-StandaloneSpirv-None-10684,
  no-compact-pass for entry-point-less shaders). 0 fail across all backends.

- **SPIR-V validation: 165/165** — Added `ptr-deref-test` shader (+1 from v0.17.2).

## [0.17.2] - 2026-04-10

### Fixed

- **SPIR-V: OpFunctionCall type mismatch with Workgroup values (BUG-SPIRV-002)** — Regression
  from v0.17.1 Workgroup fix. Values loaded from `Workgroup` variables had layout-free types
  that propagated into `OpFunctionCall` arguments, causing type mismatch with decorated function
  parameters. Fix: insert `OpCopyLogical` immediately after every `OpLoad` from Workgroup
  (in `emitLoad`, `emitAccess`, `emitAccessIndex`), converting layout-free → decorated at the
  load point. Guard `maybeCopyLogicalForStore` against double-conversion when value is already
  decorated. Validated: 164/164 naga shaders + 23/23 gg GPU shaders pass spirv-val v2026.1.
  (gogpu/wgpu#134, reported by @SideFx via gogpu/ui#67)

## [0.17.1] - 2026-04-08

### Fixed

- **SPIR-V: Workgroup ArrayStride violation (VUID-StandaloneSpirv-None-10684)** — SPIR-V backend
  emitted `ArrayStride` decoration on array types used in `Workgroup` storage class, which is
  forbidden without `SPV_KHR_workgroup_memory_explicit_layout`. Intel Vulkan silently accepted this,
  but Qualcomm Adreno correctly rejected it — causing invisible text on Snapdragon X Elite.
  Fix: emit separate layout-free type declarations for Workgroup variables (arrays without
  `ArrayStride`, structs without `Offset`/`MatrixStride`). Uses `OpCopyLogical` (SPIR-V 1.4+)
  to bridge Workgroup ↔ Storage type mismatches. Constant deduplication added to prevent
  duplicate `OpConstant` IDs that break `OpCopyLogical` type equivalence.
  Rust naga has the same bug ([gfx-rs/wgpu#7696](https://github.com/gfx-rs/wgpu/issues/7696),
  fix PR [#9295](https://github.com/gfx-rs/wgpu/pull/9295) still open).
  (gogpu/wgpu#134, reported by @SideFx via gogpu/ui#67)

## [0.17.0] - 2026-04-06

### Added

- **DXIL backend (experimental)** — First Pure Go DXIL generator. Direct DXIL
  generation from naga IR for DX12 Shader Model 6.0. Eliminates FXC/DXC compiler
  dependency entirely. Zero CGO, zero external dependencies.
  **Verified: 2400+ frames at 60 FPS on D3D12 (Intel Iris Xe).**
  Rust naga has not implemented this (open issue since 2020).
  190 tests, ~12.5K LOC. Public API: `dxil.Compile()`, `dxil.DefaultOptions()`.
  - LLVM 3.7 bitcode writer with VBR encoding, nested blocks, abbreviation records
  - DXBC container with DXIL, ISG1, OSG1, PSV0, SFI0, HASH parts
  - BYPASS hash (Agility SDK 1.615+, January 2025) — no dxil.dll signing needed
  - Full expression lowering: literals, binary/unary ops, compose, access, splat,
    select, load/store, swizzle, derivatives, relational, type casts (10 LLVM
    cast opcodes), math intrinsics (30+ functions including min/max/clamp/dot/
    cross/mix/fma/smoothstep/pow/length/distance/normalize)
  - Control flow: LLVM basic blocks with br/br_cond, loops with back edges,
    break/continue via loop context stack, BreakIf support
  - Local variables: alloca + load + store with proper bitcode serialization
  - Resource bindings: dx.op.createHandle(57) for CBV/SRV/Sampler,
    dx.op.sample(60) for texture sampling, dx.resources metadata
  - I/O signatures: ISG1/OSG1 with semantic mapping (SV_Position, TEXCOORD,
    SV_Target, SV_VertexID, SV_InstanceID, SV_IsFrontFace, SV_Depth)
  - Pipeline state validation: PSV0 with runtime info, shader stage, wave lanes
  - Vector scalarization: DXIL has no native vectors — vec4 = 4 separate values,
    per-component tracking via exprComponents map
  - dx.op intrinsic system: overload-typed function declarations, lazy caching
  - Value ID remapping: emitter local IDs → serializer global numbering via finalize()
  - Error hardening: unsupported features return errors (not silent skip)

## [0.16.6] - 2026-04-05

### Performance

- **TypeRegistry zero-alloc lookups** — Refactored `normalizeType()` to `appendTypeKey()`
  using `keyBuf []byte` with inline `string(keyBuf)` map index (Go compiler optimization).
  Eliminates heap string allocation per type lookup. large_pbr: -59 allocs/op.
- **Lexer token preallocation** — Token slice estimate `len(source)/4` (was `/6`).
  Prevents slice regrowth for typical shaders.
- **SPIR-V Backend reuse** — `Backend.Reset()` + `ModuleBuilder.Reset()` clear
  all state without deallocating (Go 1.21+ `clear()` keeps map capacity).
  `Compile()` calls `Reset()` internally. Small shaders: 4.9× fewer allocs
  (68→14), 14× fewer bytes (17KB→1.2KB). Reuse benchmarks included.
- **Lowerer/Parser pre-sizing** — Expression, statement, declaration slices
  pre-allocated based on AST size estimates. TypeRegistry capacity hints.
- **Overall: 594 → 562 allocs/op (-5.4%). SPIR-V reuse: 68 → 14 allocs (4.9×).**

## [0.16.5] - 2026-04-05

### Changed

- **Dead code removal** — Removed 3 unused functions found by full codebase audit
  (398 symbols scanned across 6 packages): `flattenBinding` (glsl), 
  `concretizeTypeInner` (ir/compact), `resolveLiteralTypeInner` (ir/process_overrides).

## [0.16.4] - 2026-04-04

### Fixed

- **GLSL: per-element loop for workgroup array zero-init** — Same fix as HLSL:
  arrays >= 256 elements use `for` loop instead of inline constructor list
  (`Type[256](elem, elem, ... ×256)` = 12KB single line). Prevents GL driver
  compiler slowdown and potential crashes on mobile/embedded GL drivers.

## [0.16.3] - 2026-04-04

### Fixed

- **Per-element loop for workgroup array zero-init** — FXC hangs 22 seconds on
  `(StructType[256])0` bulk assign. Replaced with per-element `for` loop:
  `for (uint _naga_zi_0 = 0u; ...) { arr[_naga_zi_0] = (ElementType)0; }`.
  Compilation time: 22s → 68ms. Handles nested arrays recursively.
  First implementation in the industry to fix this — Rust naga, ANGLE, Dawn
  all have the same bug or workaround it differently.
  See `docs/dev/research/HLSL-ZERO-INIT-FXC-HANG-RESEARCH.md`.
- **Defaults changed to match Rust** — `ForceLoopBounding: true`,
  `RestrictIndexing: true` (were both `false`).

#### HLSL backend: parity fixes

- **Dot4I8Packed/Dot4U8Packed** — dot4add intrinsics (SM 6.4+) with polyfill. (3 shaders)
- **Pack4xI8/U8/Clamp + Unpack4xI8/U8** — shift+mask polyfills. (2 shaders)
- **LiteralF16** — float16_t with h suffix. (2 shaders)
- **ValuePointerType** — storage access stride computation. (1 shader)
- **StmtImageAtomic** — InterlockedXxx on RWTexture coordinates. (2 shaders)
- **ExprWorkGroupUniformLoadResult** — barrier + typed load + barrier. (1 shader)
- **ExprBinary in global scope** — const binary expressions. (1 shader)
- **Float exponent formatting** — removed spurious '+' in exponents. (1 shader)
- **naga_extractBits/naga_insertBits** — per-type overloads matching Rust. (1 shader)
- **Pack/Unpack 2x16/4x8 snorm/unorm/float** — all 10 inline polyfills. (1 shader)
- **f16 infrastructure** — NagaConstants struct, matrix padding, storage Store patterns. (1 shader)
- **Demand-driven StorageLoadHelpers** — match Rust approach. (1 shader)

## [0.16.1] - 2026-04-04

### Fixed

#### SPIR-V binary validation: 29 → 0 failures, 16 → 0 compile failures (164/164 pass)

All fixes validated against Rust naga reference (`naga/src/back/spv/`).
SPIR-V TestRustReference: 4/87 → 90/93 (3 minor structural gaps, not bugs).

- **Depth texture sampling** — Dref sampling uses scalar f32 result type;
  non-Dref depth sampling uses vec4 then CompositeExtract for first component.
  Matches Rust naga image.rs:823-837. (4 shaders)

- **Function pointer arguments** — `isPointerExpression` now recognizes
  pointer-typed FunctionArgument params. `emitCall` uses `emitPointerExpression`
  for pointer parameters. (1 shader: access)

- **Integer dot product** — OpDot only works on float vectors. Integer dot
  products now use manual expansion (CompositeExtract + IMul + IAdd).
  Matches Rust naga write_dot_product. (1 shader)

- **FMix scalar-to-vector splatting** — When FMix selector is scalar but
  operands are vectors, splat scalar via OpCompositeConstruct.
  Matches Rust naga Mix handling. (2 shaders)

- **Integer vec×scalar multiply** — OpIMul requires matching types. Scalar
  operand splatted to vector via OpCompositeConstruct before IMul.
  Matches Rust naga write_vector_scalar_mult. (1 shader)

- **Matrix add/sub decomposition** — SPIR-V FAdd/FSub don't work on matrix
  types. Decomposed into column-wise vector operations.
  Matches Rust naga write_matrix_matrix_column_op. (1 shader)

- **VectorTimesMatrix result type** — Result vector size must equal matrix
  column count, not left vector size. Also fixed MatrixTimesMatrix for
  non-square matrices. (1 shader)

- **ImageQuerySize dimensionality** — Result type now matches image
  dimensionality (2D→vec2, 3D→vec3, etc.) instead of always vec3.
  ImageQueryNumLayers uses OpImageQuerySizeLod + CompositeExtract.
  Matches Rust naga image.rs:1142-1210. (1 shader)

- **Bitcast width conversion** — Width-changing integer casts now use
  OpUConvert/OpSConvert instead of OpBitcast (which requires equal bit width).
  Matches Rust naga block.rs:2061-2066. (1 shader + 5 bonus)

- **Atomic int64 result types** — Atomic operations on int64 now use correct
  64-bit scalar result type instead of hardcoded 32-bit. (2 shaders)

- **AtomicCompareExchange struct** — OpAtomicCompareExchange returns scalar,
  not struct. Now constructs `{old_value, exchanged}` struct via
  OpCompositeConstruct with OpIEqual for the exchanged bool.
  Matches Rust naga block.rs:3414+. (2 shaders + 1 bonus)

- **ControlBarrier SubgroupMemory** — Fixed MemorySemanticsSubgroupMemory
  constant (0x80, not 0x200). Barrier emission now matches Rust naga
  write_control_barrier. (1 shader)

- **ConstOffset image operand** — Changed from ConstOffsets (0x40, Gather-only)
  to ConstOffset (0x08, valid for all sampling). (partial fix)

- **Image store coordinates** — Integer coordinates now use proper bitcast
  instead of float conversion. (2 shaders)

- **External texture handling** — ImageClassExternal uses float sampled type
  and OpImageQuerySizeLod for queries. (1 shader)

- **ImageGatherExtended capability** — Removed spurious emission for
  ConstOffset operands. Only needed for dynamic Offset. (1 shader)

- **spirv-val target env** — Uses spv1.6 for general SPIR-V correctness
  validation, with --uniform-buffer-standard-layout for std430 matrix stride.

- **Pointer function arguments** — Copy-in/copy-out (spill) pattern for passing
  OpAccessChain results to OpFunctionCall. SPIR-V requires memory object declarations
  (OpVariable/OpFunctionParameter) as pointer arguments. (3 shaders)

- **Image Lod float conversion** — SPIR-V ExplicitLod requires float Lod operand.
  Integer level converted via OpConvertSToF/OpConvertUToF. (2 shaders)

- **Binding arrays RuntimeArray** — Proper wrapping in Block-decorated struct. (1 shader)

- **Mesh shaders** — MeshEXT/TaskEXT execution models, SPV_EXT_mesh_shader
  extension, OutputVertices/OutputPrimitivesEXT execution modes. (4 shaders)

- **Pipeline overrides (OpSpecConstant)** — ExprOverride expressions emitted as
  OpSpecConstant/OpSpecConstantComposite with SpecId decoration. (3 shaders)

- **Image atomics (OpImageTexelPointer)** — StmtImageAtomic emits
  OpImageTexelPointer then standard atomic ops on the texel pointer. (2 shaders)

- **WorkGroupUniformLoad** — Emitted as ControlBarrier + OpLoad + ControlBarrier
  matching Rust naga's pattern. (1 shader)

- **Pack4xI8/U8/Clamp + Unpack4xI8/U8** — Shift+mask pack/unpack polyfills
  for 4×8-bit integer packing. (5 shaders)

- **ProcessOverrides remap fixes** — StmtAtomic.Compare and StmtRayQuery
  fields now correctly remapped during override processing. (2 shaders)

### Known Issues

- **0 SPIR-V binary validation failures.** All 164 shaders compile and pass spirv-val.

- **3 SPIR-V structural differences** from Rust reference (Int8 capability,
  decoration counts) — not validation or compilation issues.

## [0.16.0] - 2026-04-02

### Added

- **GLSL: TextureMappings in TranslationInfo** — Reports texture-sampler
  binding pairs for combined sampler2D, enabling GLES HAL SamplerBindMap
  construction. Matches Rust naga ReflectionInfo pattern.

### Fixed

#### SPIR-V binary validation: 63 → 29 failures (−34 fixed)

All fixes validated against Rust naga reference (`naga/src/back/spv/`).

- **SPV_KHR_storage_buffer_storage_class extension** — StorageBuffer storage
  class requires this extension for SPIR-V < 1.3 (default version is 1.1).
  Matches Rust naga writer.rs:2580. (5 shaders)

- **Workgroup ArrayStride decoration** — Workgroup variables must not have
  explicit layout decorations per VUID-StandaloneSpirv-None-10684. Added
  layout-free type emission for Workgroup address space. (1 shader)

- **OpLoad on runtime-sized arrays** — SPIR-V forbids loading entire
  runtime-sized arrays. Global variables containing runtime arrays now return
  pointers instead of loaded values. (2 shaders)

- **OpImageFetch result type** — Hardcoded vec4<f32> replaced with correct
  type derived from image SampledKind. (1 shader)

- **SPV_KHR_multiview, SPV_KHR_fragment_shader_barycentric,
  SPV_KHR_integer_dot_product extensions** — Added when corresponding
  capabilities are used. (6 shaders)

- **CapabilityImageGatherExtended** — Added for textureGather with offset. (2 shaders)

- **SampleMask BuiltIn** — Wrapped in array<u32,1> per SPIR-V spec. (1 shader)

- **SPIR-V version 1.3 requirement** — RequireVersion(1.3) for subgroup/
  GroupNonUniform capabilities. (1 shader)

- **Duplicate OpTypeSampler** — Cached to emit exactly once. (3 shaders)

- **ClipDistance/PrimitiveIndex builtins** — Added to builtinToSPIRV mapping. (1 shader)

- **OpAtomicFAddEXT (6035)** — Float32 atomics use OpAtomicFAddEXT instead
  of OpAtomicIAdd. Atomic scalar resolution returns kind+width for int64. (4 shaders)

- **AtomicCompareExchange unequal semantics** — Unequal operand uses Acquire
  (not AcquireRelease) per VUID-StandaloneSpirv-UnequalMemorySemantics-10875. (3 shaders)

- **ExecutionModeDepthReplacing** — Fragment shaders writing FragDepth now
  emit DepthReplacing per VUID-FragDepth-FragDepth-04216. (2 shaders)

- **SPV_KHR_integer_dot_product version comparison** — Fixed 0x10006 → 0x10600
  (SPIR-V version encoding is major<<16|minor<<8). (2 shaders)

#### GLSL

- **Depth texture combined sampler naming** — When a depth texture is used with
  both regular and comparison samplers, the combined sampler names were swapped
  due to non-deterministic Go map iteration. Fixed with deterministic sorting. (1 shader)

### Known Issues

- **29 SPIR-V binary validation failures** at release time. Fixed in [Unreleased]
  section above (29 → 6 remaining).

## [0.15.2] - 2026-04-01

### Fixed

- **HLSL: revert direct sampler kostyl, restore heap pattern** — Reverted the
  direct sampler register binding from v0.15.1. DX12 HAL now implements proper
  sampler heap.

## [0.15.1] - 2026-03-31

### Fixed

- **HLSL: revert direct sampler register binding mode** — Samplers always use the
  sampler heap indirection pattern (`nagaSamplerHeap[indexBuffer[N]]`), matching
  Rust wgpu-hal architecture. The DX12 HAL now properly implements the global
  sampler heap with per-bind-group sampler index buffers and provides
  `SamplerBufferBindingMap` to naga during shader compilation.

## [0.15.0] - 2026-03-30

### Highlights

- **ALL 5 backends at 100% Rust naga parity** — complete exact output match
- **IR Reference: 144/144 (100%)** — complete structural match with Rust naga on ALL shaders
- **SPIR-V Backend: 87/87 (100%)** — exact output match with Rust naga (was 40/87)
- **MSL Backend: 91/91 (100%)** — exact output match with Rust naga
- **GLSL Backend: 68/68 (100%)** — exact output match with Rust naga
- **HLSL Backend: 58/58 (100%)** — exact output match with Rust naga
- **ir/ test coverage: 82%** — up from 24%, with 148 new unit tests
- **994 golden output files** across 4 backends, 164 test shaders
- **Quake 1** renders on gogpu/wgpu Pure Go Vulkan backend (gogpu#157)

### Added

#### IR Level
- **needsPreEmit auto-interrupt** — `addExpression` automatically interrupts emitter for
  Literal, Constant, ZeroValue, GlobalVariable, FunctionArgument, LocalVariable, Override
  expressions, matching Rust naga's `constant_evaluator::append_expr`
- **Splat in GlobalExpressions** — single-arg vector constructors produce `ExprSplat` GE
- **ZeroValue in GlobalExpressions** — zero-arg constructors produce `ExprZeroValue` GE
- **Swizzle const-fold** — `vec4(vec2(1,2), vec2(3,4)).wzyx` fully evaluated at compile time
- **dot4I8Packed/dot4U8Packed const-fold** — packed dot product evaluated at compile time
- **Abstract composite constant inline** — `ABSTRACT_ARRAY[i]` inlines array literals
- **Binary const eval → GE** — `vec2(1.0f) + vec2(3.0f, 4.0f)` produces GlobalExpressions directly
- **Constant alias GE sharing** — `const ALIAS = ORIGINAL` reuses GE handle
- **Void call emitter restart** — proper emitter state after void function calls
- **Matrix column grouping** in nested constructor GlobalExpressions
- **As conversion for scalar type mismatch** — `vec4f(u32_value)` inserts `ExprAs` convert

#### SPIR-V Backend
- **87/87 (100%) Rust naga parity** — exact binary output match on all reference shaders
- **Integer div/mod safety wrappers** — `naga_div`/`naga_mod` helper functions prevent
  division by zero and i32 MIN/-1 overflow, matching Rust naga behavior
- **Image bounds checking** — Restrict and ReadZeroSkipWrite policies with coordinate
  clamping for texture load/store operations
- **Ray query helper functions** — 6 helper functions per ray query (initialize, proceed,
  terminate, committed/candidate intersection getters)
- **Force loop bounding** — iteration counter prevents infinite loops on malformed shaders
- **Workgroup zero-init polyfill** — zero-initializes workgroup memory at entry point start
- **20+ new capabilities** — ClipDistance, Geometry, GroupNonUniform, Float16 storage,
  AtomicFloat32AddEXT, StorageImageExtendedFormats, SampleMaskPostDepthCoverage,
  SubgroupBallotKHR, and more
- **NonWritable/Flat/NonUniform decorations** — correct propagation for binding arrays
  and storage buffer access
- **OpLoad dereferencePointerType fix** — correct type resolution for pointer loads
- **Float16 constant emission** — proper OpConstant for f16 values
- **ModfStruct/FrexpStruct fix** — correct result struct types for decomposition functions
- **Saturate FClamp fix** — `saturate()` emits FClamp with 0.0/1.0 bounds
- **f16 I/O polyfill** — bitcast-based conversion for f16 entry point interface variables
- **Composite spilling** — by-value dynamic indexing spills composites to local variables
- **SSA entry point struct args** — correct argument handling for entry point structs
- **Capability-aware dot4 polyfill** — software emulation when DotProduct unavailable
- **Entry point interface vars** — SPIR-V 1.4+ globals, ForcePointSize decoration

#### GLSL Backend
- **68/68 (100%) Rust naga parity** — exact text output match on all reference shaders
- **`dominates_global_use` reachability** — correct global variable emission per entry point
- **ProcessOverrides** — pipeline constant specialization for GLSL output
- **Image bounds checking** — coordinate clamping for texture operations

#### MSL Backend
- **Vertex Pulling Transform** — complete implementation in `msl/vertex_pulling.go`:
  `_mslBufferSizes` struct, 42 vertex format unpacking functions, buffer type structs,
  bounds-checked byte unpacking from raw vertex buffers
- **External Texture Support** — `NagaExternalTextureWrapper` struct, multi-plane YUV
  sampling (`nagaTextureSampleBaseClampToEdge`), texture load (`nagaTextureLoadExternal`),
  dimensions query (`nagaTextureDimensionsExternal`), transfer function color space conversion
- **TOML inline table parsing** — `msl_pipeline = { key = val }` format support

#### WGSL Frontend
- **Ray query support** — `acceleration_structure`, `ray_query` types, `RayDesc`/
  `RayIntersection` predeclared structs, `RAY_FLAG_*` constants, 7 ray query
  builtins. Full SPIR-V + HLSL + MSL backend emission.
- **Subgroup operations** — `subgroupBallot`, `subgroupAdd/Mul/Min/Max/And/Or/Xor`,
  `subgroupBroadcast/First`, `subgroupShuffle*`, `quadSwap*`, `subgroupBarrier`.
  Full SPIR-V backend with correct capabilities. HLSL/MSL/GLSL placeholders.
- **Vector const-exprs** — component-wise binary operations at module scope
  (`const X = vec2(1.0) + vec2(3.0, 4.0)`). Splat expansion for scalar→vector.
- **Override declarations** — `@id(N) override name: type = default;`
- **f16/i64/u64/f64 scalar types** — `enable f16/int64;` directives, literal
  suffixes (`1.0h`, `42li`, `42lu`, `1.0lf`), type constructors
- **`break if` syntax** — continuing block `break if condition;`
- **Type aliases** — `alias FVec3 = vec3<f32>;` with constructor support
- **`const_assert` declarations** — compile-time assertions (evaluated as no-op)
- **`workgroupUniformLoad` builtin** — maps to `ir.StmtWorkGroupUniformLoad`
- **`atomicCompareExchange` result struct** — `.old_value`/`.exchanged` member access
- **Template list edge cases** — trailing commas, `>=` disambiguation
- **`diagnostic` directive** — top-level `diagnostic(...)` skipping
- **164 WGSL test shaders** — 144/144 Rust reference (100%) + 20 custom

### Changed

#### SPIR-V Backend
- **Block Ownership Model (NAGA-ARCH-001)** — refactored function emission
  from flat instruction list to block-based architecture matching Rust naga.
  Each SPIR-V basic block is now a first-class `Block` struct consumed by
  `FunctionBuilder`. `LoopContext` passed by value for isolated nested loop
  contexts. Eliminates `loopStack`/`breakStack` mutable state and
  `blockEndsWithTerminator()` post-hoc checks. Produces identical SPIR-V
  output — zero behavioral changes.

### Performance

#### GLSL Backend
- **Dead code elimination via entry-point reachability (GLSL-001)** — GLSL
  writer now walks the call graph from the target entry point and only emits
  reachable types, constants, globals, and functions. SDF fragment shader
  output reduced from 639KB to target <50KB. Fixes 5-10 second startup
  delay on GLES backend ([naga#42](https://github.com/gogpu/naga/issues/42)).

### Fixed

#### SPIR-V Backend
- **Workgroup Offset decoration removed (SPIRV-001)** — `emitTypeNoLayout()`
  now handles struct types by creating separate type IDs without Offset,
  ColMajor, or MatrixStride member decorations. Fixes Vulkan validation error
  `VUID-StandaloneSpirv-None-10684` on all Vello compute shaders. Matches
  Rust naga `global_needs_wrapper()` pattern.

#### MSL Backend
- **Workgroup variables emitted as entry point parameters (MSL-002)** —
  `var<workgroup>` globals now appear as `threadgroup T& name` parameters
  in compute entry points. Previously skipped because they have no resource
  binding. Fixes `undeclared identifier 'sh_scratch'` on macOS Metal.
- **Barrier calls fully namespace-qualified** — `threadgroup_barrier()` and
  `mem_flags` now prefixed with `metal::`. Fixes `undeclared identifier
  'mem_flags'` on strict Metal compilers.
- **Runtime-sized array typedef** — dynamic arrays (`array<T>`) now emit
  `typedef T name[1];` in MSL output. Fixes `unknown type name 'type_6'`
  for storage buffer parameters.

## [0.14.8] - 2026-03-16

### Fixed

#### GLSL Backend
- **Bind group collision in GLSL output** — `@group(0) @binding(0)` and
  `@group(1) @binding(0)` both generated `layout(binding = 0)`, causing
  uniform shadowing on GLES. SDF viewport uniform was overwritten by clip
  uniform, making all SDF shapes invisible. Now flattens to unique GL
  binding points: `group * 16 + binding + base`.

## [0.14.7] - 2026-03-15

### Fixed

#### MSL Backend
- **Multi-group binding index collision** — `@group(0) @binding(0)` and `@group(1) @binding(0)` both mapped to Metal `[[buffer(0)]]`, causing shader compilation failure on macOS. The MSL backend now assigns sequential per-type indices (buffer, texture, sampler) across all bind groups sorted by `(group, binding)`, matching the Rust wgpu-hal approach. When `PerEntryPointMap` provides explicit mappings, those take priority. ([gogpu/gg#209](https://github.com/gogpu/gg/issues/209))

## [0.14.6] - 2026-03-06

### Fixed

#### MSL Backend
- **Pass-through globals for helper functions** — textures, samplers, uniforms, and storage buffers used by non-entry-point functions are now passed as extra parameters and arguments; previously MSL helper functions could not access entry point resource bindings, causing `undeclared identifier` errors (e.g., `msdf_atlas`, `msdf_sampler`, `sh_scratch`) for any shader with helper functions referencing global resources ([gogpu/ui#23](https://github.com/gogpu/ui/issues/23))

## [0.14.5] - 2026-03-04

### Fixed

#### MSL Backend
- **Buffer parameters use references (`&`) instead of pointers (`*`)** — buffer parameters now generate `constant Uniforms& u [[buffer(0)]]` (reference) instead of `constant Uniforms* u [[buffer(0)]]` (pointer); pointer syntax required `->` or `(*u).` for member access while the expression writer generates `.` access, causing Metal compilation errors on Apple Silicon ([gogpu/ui#23](https://github.com/gogpu/ui/issues/23))

## [0.14.4] - 2026-03-01

### Fixed

#### MSL Backend
- **Vertex `[[stage_in]]` for struct-typed arguments** — vertex shaders with struct-typed inputs (e.g., `fn vs_main(in: VertexInput)`) now correctly generate a synthesized `_Input` struct with `[[attribute(N)]]` members and `[[stage_in]]` parameter; previously only fragment stage was handled, causing undefined `in_` reference ([gogpu/ui#23](https://github.com/gogpu/ui/issues/23))
- **`metal::discard_fragment()` namespace** — `discard_fragment()` now emits with required `metal::` namespace prefix; bare call was rejected by Metal shader compiler

## [0.14.3] - 2026-02-25

### Fixed

#### SPIR-V Backend
- **Deferred store for multiple call results** — Variables initialized from expressions containing multiple function call results now correctly emit deferred `OpStore` instructions for each intermediate result
- **Deferred store for `var x = atomicOp()`** — Atomic operation results used in variable initialization now correctly generate deferred stores instead of losing the value (NAGA-SPV-006)
- **`OpLogicalEqual` for bool comparisons** — Boolean equality expressions now emit correct `OpLogicalEqual` opcode; transitive deferred stores propagate through boolean comparison chains
- **Atomic result type for `atomic<i32>` struct fields** — Atomic operations on signed integer struct members now use correct `OpTypeInt 32 1` result type instead of unsigned
- **Prologue var init splitting** — Variable initializations that reference other local variables are now split from the function prologue into `StmtStore` at the declaration point, preventing use-before-definition in SPIR-V (NAGA-SPV-007)

## [0.14.2] - 2026-02-22

### Added

#### Test Infrastructure
- **Golden snapshot test system** (`snapshot/`) — compiles 30 WGSL shaders through all 4 backends (SPIR-V, GLSL, HLSL, MSL), compares output to ~118 stored golden files; supports `UPDATE_GOLDEN=1` for regeneration
- **20 new reference shaders** — collatz, atomics, workgroup_memory, quad, vertex_colors, uniforms_mvp, multi_output, math_builtins, conversions, swizzle, expressions_complex, structs, arrays, matrices, let_and_var, loops_advanced, switch_advanced, texture_sample, texture_storage, pointers
- **WGSL error case tests** (`wgsl/wgsl_errors_test.go`) — 76 test cases covering parse errors (39) and lowering errors (37): unknown types, unresolved identifiers, missing tokens, wrong builtin argument counts, reserved words
- **IR validator semantic tests** (`ir/validate_semantic_test.go`) — 47 test functions covering type validation, constants/globals, entry points, functions, expressions, statements, and positive edge cases
- **SPIR-V capability tracking tests** (`spirv/capabilities_test.go`) — 13 test functions verifying correct OpCapability emission: Shader always present, Float16/64, Int8/16/64, ImageQuery, DotProduct, no-emit-when-unused, no duplicates
- **SPIR-V disassembler** for golden snapshots — extracted from `cmd/spvdis/` into reusable test helper, produces diff-friendly text output

### Fixed

#### SPIR-V Backend
- **Deterministic output** — replaced `map[int]uint32` with `[]uint32` slices for entry point interface variables and struct member extraction; Go map iteration order was causing non-reproducible SPIR-V binaries

#### GLSL Backend
- Emit `#extension GL_ARB_separate_shader_objects : enable` for desktop GLSL < 4.10 — `layout(location)` on inter-stage varyings requires this extension; NVIDIA drivers reject generated code without it ([#31](https://github.com/gogpu/naga/issues/31))

## [0.14.1] - 2026-02-21

### Fixed

#### HLSL Backend
- `row_major` qualifier for matrix struct members in cbuffer/uniform blocks — DX12 `M[i]` column access was returning rows instead of columns, causing transposed transforms and invisible geometry
- `mul(right, left)` argument reversal for `row_major` matrices — HLSL `mul()` semantics differ from WGSL `*` operator when layout changes
- Unique entry point names — prevent HLSL duplicate function errors when multiple entry points reference the same function
- Typed call results — function calls now use correct return type instead of void

#### GLSL Backend
- Clear `namedExpressions` between function compilations — expression handle names from one WGSL function were leaking into subsequent functions, causing `undeclared identifier` errors in GLES shaders

## [0.14.0] - 2026-02-21

Major WGSL language coverage expansion: 15/15 Essential reference shaders from Rust naga test suite now compile to valid SPIR-V.

### Added

#### WGSL Parser
- Abstract type constructors without template parameters (`vec3(1,2,3)`, `mat2x2(...)`, `array(...)`)
- `bitcast<T>(expr)` template syntax with dedicated AST node
- `binding_array<T, N>` type syntax
- Float literal suffixes without decimal point (`1f`, `1h`)
- Switch statement: `default` as case selector, trailing commas, optional colon
- Increment/decrement statements (`i++`, `i--`)

#### WGSL Lowerer
- 48 predeclared short type aliases (`vec3f`, `mat4x4f`, `vec2i`, etc.)
- Struct constructor syntax (`StructName(field1, field2)`)
- Pointer dereference on assignment LHS (`*ptr = value`)
- `_` discard identifier in assignments and let bindings
- `modf().fract`/`.whole` and `frexp().fract`/`.exp` member access on builtin results
- `bitcast` expression lowering
- Constant expression evaluator for switch case selectors
- `dot4I8Packed` / `dot4U8Packed` packed dot product builtins
- `textureGather`, `textureGatherCompare`, `textureSampleBaseClampToEdge`
- `texture_depth_2d_array` as non-parameterized type
- `textureSampleCompare` / `textureSampleCompareLevel`
- Global variable type inference from initializer
- `BindingArrayType` for descriptor array types

#### SPIR-V Backend
- `OpTranspose` (native SPIR-V opcode 84) with matrix type swap
- Matrix type caching (prevents duplicate OpTypeMatrix)
- 25 new math functions: bit manipulation (countOneBits, reverseBits, extractBits, insertBits, firstLeadingBit, firstTrailingBit, countLeadingZeros, countTrailingZeros), pack/unpack (4x8snorm, 4x8unorm, 2x16snorm, 2x16unorm, 2x16float), quantizeToF16
- `OpSDotKHR` / `OpUDotKHR` with SPV_KHR_integer_dot_product extension
- `OpImageGather` / `OpImageDrefGather` with component index
- `OpBitCount`, `OpBitReverse`, `OpBitFieldInsert`, `OpBitFieldSExtract`, `OpBitFieldUExtract`
- Pointer access chains on function arguments
- `findCallResultInTree` extended to 12+ expression types
- `BindingArrayType` emission (OpTypeArray/OpTypeRuntimeArray)
- Identity conversion early return

#### IR
- `BindingArrayType` struct for descriptor array types

#### Testing
- **17 reference shader regression tests** — 15 Essential + 2 bonus (skybox, water) from Rust naga test suite, embedded as string literals for CI compatibility
- SPIR-V validation via `spirv-val` in CI

### Fixed

#### WGSL Frontend
- Compound assignment (`+=`, `-=`) on local variables — removed explicit ExprLoad
- `textureDimensions` accepting 1 argument (texture only)
- `>>` token splitting for nested template closing (`ptr<function, vec3<f32>>`)
- Const with constructor expressions (`const light = vec3<f32>(1,2,3)`)
- Unary negation in constant expressions (`const X = -0.1`)
- `let` bindings emitted at declaration point for SSA dominance correctness
- Float literals without trailing digit (`1.` now parsed correctly)
- Module-level constants with constructor initializers
- Switch statement termination analysis for exhaustive matching
- Trailing semicolons after closing braces no longer cause parse errors
- Vector type inference from constructor arguments

#### SPIR-V Backend
- `OpIMul` result type for scalar*vector promotion (was using scalar type instead of vector)
- `MatrixStride` decoration for uniform matrix members
- `let` variable semantics — emit `OpLoad` for `let` bindings (value semantics, not reference)
- `OpCapability ImageQuery` emitted when using `textureDimensions`/`textureNumLevels`
- Matrix multiply (`OpMatrixTimesVector`, `OpVectorTimesMatrix`, `OpMatrixTimesMatrix`) type handling
- Deferred `OpStore` for variables initialized from complex expressions
- Vector/scalar type promotion for `add`, `subtract`, `modulo` binary operations
- `select()` builtin: float-to-bool condition conversion
- Arrayed texture coordinate handling (array index as separate component)
- `OpImageWrite` operand ordering for storage textures
- Sampled type derived from storage format for `OpTypeImage` (was defaulting to float)
- `atomicStore` / `atomicLoad` — correct SPIR-V opcode emission
- Workgroup variable layout decorations (Offset, ArrayStride)
- `OpDecorate Block` deduplication — no longer emits duplicate decorations
- Loop `continuing` block codegen — correct back-edge and merge block structure
- Uniform struct wrapping — storage/uniform buffer structs get correct member decorations
- Vector type conversion in composite constructors

## [0.13.1] - 2026-02-17

SPIR-V OpArrayLength fix, comprehensive benchmarks, and compiler allocation optimization (−32%).

### Fixed

- **SPIR-V `OpArrayLength`** — Runtime-sized array length queries (`arrayLength()`) now emit
  correct `OpArrayLength` instruction. Handles both bare storage arrays (wrapped in synthetic
  struct) and struct member arrays. Fixes "unsupported expression kind: ExprArrayLength" crash
  in compute shaders with dynamic buffer sizes.

### Added

- **Comprehensive compiler benchmarks** — 68 benchmarks across all 7 packages (root, wgsl, spirv,
  glsl, hlsl, msl, ir) with `ReportAllocs()` and `b.SetBytes()` throughput metrics. Covers
  full pipeline (lex→parse→lower→validate→generate), cross-backend comparison, and per-stage
  isolation. Table-driven by shader complexity (small/medium/large).

### Changed

- **Compiler allocation reduction (−32.3%)** — Large PBR shader: 1384→937 allocs, 203KB→134KB.
  Word arena for SPIR-V instructions (eliminates per-instruction `make()`), shared
  `InstructionBuilder` with `Reset()`, package-level lookup tables in lowerer (eliminates
  6 map allocations per compile including 66-entry `getMathFunction`), capacity hints in
  parser/lexer/backend. SPIR-V generate stage: −58.4% allocs. Lowerer bytes: −68.6%.

## [0.13.0] - 2026-02-15

GLSL backend improvements, HLSL struct entry point fix, and SPIR-V vector/scalar multiply and bool conversion fixes.

### Added

#### GLSL Backend
- **UBO blocks for struct uniforms** — Struct uniform variables now emit `layout(std140) uniform BlockName { ... }` blocks instead of bare uniform declarations
- **Entry point struct I/O** — Vertex/fragment entry points with struct parameters and return types now correctly emit `in`/`out` declarations for each struct member

#### Testing
- **SPIR-V loop iteration regression tests** — Tests verifying correct loop codegen
- **SPIR-V conditional call result regression tests** — Tests for function calls inside if/return blocks

### Fixed

#### GLSL Backend
- **Array syntax** — Array declarations now use correct GLSL syntax (`float name[3]` instead of `float[3] name`)
- **Built-in mappings** — WGSL builtins correctly mapped to GLSL equivalents (`gl_Position`, `gl_VertexID`, etc.)
- **Entry point generation** — Correct `void main()` generation with proper layout qualifiers
- **Local variable initializers** — Variables with initializers now emit correct GLSL initialization

#### HLSL Backend
- **Struct entry point arguments with member bindings** — Entry points accepting struct parameters with `@builtin`/`@location` member bindings now correctly generate HLSL input structs with proper semantics

#### SPIR-V Backend
- **`OpVectorTimesScalar`** — Vector-scalar multiplication now emits the dedicated `OpVectorTimesScalar` instruction instead of component-wise multiply
- **Opcode number corrections** — Fixed incorrect opcode values for vector/scalar operations
- **Bool-to-float conversion** — `f32(bool_expr)` now generates correct `OpSelect` instead of failing with "unsupported conversion"
- **Variable initialization from expressions** — Additional fixes for `var x = expr;` patterns
- **Math function argument handling** — Improved argument ordering for `smoothstep`, `clamp`, `select`, `abs`, `min`, `max`

### Changed
- Removed unused test helper `validateSPIRVBinaryBasic`

## [0.12.1] - 2026-02-13

Hotfix: wire up HLSL codegen (was causing DPC_WATCHDOG_VIOLATION BSOD on DX12), complete all 93 WGSL built-in functions.

### Added

#### WGSL Frontend
- **All 93 WGSL built-in functions** — Complete coverage of the W3C WGSL specification
  - 14 math functions: `modf`, `frexp`, `ldexp`, `inverse`, `quantizeToF16`, `outerProduct`, `pack4xI8`/`U8`, `pack4xI8Clamp`/`U8Clamp`, `unpack4xI8`/`U8`
  - 9 derivative functions: `dpdx`, `dpdy`, `fwidth` + `Coarse`/`Fine` variants
  - 4 relational functions: `all`, `any`, `isnan`, `isinf`
  - `arrayLength` for runtime-sized arrays

#### Testing
- **HLSL end-to-end golden tests** — 14 tests covering the full WGSL → HLSL pipeline
  - Triangle shader, vertex/fragment, compute, uniform buffers
  - Math functions, control flow (if/else, switch, loops), swizzle
  - Entry point deduplication, stub detection, semantic validation

### Fixed

#### HLSL Backend
- **Wire up codegen** — Connect implemented expression/statement/function codegen to the writer
  - Entry point functions now generate actual HLSL bodies instead of stub placeholders
  - Regular functions call `writeFunctionBody()` for complete code generation
  - Entry points use `writeEntryPointWithIO()` with proper I/O structs and semantics
  - Removed duplicate function emission (entry points were written twice)
- **Fragment output semantic** — Fragment shader `@location(N)` now maps to `SV_TargetN` (was `TEXCOORD0`)
- **Builtin extraction** — `@builtin(vertex_index)` correctly extracted from input struct to local variable
- **Array syntax** — HLSL arrays now use correct syntax:
  - Declarations: `float2 name[3]` instead of `float2[3] name`
  - Initializers: `{elem1, elem2}` instead of `type[3](elem1, elem2)`
- **Named expression ordering** — Expression names cached AFTER writing initializer (prevents `float x = x;`)

## [0.12.0] - 2026-02-10

SPIR-V function call support and compute shader codegen improvements for GPU SDF pipeline.

### Added

#### SPIR-V Backend
- **`OpFunctionCall`** — Function call support for non-entry-point functions
  - Emits `OpFunctionCall` with correct result type and argument passing
  - Enables modular WGSL shaders with helper functions

#### Testing
- **SPIR-V codegen analysis tests** for SDF compute shaders (~2000 LOC)
  - `sdf_analysis_test.go` — Validates SPIR-V output for SDF batch shader patterns
  - `var_ifelse_test.go` — Tests variable initialization and if/else codegen

### Fixed

#### SPIR-V Backend
- **Compute shader codegen** — Multiple fixes for real-world compute shader patterns
  - Fixed `var` initialization from expressions (was emitting zero instead of computed value)
  - Fixed hex literal suffix parsing (`0xFFu` now correctly parsed as `u32`)
  - Improved expression handling for complex compute shader workflows

#### WGSL Frontend
- **Hex literal suffixes** — `0xFFu` and `0xFFi` now correctly parsed with type suffix

## [0.11.1] - 2026-02-09

Critical SPIR-V opcode corrections and compute shader fixes. Fixes incorrect code generation for logical operators, comparisons, shifts, and local variable initializers — all discovered during GPU SDF compute shader development.

### Fixed

#### SPIR-V Backend
- **`OpLogicalAnd` opcode** — Was 164 (`OpLogicalEqual`), corrected to 167 per SPIR-V spec
  - WGSL `&&` compiled to boolean equality instead of logical AND
  - `false && false` incorrectly evaluated to `true`
  - Caused filled rectangles to render as outlines in compute shaders
- **Comparison opcodes swapped** — `OpFOrdGreaterThan` and `OpFOrdLessThanEqual` had each other's values
  - `>` behaved as `<=` and vice versa in float comparisons
- **Shift opcodes rotated** — `OpShiftLeftLogical`, `OpShiftRightLogical`, `OpShiftRightArithmetic` corrected
  - Bit shift operations produced wrong results (e.g., RGBA channel packing)
- **Local variable initializers** — `var x: f32 = 0.0` now emits `OpStore` for the initial value
  - `OpVariable` was emitted without initializer; `LocalVariable.Init` field was ignored by backend
  - Variables started with undefined values, causing conditional stores to not propagate
- **Entry point interface** — Only Input/Output variables listed per SPIR-V 1.3 spec
  - Uniform and StorageBuffer variables no longer incorrectly included
- **Void function termination** — Explicit `OpReturn` for functions without terminator
- **Boolean literal type** — Consistent type deduplication via `emitScalarType`
- **Runtime-sized arrays** — `OpTypeRuntimeArray` for storage buffer `array<T>` (was returning error)
- **Type conversions** — `f32(x)`, `u32(y)` now generate correct conversion opcodes instead of compose
- **Unsigned integer literals** — `1u` suffix correctly parsed as `u32` (was always `i32`)
- **Array stride** — Automatic `ArrayStride` decoration for runtime arrays (std430 layout)

#### WGSL Frontend
- **`f16` pre-registration removed** — No longer emits `OpCapability Float16` in shaders that don't use `f16`
  - Fixes Vulkan validation errors on devices without `shaderFloat16` support

### Added
- `OpLogicalEqual` (164), `OpLogicalNotEqual` (165) SPIR-V opcodes
- `OpConvertFToU`, `OpConvertFToS`, `OpConvertSToF`, `OpConvertUToF`, `OpBitcast` conversion opcodes
- `nagac` now targets SPIR-V 1.3 by default

## [0.11.0] - 2026-02-07

SPIR-V control flow fix and 55 new WGSL built-in math functions.

### Fixed

#### SPIR-V Backend
- **`if/else` control flow** — Fixed invalid SPIR-V causing GPU hang on all drivers
  - Root cause: `blockEndsWithTerminator()` didn't handle `StmtBlock` wrapper from WGSL lowerer
  - Reject branch wrapped in `StmtBlock{}` by `lowerStatement()` vs flat `lowerBlock()` for accept
  - Result: `OpReturn` emitted in reject block followed by spurious `OpBranch` (two terminators)
  - Merge block left without terminator — undefined behavior in structured control flow
  - Fix: Added `StmtBlock` and nested `StmtIf` handling to `blockEndsWithTerminator()`
  - Added `OpUnreachable` emission for merge blocks when both branches terminate
  - Fixed `AddLabel()` → `AddLabelWithID()` for correct merge block targeting

### Added

#### WGSL Built-in Functions (55 new, 67 total math functions)
- **Trigonometric**: `cosh`, `sinh`, `tanh`, `acos`, `asin`, `atan`, `atan2`, `asinh`, `acosh`, `atanh`
- **Angle conversion**: `radians`, `degrees`
- **Decomposition**: `ceil`, `floor`, `round`, `fract`, `trunc`
- **Exponential**: `exp`, `exp2`, `log`, `log2`, `pow`
- **Geometric**: `distance`, `faceForward`, `reflect`, `refract`
- **Computational**: `sign`, `fma`, `mix`, `step`, `smoothstep`, `inverseSqrt`, `saturate`
- **Matrix**: `transpose`, `determinant`
- **Bit manipulation**: `countTrailingZeros`, `countLeadingZeros`, `countOneBits`, `reverseBits`, `extractBits`, `insertBits`, `firstTrailingBit`, `firstLeadingBit`
- **Data packing**: `pack4x8snorm`, `pack4x8unorm`, `pack2x16snorm`, `pack2x16unorm`, `pack2x16float`
- **Data unpacking**: `unpack4x8snorm`, `unpack4x8unorm`, `unpack2x16snorm`, `unpack2x16unorm`, `unpack2x16float`
- **Selection**: `select(falseVal, trueVal, condition)` — Component-wise selection

#### Testing
- SPIR-V `if/else` control flow test — validates correct block termination
- 55 new math function compilation tests — all functions verified end-to-end
- `select()` function test with scalar and vector variants

## [0.10.0] - 2026-02-01

WGSL language features: local const, switch statements, and storage texture support.

### Added

#### WGSL Language Features
- **Local const declarations** — `const` inside function bodies with compile-time evaluation
- **Switch statements** — Full switch/case/default support with SPIR-V `OpSwitch` generation

#### Storage Texture Support
- **`ir.StorageFormat`** — 50+ texture formats (rgba8unorm, r32float, etc.)
- **`ir.StorageAccess`** — Access modes (read, write, read_write)
- **ImageType extension** — StorageFormat and StorageAccess fields for storage textures
- **WGSL parsing** — `texture_storage_2d<rgba8unorm, write>` syntax support
- **SPIR-V generation** — Correct `OpTypeImage` format decorations for storage images

#### SPIR-V Backend
- **ImageFormat constants** — All SPIR-V image format values
- **StorageFormatToImageFormat()** — IR to SPIR-V format conversion

### Changed
- Texture type parsing refactored to `parseTextureType()` with proper dimension/class detection
- Removed unused `textureDim()` function

## [0.9.0] - 2026-01-30

Sampler support, swizzle operations, and SPIR-V development tools.

### Added

#### Sampler Types
- `sampler` and `sampler_comparison` type support in WGSL lowerer
- Lazy sampler type registration (prevents spurious OpTypeSampler in shaders without textures)

#### Swizzle Support
- Full swizzle support via `OpVectorShuffle` in SPIR-V backend
- Handles all WGSL swizzle patterns (e.g., `.xyz`, `.rgba`, `.xxyy`)

#### Struct Member Bindings
- `Binding` field on `StructMember` for `@builtin`/`@location` on struct members
- `hasPositionBuiltin` validation for vertex shader struct returns

#### SPIR-V Development Tools
- `cmd/spvdis` — SPIR-V disassembler for debugging shader compilation (~480 LOC)
- `cmd/texture_compile` — Texture shader compile tool for testing (~95 LOC)

### Fixed

#### SPIR-V Backend
- **Block decoration** — Added `OpDecorate Block` for uniform/storage/push_constant struct types
  - Required by Vulkan VUID-StandaloneSpirv-Uniform-06676
- **Member offset ordering** — Fixed `emitTypes()` to run before struct member decorations
- **Pointer/value semantics** — Fixed entry point parameter handling for correct SPIR-V output

#### WGSL Lowerer
- **Uniform buffer alignment** — Proper alignment calculation for uniform buffer layout
- **Sampler registration** — Samplers now created on-demand instead of pre-registered

### Changed
- Extracted expression ref helpers to separate functions (fixes funlen linter)
- Removed unused code from SPIR-V backend

## [0.8.4] - 2026-01-10

Critical SPIR-V backend fix for Intel Vulkan driver compatibility.

### Fixed

#### SPIR-V Backend
- **Instruction ordering** — Fixed OpVariable declarations to appear before OpLoad instructions
  - SPIR-V spec requires all OpVariable at START of first block
  - Intel Iris Xe Graphics was rejecting shaders with incorrect ordering
  - Other drivers (NVIDIA, AMD) were more lenient but technically incorrect
- **Array access semantics** — Added OpLoad after OpAccessChain
  - OpAccessChain returns pointer, but consumers expect values
  - Fixed undefined behavior in array/struct member access

### Changed
- **Constant naming** — Renamed BuiltIn*Id constants to BuiltIn*ID (Go naming convention)
  - `BuiltInVertexId` → `BuiltInVertexID`
  - `BuiltInInstanceId` → `BuiltInInstanceID`
  - `BuiltInPrimitiveId` → `BuiltInPrimitiveID`
  - `BuiltInInvocationId` → `BuiltInInvocationID`
  - `BuiltInSampleId` → `BuiltInSampleID`
  - `BuiltInWorkgroupId` → `BuiltInWorkgroupID`
  - `BuiltInLocalInvocationId` → `BuiltInLocalInvocationID`
  - `BuiltInGlobalInvocationId` → `BuiltInGlobalInvocationID`

## [0.8.3] - 2026-01-04

Critical MSL backend fix for vertex shader position output.

### Fixed

#### MSL Backend
- **[[position]] attribute placement** — Fixed to emit on struct member instead of function signature
  - MSL requires `[[position]]` on struct member, not on function return type
  - Before: `vertex float4 vs_main(...) [[position]] { }` (invalid MSL)
  - After: `struct vs_main_Output { float4 member [[position]]; }; vertex vs_main_Output vs_main(...) { }` (valid MSL)
  - Matches behavior of original Rust naga implementation
- **Simple type output structs** — Now generates output struct for simple types with builtin bindings
- **Return statement handling** — Fixed return for simple type output structs

## [0.8.2] - 2026-01-04

MSL backend improvements for ARM64 macOS triangle rendering.

### Fixed

#### MSL Backend
- **Triangle shader compilation** — Fixed entry point output struct handling for vertex shaders
- **Return attribute handling** — Improved `@builtin(position)` and other return type attributes
- **Struct member emission** — Fixed struct field ordering and attribute placement

### Added
- **MSL backend tests** — Comprehensive test coverage for struct handling and entry points
- **xcrun integration tests** — Real Metal shader validation on macOS (skipped on other platforms)

### Changed
- Improved WGSL lowering for complex struct types
- Better error messages for unsupported shader features

### Contributors
- @ppoage — ARM64 macOS fixes and testing

## [0.8.1] - 2025-12-29

### Fixed

#### WGSL Lowering
- **clamp() built-in function** — Added missing `clamp` to math function map
  - Root cause: `getMathFunction()` was missing `clamp` → `ir.MathClamp` mapping
  - Caused "unknown function: clamp" error during shader compilation
  - Affected any WGSL shader using `clamp(value, min, max)`

### Added
- **Comprehensive math function tests** — `TestMathFunctions` covering all 12 WGSL built-in math functions
  - Tests: abs, min, max, clamp, sin, cos, tan, sqrt, length, normalize, dot, cross
  - Verifies correct IR generation for each function

## [0.8.0] - 2025-12-28

Code quality improvements and SPIR-V backend bug fixes.

### Fixed

#### SPIR-V Backend
- **sign() type checking** — Now correctly uses `SSign` for signed integers vs `FSign` for floats
- **atomicMin/Max signed vs unsigned** — Now correctly uses `OpAtomicSMin`/`OpAtomicSMax` for signed integers and `OpAtomicUMin`/`OpAtomicUMax` for unsigned

#### WGSL Frontend
- **Function resolution** — Added pre-registration pass for forward function references
- **Return type attributes** — Parser now correctly handles attributes on return types (e.g., `@builtin(position)`)

### Changed
- Removed dead `Write()` method from SPIR-V writer
- Removed unused `module` field from `spirv.Writer` struct
- Code cleanup in `hlsl/types.go` nolint directives

## [0.7.0] - 2025-12-28

HLSL backend for DirectX shader compilation (~8.8K new LOC).

### Added

#### HLSL Backend (DirectX)
- `hlsl/backend.go` — Public API: `Options`, `TranslationInfo`, `Compile()`
  - DXC-first strategy (Shader Model 6.0+)
  - FXC compatibility mode (Shader Model 5.1)
  - Vertex, fragment, and compute shader support
- `hlsl/writer.go` — HLSL code generation writer (~400 LOC)
- `hlsl/types.go` — Type generation (~500 LOC)
  - Scalars: float, half, double, int, uint, bool
  - Vectors: float2, float3, float4, int*, uint*
  - Matrices: float2x2, float3x3, float4x4
  - Structs with HLSL semantics
- `hlsl/expressions.go` — Expression code generation (~1100 LOC)
  - Literals, binary/unary operations
  - Access expressions (array, struct, swizzle)
  - 70+ HLSL intrinsic functions
  - Texture sampling: Sample, SampleLevel, SampleBias, SampleGrad, Gather
  - Derivatives: ddx, ddy, fwidth (coarse/fine variants)
- `hlsl/statements.go` — Statement code generation (~600 LOC)
  - Control flow (if, switch, loop, for)
  - GPU barriers (GroupMemoryBarrier, DeviceMemoryBarrier, AllMemoryBarrier)
  - Return, discard, break, continue
- `hlsl/storage.go` — Buffer and atomic operations (~500 LOC)
  - ByteAddressBuffer, RWByteAddressBuffer
  - StructuredBuffer<T>, RWStructuredBuffer<T>
  - cbuffer for uniforms
  - Atomics: InterlockedAdd, And, Or, Xor, Min, Max, Exchange, CompareExchange
- `hlsl/functions.go` — Entry point generation (~500 LOC)
  - Input/output structs with HLSL semantics (SV_Position, TEXCOORD, SV_Target)
  - `[numthreads(x,y,z)]` for compute shaders
  - Helper functions for safe math operations
- `hlsl/keywords.go` — HLSL reserved word escaping (200+ keywords)
- `hlsl/conv.go` — IR to HLSL type/semantic conversion
- `hlsl/namer.go` — Identifier mangling for HLSL compliance
- `hlsl/errors.go` — HLSL-specific error types
- `hlsl/shader_model.go` — Shader Model version handling
- `hlsl/bind_target.go` — Register binding management (b/t/s/u)

### Notes
- HLSL backend enables DirectX GPU rendering on Windows
- Supports DirectX 11 (SM 5.1) and DirectX 12 (SM 6.0+)
- Total: ~8800 lines of code

## [0.6.0] - 2025-12-25

GLSL backend for OpenGL shader compilation (~2.8K new LOC).

### Added

#### OpenGL Shading Language Backend
- `glsl/backend.go` — Public API: `Options`, `TranslationInfo`, `Compile()`
  - `GLSLVersion` configuration (GLSL 330, 400, 450, ES 300, ES 310)
  - Vertex, fragment, and compute shader support
- `glsl/writer.go` — GLSL code generation writer
- `glsl/types.go` — Type generation (~300 LOC)
  - Scalars: float, int, uint, bool
  - Vectors: vec2, vec3, vec4, ivec*, uvec*, bvec*
  - Matrices: mat2, mat3, mat4, mat2x3, etc.
  - Arrays with fixed size
  - Textures: sampler2D, sampler3D, samplerCube
- `glsl/expressions.go` — Expression code generation (~400 LOC)
  - Literals, binary/unary operations
  - Access expressions (array, struct, swizzle)
  - GLSL built-in function calls
- `glsl/statements.go` — Statement code generation (~300 LOC)
  - Variable declarations
  - Control flow (if, for, while, loop)
  - Assignments and function calls
- `glsl/functions.go` — Entry point generation (~400 LOC)
  - `void main()` with layout qualifiers
  - Vertex: `layout(location = N) in/out`
  - Fragment: `layout(location = N) out`
  - Compute: `layout(local_size_x/y/z)` workgroup size
- `glsl/keywords.go` — GLSL reserved word escaping (183 keywords)
- `glsl/backend_test.go` — Comprehensive unit tests (40+ tests)

### Changed
- README.md updated with GLSL backend documentation
- Architecture section now includes GLSL backend structure

### Notes
- GLSL backend enables OpenGL GPU rendering on all platforms
- Supports OpenGL 3.3+, OpenGL ES 3.0+
- Required by wgpu GLES backend for Linux/embedded platforms

## [0.5.0] - 2025-12-23

MSL backend for Metal shader compilation (~3.6K new LOC).

### Added

#### Metal Shading Language Backend
- `msl/backend.go` — Public API: `Options`, `TranslationInfo`, `Compile()`
- `msl/writer.go` — MSL code generation writer
- `msl/types.go` — Type generation (~400 LOC)
  - Scalars: float, half, int, uint, bool
  - Vectors: float2, float3, float4, etc.
  - Matrices: float2x2, float3x3, float4x4
  - Arrays with fixed size
  - Textures: texture2d, texture3d, texturecube
  - Samplers: sampler
- `msl/expressions.go` — Expression code generation (~600 LOC)
  - Literals, binary/unary operations
  - Access expressions (array, struct, swizzle)
  - Math function calls
- `msl/statements.go` — Statement code generation (~350 LOC)
  - Variable declarations
  - Control flow (if, for, while, loop)
  - Assignments and function calls
- `msl/functions.go` — Entry point generation (~500 LOC)
  - `[[vertex]]` for vertex shaders
  - `[[fragment]]` for fragment shaders
  - `[[kernel]]` for compute shaders
  - Stage input/output structs
- `msl/keywords.go` — MSL/C++ reserved word escaping
- `msl/backend_test.go` — Unit tests for MSL compilation

### Changed
- Pre-release check script now uses kolkov/racedetector (Pure Go, no CGO)
- Updated ecosystem: gogpu v0.5.0 (macOS Cocoa), wgpu v0.6.0 (Metal backend)

### Notes
- MSL backend enables Metal GPU rendering on macOS/iOS
- Required by wgpu v0.6.0 Metal backend

## [0.4.0] - 2025-12-12

Compute shader support with atomics, barriers, and developer experience improvements (~2K new LOC).

### Added

#### Compute Shader Infrastructure
- `wgsl/parser.go` — Access mode parsing for storage buffers
  - `var<storage, read>` — Read-only storage buffer
  - `var<storage, read_write>` — Read-write storage buffer
  - `var<workgroup>` — Workgroup shared memory
- `wgsl/lower.go` — Workgroup size extraction from `@workgroup_size` attribute
- `ir/ir.go` — `AtomicType` for `atomic<u32>` and `atomic<i32>`

#### Atomic Operations
- `wgsl/lower.go` — Atomic function lowering (~150 LOC)
  - `atomicAdd(&ptr, value)` — Atomic addition
  - `atomicSub(&ptr, value)` — Atomic subtraction
  - `atomicMin(&ptr, value)` — Atomic minimum
  - `atomicMax(&ptr, value)` — Atomic maximum
  - `atomicAnd(&ptr, value)` — Atomic bitwise AND
  - `atomicOr(&ptr, value)` — Atomic bitwise OR
  - `atomicXor(&ptr, value)` — Atomic bitwise XOR
  - `atomicExchange(&ptr, value)` — Atomic exchange
  - `atomicCompareExchangeWeak(&ptr, cmp, val)` — Compare and exchange
- `spirv/backend.go` — SPIR-V atomic emission (~100 LOC)
  - `OpAtomicIAdd`, `OpAtomicISub`, `OpAtomicAnd`, `OpAtomicOr`, `OpAtomicXor`
  - `OpAtomicUMin`, `OpAtomicUMax`, `OpAtomicExchange`, `OpAtomicCompareExch`
- `ir/expression.go` — `ExprAtomicResult` for atomic operation results

#### Workgroup Barriers
- `wgsl/lower.go` — Barrier function lowering
  - `workgroupBarrier()` — Synchronize workgroup threads
  - `storageBarrier()` — Memory barrier for storage buffers
  - `textureBarrier()` — Memory barrier for textures
- `spirv/backend.go` — `OpControlBarrier` emission with memory semantics

#### Address-of and Dereference Operators
- `wgsl/lower.go` — `&` and `*` operator handling
  - `&var` — Returns pointer (no-op for storage variables)
  - `*ptr` — Creates `ExprLoad` for dereferencing

#### Unused Variable Warnings
- `wgsl/lower.go` — Warning infrastructure (~50 LOC)
  - `Warning` type with message and source span
  - `LowerResult` struct containing module and warnings
  - `LowerWithWarnings()` API for accessing warnings
  - Variables prefixed with `_` are intentionally ignored
  - `checkUnusedVariables()` called after each function

#### Better Error Messages
- `wgsl/errors.go` — `SourceError` type with source location
- `wgsl/errors.go` — `FormatWithContext()` for pretty error display
- `wgsl/lower.go` — `LowerWithSource()` preserves source for errors

### Changed
- `spirv/spirv.go` — Added SPIR-V opcodes for atomics and barriers
- Total: 203 tests across all packages (+79 from v0.3.0)

### Fixed
- Type switch in `emitAtomic` now uses assignment form (gocritic fix)

## [0.3.0] - 2025-12-11

`let` type inference, array initialization, and texture sampling (~3K new LOC).

### Added

#### Type Inference for `let` Bindings
- `wgsl/lower.go` — `inferTypeFromExpression()` method (~80 LOC)
  - Supports inferring type from any expression
  - `let x = 1.0` → inferred f32
  - `let v = vec3(1.0)` → inferred vec3<f32>
  - `let n = normalize(v)` → inferred from function return type
- `wgsl/lower_type_inference_test.go` — 6 new tests

#### Array Initialization Syntax
- `wgsl/lower.go` — Array constructor handling (~50 LOC)
  - `array(1, 2, 3)` shorthand with inferred type and size
  - `array<f32, 3>(...)` explicit syntax
  - Element type inferred from first element
- Tests for array shorthand and vector arrays

#### Texture Sampling Operations
- `wgsl/lower.go` — Texture function lowering (~250 LOC)
  - `textureSample(t, s, coord)` — Basic sampling
  - `textureSampleBias(t, s, coord, bias)` — With LOD bias
  - `textureSampleLevel(t, s, coord, level)` — Specific mip level
  - `textureSampleGrad(t, s, coord, ddx, ddy)` — With derivatives
  - `textureLoad(t, coord, level)` — Direct texel load
  - `textureStore(t, coord, value)` — Write to storage texture
  - `textureDimensions(t)` — Get texture size
  - `textureNumLevels(t)` — Get mip count
  - `textureNumLayers(t)` — Get array layer count
- `spirv/backend.go` — SPIR-V image operations (~200 LOC)
  - `OpSampledImage` — Combine texture and sampler
  - `OpImageSampleImplicitLod` — textureSample
  - `OpImageSampleExplicitLod` — textureSampleLevel
  - `OpImageFetch` — textureLoad
  - `OpImageWrite` — textureStore
  - `OpImageQuerySize*` — textureDimensions
  - `OpImageQueryLevels` — textureNumLevels
  - Helper methods: `getSampledImageType()`, `emitVec4F32Type()`

### Changed
- `wgsl/lower.go` — `lowerLocalVar()` supports optional type with inference
- `wgsl/lower.go` — `isBuiltinConstructor()` includes "array"
- `wgsl/lower.go` — `lowerBuiltinConstructor()` handles array shorthand
- `naga_test.go` — Enabled `TestCompileWithMathFunctions` (was skipped)
- Total: 124 tests across all packages

### Fixed
- Array size now correctly uses pointer (`*uint32`) per IR definition
- SPIR-V OpImageFetch uses coordinate without sampler

## [0.2.0] - 2025-12-11

Type inference and SPIR-V backend improvements (~2K new LOC).

### Added

#### Type Inference System
- `ir/resolve.go` — Complete type inference engine (~500 LOC)
  - Resolves types for all 25+ expression kinds
  - Handles literals, constants, composites, binary/unary ops
  - Supports nested types (vectors, matrices, arrays, structs)
  - `TypeResolution` struct for dual handle/inline representation
- `ir/resolve_test.go` — 8 comprehensive unit tests

#### Type Deduplication
- `ir/registry.go` — Type registry for SPIR-V compliance (~100 LOC)
  - Ensures each unique type appears exactly once
  - Normalized type keys for structural equality
  - Supports all IR type kinds
- `ir/registry_test.go` — 18 unit tests

#### SPIR-V Backend Improvements
- Proper type resolution instead of placeholders
- Correct int/float/uint opcode selection:
  - `IAdd/ISub/IMul` vs `FAdd/FSub/FMul`
  - `SDiv/UDiv/FDiv` for signed/unsigned/float
  - `IEqual/SLessThan` vs `FOrdEqual/FOrdLessThan`
- `emitInlineType()` for temporary types
- Range-based iteration to avoid large struct copies

#### Testing
- `spirv/shader_test.go` — 10 end-to-end shader compilation tests
- `wgsl/lower_type_inference_test.go` — 3 integration tests
- `wgsl/deduplication_test.go` — Type deduplication tests
- Total: 67+ tests across all packages

### Changed
- `ir/ir.go` — Added `TypeResolution` struct and `ExpressionTypes` to `Function`
- `wgsl/lower.go` — Integrated type registry and expression type tracking
- `spirv/backend.go` — Uses real types from inference system (~350 lines changed)
- `ir/validate.go` — Range-based iteration for performance

### Fixed
- SPIR-V binary output now has correct type IDs for all expressions
- Comparison operators correctly return `bool` or `vec<bool>`
- Math functions select correct int vs float GLSL.std.450 instructions

## [0.1.0] - 2025-12-10

First stable release. Complete WGSL to SPIR-V compilation pipeline (~10K LOC).

### Added

#### Intermediate Representation (IR)
- `ir/expression.go` — 33 expression types (~520 LOC)
  - Literals (f32, f64, i32, u32, bool)
  - Binary/Unary operators (17 binary, 3 unary)
  - Access expressions (array, struct, swizzle)
  - Math functions (60+ supported)
  - Texture operations (sample, load, query)
- `ir/statement.go` — 16 statement types (~320 LOC)
  - Control flow (if, loop, switch, break, continue)
  - Memory operations (store, atomic)
  - Function calls
- `ir/validate.go` — Comprehensive IR validation (~750 LOC)
  - Type validation
  - Expression validation
  - Statement validation
  - Entry point validation

#### AST to IR Lowering
- `wgsl/lower.go` — AST → IR converter (~1050 LOC)
  - Type resolution (scalars, vectors, matrices, arrays, structs)
  - Built-in type recognition
  - Binding resolution (@builtin, @location, @group/@binding)
  - Expression lowering
  - Statement lowering

#### SPIR-V Backend
- `spirv/writer.go` — Binary module builder (~670 LOC)
  - SPIR-V header generation
  - Instruction encoding
  - String encoding with padding
- `spirv/backend.go` — IR → SPIR-V translator (~1500 LOC)
  - Type emission (all IR types)
  - Constant emission (scalars, composites)
  - Function emission
  - Expression emission (33 expression types)
  - Control flow (if, loop, break, continue)
  - 40+ built-in math functions via GLSL.std.450
  - Derivative functions (dpdx, dpdy, fwidth)
- `spirv/spirv.go` — SPIR-V constants and opcodes
  - 100+ opcodes
  - 81 GLSL.std.450 extended instructions

#### Public API
- `naga.go` — Public API (~160 LOC)
  - `Compile(source)` — One-function compilation
  - `CompileWithOptions(source, opts)` — Custom options
  - `Parse()`, `Lower()`, `Validate()`, `GenerateSPIRV()` — Individual stages

#### CLI Tool
- `cmd/nagac/main.go` — Command-line compiler
  - `-o` output file
  - `-debug` include debug names
  - `-validate` enable validation
  - `-version` show version

#### Tests
- `naga_test.go` — 7 integration tests
- `ir/validate_test.go` — 12 validation tests
- `spirv/backend_test.go` — Backend tests
- `spirv/writer_test.go` — Writer tests
- `wgsl/lower_test.go` — Lowering tests

### Changed
- Updated `.golangci.yml` with exclusions for compiler complexity
- Expanded `spirv/spirv.go` with full opcode set

---

[Unreleased]: https://github.com/gogpu/naga/compare/v0.15.0...HEAD
[0.15.0]: https://github.com/gogpu/naga/compare/v0.14.8...v0.15.0
[0.14.8]: https://github.com/gogpu/naga/compare/v0.14.7...v0.14.8
[0.14.7]: https://github.com/gogpu/naga/compare/v0.14.6...v0.14.7
[0.14.6]: https://github.com/gogpu/naga/compare/v0.14.5...v0.14.6
[0.14.5]: https://github.com/gogpu/naga/compare/v0.14.4...v0.14.5
[0.14.4]: https://github.com/gogpu/naga/compare/v0.14.3...v0.14.4
[0.14.3]: https://github.com/gogpu/naga/compare/v0.14.2...v0.14.3
[0.14.2]: https://github.com/gogpu/naga/compare/v0.14.1...v0.14.2
[0.14.1]: https://github.com/gogpu/naga/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/gogpu/naga/compare/v0.13.1...v0.14.0
[0.13.1]: https://github.com/gogpu/naga/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/gogpu/naga/compare/v0.12.1...v0.13.0
[0.12.1]: https://github.com/gogpu/naga/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/gogpu/naga/compare/v0.11.1...v0.12.0
[0.11.1]: https://github.com/gogpu/naga/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/gogpu/naga/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/gogpu/naga/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/gogpu/naga/compare/v0.8.4...v0.9.0
[0.8.4]: https://github.com/gogpu/naga/compare/v0.8.3...v0.8.4
[0.8.3]: https://github.com/gogpu/naga/compare/v0.8.2...v0.8.3
[0.8.2]: https://github.com/gogpu/naga/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/gogpu/naga/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/gogpu/naga/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/gogpu/naga/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/gogpu/naga/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/gogpu/naga/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/gogpu/naga/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/gogpu/naga/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/gogpu/naga/releases/tag/v0.2.0
[0.1.0]: https://github.com/gogpu/naga/releases/tag/v0.1.0
