# SPIR-V Binary Writer

This package provides a low-level SPIR-V binary writer for constructing SPIR-V modules programmatically.

## Status

✅ **Implemented** - Binary writer foundation complete
🔄 **Next Step** - IR to SPIR-V translation

## Features

### ModuleBuilder

The main API for building SPIR-V modules:

```go
builder := spirv.NewModuleBuilder(spirv.Version1_3)

// Required setup
builder.AddCapability(spirv.CapabilityShader)
builder.SetMemoryModel(spirv.AddressingModelLogical, spirv.MemoryModelGLSL450)

// Types
floatType := builder.AddTypeFloat(32)
vec4Type := builder.AddTypeVector(floatType, 4)

// Entry points
funcID := builder.AddFunction(...)
builder.AddEntryPoint(spirv.ExecutionModelFragment, funcID, "main", nil)

// Generate binary
binary := builder.Build()
```

### Supported Operations

#### Type Operations
- `AddTypeVoid()` - Void type
- `AddTypeBool()` - Boolean type
- `AddTypeFloat(width)` - Float types (16, 32, 64-bit)
- `AddTypeInt(width, signed)` - Integer types
- `AddTypeVector(componentType, count)` - Vector types (vec2, vec3, vec4)
- `AddTypeMatrix(columnType, columnCount)` - Matrix types
- `AddTypeArray(elementType, length)` - Array types
- `AddTypeStruct(memberTypes...)` - Struct types
- `AddTypePointer(storageClass, baseType)` - Pointer types
- `AddTypeFunction(returnType, paramTypes...)` - Function types

#### Constant Operations
- `AddConstant(typeID, values...)` - Integer/boolean constants
- `AddConstantFloat32(typeID, value)` - 32-bit float constant
- `AddConstantFloat64(typeID, value)` - 64-bit float constant
- `AddConstantComposite(typeID, constituents...)` - Composite constants

#### Variable Operations
- `AddVariable(pointerType, storageClass)` - Global variable
- `AddVariableWithInit(pointerType, storageClass, initID)` - Variable with initializer

#### Function Operations
- `AddFunction(funcType, returnType, control)` - Function definition
- `AddFunctionParameter(typeID)` - Function parameter
- `AddLabel()` - Basic block label
- `AddReturn()` - Return from void function
- `AddReturnValue(valueID)` - Return with value
- `AddFunctionEnd()` - End of function

#### Entry Point & Execution Mode
- `AddEntryPoint(execModel, funcID, name, interfaces)` - Shader entry point
- `AddExecutionMode(entryPoint, mode, params...)` - Execution configuration

#### Debug Operations
- `AddName(id, name)` - Debug name for ID
- `AddMemberName(structID, member, name)` - Debug name for struct member
- `AddString(text)` - Debug string

#### Decoration Operations
- `AddDecorate(id, decoration, params...)` - Decorate type/variable
- `AddMemberDecorate(structID, member, decoration, params...)` - Decorate struct member

### Constants

#### Execution Models
- `ExecutionModelVertex` - Vertex shader
- `ExecutionModelFragment` - Fragment shader
- `ExecutionModelGLCompute` - Compute shader

#### Storage Classes
- `StorageClassUniformConstant` - Uniform/constant storage
- `StorageClassInput` - Shader input
- `StorageClassOutput` - Shader output
- `StorageClassUniform` - Uniform buffer
- `StorageClassWorkgroup` - Shared/workgroup memory
- `StorageClassPrivate` - Private storage
- `StorageClassFunction` - Function local storage
- `StorageClassPushConstant` - Push constants
- `StorageClassStorageBuffer` - Storage buffer

#### Decorations
- `DecorationLocation` - Location decoration
- `DecorationBinding` - Binding decoration
- `DecorationDescriptorSet` - Descriptor set decoration
- `DecorationBuiltIn` - Built-in decoration
- `DecorationBlock` - Block decoration
- `DecorationOffset` - Struct member offset

## SPIR-V Binary Format

### Header (5 words)
```
Word 0: 0x07230203 (Magic number)
Word 1: Version (major.minor << 16 | patch << 8)
Word 2: Generator ID
Word 3: Bound (max ID + 1)
Word 4: Schema (reserved, 0)
```

### Instruction Format
```
Word 0: (WordCount << 16) | Opcode
Words 1-N: Operands
```

### Module Sections (in order)
1. Capabilities
2. Extensions
3. Extended instruction imports
4. Memory model
5. Entry points
6. Execution modes
7. Debug strings
8. Debug names
9. Annotations (decorations)
10. Types and constants
11. Global variables
12. Functions

## Examples

See `example_test.go` for complete examples:

- Minimal module
- Module with types
- Fragment shader structure

## Testing

```bash
# Run tests
GOROOT="/c/Program Files/Go" go test ./spirv/...

# Run examples
GOROOT="/c/Program Files/Go" go test -v -run Example ./spirv/...
```

## Implementation Notes

### ID Allocation
- IDs start at 1 (0 is invalid)
- `AllocID()` returns sequential IDs
- `bound` field tracks max ID + 1

### String Encoding
- UTF-8 null-terminated
- Padded to 4-byte boundary
- Stored as little-endian words

### Float Encoding
- `AddConstantFloat32()` uses IEEE 754 single precision
- `AddConstantFloat64()` uses IEEE 754 double precision (2 words)

### Binary Generation
- Little-endian byte order
- Word-aligned (4 bytes)
- Sections written in SPIR-V specification order

## Next Steps

The next phase of implementation will add:

1. **IR to SPIR-V Translation** (`spirv/backend.go`)
   - Convert naga IR types to SPIR-V types
   - Convert naga IR expressions to SPIR-V instructions
   - Convert naga IR statements to SPIR-V control flow
   - Map naga IR address spaces to SPIR-V storage classes

2. **Type Deduplication**
   - Cache created types to avoid duplicates
   - Efficient type lookup

3. **Expression Emission**
   - Arithmetic operations
   - Logical operations
   - Load/store operations
   - Function calls

4. **Control Flow**
   - If/else blocks
   - Loops
   - Switch statements
   - Structured control flow

## References

- [SPIR-V Specification](https://registry.khronos.org/SPIR-V/specs/unified1/SPIRV.html)
- [SPIR-V Headers](https://github.com/KhronosGroup/SPIRV-Headers)
- [Rust naga SPIR-V backend](https://github.com/gfx-rs/naga/tree/main/src/back/spv)

## License

Same as gogpu/naga project.
