;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; no parameters
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; no parameters
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Compute Shader
; NumThreads=(2,1,1)
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 0
; SigOutputElements: 0
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 0
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: cs_main
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; storage_atomic_scalar                 UAV    byte         r/w      U0             u0     1
; storage_atomic_arr                    UAV    byte         r/w      U1             u1     1
; storage_struct                        UAV    byte         r/w      U2             u2     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%struct.S0 = type { i32 }

@"\01?workgroup_atomic_scalar@@3IA" = external addrspace(3) global i32, align 4
@"\01?workgroup_atomic_arr@@3PAHA" = external addrspace(3) global [2 x i32], align 4
@"\01?workgroup_struct@@3UStruct@@A.0" = addrspace(3) global i32 undef, align 4
@"\01?workgroup_struct@@3UStruct@@A.1" = addrspace(3) global [2 x i32] undef, align 4

define void @cs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 2, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R4 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R5 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R6 = or i32 %R4, %R3
  %R7 = or i32 %R6, %R5
  %R8 = icmp eq i32 %R7, 0
  br i1 %R8, label %R9, label %R10

; <label>:10                                      ; preds = %R11
  store i32 0, i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", align 4, !tbaa !M0
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), align 4
  store i32 0, i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), align 4
  br label %R10

; <label>:11                                      ; preds = %R9, %R11
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R2, i32 0, i32 undef, i32 1, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R1, i32 4, i32 undef, i32 1, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 0, i32 undef, i32 1, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 8, i32 undef, i32 1, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  store i32 1, i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", align 4, !tbaa !M0
  store i32 1, i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), align 4, !tbaa !M0
  store i32 1, i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", align 4, !tbaa !M0
  store i32 1, i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), align 4, !tbaa !M0
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R12 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 0, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R13 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 0, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R14 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 0, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R15 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 0, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R16 = atomicrmw add i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R17 = atomicrmw add i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R18 = atomicrmw add i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R19 = atomicrmw add i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R20 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 0, i32 0, i32 undef, i32 undef, i32 -1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R21 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 0, i32 4, i32 undef, i32 undef, i32 -1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R22 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 0, i32 0, i32 undef, i32 undef, i32 -1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R23 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 0, i32 8, i32 undef, i32 undef, i32 -1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R24 = atomicrmw add i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 -1 seq_cst
  %R25 = atomicrmw add i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 -1 seq_cst
  %R26 = atomicrmw add i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 -1 seq_cst
  %R27 = atomicrmw add i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 -1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R28 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 7, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R29 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 5, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R30 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 7, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R31 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 5, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R32 = atomicrmw umax i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R33 = atomicrmw max i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R34 = atomicrmw umax i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R35 = atomicrmw max i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R36 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 6, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R37 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 4, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R38 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 6, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R39 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 4, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R40 = atomicrmw umin i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R41 = atomicrmw min i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R42 = atomicrmw umin i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R43 = atomicrmw min i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R44 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 1, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R45 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 1, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R46 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 1, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R47 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 1, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R48 = atomicrmw and i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R49 = atomicrmw and i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R50 = atomicrmw and i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R51 = atomicrmw and i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R52 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 2, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R53 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 2, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R54 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 2, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R55 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 2, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R56 = atomicrmw or i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R57 = atomicrmw or i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R58 = atomicrmw or i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R59 = atomicrmw or i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R60 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 3, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R61 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 3, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R62 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 3, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R63 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 3, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R64 = atomicrmw xor i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R65 = atomicrmw xor i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R66 = atomicrmw xor i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R67 = atomicrmw xor i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  %R68 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R2, i32 8, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R69 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R1, i32 8, i32 4, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R70 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 8, i32 0, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R71 = call i32 @dx.op.atomicBinOp.i32(i32 78, %dx.types.Handle %R0, i32 8, i32 8, i32 undef, i32 undef, i32 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R72 = atomicrmw xchg i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1 seq_cst
  %R73 = atomicrmw xchg i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1 seq_cst
  %R74 = atomicrmw xchg i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1 seq_cst
  %R75 = atomicrmw xchg i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1 seq_cst
  %R76 = call i32 @dx.op.atomicCompareExchange.i32(i32 79, %dx.types.Handle %R2, i32 0, i32 undef, i32 undef, i32 1, i32 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R77 = call i32 @dx.op.atomicCompareExchange.i32(i32 79, %dx.types.Handle %R1, i32 4, i32 undef, i32 undef, i32 1, i32 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R78 = call i32 @dx.op.atomicCompareExchange.i32(i32 79, %dx.types.Handle %R0, i32 0, i32 undef, i32 undef, i32 1, i32 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R79 = call i32 @dx.op.atomicCompareExchange.i32(i32 79, %dx.types.Handle %R0, i32 8, i32 undef, i32 undef, i32 1, i32 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R80 = cmpxchg i32 addrspace(3)* @"\01?workgroup_atomic_scalar@@3IA", i32 1, i32 2 seq_cst seq_cst
  %R81 = cmpxchg i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_atomic_arr@@3PAHA", i32 0, i32 1), i32 1, i32 2 seq_cst seq_cst
  %R82 = cmpxchg i32 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i32 1, i32 2 seq_cst seq_cst
  %R83 = cmpxchg i32 addrspace(3)* getelementptr inbounds ([2 x i32], [2 x i32] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i32 1, i32 2 seq_cst seq_cst
  ret void
}

; Function Attrs: nounwind
declare i32 @dx.op.atomicBinOp.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32) #A0

; Function Attrs: nounwind
declare i32 @dx.op.atomicCompareExchange.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32) #A0

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A3

attributes #A0 = { nounwind }
attributes #A1 = { noduplicate nounwind }
attributes #A2 = { nounwind readonly }
attributes #A3 = { nounwind readnone }

!llvm.ident = !{!M1}
!dx.version = !{!M2}
!dx.valver = !{!M3}
!dx.shaderModel = !{!M4}
!dx.resources = !{!M5}
!dx.entryPoints = !{!M6}

!M1 = !{!"<ident>"}
!M2 = !{i32 1, i32 0}
!M3 = !{i32 1, i32 8}
!M4 = !{!"cs", i32 6, i32 0}
!M5 = !{null, !M7, null, null}
!M7 = !{!M8, !M9, !M10}
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M9 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M10 = !{i32 2, %struct.S0* undef, !"", i32 0, i32 2, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M6 = !{void ()* @cs_main, !"cs_main", null, !M5, !M11}
!M11 = !{i32 0, i64 16, i32 4, !M12}
!M12 = !{i32 2, i32 1, i32 1}
!M0 = !{!M13, !M13, i64 0}
!M13 = !{!"int", !M14, i64 0}
!M14 = !{!"omnipotent char", !M15, i64 0}
!M15 = !{!"<ident>"}

