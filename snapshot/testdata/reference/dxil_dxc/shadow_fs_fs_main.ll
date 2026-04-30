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
; u_globals                         cbuffer      NA          NA     CB0            cb0     1
; u_entity                          cbuffer      NA          NA     CB1     cb0,space1     1
; nagaComparisonSamplerHeap         sampler      NA          NA      S0      s0,space1  2048
; s_lights                          texture    byte         r/o      T0             t1     1
; t_shadow                          texture     f32     2darray      T1             t2     1
; nagaGroup0SamplerIndexArray       texture  struct         r/o      T2    t0,space255     1
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
%dx.types.ResRet.f32 = type { float, float, float, float, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%struct.S0 = type { i32 }
%"class.Texture2DArray<float>" = type { float, %"class.Texture2DArray<float>::mips_type" }
%"class.Texture2DArray<float>::mips_type" = type { i32 }
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%hostlayout.u_globals = type { %hostlayout.struct.Globals }
%hostlayout.struct.Globals = type { [4 x <4 x float>], <4 x i32> }
%hostlayout.u_entity = type { %hostlayout.struct.Entity }
%hostlayout.struct.Entity = type { [4 x <4 x float>], <4 x float> }
%struct.S1 = type { i32 }

define void @fs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 2, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
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
  %R45 = mul i32 %R31, 96
  %R46 = add i32 %R45, 80
  %R47 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R46, i32 undef)  ; BufferLoad(srv,index,wot)
  %R48 = extractvalue %dx.types.ResRet.i32 %R47, 0
  %R49 = extractvalue %dx.types.ResRet.i32 %R47, 1
  %R50 = extractvalue %dx.types.ResRet.i32 %R47, 2
  %R51 = bitcast i32 %R48 to float
  %R52 = bitcast i32 %R49 to float
  %R53 = bitcast i32 %R50 to float
  %R54 = add i32 %R45, 64
  %R55 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R54, i32 undef)  ; BufferLoad(srv,index,wot)
  %R56 = extractvalue %dx.types.ResRet.i32 %R55, 0
  %R57 = extractvalue %dx.types.ResRet.i32 %R55, 1
  %R58 = extractvalue %dx.types.ResRet.i32 %R55, 2
  %R59 = bitcast i32 %R56 to float
  %R60 = bitcast i32 %R57 to float
  %R61 = bitcast i32 %R58 to float
  %R62 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R45, i32 undef)  ; BufferLoad(srv,index,wot)
  %R63 = extractvalue %dx.types.ResRet.i32 %R62, 3
  %R64 = bitcast i32 %R63 to float
  %R65 = or i32 %R45, 16
  %R66 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R65, i32 undef)  ; BufferLoad(srv,index,wot)
  %R67 = extractvalue %dx.types.ResRet.i32 %R66, 3
  %R68 = bitcast i32 %R67 to float
  %R69 = add i32 %R45, 32
  %R70 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R69, i32 undef)  ; BufferLoad(srv,index,wot)
  %R71 = extractvalue %dx.types.ResRet.i32 %R70, 3
  %R72 = bitcast i32 %R71 to float
  %R73 = add i32 %R45, 48
  %R74 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R73, i32 undef)  ; BufferLoad(srv,index,wot)
  %R75 = extractvalue %dx.types.ResRet.i32 %R74, 3
  %R76 = bitcast i32 %R75 to float
  %R77 = fmul fast float %R64, %R5
  %R78 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R68, float %R77)  ; FMad(a,b,c)
  %R79 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R72, float %R78)  ; FMad(a,b,c)
  %R80 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R76, float %R79)  ; FMad(a,b,c)
  %R81 = fcmp ugt float %R80, 0.000000e+00
  br i1 %R81, label %R82, label %R22

; <label>:77                                      ; preds = %R43
  %R83 = extractvalue %dx.types.ResRet.i32 %R74, 2
  %R84 = bitcast i32 %R83 to float
  %R85 = extractvalue %dx.types.ResRet.i32 %R70, 2
  %R86 = bitcast i32 %R85 to float
  %R87 = extractvalue %dx.types.ResRet.i32 %R66, 2
  %R88 = bitcast i32 %R87 to float
  %R89 = extractvalue %dx.types.ResRet.i32 %R62, 2
  %R90 = bitcast i32 %R89 to float
  %R91 = fmul fast float %R90, %R5
  %R92 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R88, float %R91)  ; FMad(a,b,c)
  %R93 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R86, float %R92)  ; FMad(a,b,c)
  %R94 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R84, float %R93)  ; FMad(a,b,c)
  %R95 = extractvalue %dx.types.ResRet.i32 %R74, 1
  %R96 = bitcast i32 %R95 to float
  %R97 = extractvalue %dx.types.ResRet.i32 %R70, 1
  %R98 = bitcast i32 %R97 to float
  %R99 = extractvalue %dx.types.ResRet.i32 %R66, 1
  %R100 = bitcast i32 %R99 to float
  %R101 = extractvalue %dx.types.ResRet.i32 %R62, 1
  %R102 = bitcast i32 %R101 to float
  %R103 = fmul fast float %R102, %R5
  %R104 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R100, float %R103)  ; FMad(a,b,c)
  %R105 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R98, float %R104)  ; FMad(a,b,c)
  %R106 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R96, float %R105)  ; FMad(a,b,c)
  %R107 = extractvalue %dx.types.ResRet.i32 %R74, 0
  %R108 = bitcast i32 %R107 to float
  %R109 = extractvalue %dx.types.ResRet.i32 %R70, 0
  %R110 = bitcast i32 %R109 to float
  %R111 = extractvalue %dx.types.ResRet.i32 %R66, 0
  %R112 = bitcast i32 %R111 to float
  %R113 = extractvalue %dx.types.ResRet.i32 %R62, 0
  %R114 = bitcast i32 %R113 to float
  %R115 = fmul fast float %R114, %R5
  %R116 = call float @dx.op.tertiary.f32(i32 46, float %R6, float %R112, float %R115)  ; FMad(a,b,c)
  %R117 = call float @dx.op.tertiary.f32(i32 46, float %R7, float %R110, float %R116)  ; FMad(a,b,c)
  %R118 = call float @dx.op.tertiary.f32(i32 46, float %R8, float %R108, float %R117)  ; FMad(a,b,c)
  %R119 = fdiv fast float 1.000000e+00, %R80
  %R120 = fmul fast float %R118, 5.000000e-01
  %R121 = fmul fast float %R106, -5.000000e-01
  %R122 = fmul fast float %R120, %R119
  %R123 = fmul fast float %R121, %R119
  %R124 = fadd fast float %R122, 5.000000e-01
  %R125 = fadd fast float %R123, 5.000000e-01
  %R126 = fmul fast float %R119, %R94
  %R127 = sitofp i32 %R31 to float
  %R128 = call %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32 65, %dx.types.Handle %R1, %dx.types.Handle %R15, float %R124, float %R125, float %R127, float undef, i32 0, i32 0, i32 undef, float %R126)  ; SampleCmpLevelZero(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue)
  %R129 = extractvalue %dx.types.ResRet.f32 %R128, 0
  br label %R22

; <label>:125                                     ; preds = %R82, %R43
  %R130 = phi float [ %R129, %R82 ], [ 1.000000e+00, %R43 ]
  %R131 = fsub fast float %R59, %R5
  %R132 = fsub fast float %R60, %R6
  %R133 = fsub fast float %R61, %R7
  %R134 = call float @dx.op.dot3.f32(i32 55, float %R131, float %R132, float %R133, float %R131, float %R132, float %R133)  ; Dot3(ax,ay,az,bx,by,bz)
  %R135 = call float @dx.op.unary.f32(i32 25, float %R134)  ; Rsqrt(value)
  %R136 = fmul fast float %R135, %R131
  %R137 = fmul fast float %R135, %R132
  %R138 = fmul fast float %R135, %R133
  %R139 = call float @dx.op.dot3.f32(i32 55, float %R18, float %R19, float %R20, float %R136, float %R137, float %R138)  ; Dot3(ax,ay,az,bx,by,bz)
  %R140 = call float @dx.op.binary.f32(i32 35, float 0.000000e+00, float %R139)  ; FMax(a,b)
  %R141 = fmul fast float %R140, %R130
  %R142 = fmul fast float %R141, %R51
  %R143 = fmul fast float %R141, %R52
  %R144 = fmul fast float %R141, %R53
  %R25 = fadd fast float %R142, %R24
  %R27 = fadd fast float %R143, %R26
  %R29 = fadd fast float %R144, %R28
  %R145 = icmp eq i32 %R32, %R38
  %R146 = icmp eq i32 %R35, 0
  %R147 = and i1 %R146, %R145
  br i1 %R147, label %R44, label %R21

; <label>:147                                     ; preds = %R22, %R21
  %R148 = phi float [ %R25, %R22 ], [ %R24, %R21 ]
  %R149 = phi float [ %R27, %R22 ], [ %R26, %R21 ]
  %R150 = phi float [ %R29, %R22 ], [ %R28, %R21 ]
  %R151 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R3, i32 4)  ; CBufferLoadLegacy(handle,regIndex)
  %R152 = extractvalue %dx.types.CBufRet.f32 %R151, 0
  %R153 = extractvalue %dx.types.CBufRet.f32 %R151, 1
  %R154 = extractvalue %dx.types.CBufRet.f32 %R151, 2
  %R155 = extractvalue %dx.types.CBufRet.f32 %R151, 3
  %R156 = fmul fast float %R152, %R148
  %R157 = fmul fast float %R153, %R149
  %R158 = fmul fast float %R154, %R150
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R156)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R157)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R158)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R155)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
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
!M2 = !{i32 1, i32 8}
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{!M7, null, !M8, !M9}
!M7 = !{!M10, !M11, !M12}
!M10 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i32 0, null}
!M11 = !{i32 1, %"class.Texture2DArray<float>"* undef, !"", i32 0, i32 2, i32 1, i32 7, i32 0, !M13}
!M13 = !{i32 0, i32 9}
!M12 = !{i32 2, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 0, i32 1, i32 12, i32 0, !M14}
!M14 = !{i32 1, i32 4}
!M8 = !{!M15, !M16}
!M15 = !{i32 0, %hostlayout.u_globals* undef, !"", i32 0, i32 0, i32 1, i32 80, null}
!M16 = !{i32 1, %hostlayout.u_entity* undef, !"", i32 1, i32 0, i32 1, i32 80, null}
!M9 = !{!M17}
!M17 = !{i32 0, [2048 x %struct.S1]* undef, !"", i32 1, i32 0, i32 2048, i32 1, null}
!M5 = !{[14 x i32] [i32 12, i32 4, i32 7, i32 7, i32 7, i32 0, i32 7, i32 7, i32 7, i32 7, i32 0, i32 0, i32 0, i32 0]}
!M6 = !{void ()* @fs_main, !"fs_main", !M18, !M4, !M19}
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

