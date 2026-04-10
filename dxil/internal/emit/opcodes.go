// Package emit implements naga IR to DXIL module lowering.
//
// This package translates naga IR types, expressions, and statements into
// DXIL module constructs (LLVM 3.7 IR with dx.op intrinsics). The output
// is a module.Module that can be serialized to bitcode and wrapped in a
// DXBC container.
//
// Reference: Mesa's src/microsoft/compiler/nir_to_dxil.c
package emit

// DXILOpcode represents a dx.op intrinsic opcode number.
type DXILOpcode uint32

// dx.op opcode values from the DXIL specification.
// Reference: DXC's hctdb.py and DxilConstants.h
const (
	// I/O operations.
	OpLoadInput   DXILOpcode = 4
	OpStoreOutput DXILOpcode = 5

	// Unary float operations.
	OpFAbs        DXILOpcode = 6
	OpSaturate    DXILOpcode = 7
	OpIsNaN       DXILOpcode = 8
	OpIsInf       DXILOpcode = 9
	OpIsFinite    DXILOpcode = 10
	OpIsNormal    DXILOpcode = 11
	OpCos         DXILOpcode = 12
	OpSin         DXILOpcode = 13
	OpTan         DXILOpcode = 14
	OpAcos        DXILOpcode = 15
	OpAsin        DXILOpcode = 16
	OpAtan        DXILOpcode = 17
	OpHCos        DXILOpcode = 18
	OpHSin        DXILOpcode = 19
	OpHTan        DXILOpcode = 20
	OpExp         DXILOpcode = 21
	OpFrc         DXILOpcode = 22
	OpLog         DXILOpcode = 23
	OpSqrt        DXILOpcode = 24
	OpRsqrt       DXILOpcode = 25
	OpRoundNE     DXILOpcode = 26
	OpRoundNI     DXILOpcode = 27
	OpRoundPI     DXILOpcode = 28
	OpRoundZ      DXILOpcode = 29
	OpReverseBits DXILOpcode = 30
	OpCountBits   DXILOpcode = 31
	OpFirstbitLo  DXILOpcode = 32
	OpFirstbitHi  DXILOpcode = 33

	// Binary float/int operations.
	OpFMax DXILOpcode = 35
	OpFMin DXILOpcode = 36
	OpIMax DXILOpcode = 37
	OpIMin DXILOpcode = 38
	OpUMax DXILOpcode = 39
	OpUMin DXILOpcode = 40

	// Ternary operations.
	OpFMad DXILOpcode = 46
	OpFma  DXILOpcode = 47
	OpIMad DXILOpcode = 48
	OpUMad DXILOpcode = 49

	// Dot product operations.
	OpDot2 DXILOpcode = 54
	OpDot3 DXILOpcode = 55
	OpDot4 DXILOpcode = 56

	// Resource operations.
	OpCreateHandle       DXILOpcode = 57
	OpCBufferLoadLegacy  DXILOpcode = 59
	OpSample             DXILOpcode = 60
	OpSampleBias         DXILOpcode = 61
	OpSampleLevel        DXILOpcode = 62
	OpSampleGrad         DXILOpcode = 63
	OpSampleCmp          DXILOpcode = 64
	OpSampleCmpLevelZero DXILOpcode = 65
	OpTextureLoad        DXILOpcode = 66
	OpTextureStore       DXILOpcode = 67
	OpBufferLoad         DXILOpcode = 68
	OpBufferStore        DXILOpcode = 69

	// Derivative operations.
	OpDerivCoarseX DXILOpcode = 83
	OpDerivCoarseY DXILOpcode = 84
	OpDerivFineX   DXILOpcode = 85
	OpDerivFineY   DXILOpcode = 86

	// Atomic operations.
	OpAtomicBinOp   DXILOpcode = 78
	OpAtomicCmpXchg DXILOpcode = 79

	// Barrier.
	OpBarrier DXILOpcode = 80

	// Thread/dispatch ID operations.
	OpThreadID            DXILOpcode = 93
	OpGroupID             DXILOpcode = 94
	OpThreadIDInGroup     DXILOpcode = 95
	OpFlattenedTIDInGroup DXILOpcode = 96
)

// DXILAtomicOp represents the atomic operation kind for dx.op.atomicBinOp.
// Reference: Mesa nir_to_dxil.c enum dxil_atomic_op (line ~399)
type DXILAtomicOp uint32

const (
	DXILAtomicAdd      DXILAtomicOp = 0
	DXILAtomicAnd      DXILAtomicOp = 1
	DXILAtomicOr       DXILAtomicOp = 2
	DXILAtomicXor      DXILAtomicOp = 3
	DXILAtomicIMin     DXILAtomicOp = 4
	DXILAtomicIMax     DXILAtomicOp = 5
	DXILAtomicUMin     DXILAtomicOp = 6
	DXILAtomicUMax     DXILAtomicOp = 7
	DXILAtomicExchange DXILAtomicOp = 8
)

// DXILBarrierMode represents DXIL barrier mode flags.
// These can be combined with bitwise OR.
// Reference: Mesa nir_to_dxil.c emit_barrier_impl() (line ~3082)
type DXILBarrierMode uint32

const (
	BarrierModeSyncThreadGroup     DXILBarrierMode = 1
	BarrierModeUAVFenceGlobal      DXILBarrierMode = 2
	BarrierModeUAVFenceThreadGroup DXILBarrierMode = 4
	BarrierModeGroupSharedMemFence DXILBarrierMode = 8
)

// BinOpKind represents LLVM binary operation opcodes for DXIL.
// These are used in the FUNC_CODE_INST_BINOP record.
type BinOpKind uint32

// LLVM 3.7 binary operation codes.
const (
	BinOpAdd  BinOpKind = 0  // integer add
	BinOpFAdd BinOpKind = 1  // float add (not used directly; mapped to fadd)
	BinOpSub  BinOpKind = 2  // integer sub
	BinOpFSub BinOpKind = 3  // float sub
	BinOpMul  BinOpKind = 4  // integer mul
	BinOpFMul BinOpKind = 5  // float mul
	BinOpUDiv BinOpKind = 6  // unsigned div
	BinOpSDiv BinOpKind = 7  // signed div
	BinOpFDiv BinOpKind = 8  // float div
	BinOpURem BinOpKind = 9  // unsigned remainder
	BinOpSRem BinOpKind = 10 // signed remainder
	BinOpFRem BinOpKind = 11 // float remainder
	BinOpShl  BinOpKind = 12 // shift left
	BinOpLShr BinOpKind = 13 // logical shift right
	BinOpAShr BinOpKind = 14 // arithmetic shift right
	BinOpAnd  BinOpKind = 15 // bitwise and
	BinOpOr   BinOpKind = 16 // bitwise or
	BinOpXor  BinOpKind = 17 // bitwise xor
)

// CmpPredicate represents LLVM comparison predicates.
type CmpPredicate uint32

// LLVM 3.7 floating-point comparison predicates (FCMP).
const (
	FCmpFalse CmpPredicate = 0  // Always false
	FCmpOEQ   CmpPredicate = 1  // Ordered and equal
	FCmpOGT   CmpPredicate = 2  // Ordered and greater than
	FCmpOGE   CmpPredicate = 3  // Ordered and greater/equal
	FCmpOLT   CmpPredicate = 4  // Ordered and less than
	FCmpOLE   CmpPredicate = 5  // Ordered and less/equal
	FCmpONE   CmpPredicate = 6  // Ordered and not equal
	FCmpORD   CmpPredicate = 7  // Ordered (no NaNs)
	FCmpUNO   CmpPredicate = 8  // Unordered (at least one NaN)
	FCmpUEQ   CmpPredicate = 9  // Unordered or equal
	FCmpUGT   CmpPredicate = 10 // Unordered or greater than
	FCmpUGE   CmpPredicate = 11 // Unordered or greater/equal
	FCmpULT   CmpPredicate = 12 // Unordered or less than
	FCmpULE   CmpPredicate = 13 // Unordered or less/equal
	FCmpUNE   CmpPredicate = 14 // Unordered or not equal
	FCmpTrue  CmpPredicate = 15 // Always true
)

// LLVM 3.7 integer comparison predicates (ICMP).
const (
	ICmpEQ  CmpPredicate = 32 // Equal
	ICmpNE  CmpPredicate = 33 // Not equal
	ICmpUGT CmpPredicate = 34 // Unsigned greater than
	ICmpUGE CmpPredicate = 35 // Unsigned greater/equal
	ICmpULT CmpPredicate = 36 // Unsigned less than
	ICmpULE CmpPredicate = 37 // Unsigned less/equal
	ICmpSGT CmpPredicate = 38 // Signed greater than
	ICmpSGE CmpPredicate = 39 // Signed greater/equal
	ICmpSLT CmpPredicate = 40 // Signed less than
	ICmpSLE CmpPredicate = 41 // Signed less/equal
)

// CastOpKind represents LLVM cast operation codes.
type CastOpKind uint32

// LLVM 3.7 cast operation codes.
const (
	CastTrunc   CastOpKind = 0  // Truncate integers
	CastZExt    CastOpKind = 1  // Zero-extend integers
	CastSExt    CastOpKind = 2  // Sign-extend integers
	CastFPToUI  CastOpKind = 3  // Float to unsigned integer
	CastFPToSI  CastOpKind = 4  // Float to signed integer
	CastUIToFP  CastOpKind = 5  // Unsigned integer to float
	CastSIToFP  CastOpKind = 6  // Signed integer to float
	CastFPTrunc CastOpKind = 7  // Truncate float (e.g., double→float)
	CastFPExt   CastOpKind = 8  // Extend float (e.g., float→double)
	CastBitcast CastOpKind = 11 // Bitcast (same size, different type)
)
