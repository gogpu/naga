// Package module provides an in-memory representation of a DXIL module.
//
// DXIL modules are LLVM 3.7 IR modules with DXIL-specific metadata and
// dx.op intrinsic calls. This package defines the types needed to build
// a module in memory before serializing it to LLVM 3.7 bitcode.
//
// Reference: Mesa's dxil_internal.h and dxil_module.h
package module

// ShaderKind identifies the type of DXIL shader.
type ShaderKind uint32

// Shader kinds matching DXIL specification.
const (
	PixelShader    ShaderKind = 0
	VertexShader   ShaderKind = 1
	GeometryShader ShaderKind = 2
	HullShader     ShaderKind = 3
	DomainShader   ShaderKind = 4
	ComputeShader  ShaderKind = 5
)

// Module represents a DXIL module (LLVM 3.7 IR with DXIL metadata).
type Module struct {
	// ShaderKind is the type of shader (vertex, pixel, compute, etc.).
	ShaderKind ShaderKind

	// MajorVersion and MinorVersion are the DXIL version numbers.
	MajorVersion uint32
	MinorVersion uint32

	// TargetTriple is always "dxil-ms-dx" for DXIL.
	TargetTriple string

	// DataLayout is the LLVM data layout string.
	DataLayout string

	// Types contains all types used in the module.
	Types []*Type

	// Functions contains all function declarations and definitions.
	Functions []*Function

	// Constants contains all constant values.
	Constants []*Constant

	// GlobalVars contains global variable declarations.
	GlobalVars []*GlobalVar

	// NamedMetadata maps metadata names (e.g. "dx.version") to
	// metadata node lists.
	NamedMetadata []*NamedMetadataNode

	// Metadata contains all metadata nodes referenced by the module.
	Metadata []*MetadataNode

	// cachedTypes provides deduplication for commonly used types.
	voidType    *Type
	int1Type    *Type
	int8Type    *Type
	int16Type   *Type
	int32Type   *Type
	int64Type   *Type
	float16Type *Type
	float32Type *Type
	float64Type *Type
}

// NewModule creates a new empty DXIL module with the given shader kind.
func NewModule(kind ShaderKind) *Module {
	return &Module{
		ShaderKind:   kind,
		MajorVersion: 1,
		MinorVersion: 0,
		TargetTriple: "dxil-ms-dx",
		DataLayout:   "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64",
	}
}

// TypeKind identifies the category of a Type.
type TypeKind int

// Type kinds matching LLVM 3.7 type system subset used by DXIL.
const (
	TypeVoid     TypeKind = iota // void
	TypeInteger                  // iN (i1, i8, i16, i32, i64)
	TypeFloat                    // half, float, double
	TypePointer                  // pointer to another type
	TypeStruct                   // named or anonymous struct
	TypeArray                    // array of elements
	TypeVector                   // vector of elements
	TypeFunction                 // function signature
	TypeLabel                    // basic block label
	TypeMetadata                 // metadata type
)

// Type represents an LLVM 3.7 type.
type Type struct {
	Kind TypeKind

	// ID is the index of this type in the module's type table.
	// Assigned during serialization.
	ID int

	// For TypeInteger: bit width (1, 8, 16, 32, 64).
	IntBits uint

	// For TypeFloat: bit width (16, 32, 64).
	FloatBits uint

	// For TypePointer: the element type pointed to.
	PointerElem *Type

	// For TypeStruct: name (empty for anonymous) and element types.
	StructName  string
	StructElems []*Type

	// For TypeArray and TypeVector: element type and count.
	ElemType  *Type
	ElemCount uint

	// For TypeFunction: return type and parameter types.
	RetType    *Type
	ParamTypes []*Type
}

// GetVoidType returns the void type, creating it if needed.
func (m *Module) GetVoidType() *Type {
	if m.voidType == nil {
		m.voidType = m.addType(&Type{Kind: TypeVoid})
	}
	return m.voidType
}

// GetIntType returns an integer type with the given bit width.
func (m *Module) GetIntType(bits uint) *Type {
	switch bits {
	case 1:
		if m.int1Type == nil {
			m.int1Type = m.addType(&Type{Kind: TypeInteger, IntBits: 1})
		}
		return m.int1Type
	case 8:
		if m.int8Type == nil {
			m.int8Type = m.addType(&Type{Kind: TypeInteger, IntBits: 8})
		}
		return m.int8Type
	case 16:
		if m.int16Type == nil {
			m.int16Type = m.addType(&Type{Kind: TypeInteger, IntBits: 16})
		}
		return m.int16Type
	case 32:
		if m.int32Type == nil {
			m.int32Type = m.addType(&Type{Kind: TypeInteger, IntBits: 32})
		}
		return m.int32Type
	case 64:
		if m.int64Type == nil {
			m.int64Type = m.addType(&Type{Kind: TypeInteger, IntBits: 64})
		}
		return m.int64Type
	default:
		return m.addType(&Type{Kind: TypeInteger, IntBits: bits})
	}
}

// GetFloatType returns a floating-point type with the given bit width.
func (m *Module) GetFloatType(bits uint) *Type {
	switch bits {
	case 16:
		if m.float16Type == nil {
			m.float16Type = m.addType(&Type{Kind: TypeFloat, FloatBits: 16})
		}
		return m.float16Type
	case 32:
		if m.float32Type == nil {
			m.float32Type = m.addType(&Type{Kind: TypeFloat, FloatBits: 32})
		}
		return m.float32Type
	case 64:
		if m.float64Type == nil {
			m.float64Type = m.addType(&Type{Kind: TypeFloat, FloatBits: 64})
		}
		return m.float64Type
	default:
		return m.addType(&Type{Kind: TypeFloat, FloatBits: bits})
	}
}

// GetPointerType returns a pointer type to the given element type.
func (m *Module) GetPointerType(elem *Type) *Type {
	// Search for existing pointer type.
	for _, ty := range m.Types {
		if ty.Kind == TypePointer && ty.PointerElem == elem {
			return ty
		}
	}
	return m.addType(&Type{Kind: TypePointer, PointerElem: elem})
}

// GetFunctionType returns a function type with the given return and parameter types.
func (m *Module) GetFunctionType(ret *Type, params []*Type) *Type {
	return m.addType(&Type{
		Kind:       TypeFunction,
		RetType:    ret,
		ParamTypes: params,
	})
}

// GetStructType returns a named struct type.
func (m *Module) GetStructType(name string, elems []*Type) *Type {
	return m.addType(&Type{
		Kind:        TypeStruct,
		StructName:  name,
		StructElems: elems,
	})
}

// GetArrayType returns an array type.
func (m *Module) GetArrayType(elem *Type, count uint) *Type {
	return m.addType(&Type{
		Kind:      TypeArray,
		ElemType:  elem,
		ElemCount: count,
	})
}

// GetLabelType returns the label type used for basic block references.
func (m *Module) GetLabelType() *Type {
	return m.addType(&Type{Kind: TypeLabel})
}

// GetMetadataType returns the metadata type.
func (m *Module) GetMetadataType() *Type {
	return m.addType(&Type{Kind: TypeMetadata})
}

func (m *Module) addType(ty *Type) *Type {
	ty.ID = len(m.Types)
	m.Types = append(m.Types, ty)
	return ty
}

// Function represents an LLVM function declaration or definition.
type Function struct {
	// Name is the function name (e.g., "main", "dx.op.loadInput.f32").
	Name string

	// FuncType is the function's type (TypeFunction).
	FuncType *Type

	// IsDeclaration is true for external declarations (no body).
	IsDeclaration bool

	// BasicBlocks contains the function body (nil for declarations).
	BasicBlocks []*BasicBlock

	// ValueID is assigned during serialization.
	ValueID int
}

// AddFunction adds a function declaration or definition to the module.
func (m *Module) AddFunction(name string, funcType *Type, isDecl bool) *Function {
	f := &Function{
		Name:          name,
		FuncType:      funcType,
		IsDeclaration: isDecl,
	}
	m.Functions = append(m.Functions, f)
	return f
}

// AddBasicBlock adds a new basic block to the function.
func (f *Function) AddBasicBlock(name string) *BasicBlock {
	bb := &BasicBlock{
		Name: name,
	}
	f.BasicBlocks = append(f.BasicBlocks, bb)
	return bb
}

// BasicBlock represents a sequence of instructions with a single entry
// and single exit.
type BasicBlock struct {
	// Name is an optional label for the basic block.
	Name string

	// Instructions contains the block's instruction sequence.
	Instructions []*Instruction
}

// AddInstruction appends an instruction to the basic block.
func (bb *BasicBlock) AddInstruction(instr *Instruction) {
	bb.Instructions = append(bb.Instructions, instr)
}

// InstrKind identifies the type of an Instruction.
type InstrKind int

// Instruction kinds matching LLVM 3.7 instruction set subset used by DXIL.
const (
	InstrRet        InstrKind = iota // return
	InstrBr                          // branch
	InstrCall                        // function call
	InstrBinOp                       // binary operation
	InstrCmp                         // comparison
	InstrCast                        // type cast
	InstrSelect                      // select (ternary)
	InstrExtractVal                  // extractvalue
	InstrAlloca                      // stack allocation
	InstrLoad                        // memory load
	InstrStore                       // memory store
	InstrGEP                         // getelementptr
	InstrPhi                         // phi node
)

// Instruction represents a single LLVM IR instruction.
type Instruction struct {
	Kind InstrKind

	// HasValue is true if this instruction produces a value.
	HasValue bool

	// ResultType is the type of the produced value (nil if HasValue is false).
	ResultType *Type

	// Operands are the instruction's input values (interpretation
	// depends on Kind).
	Operands []int // indices into value numbering

	// For InstrRet: ReturnValue is the value to return (-1 for void).
	ReturnValue int

	// For InstrCall: the called function.
	CalledFunc *Function

	// ValueID is assigned during serialization.
	ValueID int
}

// NewRetVoidInstr creates a void return instruction.
func NewRetVoidInstr() *Instruction {
	return &Instruction{
		Kind:        InstrRet,
		HasValue:    false,
		ReturnValue: -1,
	}
}

// Constant represents a constant value in the module.
type Constant struct {
	// ConstType is the type of this constant.
	ConstType *Type

	// IntValue is used for integer constants.
	IntValue int64

	// FloatValue is used for floating-point constants.
	FloatValue float64

	// IsUndef is true for undef values.
	IsUndef bool

	// ValueID is assigned during serialization.
	ValueID int
}

// AddIntConst adds an integer constant to the module.
func (m *Module) AddIntConst(ty *Type, value int64) *Constant {
	c := &Constant{
		ConstType: ty,
		IntValue:  value,
	}
	m.Constants = append(m.Constants, c)
	return c
}

// GlobalVar represents a global variable.
type GlobalVar struct {
	Name        string
	VarType     *Type
	IsConstant  bool
	Initializer *Constant
	ValueID     int
}
