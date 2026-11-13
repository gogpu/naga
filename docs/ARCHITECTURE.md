# naga Architecture

This document describes the architecture of the naga shader compiler.

## Overview

naga is a shader compiler written entirely in Go. It compiles WGSL (WebGPU Shading Language)
to multiple backend formats (SPIR-V, MSL, GLSL, HLSL, DXIL) without requiring CGO or external
dependencies.

**Core principle: one IR, five backends.**

```
                     ┌──────────────────┐
                     │ WGSL Source Code │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │      Lexer       │  wgsl/internal/parser/lexer.go
                     │  (120+ tokens)   │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │      Parser      │  wgsl/internal/parser/parser.go
                     │   (recursive     │
                     │     descent)     │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │       AST        │  wgsl/internal/parser/ast.go
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │     Lowerer      │  wgsl/internal/lower/
                     │    (AST → IR)    │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │    IR Module     │  ir/ir.go
                     │  (SSA form,      │
                     │   deduplicated   │
                     │   types)         │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │    Validator     │  valid/validate.go
                     └────────┬─────────┘
                              │
           ┌──────────┬───────┴───────┬──────────┬─────────┐
           │          │               │          │         │
    ┌──────▼──────┐ ┌─▼───┐      ┌────▼───┐ ┌────▼───┐ ┌───▼────┐
    │   SPIR-V    │ │ MSL │      │  GLSL  │ │  HLSL  │ │  DXIL  │
    │  (binary)   │ │     │      │        │ │  (text)│ │(binary)│
    └──────┬──────┘ └──┬──┘      └────┬───┘ └────┬───┘ └───┬────┘
           │           │              │          │         │
        Vulkan      Metal          OpenGL    DX11/12    DX12 (SM6)
```

## Package Structure

```
naga/
├── naga.go, overrides.go, compact.go  # Public API: Compile, CompileWithOptions, ProcessOverrides, Compact
├── frontend/                      # Types the input frontends share: Diagnostic, Label, the combined image
│                                  # sampler vocabulary (policy, SamplerBindings, CombinedSampler)
│
├── wgsl/                          # WGSL frontend
│   ├── wgsl.go                    # Public API: Parse, ParseOptions, ParseInfo, ParseError
│   └── internal/
│       ├── parser/                # Lexer, token, parser, AST, enable directives, errors
│       └── lower/                 # AST → IR lowerer (enable-extension gates)
│
├── ir/                            # The IR data model (Rust naga::ir); imports no other naga package
│   ├── ir.go                      # Module, Type, Function, EntryPoint, handles
│   ├── expression.go              # expression kinds (incl. subgroup, ray query)
│   ├── statement.go               # statement kinds (incl. subgroup, ray query)
│   └── layouter.go                # Layouter: sizes and alignments (Rust proc::Layouter)
│
├── valid/                         # Validation (Rust naga::valid): Validate, Options, ModuleInfo, Capabilities, MissingCapabilities
│
├── backend/                       # Types every backend shares: bounds-check policies, entry point selector,
│                                  # shader model, zero-init mode, errors (ErrUnsupported, ErrMissingBinding)
│
├── internal/                      # Shared internal packages
│   ├── typifier/                  # The one expression typifier (Rust proc::Typifier) for validator, backends, frontend
│   ├── textutil/                  # IndentWriter (DRY for glsl/hlsl/msl)
│   ├── d3d/                       # Shared namer, sig packing, interface ordering (hlsl, dxil)
│   ├── irutil/                    # Compaction, inlining (return lowering), type reordering, handle walkers,
│   │                              # one-exit helpers for FXC (SingleExitSwitchReturns)
│   ├── ssa/                       # The expression kinds the DXIL backend adds (Alias, Phi); not part of the IR
│   ├── overrides/                 # Pipeline-constant processing (Rust back::pipeline_constants)
│   ├── entrypoint/                # Entry point selection shared by the backends
│   ├── registry/                  # Type deduplication registry
│   ├── consteval/                 # Constant evaluator shared by the WGSL frontend
│   ├── unsupported/               # Errors that wrap backend.ErrUnsupported
│   ├── dxcvalidator/              # IDxcValidator through dxil.dll (Windows, tests and cmd/dxilval)
│   └── readback/, testcorpus/, testtools/  # GPU readback corpus reader, test-data provenance, external-tool policy
│
├── spirv/                         # SPIR-V backend and input
│   ├── spirv.go                   # Public API: Compile, Options, Version, Capability, TranslationInfo,
│   │                              # SupportedCapabilities; Parse, ParseOptions, ParseInfo, ParseError
│   ├── internal/codegen/          # Backend implementation
│   └── internal/front/            # SPIR-V input frontend: types, constants, decorations, globals, control
│                                  # flow, calls, images and sampling, atomics, subgroups, combined image
│                                  # samplers, entry-point wrappers; limits in limits.go
│
│
├── msl/                           # MSL backend
│   ├── msl.go                     # Public API: Compile, Options, TranslationInfo, SupportedCapabilities
│   └── internal/codegen/          # ALL implementation
│
├── glsl/                          # GLSL backend
│   ├── glsl.go                    # Public API: Compile, Options, TranslationInfo, SupportedCapabilities
│   └── internal/codegen/          # ALL implementation
│
├── hlsl/                          # HLSL backend
│   ├── hlsl.go                    # Public API: Compile, Options, TranslationInfo, SupportedCapabilities
│   └── internal/codegen/          # ALL implementation
│
├── dxil/                          # DXIL backend (experimental; IDxcValidator and DXC parity measured by snapshot tests)
│   ├── dxil.go                    # Public API: Compile, Options, Validate, SupportedCapabilities
│   ├── sig_usage.go               # Input signature Used mask analysis
│   ├── privates.go, workgroup_*.go  # Module rewrites before emission: privates as locals, workgroup scalarization
│   ├── validate.go                # IDxcValidator integration
│   └── internal/                  # ALL implementation internal
│       ├── bitcode/               # LLVM 3.7 bit-level writer (VBR, blocks, records)
│       ├── module/                # In-memory DXIL module + bitcode serialization
│       ├── container/             # DXBC container (DXIL, ISG1, OSG1, PSG1, PSV0, SFI0, HASH)
│       ├── emit/                  # naga IR → DXIL lowering (VS/PS/CS/MS, CBV/SRV/UAV, atomics, ray query, wave ops)
│       ├── passes/dce/            # Dead code elimination (liveness through control dependence; dead loops removed)
│       ├── passes/sroa/           # Scalar Replacement of Aggregates
│       ├── passes/mem2reg/        # Memory to register promotion (SSA construction)
│       └── viewid/                # ViewID dependency analysis
│
└── cmd/
    ├── nagac/                     # CLI compiler
    ├── spvdis/                    # SPIR-V disassembler
    ├── dxilval/                   # DXIL validation through dxil.dll (Windows)
    └── texture_compile/           # Texture shader testing tool
```

## Compilation Pipeline

The stages below are the WGSL path. The SPIR-V input path (`spirv.Parse`) replaces stages 1-3: it reads the
binary sections and builds the same IR directly, including Rust naga's entry-point wrapper (inputs copied from
arguments, outputs loaded and returned, built-in outputs defaulted).

### Stage 1: Lexer

**File:** `wgsl/internal/parser/lexer.go`

Converts WGSL source into a stream of tokens. Handles:
- 120+ token types (keywords, operators, type names, literals)
- Float literal suffixes without decimal point (`1f`, `1h`)
- Nested block comments (`/* /* ... */ */`)
- Hex (`0xFF`), binary, and octal literals with suffixes (`u`, `i`)

### Stage 2: Parser

**File:** `wgsl/internal/parser/parser.go`

Recursive descent parser that builds an AST. Key features:
- Abstract type constructors without template params (`vec3(1,2,3)`)
- 48 short type aliases (`vec3f` = `vec3<f32>`, `mat4x4f` = `mat4x4<f32>`)
- Type aliases (`alias FVec3 = vec3<f32>;`)
- Override declarations (`@id(N) override name: type = default;`)
- `const_assert` declarations
- `break if` syntax in continuing blocks
- `bitcast<T>(expr)` template syntax
- `binding_array<T, N>` descriptor array type
- Template list edge cases (trailing commas, `>=` disambiguation)
- Switch with `default` as case selector, trailing commas
- `>>` token splitting for nested template closing (`vec3<vec3<f32>>`)
- `enable` directives: an unknown extension name is an error, and directives must precede every declaration (WGSL
  grammar, Rust's `DirectiveAfterFirstGlobalDecl`)

### Stage 3: Lowerer

**Files:** `wgsl/internal/lower/`

Converts AST into typed IR. This is the largest and most complex frontend stage:
- **Type resolution** via `TypeRegistry` (deduplication by structural equality)
- **Symbol resolution** with two-level scoping (module + function)
- **SSA expression building** with `ExpressionHandle` references
- **Struct constructors** (`MyStruct(field1, field2)` → `ExprCompose`)
- **Constant evaluation** through `internal/consteval` (Rust naga's `try_eval_and_append`): every expression is appended through the evaluator — module `const`, `override` and `var<private>` initializers in a module const context, function bodies in runtime behavior — and WGSL automatic conversions follow Rust's `conversion.rs`
- **Math function mapping** (100+ WGSL builtins → IR `MathFunction` enum)
- **Texture operation lowering** (sample, load, store, gather, dimensions)
- **Pointer dereference** on assignment LHS (`*ptr = value`)
- **Type inference** for `let` bindings and global variables
- **Unused variable detection** with `_` prefix exception
- **Enable-extension gates** at Rust naga's sites (types, built-ins, attributes, address spaces, literals); by the
  WGSL spec, the subgroup built-ins also need `enable subgroups;`

### Stage 4: Validation

**File:** `valid/validate.go` (public: `valid.Validate`)

Validates the IR module as Rust naga's `valid` module does, with Rust's error texts:
- Type consistency (scalar widths, vector sizes, matrix dimensions) and handle validity
- Expression typing (one typifier pass) and scope: an expression is used only after its `Emit`
- Operand types of unary and binary operators and of `select` (Rust's `InvalidUnaryOperandType`,
  `InvalidBinaryOperandTypes`, `SelectConditionNotABool`)
- Control flow as Rust's `ControlFlowAbility` (break/continue/return)
- Capability gates (`valid.Options.Capabilities`, subgroup stages and operations); a zero option is no optional
  capability
- Stage restrictions through the call tree (`ForbiddenStageOperations`): `discard` in fragment shaders, barriers in
  compute shaders, implicit-LOD sampling and derivatives in fragment shaders
- `var<immediate>`: no arrays, at most one per entry point
- Binding uniqueness per entry point (no two resources one entry point uses at one `@group`/`@binding`, as Rust)
- Entry point requirements (vertex needs `@builtin(position)`, interpolation of varyings)

The result is a `valid.ModuleInfo`, which `naga.ProcessOverrides` takes and returns, as Rust's
`process_overrides`.

### Stage 5: Backend Code Generation

Five backends share the same IR but produce different outputs:

| Backend | Output | Target | Key Feature |
|---------|--------|--------|-------------|
| **SPIR-V** | Binary (little-endian words) | Vulkan | Word arena, capability tracking, SPIR-V 1.0–1.6 |
| **MSL** | Text (C++ dialect) | Metal | Bounds check policies, MSL 1.0–3.1 |
| **GLSL** | Text | OpenGL 3.3+, ES 3.0+ | Version targeting, UBO blocks |
| **HLSL** | Text | DirectX 11/12 | Shader model 5.0–6.7, semantics; one-exit helpers for FXC below 6.0 |
| **DXIL** | Binary (LLVM 3.7 bitcode) | DirectX 12 (SM 6.0–6.9) | IDxcValidator over the Rust naga corpus and DXC roundtrip parity (snapshot tests). DCE, SROA (aggregate locals split before the emitter), mem2reg optimization passes; unreachable blocks dropped. Experimental. |

Every backend is one function, `Compile(*ir.Module, Options) (output, TranslationInfo, error)`; the options it shares
with the others come from the `backend` package, with safe zero values (bounds checks `Restrict`, workgroup memory
zeroed, loops bounded). A target version the module needs but the caller did not ask for is refused with a
`*backend.VersionError` (`errors.Is(err, backend.ErrUnsupported)`), never raised behind the caller's back.

## Intermediate Representation

### Module Structure

```go
type Module struct {
    Types             // All type definitions (deduplicated)
    SpecialTypes      // Types the IR itself needs (ray intersection, external texture parameters)
    Constants         // Module-scope constants; Init indexes GlobalExpressions
    Overrides         // Pipeline-overridable constants
    GlobalVariables   // Uniform, storage, workgroup, private, immediate variables
    GlobalExpressions // Constant and override expressions at module scope
    Functions         // Function definitions (expression arena + statement tree)
    EntryPoints       // Shader entry points (vertex/fragment/compute/task/mesh)
}
```

### Handle System

IR objects are referenced by typed handles (uint32 indices) for type safety and cache locality:

```go
type TypeHandle           uint32  // Index into Module.Types
type FunctionHandle       uint32  // Index into Module.Functions
type GlobalVariableHandle uint32  // Index into Module.GlobalVariables
type ConstantHandle       uint32  // Index into Module.Constants
type ExpressionHandle     uint32  // Index into Function.Expressions
```

Handles prevent mixing (can't pass `FunctionHandle` where `TypeHandle` expected)
while having zero runtime overhead.

### Type System

All types implement the `TypeInner` interface (marker pattern):

| Type | WGSL | Fields |
|------|------|--------|
| **ScalarType** | `f16`, `f32`, `f64`, `i32`, `u32`, `i64`, `u64`, `bool` | Kind, Width |
| **VectorType** | `vec2<T>` ... `vec4<T>` | Size, Scalar |
| **MatrixType** | `mat2x2<f32>` ... `mat4x4<f32>` | Columns, Rows, Scalar |
| **ArrayType** | `array<T, N>`, `array<T>` | Base, Size, Stride |
| **StructType** | `struct { ... }` | Members[], Span |
| **PointerType** | `ptr<space, T>` | Base, AddressSpace |
| **SamplerType** | `sampler`, `sampler_comparison` | Comparison |
| **ImageType** | `texture_2d<f32>`, etc. | Dimension, Arrayed, Class |
| **AtomicType** | `atomic<u32>`, `atomic<i32>` | Scalar |
| **BindingArrayType** | `binding_array<T, N>` | Base, Size |
| **AccelerationStructureType** | `acceleration_structure` | (opaque) |
| **RayQueryType** | `ray_query` | (opaque) |

### Expression Kinds (SSA Form)

Expressions are stored in a flat pool (`Function.Expressions[]`) and referenced by handle.
Each expression is evaluated once (SSA — Static Single Assignment):

```
Literal                   — scalar constants (f16/f32/f64, i32/u32/i64/u64, bool, abstract)
ExprConstant, ExprOverride — Reference to a module constant / pipeline override
ExprZeroValue             — Zero-initialized value of a type
ExprCompose, ExprSplat    — Construct a composite / broadcast a scalar to a vector
ExprAccess                — Dynamic index access (array[i])
ExprAccessIndex           — Constant index access (array[2], struct members, .x)
ExprSwizzle               — Vector swizzle (.xyz, .rgba)
ExprFunctionArgument      — A function's argument
ExprGlobalVariable, ExprLocalVariable — Pointer to a variable
ExprLoad                  — Dereference a pointer
ExprImageSample           — textureSample* and textureGather* (level, gather, depth reference)
ExprImageLoad, ExprImageQuery — textureLoad / textureDimensions, textureNumLevels, …
ExprUnary                 — !, -, ~
ExprBinary                — + - * / %, comparisons, & | ^, && ||, << >>
ExprSelect                — select(false, true, cond)
ExprDerivative            — dpdx, dpdy, fwidth (and coarse/fine)
ExprRelational            — all, any, isnan, isinf
ExprMath                  — 100+ built-in functions (abs, sin, dot, …)
ExprAs                    — Conversion or bitcast
ExprCallResult            — Result of a StmtCall
ExprArrayLength           — arrayLength() of a runtime-sized array
ExprAtomicResult, ExprWorkGroupUniformLoadResult — Results of the statements that produce them
ExprRayQueryProceedResult, ExprRayQueryGetIntersection — Ray query results
ExprSubgroupBallotResult, ExprSubgroupOperationResult — Subgroup operation results
```

### Statement Kinds

```
StmtEmit             — Marks expression range as evaluated (SSA milestone)
StmtBlock            — Sequential statement group
StmtIf               — Conditional (accept/reject blocks)
StmtSwitch           — Multi-way branch with case selectors
StmtLoop             — Loop with body, continuing block, break-if
StmtBreak            — Exit loop or switch
StmtContinue         — Jump to continuing block
StmtReturn           — Return from function (optional value)
StmtKill             — Fragment discard
StmtControlBarrier   — workgroupBarrier, storageBarrier, textureBarrier, subgroupBarrier
StmtMemoryBarrier    — Memory barrier without execution synchronization
StmtStore            — Write to pointer (*ptr = value)
StmtImageStore       — Write to storage texture
StmtAtomic           — Atomic operation on a pointer
StmtImageAtomic      — Atomic operation on a storage texel
StmtWorkGroupUniformLoad — workgroupUniformLoad
StmtCall             — Function call (the result, if any, is an ExprCallResult)
StmtRayQuery         — Ray query initialize/proceed/terminate
StmtSubgroupBallot, StmtSubgroupCollectiveOperation, StmtSubgroupGather — Subgroup operations
```

### Address Spaces

| Space | WGSL | Semantics |
|-------|------|-----------|
| Function | `var x: T` | Local variable, per-invocation |
| Private | `var<private>` | Thread-local, module scope |
| WorkGroup | `var<workgroup>` | Shared within workgroup |
| Uniform | `var<uniform>` | Read-only uniform buffer |
| Storage | `var<storage>` | Read/write storage buffer |
| Handle | `var t: texture_2d<f32>` (no address space) | Sampler/texture handle |
| Immediate | `var<immediate>` | Immediate data (Vulkan push constants) |
| TaskPayload | `var<task_payload>` | Mesh shading task payload |

## SPIR-V Backend

**Files:** `spirv/internal/codegen/backend.go`, `spirv/internal/codegen/writer.go`, `spirv/internal/codegen/block.go`, `spirv/internal/codegen/ray_query.go`

The SPIR-V backend produces binary bytecode for Vulkan (195 of the 201 snapshot shaders
compile and all of them pass spirv-val; see Testing Strategy). It uses a **Block Ownership Model** (matching
Rust naga) where each basic block is a first-class `Block` struct consumed by
`FunctionBuilder`. This prevents instruction interleaving bugs and provides structural
guarantees for control flow correctness.

### SPIR-V Module Layout

```
Header (5 words)
├── Magic: 0x07230203
├── Version: 0x00010300 (SPIR-V 1.3)
├── Generator ID
├── Bound (max ID + 1)
└── Schema (reserved)

Sections (strict ordering per spec):
├── OpCapability         — Required capabilities (Shader, Float64, etc.)
├── OpExtension          — SPV_KHR_integer_dot_product, etc.
├── OpExtInstImport      — GLSL.std.450 for math functions
├── OpMemoryModel        — Vulkan / GLSL450
├── OpEntryPoint         — Entry point declarations
├── OpExecutionMode      — OriginUpperLeft, LocalSize, etc.
├── OpName / OpMemberName— Debug names (optional)
├── OpDecorate           — Bindings, locations, Block, offsets
├── OpType* / OpConstant*— Type and constant definitions
├── OpVariable           — Global variables (uniform, storage)
└── OpFunction...End     — Function bodies
```

### Key Implementation Details

- **Word Arena:** Pre-allocated `[]uint32` buffer reduces GC pressure for instruction building
- **Type Caching:** `scalarTypeIDs`, `vectorTypeIDs`, `matrixTypeIDs`, `pointerTypeIDs`
  maps prevent duplicate type emissions
- **Capability Tracking:** `usedCapabilities` set, emitted only when referenced
- **Extension Tracking:** `usedExtensions` for `SPV_KHR_integer_dot_product`, etc.
- **Entry Point Interface:** Separate `OpVariable` for each Input/Output binding (SPIR-V 1.4+ globals with ForcePointSize)
- **Storage Buffer Wrapping:** Bare storage arrays wrapped in `Block`-decorated struct
- **GLSL.std.450:** 100+ math functions via extended instruction set
- **Integer Safety:** `naga_div`/`naga_mod` wrapper functions prevent division by zero and i32 MIN/-1 overflow
- **Image Bounds Checking:** Restrict and ReadZeroSkipWrite policies with coordinate clamping
- **Ray Query Helpers:** 6 helper functions per ray query (initialize, proceed, terminate, intersection getters)
- **Loop Bounding:** Iteration counter prevents infinite loops (on by default; `SkipLoopBounding` opts out)
- **Workgroup Zero-Init:** Polyfill for zero-initializing workgroup memory (the default `ZeroInitWorkgroupPolyfill`)
- **Layout-free locals:** From SPIR-V 1.5, Function and Private variables use types without explicit layout, copied
  to and from the decorated types with `OpCopyLogical` (Vulkan's VUID-StandaloneSpirv-None-10684)
- **NonUniform Decorations:** Correct NonUniform propagation for binding array access
- **Capability-Aware Emission:** Polyfills for missing capabilities (e.g., dot4 without DotProduct)
- **f16 I/O Polyfill:** Bitcast-based conversion for f16 entry point interface variables
- **Composite Spilling:** By-value dynamic indexing spills composites to local variables
- **Pointer Function Args:** Copy-in/copy-out spill pattern for OpAccessChain results passed to OpFunctionCall (SPIR-V requires memory object declarations)
- **Integer Dot Product:** Manual expansion (extract + IMul + IAdd) when OpDot is not available for integer vectors
- **Matrix Add/Sub:** Column-wise decomposition (SPIR-V FAdd/FSub don't support matrix types)
- **Integer Vec×Scalar:** Scalar splatting before OpIMul (SPIR-V requires matching types)
- **Mesh Shaders:** MeshEXT/TaskEXT execution models with SPV_EXT_mesh_shader extension
- **Pipeline Overrides:** resolved before the backend by `naga.ProcessOverrides`, as in Rust naga; a module that still uses overrides is rejected with `ir.ErrUnresolvedOverride`
- **Image Atomics:** OpImageTexelPointer + standard atomic ops on texel pointers
- **Pack/Unpack 4×8-bit:** Shift+mask polyfill for pack4xI8/U8 and unpack4xI8/U8

## Text Backends (MSL, GLSL, HLSL)

All three text backends share one shape: the public package holds `Compile(*ir.Module, Options) (string,
TranslationInfo, error)` and converts its `Options` to the writer's, and `internal/codegen` holds a writer that types
the module once (`internal/typifier`), names everything with Rust naga's namer rules, and writes in phases:

```
1. header        — version directives, language features
2. types         — struct and array definitions
3. globals       — global variables, uniforms, bindings
4. functions     — helper function bodies (arguments reused by an expansion are baked into temporaries)
5. entry points  — entry point wrappers with their I/O structs
```

### MSL (Metal)

- Entry point qualifiers: `vertex`, `fragment`, `kernel`
- Buffer bindings: `[[buffer(N)]]`, `[[texture(N)]]`, `[[sampler(N)]]`
- Workgroup memory: function-scope `threadgroup` declarations (no host `setThreadgroupMemoryLength` needed), per-entry-point usage filtering
- Bounds check policies: Unchecked, ReadZeroSkipWrite, Restrict
- Vertex pulling transform with buffer size structs
- External texture support (multi-plane YUV sampling)
- Overrides arrive resolved by `naga.ProcessOverrides` (the backend rejects a module that still uses them)

### GLSL (OpenGL)

- Version targeting: `#version 330`, `#version 430`, `#version 300 es` (any GL 3.3–4.6, ES 3.0–3.2)
- Attribute binding: `layout(location=N) in/out`
- Uniform blocks: `layout(std140, binding=N) uniform BlockName { ... }`
- Storage blocks: `layout(std430, binding=N) buffer BlockName { ... }`
- Compute: `layout(local_size_x=X, ...) in`
- BindingMap: remaps (group, binding) to flat GL binding indices (`group*16 + binding`)
- Combined texture-sampler: separate WGSL `texture_2d` + `sampler` merged into GLSL `sampler2D` at texture's binding
- TextureMappings in TranslationInfo: maps each combined sampler2D to its texture/sampler bindings (used by GLES HAL for SamplerBindMap)
- **Version-aware `layout(binding=N)` emission**: `SupportsExplicitLocations()` gates binding qualifiers on GLSL ≥ 420 (desktop) or ≥ 310 (ES), matching Rust naga `mod.rs:213`. On older GL, bindings are omitted and must be assigned at runtime by the HAL
- **UniformInfo reflection**: writer collects `UniformInfo` structs (block name, binding, storage flag) during code generation, exposed via `TranslationInfo.Uniforms` for post-link `glGetUniformBlockIndex`/`glUniformBlockBinding` fallback
- Dead code elimination via `dominates_global_use` reachability
- Overrides arrive resolved by `naga.ProcessOverrides` (the backend rejects a module that still uses them)
- Image bounds checking

### HLSL (DirectX)

- Shader model: 5.0–5.1 (FXC, DX11/12) and 6.0–6.7 (DXC); the zero value is 5.1, as Rust naga. A feature the chosen
  model lacks is a `*backend.VersionError`
- Semantics: `SV_Position`, `SV_DispatchThreadID`, `SV_Target0`
- Constant buffers: `cbuffer` with register `b0`, `b1`, ...
- Storage buffers: `ByteAddressBuffer` / `RWByteAddressBuffer` with typed loads and stores, as Rust naga
- Samplers through a sampler heap and an index buffer (wgpu-hal's binding model)
- Compute: `[numthreads(X,Y,Z)]`
- FXC (below 6.0): a helper that returns from inside a `switch` inside a loop gets one exit
  (`irutil.SingleExitSwitchReturns`), since FXC turns such a return into the case's `break`

## Type Deduplication

The `TypeRegistry` ensures each structurally unique type is stored exactly once:

```go
type TypeRegistry struct {
    types   []Type
    typeMap map[string]TypeHandle  // Normalized key → handle
    keyBuf  []byte                 // Reusable buffer (zero-alloc)
}
```

Normalization rules:
- Scalar: `(kind<<8)|width`
- Vector: `"vec:" + size + ":" + scalarKey`
- Matrix: `"mat:" + cols + ":" + rows + ":" + scalarKey`
- Struct: includes member names, types, and offsets
- Array: includes base type and size

This is critical for SPIR-V compliance (each type must appear exactly once)
and eliminates redundant type definitions across all backends.

## Testing Strategy

| Category | Count | Approach |
|----------|-------|----------|
| **SPIR-V Validation** | 196/201 | spirv-val on every snapshot shader, also in the Vulkan environment of each SPIR-V version; the 5 others do not compile yet (cooperative matrix ×2, debugPrintf, a mesh shader, ray-tracing pipelines) |
| **Rust Reference** | 0 unexplained diffs | Exact output match with Rust naga: SPIR-V 102, MSL 63, GLSL 94, HLSL 62 shaders; every other difference is recorded with its reason |
| **Validator corpus** | every shader | The Rust and own corpora, gg's production shaders, gfx-rs's ray query examples and the readback corpus validate; invalid inputs are rejected with Rust's texts |
| **Determinism** | every shader × backend | Each backend compiles every corpus and gg shader several times to the same bytes |
| **DXIL** | corpus + gg | IDxcValidator, DXC roundtrip parity (`dxil-dxc` goldens) |
| **GPU readback** | corpus in `testdata/readback` | WGSL cases with expected values, run by gogpu/wgpu on Vulkan, DX12 (FXC and DXIL), GLES and software |
| **Unit and package tests** | ~4,500 test functions | Every frontend, validator and backend package; a fix ships with a test that failed before it |
| **Golden tests** | corpus × backend | Our own output snapshots (`snapshot/testdata/golden`), regenerated only after a reviewed change |
| **Benchmarks** | 46 | Throughput and allocations per package |

### Rust Naga Reference Shaders

**The 172 WGSL shaders** of the [Rust naga](https://github.com/gfx-rs/wgpu/tree/trunk/naga) test
suite (vendored from gfx-rs/wgpu 14328f49, see UPSTREAM) are included as snapshot tests. Coverage includes:

| Category | Shaders |
|----------|---------|
| Basic language | empty, constructors, operators, control-flow, functions |
| Types | abstract-types (9 variants), f16, f64, int64, type-alias |
| Resources | texture, image, shadow, skybox, boids, sprite |
| Compute | atomicOps, workgroup, barriers, subgroup-operations |
| Ray tracing | ray-query (4 variants), acceleration structures |
| Advanced | overrides, binding-arrays, bounds-check (7 variants), mesh-shader (4 variants) |

`TestRustReference` compares our output with Rust's reference output for each
shader and backend: SPIR-V 101 exact matches and 25 recorded deviations, MSL 63
and 47, GLSL 92 and 11, HLSL 60 and 28. A difference that is not recorded fails
the test. Rust's `subgroup-operations` input is fed with the `enable subgroups;`
the WGSL spec requires, which Rust's frontend cannot parse yet.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Arena-based IR** | `Function.Expressions` (and `Module.GlobalExpressions`) are flat pools; handles are uint32 indices. Cache-friendly, single allocation, minimal pointers. |
| **IR is data** | `ir` imports no other naga package; typing (`internal/typifier`) and validation (`valid`) live outside it, as in Rust naga. A test pins the import layering. |
| **WGSL spec first** | For what WGSL accepts and rejects, the W3C WGSL spec decides; Rust naga decides IR shape and error texts. Every deviation from Rust has a test naming Rust's behavior. |
| **SSA expressions** | Each expression evaluated once; `StmtEmit` marks availability. Enables single-pass code generation without dependency graphs. |
| **Typed handles** | `TypeHandle`, `FunctionHandle`, etc. are distinct `uint32` types. Compile-time type safety at zero runtime cost. |
| **Type deduplication** | `TypeRegistry` with normalized keys. Required by SPIR-V spec; also benefits text backends. |
| **Marker interfaces** | `TypeInner`, `ExpressionKind`, `Binding` use empty marker methods. Enables exhaustive `switch` with type assertions. |
| **Word arena (SPIR-V)** | Pre-allocated `[]uint32` for instruction encoding. Reduces GC pressure by ~32% (measured in benchmarks). |
| **Shared `InstructionBuilder`** | Single reusable builder with `Reset()`. Eliminates per-instruction `make()` calls. |

## Ecosystem Integration

```
naga (this project)
  │
  ├──► wgpu (Pure Go WebGPU) ──► Born ML (machine learning on the GPU)
  │      │
  │      ├──► gogpu (GPU framework, windowing)
  │      ├──► g3d (3D rendering)
  │      └──► gg (2D graphics library; also uses naga directly)
  │             │
  │             └──► ui (GUI toolkit)
```

**Release order:** naga → wgpu → gogpu + gg + g3d → ui. Before a naga release changes the IR or a backend, the
consumers build and test against the candidate, including the GPU readback corpus.

naga has **no dependencies** outside the Go standard library. It is the foundation
of the GoGPU ecosystem — all GPU rendering ultimately depends on naga for shader compilation.

## See Also

- [README.md](../README.md) — Quick start, features, installation
- [CHANGELOG.md](../CHANGELOG.md) — Version history
- [ROADMAP.md](../ROADMAP.md) — Development milestones
