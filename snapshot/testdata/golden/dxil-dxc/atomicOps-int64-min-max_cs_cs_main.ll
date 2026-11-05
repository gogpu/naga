;
; Note: shader requires additional functionality:
;       64-Bit integer
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
%dx.types.ResBind = type { i32, i32, i32, i8 }
%dx.types.ResourceProperties = type { i32, i32 }
%dx.types.CBufRet.i64 = type { i64, i64 }
%struct.S0 = type { i32 }
%input = type { i64 }

define void @cs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 2, i32 2, i32 0, i8 1 }, i32 2, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 1, i32 1, i32 0, i8 1 }, i32 1, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 0, i32 0, i32 0, i8 1 }, i32 0, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind { i32 3, i32 3, i32 0, i8 2 }, i32 3, i1 false)  ; CreateHandleFromBinding(bind,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R3, %dx.types.ResourceProperties { i32 13, i32 8 })  ; AnnotateHandle(res,props)  resource: CBuffer
  %R5 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R6 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R4, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R7 = extractvalue %dx.types.CBufRet.i64 %R6, 0
  %R8 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R9 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R8, i32 7, i32 0, i32 undef, i32 undef, i64 %R7)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R10 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R4, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R11 = extractvalue %dx.types.CBufRet.i64 %R10, 0
  %R12 = add i64 %R11, 1
  %R13 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R14 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R13, i32 7, i32 8, i32 undef, i32 undef, i64 %R12)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R15 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R16 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R15, i32 5, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R17 = zext i32 %R5 to i64
  %R18 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R19 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R18, i32 7, i32 16, i32 undef, i32 undef, i64 %R17)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R20 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R4, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R21 = extractvalue %dx.types.CBufRet.i64 %R20, 0
  %R22 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R2, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R23 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R22, i32 6, i32 0, i32 undef, i32 undef, i64 %R21)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R24 = call %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32 59, %dx.types.Handle %R4, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R25 = extractvalue %dx.types.CBufRet.i64 %R24, 0
  %R26 = add i64 %R25, 1
  %R27 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R1, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R28 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R27, i32 6, i32 8, i32 undef, i32 undef, i64 %R26)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R29 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R30 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R29, i32 4, i32 0, i32 undef, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  %R31 = call %dx.types.Handle @dx.op.annotateHandle(i32 216, %dx.types.Handle %R0, %dx.types.ResourceProperties { i32 4107, i32 0 })  ; AnnotateHandle(res,props)  resource: RWByteAddressBuffer
  %R32 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R31, i32 6, i32 16, i32 undef, i32 undef, i64 %R17)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  ret void
}

; Function Attrs: nounwind readnone
declare %dx.types.Handle @dx.op.annotateHandle(i32, %dx.types.Handle, %dx.types.ResourceProperties) #A0

; Function Attrs: nounwind
declare i64 @dx.op.atomicBinOp.i64(i32, %dx.types.Handle, i32, i32, i32, i32, i64) #A1

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A2

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i64 @dx.op.cbufferLoadLegacy.i64(i32, %dx.types.Handle, i32) #A3

; Function Attrs: nounwind readnone
declare %dx.types.Handle @dx.op.createHandleFromBinding(i32, %dx.types.ResBind, i32, i1) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A0

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }
attributes #A2 = { noduplicate nounwind }
attributes #A3 = { nounwind readonly }

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
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, !M11}
!M11 = !{i32 3, i32 1}
!M9 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, !M11}
!M10 = !{i32 2, %struct.S0* undef, !"", i32 0, i32 2, i32 1, i32 11, i1 false, i1 false, i1 false, !M11}
!M7 = !{!M12}
!M12 = !{i32 0, %input* undef, !"", i32 0, i32 3, i32 1, i32 8, null}
!M5 = !{void ()* @cs_main, !"cs_main", null, !M4, !M13}
!M13 = !{i32 0, i64 1048592, i32 4, !M14}
!M14 = !{i32 2, i32 1, i32 1}

