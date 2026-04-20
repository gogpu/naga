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
	PixelShader         ShaderKind = 0
	VertexShader        ShaderKind = 1
	GeometryShader      ShaderKind = 2
	HullShader          ShaderKind = 3
	DomainShader        ShaderKind = 4
	ComputeShader       ShaderKind = 5
	MeshShader          ShaderKind = 13
	AmplificationShader ShaderKind = 14
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
	// For TypePointer: address space (0 = default, 3 = thread group shared).
	// Workgroup global variables in DXIL live in addrspace 3 and any pointer
	// derived from them carries the same addrspace, which the validator
	// memcmp's against atomicrmw operand types.
	PointerAddrSpace uint8

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

// GetPointerType returns a pointer type to the given element type in the
// default address space (0).
func (m *Module) GetPointerType(elem *Type) *Type {
	return m.GetPointerTypeAS(elem, 0)
}

// GetPointerTypeAS returns a pointer type to the given element type in the
// specified address space. addrspace 3 is the DXIL thread-group-shared
// space used by var<workgroup> globals; pointers derived from such globals
// MUST carry the matching addrspace or atomicrmw on them is rejected by
// the validator with 'Non-groupshared or node record destination to
// atomic operation'.
func (m *Module) GetPointerTypeAS(elem *Type, addrSpace uint8) *Type {
	// Search for existing pointer type with matching addrspace.
	for _, ty := range m.Types {
		if ty.Kind == TypePointer && ty.PointerElem == elem && ty.PointerAddrSpace == addrSpace {
			return ty
		}
	}
	return m.addType(&Type{Kind: TypePointer, PointerElem: elem, PointerAddrSpace: addrSpace})
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

// GetArrayType returns an array type. Identical (elem, count) pairs return the
// SAME *Type instance — LLVM's type system is structural, and the validator
// rejects load/store pairs whose pointee types are nominally distinct even
// when structurally identical (HRESULT 0x80aa0009 "Explicit load/store type
// does not match pointee type"). Without dedup, two callers walking the same
// IR ArrayType independently would mint different *Type IDs, and a store
// rooted at one alloca would be unloadable through a pointer typed by the
// other.
func (m *Module) GetArrayType(elem *Type, count uint) *Type {
	for _, ty := range m.Types {
		if ty.Kind == TypeArray && ty.ElemType == elem && ty.ElemCount == count {
			return ty
		}
	}
	return m.addType(&Type{
		Kind:      TypeArray,
		ElemType:  elem,
		ElemCount: count,
	})
}

// GetVectorType returns a vector type with the given element type and count.
// Deduplicates identical (elem, count) pairs like GetArrayType.
// Used for texture resource struct types that carry vector members
// (e.g., class.Texture2D<vector<float, 4>> needs <4 x float> in its struct).
func (m *Module) GetVectorType(elem *Type, count uint) *Type {
	for _, ty := range m.Types {
		if ty.Kind == TypeVector && ty.ElemType == elem && ty.ElemCount == count {
			return ty
		}
	}
	return m.addType(&Type{
		Kind:      TypeVector,
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

	// AttrSetID is the 1-based index into the PARAMATTR_BLOCK (0 = none).
	// DXC marks every intrinsic declaration with at least "nounwind", which
	// the D3D12 runtime validator checks. Declarations default to
	// AttrSetNounwind; bodies default to 0.
	AttrSetID uint32
}

// Function attribute set IDs emitted in the PARAMATTR_GROUP_BLOCK.
// Keep in sync with serialize.go::emitParamAttrGroupBlock and the per-
// intrinsic classification table in dxil/internal/emit. DXC distinguishes
// pure functions (provably no memory effects), readonly functions (read but
// don't write), and impure functions; correct attribution lets downstream
// LLVM passes (DCE, GVN, LICM) reason about safe motion / elimination.
//
// The set IDs are the positional indices in the emitted PARAMATTR_BLOCK,
// 1-based; 0 means "no attributes".
const (
	// AttrSetNone is the sentinel for "no attributes".
	AttrSetNone uint32 = 0
	// AttrSetNounwind = group {nounwind} on the function-level slot. Used
	// for impure intrinsics (stores, atomics, barriers, discard) and for
	// the entry point @main (it stores outputs, so not pure).
	AttrSetNounwind uint32 = 1
	// AttrSetReadNone = group {nounwind, readnone} — pure intrinsics:
	// threadId/groupId/loadInput, math, conversions. No memory effects at
	// all; LLVM may freely move, eliminate, or hoist these calls.
	AttrSetReadNone uint32 = 2
	// AttrSetReadOnly = group {nounwind, readonly} — memory-reading
	// intrinsics: bufferLoad/cbufferLoadLegacy/sample/textureLoad. Don't
	// write memory but do read it; LLVM can hoist out of loops over
	// memory-disjoint stores but must not eliminate as dead.
	AttrSetReadOnly uint32 = 3
	// AttrSetNoDuplicate = group {noduplicate, nounwind} — barrier
	// intrinsics. DXC marks dx.op.barrier with noduplicate to prevent
	// LLVM from duplicating barrier calls across code paths.
	AttrSetNoDuplicate uint32 = 4
)

// AddFunction adds a function declaration or definition to the module.
func (m *Module) AddFunction(name string, funcType *Type, isDecl bool) *Function {
	attrs := AttrSetNone
	if isDecl {
		// Every DXIL intrinsic declaration carries "nounwind". D3D12
		// rejects DXIL modules whose declarations omit this attribute
		// with "shader is corrupt" (HRESULT 0x80070057).
		attrs = AttrSetNounwind
	}
	f := &Function{
		Name:          name,
		FuncType:      funcType,
		IsDeclaration: isDecl,
		AttrSetID:     attrs,
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

// PrependInstruction inserts an instruction at the front of the basic
// block. Used for late-discovered allocas: LLVM's SSA verifier requires
// alloca instructions to live in the entry block so they dominate every
// use; emitting them where the first use happens (e.g. inside an if/loop
// body) trips 'Instruction does not dominate all uses'. Insertion at the
// front of the entry block guarantees the alloca is the earliest user
// of its name and dominates every subsequent reference.
func (bb *BasicBlock) PrependInstruction(instr *Instruction) {
	bb.Instructions = append([]*Instruction{instr}, bb.Instructions...)
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
	InstrInsertVal                   // insertvalue
	InstrAlloca                      // stack allocation
	InstrLoad                        // memory load
	InstrStore                       // memory store
	InstrGEP                         // getelementptr
	InstrPhi                         // phi node
	InstrAtomicRMW                   // atomicrmw (atomic read-modify-write)
	InstrCmpXchg                     // cmpxchg (atomic compare-exchange)
)

// PhiIncoming is one (value, predecessor-block) pair attached to an
// InstrPhi instruction. ValueID identifies the SSA value that flows in
// from BBIndex; BBIndex is the position of the predecessor in the
// parent function's BasicBlocks slice.
type PhiIncoming struct {
	ValueID int
	BBIndex int
}

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

	// For InstrPhi: incoming (value, predecessor-bb) pairs.
	PhiIncomings []PhiIncoming

	// Flags holds optional instruction-level flags. For InstrBinOp with
	// floating-point operands, this carries fast-math flags (FMF) matching
	// the LLVM 3.7 bitcode encoding: bit 0 = UnsafeAlgebra ("fast"),
	// bit 1 = NoNaNs, bit 2 = NoInfs, bit 3 = NoSignedZeros,
	// bit 4 = AllowReciprocal. DXC always sets UnsafeAlgebra (bit 0) for
	// non-precise float ops, producing the "fast" keyword in IR text.
	Flags uint32

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

// NewBrInstr creates an unconditional branch to the target basic block.
// The operand is the basic block index within the parent function.
func NewBrInstr(targetBBIndex int) *Instruction {
	return &Instruction{
		Kind:        InstrBr,
		HasValue:    false,
		Operands:    []int{targetBBIndex},
		ReturnValue: -1,
	}
}

// NewBrCondInstr creates a conditional branch.
// If cond is true, branches to trueBBIndex; otherwise to falseBBIndex.
// cond is an emitter value ID (i1 type). The BB indices are positions
// within the parent function's BasicBlocks slice.
func NewBrCondInstr(trueBBIndex, falseBBIndex, cond int) *Instruction {
	return &Instruction{
		Kind:        InstrBr,
		HasValue:    false,
		Operands:    []int{trueBBIndex, falseBBIndex, cond},
		ReturnValue: -1,
	}
}

// NewPhiInstr creates an SSA phi node merging values from multiple
// predecessor basic blocks. resultType is the i1/i32/f32/etc. type of
// the merged value; incomings pairs each predecessor BB index with the
// value ID that flows in from it. LLVM 3.7 FUNC_CODE_INST_PHI requires
// exactly one (value, bb) pair per predecessor at serialization time;
// callers should ensure the incomings slice covers every predecessor of
// the BB that hosts the phi.
func NewPhiInstr(resultType *Type, incomings []PhiIncoming) *Instruction {
	return &Instruction{
		Kind:         InstrPhi,
		HasValue:     true,
		ResultType:   resultType,
		PhiIncomings: incomings,
		ReturnValue:  -1,
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

	// IsAggregate is true for aggregate constants (arrays, structs).
	// Elements contains the sub-constant value IDs.
	IsAggregate bool
	Elements    []*Constant

	// IsDataArray is true for ConstantDataArray constants — serialized as
	// CST_CODE_DATA (code 22) instead of CST_CODE_AGGREGATE (code 7).
	//
	// Required for metadata payloads whose consumer-side loader does a
	// hard `dyn_cast<ConstantDataArray>`. Example: dx.viewIdState —
	// `DxilMDHelper::LoadDxilViewIdState` (DxilMetadataHelper.cpp:2211)
	// runs at D3D12 runtime format validation during
	// CreateGraphicsPipelineState and rejects the `ConstantArray` form
	// that CST_CODE_AGGREGATE produces.
	//
	// Element values are stored inline as raw uint64; decoding rules
	// depend on the array element type (i32 → unsigned integer value,
	// f32 → Float32bits, etc.). The element type is carried by
	// ConstType.ArrayElem.
	IsDataArray bool
	DataValues  []uint64

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

// AddAggregateConst adds an aggregate constant (array or struct) to the module.
// The elements are the sub-constants that make up the aggregate.
func (m *Module) AddAggregateConst(ty *Type, elements []*Constant) *Constant {
	c := &Constant{
		ConstType:   ty,
		IsAggregate: true,
		Elements:    elements,
	}
	m.Constants = append(m.Constants, c)
	return c
}

// AddDataArrayConst adds a ConstantDataArray constant — an array of primitive
// elements encoded inline as CST_CODE_DATA (code 22) rather than referencing
// separate element constants by value ID (CST_CODE_AGGREGATE, code 7).
//
// Required for metadata payloads whose consumer performs a hard
// `dyn_cast<ConstantDataArray>`. The most important case is dx.viewIdState,
// which D3D12's CreateGraphicsPipelineState validates via
// DxilMDHelper::LoadDxilViewIdState and rejects the ConstantArray form.
//
// Each value in `values` is the raw element bit pattern:
//   - i32: the unsigned integer value
//   - i8/i16: the unsigned integer value (zero-extended)
//   - f32: math.Float32bits result zero-extended
//   - f64: math.Float64bits result
//
// Reference: LLVM Bitcode/LLVMBitCodes.h (CST_CODE_DATA = 22) and
// BitcodeReader's handling of ConstantDataSequential.
func (m *Module) AddDataArrayConst(ty *Type, values []uint64) *Constant {
	c := &Constant{
		ConstType:   ty,
		IsDataArray: true,
		DataValues:  values,
	}
	m.Constants = append(m.Constants, c)
	return c
}

// AddUndefConst creates an undef constant of the given type.
// Used for resource metadata fields[1] which require an undef pointer value.
//
// Reference: Mesa dxil_module.c dxil_module_get_undef() line ~1845
func (m *Module) AddUndefConst(ty *Type) *Constant {
	c := &Constant{
		ConstType: ty,
		IsUndef:   true,
	}
	m.Constants = append(m.Constants, c)
	return c
}

// GlobalVar represents a global variable.
//
// VarType is the POINTEE type (the element stored at the global), NOT the
// pointer type. LLVM bitcode uses the EXPLICIT_TYPE flag (Mesa
// dxil_module.c:GVAR_FLAG_EXPLICIT_TYPE) to indicate that the type ID in
// the GLOBALVAR record is the element type — the global itself is
// implicitly an addrspace(N) pointer to that element.
type GlobalVar struct {
	Name        string
	VarType     *Type
	AddrSpace   uint8
	IsConstant  bool
	Initializer *Constant
	ValueID     int
}

// AddGlobalVar registers a global variable with the given pointee type
// and address space. Returns the GlobalVar so the caller can read its
// ValueID after serialization.
func (m *Module) AddGlobalVar(name string, elemType *Type, addrSpace uint8) *GlobalVar {
	gv := &GlobalVar{
		Name:      name,
		VarType:   elemType,
		AddrSpace: addrSpace,
	}
	m.GlobalVars = append(m.GlobalVars, gv)
	return gv
}
