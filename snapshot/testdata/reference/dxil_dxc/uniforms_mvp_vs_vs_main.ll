;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
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
; LOC                      0
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
; uniforms                          cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 3, outputs: 8
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0, 1, 2 }
;   output 1 depends on inputs: { 0, 1, 2 }
;   output 2 depends on inputs: { 0, 1, 2 }
;   output 4 depends on inputs: { 0, 1, 2 }
;   output 5 depends on inputs: { 0, 1, 2 }
;   output 6 depends on inputs: { 0, 1, 2 }
;   output 7 depends on inputs: { 0, 1, 2 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%hostlayout.uniforms = type { %hostlayout.struct.Uniforms }
%hostlayout.struct.Uniforms = type { [4 x <4 x float>], [4 x <4 x float>], [4 x <4 x float>] }

define void @vs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R5 = extractvalue %dx.types.CBufRet.f32 %R4, 0
  %R6 = extractvalue %dx.types.CBufRet.f32 %R4, 1
  %R7 = extractvalue %dx.types.CBufRet.f32 %R4, 2
  %R8 = extractvalue %dx.types.CBufRet.f32 %R4, 3
  %R9 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R10 = extractvalue %dx.types.CBufRet.f32 %R9, 0
  %R11 = extractvalue %dx.types.CBufRet.f32 %R9, 1
  %R12 = extractvalue %dx.types.CBufRet.f32 %R9, 2
  %R13 = extractvalue %dx.types.CBufRet.f32 %R9, 3
  %R14 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 2)  ; CBufferLoadLegacy(handle,regIndex)
  %R15 = extractvalue %dx.types.CBufRet.f32 %R14, 0
  %R16 = extractvalue %dx.types.CBufRet.f32 %R14, 1
  %R17 = extractvalue %dx.types.CBufRet.f32 %R14, 2
  %R18 = extractvalue %dx.types.CBufRet.f32 %R14, 3
  %R19 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 3)  ; CBufferLoadLegacy(handle,regIndex)
  %R20 = extractvalue %dx.types.CBufRet.f32 %R19, 0
  %R21 = extractvalue %dx.types.CBufRet.f32 %R19, 1
  %R22 = extractvalue %dx.types.CBufRet.f32 %R19, 2
  %R23 = extractvalue %dx.types.CBufRet.f32 %R19, 3
  %R24 = fmul fast float %R5, %R1
  %R25 = call float @dx.op.tertiary.f32(i32 46, float %R2, float %R10, float %R24)  ; FMad(a,b,c)
  %R26 = call float @dx.op.tertiary.f32(i32 46, float %R3, float %R15, float %R25)  ; FMad(a,b,c)
  %R27 = fadd fast float %R26, %R20
  %R28 = fmul fast float %R6, %R1
  %R29 = call float @dx.op.tertiary.f32(i32 46, float %R2, float %R11, float %R28)  ; FMad(a,b,c)
  %R30 = call float @dx.op.tertiary.f32(i32 46, float %R3, float %R16, float %R29)  ; FMad(a,b,c)
  %R31 = fadd fast float %R30, %R21
  %R32 = fmul fast float %R7, %R1
  %R33 = call float @dx.op.tertiary.f32(i32 46, float %R2, float %R12, float %R32)  ; FMad(a,b,c)
  %R34 = call float @dx.op.tertiary.f32(i32 46, float %R3, float %R17, float %R33)  ; FMad(a,b,c)
  %R35 = fadd fast float %R34, %R22
  %R36 = fmul fast float %R8, %R1
  %R37 = call float @dx.op.tertiary.f32(i32 46, float %R2, float %R13, float %R36)  ; FMad(a,b,c)
  %R38 = call float @dx.op.tertiary.f32(i32 46, float %R3, float %R18, float %R37)  ; FMad(a,b,c)
  %R39 = fadd fast float %R38, %R23
  %R40 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 8)  ; CBufferLoadLegacy(handle,regIndex)
  %R41 = extractvalue %dx.types.CBufRet.f32 %R40, 0
  %R42 = extractvalue %dx.types.CBufRet.f32 %R40, 1
  %R43 = extractvalue %dx.types.CBufRet.f32 %R40, 2
  %R44 = extractvalue %dx.types.CBufRet.f32 %R40, 3
  %R45 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 9)  ; CBufferLoadLegacy(handle,regIndex)
  %R46 = extractvalue %dx.types.CBufRet.f32 %R45, 0
  %R47 = extractvalue %dx.types.CBufRet.f32 %R45, 1
  %R48 = extractvalue %dx.types.CBufRet.f32 %R45, 2
  %R49 = extractvalue %dx.types.CBufRet.f32 %R45, 3
  %R50 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 10)  ; CBufferLoadLegacy(handle,regIndex)
  %R51 = extractvalue %dx.types.CBufRet.f32 %R50, 0
  %R52 = extractvalue %dx.types.CBufRet.f32 %R50, 1
  %R53 = extractvalue %dx.types.CBufRet.f32 %R50, 2
  %R54 = extractvalue %dx.types.CBufRet.f32 %R50, 3
  %R55 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 11)  ; CBufferLoadLegacy(handle,regIndex)
  %R56 = extractvalue %dx.types.CBufRet.f32 %R55, 0
  %R57 = extractvalue %dx.types.CBufRet.f32 %R55, 1
  %R58 = extractvalue %dx.types.CBufRet.f32 %R55, 2
  %R59 = extractvalue %dx.types.CBufRet.f32 %R55, 3
  %R60 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R61 = extractvalue %dx.types.CBufRet.f32 %R60, 0
  %R62 = extractvalue %dx.types.CBufRet.f32 %R60, 1
  %R63 = extractvalue %dx.types.CBufRet.f32 %R60, 2
  %R64 = extractvalue %dx.types.CBufRet.f32 %R60, 3
  %R65 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 5)  ; CBufferLoadLegacy(handle,regIndex)
  %R66 = extractvalue %dx.types.CBufRet.f32 %R65, 0
  %R67 = extractvalue %dx.types.CBufRet.f32 %R65, 1
  %R68 = extractvalue %dx.types.CBufRet.f32 %R65, 2
  %R69 = extractvalue %dx.types.CBufRet.f32 %R65, 3
  %R70 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 6)  ; CBufferLoadLegacy(handle,regIndex)
  %R71 = extractvalue %dx.types.CBufRet.f32 %R70, 0
  %R72 = extractvalue %dx.types.CBufRet.f32 %R70, 1
  %R73 = extractvalue %dx.types.CBufRet.f32 %R70, 2
  %R74 = extractvalue %dx.types.CBufRet.f32 %R70, 3
  %R75 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 7)  ; CBufferLoadLegacy(handle,regIndex)
  %R76 = extractvalue %dx.types.CBufRet.f32 %R75, 0
  %R77 = extractvalue %dx.types.CBufRet.f32 %R75, 1
  %R78 = extractvalue %dx.types.CBufRet.f32 %R75, 2
  %R79 = extractvalue %dx.types.CBufRet.f32 %R75, 3
  %R80 = fmul fast float %R61, %R41
  %R81 = call float @dx.op.tertiary.f32(i32 46, float %R62, float %R46, float %R80)  ; FMad(a,b,c)
  %R82 = call float @dx.op.tertiary.f32(i32 46, float %R63, float %R51, float %R81)  ; FMad(a,b,c)
  %R83 = call float @dx.op.tertiary.f32(i32 46, float %R64, float %R56, float %R82)  ; FMad(a,b,c)
  %R84 = fmul fast float %R61, %R42
  %R85 = call float @dx.op.tertiary.f32(i32 46, float %R62, float %R47, float %R84)  ; FMad(a,b,c)
  %R86 = call float @dx.op.tertiary.f32(i32 46, float %R63, float %R52, float %R85)  ; FMad(a,b,c)
  %R87 = call float @dx.op.tertiary.f32(i32 46, float %R64, float %R57, float %R86)  ; FMad(a,b,c)
  %R88 = fmul fast float %R61, %R43
  %R89 = call float @dx.op.tertiary.f32(i32 46, float %R62, float %R48, float %R88)  ; FMad(a,b,c)
  %R90 = call float @dx.op.tertiary.f32(i32 46, float %R63, float %R53, float %R89)  ; FMad(a,b,c)
  %R91 = call float @dx.op.tertiary.f32(i32 46, float %R64, float %R58, float %R90)  ; FMad(a,b,c)
  %R92 = fmul fast float %R61, %R44
  %R93 = call float @dx.op.tertiary.f32(i32 46, float %R62, float %R49, float %R92)  ; FMad(a,b,c)
  %R94 = call float @dx.op.tertiary.f32(i32 46, float %R63, float %R54, float %R93)  ; FMad(a,b,c)
  %R95 = call float @dx.op.tertiary.f32(i32 46, float %R64, float %R59, float %R94)  ; FMad(a,b,c)
  %R96 = fmul fast float %R66, %R41
  %R97 = call float @dx.op.tertiary.f32(i32 46, float %R67, float %R46, float %R96)  ; FMad(a,b,c)
  %R98 = call float @dx.op.tertiary.f32(i32 46, float %R68, float %R51, float %R97)  ; FMad(a,b,c)
  %R99 = call float @dx.op.tertiary.f32(i32 46, float %R69, float %R56, float %R98)  ; FMad(a,b,c)
  %R100 = fmul fast float %R66, %R42
  %R101 = call float @dx.op.tertiary.f32(i32 46, float %R67, float %R47, float %R100)  ; FMad(a,b,c)
  %R102 = call float @dx.op.tertiary.f32(i32 46, float %R68, float %R52, float %R101)  ; FMad(a,b,c)
  %R103 = call float @dx.op.tertiary.f32(i32 46, float %R69, float %R57, float %R102)  ; FMad(a,b,c)
  %R104 = fmul fast float %R66, %R43
  %R105 = call float @dx.op.tertiary.f32(i32 46, float %R67, float %R48, float %R104)  ; FMad(a,b,c)
  %R106 = call float @dx.op.tertiary.f32(i32 46, float %R68, float %R53, float %R105)  ; FMad(a,b,c)
  %R107 = call float @dx.op.tertiary.f32(i32 46, float %R69, float %R58, float %R106)  ; FMad(a,b,c)
  %R108 = fmul fast float %R66, %R44
  %R109 = call float @dx.op.tertiary.f32(i32 46, float %R67, float %R49, float %R108)  ; FMad(a,b,c)
  %R110 = call float @dx.op.tertiary.f32(i32 46, float %R68, float %R54, float %R109)  ; FMad(a,b,c)
  %R111 = call float @dx.op.tertiary.f32(i32 46, float %R69, float %R59, float %R110)  ; FMad(a,b,c)
  %R112 = fmul fast float %R71, %R41
  %R113 = call float @dx.op.tertiary.f32(i32 46, float %R72, float %R46, float %R112)  ; FMad(a,b,c)
  %R114 = call float @dx.op.tertiary.f32(i32 46, float %R73, float %R51, float %R113)  ; FMad(a,b,c)
  %R115 = call float @dx.op.tertiary.f32(i32 46, float %R74, float %R56, float %R114)  ; FMad(a,b,c)
  %R116 = fmul fast float %R71, %R42
  %R117 = call float @dx.op.tertiary.f32(i32 46, float %R72, float %R47, float %R116)  ; FMad(a,b,c)
  %R118 = call float @dx.op.tertiary.f32(i32 46, float %R73, float %R52, float %R117)  ; FMad(a,b,c)
  %R119 = call float @dx.op.tertiary.f32(i32 46, float %R74, float %R57, float %R118)  ; FMad(a,b,c)
  %R120 = fmul fast float %R71, %R43
  %R121 = call float @dx.op.tertiary.f32(i32 46, float %R72, float %R48, float %R120)  ; FMad(a,b,c)
  %R122 = call float @dx.op.tertiary.f32(i32 46, float %R73, float %R53, float %R121)  ; FMad(a,b,c)
  %R123 = call float @dx.op.tertiary.f32(i32 46, float %R74, float %R58, float %R122)  ; FMad(a,b,c)
  %R124 = fmul fast float %R71, %R44
  %R125 = call float @dx.op.tertiary.f32(i32 46, float %R72, float %R49, float %R124)  ; FMad(a,b,c)
  %R126 = call float @dx.op.tertiary.f32(i32 46, float %R73, float %R54, float %R125)  ; FMad(a,b,c)
  %R127 = call float @dx.op.tertiary.f32(i32 46, float %R74, float %R59, float %R126)  ; FMad(a,b,c)
  %R128 = fmul fast float %R76, %R41
  %R129 = call float @dx.op.tertiary.f32(i32 46, float %R77, float %R46, float %R128)  ; FMad(a,b,c)
  %R130 = call float @dx.op.tertiary.f32(i32 46, float %R78, float %R51, float %R129)  ; FMad(a,b,c)
  %R131 = call float @dx.op.tertiary.f32(i32 46, float %R79, float %R56, float %R130)  ; FMad(a,b,c)
  %R132 = fmul fast float %R76, %R42
  %R133 = call float @dx.op.tertiary.f32(i32 46, float %R77, float %R47, float %R132)  ; FMad(a,b,c)
  %R134 = call float @dx.op.tertiary.f32(i32 46, float %R78, float %R52, float %R133)  ; FMad(a,b,c)
  %R135 = call float @dx.op.tertiary.f32(i32 46, float %R79, float %R57, float %R134)  ; FMad(a,b,c)
  %R136 = fmul fast float %R76, %R43
  %R137 = call float @dx.op.tertiary.f32(i32 46, float %R77, float %R48, float %R136)  ; FMad(a,b,c)
  %R138 = call float @dx.op.tertiary.f32(i32 46, float %R78, float %R53, float %R137)  ; FMad(a,b,c)
  %R139 = call float @dx.op.tertiary.f32(i32 46, float %R79, float %R58, float %R138)  ; FMad(a,b,c)
  %R140 = fmul fast float %R76, %R44
  %R141 = call float @dx.op.tertiary.f32(i32 46, float %R77, float %R49, float %R140)  ; FMad(a,b,c)
  %R142 = call float @dx.op.tertiary.f32(i32 46, float %R78, float %R54, float %R141)  ; FMad(a,b,c)
  %R143 = call float @dx.op.tertiary.f32(i32 46, float %R79, float %R59, float %R142)  ; FMad(a,b,c)
  %R144 = fmul fast float %R83, %R27
  %R145 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R99, float %R144)  ; FMad(a,b,c)
  %R146 = call float @dx.op.tertiary.f32(i32 46, float %R35, float %R115, float %R145)  ; FMad(a,b,c)
  %R147 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R131, float %R146)  ; FMad(a,b,c)
  %R148 = fmul fast float %R87, %R27
  %R149 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R103, float %R148)  ; FMad(a,b,c)
  %R150 = call float @dx.op.tertiary.f32(i32 46, float %R35, float %R119, float %R149)  ; FMad(a,b,c)
  %R151 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R135, float %R150)  ; FMad(a,b,c)
  %R152 = fmul fast float %R91, %R27
  %R153 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R107, float %R152)  ; FMad(a,b,c)
  %R154 = call float @dx.op.tertiary.f32(i32 46, float %R35, float %R123, float %R153)  ; FMad(a,b,c)
  %R155 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R139, float %R154)  ; FMad(a,b,c)
  %R156 = fmul fast float %R95, %R27
  %R157 = call float @dx.op.tertiary.f32(i32 46, float %R31, float %R111, float %R156)  ; FMad(a,b,c)
  %R158 = call float @dx.op.tertiary.f32(i32 46, float %R35, float %R127, float %R157)  ; FMad(a,b,c)
  %R159 = call float @dx.op.tertiary.f32(i32 46, float %R39, float %R143, float %R158)  ; FMad(a,b,c)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R27)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R31)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R35)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float %R147)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 1, float %R151)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 2, float %R155)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 3, float %R159)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
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
!M8 = !{i32 0, %hostlayout.uniforms* undef, !"", i32 0, i32 0, i32 1, i32 192, null}
!M5 = !{[5 x i32] [i32 3, i32 8, i32 247, i32 247, i32 247]}
!M6 = !{void ()* @vs_main, !"vs_main", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12}
!M12 = !{i32 0, !"LOC", i8 9, i8 0, !M13, i8 0, i32 1, i8 3, i32 0, i8 0, !M14}
!M13 = !{i32 0}
!M14 = !{i32 3, i32 7}
!M11 = !{!M15, !M16}
!M15 = !{i32 0, !"LOC", i8 9, i8 0, !M13, i8 2, i32 1, i8 3, i32 0, i8 0, !M14}
!M16 = !{i32 1, !"SV_Position", i8 9, i8 3, !M13, i8 4, i32 1, i8 4, i32 1, i8 0, !M17}
!M17 = !{i32 3, i32 15}

