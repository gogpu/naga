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
; SigInputElements: 0
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: texture_sample
;
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
; nagaSamplerHeap                   sampler      NA          NA      S0             s0  2048
; image_1d                          texture     f32          1d      T0             t0     1
; image_2d                          texture     f32          2d      T1             t1     1
; image_2d_array                    texture     f32     2darray      T2             t4     1
; image_cube_array                  texture     f32   cubearray      T3             t6     1
; nagaGroup1SamplerIndexArray       texture  struct         r/o      T4    t1,space255     1
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
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.ResRet.f32 = type { float, float, float, float, i32 }
%dx.types.Dimensions = type { i32, i32, i32, i32 }
%"class.Texture1D<vector<float, 4> >" = type { <4 x float>, %"class.Texture1D<vector<float, 4> >::mips_type" }
%"class.Texture1D<vector<float, 4> >::mips_type" = type { i32 }
%"class.Texture2D<vector<float, 4> >" = type { <4 x float>, %"class.Texture2D<vector<float, 4> >::mips_type" }
%"class.Texture2D<vector<float, 4> >::mips_type" = type { i32 }
%"class.Texture2DArray<vector<float, 4> >" = type { <4 x float>, %"class.Texture2DArray<vector<float, 4> >::mips_type" }
%"class.Texture2DArray<vector<float, 4> >::mips_type" = type { i32 }
%"class.TextureCubeArray<vector<float, 4> >" = type { <4 x float> }
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%struct.S0 = type { i32 }

define void @texture_sample() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 4, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 3, i32 6, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 2, i32 4, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R5 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 0, i32 0)  ; BufferLoad(srv,index,wot)
  %R6 = extractvalue %dx.types.ResRet.i32 %R5, 0
  %R7 = add i32 %R6, 0
  %R8 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R7, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R9 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R4, %dx.types.Handle %R8, float 5.000000e-01, float undef, float undef, float undef, i32 0, i32 undef, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R10 = extractvalue %dx.types.ResRet.f32 %R9, 0
  %R11 = extractvalue %dx.types.ResRet.f32 %R9, 1
  %R12 = extractvalue %dx.types.ResRet.f32 %R9, 2
  %R13 = extractvalue %dx.types.ResRet.f32 %R9, 3
  %R14 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R3, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R15 = extractvalue %dx.types.ResRet.f32 %R14, 0
  %R16 = extractvalue %dx.types.ResRet.f32 %R14, 1
  %R17 = extractvalue %dx.types.ResRet.f32 %R14, 2
  %R18 = extractvalue %dx.types.ResRet.f32 %R14, 3
  %R19 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R3, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 3, i32 1, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R20 = extractvalue %dx.types.ResRet.f32 %R19, 0
  %R21 = extractvalue %dx.types.ResRet.f32 %R19, 1
  %R22 = extractvalue %dx.types.ResRet.f32 %R19, 2
  %R23 = extractvalue %dx.types.ResRet.f32 %R19, 3
  %R24 = call %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32 62, %dx.types.Handle %R3, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 undef, float 0x4002666660000000)  ; SampleLevel(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,LOD)
  %R25 = extractvalue %dx.types.ResRet.f32 %R24, 0
  %R26 = extractvalue %dx.types.ResRet.f32 %R24, 1
  %R27 = extractvalue %dx.types.ResRet.f32 %R24, 2
  %R28 = extractvalue %dx.types.ResRet.f32 %R24, 3
  %R29 = call %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32 62, %dx.types.Handle %R3, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 3, i32 1, i32 undef, float 0x4002666660000000)  ; SampleLevel(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,LOD)
  %R30 = extractvalue %dx.types.ResRet.f32 %R29, 0
  %R31 = extractvalue %dx.types.ResRet.f32 %R29, 1
  %R32 = extractvalue %dx.types.ResRet.f32 %R29, 2
  %R33 = extractvalue %dx.types.ResRet.f32 %R29, 3
  %R34 = call %dx.types.ResRet.f32 @dx.op.sampleBias.f32(i32 61, %dx.types.Handle %R3, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 3, i32 1, i32 undef, float 2.000000e+00, float undef)  ; SampleBias(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,bias,clamp)
  %R35 = extractvalue %dx.types.ResRet.f32 %R34, 0
  %R36 = extractvalue %dx.types.ResRet.f32 %R34, 1
  %R37 = extractvalue %dx.types.ResRet.f32 %R34, 2
  %R38 = extractvalue %dx.types.ResRet.f32 %R34, 3
  %R39 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R3, i32 0)  ; GetDimensions(handle,mipLevel)
  %R40 = extractvalue %dx.types.Dimensions %R39, 0
  %R41 = sitofp i32 %R40 to float
  %R42 = extractvalue %dx.types.Dimensions %R39, 1
  %R43 = sitofp i32 %R42 to float
  %R44 = fdiv fast float 5.000000e-01, %R41
  %R45 = fdiv fast float 5.000000e-01, %R43
  %R46 = fsub fast float 1.000000e+00, %R44
  %R47 = fsub fast float 1.000000e+00, %R45
  %R48 = call float @dx.op.binary.f32(i32 35, float 5.000000e-01, float %R44)  ; FMax(a,b)
  %R49 = call float @dx.op.binary.f32(i32 35, float 5.000000e-01, float %R45)  ; FMax(a,b)
  %R50 = call float @dx.op.binary.f32(i32 36, float %R48, float %R46)  ; FMin(a,b)
  %R51 = call float @dx.op.binary.f32(i32 36, float %R49, float %R47)  ; FMin(a,b)
  %R52 = call %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32 62, %dx.types.Handle %R3, %dx.types.Handle %R8, float %R50, float %R51, float undef, float undef, i32 0, i32 0, i32 undef, float 0.000000e+00)  ; SampleLevel(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,LOD)
  %R53 = extractvalue %dx.types.ResRet.f32 %R52, 0
  %R54 = extractvalue %dx.types.ResRet.f32 %R52, 1
  %R55 = extractvalue %dx.types.ResRet.f32 %R52, 2
  %R56 = extractvalue %dx.types.ResRet.f32 %R52, 3
  %R57 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R2, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 0, i32 0, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R58 = extractvalue %dx.types.ResRet.f32 %R57, 0
  %R59 = extractvalue %dx.types.ResRet.f32 %R57, 1
  %R60 = extractvalue %dx.types.ResRet.f32 %R57, 2
  %R61 = extractvalue %dx.types.ResRet.f32 %R57, 3
  %R62 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R2, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 3, i32 1, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R63 = extractvalue %dx.types.ResRet.f32 %R62, 0
  %R64 = extractvalue %dx.types.ResRet.f32 %R62, 1
  %R65 = extractvalue %dx.types.ResRet.f32 %R62, 2
  %R66 = extractvalue %dx.types.ResRet.f32 %R62, 3
  %R67 = call %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32 62, %dx.types.Handle %R2, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 0, i32 0, i32 undef, float 0x4002666660000000)  ; SampleLevel(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,LOD)
  %R68 = extractvalue %dx.types.ResRet.f32 %R67, 0
  %R69 = extractvalue %dx.types.ResRet.f32 %R67, 1
  %R70 = extractvalue %dx.types.ResRet.f32 %R67, 2
  %R71 = extractvalue %dx.types.ResRet.f32 %R67, 3
  %R72 = call %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32 62, %dx.types.Handle %R2, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 3, i32 1, i32 undef, float 0x4002666660000000)  ; SampleLevel(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,LOD)
  %R73 = extractvalue %dx.types.ResRet.f32 %R72, 0
  %R74 = extractvalue %dx.types.ResRet.f32 %R72, 1
  %R75 = extractvalue %dx.types.ResRet.f32 %R72, 2
  %R76 = extractvalue %dx.types.ResRet.f32 %R72, 3
  %R77 = call %dx.types.ResRet.f32 @dx.op.sampleBias.f32(i32 61, %dx.types.Handle %R2, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 3, i32 1, i32 undef, float 2.000000e+00, float undef)  ; SampleBias(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,bias,clamp)
  %R78 = extractvalue %dx.types.ResRet.f32 %R77, 0
  %R79 = extractvalue %dx.types.ResRet.f32 %R77, 1
  %R80 = extractvalue %dx.types.ResRet.f32 %R77, 2
  %R81 = extractvalue %dx.types.ResRet.f32 %R77, 3
  %R82 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R1, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, i32 undef, i32 undef, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R83 = extractvalue %dx.types.ResRet.f32 %R82, 0
  %R84 = extractvalue %dx.types.ResRet.f32 %R82, 1
  %R85 = extractvalue %dx.types.ResRet.f32 %R82, 2
  %R86 = extractvalue %dx.types.ResRet.f32 %R82, 3
  %R87 = call %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32 62, %dx.types.Handle %R1, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, i32 undef, i32 undef, i32 undef, float 0x4002666660000000)  ; SampleLevel(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,LOD)
  %R88 = extractvalue %dx.types.ResRet.f32 %R87, 0
  %R89 = extractvalue %dx.types.ResRet.f32 %R87, 1
  %R90 = extractvalue %dx.types.ResRet.f32 %R87, 2
  %R91 = extractvalue %dx.types.ResRet.f32 %R87, 3
  %R92 = call %dx.types.ResRet.f32 @dx.op.sampleBias.f32(i32 61, %dx.types.Handle %R1, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, i32 undef, i32 undef, i32 undef, float 2.000000e+00, float undef)  ; SampleBias(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,bias,clamp)
  %R93 = extractvalue %dx.types.ResRet.f32 %R92, 0
  %R94 = extractvalue %dx.types.ResRet.f32 %R92, 1
  %R95 = extractvalue %dx.types.ResRet.f32 %R92, 2
  %R96 = extractvalue %dx.types.ResRet.f32 %R92, 3
  %R97 = fmul fast float %R93, 2.000000e+00
  %R98 = fmul fast float %R88, 2.000000e+00
  %R99 = fmul fast float %R83, 2.000000e+00
  %R100 = fmul fast float %R78, 2.000000e+00
  %R101 = fmul fast float %R73, 2.000000e+00
  %R102 = fmul fast float %R68, 2.000000e+00
  %R103 = fmul fast float %R63, 2.000000e+00
  %R104 = fmul fast float %R58, 2.000000e+00
  %R105 = fadd fast float %R15, %R10
  %R106 = fadd fast float %R105, %R20
  %R107 = fadd fast float %R106, %R25
  %R108 = fadd fast float %R107, %R30
  %R109 = fadd fast float %R108, %R35
  %R110 = fadd fast float %R109, %R53
  %R111 = fadd fast float %R110, %R97
  %R112 = fadd fast float %R111, %R98
  %R113 = fadd fast float %R112, %R99
  %R114 = fadd fast float %R113, %R100
  %R115 = fadd fast float %R114, %R101
  %R116 = fadd fast float %R115, %R102
  %R117 = fadd fast float %R116, %R103
  %R118 = fadd fast float %R117, %R104
  %R119 = fmul fast float %R94, 2.000000e+00
  %R120 = fmul fast float %R89, 2.000000e+00
  %R121 = fmul fast float %R84, 2.000000e+00
  %R122 = fmul fast float %R79, 2.000000e+00
  %R123 = fmul fast float %R74, 2.000000e+00
  %R124 = fmul fast float %R69, 2.000000e+00
  %R125 = fmul fast float %R64, 2.000000e+00
  %R126 = fmul fast float %R59, 2.000000e+00
  %R127 = fadd fast float %R16, %R11
  %R128 = fadd fast float %R127, %R21
  %R129 = fadd fast float %R128, %R26
  %R130 = fadd fast float %R129, %R31
  %R131 = fadd fast float %R130, %R36
  %R132 = fadd fast float %R131, %R54
  %R133 = fadd fast float %R132, %R119
  %R134 = fadd fast float %R133, %R120
  %R135 = fadd fast float %R134, %R121
  %R136 = fadd fast float %R135, %R122
  %R137 = fadd fast float %R136, %R123
  %R138 = fadd fast float %R137, %R124
  %R139 = fadd fast float %R138, %R125
  %R140 = fadd fast float %R139, %R126
  %R141 = fmul fast float %R95, 2.000000e+00
  %R142 = fmul fast float %R90, 2.000000e+00
  %R143 = fmul fast float %R85, 2.000000e+00
  %R144 = fmul fast float %R80, 2.000000e+00
  %R145 = fmul fast float %R75, 2.000000e+00
  %R146 = fmul fast float %R70, 2.000000e+00
  %R147 = fmul fast float %R65, 2.000000e+00
  %R148 = fmul fast float %R60, 2.000000e+00
  %R149 = fadd fast float %R17, %R12
  %R150 = fadd fast float %R149, %R22
  %R151 = fadd fast float %R150, %R27
  %R152 = fadd fast float %R151, %R32
  %R153 = fadd fast float %R152, %R37
  %R154 = fadd fast float %R153, %R55
  %R155 = fadd fast float %R154, %R141
  %R156 = fadd fast float %R155, %R142
  %R157 = fadd fast float %R156, %R143
  %R158 = fadd fast float %R157, %R144
  %R159 = fadd fast float %R158, %R145
  %R160 = fadd fast float %R159, %R146
  %R161 = fadd fast float %R160, %R147
  %R162 = fadd fast float %R161, %R148
  %R163 = fmul fast float %R96, 2.000000e+00
  %R164 = fmul fast float %R91, 2.000000e+00
  %R165 = fmul fast float %R86, 2.000000e+00
  %R166 = fmul fast float %R81, 2.000000e+00
  %R167 = fmul fast float %R76, 2.000000e+00
  %R168 = fmul fast float %R71, 2.000000e+00
  %R169 = fmul fast float %R66, 2.000000e+00
  %R170 = fmul fast float %R61, 2.000000e+00
  %R171 = fadd fast float %R18, %R13
  %R172 = fadd fast float %R171, %R23
  %R173 = fadd fast float %R172, %R28
  %R174 = fadd fast float %R173, %R33
  %R175 = fadd fast float %R174, %R38
  %R176 = fadd fast float %R175, %R56
  %R177 = fadd fast float %R176, %R163
  %R178 = fadd fast float %R177, %R164
  %R179 = fadd fast float %R178, %R165
  %R180 = fadd fast float %R179, %R166
  %R181 = fadd fast float %R180, %R167
  %R182 = fadd fast float %R181, %R168
  %R183 = fadd fast float %R182, %R169
  %R184 = fadd fast float %R183, %R170
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R118)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R140)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R162)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R184)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Dimensions @dx.op.getDimensions(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sample.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sampleBias.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sampleLevel.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A1

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A2

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
!M4 = !{!M7, null, null, !M8}
!M7 = !{!M9, !M10, !M11, !M12, !M13}
!M9 = !{i32 0, %"class.Texture1D<vector<float, 4> >"* undef, !"", i32 0, i32 0, i32 1, i32 1, i32 0, !M14}
!M14 = !{i32 0, i32 9}
!M10 = !{i32 1, %"class.Texture2D<vector<float, 4> >"* undef, !"", i32 0, i32 1, i32 1, i32 2, i32 0, !M14}
!M11 = !{i32 2, %"class.Texture2DArray<vector<float, 4> >"* undef, !"", i32 0, i32 4, i32 1, i32 7, i32 0, !M14}
!M12 = !{i32 3, %"class.TextureCubeArray<vector<float, 4> >"* undef, !"", i32 0, i32 6, i32 1, i32 9, i32 0, !M14}
!M13 = !{i32 4, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 1, i32 1, i32 12, i32 0, !M15}
!M15 = !{i32 1, i32 0}
!M8 = !{!M16}
!M16 = !{i32 0, [2048 x %struct.S0]* undef, !"", i32 0, i32 0, i32 2048, i32 0, null}
!M5 = !{[2 x i32] [i32 0, i32 4]}
!M6 = !{void ()* @texture_sample, !"texture_sample", !M17, !M4, !M18}
!M17 = !{null, !M19, null}
!M19 = !{!M20}
!M20 = !{i32 0, !"SV_Target", i8 9, i8 16, !M21, i8 0, i32 1, i8 4, i32 0, i8 0, !M22}
!M21 = !{i32 0}
!M22 = !{i32 3, i32 15}
!M18 = !{i32 0, i64 16}

