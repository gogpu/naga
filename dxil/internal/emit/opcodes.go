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
	OpFAbs             DXILOpcode = 6
	OpSaturate         DXILOpcode = 7
	OpIsNaN            DXILOpcode = 8
	OpIsInf            DXILOpcode = 9
	OpIsFinite         DXILOpcode = 10
	OpIsNormal         DXILOpcode = 11
	OpCos              DXILOpcode = 12
	OpSin              DXILOpcode = 13
	OpTan              DXILOpcode = 14
	OpAcos             DXILOpcode = 15
	OpAsin             DXILOpcode = 16
	OpAtan             DXILOpcode = 17
	OpHCos             DXILOpcode = 18
	OpHSin             DXILOpcode = 19
	OpHTan             DXILOpcode = 20
	OpExp              DXILOpcode = 21
	OpFrc              DXILOpcode = 22
	OpLog              DXILOpcode = 23
	OpSqrt             DXILOpcode = 24
	OpRsqrt            DXILOpcode = 25
	OpRoundNE          DXILOpcode = 26
	OpRoundNI          DXILOpcode = 27
	OpRoundPI          DXILOpcode = 28
	OpRoundZ           DXILOpcode = 29
	OpReverseBits      DXILOpcode = 30
	OpCountBits        DXILOpcode = 31
	OpFirstbitLo       DXILOpcode = 32
	OpFirstbitHi       DXILOpcode = 33
	OpFirstbitShiHi    DXILOpcode = 34 // firstbit_shi_hi (signed high bit)
	OpBfrev            DXILOpcode = 30 // alias for ReverseBits
	OpLdexp            DXILOpcode = 43 // ldexp(value, exp)
	OpMakeDouble       DXILOpcode = 101
	OpSplitDouble      DXILOpcode = 102
	OpBitcastI16toF16  DXILOpcode = 125
	OpBitcastF16toI16  DXILOpcode = 126
	OpLegacyF32ToF16   DXILOpcode = 130
	OpLegacyF16ToF32   DXILOpcode = 131
	OpFirstbitShiHiAlt DXILOpcode = 34

	// Bit field operations.
	OpBfi  DXILOpcode = 53 // bit field insert
	OpIBfe DXILOpcode = 51 // signed bit field extract
	OpUBfe DXILOpcode = 52 // unsigned bit field extract

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

	// Query operations.
	OpGetDimensions DXILOpcode = 72

	// Gather operations.
	OpTextureGather    DXILOpcode = 73
	OpTextureGatherCmp DXILOpcode = 74

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

	// Wave (subgroup) operations.
	OpWaveIsFirstLane   DXILOpcode = 110
	OpWaveGetLaneIndex  DXILOpcode = 111
	OpWaveGetLaneCount  DXILOpcode = 112
	OpWaveAnyTrue       DXILOpcode = 113
	OpWaveAllTrue       DXILOpcode = 114
	OpWaveBallot        DXILOpcode = 116
	OpWaveReadLaneAt    DXILOpcode = 117
	OpWaveReadLaneFirst DXILOpcode = 118
	OpWaveActiveOp      DXILOpcode = 119
	OpWaveActiveBit     DXILOpcode = 120
	OpWavePrefixOp      DXILOpcode = 121
	OpQuadReadLaneAt    DXILOpcode = 122
	OpQuadOp            DXILOpcode = 123

	// Ray query operations (SM 6.5).
	OpAllocateRayQuery                              DXILOpcode = 178
	OpRayQueryTraceRayInline                        DXILOpcode = 179
	OpRayQueryProceed                               DXILOpcode = 180
	OpRayQueryAbort                                 DXILOpcode = 181
	OpRayQueryCommitNonOpaqueTriangleHit            DXILOpcode = 182
	OpRayQueryCommitProceduralPrimitiveHit          DXILOpcode = 183
	OpRayQueryCommittedStatus                       DXILOpcode = 184
	OpRayQueryCandidateType                         DXILOpcode = 185
	OpRayQueryCandidateObjectToWorld3x4             DXILOpcode = 186
	OpRayQueryCandidateWorldToObject3x4             DXILOpcode = 187
	OpRayQueryCommittedObjectToWorld3x4             DXILOpcode = 188
	OpRayQueryCommittedWorldToObject3x4             DXILOpcode = 189
	OpRayQueryCandidateProceduralPrimitiveNonOpaque DXILOpcode = 190
	OpRayQueryCandidateTriangleFrontFace            DXILOpcode = 191
	OpRayQueryCommittedTriangleFrontFace            DXILOpcode = 192
	OpRayQueryCandidateTriangleBarycentrics         DXILOpcode = 193
	OpRayQueryCommittedTriangleBarycentrics         DXILOpcode = 194
	OpRayQueryRayFlags                              DXILOpcode = 195
	OpRayQueryWorldRayOrigin                        DXILOpcode = 196
	OpRayQueryWorldRayDirection                     DXILOpcode = 197
	OpRayQueryRayTMin                               DXILOpcode = 198
	OpRayQueryCandidateTriangleRayT                 DXILOpcode = 199
	OpRayQueryCommittedRayT                         DXILOpcode = 200
	OpRayQueryCandidateInstanceIndex                DXILOpcode = 201
	OpRayQueryCandidateInstanceID                   DXILOpcode = 202
	OpRayQueryCandidateGeometryIndex                DXILOpcode = 203
	OpRayQueryCandidatePrimitiveIndex               DXILOpcode = 204
	OpRayQueryCandidateObjectRayOrigin              DXILOpcode = 205
	OpRayQueryCandidateObjectRayDirection           DXILOpcode = 206
	OpRayQueryCommittedInstanceIndex                DXILOpcode = 207
	OpRayQueryCommittedInstanceID                   DXILOpcode = 208
	OpRayQueryCommittedGeometryIndex                DXILOpcode = 209
	OpRayQueryCommittedPrimitiveIndex               DXILOpcode = 210
	OpRayQueryCommittedObjectRayOrigin              DXILOpcode = 211
	OpRayQueryCommittedObjectRayDirection           DXILOpcode = 212

	// Mesh shader operations.
	OpSetMeshOutputCounts  DXILOpcode = 168
	OpEmitIndices          DXILOpcode = 169
	OpGetMeshPayload       DXILOpcode = 170
	OpStoreVertexOutput    DXILOpcode = 171
	OpStorePrimitiveOutput DXILOpcode = 172
	OpDispatchMesh         DXILOpcode = 173
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

// LLVM 3.7 bitcode binary operation codes.
// In LLVM bitcode, int and float ops share the same opcode.
// The reader distinguishes float vs int by the operand type.
// Reference: LLVM BitcodeReader.cpp getDecodedBinaryOpcode()
// Reference: Mesa dxil_module.h enum dxil_bin_opcode
const (
	BinOpAdd  BinOpKind = 0  // add (int) / fadd (float)
	BinOpFAdd BinOpKind = 0  // same as Add — float add uses opcode 0 with float operands
	BinOpSub  BinOpKind = 1  // sub (int) / fsub (float)
	BinOpFSub BinOpKind = 1  // same as Sub
	BinOpMul  BinOpKind = 2  // mul (int) / fmul (float)
	BinOpFMul BinOpKind = 2  // same as Mul
	BinOpUDiv BinOpKind = 3  // udiv (int only)
	BinOpSDiv BinOpKind = 4  // sdiv (int) / fdiv (float)
	BinOpFDiv BinOpKind = 4  // same as SDiv
	BinOpURem BinOpKind = 5  // urem (int) / frem (float)
	BinOpSRem BinOpKind = 6  // srem (int only)
	BinOpFRem BinOpKind = 5  // same as URem
	BinOpShl  BinOpKind = 7  // shift left
	BinOpLShr BinOpKind = 8  // logical shift right
	BinOpAShr BinOpKind = 9  // arithmetic shift right
	BinOpAnd  BinOpKind = 10 // bitwise and
	BinOpOr   BinOpKind = 11 // bitwise or
	BinOpXor  BinOpKind = 12 // bitwise xor
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

// AtomicRMWOp represents LLVM atomicrmw operation codes.
// Used in FUNC_CODE_INST_ATOMICRMW record.
// Reference: LLVM LLVMAtomicRMWBinOp enum, LLVM BitcodeReader.cpp
type AtomicRMWOp uint32

const (
	AtomicRMWXchg AtomicRMWOp = 0  // exchange
	AtomicRMWAdd  AtomicRMWOp = 1  // add
	AtomicRMWSub  AtomicRMWOp = 2  // subtract
	AtomicRMWAnd  AtomicRMWOp = 3  // bitwise and
	AtomicRMWNand AtomicRMWOp = 4  // bitwise nand
	AtomicRMWOr   AtomicRMWOp = 5  // bitwise or
	AtomicRMWXor  AtomicRMWOp = 6  // bitwise xor
	AtomicRMWMax  AtomicRMWOp = 7  // signed max
	AtomicRMWMin  AtomicRMWOp = 8  // signed min
	AtomicRMWUMax AtomicRMWOp = 9  // unsigned max
	AtomicRMWUMin AtomicRMWOp = 10 // unsigned min
)

// DXILWaveOp represents the operation kind for dx.op.waveActiveOp / dx.op.wavePrefixOp.
// Reference: DXC DXIL.rst WaveActiveOp/WavePrefixOp
type DXILWaveOp uint32

const (
	DXILWaveOpSum DXILWaveOp = 0 // Add
	DXILWaveOpMul DXILWaveOp = 1 // Product (Mul)
	DXILWaveOpMin DXILWaveOp = 2 // Min
	DXILWaveOpMax DXILWaveOp = 3 // Max
)

// DXILWaveOpSign represents the signed/unsigned flag for dx.op.waveActiveOp.
type DXILWaveOpSign uint32

const (
	DXILWaveOpSignSigned   DXILWaveOpSign = 0
	DXILWaveOpSignUnsigned DXILWaveOpSign = 1
)

// DXILWaveBitOp represents the bit operation kind for dx.op.waveActiveBit.
type DXILWaveBitOp uint32

const (
	DXILWaveBitAnd DXILWaveBitOp = 0
	DXILWaveBitOr  DXILWaveBitOp = 1
	DXILWaveBitXor DXILWaveBitOp = 2
)

// DXILQuadOpKind represents the operation kind for dx.op.quadOp.
type DXILQuadOpKind uint32

const (
	DXILQuadOpReadAcrossX    DXILQuadOpKind = 0 // SwapX (horizontal)
	DXILQuadOpReadAcrossY    DXILQuadOpKind = 1 // SwapY (vertical)
	DXILQuadOpReadAcrossDiag DXILQuadOpKind = 2 // SwapDiagonal
)

// LLVM memory ordering constants.
const (
	AtomicOrderingMonotonic uint32 = 2 // Monotonic (relaxed)
	AtomicOrderingAcquire   uint32 = 4 // Acquire
	AtomicOrderingRelease   uint32 = 5 // Release
	AtomicOrderingAcqRel    uint32 = 6 // Acquire-Release
	AtomicOrderingSeqCst    uint32 = 7 // Sequentially Consistent
)
