;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; LOC                      1   xyz         1     NONE   float   xyz
; SV_Position              0   xyzw        2      POS   float
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Target                0   xyzw        0   TARGET   float   xyzw
;
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Pixel Shader
; DepthOutput=0
; SampleFrequency=0
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 3
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 3
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: fs_main
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0                 linear
; LOC                      1                 linear
; SV_Position              0          noperspective
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Target                0
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; camera                            cbuffer      NA          NA     CB0            cb0     1
; light                             cbuffer      NA          NA     CB1     cb0,space1     1
;
;
; ViewId state:
;
; Number of inputs: 12, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0, 1, 2, 4, 5, 6 }
;   output 1 depends on inputs: { 0, 1, 2, 4, 5, 6 }
;   output 2 depends on inputs: { 0, 1, 2, 4, 5, 6 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%hostlayout.camera = type { %hostlayout.struct.Camera }
%hostlayout.struct.Camera = type { <4 x float>, [4 x <4 x float>] }
%light = type { %struct.S0 }
%struct.S0 = type { <3 x float>, i32, <3 x float>, i32 }

define void @fs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 1, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R5 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R6 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R7 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R8 = call float @dx.op.unary.f32(i32 22, float %R2)  ; Frc(value)
  %R9 = call float @dx.op.unary.f32(i32 22, float %R3)  ; Frc(value)
  %R10 = call float @dx.op.unary.f32(i32 22, float %R4)  ; Frc(value)
  %R11 = fmul fast float %R8, 1.000000e+01
  %R12 = fmul fast float %R9, 1.000000e+01
  %R13 = fmul fast float %R10, 1.000000e+01
  %R14 = call float @dx.op.unary.f32(i32 7, float %R11)  ; Saturate(value)
  %R15 = call float @dx.op.unary.f32(i32 7, float %R12)  ; Saturate(value)
  %R16 = call float @dx.op.unary.f32(i32 7, float %R13)  ; Saturate(value)
  %R17 = fmul fast float %R14, 2.000000e+00
  %R18 = fmul fast float %R15, 2.000000e+00
  %R19 = fmul fast float %R16, 2.000000e+00
  %R20 = fsub fast float 3.000000e+00, %R17
  %R21 = fsub fast float 3.000000e+00, %R18
  %R22 = fsub fast float 3.000000e+00, %R19
  %R23 = fmul float %R14, %R15
  %R24 = fmul float %R23, %R16
  %R25 = fmul float %R24, %R24
  %R26 = fmul fast float %R21, %R20
  %R27 = fmul fast float %R26, %R22
  %R28 = fmul fast float %R27, %R25
  %R29 = fmul fast float %R28, 0x3FD3333340000000
  %R30 = fmul fast float %R28, 0x3FB99999A0000000
  %R31 = fmul fast float %R28, 5.000000e-01
  %R32 = fsub fast float 5.000000e-01, %R29
  %R33 = fadd fast float %R30, 0x3FB99999A0000000
  %R34 = fsub fast float 0x3FE6666660000000, %R31
  %R35 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R36 = extractvalue %dx.types.CBufRet.f32 %R35, 0
  %R37 = extractvalue %dx.types.CBufRet.f32 %R35, 1
  %R38 = extractvalue %dx.types.CBufRet.f32 %R35, 2
  %R39 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R40 = extractvalue %dx.types.CBufRet.f32 %R39, 0
  %R41 = extractvalue %dx.types.CBufRet.f32 %R39, 1
  %R42 = extractvalue %dx.types.CBufRet.f32 %R39, 2
  %R43 = fsub fast float %R40, %R2
  %R44 = fsub fast float %R41, %R3
  %R45 = fsub fast float %R42, %R4
  %R46 = call float @dx.op.dot3.f32(i32 55, float %R43, float %R44, float %R45, float %R43, float %R44, float %R45)  ; Dot3(ax,ay,az,bx,by,bz)
  %R47 = call float @dx.op.unary.f32(i32 25, float %R46)  ; Rsqrt(value)
  %R48 = fmul fast float %R43, %R47
  %R49 = fmul fast float %R44, %R47
  %R50 = fmul fast float %R45, %R47
  %R51 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R1, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R52 = extractvalue %dx.types.CBufRet.f32 %R51, 0
  %R53 = extractvalue %dx.types.CBufRet.f32 %R51, 1
  %R54 = extractvalue %dx.types.CBufRet.f32 %R51, 2
  %R55 = fsub fast float %R52, %R2
  %R56 = fsub fast float %R53, %R3
  %R57 = fsub fast float %R54, %R4
  %R58 = call float @dx.op.dot3.f32(i32 55, float %R55, float %R56, float %R57, float %R55, float %R56, float %R57)  ; Dot3(ax,ay,az,bx,by,bz)
  %R59 = call float @dx.op.unary.f32(i32 25, float %R58)  ; Rsqrt(value)
  %R60 = fmul fast float %R55, %R59
  %R61 = fmul fast float %R56, %R59
  %R62 = fmul fast float %R57, %R59
  %R63 = fadd fast float %R60, %R48
  %R64 = fadd fast float %R61, %R49
  %R65 = fadd fast float %R62, %R50
  %R66 = call float @dx.op.dot3.f32(i32 55, float %R63, float %R64, float %R65, float %R63, float %R64, float %R65)  ; Dot3(ax,ay,az,bx,by,bz)
  %R67 = call float @dx.op.unary.f32(i32 25, float %R66)  ; Rsqrt(value)
  %R68 = fmul fast float %R63, %R67
  %R69 = fmul fast float %R64, %R67
  %R70 = fmul fast float %R65, %R67
  %R71 = call float @dx.op.dot3.f32(i32 55, float %R5, float %R6, float %R7, float %R48, float %R49, float %R50)  ; Dot3(ax,ay,az,bx,by,bz)
  %R72 = call float @dx.op.binary.f32(i32 35, float %R71, float 0.000000e+00)  ; FMax(a,b)
  %R73 = call float @dx.op.dot3.f32(i32 55, float %R5, float %R6, float %R7, float %R68, float %R69, float %R70)  ; Dot3(ax,ay,az,bx,by,bz)
  %R74 = call float @dx.op.binary.f32(i32 35, float %R73, float 0.000000e+00)  ; FMax(a,b)
  %R75 = call float @dx.op.unary.f32(i32 23, float %R74)  ; Log(value)
  %R76 = fmul fast float %R75, 3.200000e+01
  %R77 = call float @dx.op.unary.f32(i32 21, float %R76)  ; Exp(value)
  %R78 = fadd fast float %R72, 0x3FB99999A0000000
  %R79 = fadd fast float %R78, %R77
  %R80 = fmul fast float %R36, %R32
  %R81 = fmul fast float %R80, %R79
  %R82 = fmul fast float %R37, %R33
  %R83 = fmul fast float %R82, %R79
  %R84 = fmul fast float %R38, %R34
  %R85 = fmul fast float %R84, %R79
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R81)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R83)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R85)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readnone
declare float @dx.op.dot3.f32(i32, float, float, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A2

; Function Attrs: nounwind readnone
declare float @dx.op.unary.f32(i32, float) #A0

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind readonly }
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
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{null, null, !M7, null}
!M7 = !{!M8, !M9}
!M8 = !{i32 0, %hostlayout.camera* undef, !"", i32 0, i32 0, i32 1, i32 80, null}
!M9 = !{i32 1, %light* undef, !"", i32 1, i32 0, i32 1, i32 32, null}
!M5 = !{[14 x i32] [i32 12, i32 4, i32 7, i32 7, i32 7, i32 0, i32 7, i32 7, i32 7, i32 0, i32 0, i32 0, i32 0, i32 0]}
!M6 = !{void ()* @fs_main, !"fs_main", !M10, !M4, null}
!M10 = !{!M11, !M12, null}
!M11 = !{!M13, !M14, !M15}
!M13 = !{i32 0, !"LOC", i8 9, i8 0, !M16, i8 2, i32 1, i8 3, i32 0, i8 0, !M17}
!M16 = !{i32 0}
!M17 = !{i32 3, i32 7}
!M14 = !{i32 1, !"LOC", i8 9, i8 0, !M18, i8 2, i32 1, i8 3, i32 1, i8 0, !M17}
!M18 = !{i32 1}
!M15 = !{i32 2, !"SV_Position", i8 9, i8 3, !M16, i8 4, i32 1, i8 4, i32 2, i8 0, null}
!M12 = !{!M19}
!M19 = !{i32 0, !"SV_Target", i8 9, i8 16, !M16, i8 0, i32 1, i8 4, i32 0, i8 0, !M20}
!M20 = !{i32 3, i32 15}

