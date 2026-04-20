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
; SV_Position              0   xyzw        0      POS   float   xyzw
;
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Vertex Shader
; OutputPositionPresent=1
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 0
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vertex
;
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Position              0          noperspective
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; input1_                           cbuffer      NA          NA     CB0            cb0     1
; input2_                           cbuffer      NA          NA     CB1            cb1     1
; input3_                           cbuffer      NA          NA     CB2            cb2     1
;
;
; ViewId state:
;
; Number of inputs: 0, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%input1_ = type { %struct.S0 }
%struct.S0 = type { %struct.S1, float, i32, i32, i32 }
%struct.S1 = type { <3 x float>, i32 }
%input2_ = type { %struct.S2 }
%struct.S2 = type { [2 x <3 x float>], i32, float, i32, i32, i32 }
%hostlayout.input3_ = type { %hostlayout.struct.Test3_ }
%hostlayout.struct.Test3_ = type { [4 x <3 x float>], i32, float, i32, i32, i32 }

define void @vertex() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 2, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R4 = extractvalue %dx.types.CBufRet.f32 %R3, 0
  %R5 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R1, i32 2)  ; CBufferLoadLegacy(handle,regIndex)
  %R6 = extractvalue %dx.types.CBufRet.f32 %R5, 0
  %R7 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R8 = extractvalue %dx.types.CBufRet.f32 %R7, 0
  %R9 = fmul fast float %R6, %R4
  %R10 = fmul fast float %R9, %R8
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R10)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R10)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R10)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R10)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

attributes #A0 = { nounwind }
attributes #A1 = { nounwind readonly }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.viewIdState = !{!M5}
!dx.entryPoints = !{!M6}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 8}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{null, null, !M7, null}
!M7 = !{!M8, !M9, !M10}
!M8 = !{i32 0, %input1_* undef, !"", i32 0, i32 0, i32 1, i32 32, null}
!M9 = !{i32 1, %input2_* undef, !"", i32 0, i32 1, i32 1, i32 48, null}
!M10 = !{i32 2, %hostlayout.input3_* undef, !"", i32 0, i32 2, i32 1, i32 80, null}
!M5 = !{[2 x i32] [i32 0, i32 4]}
!M6 = !{void ()* @vertex, !"vertex", !M11, !M4, null}
!M11 = !{null, !M12, null}
!M12 = !{!M13}
!M13 = !{i32 0, !"SV_Position", i8 9, i8 3, !M14, i8 4, i32 1, i8 4, i32 0, i8 0, !M15}
!M14 = !{i32 0}
!M15 = !{i32 3, i32 15}

