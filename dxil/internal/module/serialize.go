package module

import (
	"math"

	"github.com/gogpu/naga/dxil/internal/bitcode"
)

// uid converts a non-negative int (ID or index) to uint64.
// All IDs in the module are non-negative by construction.
func uid(v int) uint64 {
	return uint64(v) //nolint:gosec // IDs are always non-negative
}

// LLVM 3.7 block IDs.
const (
	blockInfoID     = 0
	firstAppBlockID = 8

	moduleBlockID   = firstAppBlockID     // 8
	paramAttrID     = firstAppBlockID + 1 // 9
	paramAttrGrpID  = firstAppBlockID + 2 // 10
	constBlockID    = firstAppBlockID + 3 // 11
	functionBlockID = firstAppBlockID + 4 // 12
	valueSymtabID   = firstAppBlockID + 6 // 14
	metadataBlockID = firstAppBlockID + 7 // 15
	typeBlockID     = firstAppBlockID + 9 // 17
)

// Module info record codes.
const (
	moduleCodeVersion    = 1
	moduleCodeTriple     = 2
	moduleCodeDataLayout = 3
	moduleCodeGlobalVar  = 7
	moduleCodeFunction   = 8
)

// Type table record codes.
const (
	typeCodeNumEntry    = 1
	typeCodeVoid        = 2
	typeCodeFloat       = 3
	typeCodeDouble      = 4
	typeCodeLabel       = 5
	typeCodeInteger     = 7
	typeCodePointer     = 8
	typeCodeHalf        = 10
	typeCodeArray       = 11
	typeCodeVector      = 12
	typeCodeMetadata    = 16
	typeCodeStructAnon  = 18
	typeCodeStructName  = 19
	typeCodeStructNamed = 20
	typeCodeFuncType    = 21 // LLVM TYPE_CODE_FUNCTION
)

// Constant record codes.
const (
	constCodeSetType   = 1
	constCodeNull      = 2
	constCodeUndef     = 3
	constCodeInteger   = 4
	constCodeFloat     = 6
	constCodeAggregate = 7
)

// Function body record codes.
const (
	funcCodeDeclareBlocks = 1
	funcCodeInstBinop     = 2
	funcCodeInstCast      = 3
	funcCodeInstGEPOld    = 9
	funcCodeInstRet       = 10
	funcCodeInstBr        = 11
	funcCodeInstSelect    = 29 // FUNC_CODE_INST_VSELECT
	funcCodeInstCmp2      = 28 // FUNC_CODE_INST_CMP2
	funcCodeInstCall      = 34
	funcCodeInstAlloca    = 19
	funcCodeInstLoad      = 20
	funcCodeInstGEP       = 43 // FUNC_CODE_INST_GEP (new format, LLVM 3.7)
	funcCodeInstStore     = 44
	funcCodeInstAtomicRMW = 38 // FUNC_CODE_INST_ATOMICRMW
	funcCodeInstCmpXchg   = 46 // FUNC_CODE_INST_CMPXCHG_OLD (LLVM 3.7)
)

// Metadata record codes.
const (
	metadataString    = 1
	metadataValue     = 2
	metadataNode      = 3
	metadataName      = 4
	metadataNamedNode = 10
)

// Value symbol table record codes.
const (
	vstCodeEntry   = 1
	vstCodeBBEntry = 2
)

// Serialize writes the Module to LLVM 3.7 bitcode format.
//
// The serialization order follows Mesa's dxil_emit_module():
//  1. Bitcode magic: 'B','C', 0xC0, 0xDE
//  2. MODULE_BLOCK (blockid=8, abbrevWidth=3)
//  3. VERSION record (code=1, value=1 for LLVM 3.7)
//  4. TRIPLE record
//  5. DATALAYOUT record
//  6. TYPE_BLOCK — all types
//  7. Module info — function declarations
//  8. CONSTANTS_BLOCK — constant values
//  9. METADATA_BLOCK — metadata nodes and named metadata
//  10. Function bodies
//  11. VALUE_SYMTAB_BLOCK — symbol names
//  12. Exit MODULE_BLOCK
func Serialize(m *Module) []byte {
	s := &serializer{
		mod: m,
		w:   bitcode.NewWriter(2),
	}
	s.assignIDs()
	s.emitModule()
	return s.w.Bytes()
}

type serializer struct {
	mod *Module
	w   *bitcode.Writer
}

// assignIDs assigns sequential value IDs to all global values.
func (s *serializer) assignIDs() {
	nextID := 0

	// Assign type IDs.
	for i, ty := range s.mod.Types {
		ty.ID = i
	}

	// Global variables get value IDs first.
	for _, gv := range s.mod.GlobalVars {
		gv.ValueID = nextID
		nextID++
	}

	// Then functions.
	for _, fn := range s.mod.Functions {
		fn.ValueID = nextID
		nextID++
	}

	// Then constants.
	for _, c := range s.mod.Constants {
		c.ValueID = nextID
		nextID++
	}

	// Metadata IDs (separate numbering).
	for i, md := range s.mod.Metadata {
		md.ID = i
	}
}

// emitModule writes the complete bitcode module.
func (s *serializer) emitModule() {
	// Bitcode magic.
	s.w.WriteBits('B', 8)
	s.w.WriteBits('C', 8)
	s.w.WriteBits(0xC0, 8)
	s.w.WriteBits(0xDE, 8)

	// MODULE_BLOCK.
	s.w.EnterBlock(moduleBlockID, 3)

	// VERSION record: value=1 means relative IDs (LLVM 3.7 bitcode).
	s.w.EmitRecord(moduleCodeVersion, []uint64{1})

	// TRIPLE record.
	s.emitStringRecord(moduleCodeTriple, s.mod.TargetTriple)

	// DATALAYOUT record.
	if s.mod.DataLayout != "" {
		s.emitStringRecord(moduleCodeDataLayout, s.mod.DataLayout)
	}

	// TYPE_BLOCK.
	s.emitTypeTable()

	// Module info: function declarations.
	s.emitModuleInfo()

	// CONSTANTS_BLOCK.
	if len(s.mod.Constants) > 0 {
		s.emitConstants()
	}

	// METADATA_BLOCK.
	if len(s.mod.Metadata) > 0 || len(s.mod.NamedMetadata) > 0 {
		s.emitMetadata()
	}

	// Function bodies.
	for i := range s.mod.Functions {
		fn := s.mod.Functions[i]
		if !fn.IsDeclaration && len(fn.BasicBlocks) > 0 {
			s.emitFunctionBody(fn)
		}
	}

	// VALUE_SYMTAB_BLOCK.
	s.emitValueSymtab()

	s.w.ExitBlock() // MODULE_BLOCK
}

// emitStringRecord encodes a string as a sequence of byte values in a record.
func (s *serializer) emitStringRecord(code uint, str string) {
	vals := make([]uint64, len(str))
	for i := 0; i < len(str); i++ {
		vals[i] = uint64(str[i])
	}
	s.w.EmitRecord(code, vals)
}

// emitTypeTable writes the TYPE_BLOCK containing all module types.
func (s *serializer) emitTypeTable() {
	// The type count includes the metadata type which is always appended.
	typeCount := len(s.mod.Types) + 1

	s.w.EnterBlock(typeBlockID, 4)

	// NUMENTRY record: total number of types.
	s.w.EmitRecord(typeCodeNumEntry, []uint64{uint64(typeCount)})

	for _, ty := range s.mod.Types {
		s.emitType(ty)
	}

	// Metadata type is always appended last (Mesa: emit_metadata_type).
	s.w.EmitRecord(typeCodeMetadata, nil)

	s.w.ExitBlock()
}

// emitType writes a single type record.
func (s *serializer) emitType(ty *Type) {
	switch ty.Kind {
	case TypeVoid:
		s.w.EmitRecord(typeCodeVoid, nil)

	case TypeInteger:
		s.w.EmitRecord(typeCodeInteger, []uint64{uint64(ty.IntBits)})

	case TypeFloat:
		switch ty.FloatBits {
		case 16:
			s.w.EmitRecord(typeCodeHalf, nil)
		case 32:
			s.w.EmitRecord(typeCodeFloat, nil)
		case 64:
			s.w.EmitRecord(typeCodeDouble, nil)
		}

	case TypePointer:
		// POINTER: [pointee type index, address space=0]
		s.w.EmitRecord(typeCodePointer, []uint64{uid(ty.PointerElem.ID), 0})

	case TypeStruct:
		if ty.StructName != "" {
			// Named struct: emit STRUCT_NAME first, then STRUCT_NAMED.
			s.emitStringRecord(typeCodeStructName, ty.StructName)
			data := make([]uint64, 1+len(ty.StructElems))
			data[0] = 0 // packed = false
			for i, elem := range ty.StructElems {
				data[1+i] = uid(elem.ID)
			}
			s.w.EmitRecord(typeCodeStructNamed, data)
		} else {
			// Anonymous struct.
			data := make([]uint64, 1+len(ty.StructElems))
			data[0] = 0 // packed = false
			for i, elem := range ty.StructElems {
				data[1+i] = uid(elem.ID)
			}
			s.w.EmitRecord(typeCodeStructAnon, data)
		}

	case TypeArray:
		// ARRAY: [numelems, eltty]
		s.w.EmitRecord(typeCodeArray, []uint64{uint64(ty.ElemCount), uid(ty.ElemType.ID)})

	case TypeVector:
		// VECTOR: [numelems, eltty]
		s.w.EmitRecord(typeCodeVector, []uint64{uint64(ty.ElemCount), uid(ty.ElemType.ID)})

	case TypeFunction:
		// FUNCTION: [vararg, retty, ...paramtys]
		data := make([]uint64, 2+len(ty.ParamTypes))
		data[0] = 0 // vararg = false
		data[1] = uid(ty.RetType.ID)
		for i, pt := range ty.ParamTypes {
			data[2+i] = uid(pt.ID)
		}
		s.w.EmitRecord(typeCodeFuncType, data)

	case TypeLabel:
		s.w.EmitRecord(typeCodeLabel, nil)

	case TypeMetadata:
		s.w.EmitRecord(typeCodeMetadata, nil)
	}
}

// emitModuleInfo writes global variable and function declaration records.
func (s *serializer) emitModuleInfo() {
	for i := range s.mod.Functions {
		fn := s.mod.Functions[i]
		s.emitFunctionDecl(fn)
	}
}

// emitFunctionDecl writes a MODULE_CODE_FUNCTION record.
func (s *serializer) emitFunctionDecl(fn *Function) {
	isDecl := uint64(0)
	if fn.IsDeclaration {
		isDecl = 1
	}
	data := []uint64{
		uid(fn.FuncType.ID), // type
		0,                   // callingconv (default=0)
		isDecl,              // isproto
		0,                   // linkage (external=0)
		0,                   // paramattr
		0,                   // alignment
		0,                   // section
		0,                   // visibility
		0,                   // gc
		0,                   // unnamed_addr
		0,                   // prologuedata
		0,                   // dllstorageclass
		0,                   // comdat
		0,                   // prefixdata
	}
	s.w.EmitRecord(moduleCodeFunction, data)
}

// emitConstants writes the CONSTANTS_BLOCK.
func (s *serializer) emitConstants() {
	s.w.EnterBlock(constBlockID, 4)

	var lastType *Type
	for _, c := range s.mod.Constants {
		// Emit SETTYPE if the type changes.
		if c.ConstType != lastType {
			s.w.EmitRecord(constCodeSetType, []uint64{uid(c.ConstType.ID)})
			lastType = c.ConstType
		}

		if c.IsUndef {
			s.w.EmitRecord(constCodeUndef, nil)
		} else if c.IsAggregate {
			// AGGREGATE: [elt0_valueid, elt1_valueid, ...]
			vals := make([]uint64, len(c.Elements))
			for i, elem := range c.Elements {
				vals[i] = uid(elem.ValueID)
			}
			s.w.EmitRecord(constCodeAggregate, vals)
		} else if c.ConstType.Kind == TypeInteger {
			// Encode signed integers using the LLVM sign-rotating encoding:
			// positive N → 2*N, negative N → 2*(-N)-1
			encoded := encodeSignRotated(c.IntValue)
			s.w.EmitRecord(constCodeInteger, []uint64{encoded})
		} else if c.ConstType.Kind == TypeFloat {
			// Float constants are stored as IEEE 754 bit patterns.
			bits := floatBits(c)
			s.w.EmitRecord(constCodeFloat, []uint64{bits})
		} else {
			s.w.EmitRecord(constCodeNull, nil)
		}
	}

	s.w.ExitBlock()
}

// encodeSignRotated encodes a signed value using LLVM's sign-rotating
// encoding: non-negative N maps to 2*N, negative N maps to 2*(-N)-1.
func encodeSignRotated(v int64) uint64 {
	if v >= 0 {
		return uint64(v) << 1
	}
	return (uint64(-v) << 1) - 1
}

// floatBits returns the IEEE 754 bit pattern for a float constant.
// For f16 and f32, returns 32-bit pattern; for f64, returns 64-bit pattern.
func floatBits(c *Constant) uint64 {
	switch c.ConstType.FloatBits {
	case 64:
		return math.Float64bits(c.FloatValue)
	default:
		// f16 and f32 are stored as 32-bit float bit patterns.
		return uint64(math.Float32bits(float32(c.FloatValue)))
	}
}

// emitMetadata writes the METADATA_BLOCK containing all metadata nodes
// and named metadata entries.
func (s *serializer) emitMetadata() {
	s.w.EnterBlock(metadataBlockID, 3)

	// Emit all metadata nodes.
	for _, md := range s.mod.Metadata {
		switch md.Kind {
		case MDString:
			s.emitMetadataString(md)
		case MDValue:
			s.emitMetadataValue(md)
		case MDTuple:
			s.emitMetadataTuple(md)
		}
	}

	// Emit named metadata entries.
	for _, named := range s.mod.NamedMetadata {
		s.emitNamedMetadata(named)
	}

	s.w.ExitBlock()
}

// emitMetadataString writes a METADATA_STRING record.
func (s *serializer) emitMetadataString(md *MetadataNode) {
	vals := make([]uint64, len(md.StringValue))
	for i := 0; i < len(md.StringValue); i++ {
		vals[i] = uint64(md.StringValue[i])
	}
	s.w.EmitRecord(metadataString, vals)
}

// emitMetadataValue writes a METADATA_VALUE record: [type, value].
func (s *serializer) emitMetadataValue(md *MetadataNode) {
	typeID := uint64(0)
	valueID := uint64(0)
	if md.ValueType != nil {
		typeID = uid(md.ValueType.ID)
	}
	if md.ValueConst != nil {
		valueID = uid(md.ValueConst.ValueID)
	}
	s.w.EmitRecord(metadataValue, []uint64{typeID, valueID})
}

// emitMetadataTuple writes a METADATA_NODE record: [n x md_id].
func (s *serializer) emitMetadataTuple(md *MetadataNode) {
	vals := make([]uint64, len(md.SubNodes))
	for i, sub := range md.SubNodes {
		if sub != nil {
			// Metadata IDs are offset by 1 (0 means null).
			vals[i] = uid(sub.ID) + 1
		}
		// else: 0 means null operand
	}
	s.w.EmitRecord(metadataNode, vals)
}

// emitNamedMetadata writes METADATA_NAME + METADATA_NAMED_NODE records.
func (s *serializer) emitNamedMetadata(named *NamedMetadataNode) {
	// METADATA_NAME: the name as bytes.
	nameVals := make([]uint64, len(named.Name))
	for i := 0; i < len(named.Name); i++ {
		nameVals[i] = uint64(named.Name[i])
	}
	s.w.EmitRecord(metadataName, nameVals)

	// METADATA_NAMED_NODE: list of metadata IDs.
	ids := make([]uint64, len(named.Operands))
	for i, op := range named.Operands {
		ids[i] = uid(op.ID)
	}
	s.w.EmitRecord(metadataNamedNode, ids)
}

// emitFunctionBody writes a FUNCTION_BLOCK for one function definition.
//
// Within a function body, value IDs are assigned sequentially starting
// from the next ID after all global values (globals, functions, constants).
// Operand references use relative encoding: current_value_id - operand_id.
//
// Reference: Mesa dxil_module.c emit_function()
func (s *serializer) emitFunctionBody(fn *Function) {
	s.w.EnterBlock(functionBlockID, 4)

	// DECLAREBLOCKS: number of basic blocks.
	s.w.EmitRecord(funcCodeDeclareBlocks, []uint64{uint64(len(fn.BasicBlocks))})

	// The current value ID counter starts after all global values
	// plus function parameters (which have implicit value IDs in LLVM bitcode).
	// Each instruction that produces a value increments this counter.
	nextValueID := s.globalValueCount()
	if fn.FuncType != nil {
		nextValueID += len(fn.FuncType.ParamTypes)
	}

	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			s.emitInstruction(instr, nextValueID)
			if instr.HasValue {
				nextValueID++
			}
		}
	}

	s.w.ExitBlock()
}

// globalValueCount returns the total number of global values
// (global vars + functions + constants).
func (s *serializer) globalValueCount() int {
	return len(s.mod.GlobalVars) + len(s.mod.Functions) + len(s.mod.Constants)
}

// emitInstruction writes a single instruction record.
//
// Reference: Mesa dxil_module.c emit_instr()
//
//nolint:gocognit,gocyclo,cyclop,funlen // instruction dispatch requires handling all LLVM instruction kinds
func (s *serializer) emitInstruction(instr *Instruction, currentValueID int) {
	switch instr.Kind {
	case InstrRet:
		if instr.ReturnValue < 0 {
			s.w.EmitRecord(funcCodeInstRet, nil)
		} else {
			// Relative encoding for return value.
			s.w.EmitRecord(funcCodeInstRet, []uint64{uint64(currentValueID - instr.ReturnValue)}) //nolint:gosec // delta always positive
		}

	case InstrBinOp:
		// BINOP: [opval_delta, opval_delta, opcode]
		// Operands: [lhs, rhs, opcode_as_int]
		if len(instr.Operands) >= 3 {
			lhsDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always non-negative
			rhsDelta := uint64(currentValueID - instr.Operands[1]) //nolint:gosec // delta always non-negative
			opcode := uint64(instr.Operands[2])                    //nolint:gosec // opcode is small positive int
			s.w.EmitRecord(funcCodeInstBinop, []uint64{lhsDelta, rhsDelta, opcode})
		}

	case InstrCmp:
		// CMP2: [opval_delta, opval_delta, pred]
		// Operands: [lhs, rhs, pred_as_int]
		if len(instr.Operands) >= 3 {
			lhsDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always non-negative
			rhsDelta := uint64(currentValueID - instr.Operands[1]) //nolint:gosec // delta always non-negative
			pred := uint64(instr.Operands[2])                      //nolint:gosec // predicate is small positive int
			s.w.EmitRecord(funcCodeInstCmp2, []uint64{lhsDelta, rhsDelta, pred})
		}

	case InstrCall:
		// CALL: [attr, cc, fnty, fn_delta, ...arg_deltas]
		// With explicit function type (bit 15 set).
		// Reference: Mesa emit_call(), LLVM 3.7 bitcode format.
		if instr.CalledFunc != nil {
			fnDelta := uint64(currentValueID - instr.CalledFunc.ValueID) //nolint:gosec // delta always positive

			// Build record with explicit function type.
			// cc: bit 15 = explicit function type present.
			data := make([]uint64, 4, 4+len(instr.Operands))
			data[0] = 0                                 // paramattr
			data[1] = 1 << 15                           // calling convention (explicit fn type)
			data[2] = uid(instr.CalledFunc.FuncType.ID) // function type
			data[3] = fnDelta                           // callee value delta
			for _, op := range instr.Operands {
				data = append(data, uint64(currentValueID-op)) //nolint:gosec // delta always positive
			}
			s.w.EmitRecord(funcCodeInstCall, data)
		}

	case InstrSelect:
		// VSELECT: [cond_delta, true_delta, false_delta]
		// Operands: [cond, trueVal, falseVal]
		if len(instr.Operands) >= 3 {
			trueDelta := uint64(currentValueID - instr.Operands[1])  //nolint:gosec // delta always positive
			falseDelta := uint64(currentValueID - instr.Operands[2]) //nolint:gosec // same
			condDelta := uint64(currentValueID - instr.Operands[0])  //nolint:gosec // same
			s.w.EmitRecord(funcCodeInstSelect, []uint64{trueDelta, falseDelta, condDelta})
		}

	case InstrCast:
		// CAST: [opval_delta, destty, castopc]
		if len(instr.Operands) >= 2 && instr.ResultType != nil {
			opDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always positive
			castOp := uint64(instr.Operands[1])                   //nolint:gosec // cast opcode is small positive int
			s.w.EmitRecord(funcCodeInstCast, []uint64{opDelta, uid(instr.ResultType.ID), castOp})
		}

	case InstrBr:
		// BR: [bb#] for unconditional, [bb#true, bb#false, cond_delta] for conditional
		if len(instr.Operands) == 1 {
			s.w.EmitRecord(funcCodeInstBr, []uint64{uint64(instr.Operands[0])}) //nolint:gosec // basic block index
		} else if len(instr.Operands) >= 3 {
			condDelta := uint64(currentValueID - instr.Operands[2]) //nolint:gosec // delta always positive
			s.w.EmitRecord(funcCodeInstBr, []uint64{
				uint64(instr.Operands[0]), //nolint:gosec // true BB index
				uint64(instr.Operands[1]), //nolint:gosec // false BB index
				condDelta,
			})
		}

	case InstrExtractVal:
		// EXTRACTVALUE: [opval_delta, idx]
		if len(instr.Operands) >= 2 {
			opDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always positive
			idx := uint64(instr.Operands[1])                      //nolint:gosec // index is small positive int
			s.w.EmitRecord(26, []uint64{opDelta, idx})            // FUNC_CODE_INST_EXTRACTVAL = 26
		}

	case InstrAlloca:
		// ALLOCA: [alloc_type_id, size_type_id, size_value_delta, align_flags]
		// Operands: [allocTypeID, sizeTypeID, sizeValueID, alignFlags]
		if len(instr.Operands) >= 4 {
			allocTypeID := uint64(instr.Operands[0]) //nolint:gosec // type ID
			sizeTypeID := uint64(instr.Operands[1])  //nolint:gosec // type ID
			sizeID := uint64(instr.Operands[2])      //nolint:gosec // value ID (absolute, not delta)
			alignFlags := uint64(instr.Operands[3])  //nolint:gosec // alignment flags
			s.w.EmitRecord(funcCodeInstAlloca, []uint64{allocTypeID, sizeTypeID, sizeID, alignFlags})
		}

	case InstrLoad:
		// LOAD: [ptr_delta, type_id, align, is_volatile]
		// Operands: [ptrValueID, typeID, align, isVolatile]
		if len(instr.Operands) >= 4 {
			ptrDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always positive
			typeID := uint64(instr.Operands[1])                    //nolint:gosec // type ID
			align := uint64(instr.Operands[2])                     //nolint:gosec // alignment
			isVolatile := uint64(instr.Operands[3])                //nolint:gosec // 0 or 1
			s.w.EmitRecord(funcCodeInstLoad, []uint64{ptrDelta, typeID, align, isVolatile})
		}

	case InstrGEP:
		// GEP (new format, code=43): [inbounds, source_elem_type_id, ptr_delta, ...idx_deltas]
		// Operands: [inbounds, sourceElemTypeID, ptrValueID, ...indexValueIDs]
		// Reference: Mesa dxil_module.c emit_gep()
		if len(instr.Operands) >= 3 {
			inbounds := uint64(instr.Operands[0])                  //nolint:gosec // 0 or 1
			elemTypeID := uint64(instr.Operands[1])                //nolint:gosec // type ID (not remapped)
			ptrDelta := uint64(currentValueID - instr.Operands[2]) //nolint:gosec // delta
			data := make([]uint64, 3, 3+len(instr.Operands)-3)
			data[0] = inbounds
			data[1] = elemTypeID
			data[2] = ptrDelta
			for i := 3; i < len(instr.Operands); i++ {
				data = append(data, uint64(currentValueID-instr.Operands[i])) //nolint:gosec // delta
			}
			s.w.EmitRecord(funcCodeInstGEP, data)
		}

	case InstrStore:
		// STORE: [ptr_delta, value_delta, align, is_volatile]
		// Operands: [ptrValueID, valueID, align, isVolatile]
		// Store does not produce a value, but uses currentValueID for deltas.
		// Mesa uses instr->value.id for delta computation even though store has no result.
		// In our model, the "virtual" ID is currentValueID (next would-be value).
		if len(instr.Operands) >= 4 {
			ptrDelta := uint64(currentValueID - instr.Operands[0])   //nolint:gosec // delta always positive
			valueDelta := uint64(currentValueID - instr.Operands[1]) //nolint:gosec // delta always positive
			align := uint64(instr.Operands[2])                       //nolint:gosec // alignment
			isVolatile := uint64(instr.Operands[3])                  //nolint:gosec // 0 or 1
			s.w.EmitRecord(funcCodeInstStore, []uint64{ptrDelta, valueDelta, align, isVolatile})
		}

	case InstrAtomicRMW:
		// ATOMICRMW: [ptr_delta, val_delta, operation, is_volatile, ordering, synchscope]
		// Operands: [ptrValueID, valueID, atomicOp, isVolatile, ordering, synchscope]
		if len(instr.Operands) >= 6 {
			ptrDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always positive
			valDelta := uint64(currentValueID - instr.Operands[1]) //nolint:gosec // delta always positive
			atomicOp := uint64(instr.Operands[2])                  //nolint:gosec // atomic operation enum
			isVolatile := uint64(instr.Operands[3])                //nolint:gosec // 0 or 1
			ordering := uint64(instr.Operands[4])                  //nolint:gosec // memory ordering
			synchscope := uint64(instr.Operands[5])                //nolint:gosec // synch scope
			s.w.EmitRecord(funcCodeInstAtomicRMW, []uint64{ptrDelta, valDelta, atomicOp, isVolatile, ordering, synchscope})
		}

	case InstrCmpXchg:
		// CMPXCHG: [ptr_delta, cmp_delta, new_delta, is_volatile, ordering, synchscope]
		// Operands: [ptrValueID, cmpValueID, newValueID, isVolatile, ordering, synchscope]
		if len(instr.Operands) >= 6 {
			ptrDelta := uint64(currentValueID - instr.Operands[0]) //nolint:gosec // delta always positive
			cmpDelta := uint64(currentValueID - instr.Operands[1]) //nolint:gosec // delta always positive
			newDelta := uint64(currentValueID - instr.Operands[2]) //nolint:gosec // delta always positive
			isVolatile := uint64(instr.Operands[3])                //nolint:gosec // 0 or 1
			ordering := uint64(instr.Operands[4])                  //nolint:gosec // memory ordering
			synchscope := uint64(instr.Operands[5])                //nolint:gosec // synch scope
			s.w.EmitRecord(funcCodeInstCmpXchg, []uint64{ptrDelta, cmpDelta, newDelta, isVolatile, ordering, synchscope})
		}
	}
}

// emitValueSymtab writes the VALUE_SYMTAB_BLOCK containing function names.
func (s *serializer) emitValueSymtab() {
	// Collect entries: we need names for all functions.
	type entry struct {
		valueID int
		name    string
	}
	var entries []entry
	for i := range s.mod.Functions {
		fn := s.mod.Functions[i]
		if fn.Name != "" {
			entries = append(entries, entry{fn.ValueID, fn.Name})
		}
	}

	if len(entries) == 0 {
		return
	}

	s.w.EnterBlock(valueSymtabID, 4)

	for _, e := range entries {
		// VST_CODE_ENTRY: [valueid, ...namechar]
		vals := make([]uint64, 1+len(e.name))
		vals[0] = uid(e.valueID)
		for i := 0; i < len(e.name); i++ {
			vals[1+i] = uint64(e.name[i])
		}
		s.w.EmitRecord(vstCodeEntry, vals)
	}

	s.w.ExitBlock()
}
