;
; Note: shader requires additional functionality:
;       64-Bit integer
;       64-bit Atomics on Heap Resources
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
; input                             cbuffer      NA          NA     CB0            cb3     1
; storage_atomic_scalar                 UAV    byte         r/w      U0             u0     1
; storage_atomic_arr                    UAV    byte         r/w      U1             u1     1
; storage_struct                        UAV    byte         r/w      U2             u2     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.i64 = type { i64, i64 }
%struct.S0 = type { i32 }
%input = type { i64 }

define void @cs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 2, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 3, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R5 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R3, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R6 = extractvalue %dx.types.CBufRet.i64 %R5, 0
  %R7 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R2, i32 7, i32 0, i32 undef, i32 undef, i64 %R6)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R8 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R3, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R9 = extractvalue %dx.types.CBufRet.i64 %R8, 0
  %R10 = add i64 %R9, 1
  %R11 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R1, i32 7, i32 8, i32 undef, i32 undef, i64 %R10)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R12 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R0, i32 5, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R13 = zext i32 %R4 to i64
  %R14 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R0, i32 7, i32 16, i32 undef, i32 undef, i64 %R13)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R15 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R3, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R16 = extractvalue %dx.types.CBufRet.i64 %R15, 0
  %R17 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R2, i32 6, i32 0, i32 undef, i32 undef, i64 %R16)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R18 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R3, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R19 = extractvalue %dx.types.CBufRet.i64 %R18, 0
  %R20 = add i64 %R19, 1
  %R21 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R1, i32 6, i32 8, i32 undef, i32 undef, i64 %R20)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R22 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R0, i32 4, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R23 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R0, i32 6, i32 16, i32 undef, i32 undef, i64 %R13)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  ret void
}

; Function Attrs: nounwind
declare i64 @dx.op.atomicBinOp.i64(i32, %dx.types.Handle, i32, i32, i32, i32, i64) #A0

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32, %dx.types.Handle, i32) #A2

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A3

attributes #A0 = { nounwind }
attributes #A1 = { noduplicate nounwind }
attributes #A2 = { nounwind readonly }
attributes #A3 = { nounwind readnone }

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
!M4 = !{null, !M6, !M7, null}
!M6 = !{!M8, !M9, !M10}
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M9 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M10 = !{i32 2, %struct.S0* undef, !"", i32 0, i32 2, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M7 = !{!M11}
!M11 = !{i32 0, %input* undef, !"", i32 0, i32 3, i32 1, i32 8, null}
!M5 = !{void ()* @cs_main, !"cs_main", null, !M4, !M12}
!M12 = !{i32 0, i64 4296015888, i32 4, !M13}
!M13 = !{i32 2, i32 1, i32 1}

