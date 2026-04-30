;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyzw        0     NONE     int   xyzw
; LOC                      1   xyzw        1     NONE     int   xyz
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; LOC                      1   xyzw        1     NONE   float   xyzw
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
; u_globals                         cbuffer      NA          NA     CB0            cb0     1
; u_entity                          cbuffer      NA          NA     CB1     cb0,space1     1
;
;
; ViewId state:
;
; Number of inputs: 8, outputs: 12
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 4, 5, 6 }
;   output 1 depends on inputs: { 4, 5, 6 }
;   output 2 depends on inputs: { 4, 5, 6 }
;   output 4 depends on inputs: { 0, 1, 2, 3 }
;   output 5 depends on inputs: { 0, 1, 2, 3 }
;   output 6 depends on inputs: { 0, 1, 2, 3 }
;   output 7 depends on inputs: { 0, 1, 2, 3 }
;   output 8 depends on inputs: { 0, 1, 2, 3 }
;   output 9 depends on inputs: { 0, 1, 2, 3 }
;   output 10 depends on inputs: { 0, 1, 2, 3 }
;   output 11 depends on inputs: { 0, 1, 2, 3 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%hostlayout.u_globals = type { %hostlayout.struct.Globals }
%hostlayout.struct.Globals = type { [4 x <4 x float>], <4 x i32> }
%hostlayout.u_entity = type { %hostlayout.struct.Entity }
%hostlayout.struct.Entity = type { [4 x <4 x float>], <4 x float> }

define void @vs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 1, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call i32 @dx.op.loadInput.i32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call i32 @dx.op.loadInput.i32(i32 4, i32 1, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call i32 @dx.op.loadInput.i32(i32 4, i32 1, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R5 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R6 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R7 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R8 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 3, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R9 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R10 = extractvalue %dx.types.CBufRet.f32 %R9, 0
  %R11 = extractvalue %dx.types.CBufRet.f32 %R9, 1
  %R12 = extractvalue %dx.types.CBufRet.f32 %R9, 2
  %R13 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R14 = extractvalue %dx.types.CBufRet.f32 %R13, 0
  %R15 = extractvalue %dx.types.CBufRet.f32 %R13, 1
  %R16 = extractvalue %dx.types.CBufRet.f32 %R13, 2
  %R17 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 2)  ; CBufferLoadLegacy(handle,regIndex)
  %R18 = extractvalue %dx.types.CBufRet.f32 %R17, 0
  %R19 = extractvalue %dx.types.CBufRet.f32 %R17, 1
  %R20 = extractvalue %dx.types.CBufRet.f32 %R17, 2
  %R21 = extractvalue %dx.types.CBufRet.f32 %R9, 3
  %R22 = extractvalue %dx.types.CBufRet.f32 %R13, 3
  %R23 = extractvalue %dx.types.CBufRet.f32 %R17, 3
  %R24 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 3)  ; CBufferLoadLegacy(handle,regIndex)
  %R25 = extractvalue %dx.types.CBufRet.f32 %R24, 0
  %R26 = extractvalue %dx.types.CBufRet.f32 %R24, 1
  %R27 = extractvalue %dx.types.CBufRet.f32 %R24, 2
  %R28 = extractvalue %dx.types.CBufRet.f32 %R24, 3
  %R29 = sitofp i32 %R5 to float
  %R30 = sitofp i32 %R6 to float
  %R31 = sitofp i32 %R7 to float
  %R32 = sitofp i32 %R8 to float
  %R33 = fmul fast float %R10, %R29
  %R34 = call float @dx.op.tertiary.f32(i32 46, float %R30, float %R14, float %R33)  ; FMad(a,b,c)
  %R35 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R18, float %R34)  ; FMad(a,b,c)
  %R36 = call float @dx.op.tertiary.f32(i32 46, float %R32, float %R25, float %R35)  ; FMad(a,b,c)
  %R37 = fmul fast float %R11, %R29
  %R38 = call float @dx.op.tertiary.f32(i32 46, float %R30, float %R15, float %R37)  ; FMad(a,b,c)
  %R39 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R19, float %R38)  ; FMad(a,b,c)
  %R40 = call float @dx.op.tertiary.f32(i32 46, float %R32, float %R26, float %R39)  ; FMad(a,b,c)
  %R41 = fmul fast float %R12, %R29
  %R42 = call float @dx.op.tertiary.f32(i32 46, float %R30, float %R16, float %R41)  ; FMad(a,b,c)
  %R43 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R20, float %R42)  ; FMad(a,b,c)
  %R44 = call float @dx.op.tertiary.f32(i32 46, float %R32, float %R27, float %R43)  ; FMad(a,b,c)
  %R45 = fmul fast float %R21, %R29
  %R46 = call float @dx.op.tertiary.f32(i32 46, float %R30, float %R22, float %R45)  ; FMad(a,b,c)
  %R47 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R23, float %R46)  ; FMad(a,b,c)
  %R48 = call float @dx.op.tertiary.f32(i32 46, float %R32, float %R28, float %R47)  ; FMad(a,b,c)
  %R49 = sitofp i32 %R2 to float
  %R50 = sitofp i32 %R3 to float
  %R51 = sitofp i32 %R4 to float
  %R52 = fmul fast float %R10, %R49
  %R53 = call float @dx.op.tertiary.f32(i32 46, float %R50, float %R14, float %R52)  ; FMad(a,b,c)
  %R54 = call float @dx.op.tertiary.f32(i32 46, float %R51, float %R18, float %R53)  ; FMad(a,b,c)
  %R55 = fmul fast float %R11, %R49
  %R56 = call float @dx.op.tertiary.f32(i32 46, float %R50, float %R15, float %R55)  ; FMad(a,b,c)
  %R57 = call float @dx.op.tertiary.f32(i32 46, float %R51, float %R19, float %R56)  ; FMad(a,b,c)
  %R58 = fmul fast float %R12, %R49
  %R59 = call float @dx.op.tertiary.f32(i32 46, float %R50, float %R16, float %R58)  ; FMad(a,b,c)
  %R60 = call float @dx.op.tertiary.f32(i32 46, float %R51, float %R20, float %R59)  ; FMad(a,b,c)
  %R61 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R1, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R62 = extractvalue %dx.types.CBufRet.f32 %R61, 0
  %R63 = extractvalue %dx.types.CBufRet.f32 %R61, 1
  %R64 = extractvalue %dx.types.CBufRet.f32 %R61, 2
  %R65 = extractvalue %dx.types.CBufRet.f32 %R61, 3
  %R66 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R1, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R67 = extractvalue %dx.types.CBufRet.f32 %R66, 0
  %R68 = extractvalue %dx.types.CBufRet.f32 %R66, 1
  %R69 = extractvalue %dx.types.CBufRet.f32 %R66, 2
  %R70 = extractvalue %dx.types.CBufRet.f32 %R66, 3
  %R71 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R1, i32 2)  ; CBufferLoadLegacy(handle,regIndex)
  %R72 = extractvalue %dx.types.CBufRet.f32 %R71, 0
  %R73 = extractvalue %dx.types.CBufRet.f32 %R71, 1
  %R74 = extractvalue %dx.types.CBufRet.f32 %R71, 2
  %R75 = extractvalue %dx.types.CBufRet.f32 %R71, 3
  %R76 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R1, i32 3)  ; CBufferLoadLegacy(handle,regIndex)
  %R77 = extractvalue %dx.types.CBufRet.f32 %R76, 0
  %R78 = extractvalue %dx.types.CBufRet.f32 %R76, 1
  %R79 = extractvalue %dx.types.CBufRet.f32 %R76, 2
  %R80 = extractvalue %dx.types.CBufRet.f32 %R76, 3
  %R81 = fmul fast float %R62, %R36
  %R82 = call float @dx.op.tertiary.f32(i32 46, float %R40, float %R67, float %R81)  ; FMad(a,b,c)
  %R83 = call float @dx.op.tertiary.f32(i32 46, float %R44, float %R72, float %R82)  ; FMad(a,b,c)
  %R84 = call float @dx.op.tertiary.f32(i32 46, float %R48, float %R77, float %R83)  ; FMad(a,b,c)
  %R85 = fmul fast float %R63, %R36
  %R86 = call float @dx.op.tertiary.f32(i32 46, float %R40, float %R68, float %R85)  ; FMad(a,b,c)
  %R87 = call float @dx.op.tertiary.f32(i32 46, float %R44, float %R73, float %R86)  ; FMad(a,b,c)
  %R88 = call float @dx.op.tertiary.f32(i32 46, float %R48, float %R78, float %R87)  ; FMad(a,b,c)
  %R89 = fmul fast float %R64, %R36
  %R90 = call float @dx.op.tertiary.f32(i32 46, float %R40, float %R69, float %R89)  ; FMad(a,b,c)
  %R91 = call float @dx.op.tertiary.f32(i32 46, float %R44, float %R74, float %R90)  ; FMad(a,b,c)
  %R92 = call float @dx.op.tertiary.f32(i32 46, float %R48, float %R79, float %R91)  ; FMad(a,b,c)
  %R93 = fmul fast float %R65, %R36
  %R94 = call float @dx.op.tertiary.f32(i32 46, float %R40, float %R70, float %R93)  ; FMad(a,b,c)
  %R95 = call float @dx.op.tertiary.f32(i32 46, float %R44, float %R75, float %R94)  ; FMad(a,b,c)
  %R96 = call float @dx.op.tertiary.f32(i32 46, float %R48, float %R80, float %R95)  ; FMad(a,b,c)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R54)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R57)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R60)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float %R36)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 1, float %R40)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 2, float %R44)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 3, float %R48)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float %R84)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 1, float %R88)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 2, float %R92)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 3, float %R96)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A1

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
!M2 = !{i32 1, i32 0}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{null, null, !M7, null}
!M7 = !{!M8, !M9}
!M8 = !{i32 0, %hostlayout.u_globals* undef, !"", i32 0, i32 0, i32 1, i32 80, null}
!M9 = !{i32 1, %hostlayout.u_entity* undef, !"", i32 1, i32 0, i32 1, i32 80, null}
!M5 = !{[10 x i32] [i32 8, i32 12, i32 4080, i32 4080, i32 4080, i32 4080, i32 7, i32 7, i32 7, i32 0]}
!M6 = !{void ()* @vs_main, !"vs_main", !M10, !M4, null}
!M10 = !{!M11, !M12, null}
!M11 = !{!M13, !M14}
!M13 = !{i32 0, !"LOC", i8 4, i8 0, !M15, i8 0, i32 1, i8 4, i32 0, i8 0, !M16}
!M15 = !{i32 0}
!M16 = !{i32 3, i32 15}
!M14 = !{i32 1, !"LOC", i8 4, i8 0, !M17, i8 0, i32 1, i8 4, i32 1, i8 0, !M18}
!M17 = !{i32 1}
!M18 = !{i32 3, i32 7}
!M12 = !{!M19, !M20, !M21}
!M19 = !{i32 0, !"LOC", i8 9, i8 0, !M15, i8 2, i32 1, i8 3, i32 0, i8 0, !M18}
!M20 = !{i32 1, !"LOC", i8 9, i8 0, !M17, i8 2, i32 1, i8 4, i32 1, i8 0, !M16}
!M21 = !{i32 2, !"SV_Position", i8 9, i8 3, !M15, i8 4, i32 1, i8 4, i32 2, i8 0, !M16}

