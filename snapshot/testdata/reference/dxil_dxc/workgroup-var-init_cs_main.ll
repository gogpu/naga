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
; output                                UAV    byte         r/w      U0             u0     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%struct.S0 = type { i32 }

@"\01?w_mem@@3UWStruct@@A.0" = addrspace(3) global [512 x i32] undef, align 4
@"\01?w_mem@@3UWStruct@@A.2.1dim" = addrspace(3) global [64 x i32] undef, align 4

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R2 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R3 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R4 = or i32 %R2, %R1
  %R5 = or i32 %R4, %R3
  %R6 = icmp eq i32 %R5, 0
  br i1 %R6, label %R7, label %R8

; <label>:8                                       ; preds = %R9
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 1), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 2), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 3), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 4), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 5), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 6), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 7), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 8), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 9), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 10), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 11), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 12), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 13), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 14), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 15), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 16), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 17), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 18), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 19), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 20), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 21), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 22), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 23), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 24), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 25), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 26), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 27), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 28), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 29), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 30), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 31), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 32), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 33), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 34), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 35), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 36), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 37), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 38), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 39), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 40), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 41), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 42), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 43), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 44), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 45), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 46), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 47), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 48), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 49), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 50), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 51), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 52), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 53), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 54), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 55), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 56), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 57), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 58), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 59), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 60), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 61), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 62), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 63), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 64), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 65), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 66), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 67), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 68), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 69), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 70), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 71), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 72), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 73), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 74), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 75), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 76), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 77), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 78), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 79), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 80), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 81), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 82), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 83), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 84), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 85), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 86), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 87), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 88), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 89), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 90), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 91), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 92), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 93), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 94), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 95), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 96), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 97), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 98), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 99), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 100), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 101), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 102), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 103), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 104), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 105), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 106), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 107), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 108), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 109), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 110), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 111), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 112), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 113), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 114), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 115), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 116), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 117), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 118), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 119), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 120), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 121), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 122), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 123), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 124), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 125), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 126), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 127), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 128), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 129), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 130), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 131), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 132), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 133), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 134), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 135), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 136), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 137), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 138), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 139), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 140), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 141), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 142), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 143), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 144), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 145), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 146), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 147), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 148), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 149), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 150), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 151), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 152), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 153), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 154), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 155), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 156), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 157), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 158), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 159), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 160), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 161), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 162), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 163), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 164), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 165), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 166), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 167), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 168), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 169), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 170), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 171), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 172), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 173), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 174), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 175), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 176), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 177), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 178), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 179), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 180), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 181), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 182), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 183), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 184), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 185), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 186), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 187), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 188), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 189), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 190), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 191), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 192), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 193), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 194), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 195), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 196), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 197), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 198), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 199), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 200), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 201), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 202), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 203), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 204), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 205), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 206), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 207), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 208), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 209), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 210), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 211), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 212), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 213), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 214), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 215), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 216), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 217), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 218), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 219), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 220), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 221), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 222), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 223), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 224), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 225), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 226), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 227), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 228), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 229), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 230), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 231), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 232), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 233), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 234), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 235), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 236), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 237), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 238), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 239), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 240), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 241), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 242), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 243), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 244), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 245), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 246), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 247), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 248), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 249), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 250), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 251), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 252), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 253), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 254), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 255), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 256), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 257), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 258), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 259), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 260), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 261), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 262), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 263), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 264), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 265), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 266), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 267), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 268), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 269), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 270), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 271), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 272), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 273), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 274), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 275), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 276), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 277), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 278), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 279), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 280), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 281), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 282), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 283), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 284), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 285), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 286), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 287), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 288), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 289), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 290), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 291), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 292), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 293), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 294), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 295), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 296), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 297), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 298), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 299), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 300), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 301), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 302), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 303), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 304), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 305), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 306), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 307), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 308), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 309), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 310), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 311), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 312), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 313), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 314), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 315), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 316), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 317), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 318), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 319), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 320), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 321), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 322), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 323), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 324), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 325), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 326), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 327), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 328), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 329), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 330), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 331), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 332), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 333), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 334), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 335), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 336), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 337), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 338), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 339), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 340), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 341), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 342), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 343), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 344), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 345), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 346), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 347), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 348), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 349), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 350), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 351), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 352), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 353), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 354), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 355), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 356), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 357), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 358), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 359), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 360), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 361), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 362), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 363), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 364), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 365), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 366), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 367), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 368), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 369), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 370), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 371), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 372), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 373), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 374), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 375), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 376), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 377), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 378), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 379), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 380), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 381), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 382), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 383), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 384), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 385), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 386), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 387), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 388), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 389), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 390), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 391), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 392), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 393), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 394), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 395), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 396), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 397), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 398), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 399), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 400), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 401), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 402), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 403), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 404), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 405), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 406), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 407), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 408), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 409), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 410), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 411), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 412), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 413), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 414), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 415), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 416), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 417), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 418), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 419), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 420), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 421), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 422), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 423), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 424), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 425), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 426), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 427), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 428), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 429), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 430), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 431), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 432), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 433), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 434), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 435), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 436), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 437), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 438), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 439), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 440), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 441), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 442), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 443), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 444), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 445), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 446), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 447), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 448), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 449), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 450), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 451), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 452), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 453), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 454), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 455), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 456), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 457), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 458), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 459), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 460), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 461), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 462), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 463), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 464), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 465), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 466), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 467), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 468), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 469), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 470), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 471), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 472), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 473), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 474), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 475), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 476), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 477), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 478), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 479), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 480), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 481), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 482), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 483), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 484), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 485), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 486), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 487), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 488), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 489), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 490), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 491), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 492), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 493), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 494), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 495), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 496), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 497), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 498), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 499), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 500), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 501), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 502), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 503), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 504), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 505), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 506), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 507), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 508), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 509), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 510), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 511), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 1), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 2), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 3), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 4), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 5), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 6), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 7), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 8), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 9), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 10), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 11), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 12), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 13), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 14), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 15), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 16), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 17), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 18), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 19), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 20), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 21), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 22), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 23), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 24), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 25), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 26), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 27), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 28), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 29), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 30), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 31), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 32), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 33), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 34), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 35), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 36), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 37), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 38), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 39), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 40), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 41), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 42), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 43), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 44), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 45), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 46), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 47), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 48), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 49), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 50), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 51), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 52), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 53), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 54), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 55), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 56), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 57), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 58), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 59), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 60), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 61), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 62), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([64 x i32], [64 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.2.1dim", i32 0, i32 63), align 4
  br label %R8

; <label>:9                                       ; preds = %R7, %R9
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R10 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 0), align 4
  %R11 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 1), align 4
  %R12 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 2), align 4
  %R13 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 3), align 4
  %R14 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 4), align 4
  %R15 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 5), align 4
  %R16 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 6), align 4
  %R17 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 7), align 4
  %R18 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 8), align 4
  %R19 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 9), align 4
  %R20 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 10), align 4
  %R21 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 11), align 4
  %R22 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 12), align 4
  %R23 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 13), align 4
  %R24 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 14), align 4
  %R25 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 15), align 4
  %R26 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 16), align 4
  %R27 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 17), align 4
  %R28 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 18), align 4
  %R29 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 19), align 4
  %R30 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 20), align 4
  %R31 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 21), align 4
  %R32 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 22), align 4
  %R33 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 23), align 4
  %R34 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 24), align 4
  %R35 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 25), align 4
  %R36 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 26), align 4
  %R37 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 27), align 4
  %R38 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 28), align 4
  %R39 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 29), align 4
  %R40 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 30), align 4
  %R41 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 31), align 4
  %R42 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 32), align 4
  %R43 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 33), align 4
  %R44 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 34), align 4
  %R45 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 35), align 4
  %R46 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 36), align 4
  %R47 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 37), align 4
  %R48 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 38), align 4
  %R49 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 39), align 4
  %R50 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 40), align 4
  %R51 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 41), align 4
  %R52 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 42), align 4
  %R53 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 43), align 4
  %R54 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 44), align 4
  %R55 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 45), align 4
  %R56 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 46), align 4
  %R57 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 47), align 4
  %R58 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 48), align 4
  %R59 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 49), align 4
  %R60 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 50), align 4
  %R61 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 51), align 4
  %R62 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 52), align 4
  %R63 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 53), align 4
  %R64 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 54), align 4
  %R65 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 55), align 4
  %R66 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 56), align 4
  %R67 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 57), align 4
  %R68 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 58), align 4
  %R69 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 59), align 4
  %R70 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 60), align 4
  %R71 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 61), align 4
  %R72 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 62), align 4
  %R73 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 63), align 4
  %R74 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 64), align 4
  %R75 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 65), align 4
  %R76 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 66), align 4
  %R77 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 67), align 4
  %R78 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 68), align 4
  %R79 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 69), align 4
  %R80 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 70), align 4
  %R81 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 71), align 4
  %R82 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 72), align 4
  %R83 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 73), align 4
  %R84 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 74), align 4
  %R85 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 75), align 4
  %R86 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 76), align 4
  %R87 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 77), align 4
  %R88 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 78), align 4
  %R89 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 79), align 4
  %R90 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 80), align 4
  %R91 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 81), align 4
  %R92 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 82), align 4
  %R93 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 83), align 4
  %R94 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 84), align 4
  %R95 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 85), align 4
  %R96 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 86), align 4
  %R97 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 87), align 4
  %R98 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 88), align 4
  %R99 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 89), align 4
  %R100 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 90), align 4
  %R101 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 91), align 4
  %R102 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 92), align 4
  %R103 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 93), align 4
  %R104 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 94), align 4
  %R105 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 95), align 4
  %R106 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 96), align 4
  %R107 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 97), align 4
  %R108 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 98), align 4
  %R109 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 99), align 4
  %R110 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 100), align 4
  %R111 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 101), align 4
  %R112 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 102), align 4
  %R113 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 103), align 4
  %R114 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 104), align 4
  %R115 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 105), align 4
  %R116 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 106), align 4
  %R117 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 107), align 4
  %R118 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 108), align 4
  %R119 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 109), align 4
  %R120 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 110), align 4
  %R121 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 111), align 4
  %R122 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 112), align 4
  %R123 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 113), align 4
  %R124 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 114), align 4
  %R125 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 115), align 4
  %R126 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 116), align 4
  %R127 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 117), align 4
  %R128 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 118), align 4
  %R129 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 119), align 4
  %R130 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 120), align 4
  %R131 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 121), align 4
  %R132 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 122), align 4
  %R133 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 123), align 4
  %R134 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 124), align 4
  %R135 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 125), align 4
  %R136 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 126), align 4
  %R137 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 127), align 4
  %R138 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 128), align 4
  %R139 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 129), align 4
  %R140 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 130), align 4
  %R141 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 131), align 4
  %R142 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 132), align 4
  %R143 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 133), align 4
  %R144 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 134), align 4
  %R145 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 135), align 4
  %R146 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 136), align 4
  %R147 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 137), align 4
  %R148 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 138), align 4
  %R149 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 139), align 4
  %R150 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 140), align 4
  %R151 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 141), align 4
  %R152 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 142), align 4
  %R153 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 143), align 4
  %R154 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 144), align 4
  %R155 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 145), align 4
  %R156 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 146), align 4
  %R157 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 147), align 4
  %R158 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 148), align 4
  %R159 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 149), align 4
  %R160 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 150), align 4
  %R161 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 151), align 4
  %R162 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 152), align 4
  %R163 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 153), align 4
  %R164 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 154), align 4
  %R165 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 155), align 4
  %R166 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 156), align 4
  %R167 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 157), align 4
  %R168 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 158), align 4
  %R169 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 159), align 4
  %R170 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 160), align 4
  %R171 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 161), align 4
  %R172 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 162), align 4
  %R173 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 163), align 4
  %R174 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 164), align 4
  %R175 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 165), align 4
  %R176 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 166), align 4
  %R177 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 167), align 4
  %R178 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 168), align 4
  %R179 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 169), align 4
  %R180 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 170), align 4
  %R181 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 171), align 4
  %R182 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 172), align 4
  %R183 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 173), align 4
  %R184 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 174), align 4
  %R185 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 175), align 4
  %R186 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 176), align 4
  %R187 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 177), align 4
  %R188 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 178), align 4
  %R189 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 179), align 4
  %R190 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 180), align 4
  %R191 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 181), align 4
  %R192 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 182), align 4
  %R193 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 183), align 4
  %R194 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 184), align 4
  %R195 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 185), align 4
  %R196 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 186), align 4
  %R197 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 187), align 4
  %R198 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 188), align 4
  %R199 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 189), align 4
  %R200 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 190), align 4
  %R201 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 191), align 4
  %R202 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 192), align 4
  %R203 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 193), align 4
  %R204 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 194), align 4
  %R205 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 195), align 4
  %R206 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 196), align 4
  %R207 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 197), align 4
  %R208 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 198), align 4
  %R209 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 199), align 4
  %R210 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 200), align 4
  %R211 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 201), align 4
  %R212 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 202), align 4
  %R213 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 203), align 4
  %R214 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 204), align 4
  %R215 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 205), align 4
  %R216 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 206), align 4
  %R217 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 207), align 4
  %R218 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 208), align 4
  %R219 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 209), align 4
  %R220 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 210), align 4
  %R221 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 211), align 4
  %R222 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 212), align 4
  %R223 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 213), align 4
  %R224 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 214), align 4
  %R225 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 215), align 4
  %R226 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 216), align 4
  %R227 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 217), align 4
  %R228 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 218), align 4
  %R229 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 219), align 4
  %R230 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 220), align 4
  %R231 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 221), align 4
  %R232 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 222), align 4
  %R233 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 223), align 4
  %R234 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 224), align 4
  %R235 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 225), align 4
  %R236 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 226), align 4
  %R237 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 227), align 4
  %R238 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 228), align 4
  %R239 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 229), align 4
  %R240 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 230), align 4
  %R241 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 231), align 4
  %R242 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 232), align 4
  %R243 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 233), align 4
  %R244 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 234), align 4
  %R245 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 235), align 4
  %R246 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 236), align 4
  %R247 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 237), align 4
  %R248 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 238), align 4
  %R249 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 239), align 4
  %R250 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 240), align 4
  %R251 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 241), align 4
  %R252 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 242), align 4
  %R253 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 243), align 4
  %R254 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 244), align 4
  %R255 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 245), align 4
  %R256 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 246), align 4
  %R257 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 247), align 4
  %R258 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 248), align 4
  %R259 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 249), align 4
  %R260 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 250), align 4
  %R261 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 251), align 4
  %R262 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 252), align 4
  %R263 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 253), align 4
  %R264 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 254), align 4
  %R265 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 255), align 4
  %R266 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 256), align 4
  %R267 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 257), align 4
  %R268 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 258), align 4
  %R269 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 259), align 4
  %R270 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 260), align 4
  %R271 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 261), align 4
  %R272 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 262), align 4
  %R273 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 263), align 4
  %R274 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 264), align 4
  %R275 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 265), align 4
  %R276 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 266), align 4
  %R277 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 267), align 4
  %R278 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 268), align 4
  %R279 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 269), align 4
  %R280 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 270), align 4
  %R281 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 271), align 4
  %R282 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 272), align 4
  %R283 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 273), align 4
  %R284 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 274), align 4
  %R285 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 275), align 4
  %R286 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 276), align 4
  %R287 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 277), align 4
  %R288 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 278), align 4
  %R289 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 279), align 4
  %R290 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 280), align 4
  %R291 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 281), align 4
  %R292 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 282), align 4
  %R293 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 283), align 4
  %R294 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 284), align 4
  %R295 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 285), align 4
  %R296 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 286), align 4
  %R297 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 287), align 4
  %R298 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 288), align 4
  %R299 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 289), align 4
  %R300 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 290), align 4
  %R301 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 291), align 4
  %R302 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 292), align 4
  %R303 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 293), align 4
  %R304 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 294), align 4
  %R305 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 295), align 4
  %R306 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 296), align 4
  %R307 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 297), align 4
  %R308 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 298), align 4
  %R309 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 299), align 4
  %R310 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 300), align 4
  %R311 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 301), align 4
  %R312 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 302), align 4
  %R313 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 303), align 4
  %R314 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 304), align 4
  %R315 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 305), align 4
  %R316 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 306), align 4
  %R317 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 307), align 4
  %R318 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 308), align 4
  %R319 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 309), align 4
  %R320 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 310), align 4
  %R321 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 311), align 4
  %R322 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 312), align 4
  %R323 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 313), align 4
  %R324 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 314), align 4
  %R325 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 315), align 4
  %R326 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 316), align 4
  %R327 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 317), align 4
  %R328 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 318), align 4
  %R329 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 319), align 4
  %R330 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 320), align 4
  %R331 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 321), align 4
  %R332 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 322), align 4
  %R333 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 323), align 4
  %R334 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 324), align 4
  %R335 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 325), align 4
  %R336 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 326), align 4
  %R337 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 327), align 4
  %R338 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 328), align 4
  %R339 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 329), align 4
  %R340 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 330), align 4
  %R341 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 331), align 4
  %R342 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 332), align 4
  %R343 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 333), align 4
  %R344 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 334), align 4
  %R345 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 335), align 4
  %R346 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 336), align 4
  %R347 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 337), align 4
  %R348 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 338), align 4
  %R349 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 339), align 4
  %R350 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 340), align 4
  %R351 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 341), align 4
  %R352 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 342), align 4
  %R353 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 343), align 4
  %R354 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 344), align 4
  %R355 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 345), align 4
  %R356 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 346), align 4
  %R357 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 347), align 4
  %R358 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 348), align 4
  %R359 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 349), align 4
  %R360 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 350), align 4
  %R361 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 351), align 4
  %R362 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 352), align 4
  %R363 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 353), align 4
  %R364 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 354), align 4
  %R365 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 355), align 4
  %R366 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 356), align 4
  %R367 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 357), align 4
  %R368 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 358), align 4
  %R369 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 359), align 4
  %R370 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 360), align 4
  %R371 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 361), align 4
  %R372 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 362), align 4
  %R373 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 363), align 4
  %R374 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 364), align 4
  %R375 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 365), align 4
  %R376 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 366), align 4
  %R377 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 367), align 4
  %R378 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 368), align 4
  %R379 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 369), align 4
  %R380 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 370), align 4
  %R381 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 371), align 4
  %R382 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 372), align 4
  %R383 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 373), align 4
  %R384 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 374), align 4
  %R385 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 375), align 4
  %R386 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 376), align 4
  %R387 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 377), align 4
  %R388 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 378), align 4
  %R389 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 379), align 4
  %R390 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 380), align 4
  %R391 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 381), align 4
  %R392 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 382), align 4
  %R393 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 383), align 4
  %R394 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 384), align 4
  %R395 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 385), align 4
  %R396 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 386), align 4
  %R397 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 387), align 4
  %R398 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 388), align 4
  %R399 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 389), align 4
  %R400 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 390), align 4
  %R401 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 391), align 4
  %R402 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 392), align 4
  %R403 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 393), align 4
  %R404 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 394), align 4
  %R405 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 395), align 4
  %R406 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 396), align 4
  %R407 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 397), align 4
  %R408 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 398), align 4
  %R409 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 399), align 4
  %R410 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 400), align 4
  %R411 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 401), align 4
  %R412 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 402), align 4
  %R413 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 403), align 4
  %R414 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 404), align 4
  %R415 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 405), align 4
  %R416 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 406), align 4
  %R417 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 407), align 4
  %R418 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 408), align 4
  %R419 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 409), align 4
  %R420 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 410), align 4
  %R421 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 411), align 4
  %R422 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 412), align 4
  %R423 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 413), align 4
  %R424 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 414), align 4
  %R425 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 415), align 4
  %R426 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 416), align 4
  %R427 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 417), align 4
  %R428 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 418), align 4
  %R429 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 419), align 4
  %R430 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 420), align 4
  %R431 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 421), align 4
  %R432 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 422), align 4
  %R433 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 423), align 4
  %R434 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 424), align 4
  %R435 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 425), align 4
  %R436 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 426), align 4
  %R437 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 427), align 4
  %R438 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 428), align 4
  %R439 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 429), align 4
  %R440 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 430), align 4
  %R441 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 431), align 4
  %R442 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 432), align 4
  %R443 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 433), align 4
  %R444 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 434), align 4
  %R445 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 435), align 4
  %R446 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 436), align 4
  %R447 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 437), align 4
  %R448 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 438), align 4
  %R449 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 439), align 4
  %R450 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 440), align 4
  %R451 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 441), align 4
  %R452 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 442), align 4
  %R453 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 443), align 4
  %R454 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 444), align 4
  %R455 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 445), align 4
  %R456 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 446), align 4
  %R457 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 447), align 4
  %R458 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 448), align 4
  %R459 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 449), align 4
  %R460 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 450), align 4
  %R461 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 451), align 4
  %R462 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 452), align 4
  %R463 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 453), align 4
  %R464 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 454), align 4
  %R465 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 455), align 4
  %R466 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 456), align 4
  %R467 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 457), align 4
  %R468 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 458), align 4
  %R469 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 459), align 4
  %R470 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 460), align 4
  %R471 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 461), align 4
  %R472 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 462), align 4
  %R473 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 463), align 4
  %R474 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 464), align 4
  %R475 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 465), align 4
  %R476 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 466), align 4
  %R477 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 467), align 4
  %R478 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 468), align 4
  %R479 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 469), align 4
  %R480 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 470), align 4
  %R481 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 471), align 4
  %R482 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 472), align 4
  %R483 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 473), align 4
  %R484 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 474), align 4
  %R485 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 475), align 4
  %R486 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 476), align 4
  %R487 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 477), align 4
  %R488 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 478), align 4
  %R489 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 479), align 4
  %R490 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 480), align 4
  %R491 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 481), align 4
  %R492 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 482), align 4
  %R493 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 483), align 4
  %R494 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 484), align 4
  %R495 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 485), align 4
  %R496 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 486), align 4
  %R497 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 487), align 4
  %R498 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 488), align 4
  %R499 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 489), align 4
  %R500 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 490), align 4
  %R501 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 491), align 4
  %R502 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 492), align 4
  %R503 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 493), align 4
  %R504 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 494), align 4
  %R505 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 495), align 4
  %R506 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 496), align 4
  %R507 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 497), align 4
  %R508 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 498), align 4
  %R509 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 499), align 4
  %R510 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 500), align 4
  %R511 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 501), align 4
  %R512 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 502), align 4
  %R513 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 503), align 4
  %R514 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 504), align 4
  %R515 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 505), align 4
  %R516 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 506), align 4
  %R517 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 507), align 4
  %R518 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 508), align 4
  %R519 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 509), align 4
  %R520 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 510), align 4
  %R521 = load i32, i32 addrspace(3)* getelementptr inbounds ([512 x i32], [512 x i32] addrspace(3)* @"\01?w_mem@@3UWStruct@@A.0", i32 0, i32 511), align 4
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 0, i32 undef, i32 %R10, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 4, i32 undef, i32 %R11, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 8, i32 undef, i32 %R12, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 12, i32 undef, i32 %R13, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 16, i32 undef, i32 %R14, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 20, i32 undef, i32 %R15, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 24, i32 undef, i32 %R16, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 28, i32 undef, i32 %R17, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 32, i32 undef, i32 %R18, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 36, i32 undef, i32 %R19, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 40, i32 undef, i32 %R20, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 44, i32 undef, i32 %R21, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 48, i32 undef, i32 %R22, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 52, i32 undef, i32 %R23, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 56, i32 undef, i32 %R24, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 60, i32 undef, i32 %R25, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 64, i32 undef, i32 %R26, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 68, i32 undef, i32 %R27, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 72, i32 undef, i32 %R28, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 76, i32 undef, i32 %R29, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 80, i32 undef, i32 %R30, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 84, i32 undef, i32 %R31, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 88, i32 undef, i32 %R32, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 92, i32 undef, i32 %R33, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 96, i32 undef, i32 %R34, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 100, i32 undef, i32 %R35, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 104, i32 undef, i32 %R36, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 108, i32 undef, i32 %R37, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 112, i32 undef, i32 %R38, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 116, i32 undef, i32 %R39, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 120, i32 undef, i32 %R40, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 124, i32 undef, i32 %R41, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 128, i32 undef, i32 %R42, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 132, i32 undef, i32 %R43, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 136, i32 undef, i32 %R44, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 140, i32 undef, i32 %R45, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 144, i32 undef, i32 %R46, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 148, i32 undef, i32 %R47, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 152, i32 undef, i32 %R48, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 156, i32 undef, i32 %R49, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 160, i32 undef, i32 %R50, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 164, i32 undef, i32 %R51, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 168, i32 undef, i32 %R52, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 172, i32 undef, i32 %R53, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 176, i32 undef, i32 %R54, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 180, i32 undef, i32 %R55, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 184, i32 undef, i32 %R56, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 188, i32 undef, i32 %R57, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 192, i32 undef, i32 %R58, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 196, i32 undef, i32 %R59, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 200, i32 undef, i32 %R60, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 204, i32 undef, i32 %R61, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 208, i32 undef, i32 %R62, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 212, i32 undef, i32 %R63, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 216, i32 undef, i32 %R64, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 220, i32 undef, i32 %R65, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 224, i32 undef, i32 %R66, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 228, i32 undef, i32 %R67, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 232, i32 undef, i32 %R68, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 236, i32 undef, i32 %R69, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 240, i32 undef, i32 %R70, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 244, i32 undef, i32 %R71, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 248, i32 undef, i32 %R72, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 252, i32 undef, i32 %R73, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 256, i32 undef, i32 %R74, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 260, i32 undef, i32 %R75, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 264, i32 undef, i32 %R76, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 268, i32 undef, i32 %R77, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 272, i32 undef, i32 %R78, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 276, i32 undef, i32 %R79, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 280, i32 undef, i32 %R80, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 284, i32 undef, i32 %R81, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 288, i32 undef, i32 %R82, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 292, i32 undef, i32 %R83, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 296, i32 undef, i32 %R84, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 300, i32 undef, i32 %R85, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 304, i32 undef, i32 %R86, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 308, i32 undef, i32 %R87, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 312, i32 undef, i32 %R88, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 316, i32 undef, i32 %R89, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 320, i32 undef, i32 %R90, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 324, i32 undef, i32 %R91, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 328, i32 undef, i32 %R92, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 332, i32 undef, i32 %R93, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 336, i32 undef, i32 %R94, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 340, i32 undef, i32 %R95, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 344, i32 undef, i32 %R96, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 348, i32 undef, i32 %R97, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 352, i32 undef, i32 %R98, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 356, i32 undef, i32 %R99, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 360, i32 undef, i32 %R100, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 364, i32 undef, i32 %R101, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 368, i32 undef, i32 %R102, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 372, i32 undef, i32 %R103, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 376, i32 undef, i32 %R104, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 380, i32 undef, i32 %R105, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 384, i32 undef, i32 %R106, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 388, i32 undef, i32 %R107, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 392, i32 undef, i32 %R108, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 396, i32 undef, i32 %R109, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 400, i32 undef, i32 %R110, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 404, i32 undef, i32 %R111, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 408, i32 undef, i32 %R112, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 412, i32 undef, i32 %R113, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 416, i32 undef, i32 %R114, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 420, i32 undef, i32 %R115, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 424, i32 undef, i32 %R116, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 428, i32 undef, i32 %R117, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 432, i32 undef, i32 %R118, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 436, i32 undef, i32 %R119, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 440, i32 undef, i32 %R120, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 444, i32 undef, i32 %R121, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 448, i32 undef, i32 %R122, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 452, i32 undef, i32 %R123, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 456, i32 undef, i32 %R124, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 460, i32 undef, i32 %R125, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 464, i32 undef, i32 %R126, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 468, i32 undef, i32 %R127, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 472, i32 undef, i32 %R128, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 476, i32 undef, i32 %R129, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 480, i32 undef, i32 %R130, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 484, i32 undef, i32 %R131, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 488, i32 undef, i32 %R132, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 492, i32 undef, i32 %R133, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 496, i32 undef, i32 %R134, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 500, i32 undef, i32 %R135, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 504, i32 undef, i32 %R136, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 508, i32 undef, i32 %R137, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 512, i32 undef, i32 %R138, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 516, i32 undef, i32 %R139, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 520, i32 undef, i32 %R140, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 524, i32 undef, i32 %R141, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 528, i32 undef, i32 %R142, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 532, i32 undef, i32 %R143, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 536, i32 undef, i32 %R144, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 540, i32 undef, i32 %R145, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 544, i32 undef, i32 %R146, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 548, i32 undef, i32 %R147, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 552, i32 undef, i32 %R148, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 556, i32 undef, i32 %R149, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 560, i32 undef, i32 %R150, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 564, i32 undef, i32 %R151, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 568, i32 undef, i32 %R152, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 572, i32 undef, i32 %R153, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 576, i32 undef, i32 %R154, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 580, i32 undef, i32 %R155, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 584, i32 undef, i32 %R156, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 588, i32 undef, i32 %R157, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 592, i32 undef, i32 %R158, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 596, i32 undef, i32 %R159, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 600, i32 undef, i32 %R160, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 604, i32 undef, i32 %R161, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 608, i32 undef, i32 %R162, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 612, i32 undef, i32 %R163, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 616, i32 undef, i32 %R164, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 620, i32 undef, i32 %R165, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 624, i32 undef, i32 %R166, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 628, i32 undef, i32 %R167, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 632, i32 undef, i32 %R168, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 636, i32 undef, i32 %R169, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 640, i32 undef, i32 %R170, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 644, i32 undef, i32 %R171, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 648, i32 undef, i32 %R172, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 652, i32 undef, i32 %R173, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 656, i32 undef, i32 %R174, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 660, i32 undef, i32 %R175, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 664, i32 undef, i32 %R176, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 668, i32 undef, i32 %R177, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 672, i32 undef, i32 %R178, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 676, i32 undef, i32 %R179, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 680, i32 undef, i32 %R180, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 684, i32 undef, i32 %R181, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 688, i32 undef, i32 %R182, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 692, i32 undef, i32 %R183, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 696, i32 undef, i32 %R184, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 700, i32 undef, i32 %R185, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 704, i32 undef, i32 %R186, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 708, i32 undef, i32 %R187, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 712, i32 undef, i32 %R188, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 716, i32 undef, i32 %R189, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 720, i32 undef, i32 %R190, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 724, i32 undef, i32 %R191, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 728, i32 undef, i32 %R192, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 732, i32 undef, i32 %R193, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 736, i32 undef, i32 %R194, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 740, i32 undef, i32 %R195, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 744, i32 undef, i32 %R196, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 748, i32 undef, i32 %R197, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 752, i32 undef, i32 %R198, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 756, i32 undef, i32 %R199, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 760, i32 undef, i32 %R200, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 764, i32 undef, i32 %R201, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 768, i32 undef, i32 %R202, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 772, i32 undef, i32 %R203, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 776, i32 undef, i32 %R204, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 780, i32 undef, i32 %R205, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 784, i32 undef, i32 %R206, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 788, i32 undef, i32 %R207, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 792, i32 undef, i32 %R208, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 796, i32 undef, i32 %R209, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 800, i32 undef, i32 %R210, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 804, i32 undef, i32 %R211, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 808, i32 undef, i32 %R212, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 812, i32 undef, i32 %R213, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 816, i32 undef, i32 %R214, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 820, i32 undef, i32 %R215, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 824, i32 undef, i32 %R216, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 828, i32 undef, i32 %R217, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 832, i32 undef, i32 %R218, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 836, i32 undef, i32 %R219, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 840, i32 undef, i32 %R220, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 844, i32 undef, i32 %R221, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 848, i32 undef, i32 %R222, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 852, i32 undef, i32 %R223, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 856, i32 undef, i32 %R224, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 860, i32 undef, i32 %R225, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 864, i32 undef, i32 %R226, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 868, i32 undef, i32 %R227, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 872, i32 undef, i32 %R228, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 876, i32 undef, i32 %R229, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 880, i32 undef, i32 %R230, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 884, i32 undef, i32 %R231, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 888, i32 undef, i32 %R232, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 892, i32 undef, i32 %R233, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 896, i32 undef, i32 %R234, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 900, i32 undef, i32 %R235, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 904, i32 undef, i32 %R236, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 908, i32 undef, i32 %R237, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 912, i32 undef, i32 %R238, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 916, i32 undef, i32 %R239, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 920, i32 undef, i32 %R240, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 924, i32 undef, i32 %R241, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 928, i32 undef, i32 %R242, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 932, i32 undef, i32 %R243, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 936, i32 undef, i32 %R244, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 940, i32 undef, i32 %R245, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 944, i32 undef, i32 %R246, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 948, i32 undef, i32 %R247, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 952, i32 undef, i32 %R248, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 956, i32 undef, i32 %R249, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 960, i32 undef, i32 %R250, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 964, i32 undef, i32 %R251, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 968, i32 undef, i32 %R252, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 972, i32 undef, i32 %R253, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 976, i32 undef, i32 %R254, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 980, i32 undef, i32 %R255, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 984, i32 undef, i32 %R256, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 988, i32 undef, i32 %R257, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 992, i32 undef, i32 %R258, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 996, i32 undef, i32 %R259, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1000, i32 undef, i32 %R260, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1004, i32 undef, i32 %R261, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1008, i32 undef, i32 %R262, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1012, i32 undef, i32 %R263, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1016, i32 undef, i32 %R264, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1020, i32 undef, i32 %R265, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1024, i32 undef, i32 %R266, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1028, i32 undef, i32 %R267, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1032, i32 undef, i32 %R268, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1036, i32 undef, i32 %R269, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1040, i32 undef, i32 %R270, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1044, i32 undef, i32 %R271, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1048, i32 undef, i32 %R272, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1052, i32 undef, i32 %R273, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1056, i32 undef, i32 %R274, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1060, i32 undef, i32 %R275, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1064, i32 undef, i32 %R276, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1068, i32 undef, i32 %R277, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1072, i32 undef, i32 %R278, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1076, i32 undef, i32 %R279, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1080, i32 undef, i32 %R280, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1084, i32 undef, i32 %R281, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1088, i32 undef, i32 %R282, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1092, i32 undef, i32 %R283, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1096, i32 undef, i32 %R284, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1100, i32 undef, i32 %R285, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1104, i32 undef, i32 %R286, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1108, i32 undef, i32 %R287, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1112, i32 undef, i32 %R288, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1116, i32 undef, i32 %R289, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1120, i32 undef, i32 %R290, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1124, i32 undef, i32 %R291, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1128, i32 undef, i32 %R292, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1132, i32 undef, i32 %R293, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1136, i32 undef, i32 %R294, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1140, i32 undef, i32 %R295, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1144, i32 undef, i32 %R296, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1148, i32 undef, i32 %R297, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1152, i32 undef, i32 %R298, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1156, i32 undef, i32 %R299, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1160, i32 undef, i32 %R300, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1164, i32 undef, i32 %R301, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1168, i32 undef, i32 %R302, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1172, i32 undef, i32 %R303, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1176, i32 undef, i32 %R304, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1180, i32 undef, i32 %R305, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1184, i32 undef, i32 %R306, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1188, i32 undef, i32 %R307, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1192, i32 undef, i32 %R308, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1196, i32 undef, i32 %R309, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1200, i32 undef, i32 %R310, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1204, i32 undef, i32 %R311, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1208, i32 undef, i32 %R312, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1212, i32 undef, i32 %R313, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1216, i32 undef, i32 %R314, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1220, i32 undef, i32 %R315, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1224, i32 undef, i32 %R316, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1228, i32 undef, i32 %R317, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1232, i32 undef, i32 %R318, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1236, i32 undef, i32 %R319, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1240, i32 undef, i32 %R320, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1244, i32 undef, i32 %R321, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1248, i32 undef, i32 %R322, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1252, i32 undef, i32 %R323, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1256, i32 undef, i32 %R324, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1260, i32 undef, i32 %R325, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1264, i32 undef, i32 %R326, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1268, i32 undef, i32 %R327, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1272, i32 undef, i32 %R328, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1276, i32 undef, i32 %R329, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1280, i32 undef, i32 %R330, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1284, i32 undef, i32 %R331, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1288, i32 undef, i32 %R332, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1292, i32 undef, i32 %R333, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1296, i32 undef, i32 %R334, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1300, i32 undef, i32 %R335, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1304, i32 undef, i32 %R336, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1308, i32 undef, i32 %R337, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1312, i32 undef, i32 %R338, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1316, i32 undef, i32 %R339, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1320, i32 undef, i32 %R340, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1324, i32 undef, i32 %R341, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1328, i32 undef, i32 %R342, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1332, i32 undef, i32 %R343, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1336, i32 undef, i32 %R344, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1340, i32 undef, i32 %R345, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1344, i32 undef, i32 %R346, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1348, i32 undef, i32 %R347, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1352, i32 undef, i32 %R348, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1356, i32 undef, i32 %R349, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1360, i32 undef, i32 %R350, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1364, i32 undef, i32 %R351, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1368, i32 undef, i32 %R352, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1372, i32 undef, i32 %R353, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1376, i32 undef, i32 %R354, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1380, i32 undef, i32 %R355, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1384, i32 undef, i32 %R356, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1388, i32 undef, i32 %R357, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1392, i32 undef, i32 %R358, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1396, i32 undef, i32 %R359, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1400, i32 undef, i32 %R360, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1404, i32 undef, i32 %R361, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1408, i32 undef, i32 %R362, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1412, i32 undef, i32 %R363, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1416, i32 undef, i32 %R364, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1420, i32 undef, i32 %R365, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1424, i32 undef, i32 %R366, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1428, i32 undef, i32 %R367, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1432, i32 undef, i32 %R368, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1436, i32 undef, i32 %R369, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1440, i32 undef, i32 %R370, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1444, i32 undef, i32 %R371, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1448, i32 undef, i32 %R372, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1452, i32 undef, i32 %R373, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1456, i32 undef, i32 %R374, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1460, i32 undef, i32 %R375, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1464, i32 undef, i32 %R376, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1468, i32 undef, i32 %R377, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1472, i32 undef, i32 %R378, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1476, i32 undef, i32 %R379, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1480, i32 undef, i32 %R380, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1484, i32 undef, i32 %R381, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1488, i32 undef, i32 %R382, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1492, i32 undef, i32 %R383, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1496, i32 undef, i32 %R384, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1500, i32 undef, i32 %R385, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1504, i32 undef, i32 %R386, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1508, i32 undef, i32 %R387, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1512, i32 undef, i32 %R388, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1516, i32 undef, i32 %R389, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1520, i32 undef, i32 %R390, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1524, i32 undef, i32 %R391, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1528, i32 undef, i32 %R392, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1532, i32 undef, i32 %R393, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1536, i32 undef, i32 %R394, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1540, i32 undef, i32 %R395, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1544, i32 undef, i32 %R396, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1548, i32 undef, i32 %R397, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1552, i32 undef, i32 %R398, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1556, i32 undef, i32 %R399, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1560, i32 undef, i32 %R400, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1564, i32 undef, i32 %R401, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1568, i32 undef, i32 %R402, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1572, i32 undef, i32 %R403, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1576, i32 undef, i32 %R404, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1580, i32 undef, i32 %R405, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1584, i32 undef, i32 %R406, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1588, i32 undef, i32 %R407, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1592, i32 undef, i32 %R408, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1596, i32 undef, i32 %R409, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1600, i32 undef, i32 %R410, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1604, i32 undef, i32 %R411, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1608, i32 undef, i32 %R412, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1612, i32 undef, i32 %R413, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1616, i32 undef, i32 %R414, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1620, i32 undef, i32 %R415, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1624, i32 undef, i32 %R416, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1628, i32 undef, i32 %R417, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1632, i32 undef, i32 %R418, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1636, i32 undef, i32 %R419, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1640, i32 undef, i32 %R420, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1644, i32 undef, i32 %R421, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1648, i32 undef, i32 %R422, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1652, i32 undef, i32 %R423, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1656, i32 undef, i32 %R424, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1660, i32 undef, i32 %R425, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1664, i32 undef, i32 %R426, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1668, i32 undef, i32 %R427, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1672, i32 undef, i32 %R428, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1676, i32 undef, i32 %R429, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1680, i32 undef, i32 %R430, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1684, i32 undef, i32 %R431, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1688, i32 undef, i32 %R432, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1692, i32 undef, i32 %R433, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1696, i32 undef, i32 %R434, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1700, i32 undef, i32 %R435, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1704, i32 undef, i32 %R436, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1708, i32 undef, i32 %R437, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1712, i32 undef, i32 %R438, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1716, i32 undef, i32 %R439, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1720, i32 undef, i32 %R440, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1724, i32 undef, i32 %R441, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1728, i32 undef, i32 %R442, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1732, i32 undef, i32 %R443, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1736, i32 undef, i32 %R444, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1740, i32 undef, i32 %R445, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1744, i32 undef, i32 %R446, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1748, i32 undef, i32 %R447, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1752, i32 undef, i32 %R448, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1756, i32 undef, i32 %R449, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1760, i32 undef, i32 %R450, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1764, i32 undef, i32 %R451, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1768, i32 undef, i32 %R452, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1772, i32 undef, i32 %R453, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1776, i32 undef, i32 %R454, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1780, i32 undef, i32 %R455, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1784, i32 undef, i32 %R456, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1788, i32 undef, i32 %R457, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1792, i32 undef, i32 %R458, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1796, i32 undef, i32 %R459, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1800, i32 undef, i32 %R460, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1804, i32 undef, i32 %R461, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1808, i32 undef, i32 %R462, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1812, i32 undef, i32 %R463, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1816, i32 undef, i32 %R464, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1820, i32 undef, i32 %R465, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1824, i32 undef, i32 %R466, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1828, i32 undef, i32 %R467, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1832, i32 undef, i32 %R468, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1836, i32 undef, i32 %R469, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1840, i32 undef, i32 %R470, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1844, i32 undef, i32 %R471, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1848, i32 undef, i32 %R472, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1852, i32 undef, i32 %R473, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1856, i32 undef, i32 %R474, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1860, i32 undef, i32 %R475, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1864, i32 undef, i32 %R476, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1868, i32 undef, i32 %R477, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1872, i32 undef, i32 %R478, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1876, i32 undef, i32 %R479, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1880, i32 undef, i32 %R480, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1884, i32 undef, i32 %R481, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1888, i32 undef, i32 %R482, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1892, i32 undef, i32 %R483, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1896, i32 undef, i32 %R484, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1900, i32 undef, i32 %R485, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1904, i32 undef, i32 %R486, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1908, i32 undef, i32 %R487, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1912, i32 undef, i32 %R488, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1916, i32 undef, i32 %R489, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1920, i32 undef, i32 %R490, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1924, i32 undef, i32 %R491, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1928, i32 undef, i32 %R492, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1932, i32 undef, i32 %R493, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1936, i32 undef, i32 %R494, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1940, i32 undef, i32 %R495, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1944, i32 undef, i32 %R496, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1948, i32 undef, i32 %R497, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1952, i32 undef, i32 %R498, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1956, i32 undef, i32 %R499, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1960, i32 undef, i32 %R500, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1964, i32 undef, i32 %R501, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1968, i32 undef, i32 %R502, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1972, i32 undef, i32 %R503, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1976, i32 undef, i32 %R504, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1980, i32 undef, i32 %R505, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1984, i32 undef, i32 %R506, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1988, i32 undef, i32 %R507, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1992, i32 undef, i32 %R508, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 1996, i32 undef, i32 %R509, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2000, i32 undef, i32 %R510, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2004, i32 undef, i32 %R511, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2008, i32 undef, i32 %R512, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2012, i32 undef, i32 %R513, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2016, i32 undef, i32 %R514, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2020, i32 undef, i32 %R515, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2024, i32 undef, i32 %R516, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2028, i32 undef, i32 %R517, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2032, i32 undef, i32 %R518, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2036, i32 undef, i32 %R519, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2040, i32 undef, i32 %R520, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 2044, i32 undef, i32 %R521, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A0

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A3

attributes #A0 = { nounwind readnone }
attributes #A1 = { noduplicate nounwind }
attributes #A2 = { nounwind readonly }
attributes #A3 = { nounwind }

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

