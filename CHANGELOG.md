# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[Unreleased]: https://github.com/gogpu/naga/compare/v0.13.1...HEAD
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
