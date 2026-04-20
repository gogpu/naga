;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_VertexID              0   x           0   VERTID    uint   x
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; SV_Position              0   xyzw        1      POS   float   xyzw
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
; SigInputElements: 1
; SigOutputElements: 2
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 1
; SigOutputVectors[0]: 2
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
; SV_VertexID              0
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0                 linear
; SV_Position              0          noperspective
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; r_data                            cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 1, outputs: 8
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0 }
;   output 1 depends on inputs: { 0 }
;   output 2 depends on inputs: { 0 }
;   output 4 depends on inputs: { 0 }
;   output 5 depends on inputs: { 0 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%hostlayout.r_data = type { %hostlayout.struct.Data }
%hostlayout.struct.Data = type { [4 x <4 x float>], [4 x <4 x float>] }

define void @vs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = sdiv i32 %R1, 2
  %R3 = and i32 %R1, 1
  %R4 = sitofp i32 %R2 to float
  %R5 = fmul fast float %R4, 4.000000e+00
  %R6 = fadd fast float %R5, -1.000000e+00
  %R7 = sitofp i32 %R3 to float
  %R8 = fmul fast float %R7, 4.000000e+00
  %R9 = fadd fast float %R8, -1.000000e+00
  %R10 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R11 = extractvalue %dx.types.CBufRet.f32 %R10, 0
  %R12 = extractvalue %dx.types.CBufRet.f32 %R10, 1
  %R13 = extractvalue %dx.types.CBufRet.f32 %R10, 2
  %R14 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 5)  ; CBufferLoadLegacy(handle,regIndex)
  %R15 = extractvalue %dx.types.CBufRet.f32 %R14, 0
  %R16 = extractvalue %dx.types.CBufRet.f32 %R14, 1
  %R17 = extractvalue %dx.types.CBufRet.f32 %R14, 2
  %R18 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 6)  ; CBufferLoadLegacy(handle,regIndex)
  %R19 = extractvalue %dx.types.CBufRet.f32 %R18, 0
  %R20 = extractvalue %dx.types.CBufRet.f32 %R18, 1
  %R21 = extractvalue %dx.types.CBufRet.f32 %R18, 2
  %R22 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R23 = extractvalue %dx.types.CBufRet.f32 %R22, 0
  %R24 = extractvalue %dx.types.CBufRet.f32 %R22, 1
  %R25 = extractvalue %dx.types.CBufRet.f32 %R22, 2
  %R26 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R27 = extractvalue %dx.types.CBufRet.f32 %R26, 0
  %R28 = extractvalue %dx.types.CBufRet.f32 %R26, 1
  %R29 = extractvalue %dx.types.CBufRet.f32 %R26, 2
  %R30 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 3)  ; CBufferLoadLegacy(handle,regIndex)
  %R31 = extractvalue %dx.types.CBufRet.f32 %R30, 0
  %R32 = extractvalue %dx.types.CBufRet.f32 %R30, 1
  %R33 = extractvalue %dx.types.CBufRet.f32 %R30, 2
  %R34 = fmul fast float %R23, %R6
  %R35 = call float @dx.op.tertiary.f32(i32 46, float %R9, float %R27, float %R34)  ; FMad(a,b,c)
  %R36 = fadd fast float %R31, %R35
  %R37 = fmul fast float %R24, %R6
  %R38 = call float @dx.op.tertiary.f32(i32 46, float %R9, float %R28, float %R37)  ; FMad(a,b,c)
  %R39 = fadd fast float %R38, %R32
  %R40 = fmul fast float %R25, %R6
  %R41 = call float @dx.op.tertiary.f32(i32 46, float %R9, float %R29, float %R40)  ; FMad(a,b,c)
  %R42 = fadd fast float %R41, %R33
  %R43 = fmul fast float %R36, %R11
  %R44 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R12, float %R43)  ; FMad(a,b,c)
  %R45 = call float @dx.op.tertiary.f32(i32 46, float %R42, float %R13, float %R44)  ; FMad(a,b,c)
  %R46 = fmul fast float %R36, %R15
  %R47 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R16, float %R46)  ; FMad(a,b,c)
  %R48 = call float @dx.op.tertiary.f32(i32 46, float %R42, float %R17, float %R47)  ; FMad(a,b,c)
  %R49 = fmul fast float %R36, %R19
  %R50 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R20, float %R49)  ; FMad(a,b,c)
  %R51 = call float @dx.op.tertiary.f32(i32 46, float %R42, float %R21, float %R50)  ; FMad(a,b,c)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R45)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R48)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R51)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float %R6)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 1, float %R9)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 2, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A2

; Function Attrs: nounwind readnone
declare float @dx.op.tertiary.f32(i32, float, float, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }
attributes #A2 = { nounwind readonly }

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
!M8 = !{i32 0, %hostlayout.r_data* undef, !"", i32 0, i32 0, i32 1, i32 128, null}
!M5 = !{[3 x i32] [i32 1, i32 8, i32 55]}
!M6 = !{void ()* @vs_main, !"vs_main", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12}
!M12 = !{i32 0, !"SV_VertexID", i8 5, i8 1, !M13, i8 0, i32 1, i8 1, i32 0, i8 0, !M14}
!M13 = !{i32 0}
!M14 = !{i32 3, i32 1}
!M11 = !{!M15, !M16}
!M15 = !{i32 0, !"LOC", i8 9, i8 0, !M13, i8 2, i32 1, i8 3, i32 0, i8 0, !M17}
!M17 = !{i32 3, i32 7}
!M16 = !{i32 1, !"SV_Position", i8 9, i8 3, !M13, i8 4, i32 1, i8 4, i32 1, i8 0, !M18}
!M18 = !{i32 3, i32 15}

