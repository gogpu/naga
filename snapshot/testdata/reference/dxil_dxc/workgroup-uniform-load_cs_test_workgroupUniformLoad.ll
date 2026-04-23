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
; NumThreads=(4,1,1)
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
; EntryFunctionName: test_workgroupUniformLoad
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

@"\01?arr_i32_@@3PAHA" = external addrspace(3) global [128 x i32], align 4

define void @test_workgroupUniformLoad() {
  %R0 = call i32 @dx.op.groupId.i32(i32 94, i32 0)  ; GroupId(component)
  %R1 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R2 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R3 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R4 = or i32 %R2, %R1
  %R5 = or i32 %R4, %R3
  %R6 = icmp eq i32 %R5, 0
  br i1 %R6, label %R7, label %R8

; <label>:8                                       ; preds = %R9
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 1), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 2), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 3), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 4), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 5), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 6), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 7), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 8), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 9), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 10), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 11), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 12), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 13), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 14), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 15), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 16), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 17), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 18), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 19), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 20), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 21), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 22), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 23), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 24), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 25), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 26), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 27), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 28), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 29), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 30), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 31), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 32), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 33), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 34), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 35), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 36), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 37), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 38), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 39), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 40), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 41), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 42), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 43), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 44), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 45), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 46), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 47), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 48), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 49), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 50), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 51), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 52), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 53), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 54), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 55), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 56), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 57), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 58), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 59), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 60), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 61), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 62), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 63), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 64), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 65), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 66), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 67), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 68), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 69), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 70), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 71), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 72), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 73), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 74), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 75), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 76), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 77), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 78), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 79), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 80), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 81), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 82), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 83), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 84), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 85), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 86), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 87), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 88), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 89), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 90), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 91), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 92), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 93), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 94), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 95), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 96), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 97), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 98), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 99), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 100), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 101), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 102), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 103), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 104), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 105), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 106), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 107), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 108), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 109), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 110), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 111), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 112), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 113), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 114), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 115), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 116), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 117), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 118), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 119), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 120), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 121), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 122), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 123), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 124), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 125), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 126), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 127), align 4
  br label %R8

; <label>:9                                       ; preds = %R7, %R9
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R10 = call i32 @dx.op.binary.i32(i32 40, i32 %R0, i32 127)  ; UMin(a,b)
  %R11 = getelementptr [128 x i32], [128 x i32] addrspace(3)* @"\01?arr_i32_@@3PAHA", i32 0, i32 %R10
  %R12 = load i32, i32 addrspace(3)* %R11, align 4, !tbaa !M0
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R13 = icmp sgt i32 %R12, 10
  br i1 %R13, label %R14, label %R15

; <label>:14                                      ; preds = %R8
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  br label %R15

; <label>:15                                      ; preds = %R14, %R8
  ret void
}

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.binary.i32(i32, i32, i32) #A1

; Function Attrs: nounwind readnone
declare i32 @dx.op.groupId.i32(i32, i32) #A1

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A1

attributes #A0 = { noduplicate nounwind }
attributes #A1 = { nounwind readnone }

!llvm.ident = !{!M1}
!dx.version = !{!M2}
!dx.valver = !{!M3}
!dx.shaderModel = !{!M4}
!dx.entryPoints = !{!M5}

!M1 = !{!"<ident>"}
!M2 = !{i32 1, i32 0}
!M3 = !{i32 1, i32 8}
!M4 = !{!"cs", i32 6, i32 0}
!M5 = !{void ()* @test_workgroupUniformLoad, !"test_workgroupUniformLoad", null, null, !M6}
!M6 = !{i32 4, !M7}
!M7 = !{i32 4, i32 1, i32 1}
!M0 = !{!M8, !M8, i64 0}
!M8 = !{!"int", !M9, i64 0}
!M9 = !{!"omnipotent char", !M10, i64 0}
!M10 = !{!"<ident>"}

