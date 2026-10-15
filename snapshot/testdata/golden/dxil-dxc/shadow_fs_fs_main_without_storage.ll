;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; LOC                      1   xyzw        1     NONE   float   xyzw
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
; EntryFunctionName: fs_main_without_storage
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
; u_globals                         cbuffer      NA          NA     CB0            cb0     1
; u_entity                          cbuffer      NA          NA     CB1     cb0,space1     1
; u_lights                          cbuffer      NA          NA     CB2            cb1     1
; nagaComparisonSamplerHeap         sampler      NA          NA      S0      s0,space1  2048
; t_shadow                          texture     f32     2darray      T0             t2     1
; nagaGroup0SamplerIndexArray       texture  struct         r/o      T1    t0,space255     1
;
;
; ViewId state:
;
; Number of inputs: 12, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0, 1, 2, 4, 5, 6, 7 }
;   output 1 depends on inputs: { 0, 1, 2, 4, 5, 6, 7 }
;   output 2 depends on inputs: { 0, 1, 2, 4, 5, 6, 7 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.CBufRet.i32 = type { i32, i32, i32, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%dx.types.ResRet.f32 = type { float, float, float, float, i32 }
%"class.Texture2DArray<float>" = type { float, %"class.Texture2DArray<float>::mips_type" }
%"class.Texture2DArray<float>::mips_type" = type { i32 }
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%hostlayout.u_globals = type { %hostlayout.struct.Globals }
%hostlayout.struct.Globals = type { [4 x <4 x float>], <4 x i32> }
%hostlayout.u_entity = type { %hostlayout.struct.Entity }
%hostlayout.struct.Entity = type { [4 x <4 x float>], <4 x float> }
%hostlayout.u_lights = type { [10 x %hostlayout.struct.Light] }
%hostlayout.struct.Light = type { [4 x <4 x float>], <4 x float>, <4 x float> }
%struct.S0 = type { i32 }

define void @fs_main_without_storage() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 2, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 1, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R5 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R6 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R7 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R8 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 3, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R9 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R10 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R11 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R12 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 3, i32 0)  ; BufferLoad(srv,index,wot)
  %R13 = extractvalue %dx.types.ResRet.i32 %R12, 0
  %R14 = add i32 %R13, 0
  %R15 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R14, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R16 = call float @dx.op.dot3.f32(i32 55, float %R9, float %R10, float %R11, float %R9, float %R10, float %R11)  ; Dot3(ax,ay,az,bx,by,bz)
  %R17 = call float @dx.op.unary.f32(i32 25, float %R16)  ; Rsqrt(value)
  %R18 = fmul fast float %R17, %R9
  %R19 = fmul fast float %R17, %R10
  %R20 = fmul fast float %R17, %R11
  br label %R21

; <label>:22                                      ; preds = %R22, %R23
  %R24 = phi float [ %R25, %R22 ], [ 0x3FA99999A0000000, %R23 ]
  %R26 = phi float [ %R27, %R22 ], [ 0x3FA99999A0000000, %R23 ]
  %R28 = phi float [ %R29, %R22 ], [ 0x3FA99999A0000000, %R23 ]
  %R30 = phi i32 [ %R31, %R22 ], [ 0, %R23 ]
  %R32 = phi i32 [ %R33, %R22 ], [ -1, %R23 ]
  %R34 = phi i32 [ %R35, %R22 ], [ -1, %R23 ]
  %R36 = phi i32 [ 1, %R22 ], [ 0, %R23 ]
  %R37 = icmp eq i32 %R34, 0
  %R38 = zext i1 %R37 to i32
  %R33 = sub i32 %R32, %R38
  %R35 = add i32 %R34, -1
  %R31 = add i32 %R36, %R30
  %R39 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R4, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R40 = extractvalue %dx.types.CBufRet.i32 %R39, 0
  %R41 = call i32 @dx.op.binary.i32(i32 40, i32 %R40, i32 10)  ; UMin(a,b)
  %R42 = icmp ult i32 %R31, %R41
  br i1 %R42, label %R43, label %R44

; <label>:39                                      ; preds = %R21
  %R45 = mul i32 %R31, 6
  %R46 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 %R45)  ; CBufferLoadLegacy(handle,regIndex)
  %R47 = extractvalue %dx.types.CBufRet.f32 %R46, 3
  %R48 = or i32 %R45, 1
  %R49 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 %R48)  ; CBufferLoadLegacy(handle,regIndex)
  %R50 = extractvalue %dx.types.CBufRet.f32 %R49, 3
  %R51 = add i32 %R48, 1
  %R52 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 %R51)  ; CBufferLoadLegacy(handle,regIndex)
  %R53 = extractvalue %dx.types.CBufRet.f32 %R52, 3
  %R54 = add i32 %R48, 2
  %R55 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 %R54)  ; CBufferLoadLegacy(handle,regIndex)
  %R56 = extractvalue %dx.types.CBufRet.f32 %R55, 3
  %R57 = fmul fast float %R47, %R5
  %R58 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R50, float %R57)  ; FMad(a,b,c)
  %R59 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R53, float %R58)  ; FMad(a,b,c)
  %R60 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R56, float %R59)  ; FMad(a,b,c)
  %R61 = fcmp ugt float %R60, 0.000000e+00
  br i1 %R61, label %R62, label %R22

; <label>:57                                      ; preds = %R43
  %R63 = extractvalue %dx.types.CBufRet.f32 %R55, 2
  %R64 = extractvalue %dx.types.CBufRet.f32 %R52, 2
  %R65 = extractvalue %dx.types.CBufRet.f32 %R49, 2
  %R66 = extractvalue %dx.types.CBufRet.f32 %R46, 2
  %R67 = fmul fast float %R66, %R5
  %R68 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R65, float %R67)  ; FMad(a,b,c)
  %R69 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R64, float %R68)  ; FMad(a,b,c)
  %R70 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R63, float %R69)  ; FMad(a,b,c)
  %R71 = extractvalue %dx.types.CBufRet.f32 %R55, 1
  %R72 = extractvalue %dx.types.CBufRet.f32 %R52, 1
  %R73 = extractvalue %dx.types.CBufRet.f32 %R49, 1
  %R74 = extractvalue %dx.types.CBufRet.f32 %R46, 1
  %R75 = fmul fast float %R74, %R5
  %R76 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R73, float %R75)  ; FMad(a,b,c)
  %R77 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R72, float %R76)  ; FMad(a,b,c)
  %R78 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R71, float %R77)  ; FMad(a,b,c)
  %R79 = extractvalue %dx.types.CBufRet.f32 %R55, 0
  %R80 = extractvalue %dx.types.CBufRet.f32 %R52, 0
  %R81 = extractvalue %dx.types.CBufRet.f32 %R49, 0
  %R82 = extractvalue %dx.types.CBufRet.f32 %R46, 0
  %R83 = fmul fast float %R82, %R5
  %R84 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R81, float %R83)  ; FMad(a,b,c)
  %R85 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R80, float %R84)  ; FMad(a,b,c)
  %R86 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R79, float %R85)  ; FMad(a,b,c)
  %R87 = fdiv fast float 1.000000e+00, %R60
  %R88 = fmul fast float %R86, 5.000000e-01
  %R89 = fmul fast float %R78, -5.000000e-01
  %R90 = fmul fast float %R88, %R87
  %R91 = fmul fast float %R89, %R87
  %R92 = fadd fast float %R90, 5.000000e-01
  %R93 = fadd fast float %R91, 5.000000e-01
  %R94 = fmul fast float %R87, %R70
  %R95 = sitofp i32 %R31 to float
  %R96 = call %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32 65, %dx.types.Handle %R1, %dx.types.Handle %R15, float %R92, float %R93, float %R95, float undef, i32 0, i32 0, i32 undef, float %R94)  ; SampleCmpLevelZero(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue)
  %R97 = extractvalue %dx.types.ResRet.f32 %R96, 0
  br label %R22

; <label>:93                                      ; preds = %R62, %R43
  %R98 = phi float [ %R97, %R62 ], [ 1.000000e+00, %R43 ]
  %R99 = add i32 %R45, 4
  %R100 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 %R99)  ; CBufferLoadLegacy(handle,regIndex)
  %R101 = extractvalue %dx.types.CBufRet.f32 %R100, 0
  %R102 = extractvalue %dx.types.CBufRet.f32 %R100, 1
  %R103 = extractvalue %dx.types.CBufRet.f32 %R100, 2
  %R104 = fsub fast float %R101, %R5
  %R105 = fsub fast float %R102, %R6
  %R106 = fsub fast float %R103, %R7
  %R107 = call float @dx.op.dot3.f32(i32 55, float %R104, float %R105, float %R106, float %R104, float %R105, float %R106)  ; Dot3(ax,ay,az,bx,by,bz)
  %R108 = call float @dx.op.unary.f32(i32 25, float %R107)  ; Rsqrt(value)
  %R109 = fmul fast float %R104, %R108
  %R110 = fmul fast float %R105, %R108
  %R111 = fmul fast float %R106, %R108
  %R112 = call float @dx.op.dot3.f32(i32 55, float %R18, float %R19, float %R20, float %R109, float %R110, float %R111)  ; Dot3(ax,ay,az,bx,by,bz)
  %R113 = call float @dx.op.binary.f32(i32 35, float 0.000000e+00, float %R112)  ; FMax(a,b)
  %R114 = fmul fast float %R113, %R98
  %R115 = add i32 %R45, 5
  %R116 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 %R115)  ; CBufferLoadLegacy(handle,regIndex)
  %R117 = extractvalue %dx.types.CBufRet.f32 %R116, 0
  %R118 = extractvalue %dx.types.CBufRet.f32 %R116, 1
  %R119 = extractvalue %dx.types.CBufRet.f32 %R116, 2
  %R120 = fmul fast float %R117, %R114
  %R121 = fmul fast float %R118, %R114
  %R122 = fmul fast float %R119, %R114
  %R25 = fadd fast float %R120, %R24
  %R27 = fadd fast float %R121, %R26
  %R29 = fadd fast float %R122, %R28
  %R123 = icmp eq i32 %R32, %R38
  %R124 = icmp eq i32 %R35, 0
  %R125 = and i1 %R124, %R123
  br i1 %R125, label %R44, label %R21

; <label>:125                                     ; preds = %R22, %R21
  %R126 = phi float [ %R25, %R22 ], [ %R24, %R21 ]
  %R127 = phi float [ %R27, %R22 ], [ %R26, %R21 ]
  %R128 = phi float [ %R29, %R22 ], [ %R28, %R21 ]
  %R129 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R3, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R130 = extractvalue %dx.types.CBufRet.f32 %R129, 0
  %R131 = extractvalue %dx.types.CBufRet.f32 %R129, 1
  %R132 = extractvalue %dx.types.CBufRet.f32 %R129, 2
  %R133 = extractvalue %dx.types.CBufRet.f32 %R129, 3
  %R134 = fmul fast float %R130, %R126
  %R135 = fmul fast float %R131, %R127
  %R136 = fmul fast float %R132, %R128
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R134)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R135)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R136)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R133)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.binary.i32(i32, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readnone
declare float @dx.op.dot3.f32(i32, float, float, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A1

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A2

; Function Attrs: nounwind readnone
declare float @dx.op.tertiary.f32(i32, float, float, float) #A0

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
!M4 = !{!M7, null, !M8, !M9}
!M7 = !{!M10, !M11}
!M10 = !{i32 0, %"class.Texture2DArray<float>"* undef, !"", i32 0, i32 2, i32 1, i32 7, i32 0, !M12}
!M12 = !{i32 0, i32 9}
!M11 = !{i32 1, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 0, i32 1, i32 12, i32 0, !M13}
!M13 = !{i32 1, i32 0}
!M8 = !{!M14, !M15, !M16}
!M14 = !{i32 0, %hostlayout.u_globals* undef, !"", i32 0, i32 0, i32 1, i32 80, null}
!M15 = !{i32 1, %hostlayout.u_entity* undef, !"", i32 1, i32 0, i32 1, i32 80, null}
!M16 = !{i32 2, %hostlayout.u_lights* undef, !"", i32 0, i32 1, i32 1, i32 960, null}
!M9 = !{!M17}
!M17 = !{i32 0, [2048 x %struct.S0]* undef, !"", i32 1, i32 0, i32 2048, i32 1, null}
!M5 = !{[14 x i32] [i32 12, i32 4, i32 7, i32 7, i32 7, i32 0, i32 7, i32 7, i32 7, i32 7, i32 0, i32 0, i32 0, i32 0]}
!M6 = !{void ()* @fs_main_without_storage, !"fs_main_without_storage", !M18, !M4, !M19}
!M18 = !{!M20, !M21, null}
!M20 = !{!M22, !M23, !M24}
!M22 = !{i32 0, !"LOC", i8 9, i8 0, !M25, i8 2, i32 1, i8 3, i32 0, i8 0, !M26}
!M25 = !{i32 0}
!M26 = !{i32 3, i32 7}
!M23 = !{i32 1, !"LOC", i8 9, i8 0, !M27, i8 2, i32 1, i8 4, i32 1, i8 0, !M28}
!M27 = !{i32 1}
!M28 = !{i32 3, i32 15}
!M24 = !{i32 2, !"SV_Position", i8 9, i8 3, !M25, i8 4, i32 1, i8 4, i32 2, i8 0, null}
!M21 = !{!M29}
!M29 = !{i32 0, !"SV_Target", i8 9, i8 16, !M25, i8 0, i32 1, i8 4, i32 0, i8 0, !M28}
!M19 = !{i32 0, i64 16}

