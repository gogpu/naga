;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; LOC                      1   xyz         1     NONE   float   xyz
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; LOC                      1   xyz         1     NONE   float   xyz
; SV_Position              0   xyzw        2      POS   float   xyzw
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
; SigInputElements: 2
; SigOutputElements: 3
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 2
; SigOutputVectors[0]: 3
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vs_main
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0
; LOC                      1
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0                 linear
; LOC                      1                 linear
; SV_Position              0          noperspective
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; camera                            cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 7, outputs: 12
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 4 }
;   output 1 depends on inputs: { 5 }
;   output 2 depends on inputs: { 6 }
;   output 4 depends on inputs: { 0 }
;   output 5 depends on inputs: { 1 }
;   output 6 depends on inputs: { 2 }
;   output 8 depends on inputs: { 0, 1, 2 }
;   output 9 depends on inputs: { 0, 1, 2 }
;   output 10 depends on inputs: { 0, 1, 2 }
;   output 11 depends on inputs: { 0, 1, 2 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%hostlayout.camera = type { %hostlayout.struct.Camera }
%hostlayout.struct.Camera = type { <4 x float>, [4 x <4 x float>] }

define void @vs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R5 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R6 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R7 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R8 = extractvalue %dx.types.CBufRet.f32 %R7, 0
  %R9 = extractvalue %dx.types.CBufRet.f32 %R7, 1
  %R10 = extractvalue %dx.types.CBufRet.f32 %R7, 2
  %R11 = extractvalue %dx.types.CBufRet.f32 %R7, 3
  %R12 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 2)  ; CBufferLoadLegacy(handle,regIndex)
  %R13 = extractvalue %dx.types.CBufRet.f32 %R12, 0
  %R14 = extractvalue %dx.types.CBufRet.f32 %R12, 1
  %R15 = extractvalue %dx.types.CBufRet.f32 %R12, 2
  %R16 = extractvalue %dx.types.CBufRet.f32 %R12, 3
  %R17 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 3)  ; CBufferLoadLegacy(handle,regIndex)
  %R18 = extractvalue %dx.types.CBufRet.f32 %R17, 0
  %R19 = extractvalue %dx.types.CBufRet.f32 %R17, 1
  %R20 = extractvalue %dx.types.CBufRet.f32 %R17, 2
  %R21 = extractvalue %dx.types.CBufRet.f32 %R17, 3
  %R22 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R23 = extractvalue %dx.types.CBufRet.f32 %R22, 0
  %R24 = extractvalue %dx.types.CBufRet.f32 %R22, 1
  %R25 = extractvalue %dx.types.CBufRet.f32 %R22, 2
  %R26 = extractvalue %dx.types.CBufRet.f32 %R22, 3
  %R27 = fmul fast float %R8, %R4
  %R28 = call float @dx.op.tertiary.f32(i32 46, float %R5, float %R13, float %R27)  ; FMad(a,b,c)
  %R29 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R18, float %R28)  ; FMad(a,b,c)
  %R30 = fadd fast float %R29, %R23
  %R31 = fmul fast float %R9, %R4
  %R32 = call float @dx.op.tertiary.f32(i32 46, float %R5, float %R14, float %R31)  ; FMad(a,b,c)
  %R33 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R19, float %R32)  ; FMad(a,b,c)
  %R34 = fadd fast float %R33, %R24
  %R35 = fmul fast float %R10, %R4
  %R36 = call float @dx.op.tertiary.f32(i32 46, float %R5, float %R15, float %R35)  ; FMad(a,b,c)
  %R37 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R20, float %R36)  ; FMad(a,b,c)
  %R38 = fadd fast float %R37, %R25
  %R39 = fmul fast float %R11, %R4
  %R40 = call float @dx.op.tertiary.f32(i32 46, float %R5, float %R16, float %R39)  ; FMad(a,b,c)
  %R41 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R21, float %R40)  ; FMad(a,b,c)
  %R42 = fadd fast float %R41, %R26
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R2)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R3)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float %R4)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 1, float %R5)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 2, float %R6)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float %R30)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 1, float %R34)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 2, float %R38)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 3, float %R42)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A2

; Function Attrs: nounwind readnone
declare float @dx.op.tertiary.f32(i32, float, float, float) #A1

attributes #A0 = { nounwind readonly }
attributes #A1 = { nounwind readnone }
attributes #A2 = { nounwind }

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
!M7 = !{!M8}
!M8 = !{i32 0, %hostlayout.camera* undef, !"", i32 0, i32 0, i32 1, i32 80, null}
!M5 = !{[9 x i32] [i32 7, i32 12, i32 3856, i32 3872, i32 3904, i32 0, i32 1, i32 2, i32 4]}
!M6 = !{void ()* @vs_main, !"vs_main", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12, !M13}
!M12 = !{i32 0, !"LOC", i8 9, i8 0, !M14, i8 0, i32 1, i8 3, i32 0, i8 0, !M15}
!M14 = !{i32 0}
!M15 = !{i32 3, i32 7}
!M13 = !{i32 1, !"LOC", i8 9, i8 0, !M16, i8 0, i32 1, i8 3, i32 1, i8 0, !M15}
!M16 = !{i32 1}
!M11 = !{!M17, !M18, !M19}
!M17 = !{i32 0, !"LOC", i8 9, i8 0, !M14, i8 2, i32 1, i8 3, i32 0, i8 0, !M15}
!M18 = !{i32 1, !"LOC", i8 9, i8 0, !M16, i8 2, i32 1, i8 3, i32 1, i8 0, !M15}
!M19 = !{i32 2, !"SV_Position", i8 9, i8 3, !M14, i8 4, i32 1, i8 4, i32 2, i8 0, !M20}
!M20 = !{i32 3, i32 15}

