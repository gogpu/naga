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
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

@"\01?a@@3PAIA" = external addrspace(3) global [64 x i32], align 4

define void @main() {
  %R0 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R1 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R2 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R3 = or i32 %R1, %R0
  %R4 = or i32 %R3, %R2
  %R5 = icmp eq i32 %R4, 0
  br i1 %R5, label %R6, label %R7

; <label>:7                                       ; preds = %R8
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 1), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 2), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 3), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 4), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 5), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 6), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 7), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 8), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 9), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 10), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 11), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 12), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 13), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 14), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 15), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 16), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 17), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 18), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 19), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 20), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 21), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 22), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 23), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 24), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 25), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 26), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 27), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 28), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 29), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 30), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 31), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 32), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 33), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 34), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 35), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 36), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 37), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 38), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 39), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 40), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 41), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 42), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 43), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 44), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 45), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 46), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 47), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 48), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 49), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 50), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 51), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 52), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 53), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 54), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 55), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 56), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 57), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 58), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 59), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 60), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 61), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 62), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?a@@3PAIA", i32 0, i32 63), align 4
  br label %R7

; <label>:8                                       ; preds = %R6, %R8
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A0

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A1

attributes #A0 = { nounwind readnone }
attributes #A1 = { noduplicate nounwind }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.entryPoints = !{!M4}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 8}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{void ()* @main, !"main", null, null, !M5}
!M5 = !{i32 4, !M6}
!M6 = !{i32 1, i32 1, i32 1}

