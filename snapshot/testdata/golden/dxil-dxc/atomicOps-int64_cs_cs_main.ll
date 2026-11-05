;
; Note: shader requires additional functionality:
;       64-Bit integer
;       64-bit Atomics on Group Shared
;
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
%dx.types.ResBind = type { i32, i32, i32, i8 }
%dx.types.ResourceProperties = type { i32, i32 }
%struct.S0 = type { i32 }

@"\01?workgroup_atomic_scalar@@3_KA" = external addrspace(3) global i64, align 8
@"\01?workgroup_atomic_arr@@3PA_JA" = external addrspace(3) global [2 x i64], align 8
@"\01?workgroup_struct@@3UStruct@@A.0" = addrspace(3) global i64 undef, align 8
@"\01?workgroup_struct@@3UStruct@@A.1" = addrspace(3) global [2 x i64] undef, align 8

define void @cs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 2, i32 2, i32 0, i8 1 }, i32 2, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 1, i32 1, i32 0, i8 1 }, i32 1, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 0, i32 0, i32 0, i8 1 }, i32 0, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R3 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R4 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R5 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R6 = or i32 %R4, %R3
  %R7 = or i32 %R6, %R5
  %R8 = icmp eq i32 %R7, 0
  br i1 %R8, label %R9, label %R10

; <label>:10                                      ; preds = %R11
  store i64 0, i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", align 8
  store i64 0, i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 0), align 8
  store i64 0, i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), align 8
  store i64 0, i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", align 8
  store i64 0, i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 0), align 8
  store i64 0, i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), align 8
  br label %R10

; <label>:11                                      ; preds = %R9, %R11
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R12 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R13 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R12, i32 8, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R14 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R15 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R14, i32 8, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R16 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R17 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R16, i32 8, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R18 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R19 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R18, i32 8, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R20 = atomicrmw xchg i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R21 = atomicrmw xchg i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R22 = atomicrmw xchg i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R23 = atomicrmw xchg i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R24 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R25 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R24, i32 2, i32 0, i32 undef, i32 undef, i64 0)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R26 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R27 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R26, i32 2, i32 8, i32 undef, i32 undef, i64 0)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R28 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R29 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R28, i32 2, i32 0, i32 undef, i32 undef, i64 0)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R30 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R31 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R30, i32 2, i32 16, i32 undef, i32 undef, i64 0)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R32 = atomicrmw or i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 0 seq_cst
  %R33 = atomicrmw or i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 0 seq_cst
  %R34 = atomicrmw or i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 0 seq_cst
  %R35 = atomicrmw or i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 0 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R36 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R37 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R36, i32 0, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R38 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R39 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R38, i32 0, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R40 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R41 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R40, i32 0, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R42 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R43 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R42, i32 0, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R44 = atomicrmw add i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R45 = atomicrmw add i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R46 = atomicrmw add i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R47 = atomicrmw add i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R48 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R49 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R48, i32 0, i32 0, i32 undef, i32 undef, i64 4294967295)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R50 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R51 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R50, i32 0, i32 8, i32 undef, i32 undef, i64 -1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R52 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R53 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R52, i32 0, i32 0, i32 undef, i32 undef, i64 4294967295)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R54 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R55 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R54, i32 0, i32 16, i32 undef, i32 undef, i64 -1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R56 = atomicrmw add i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 4294967295 seq_cst
  %R57 = atomicrmw add i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 -1 seq_cst
  %R58 = atomicrmw add i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 4294967295 seq_cst
  %R59 = atomicrmw add i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 -1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R60 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R61 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R60, i32 5, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R62 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R63 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R62, i32 5, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R64 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R65 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R64, i32 5, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R66 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R67 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R66, i32 5, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R68 = atomicrmw umax i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R69 = atomicrmw max i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R70 = atomicrmw umax i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R71 = atomicrmw max i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R72 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R73 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R72, i32 4, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R74 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R75 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R74, i32 4, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R76 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R77 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R76, i32 4, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R78 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R79 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R78, i32 4, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R80 = atomicrmw umin i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R81 = atomicrmw min i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R82 = atomicrmw umin i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R83 = atomicrmw min i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R84 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R85 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R84, i32 1, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R86 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R87 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R86, i32 1, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R88 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R89 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R88, i32 1, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R90 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R91 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R90, i32 1, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R92 = atomicrmw and i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R93 = atomicrmw and i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R94 = atomicrmw and i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R95 = atomicrmw and i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R96 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R97 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R96, i32 2, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R98 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R99 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R98, i32 2, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R100 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R101 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R100, i32 2, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R102 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R103 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R102, i32 2, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R104 = atomicrmw or i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R105 = atomicrmw or i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R106 = atomicrmw or i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R107 = atomicrmw or i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R108 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R109 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R108, i32 3, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R110 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R111 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R110, i32 3, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R112 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R113 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R112, i32 3, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R114 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R115 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R114, i32 3, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R116 = atomicrmw xor i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R117 = atomicrmw xor i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R118 = atomicrmw xor i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R119 = atomicrmw xor i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  %R120 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R121 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R120, i32 8, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R122 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R123 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R122, i32 8, i32 8, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R124 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R125 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R124, i32 8, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R126 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R127 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R126, i32 8, i32 16, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R128 = atomicrmw xchg i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1 seq_cst
  %R129 = atomicrmw xchg i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1 seq_cst
  %R130 = atomicrmw xchg i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1 seq_cst
  %R131 = atomicrmw xchg i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1 seq_cst
  %R132 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R133 = call i64 @dx.op.atomicCompareExchange.i64(i32 79, %dx.types.Handle %R132, i32 0, i32 undef, i32 undef, i64 1, i64 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R134 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R135 = call i64 @dx.op.atomicCompareExchange.i64(i32 79, %dx.types.Handle %R134, i32 8, i32 undef, i32 undef, i64 1, i64 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R136 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R137 = call i64 @dx.op.atomicCompareExchange.i64(i32 79, %dx.types.Handle %R136, i32 0, i32 undef, i32 undef, i64 1, i64 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R138 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R139 = call i64 @dx.op.atomicCompareExchange.i64(i32 79, %dx.types.Handle %R138, i32 16, i32 undef, i32 undef, i64 1, i64 2)  ; AtomicCompareExchange(handle,offset0,offset1,offset2,compareValue,newValue)
  %R140 = cmpxchg i64 addrspace(3)* @"\01?workgroup_atomic_scalar@@3_KA", i64 1, i64 2 seq_cst seq_cst
  %R141 = cmpxchg i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_atomic_arr@@3PA_JA", i32 0, i32 1), i64 1, i64 2 seq_cst seq_cst
  %R142 = cmpxchg i64 addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.0", i64 1, i64 2 seq_cst seq_cst
  %R143 = cmpxchg i64 addrspace(3)* getelementptr inbounds ([2 x i64], [2 x i64] addrspace(3)* @"\01?workgroup_struct@@3UStruct@@A.1", i32 0, i32 1), i64 1, i64 2 seq_cst seq_cst
  ret void
}

; Function Attrs: nounwind readnone
declare %dx.types.Handle @dx.op.annotateHandle(i32, %dx.types.Handle, %dx.types.ResourceProperties) #A0

; Function Attrs: nounwind
declare i64 @dx.op.atomicBinOp.i64(i32, %dx.types.Handle, i32, i32, i32, i32, i64) #A1

; Function Attrs: nounwind
declare i64 @dx.op.atomicCompareExchange.i64(i32, %dx.types.Handle, i32, i32, i32, i64, i64) #A1

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A2

; Function Attrs: nounwind readnone
declare %dx.types.Handle @dx.op.createHandleFromBinding(i32, %dx.types.ResBind, i32, i1) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A0

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }
attributes #A2 = { noduplicate nounwind }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{null, !M6, null, null}
!M6 = !{!M7, !M8, !M9}
!M7 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, !M10}
!M10 = !{i32 3, i32 1}
!M8 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, !M10}
!M9 = !{i32 2, %struct.S0* undef, !"", i32 0, i32 2, i32 1, i32 11, i1 false, i1 false, i1 false, !M10}
!M5 = !{void ()* @cs_main, !"cs_main", null, !M4, !M11}
!M11 = !{i32 0, i64 269484048, i32 4, !M12}
!M12 = !{i32 2, i32 1, i32 1}
!M13 = !{!M14, !M14, i64 0}
!M14 = !{!"long long", !M15, i64 0}
!M15 = !{!"omnipotent char", !M16, i64 0}
!M16 = !{!"<ident>"}

