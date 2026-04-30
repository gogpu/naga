;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   x           0     NONE    uint   x
; LOC                      1   xy          1     NONE   float   xy
; SV_Position              0   xyzw        2      POS   float
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Target                0   x           0   TARGET    uint   x
; SV_Target                1   x           1   TARGET    uint   x
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
; SigOutputElements: 2
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 3
; SigOutputVectors[0]: 2
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: gen_terrain_fragment
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0        nointerpolation
; LOC                      1                 linear
; SV_Position              0          noperspective
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Target                0
; SV_Target                1
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; gen_data                          cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 12, outputs: 5
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 4, 5 }
;   output 4 depends on inputs: { 0 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.i32 = type { i32, i32, i32, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%gen_data = type { %struct.S0 }
%struct.S0 = type { <2 x i32>, <2 x i32>, <2 x float>, i32, i32 }

define void @gen_terrain_fragment() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R5 = extractvalue %dx.types.CBufRet.i32 %R4, 2
  %R6 = extractvalue %dx.types.CBufRet.i32 %R4, 3
  %R7 = uitofp i32 %R5 to float
  %R8 = fmul fast float %R7, %R1
  %R9 = mul i32 %R5, %R5
  %R10 = uitofp i32 %R9 to float
  %R11 = fmul fast float %R10, %R2
  %R12 = fadd fast float %R11, %R8
  %R13 = call float @dx.op.binary.f32(i32 35, float %R12, float 0.000000e+00)  ; FMax(a,b)
  %R14 = call float @dx.op.binary.f32(i32 36, float %R13, float 0x41EFFFFFE0000000)  ; FMin(a,b)
  %R15 = fptoui float %R14 to i32
  %R16 = add i32 %R15, %R6
  %R17 = uitofp i32 %R16 to float
  %R18 = fmul fast float %R17, 0x3FC5555560000000
  %R19 = call float @dx.op.unary.f32(i32 27, float %R18)  ; Round_ni(value)
  %R20 = call float @dx.op.binary.f32(i32 35, float %R19, float 0.000000e+00)  ; FMax(a,b)
  %R21 = call float @dx.op.binary.f32(i32 36, float %R20, float 0x41EFFFFFE0000000)  ; FMin(a,b)
  %R22 = fptoui float %R21 to i32
  %R23 = urem i32 %R16, 6
  %R24 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R25 = extractvalue %dx.types.CBufRet.i32 %R24, 0
  %R26 = extractvalue %dx.types.CBufRet.i32 %R24, 2
  %R27 = extractvalue %dx.types.CBufRet.i32 %R24, 3
  %R28 = add i32 %R25, 1
  %R29 = uitofp i32 %R28 to float
  %R30 = uitofp i32 %R22 to float
  %R31 = fdiv fast float %R30, %R29
  %R32 = call float @dx.op.unary.f32(i32 29, float %R31)  ; Round_z(value)
  %R33 = fmul fast float %R29, %R32
  %R34 = fsub fast float %R30, %R33
  %R35 = icmp eq i32 %R28, 0
  %R36 = select i1 %R35, i32 1, i32 %R28
  %R37 = udiv i32 %R22, %R36
  %R38 = uitofp i32 %R37 to float
  %R39 = sitofp i32 %R26 to float
  %R40 = sitofp i32 %R27 to float
  %R41 = fadd fast float %R34, %R39
  %R42 = fadd fast float %R38, %R40
  %R43 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R44 = extractvalue %dx.types.CBufRet.f32 %R43, 0
  %R45 = extractvalue %dx.types.CBufRet.f32 %R43, 1
  %R46 = fmul fast float %R41, 0x3F847AE140000000
  %R47 = fmul fast float %R42, 0x3F847AE140000000
  br label %R48

; <label>:49                                      ; preds = %R49, %R50
  %R51 = phi float [ %R52, %R49 ], [ %R46, %R50 ]
  %R53 = phi float [ %R54, %R49 ], [ %R47, %R50 ]
  %R55 = phi float [ %R56, %R49 ], [ 0.000000e+00, %R50 ]
  %R57 = phi float [ %R58, %R49 ], [ 5.000000e-01, %R50 ]
  %R59 = phi i32 [ %R60, %R49 ], [ 0, %R50 ]
  %R61 = phi i32 [ %R62, %R49 ], [ -1, %R50 ]
  %R63 = phi i32 [ %R64, %R49 ], [ -1, %R50 ]
  %R65 = phi i32 [ 1, %R49 ], [ 0, %R50 ]
  %R66 = icmp eq i32 %R63, 0
  %R67 = zext i1 %R66 to i32
  %R64 = add i32 %R63, -1
  %R60 = add i32 %R65, %R59
  %R68 = icmp ult i32 %R60, 5
  br i1 %R68, label %R49, label %R69

; <label>:63                                      ; preds = %R48
  %R62 = sub i32 %R61, %R67
  %R70 = call float @dx.op.dot2.f32(i32 54, float %R51, float %R53, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R71 = fadd fast float %R70, %R51
  %R72 = fadd fast float %R70, %R53
  %R73 = call float @dx.op.unary.f32(i32 27, float %R71)  ; Round_ni(value)
  %R74 = call float @dx.op.unary.f32(i32 27, float %R72)  ; Round_ni(value)
  %R75 = fsub fast float %R51, %R73
  %R76 = fsub fast float %R53, %R74
  %R77 = call float @dx.op.dot2.f32(i32 54, float %R73, float %R74, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R78 = fadd fast float %R77, %R75
  %R79 = fadd fast float %R76, %R77
  %R80 = fcmp olt float %R78, %R79
  %R81 = select i1 %R80, float 0.000000e+00, float 1.000000e+00
  %R82 = select i1 %R80, float 1.000000e+00, float 0.000000e+00
  %R83 = fadd fast float %R78, 0x3FCB0CB180000000
  %R84 = fadd fast float %R79, 0x3FCB0CB180000000
  %R85 = fadd fast float %R78, 0xBFE279A740000000
  %R86 = fadd fast float %R79, 0xBFE279A740000000
  %R87 = fsub fast float %R83, %R81
  %R88 = fsub fast float %R84, %R82
  %R89 = fmul fast float %R73, 0x3F6C5894E0000000
  %R90 = fmul fast float %R74, 0x3F6C5894E0000000
  %R91 = call float @dx.op.unary.f32(i32 29, float %R89)  ; Round_z(value)
  %R92 = call float @dx.op.unary.f32(i32 29, float %R90)  ; Round_z(value)
  %R93 = fmul fast float %R91, 2.890000e+02
  %R94 = fmul fast float %R92, 2.890000e+02
  %R95 = fsub fast float %R73, %R93
  %R96 = fsub fast float %R74, %R94
  %R97 = fadd fast float %R96, %R82
  %R98 = fadd fast float %R96, 1.000000e+00
  %R99 = fmul fast float %R96, 3.400000e+01
  %R100 = fmul fast float %R97, 3.400000e+01
  %R101 = fmul fast float %R98, 3.400000e+01
  %R102 = fadd fast float %R99, 1.000000e+00
  %R103 = fadd fast float %R100, 1.000000e+00
  %R104 = fadd fast float %R101, 1.000000e+00
  %R105 = fmul fast float %R102, %R96
  %R106 = fmul fast float %R103, %R97
  %R107 = fmul fast float %R104, %R98
  %R108 = fmul fast float %R105, 0x3F6C5894E0000000
  %R109 = fmul fast float %R106, 0x3F6C5894E0000000
  %R110 = fmul fast float %R107, 0x3F6C5894E0000000
  %R111 = call float @dx.op.unary.f32(i32 29, float %R108)  ; Round_z(value)
  %R112 = call float @dx.op.unary.f32(i32 29, float %R109)  ; Round_z(value)
  %R113 = call float @dx.op.unary.f32(i32 29, float %R110)  ; Round_z(value)
  %R114 = fmul fast float %R111, 2.890000e+02
  %R115 = fmul fast float %R112, 2.890000e+02
  %R116 = fmul fast float %R113, 2.890000e+02
  %R117 = fsub fast float %R105, %R114
  %R118 = fadd fast float %R117, %R95
  %R119 = fadd fast float %R81, %R95
  %R120 = fsub fast float %R119, %R115
  %R121 = fadd fast float %R120, %R106
  %R122 = fadd fast float %R95, 1.000000e+00
  %R123 = fsub fast float %R122, %R116
  %R124 = fadd fast float %R123, %R107
  %R125 = fmul fast float %R118, 3.400000e+01
  %R126 = fmul fast float %R121, 3.400000e+01
  %R127 = fmul fast float %R124, 3.400000e+01
  %R128 = fadd fast float %R125, 1.000000e+00
  %R129 = fadd fast float %R126, 1.000000e+00
  %R130 = fadd fast float %R127, 1.000000e+00
  %R131 = fmul fast float %R128, %R118
  %R132 = fmul fast float %R129, %R121
  %R133 = fmul fast float %R130, %R124
  %R134 = fmul fast float %R131, 0x3F6C5894E0000000
  %R135 = fmul fast float %R132, 0x3F6C5894E0000000
  %R136 = fmul fast float %R133, 0x3F6C5894E0000000
  %R137 = call float @dx.op.unary.f32(i32 29, float %R134)  ; Round_z(value)
  %R138 = call float @dx.op.unary.f32(i32 29, float %R135)  ; Round_z(value)
  %R139 = call float @dx.op.unary.f32(i32 29, float %R136)  ; Round_z(value)
  %R140 = fmul fast float %R137, 2.890000e+02
  %R141 = fmul fast float %R138, 2.890000e+02
  %R142 = fmul fast float %R139, 2.890000e+02
  %R143 = fsub fast float %R131, %R140
  %R144 = fsub fast float %R132, %R141
  %R145 = fsub fast float %R133, %R142
  %R146 = call float @dx.op.dot2.f32(i32 54, float %R78, float %R79, float %R78, float %R79)  ; Dot2(ax,ay,bx,by)
  %R147 = call float @dx.op.dot2.f32(i32 54, float %R87, float %R88, float %R87, float %R88)  ; Dot2(ax,ay,bx,by)
  %R148 = call float @dx.op.dot2.f32(i32 54, float %R85, float %R86, float %R85, float %R86)  ; Dot2(ax,ay,bx,by)
  %R149 = fsub fast float 5.000000e-01, %R146
  %R150 = fsub fast float 5.000000e-01, %R147
  %R151 = fsub fast float 5.000000e-01, %R148
  %R152 = call float @dx.op.binary.f32(i32 35, float %R149, float 0.000000e+00)  ; FMax(a,b)
  %R153 = call float @dx.op.binary.f32(i32 35, float %R150, float 0.000000e+00)  ; FMax(a,b)
  %R154 = call float @dx.op.binary.f32(i32 35, float %R151, float 0.000000e+00)  ; FMax(a,b)
  %R155 = fmul fast float %R152, %R152
  %R156 = fmul fast float %R153, %R153
  %R157 = fmul fast float %R154, %R154
  %R158 = fmul fast float %R155, %R155
  %R159 = fmul fast float %R156, %R156
  %R160 = fmul fast float %R157, %R157
  %R161 = fmul fast float %R143, 0x3F98F9C180000000
  %R162 = fmul fast float %R144, 0x3F98F9C180000000
  %R163 = fmul fast float %R145, 0x3F98F9C180000000
  %R164 = call float @dx.op.unary.f32(i32 22, float %R161)  ; Frc(value)
  %R165 = call float @dx.op.unary.f32(i32 22, float %R162)  ; Frc(value)
  %R166 = call float @dx.op.unary.f32(i32 22, float %R163)  ; Frc(value)
  %R167 = fmul fast float %R164, 2.000000e+00
  %R168 = fmul fast float %R165, 2.000000e+00
  %R169 = fmul fast float %R166, 2.000000e+00
  %R170 = fadd fast float %R167, -1.000000e+00
  %R171 = fadd fast float %R168, -1.000000e+00
  %R172 = fadd fast float %R169, -1.000000e+00
  %R173 = call float @dx.op.unary.f32(i32 6, float %R170)  ; FAbs(value)
  %R174 = call float @dx.op.unary.f32(i32 6, float %R171)  ; FAbs(value)
  %R175 = call float @dx.op.unary.f32(i32 6, float %R172)  ; FAbs(value)
  %R176 = fadd fast float %R173, -5.000000e-01
  %R177 = fadd fast float %R174, -5.000000e-01
  %R178 = fadd fast float %R175, -5.000000e-01
  %R179 = fadd fast float %R167, -5.000000e-01
  %R180 = fadd fast float %R168, -5.000000e-01
  %R181 = fadd fast float %R169, -5.000000e-01
  %R182 = call float @dx.op.unary.f32(i32 27, float %R179)  ; Round_ni(value)
  %R183 = call float @dx.op.unary.f32(i32 27, float %R180)  ; Round_ni(value)
  %R184 = call float @dx.op.unary.f32(i32 27, float %R181)  ; Round_ni(value)
  %R185 = fsub fast float %R170, %R182
  %R186 = fsub fast float %R171, %R183
  %R187 = fsub fast float %R172, %R184
  %R188 = fmul fast float %R185, %R185
  %R189 = fmul fast float %R186, %R186
  %R190 = fmul fast float %R187, %R187
  %R191 = fmul fast float %R176, %R176
  %R192 = fmul fast float %R177, %R177
  %R193 = fmul fast float %R178, %R178
  %R194 = fadd fast float %R188, %R191
  %R195 = fadd fast float %R189, %R192
  %R196 = fadd fast float %R190, %R193
  %R197 = fmul fast float %R194, 0x3FEB51CB80000000
  %R198 = fmul fast float %R195, 0x3FEB51CB80000000
  %R199 = fmul fast float %R196, 0x3FEB51CB80000000
  %R200 = fsub fast float 0x3FFCAF7C00000000, %R197
  %R201 = fsub fast float 0x3FFCAF7C00000000, %R198
  %R202 = fsub fast float 0x3FFCAF7C00000000, %R199
  %R203 = fmul fast float %R158, %R200
  %R204 = fmul fast float %R159, %R201
  %R205 = fmul fast float %R160, %R202
  %R206 = fmul fast float %R185, %R78
  %R207 = fmul fast float %R176, %R79
  %R208 = fadd fast float %R206, %R207
  %R209 = fmul fast float %R186, %R87
  %R210 = fmul fast float %R187, %R85
  %R211 = fmul fast float %R177, %R88
  %R212 = fmul fast float %R178, %R86
  %R213 = fadd fast float %R209, %R211
  %R214 = fadd fast float %R210, %R212
  %R215 = call float @dx.op.dot3.f32(i32 55, float %R203, float %R204, float %R205, float %R208, float %R213, float %R214)  ; Dot3(ax,ay,az,bx,by,bz)
  %R216 = fmul fast float %R57, 1.300000e+02
  %R217 = fmul fast float %R216, %R215
  %R56 = fadd fast float %R217, %R55
  %R218 = fmul fast float %R51, 0x3FEC152800000000
  %R219 = call float @dx.op.tertiary.f32(i32 46, float %R53, float 0xBFDEAEE880000000, float %R218)  ; FMad(a,b,c)
  %R220 = fmul fast float %R51, 0x3FDEAEE880000000
  %R221 = call float @dx.op.tertiary.f32(i32 46, float %R53, float 0x3FEC152800000000, float %R220)  ; FMad(a,b,c)
  %R222 = fmul fast float %R219, 2.000000e+00
  %R223 = fmul fast float %R221, 2.000000e+00
  %R52 = fadd fast float %R222, 1.000000e+02
  %R54 = fadd fast float %R223, 1.000000e+02
  %R58 = fmul fast float %R57, 5.000000e-01
  %R224 = icmp eq i32 %R61, %R67
  %R225 = icmp eq i32 %R64, 0
  %R226 = and i1 %R225, %R224
  br i1 %R226, label %R69, label %R48

; <label>:226                                     ; preds = %R49, %R48
  %R227 = phi float [ %R56, %R49 ], [ %R55, %R48 ]
  %R228 = fsub fast float %R45, %R44
  %R229 = fmul fast float %R227, %R228
  %R230 = fadd fast float %R229, %R44
  %R231 = fadd fast float %R41, 0x3FB99999A0000000
  %R232 = fmul fast float %R231, 0x3F847AE140000000
  br label %R233

; <label>:233                                     ; preds = %R234, %R69
  %R235 = phi float [ %R236, %R234 ], [ %R232, %R69 ]
  %R237 = phi float [ %R238, %R234 ], [ %R47, %R69 ]
  %R239 = phi float [ %R240, %R234 ], [ 0.000000e+00, %R69 ]
  %R241 = phi float [ %R242, %R234 ], [ 5.000000e-01, %R69 ]
  %R243 = phi i32 [ %R244, %R234 ], [ 0, %R69 ]
  %R245 = phi i32 [ %R246, %R234 ], [ -1, %R69 ]
  %R247 = phi i32 [ %R248, %R234 ], [ -1, %R69 ]
  %R249 = phi i32 [ 1, %R234 ], [ 0, %R69 ]
  %R250 = icmp eq i32 %R247, 0
  %R251 = zext i1 %R250 to i32
  %R248 = add i32 %R247, -1
  %R244 = add i32 %R249, %R243
  %R252 = icmp ult i32 %R244, 5
  br i1 %R252, label %R234, label %R253

; <label>:247                                     ; preds = %R233
  %R246 = sub i32 %R245, %R251
  %R254 = call float @dx.op.dot2.f32(i32 54, float %R235, float %R237, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R255 = fadd fast float %R254, %R235
  %R256 = fadd fast float %R254, %R237
  %R257 = call float @dx.op.unary.f32(i32 27, float %R255)  ; Round_ni(value)
  %R258 = call float @dx.op.unary.f32(i32 27, float %R256)  ; Round_ni(value)
  %R259 = fsub fast float %R235, %R257
  %R260 = fsub fast float %R237, %R258
  %R261 = call float @dx.op.dot2.f32(i32 54, float %R257, float %R258, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R262 = fadd fast float %R261, %R259
  %R263 = fadd fast float %R260, %R261
  %R264 = fcmp olt float %R262, %R263
  %R265 = select i1 %R264, float 0.000000e+00, float 1.000000e+00
  %R266 = select i1 %R264, float 1.000000e+00, float 0.000000e+00
  %R267 = fadd fast float %R262, 0x3FCB0CB180000000
  %R268 = fadd fast float %R263, 0x3FCB0CB180000000
  %R269 = fadd fast float %R262, 0xBFE279A740000000
  %R270 = fadd fast float %R263, 0xBFE279A740000000
  %R271 = fsub fast float %R267, %R265
  %R272 = fsub fast float %R268, %R266
  %R273 = fmul fast float %R257, 0x3F6C5894E0000000
  %R274 = fmul fast float %R258, 0x3F6C5894E0000000
  %R275 = call float @dx.op.unary.f32(i32 29, float %R273)  ; Round_z(value)
  %R276 = call float @dx.op.unary.f32(i32 29, float %R274)  ; Round_z(value)
  %R277 = fmul fast float %R275, 2.890000e+02
  %R278 = fmul fast float %R276, 2.890000e+02
  %R279 = fsub fast float %R257, %R277
  %R280 = fsub fast float %R258, %R278
  %R281 = fadd fast float %R280, %R266
  %R282 = fadd fast float %R280, 1.000000e+00
  %R283 = fmul fast float %R280, 3.400000e+01
  %R284 = fmul fast float %R281, 3.400000e+01
  %R285 = fmul fast float %R282, 3.400000e+01
  %R286 = fadd fast float %R283, 1.000000e+00
  %R287 = fadd fast float %R284, 1.000000e+00
  %R288 = fadd fast float %R285, 1.000000e+00
  %R289 = fmul fast float %R286, %R280
  %R290 = fmul fast float %R287, %R281
  %R291 = fmul fast float %R288, %R282
  %R292 = fmul fast float %R289, 0x3F6C5894E0000000
  %R293 = fmul fast float %R290, 0x3F6C5894E0000000
  %R294 = fmul fast float %R291, 0x3F6C5894E0000000
  %R295 = call float @dx.op.unary.f32(i32 29, float %R292)  ; Round_z(value)
  %R296 = call float @dx.op.unary.f32(i32 29, float %R293)  ; Round_z(value)
  %R297 = call float @dx.op.unary.f32(i32 29, float %R294)  ; Round_z(value)
  %R298 = fmul fast float %R295, 2.890000e+02
  %R299 = fmul fast float %R296, 2.890000e+02
  %R300 = fmul fast float %R297, 2.890000e+02
  %R301 = fsub fast float %R289, %R298
  %R302 = fadd fast float %R301, %R279
  %R303 = fadd fast float %R265, %R279
  %R304 = fsub fast float %R303, %R299
  %R305 = fadd fast float %R304, %R290
  %R306 = fadd fast float %R279, 1.000000e+00
  %R307 = fsub fast float %R306, %R300
  %R308 = fadd fast float %R307, %R291
  %R309 = fmul fast float %R302, 3.400000e+01
  %R310 = fmul fast float %R305, 3.400000e+01
  %R311 = fmul fast float %R308, 3.400000e+01
  %R312 = fadd fast float %R309, 1.000000e+00
  %R313 = fadd fast float %R310, 1.000000e+00
  %R314 = fadd fast float %R311, 1.000000e+00
  %R315 = fmul fast float %R312, %R302
  %R316 = fmul fast float %R313, %R305
  %R317 = fmul fast float %R314, %R308
  %R318 = fmul fast float %R315, 0x3F6C5894E0000000
  %R319 = fmul fast float %R316, 0x3F6C5894E0000000
  %R320 = fmul fast float %R317, 0x3F6C5894E0000000
  %R321 = call float @dx.op.unary.f32(i32 29, float %R318)  ; Round_z(value)
  %R322 = call float @dx.op.unary.f32(i32 29, float %R319)  ; Round_z(value)
  %R323 = call float @dx.op.unary.f32(i32 29, float %R320)  ; Round_z(value)
  %R324 = fmul fast float %R321, 2.890000e+02
  %R325 = fmul fast float %R322, 2.890000e+02
  %R326 = fmul fast float %R323, 2.890000e+02
  %R327 = fsub fast float %R315, %R324
  %R328 = fsub fast float %R316, %R325
  %R329 = fsub fast float %R317, %R326
  %R330 = call float @dx.op.dot2.f32(i32 54, float %R262, float %R263, float %R262, float %R263)  ; Dot2(ax,ay,bx,by)
  %R331 = call float @dx.op.dot2.f32(i32 54, float %R271, float %R272, float %R271, float %R272)  ; Dot2(ax,ay,bx,by)
  %R332 = call float @dx.op.dot2.f32(i32 54, float %R269, float %R270, float %R269, float %R270)  ; Dot2(ax,ay,bx,by)
  %R333 = fsub fast float 5.000000e-01, %R330
  %R334 = fsub fast float 5.000000e-01, %R331
  %R335 = fsub fast float 5.000000e-01, %R332
  %R336 = call float @dx.op.binary.f32(i32 35, float %R333, float 0.000000e+00)  ; FMax(a,b)
  %R337 = call float @dx.op.binary.f32(i32 35, float %R334, float 0.000000e+00)  ; FMax(a,b)
  %R338 = call float @dx.op.binary.f32(i32 35, float %R335, float 0.000000e+00)  ; FMax(a,b)
  %R339 = fmul fast float %R336, %R336
  %R340 = fmul fast float %R337, %R337
  %R341 = fmul fast float %R338, %R338
  %R342 = fmul fast float %R339, %R339
  %R343 = fmul fast float %R340, %R340
  %R344 = fmul fast float %R341, %R341
  %R345 = fmul fast float %R327, 0x3F98F9C180000000
  %R346 = fmul fast float %R328, 0x3F98F9C180000000
  %R347 = fmul fast float %R329, 0x3F98F9C180000000
  %R348 = call float @dx.op.unary.f32(i32 22, float %R345)  ; Frc(value)
  %R349 = call float @dx.op.unary.f32(i32 22, float %R346)  ; Frc(value)
  %R350 = call float @dx.op.unary.f32(i32 22, float %R347)  ; Frc(value)
  %R351 = fmul fast float %R348, 2.000000e+00
  %R352 = fmul fast float %R349, 2.000000e+00
  %R353 = fmul fast float %R350, 2.000000e+00
  %R354 = fadd fast float %R351, -1.000000e+00
  %R355 = fadd fast float %R352, -1.000000e+00
  %R356 = fadd fast float %R353, -1.000000e+00
  %R357 = call float @dx.op.unary.f32(i32 6, float %R354)  ; FAbs(value)
  %R358 = call float @dx.op.unary.f32(i32 6, float %R355)  ; FAbs(value)
  %R359 = call float @dx.op.unary.f32(i32 6, float %R356)  ; FAbs(value)
  %R360 = fadd fast float %R357, -5.000000e-01
  %R361 = fadd fast float %R358, -5.000000e-01
  %R362 = fadd fast float %R359, -5.000000e-01
  %R363 = fadd fast float %R351, -5.000000e-01
  %R364 = fadd fast float %R352, -5.000000e-01
  %R365 = fadd fast float %R353, -5.000000e-01
  %R366 = call float @dx.op.unary.f32(i32 27, float %R363)  ; Round_ni(value)
  %R367 = call float @dx.op.unary.f32(i32 27, float %R364)  ; Round_ni(value)
  %R368 = call float @dx.op.unary.f32(i32 27, float %R365)  ; Round_ni(value)
  %R369 = fsub fast float %R354, %R366
  %R370 = fsub fast float %R355, %R367
  %R371 = fsub fast float %R356, %R368
  %R372 = fmul fast float %R369, %R369
  %R373 = fmul fast float %R370, %R370
  %R374 = fmul fast float %R371, %R371
  %R375 = fmul fast float %R360, %R360
  %R376 = fmul fast float %R361, %R361
  %R377 = fmul fast float %R362, %R362
  %R378 = fadd fast float %R372, %R375
  %R379 = fadd fast float %R373, %R376
  %R380 = fadd fast float %R374, %R377
  %R381 = fmul fast float %R378, 0x3FEB51CB80000000
  %R382 = fmul fast float %R379, 0x3FEB51CB80000000
  %R383 = fmul fast float %R380, 0x3FEB51CB80000000
  %R384 = fsub fast float 0x3FFCAF7C00000000, %R381
  %R385 = fsub fast float 0x3FFCAF7C00000000, %R382
  %R386 = fsub fast float 0x3FFCAF7C00000000, %R383
  %R387 = fmul fast float %R342, %R384
  %R388 = fmul fast float %R343, %R385
  %R389 = fmul fast float %R344, %R386
  %R390 = fmul fast float %R369, %R262
  %R391 = fmul fast float %R360, %R263
  %R392 = fadd fast float %R390, %R391
  %R393 = fmul fast float %R370, %R271
  %R394 = fmul fast float %R371, %R269
  %R395 = fmul fast float %R361, %R272
  %R396 = fmul fast float %R362, %R270
  %R397 = fadd fast float %R393, %R395
  %R398 = fadd fast float %R394, %R396
  %R399 = call float @dx.op.dot3.f32(i32 55, float %R387, float %R388, float %R389, float %R392, float %R397, float %R398)  ; Dot3(ax,ay,az,bx,by,bz)
  %R400 = fmul fast float %R241, 1.300000e+02
  %R401 = fmul fast float %R400, %R399
  %R240 = fadd fast float %R401, %R239
  %R402 = fmul fast float %R235, 0x3FEC152800000000
  %R403 = call float @dx.op.tertiary.f32(i32 46, float %R237, float 0xBFDEAEE880000000, float %R402)  ; FMad(a,b,c)
  %R404 = fmul fast float %R235, 0x3FDEAEE880000000
  %R405 = call float @dx.op.tertiary.f32(i32 46, float %R237, float 0x3FEC152800000000, float %R404)  ; FMad(a,b,c)
  %R406 = fmul fast float %R403, 2.000000e+00
  %R407 = fmul fast float %R405, 2.000000e+00
  %R236 = fadd fast float %R406, 1.000000e+02
  %R238 = fadd fast float %R407, 1.000000e+02
  %R242 = fmul fast float %R241, 5.000000e-01
  %R408 = icmp eq i32 %R245, %R251
  %R409 = icmp eq i32 %R248, 0
  %R410 = and i1 %R409, %R408
  br i1 %R410, label %R253, label %R233

; <label>:410                                     ; preds = %R234, %R233
  %R411 = phi float [ %R240, %R234 ], [ %R239, %R233 ]
  %R412 = fsub fast float %R411, %R227
  %R413 = fmul fast float %R228, %R412
  %R414 = fadd fast float %R42, 0x3FB99999A0000000
  %R415 = fmul fast float %R414, 0x3F847AE140000000
  br label %R416

; <label>:416                                     ; preds = %R417, %R253
  %R418 = phi float [ %R419, %R417 ], [ %R46, %R253 ]
  %R420 = phi float [ %R421, %R417 ], [ %R415, %R253 ]
  %R422 = phi float [ %R423, %R417 ], [ 0.000000e+00, %R253 ]
  %R424 = phi float [ %R425, %R417 ], [ 5.000000e-01, %R253 ]
  %R426 = phi i32 [ %R427, %R417 ], [ 0, %R253 ]
  %R428 = phi i32 [ %R429, %R417 ], [ -1, %R253 ]
  %R430 = phi i32 [ %R431, %R417 ], [ -1, %R253 ]
  %R432 = phi i32 [ 1, %R417 ], [ 0, %R253 ]
  %R433 = icmp eq i32 %R430, 0
  %R434 = zext i1 %R433 to i32
  %R431 = add i32 %R430, -1
  %R427 = add i32 %R432, %R426
  %R435 = icmp ult i32 %R427, 5
  br i1 %R435, label %R417, label %R436

; <label>:430                                     ; preds = %R416
  %R429 = sub i32 %R428, %R434
  %R437 = call float @dx.op.dot2.f32(i32 54, float %R418, float %R420, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R438 = fadd fast float %R437, %R418
  %R439 = fadd fast float %R437, %R420
  %R440 = call float @dx.op.unary.f32(i32 27, float %R438)  ; Round_ni(value)
  %R441 = call float @dx.op.unary.f32(i32 27, float %R439)  ; Round_ni(value)
  %R442 = fsub fast float %R418, %R440
  %R443 = fsub fast float %R420, %R441
  %R444 = call float @dx.op.dot2.f32(i32 54, float %R440, float %R441, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R445 = fadd fast float %R444, %R442
  %R446 = fadd fast float %R443, %R444
  %R447 = fcmp olt float %R445, %R446
  %R448 = select i1 %R447, float 0.000000e+00, float 1.000000e+00
  %R449 = select i1 %R447, float 1.000000e+00, float 0.000000e+00
  %R450 = fadd fast float %R445, 0x3FCB0CB180000000
  %R451 = fadd fast float %R446, 0x3FCB0CB180000000
  %R452 = fadd fast float %R445, 0xBFE279A740000000
  %R453 = fadd fast float %R446, 0xBFE279A740000000
  %R454 = fsub fast float %R450, %R448
  %R455 = fsub fast float %R451, %R449
  %R456 = fmul fast float %R440, 0x3F6C5894E0000000
  %R457 = fmul fast float %R441, 0x3F6C5894E0000000
  %R458 = call float @dx.op.unary.f32(i32 29, float %R456)  ; Round_z(value)
  %R459 = call float @dx.op.unary.f32(i32 29, float %R457)  ; Round_z(value)
  %R460 = fmul fast float %R458, 2.890000e+02
  %R461 = fmul fast float %R459, 2.890000e+02
  %R462 = fsub fast float %R440, %R460
  %R463 = fsub fast float %R441, %R461
  %R464 = fadd fast float %R463, %R449
  %R465 = fadd fast float %R463, 1.000000e+00
  %R466 = fmul fast float %R463, 3.400000e+01
  %R467 = fmul fast float %R464, 3.400000e+01
  %R468 = fmul fast float %R465, 3.400000e+01
  %R469 = fadd fast float %R466, 1.000000e+00
  %R470 = fadd fast float %R467, 1.000000e+00
  %R471 = fadd fast float %R468, 1.000000e+00
  %R472 = fmul fast float %R469, %R463
  %R473 = fmul fast float %R470, %R464
  %R474 = fmul fast float %R471, %R465
  %R475 = fmul fast float %R472, 0x3F6C5894E0000000
  %R476 = fmul fast float %R473, 0x3F6C5894E0000000
  %R477 = fmul fast float %R474, 0x3F6C5894E0000000
  %R478 = call float @dx.op.unary.f32(i32 29, float %R475)  ; Round_z(value)
  %R479 = call float @dx.op.unary.f32(i32 29, float %R476)  ; Round_z(value)
  %R480 = call float @dx.op.unary.f32(i32 29, float %R477)  ; Round_z(value)
  %R481 = fmul fast float %R478, 2.890000e+02
  %R482 = fmul fast float %R479, 2.890000e+02
  %R483 = fmul fast float %R480, 2.890000e+02
  %R484 = fsub fast float %R472, %R481
  %R485 = fadd fast float %R484, %R462
  %R486 = fadd fast float %R448, %R462
  %R487 = fsub fast float %R486, %R482
  %R488 = fadd fast float %R487, %R473
  %R489 = fadd fast float %R462, 1.000000e+00
  %R490 = fsub fast float %R489, %R483
  %R491 = fadd fast float %R490, %R474
  %R492 = fmul fast float %R485, 3.400000e+01
  %R493 = fmul fast float %R488, 3.400000e+01
  %R494 = fmul fast float %R491, 3.400000e+01
  %R495 = fadd fast float %R492, 1.000000e+00
  %R496 = fadd fast float %R493, 1.000000e+00
  %R497 = fadd fast float %R494, 1.000000e+00
  %R498 = fmul fast float %R495, %R485
  %R499 = fmul fast float %R496, %R488
  %R500 = fmul fast float %R497, %R491
  %R501 = fmul fast float %R498, 0x3F6C5894E0000000
  %R502 = fmul fast float %R499, 0x3F6C5894E0000000
  %R503 = fmul fast float %R500, 0x3F6C5894E0000000
  %R504 = call float @dx.op.unary.f32(i32 29, float %R501)  ; Round_z(value)
  %R505 = call float @dx.op.unary.f32(i32 29, float %R502)  ; Round_z(value)
  %R506 = call float @dx.op.unary.f32(i32 29, float %R503)  ; Round_z(value)
  %R507 = fmul fast float %R504, 2.890000e+02
  %R508 = fmul fast float %R505, 2.890000e+02
  %R509 = fmul fast float %R506, 2.890000e+02
  %R510 = fsub fast float %R498, %R507
  %R511 = fsub fast float %R499, %R508
  %R512 = fsub fast float %R500, %R509
  %R513 = call float @dx.op.dot2.f32(i32 54, float %R445, float %R446, float %R445, float %R446)  ; Dot2(ax,ay,bx,by)
  %R514 = call float @dx.op.dot2.f32(i32 54, float %R454, float %R455, float %R454, float %R455)  ; Dot2(ax,ay,bx,by)
  %R515 = call float @dx.op.dot2.f32(i32 54, float %R452, float %R453, float %R452, float %R453)  ; Dot2(ax,ay,bx,by)
  %R516 = fsub fast float 5.000000e-01, %R513
  %R517 = fsub fast float 5.000000e-01, %R514
  %R518 = fsub fast float 5.000000e-01, %R515
  %R519 = call float @dx.op.binary.f32(i32 35, float %R516, float 0.000000e+00)  ; FMax(a,b)
  %R520 = call float @dx.op.binary.f32(i32 35, float %R517, float 0.000000e+00)  ; FMax(a,b)
  %R521 = call float @dx.op.binary.f32(i32 35, float %R518, float 0.000000e+00)  ; FMax(a,b)
  %R522 = fmul fast float %R519, %R519
  %R523 = fmul fast float %R520, %R520
  %R524 = fmul fast float %R521, %R521
  %R525 = fmul fast float %R522, %R522
  %R526 = fmul fast float %R523, %R523
  %R527 = fmul fast float %R524, %R524
  %R528 = fmul fast float %R510, 0x3F98F9C180000000
  %R529 = fmul fast float %R511, 0x3F98F9C180000000
  %R530 = fmul fast float %R512, 0x3F98F9C180000000
  %R531 = call float @dx.op.unary.f32(i32 22, float %R528)  ; Frc(value)
  %R532 = call float @dx.op.unary.f32(i32 22, float %R529)  ; Frc(value)
  %R533 = call float @dx.op.unary.f32(i32 22, float %R530)  ; Frc(value)
  %R534 = fmul fast float %R531, 2.000000e+00
  %R535 = fmul fast float %R532, 2.000000e+00
  %R536 = fmul fast float %R533, 2.000000e+00
  %R537 = fadd fast float %R534, -1.000000e+00
  %R538 = fadd fast float %R535, -1.000000e+00
  %R539 = fadd fast float %R536, -1.000000e+00
  %R540 = call float @dx.op.unary.f32(i32 6, float %R537)  ; FAbs(value)
  %R541 = call float @dx.op.unary.f32(i32 6, float %R538)  ; FAbs(value)
  %R542 = call float @dx.op.unary.f32(i32 6, float %R539)  ; FAbs(value)
  %R543 = fadd fast float %R540, -5.000000e-01
  %R544 = fadd fast float %R541, -5.000000e-01
  %R545 = fadd fast float %R542, -5.000000e-01
  %R546 = fadd fast float %R534, -5.000000e-01
  %R547 = fadd fast float %R535, -5.000000e-01
  %R548 = fadd fast float %R536, -5.000000e-01
  %R549 = call float @dx.op.unary.f32(i32 27, float %R546)  ; Round_ni(value)
  %R550 = call float @dx.op.unary.f32(i32 27, float %R547)  ; Round_ni(value)
  %R551 = call float @dx.op.unary.f32(i32 27, float %R548)  ; Round_ni(value)
  %R552 = fsub fast float %R537, %R549
  %R553 = fsub fast float %R538, %R550
  %R554 = fsub fast float %R539, %R551
  %R555 = fmul fast float %R552, %R552
  %R556 = fmul fast float %R553, %R553
  %R557 = fmul fast float %R554, %R554
  %R558 = fmul fast float %R543, %R543
  %R559 = fmul fast float %R544, %R544
  %R560 = fmul fast float %R545, %R545
  %R561 = fadd fast float %R555, %R558
  %R562 = fadd fast float %R556, %R559
  %R563 = fadd fast float %R557, %R560
  %R564 = fmul fast float %R561, 0x3FEB51CB80000000
  %R565 = fmul fast float %R562, 0x3FEB51CB80000000
  %R566 = fmul fast float %R563, 0x3FEB51CB80000000
  %R567 = fsub fast float 0x3FFCAF7C00000000, %R564
  %R568 = fsub fast float 0x3FFCAF7C00000000, %R565
  %R569 = fsub fast float 0x3FFCAF7C00000000, %R566
  %R570 = fmul fast float %R525, %R567
  %R571 = fmul fast float %R526, %R568
  %R572 = fmul fast float %R527, %R569
  %R573 = fmul fast float %R552, %R445
  %R574 = fmul fast float %R543, %R446
  %R575 = fadd fast float %R573, %R574
  %R576 = fmul fast float %R553, %R454
  %R577 = fmul fast float %R554, %R452
  %R578 = fmul fast float %R544, %R455
  %R579 = fmul fast float %R545, %R453
  %R580 = fadd fast float %R576, %R578
  %R581 = fadd fast float %R577, %R579
  %R582 = call float @dx.op.dot3.f32(i32 55, float %R570, float %R571, float %R572, float %R575, float %R580, float %R581)  ; Dot3(ax,ay,az,bx,by,bz)
  %R583 = fmul fast float %R424, 1.300000e+02
  %R584 = fmul fast float %R583, %R582
  %R423 = fadd fast float %R584, %R422
  %R585 = fmul fast float %R418, 0x3FEC152800000000
  %R586 = call float @dx.op.tertiary.f32(i32 46, float %R420, float 0xBFDEAEE880000000, float %R585)  ; FMad(a,b,c)
  %R587 = fmul fast float %R418, 0x3FDEAEE880000000
  %R588 = call float @dx.op.tertiary.f32(i32 46, float %R420, float 0x3FEC152800000000, float %R587)  ; FMad(a,b,c)
  %R589 = fmul fast float %R586, 2.000000e+00
  %R590 = fmul fast float %R588, 2.000000e+00
  %R419 = fadd fast float %R589, 1.000000e+02
  %R421 = fadd fast float %R590, 1.000000e+02
  %R425 = fmul fast float %R424, 5.000000e-01
  %R591 = icmp eq i32 %R428, %R434
  %R592 = icmp eq i32 %R431, 0
  %R593 = and i1 %R592, %R591
  br i1 %R593, label %R436, label %R416

; <label>:593                                     ; preds = %R417, %R416
  %R594 = phi float [ %R423, %R417 ], [ %R422, %R416 ]
  %R595 = fsub fast float %R594, %R227
  %R596 = fmul fast float %R228, %R595
  %R597 = fadd fast float %R41, 0xBFB99999A0000000
  %R598 = fmul fast float %R597, 0x3F847AE140000000
  br label %R599

; <label>:599                                     ; preds = %R600, %R436
  %R601 = phi float [ %R602, %R600 ], [ %R598, %R436 ]
  %R603 = phi float [ %R604, %R600 ], [ %R47, %R436 ]
  %R605 = phi float [ %R606, %R600 ], [ 0.000000e+00, %R436 ]
  %R607 = phi float [ %R608, %R600 ], [ 5.000000e-01, %R436 ]
  %R609 = phi i32 [ %R610, %R600 ], [ 0, %R436 ]
  %R611 = phi i32 [ %R612, %R600 ], [ -1, %R436 ]
  %R613 = phi i32 [ %R614, %R600 ], [ -1, %R436 ]
  %R615 = phi i32 [ 1, %R600 ], [ 0, %R436 ]
  %R616 = icmp eq i32 %R613, 0
  %R617 = zext i1 %R616 to i32
  %R614 = add i32 %R613, -1
  %R610 = add i32 %R615, %R609
  %R618 = icmp ult i32 %R610, 5
  br i1 %R618, label %R600, label %R619

; <label>:613                                     ; preds = %R599
  %R612 = sub i32 %R611, %R617
  %R620 = call float @dx.op.dot2.f32(i32 54, float %R601, float %R603, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R621 = fadd fast float %R620, %R601
  %R622 = fadd fast float %R620, %R603
  %R623 = call float @dx.op.unary.f32(i32 27, float %R621)  ; Round_ni(value)
  %R624 = call float @dx.op.unary.f32(i32 27, float %R622)  ; Round_ni(value)
  %R625 = fsub fast float %R601, %R623
  %R626 = fsub fast float %R603, %R624
  %R627 = call float @dx.op.dot2.f32(i32 54, float %R623, float %R624, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R628 = fadd fast float %R627, %R625
  %R629 = fadd fast float %R626, %R627
  %R630 = fcmp olt float %R628, %R629
  %R631 = select i1 %R630, float 0.000000e+00, float 1.000000e+00
  %R632 = select i1 %R630, float 1.000000e+00, float 0.000000e+00
  %R633 = fadd fast float %R628, 0x3FCB0CB180000000
  %R634 = fadd fast float %R629, 0x3FCB0CB180000000
  %R635 = fadd fast float %R628, 0xBFE279A740000000
  %R636 = fadd fast float %R629, 0xBFE279A740000000
  %R637 = fsub fast float %R633, %R631
  %R638 = fsub fast float %R634, %R632
  %R639 = fmul fast float %R623, 0x3F6C5894E0000000
  %R640 = fmul fast float %R624, 0x3F6C5894E0000000
  %R641 = call float @dx.op.unary.f32(i32 29, float %R639)  ; Round_z(value)
  %R642 = call float @dx.op.unary.f32(i32 29, float %R640)  ; Round_z(value)
  %R643 = fmul fast float %R641, 2.890000e+02
  %R644 = fmul fast float %R642, 2.890000e+02
  %R645 = fsub fast float %R623, %R643
  %R646 = fsub fast float %R624, %R644
  %R647 = fadd fast float %R646, %R632
  %R648 = fadd fast float %R646, 1.000000e+00
  %R649 = fmul fast float %R646, 3.400000e+01
  %R650 = fmul fast float %R647, 3.400000e+01
  %R651 = fmul fast float %R648, 3.400000e+01
  %R652 = fadd fast float %R649, 1.000000e+00
  %R653 = fadd fast float %R650, 1.000000e+00
  %R654 = fadd fast float %R651, 1.000000e+00
  %R655 = fmul fast float %R652, %R646
  %R656 = fmul fast float %R653, %R647
  %R657 = fmul fast float %R654, %R648
  %R658 = fmul fast float %R655, 0x3F6C5894E0000000
  %R659 = fmul fast float %R656, 0x3F6C5894E0000000
  %R660 = fmul fast float %R657, 0x3F6C5894E0000000
  %R661 = call float @dx.op.unary.f32(i32 29, float %R658)  ; Round_z(value)
  %R662 = call float @dx.op.unary.f32(i32 29, float %R659)  ; Round_z(value)
  %R663 = call float @dx.op.unary.f32(i32 29, float %R660)  ; Round_z(value)
  %R664 = fmul fast float %R661, 2.890000e+02
  %R665 = fmul fast float %R662, 2.890000e+02
  %R666 = fmul fast float %R663, 2.890000e+02
  %R667 = fsub fast float %R655, %R664
  %R668 = fadd fast float %R667, %R645
  %R669 = fadd fast float %R631, %R645
  %R670 = fsub fast float %R669, %R665
  %R671 = fadd fast float %R670, %R656
  %R672 = fadd fast float %R645, 1.000000e+00
  %R673 = fsub fast float %R672, %R666
  %R674 = fadd fast float %R673, %R657
  %R675 = fmul fast float %R668, 3.400000e+01
  %R676 = fmul fast float %R671, 3.400000e+01
  %R677 = fmul fast float %R674, 3.400000e+01
  %R678 = fadd fast float %R675, 1.000000e+00
  %R679 = fadd fast float %R676, 1.000000e+00
  %R680 = fadd fast float %R677, 1.000000e+00
  %R681 = fmul fast float %R678, %R668
  %R682 = fmul fast float %R679, %R671
  %R683 = fmul fast float %R680, %R674
  %R684 = fmul fast float %R681, 0x3F6C5894E0000000
  %R685 = fmul fast float %R682, 0x3F6C5894E0000000
  %R686 = fmul fast float %R683, 0x3F6C5894E0000000
  %R687 = call float @dx.op.unary.f32(i32 29, float %R684)  ; Round_z(value)
  %R688 = call float @dx.op.unary.f32(i32 29, float %R685)  ; Round_z(value)
  %R689 = call float @dx.op.unary.f32(i32 29, float %R686)  ; Round_z(value)
  %R690 = fmul fast float %R687, 2.890000e+02
  %R691 = fmul fast float %R688, 2.890000e+02
  %R692 = fmul fast float %R689, 2.890000e+02
  %R693 = fsub fast float %R681, %R690
  %R694 = fsub fast float %R682, %R691
  %R695 = fsub fast float %R683, %R692
  %R696 = call float @dx.op.dot2.f32(i32 54, float %R628, float %R629, float %R628, float %R629)  ; Dot2(ax,ay,bx,by)
  %R697 = call float @dx.op.dot2.f32(i32 54, float %R637, float %R638, float %R637, float %R638)  ; Dot2(ax,ay,bx,by)
  %R698 = call float @dx.op.dot2.f32(i32 54, float %R635, float %R636, float %R635, float %R636)  ; Dot2(ax,ay,bx,by)
  %R699 = fsub fast float 5.000000e-01, %R696
  %R700 = fsub fast float 5.000000e-01, %R697
  %R701 = fsub fast float 5.000000e-01, %R698
  %R702 = call float @dx.op.binary.f32(i32 35, float %R699, float 0.000000e+00)  ; FMax(a,b)
  %R703 = call float @dx.op.binary.f32(i32 35, float %R700, float 0.000000e+00)  ; FMax(a,b)
  %R704 = call float @dx.op.binary.f32(i32 35, float %R701, float 0.000000e+00)  ; FMax(a,b)
  %R705 = fmul fast float %R702, %R702
  %R706 = fmul fast float %R703, %R703
  %R707 = fmul fast float %R704, %R704
  %R708 = fmul fast float %R705, %R705
  %R709 = fmul fast float %R706, %R706
  %R710 = fmul fast float %R707, %R707
  %R711 = fmul fast float %R693, 0x3F98F9C180000000
  %R712 = fmul fast float %R694, 0x3F98F9C180000000
  %R713 = fmul fast float %R695, 0x3F98F9C180000000
  %R714 = call float @dx.op.unary.f32(i32 22, float %R711)  ; Frc(value)
  %R715 = call float @dx.op.unary.f32(i32 22, float %R712)  ; Frc(value)
  %R716 = call float @dx.op.unary.f32(i32 22, float %R713)  ; Frc(value)
  %R717 = fmul fast float %R714, 2.000000e+00
  %R718 = fmul fast float %R715, 2.000000e+00
  %R719 = fmul fast float %R716, 2.000000e+00
  %R720 = fadd fast float %R717, -1.000000e+00
  %R721 = fadd fast float %R718, -1.000000e+00
  %R722 = fadd fast float %R719, -1.000000e+00
  %R723 = call float @dx.op.unary.f32(i32 6, float %R720)  ; FAbs(value)
  %R724 = call float @dx.op.unary.f32(i32 6, float %R721)  ; FAbs(value)
  %R725 = call float @dx.op.unary.f32(i32 6, float %R722)  ; FAbs(value)
  %R726 = fadd fast float %R723, -5.000000e-01
  %R727 = fadd fast float %R724, -5.000000e-01
  %R728 = fadd fast float %R725, -5.000000e-01
  %R729 = fadd fast float %R717, -5.000000e-01
  %R730 = fadd fast float %R718, -5.000000e-01
  %R731 = fadd fast float %R719, -5.000000e-01
  %R732 = call float @dx.op.unary.f32(i32 27, float %R729)  ; Round_ni(value)
  %R733 = call float @dx.op.unary.f32(i32 27, float %R730)  ; Round_ni(value)
  %R734 = call float @dx.op.unary.f32(i32 27, float %R731)  ; Round_ni(value)
  %R735 = fsub fast float %R720, %R732
  %R736 = fsub fast float %R721, %R733
  %R737 = fsub fast float %R722, %R734
  %R738 = fmul fast float %R735, %R735
  %R739 = fmul fast float %R736, %R736
  %R740 = fmul fast float %R737, %R737
  %R741 = fmul fast float %R726, %R726
  %R742 = fmul fast float %R727, %R727
  %R743 = fmul fast float %R728, %R728
  %R744 = fadd fast float %R738, %R741
  %R745 = fadd fast float %R739, %R742
  %R746 = fadd fast float %R740, %R743
  %R747 = fmul fast float %R744, 0x3FEB51CB80000000
  %R748 = fmul fast float %R745, 0x3FEB51CB80000000
  %R749 = fmul fast float %R746, 0x3FEB51CB80000000
  %R750 = fsub fast float 0x3FFCAF7C00000000, %R747
  %R751 = fsub fast float 0x3FFCAF7C00000000, %R748
  %R752 = fsub fast float 0x3FFCAF7C00000000, %R749
  %R753 = fmul fast float %R708, %R750
  %R754 = fmul fast float %R709, %R751
  %R755 = fmul fast float %R710, %R752
  %R756 = fmul fast float %R735, %R628
  %R757 = fmul fast float %R726, %R629
  %R758 = fadd fast float %R756, %R757
  %R759 = fmul fast float %R736, %R637
  %R760 = fmul fast float %R737, %R635
  %R761 = fmul fast float %R727, %R638
  %R762 = fmul fast float %R728, %R636
  %R763 = fadd fast float %R759, %R761
  %R764 = fadd fast float %R760, %R762
  %R765 = call float @dx.op.dot3.f32(i32 55, float %R753, float %R754, float %R755, float %R758, float %R763, float %R764)  ; Dot3(ax,ay,az,bx,by,bz)
  %R766 = fmul fast float %R607, 1.300000e+02
  %R767 = fmul fast float %R766, %R765
  %R606 = fadd fast float %R767, %R605
  %R768 = fmul fast float %R601, 0x3FEC152800000000
  %R769 = call float @dx.op.tertiary.f32(i32 46, float %R603, float 0xBFDEAEE880000000, float %R768)  ; FMad(a,b,c)
  %R770 = fmul fast float %R601, 0x3FDEAEE880000000
  %R771 = call float @dx.op.tertiary.f32(i32 46, float %R603, float 0x3FEC152800000000, float %R770)  ; FMad(a,b,c)
  %R772 = fmul fast float %R769, 2.000000e+00
  %R773 = fmul fast float %R771, 2.000000e+00
  %R602 = fadd fast float %R772, 1.000000e+02
  %R604 = fadd fast float %R773, 1.000000e+02
  %R608 = fmul fast float %R607, 5.000000e-01
  %R774 = icmp eq i32 %R611, %R617
  %R775 = icmp eq i32 %R614, 0
  %R776 = and i1 %R775, %R774
  br i1 %R776, label %R619, label %R599

; <label>:776                                     ; preds = %R600, %R599
  %R777 = phi float [ %R606, %R600 ], [ %R605, %R599 ]
  %R778 = fsub fast float %R777, %R227
  %R779 = fmul fast float %R228, %R778
  %R780 = fadd fast float %R42, 0xBFB99999A0000000
  %R781 = fmul fast float %R780, 0x3F847AE140000000
  br label %R782

; <label>:782                                     ; preds = %R783, %R619
  %R784 = phi float [ %R785, %R783 ], [ %R46, %R619 ]
  %R786 = phi float [ %R787, %R783 ], [ %R781, %R619 ]
  %R788 = phi float [ %R789, %R783 ], [ 0.000000e+00, %R619 ]
  %R790 = phi float [ %R791, %R783 ], [ 5.000000e-01, %R619 ]
  %R792 = phi i32 [ %R793, %R783 ], [ 0, %R619 ]
  %R794 = phi i32 [ %R795, %R783 ], [ -1, %R619 ]
  %R796 = phi i32 [ %R797, %R783 ], [ -1, %R619 ]
  %R798 = phi i32 [ 1, %R783 ], [ 0, %R619 ]
  %R799 = icmp eq i32 %R796, 0
  %R800 = zext i1 %R799 to i32
  %R797 = add i32 %R796, -1
  %R793 = add i32 %R798, %R792
  %R801 = icmp ult i32 %R793, 5
  br i1 %R801, label %R783, label %R802

; <label>:796                                     ; preds = %R782
  %R795 = sub i32 %R794, %R800
  %R803 = call float @dx.op.dot2.f32(i32 54, float %R784, float %R786, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R804 = fadd fast float %R803, %R784
  %R805 = fadd fast float %R803, %R786
  %R806 = call float @dx.op.unary.f32(i32 27, float %R804)  ; Round_ni(value)
  %R807 = call float @dx.op.unary.f32(i32 27, float %R805)  ; Round_ni(value)
  %R808 = fsub fast float %R784, %R806
  %R809 = fsub fast float %R786, %R807
  %R810 = call float @dx.op.dot2.f32(i32 54, float %R806, float %R807, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R811 = fadd fast float %R810, %R808
  %R812 = fadd fast float %R809, %R810
  %R813 = fcmp olt float %R811, %R812
  %R814 = select i1 %R813, float 0.000000e+00, float 1.000000e+00
  %R815 = select i1 %R813, float 1.000000e+00, float 0.000000e+00
  %R816 = fadd fast float %R811, 0x3FCB0CB180000000
  %R817 = fadd fast float %R812, 0x3FCB0CB180000000
  %R818 = fadd fast float %R811, 0xBFE279A740000000
  %R819 = fadd fast float %R812, 0xBFE279A740000000
  %R820 = fsub fast float %R816, %R814
  %R821 = fsub fast float %R817, %R815
  %R822 = fmul fast float %R806, 0x3F6C5894E0000000
  %R823 = fmul fast float %R807, 0x3F6C5894E0000000
  %R824 = call float @dx.op.unary.f32(i32 29, float %R822)  ; Round_z(value)
  %R825 = call float @dx.op.unary.f32(i32 29, float %R823)  ; Round_z(value)
  %R826 = fmul fast float %R824, 2.890000e+02
  %R827 = fmul fast float %R825, 2.890000e+02
  %R828 = fsub fast float %R806, %R826
  %R829 = fsub fast float %R807, %R827
  %R830 = fadd fast float %R829, %R815
  %R831 = fadd fast float %R829, 1.000000e+00
  %R832 = fmul fast float %R829, 3.400000e+01
  %R833 = fmul fast float %R830, 3.400000e+01
  %R834 = fmul fast float %R831, 3.400000e+01
  %R835 = fadd fast float %R832, 1.000000e+00
  %R836 = fadd fast float %R833, 1.000000e+00
  %R837 = fadd fast float %R834, 1.000000e+00
  %R838 = fmul fast float %R835, %R829
  %R839 = fmul fast float %R836, %R830
  %R840 = fmul fast float %R837, %R831
  %R841 = fmul fast float %R838, 0x3F6C5894E0000000
  %R842 = fmul fast float %R839, 0x3F6C5894E0000000
  %R843 = fmul fast float %R840, 0x3F6C5894E0000000
  %R844 = call float @dx.op.unary.f32(i32 29, float %R841)  ; Round_z(value)
  %R845 = call float @dx.op.unary.f32(i32 29, float %R842)  ; Round_z(value)
  %R846 = call float @dx.op.unary.f32(i32 29, float %R843)  ; Round_z(value)
  %R847 = fmul fast float %R844, 2.890000e+02
  %R848 = fmul fast float %R845, 2.890000e+02
  %R849 = fmul fast float %R846, 2.890000e+02
  %R850 = fsub fast float %R838, %R847
  %R851 = fadd fast float %R850, %R828
  %R852 = fadd fast float %R814, %R828
  %R853 = fsub fast float %R852, %R848
  %R854 = fadd fast float %R853, %R839
  %R855 = fadd fast float %R828, 1.000000e+00
  %R856 = fsub fast float %R855, %R849
  %R857 = fadd fast float %R856, %R840
  %R858 = fmul fast float %R851, 3.400000e+01
  %R859 = fmul fast float %R854, 3.400000e+01
  %R860 = fmul fast float %R857, 3.400000e+01
  %R861 = fadd fast float %R858, 1.000000e+00
  %R862 = fadd fast float %R859, 1.000000e+00
  %R863 = fadd fast float %R860, 1.000000e+00
  %R864 = fmul fast float %R861, %R851
  %R865 = fmul fast float %R862, %R854
  %R866 = fmul fast float %R863, %R857
  %R867 = fmul fast float %R864, 0x3F6C5894E0000000
  %R868 = fmul fast float %R865, 0x3F6C5894E0000000
  %R869 = fmul fast float %R866, 0x3F6C5894E0000000
  %R870 = call float @dx.op.unary.f32(i32 29, float %R867)  ; Round_z(value)
  %R871 = call float @dx.op.unary.f32(i32 29, float %R868)  ; Round_z(value)
  %R872 = call float @dx.op.unary.f32(i32 29, float %R869)  ; Round_z(value)
  %R873 = fmul fast float %R870, 2.890000e+02
  %R874 = fmul fast float %R871, 2.890000e+02
  %R875 = fmul fast float %R872, 2.890000e+02
  %R876 = fsub fast float %R864, %R873
  %R877 = fsub fast float %R865, %R874
  %R878 = fsub fast float %R866, %R875
  %R879 = call float @dx.op.dot2.f32(i32 54, float %R811, float %R812, float %R811, float %R812)  ; Dot2(ax,ay,bx,by)
  %R880 = call float @dx.op.dot2.f32(i32 54, float %R820, float %R821, float %R820, float %R821)  ; Dot2(ax,ay,bx,by)
  %R881 = call float @dx.op.dot2.f32(i32 54, float %R818, float %R819, float %R818, float %R819)  ; Dot2(ax,ay,bx,by)
  %R882 = fsub fast float 5.000000e-01, %R879
  %R883 = fsub fast float 5.000000e-01, %R880
  %R884 = fsub fast float 5.000000e-01, %R881
  %R885 = call float @dx.op.binary.f32(i32 35, float %R882, float 0.000000e+00)  ; FMax(a,b)
  %R886 = call float @dx.op.binary.f32(i32 35, float %R883, float 0.000000e+00)  ; FMax(a,b)
  %R887 = call float @dx.op.binary.f32(i32 35, float %R884, float 0.000000e+00)  ; FMax(a,b)
  %R888 = fmul fast float %R885, %R885
  %R889 = fmul fast float %R886, %R886
  %R890 = fmul fast float %R887, %R887
  %R891 = fmul fast float %R888, %R888
  %R892 = fmul fast float %R889, %R889
  %R893 = fmul fast float %R890, %R890
  %R894 = fmul fast float %R876, 0x3F98F9C180000000
  %R895 = fmul fast float %R877, 0x3F98F9C180000000
  %R896 = fmul fast float %R878, 0x3F98F9C180000000
  %R897 = call float @dx.op.unary.f32(i32 22, float %R894)  ; Frc(value)
  %R898 = call float @dx.op.unary.f32(i32 22, float %R895)  ; Frc(value)
  %R899 = call float @dx.op.unary.f32(i32 22, float %R896)  ; Frc(value)
  %R900 = fmul fast float %R897, 2.000000e+00
  %R901 = fmul fast float %R898, 2.000000e+00
  %R902 = fmul fast float %R899, 2.000000e+00
  %R903 = fadd fast float %R900, -1.000000e+00
  %R904 = fadd fast float %R901, -1.000000e+00
  %R905 = fadd fast float %R902, -1.000000e+00
  %R906 = call float @dx.op.unary.f32(i32 6, float %R903)  ; FAbs(value)
  %R907 = call float @dx.op.unary.f32(i32 6, float %R904)  ; FAbs(value)
  %R908 = call float @dx.op.unary.f32(i32 6, float %R905)  ; FAbs(value)
  %R909 = fadd fast float %R906, -5.000000e-01
  %R910 = fadd fast float %R907, -5.000000e-01
  %R911 = fadd fast float %R908, -5.000000e-01
  %R912 = fadd fast float %R900, -5.000000e-01
  %R913 = fadd fast float %R901, -5.000000e-01
  %R914 = fadd fast float %R902, -5.000000e-01
  %R915 = call float @dx.op.unary.f32(i32 27, float %R912)  ; Round_ni(value)
  %R916 = call float @dx.op.unary.f32(i32 27, float %R913)  ; Round_ni(value)
  %R917 = call float @dx.op.unary.f32(i32 27, float %R914)  ; Round_ni(value)
  %R918 = fsub fast float %R903, %R915
  %R919 = fsub fast float %R904, %R916
  %R920 = fsub fast float %R905, %R917
  %R921 = fmul fast float %R918, %R918
  %R922 = fmul fast float %R919, %R919
  %R923 = fmul fast float %R920, %R920
  %R924 = fmul fast float %R909, %R909
  %R925 = fmul fast float %R910, %R910
  %R926 = fmul fast float %R911, %R911
  %R927 = fadd fast float %R921, %R924
  %R928 = fadd fast float %R922, %R925
  %R929 = fadd fast float %R923, %R926
  %R930 = fmul fast float %R927, 0x3FEB51CB80000000
  %R931 = fmul fast float %R928, 0x3FEB51CB80000000
  %R932 = fmul fast float %R929, 0x3FEB51CB80000000
  %R933 = fsub fast float 0x3FFCAF7C00000000, %R930
  %R934 = fsub fast float 0x3FFCAF7C00000000, %R931
  %R935 = fsub fast float 0x3FFCAF7C00000000, %R932
  %R936 = fmul fast float %R891, %R933
  %R937 = fmul fast float %R892, %R934
  %R938 = fmul fast float %R893, %R935
  %R939 = fmul fast float %R918, %R811
  %R940 = fmul fast float %R909, %R812
  %R941 = fadd fast float %R939, %R940
  %R942 = fmul fast float %R919, %R820
  %R943 = fmul fast float %R920, %R818
  %R944 = fmul fast float %R910, %R821
  %R945 = fmul fast float %R911, %R819
  %R946 = fadd fast float %R942, %R944
  %R947 = fadd fast float %R943, %R945
  %R948 = call float @dx.op.dot3.f32(i32 55, float %R936, float %R937, float %R938, float %R941, float %R946, float %R947)  ; Dot3(ax,ay,az,bx,by,bz)
  %R949 = fmul fast float %R790, 1.300000e+02
  %R950 = fmul fast float %R949, %R948
  %R789 = fadd fast float %R950, %R788
  %R951 = fmul fast float %R784, 0x3FEC152800000000
  %R952 = call float @dx.op.tertiary.f32(i32 46, float %R786, float 0xBFDEAEE880000000, float %R951)  ; FMad(a,b,c)
  %R953 = fmul fast float %R784, 0x3FDEAEE880000000
  %R954 = call float @dx.op.tertiary.f32(i32 46, float %R786, float 0x3FEC152800000000, float %R953)  ; FMad(a,b,c)
  %R955 = fmul fast float %R952, 2.000000e+00
  %R956 = fmul fast float %R954, 2.000000e+00
  %R785 = fadd fast float %R955, 1.000000e+02
  %R787 = fadd fast float %R956, 1.000000e+02
  %R791 = fmul fast float %R790, 5.000000e-01
  %R957 = icmp eq i32 %R794, %R800
  %R958 = icmp eq i32 %R797, 0
  %R959 = and i1 %R958, %R957
  br i1 %R959, label %R802, label %R782

; <label>:959                                     ; preds = %R783, %R782
  %R960 = phi float [ %R788, %R782 ], [ %R789, %R783 ]
  %R961 = fsub fast float %R960, %R227
  %R962 = fmul fast float %R228, %R961
  %R963 = fmul fast float %R413, 0xBFB99999A0000000
  %R964 = fmul fast float %R596, 0xBFB99999A0000000
  %R965 = call float @dx.op.dot3.f32(i32 55, float %R963, float 0x3F847AE160000000, float %R964, float %R963, float 0x3F847AE160000000, float %R964)  ; Dot3(ax,ay,az,bx,by,bz)
  %R966 = call float @dx.op.unary.f32(i32 25, float %R965)  ; Rsqrt(value)
  %R967 = fmul fast float %R779, 0x3FB99999A0000000
  %R968 = fmul fast float %R962, 0x3FB99999A0000000
  %R969 = call float @dx.op.dot3.f32(i32 55, float %R967, float 0x3F847AE160000000, float %R968, float %R967, float 0x3F847AE160000000, float %R968)  ; Dot3(ax,ay,az,bx,by,bz)
  %R970 = call float @dx.op.unary.f32(i32 25, float %R969)  ; Rsqrt(value)
  switch i32 %R23, label %R971 [
    i32 0, label %R972
    i32 1, label %R973
    i32 2, label %R974
    i32 3, label %R975
    i32 4, label %R976
    i32 5, label %R977
  ]

; <label>:971                                     ; preds = %R802
  br label %R971

; <label>:972                                     ; preds = %R802
  br label %R971

; <label>:973                                     ; preds = %R802
  br label %R971

; <label>:974                                     ; preds = %R802
  %R978 = fmul fast float %R966, %R963
  %R979 = fmul fast float %R970, %R967
  %R980 = fadd fast float %R979, %R978
  %R981 = fmul fast float %R980, 5.000000e-01
  br label %R971

; <label>:979                                     ; preds = %R802
  %R982 = fadd fast float %R970, %R966
  %R983 = fmul fast float %R982, 0x3F747AE160000000
  br label %R971

; <label>:982                                     ; preds = %R802
  %R984 = fmul fast float %R966, %R964
  %R985 = fmul fast float %R970, %R968
  %R986 = fadd fast float %R985, %R984
  %R987 = fmul fast float %R986, 5.000000e-01
  br label %R971

; <label>:987                                     ; preds = %R977, %R976, %R975, %R974, %R973, %R972, %R802
  %R988 = phi float [ 0.000000e+00, %R802 ], [ %R987, %R977 ], [ %R983, %R976 ], [ %R981, %R975 ], [ %R42, %R974 ], [ %R230, %R973 ], [ %R41, %R972 ]
  %R989 = bitcast float %R988 to i32
  call void @dx.op.storeOutput.i32(i32 5, i32 0, i32 0, i8 0, i32 %R989)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 1, i32 0, i8 0, i32 %R3)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readnone
declare float @dx.op.dot2.f32(i32, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.dot3.f32(i32, float, float, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.i32(i32, i32, i32, i8, i32) #A2

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
!M4 = !{null, null, !M7, null}
!M7 = !{!M8}
!M8 = !{i32 0, %gen_data* undef, !"", i32 0, i32 0, i32 1, i32 32, null}
!M5 = !{[14 x i32] [i32 12, i32 5, i32 16, i32 0, i32 0, i32 0, i32 1, i32 1, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0]}
!M6 = !{void ()* @gen_terrain_fragment, !"gen_terrain_fragment", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12, !M13, !M14}
!M12 = !{i32 0, !"LOC", i8 5, i8 0, !M15, i8 1, i32 1, i8 1, i32 0, i8 0, !M16}
!M15 = !{i32 0}
!M16 = !{i32 3, i32 1}
!M13 = !{i32 1, !"LOC", i8 9, i8 0, !M17, i8 2, i32 1, i8 2, i32 1, i8 0, !M18}
!M17 = !{i32 1}
!M18 = !{i32 3, i32 3}
!M14 = !{i32 2, !"SV_Position", i8 9, i8 3, !M15, i8 4, i32 1, i8 4, i32 2, i8 0, null}
!M11 = !{!M19, !M20}
!M19 = !{i32 0, !"SV_Target", i8 5, i8 16, !M15, i8 0, i32 1, i8 1, i32 0, i8 0, !M16}
!M20 = !{i32 1, !"SV_Target", i8 5, i8 16, !M17, i8 0, i32 1, i8 1, i32 1, i8 0, !M16}

