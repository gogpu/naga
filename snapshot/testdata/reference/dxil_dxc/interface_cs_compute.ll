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
; EntryFunctionName: compute
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

@"\01?output@@3PAIA" = external addrspace(3) global [1 x i32], align 4

define void @compute() {
  %R0 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R1 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R2 = call i32 @dx.op.flattenedThreadIdInGroup.i32(i32 96)  ; FlattenedThreadIdInGroup()
  %R3 = call i32 @dx.op.groupId.i32(i32 94, i32 0)  ; GroupId(component)
  %R4 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R5 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R6 = or i32 %R4, %R1
  %R7 = or i32 %R6, %R5
  %R8 = icmp eq i32 %R7, 0
  br i1 %R8, label %R9, label %R10

; <label>:10                                      ; preds = %R11
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([1 x i32], [1 x i32] addrspace(3)* @"\01?output@@3PAIA", i32 0, i32 0), align 4
  br label %R10

; <label>:11                                      ; preds = %R9, %R11
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R12 = shl i32 %R3, 1
  %R13 = add i32 %R1, %R0
  %R14 = add i32 %R13, %R2
  %R15 = add i32 %R14, %R12
  store i32 %R15, i32 addrspace(3)* getelementptr inbounds ([1 x i32], [1 x i32] addrspace(3)* @"\01?output@@3PAIA", i32 0, i32 0), align 4, !tbaa !M0
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.flattenedThreadIdInGroup.i32(i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.groupId.i32(i32, i32) #A0

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A1

attributes #A0 = { nounwind readnone }
attributes #A1 = { noduplicate nounwind }

!llvm.ident = !{!M1}
!dx.version = !{!M2}
!dx.valver = !{!M3}
!dx.shaderModel = !{!M4}
!dx.entryPoints = !{!M5}

!M1 = !{!"<ident>"}
!M2 = !{i32 1, i32 0}
!M3 = !{i32 1, i32 8}
!M4 = !{!"cs", i32 6, i32 0}
!M5 = !{void ()* @compute, !"compute", null, null, !M6}
!M6 = !{i32 4, !M7}
!M7 = !{i32 1, i32 1, i32 1}
!M0 = !{!M8, !M8, i64 0}
!M8 = !{!"int", !M9, i64 0}
!M9 = !{!"omnipotent char", !M10, i64 0}
!M10 = !{!"<ident>"}

