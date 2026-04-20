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
; EntryFunctionName: test_ep
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

@"\01?w_mem2_@@3V?$matrix@M$01$01@@A.v.v" = addrspace(3) global [4 x float] undef, align 4

define void @test_ep() {
  %R0 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R1 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R2 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R3 = or i32 %R1, %R0
  %R4 = or i32 %R3, %R2
  %R5 = icmp eq i32 %R4, 0
  br i1 %R5, label %R6, label %R7

; <label>:7                                       ; preds = %R8
  store float 0.000000e+00, float addrspace(3)* getelementptr inbounds ([4 x float], [4 x float] addrspace(3)* @"\01?w_mem2_@@3V?$matrix@M$01$01@@A.v.v", i32 0, i32 0), align 4
  store float 0.000000e+00, float addrspace(3)* getelementptr inbounds ([4 x float], [4 x float] addrspace(3)* @"\01?w_mem2_@@3V?$matrix@M$01$01@@A.v.v", i32 0, i32 1), align 4
  store float 0.000000e+00, float addrspace(3)* getelementptr inbounds ([4 x float], [4 x float] addrspace(3)* @"\01?w_mem2_@@3V?$matrix@M$01$01@@A.v.v", i32 0, i32 2), align 4
  store float 0.000000e+00, float addrspace(3)* getelementptr inbounds ([4 x float], [4 x float] addrspace(3)* @"\01?w_mem2_@@3V?$matrix@M$01$01@@A.v.v", i32 0, i32 3), align 4
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
!M4 = !{void ()* @test_ep, !"test_ep", null, null, !M5}
!M5 = !{i32 4, !M6}
!M6 = !{i32 1, i32 1, i32 1}

