# naga Architecture

This document describes the architecture of the naga shader compiler.

## Overview

naga is a shader compiler written entirely in Go. It compiles WGSL (WebGPU Shading Language)
to multiple backend formats (SPIR-V, MSL, GLSL, HLSL) without requiring CGO or external
dependencies.

**Core principle: one IR, four backends.**

```
                     ┌──────────────────┐
                     │ WGSL Source Code │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │      Lexer       │  wgsl/lexer.go
                     │  (120+ tokens)   │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │      Parser      │  wgsl/parser.go
                     │   (recursive     │
                     │     descent)     │
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │       AST        │  wgsl/ast.go
                     └────────┬─────────┘
                              │
                     ┌────────▼─────────┐
                     │     Lowerer      │  wgsl/lower.go
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
                     │    Validator     │  ir/validate.go
                     └────────┬─────────┘
                              │
           ┌──────────┬───────┴───────┬──────────┐
           │          │               │          │
    ┌──────▼──────┐ ┌─▼───┐      ┌────▼───┐ ┌────▼───┐
    │   SPIR-V    │ │ MSL │      │  GLSL  │ │  HLSL  │
    │  (binary)   │ │     │      │        │ │        │
    └──────┬──────┘ └──┬──┘      └────┬───┘ └────┬───┘
           │           │              │          │
        Vulkan      Metal          OpenGL     DirectX
```

## Package Structure

```
naga/                              ~53K LOC total
├── naga.go                        # Public API: Compile, Parse, Lower, Validate, GenerateSPIRV
├── wgsl/                          # WGSL frontend (~10K LOC)
│   ├── token.go                   # 120+ token types
│   ├── lexer.go                   # Tokenizer (UTF-8, nested block comments)
│   ├── ast.go                     # AST types (declarations, statements, expressions)
│   ├── parser.go                  # Recursive descent parser (~1400 LOC)
│   ├── lower.go                   # AST → IR lowerer (~2500 LOC)
│   └── errors.go                  # Source-located error formatting
│
├── ir/                            # Intermediate Representation (~4.3K LOC)
│   ├── ir.go                      # Module, Type, Function, EntryPoint, handles
│   ├── expression.go              # 24 expression kinds (~520 LOC)
│   ├── statement.go               # 16 statement kinds (~320 LOC)
│   ├── validate.go                # IR validation (~750 LOC)
│   ├── resolve.go                 # Type inference engine (~500 LOC)
│   └── registry.go                # Type deduplication registry (~100 LOC)
│
├── spirv/                         # SPIR-V backend (~17K LOC incl. tests)
│   ├── spirv.go                   # SPIR-V constants, opcodes, capabilities
│   ├── writer.go                  # Binary module builder with word arena (~670 LOC)
│   ├── backend.go                 # IR → SPIR-V translator (~3700 LOC)
│   └── reference_shaders_test.go  # 17 Essential reference shader tests
│
├── msl/                           # MSL backend (~4.5K LOC)
│   ├── backend.go                 # Public API, Options, Compile()
│   ├── writer.go                  # MSL code writer
│   ├── types.go                   # Type generation (~400 LOC)
│   ├── expressions.go             # Expression codegen (~1175 LOC)
│   ├── statements.go              # Statement codegen (~350 LOC)
│   ├── functions.go               # Entry points and functions (~500 LOC)
│   └── keywords.go                # MSL/C++ reserved words
│
├── glsl/                          # GLSL backend (~5.8K LOC)
│   ├── backend.go                 # Public API, version targeting
│   ├── writer.go                  # GLSL code writer with UBO block syntax
│   ├── types.go                   # Type generation
│   ├── expressions.go             # Expression codegen
│   ├── statements.go              # Statement codegen
│   └── keywords.go                # Reserved word escaping
│
├── hlsl/                          # HLSL backend (~9.6K LOC)
│   ├── backend.go                 # Public API, shader model selection
│   ├── writer.go                  # HLSL code writer (~400 LOC)
│   ├── types.go                   # Type generation (~500 LOC)
│   ├── expressions.go             # Expression codegen (~1100 LOC)
│   ├── statements.go              # Statement codegen (~600 LOC)
│   ├── storage.go                 # Buffer/atomic operations (~500 LOC)
│   ├── functions.go               # Entry points with semantics (~500 LOC)
│   └── keywords.go                # HLSL reserved words
│
└── cmd/
    ├── nagac/                     # CLI compiler
    ├── spvdis/                    # SPIR-V disassembler
    └── texture_compile/           # Texture shader testing tool
```

## Compilation Pipeline

### Stage 1: Lexer

**File:** `wgsl/lexer.go` (~460 LOC)

Converts WGSL source into a stream of tokens. Handles:
- 120+ token types (keywords, operators, type names, literals)
- Float literal suffixes without decimal point (`1f`, `1h`)
- Nested block comments (`/* /* ... */ */`)
- Hex (`0xFF`), binary, and octal literals with suffixes (`u`, `i`)

### Stage 2: Parser

**File:** `wgsl/parser.go` (~1400 LOC)

Recursive descent parser that builds an AST. Key features:
- Abstract type constructors without template params (`vec3(1,2,3)`)
- 48 short type aliases (`vec3f` = `vec3<f32>`, `mat4x4f` = `mat4x4<f32>`)
- `bitcast<T>(expr)` template syntax
- `binding_array<T, N>` descriptor array type
- Switch with `default` as case selector, trailing commas
- `>>` token splitting for nested template closing (`vec3<vec3<f32>>`)

### Stage 3: Lowerer

**File:** `wgsl/lower.go` (~2500 LOC)

Converts AST into typed IR. This is the largest and most complex frontend stage:
- **Type resolution** via `TypeRegistry` (deduplication by structural equality)
- **Symbol resolution** with two-level scoping (module + function)
- **SSA expression building** with `ExpressionHandle` references
- **Struct constructors** (`MyStruct(field1, field2)` → `ExprCompose`)
- **Constant expression evaluator** for switch case selectors
- **Math function mapping** (100+ WGSL builtins → IR `MathFunction` enum)
- **Texture operation lowering** (sample, load, store, gather, dimensions)
- **Pointer dereference** on assignment LHS (`*ptr = value`)
- **Type inference** for `let` bindings and global variables
- **Unused variable detection** with `_` prefix exception

### Stage 4: Validation

**File:** `ir/validate.go` (~750 LOC)

Validates the IR module for correctness:
- Type consistency (scalar widths, vector sizes, matrix dimensions)
- Handle validity (all references point to existing objects)
- Control flow (break/continue only in loops, return in functions)
- Binding uniqueness (no duplicate `@group`/`@binding` pairs)
- Entry point requirements (vertex needs `@builtin(position)`)

### Stage 5: Backend Code Generation

Four backends share the same IR but produce different outputs:

| Backend | Output | Target | Key Feature |
|---------|--------|--------|-------------|
| **SPIR-V** | Binary (little-endian words) | Vulkan | Word arena, capability tracking |
| **MSL** | Text (C++ dialect) | Metal | Bounds check policies |
| **GLSL** | Text | OpenGL 3.3+, ES 3.0+ | Version targeting, UBO blocks |
| **HLSL** | Text | DirectX 11/12 | Shader model selection, semantics |

## Intermediate Representation

### Module Structure

```go
type Module struct {
    Types[]            // All type definitions (deduplicated)
    Constants[]        // Module-scope constants
    GlobalVariables[]  // Uniform, storage, workgroup variables
    Functions[]        // Function definitions (SSA expressions)
    EntryPoints[]      // Shader entry points (vertex/fragment/compute)
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
| **ScalarType** | `f32`, `i32`, `u32`, `bool`, `f16` | Kind, Width |
| **VectorType** | `vec2<T>` ... `vec4<T>` | Size, Scalar |
| **MatrixType** | `mat2x2<f32>` ... `mat4x4<f32>` | Columns, Rows, Scalar |
| **ArrayType** | `array<T, N>`, `array<T>` | Base, Size, Stride |
| **StructType** | `struct { ... }` | Members[], Span |
| **PointerType** | `ptr<space, T>` | Base, AddressSpace |
| **SamplerType** | `sampler`, `sampler_comparison` | Comparison |
| **ImageType** | `texture_2d<f32>`, etc. | Dimension, Arrayed, Class |
| **AtomicType** | `atomic<u32>`, `atomic<i32>` | Scalar |
| **BindingArrayType** | `binding_array<T, N>` | Base, Size |

### Expression Kinds (SSA Form)

Expressions are stored in a flat pool (`Function.Expressions[]`) and referenced by handle.
Each expression is evaluated once (SSA — Static Single Assignment):

```
Literal              — f32, i32, u32, bool constants
ExprConstant         — Reference to module constant
ExprZeroValue        — Zero-initialized value of given type
ExprCompose          — Construct composite (struct/vector/array)
ExprAccess           — Dynamic index access (array[i])
ExprAccessIndex      — Compile-time index access (array[2])
ExprMember           — Struct member access (.x, .field)
ExprSwizzle          — Vector swizzle (.xyz, .rgba)
ExprCast             — Type conversion (f32(x), u32(y))
ExprBitcast          — Bit reinterpretation (bitcast<T>)
ExprUnaryOp          — !, -, ~
ExprBinaryOp         — +, -, *, /, %, &, |, ^, ==, !=, <, >
ExprLogicalOp        — &&, ||
ExprLoad             — Dereference pointer
ExprFunctionCall     — Call function with arguments
ExprImageSample      — textureSample, textureSampleLevel, etc.
ExprImageLoad        — textureLoad
ExprImageQuery       — textureDimensions, textureNumLevels
ExprImageGather      — textureGather, textureGatherCompare
ExprArrayLength      — arrayLength() for runtime-sized arrays
ExprSelect           — Ternary select(false, true, cond)
ExprMath             — 100+ built-in functions (abs, sin, dot, etc.)
ExprAtomicResult     — Result of atomic operation
ExprAs               — Conversion with cast kind (bitcast, etc.)
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
StmtBarrier          — Workgroup/storage/texture barrier
StmtStore            — Write to pointer (*ptr = value)
StmtImageStore       — Write to storage texture
StmtAtomic           — Atomic read-modify-write operation
StmtCall             — Function call as statement (no return value used)
```

### Address Spaces

| Space | WGSL | Semantics |
|-------|------|-----------|
| Function | `var x: T` | Local variable, per-invocation |
| Private | `var<private>` | Thread-local, module scope |
| WorkGroup | `var<workgroup>` | Shared within workgroup |
| Uniform | `var<uniform>` | Read-only uniform buffer |
| Storage | `var<storage>` | Read/write storage buffer |
| PushConstant | `var<push_constant>` | Fast push constant data |
| Handle | (internal) | Sampler/texture handle |

## SPIR-V Backend

**Files:** `spirv/backend.go` (~3700 LOC), `spirv/writer.go` (~670 LOC)

The SPIR-V backend produces binary bytecode for Vulkan. It is the most complex backend
because SPIR-V has strict ordering and deduplication requirements.

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
- **Entry Point Interface:** Separate `OpVariable` for each Input/Output binding
- **Storage Buffer Wrapping:** Bare storage arrays wrapped in `Block`-decorated struct
- **GLSL.std.450:** 100+ math functions via extended instruction set

## Text Backends (MSL, GLSL, HLSL)

All three text backends share a common pattern:

```go
type Backend struct {
    module  *ir.Module
    options Options
    writer  *Writer      // strings.Builder wrapper
}

// Generation phases:
// 1. writeHeader()     — Version directives, language features
// 2. writeTypes()      — Type definitions (struct, array, etc.)
// 3. writeGlobals()    — Global variables, uniforms, bindings
// 4. writeFunctions()  — Helper function bodies
// 5. writeEntryPoints()— Entry point wrappers with I/O structs
```

### MSL (Metal)

- Entry point qualifiers: `vertex`, `fragment`, `kernel`
- Buffer bindings: `[[buffer(N)]]`, `[[texture(N)]]`, `[[sampler(N)]]`
- Workgroup memory: `threadgroup` address space
- Bounds check policies: Unchecked, ReadZeroSkipWrite, Restrict

### GLSL (OpenGL)

- Version targeting: `#version 330`, `#version 450`, `#version 300 es`
- Attribute binding: `layout(location=N) in/out`
- Uniform blocks: `layout(std140) uniform BlockName { ... }`
- Storage blocks: `layout(std430) buffer BlockName { ... }`
- Compute: `layout(local_size_x=X, ...) in`

### HLSL (DirectX)

- Shader model: SM 5.0 (DX11), SM 6.0+ (DX12)
- Semantics: `SV_Position`, `SV_DispatchThreadID`, `SV_Target0`
- Constant buffers: `cbuffer` with register `b0`, `b1`, ...
- Structured buffers: `RWStructuredBuffer<T>` for storage
- Compute: `[numthreads(X,Y,Z)]`

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
| **Reference Shaders** | 17 | 15 Essential + 2 bonus from Rust naga test suite |
| **Unit Tests** | 28+ | Pointer access, math functions, texture gather |
| **Backend Tests** | 40+ | GLSL, HLSL, MSL golden output comparison |
| **Integration Tests** | 7 | Full pipeline: WGSL source → SPIR-V binary |
| **Benchmarks** | 68 | All packages, throughput metrics, allocation tracking |

### Rust Naga Reference Shaders

15 Essential WGSL shaders from the [Rust naga](https://github.com/gfx-rs/naga) test suite
are embedded as string literals in `spirv/reference_shaders_test.go`. These cover:

| Shader | Coverage |
|--------|----------|
| empty, constructors, operators | Basic language features |
| control-flow, functions | Branching, loops, function calls |
| globals, interface | Module-scope variables, entry point I/O |
| texture, image-query | Texture sampling, dimensions, gather |
| variables, pointers | Address spaces, pointer dereference |
| shadow, terrain, water | Real-world rendering patterns |

All 15 shaders compile to valid SPIR-V (magic number `0x07230203` verified).

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Arena-based IR** | `Module.Expressions[]` is a flat pool; handles are uint32 indices. Cache-friendly, single allocation, minimal pointers. |
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
  └──► wgpu (Pure Go WebGPU)
         │
         ├──► gogpu (GPU framework, windowing)
         │
         └──► gg (2D graphics library)
                │
                └──► ui (GUI toolkit)
```

**Release order:** naga → wgpu → gogpu + gg → ui

naga has **no dependencies** outside the Go standard library. It is the foundation
of the GoGPU ecosystem — all GPU rendering ultimately depends on naga for shader compilation.

## See Also

- [README.md](../README.md) — Quick start, features, installation
- [CHANGELOG.md](../CHANGELOG.md) — Version history
- [ROADMAP.md](../ROADMAP.md) — Development milestones
