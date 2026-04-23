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
; NumThreads=(1,1,1)
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
; EntryFunctionName: main
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; v_indices                             UAV    byte         r/w      U0             u0     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%struct.S0 = type { i32 }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R2 = shl i32 %R1, 2
  %R3 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 %R2, i32 undef)  ; BufferLoad(srv,index,wot)
  %R4 = extractvalue %dx.types.ResRet.i32 %R3, 0
  br label %R5

; <label>:6                                       ; preds = %R6, %R7
  %R8 = phi i32 [ %R9, %R6 ], [ %R4, %R7 ]
  %R10 = phi i32 [ %R11, %R6 ], [ 0, %R7 ]
  %R12 = phi i32 [ %R13, %R6 ], [ -1, %R7 ]
  %R14 = phi i32 [ %R15, %R6 ], [ -1, %R7 ]
  %R16 = icmp eq i32 %R14, 0
  %R17 = zext i1 %R16 to i32
  %R15 = add i32 %R14, -1
  %R18 = icmp ugt i32 %R8, 1
  br i1 %R18, label %R6, label %R19

; <label>:15                                      ; preds = %R5
  %R13 = sub i32 %R12, %R17
  %R20 = and i32 %R8, 1
  %R21 = icmp eq i32 %R20, 0
  %R22 = lshr i32 %R8, 1
  %R23 = mul i32 %R8, 3
  %R24 = add i32 %R23, 1
  %R9 = select i1 %R21, i32 %R22, i32 %R24
  %R11 = add i32 %R10, 1
  %R25 = icmp eq i32 %R12, %R17
  %R26 = icmp eq i32 %R15, 0
  %R27 = and i1 %R26, %R25
  br i1 %R27, label %R19, label %R5

; <label>:27                                      ; preds = %R6, %R5
  %R28 = phi i32 [ %R10, %R5 ], [ %R11, %R6 ]
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R2, i32 undef, i32 %R28, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A2

attributes #A0 = { nounwind readonly }
attributes #A1 = { nounwind }
attributes #A2 = { nounwind readnone }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 8}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{null, !M6, null, null}
!M6 = !{!M7}
!M7 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M5 = !{void ()* @main, !"main", null, !M4, !M8}
!M8 = !{i32 0, i64 16, i32 4, !M9}
!M9 = !{i32 1, i32 1, i32 1}

