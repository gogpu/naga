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
; no parameters
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Compute Shader
; NumThreads=(64,1,1)
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 0
; SigOutputElements: 0
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 0
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: gen_terrain_compute
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; chunk_data                        cbuffer      NA          NA     CB0            cb0     1
; vertices_                             UAV    byte         r/w      U0             u1     1
; indices_                              UAV    byte         r/w      U1             u2     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.i32 = type { i32, i32, i32, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%struct.S0 = type { i32 }
%chunk_data = type { %struct.S1 }
%struct.S1 = type { <2 x i32>, <2 x i32>, <2 x float> }

define void @gen_terrain_compute() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 1, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R4 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R5 = extractvalue %dx.types.CBufRet.i32 %R4, 0
  %R6 = extractvalue %dx.types.CBufRet.i32 %R4, 2
  %R7 = extractvalue %dx.types.CBufRet.i32 %R4, 3
  %R8 = add i32 %R5, 1
  %R9 = uitofp i32 %R8 to float
  %R10 = uitofp i32 %R3 to float
  %R11 = fdiv fast float %R10, %R9
  %R12 = call float @dx.op.unary.f32(i32 29, float %R11)  ; Round_z(value)
  %R13 = fmul fast float %R9, %R12
  %R14 = fsub fast float %R10, %R13
  %R15 = icmp eq i32 %R8, 0
  %R16 = select i1 %R15, i32 1, i32 %R8
  %R17 = udiv i32 %R3, %R16
  %R18 = uitofp i32 %R17 to float
  %R19 = sitofp i32 %R6 to float
  %R20 = sitofp i32 %R7 to float
  %R21 = fadd fast float %R14, %R19
  %R22 = fadd fast float %R18, %R20
  %R23 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R24 = extractvalue %dx.types.CBufRet.f32 %R23, 0
  %R25 = extractvalue %dx.types.CBufRet.f32 %R23, 1
  %R26 = fmul fast float %R21, 0x3F847AE140000000
  %R27 = fmul fast float %R22, 0x3F847AE140000000
  br label %R28

; <label>:29                                      ; preds = %R29, %R30
  %R31 = phi float [ %R32, %R29 ], [ %R26, %R30 ]
  %R33 = phi float [ %R34, %R29 ], [ %R27, %R30 ]
  %R35 = phi float [ %R36, %R29 ], [ 0.000000e+00, %R30 ]
  %R37 = phi float [ %R38, %R29 ], [ 5.000000e-01, %R30 ]
  %R39 = phi i32 [ %R40, %R29 ], [ 0, %R30 ]
  %R41 = phi i32 [ %R42, %R29 ], [ -1, %R30 ]
  %R43 = phi i32 [ %R44, %R29 ], [ -1, %R30 ]
  %R45 = phi i32 [ 1, %R29 ], [ 0, %R30 ]
  %R46 = icmp eq i32 %R43, 0
  %R47 = zext i1 %R46 to i32
  %R44 = add i32 %R43, -1
  %R40 = add i32 %R45, %R39
  %R48 = icmp ult i32 %R40, 5
  br i1 %R48, label %R29, label %R49

; <label>:43                                      ; preds = %R28
  %R42 = sub i32 %R41, %R47
  %R50 = call float @dx.op.dot2.f32(i32 54, float %R31, float %R33, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R51 = fadd fast float %R50, %R31
  %R52 = fadd fast float %R50, %R33
  %R53 = call float @dx.op.unary.f32(i32 27, float %R51)  ; Round_ni(value)
  %R54 = call float @dx.op.unary.f32(i32 27, float %R52)  ; Round_ni(value)
  %R55 = fsub fast float %R31, %R53
  %R56 = fsub fast float %R33, %R54
  %R57 = call float @dx.op.dot2.f32(i32 54, float %R53, float %R54, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R58 = fadd fast float %R57, %R55
  %R59 = fadd fast float %R56, %R57
  %R60 = fcmp olt float %R58, %R59
  %R61 = select i1 %R60, float 0.000000e+00, float 1.000000e+00
  %R62 = select i1 %R60, float 1.000000e+00, float 0.000000e+00
  %R63 = fadd fast float %R58, 0x3FCB0CB180000000
  %R64 = fadd fast float %R59, 0x3FCB0CB180000000
  %R65 = fadd fast float %R58, 0xBFE279A740000000
  %R66 = fadd fast float %R59, 0xBFE279A740000000
  %R67 = fsub fast float %R63, %R61
  %R68 = fsub fast float %R64, %R62
  %R69 = fmul fast float %R53, 0x3F6C5894E0000000
  %R70 = fmul fast float %R54, 0x3F6C5894E0000000
  %R71 = call float @dx.op.unary.f32(i32 29, float %R69)  ; Round_z(value)
  %R72 = call float @dx.op.unary.f32(i32 29, float %R70)  ; Round_z(value)
  %R73 = fmul fast float %R71, 2.890000e+02
  %R74 = fmul fast float %R72, 2.890000e+02
  %R75 = fsub fast float %R53, %R73
  %R76 = fsub fast float %R54, %R74
  %R77 = fadd fast float %R76, %R62
  %R78 = fadd fast float %R76, 1.000000e+00
  %R79 = fmul fast float %R76, 3.400000e+01
  %R80 = fmul fast float %R77, 3.400000e+01
  %R81 = fmul fast float %R78, 3.400000e+01
  %R82 = fadd fast float %R79, 1.000000e+00
  %R83 = fadd fast float %R80, 1.000000e+00
  %R84 = fadd fast float %R81, 1.000000e+00
  %R85 = fmul fast float %R82, %R76
  %R86 = fmul fast float %R83, %R77
  %R87 = fmul fast float %R84, %R78
  %R88 = fmul fast float %R85, 0x3F6C5894E0000000
  %R89 = fmul fast float %R86, 0x3F6C5894E0000000
  %R90 = fmul fast float %R87, 0x3F6C5894E0000000
  %R91 = call float @dx.op.unary.f32(i32 29, float %R88)  ; Round_z(value)
  %R92 = call float @dx.op.unary.f32(i32 29, float %R89)  ; Round_z(value)
  %R93 = call float @dx.op.unary.f32(i32 29, float %R90)  ; Round_z(value)
  %R94 = fmul fast float %R91, 2.890000e+02
  %R95 = fmul fast float %R92, 2.890000e+02
  %R96 = fmul fast float %R93, 2.890000e+02
  %R97 = fsub fast float %R85, %R94
  %R98 = fadd fast float %R97, %R75
  %R99 = fadd fast float %R61, %R75
  %R100 = fsub fast float %R99, %R95
  %R101 = fadd fast float %R100, %R86
  %R102 = fadd fast float %R75, 1.000000e+00
  %R103 = fsub fast float %R102, %R96
  %R104 = fadd fast float %R103, %R87
  %R105 = fmul fast float %R98, 3.400000e+01
  %R106 = fmul fast float %R101, 3.400000e+01
  %R107 = fmul fast float %R104, 3.400000e+01
  %R108 = fadd fast float %R105, 1.000000e+00
  %R109 = fadd fast float %R106, 1.000000e+00
  %R110 = fadd fast float %R107, 1.000000e+00
  %R111 = fmul fast float %R108, %R98
  %R112 = fmul fast float %R109, %R101
  %R113 = fmul fast float %R110, %R104
  %R114 = fmul fast float %R111, 0x3F6C5894E0000000
  %R115 = fmul fast float %R112, 0x3F6C5894E0000000
  %R116 = fmul fast float %R113, 0x3F6C5894E0000000
  %R117 = call float @dx.op.unary.f32(i32 29, float %R114)  ; Round_z(value)
  %R118 = call float @dx.op.unary.f32(i32 29, float %R115)  ; Round_z(value)
  %R119 = call float @dx.op.unary.f32(i32 29, float %R116)  ; Round_z(value)
  %R120 = fmul fast float %R117, 2.890000e+02
  %R121 = fmul fast float %R118, 2.890000e+02
  %R122 = fmul fast float %R119, 2.890000e+02
  %R123 = fsub fast float %R111, %R120
  %R124 = fsub fast float %R112, %R121
  %R125 = fsub fast float %R113, %R122
  %R126 = call float @dx.op.dot2.f32(i32 54, float %R58, float %R59, float %R58, float %R59)  ; Dot2(ax,ay,bx,by)
  %R127 = call float @dx.op.dot2.f32(i32 54, float %R67, float %R68, float %R67, float %R68)  ; Dot2(ax,ay,bx,by)
  %R128 = call float @dx.op.dot2.f32(i32 54, float %R65, float %R66, float %R65, float %R66)  ; Dot2(ax,ay,bx,by)
  %R129 = fsub fast float 5.000000e-01, %R126
  %R130 = fsub fast float 5.000000e-01, %R127
  %R131 = fsub fast float 5.000000e-01, %R128
  %R132 = call float @dx.op.binary.f32(i32 35, float %R129, float 0.000000e+00)  ; FMax(a,b)
  %R133 = call float @dx.op.binary.f32(i32 35, float %R130, float 0.000000e+00)  ; FMax(a,b)
  %R134 = call float @dx.op.binary.f32(i32 35, float %R131, float 0.000000e+00)  ; FMax(a,b)
  %R135 = fmul fast float %R132, %R132
  %R136 = fmul fast float %R133, %R133
  %R137 = fmul fast float %R134, %R134
  %R138 = fmul fast float %R135, %R135
  %R139 = fmul fast float %R136, %R136
  %R140 = fmul fast float %R137, %R137
  %R141 = fmul fast float %R123, 0x3F98F9C180000000
  %R142 = fmul fast float %R124, 0x3F98F9C180000000
  %R143 = fmul fast float %R125, 0x3F98F9C180000000
  %R144 = call float @dx.op.unary.f32(i32 22, float %R141)  ; Frc(value)
  %R145 = call float @dx.op.unary.f32(i32 22, float %R142)  ; Frc(value)
  %R146 = call float @dx.op.unary.f32(i32 22, float %R143)  ; Frc(value)
  %R147 = fmul fast float %R144, 2.000000e+00
  %R148 = fmul fast float %R145, 2.000000e+00
  %R149 = fmul fast float %R146, 2.000000e+00
  %R150 = fadd fast float %R147, -1.000000e+00
  %R151 = fadd fast float %R148, -1.000000e+00
  %R152 = fadd fast float %R149, -1.000000e+00
  %R153 = call float @dx.op.unary.f32(i32 6, float %R150)  ; FAbs(value)
  %R154 = call float @dx.op.unary.f32(i32 6, float %R151)  ; FAbs(value)
  %R155 = call float @dx.op.unary.f32(i32 6, float %R152)  ; FAbs(value)
  %R156 = fadd fast float %R153, -5.000000e-01
  %R157 = fadd fast float %R154, -5.000000e-01
  %R158 = fadd fast float %R155, -5.000000e-01
  %R159 = fadd fast float %R147, -5.000000e-01
  %R160 = fadd fast float %R148, -5.000000e-01
  %R161 = fadd fast float %R149, -5.000000e-01
  %R162 = call float @dx.op.unary.f32(i32 27, float %R159)  ; Round_ni(value)
  %R163 = call float @dx.op.unary.f32(i32 27, float %R160)  ; Round_ni(value)
  %R164 = call float @dx.op.unary.f32(i32 27, float %R161)  ; Round_ni(value)
  %R165 = fsub fast float %R150, %R162
  %R166 = fsub fast float %R151, %R163
  %R167 = fsub fast float %R152, %R164
  %R168 = fmul fast float %R165, %R165
  %R169 = fmul fast float %R166, %R166
  %R170 = fmul fast float %R167, %R167
  %R171 = fmul fast float %R156, %R156
  %R172 = fmul fast float %R157, %R157
  %R173 = fmul fast float %R158, %R158
  %R174 = fadd fast float %R168, %R171
  %R175 = fadd fast float %R169, %R172
  %R176 = fadd fast float %R170, %R173
  %R177 = fmul fast float %R174, 0x3FEB51CB80000000
  %R178 = fmul fast float %R175, 0x3FEB51CB80000000
  %R179 = fmul fast float %R176, 0x3FEB51CB80000000
  %R180 = fsub fast float 0x3FFCAF7C00000000, %R177
  %R181 = fsub fast float 0x3FFCAF7C00000000, %R178
  %R182 = fsub fast float 0x3FFCAF7C00000000, %R179
  %R183 = fmul fast float %R138, %R180
  %R184 = fmul fast float %R139, %R181
  %R185 = fmul fast float %R140, %R182
  %R186 = fmul fast float %R165, %R58
  %R187 = fmul fast float %R156, %R59
  %R188 = fadd fast float %R186, %R187
  %R189 = fmul fast float %R166, %R67
  %R190 = fmul fast float %R167, %R65
  %R191 = fmul fast float %R157, %R68
  %R192 = fmul fast float %R158, %R66
  %R193 = fadd fast float %R189, %R191
  %R194 = fadd fast float %R190, %R192
  %R195 = call float @dx.op.dot3.f32(i32 55, float %R183, float %R184, float %R185, float %R188, float %R193, float %R194)  ; Dot3(ax,ay,az,bx,by,bz)
  %R196 = fmul fast float %R37, 1.300000e+02
  %R197 = fmul fast float %R196, %R195
  %R36 = fadd fast float %R197, %R35
  %R198 = fmul fast float %R31, 0x3FEC152800000000
  %R199 = call float @dx.op.tertiary.f32(i32 46, float %R33, float 0xBFDEAEE880000000, float %R198)  ; FMad(a,b,c)
  %R200 = fmul fast float %R31, 0x3FDEAEE880000000
  %R201 = call float @dx.op.tertiary.f32(i32 46, float %R33, float 0x3FEC152800000000, float %R200)  ; FMad(a,b,c)
  %R202 = fmul fast float %R199, 2.000000e+00
  %R203 = fmul fast float %R201, 2.000000e+00
  %R32 = fadd fast float %R202, 1.000000e+02
  %R34 = fadd fast float %R203, 1.000000e+02
  %R38 = fmul fast float %R37, 5.000000e-01
  %R204 = icmp eq i32 %R41, %R47
  %R205 = icmp eq i32 %R44, 0
  %R206 = and i1 %R205, %R204
  br i1 %R206, label %R49, label %R28

; <label>:206                                     ; preds = %R29, %R28
  %R207 = phi float [ %R36, %R29 ], [ %R35, %R28 ]
  %R208 = fsub fast float %R25, %R24
  %R209 = fmul fast float %R207, %R208
  %R210 = fadd fast float %R209, %R24
  %R211 = fadd fast float %R21, 0x3FB99999A0000000
  %R212 = fmul fast float %R211, 0x3F847AE140000000
  br label %R213

; <label>:213                                     ; preds = %R214, %R49
  %R215 = phi float [ %R216, %R214 ], [ %R212, %R49 ]
  %R217 = phi float [ %R218, %R214 ], [ %R27, %R49 ]
  %R219 = phi float [ %R220, %R214 ], [ 0.000000e+00, %R49 ]
  %R221 = phi float [ %R222, %R214 ], [ 5.000000e-01, %R49 ]
  %R223 = phi i32 [ %R224, %R214 ], [ 0, %R49 ]
  %R225 = phi i32 [ %R226, %R214 ], [ -1, %R49 ]
  %R227 = phi i32 [ %R228, %R214 ], [ -1, %R49 ]
  %R229 = phi i32 [ 1, %R214 ], [ 0, %R49 ]
  %R230 = icmp eq i32 %R227, 0
  %R231 = zext i1 %R230 to i32
  %R228 = add i32 %R227, -1
  %R224 = add i32 %R229, %R223
  %R232 = icmp ult i32 %R224, 5
  br i1 %R232, label %R214, label %R233

; <label>:227                                     ; preds = %R213
  %R226 = sub i32 %R225, %R231
  %R234 = call float @dx.op.dot2.f32(i32 54, float %R215, float %R217, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R235 = fadd fast float %R234, %R215
  %R236 = fadd fast float %R234, %R217
  %R237 = call float @dx.op.unary.f32(i32 27, float %R235)  ; Round_ni(value)
  %R238 = call float @dx.op.unary.f32(i32 27, float %R236)  ; Round_ni(value)
  %R239 = fsub fast float %R215, %R237
  %R240 = fsub fast float %R217, %R238
  %R241 = call float @dx.op.dot2.f32(i32 54, float %R237, float %R238, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R242 = fadd fast float %R241, %R239
  %R243 = fadd fast float %R240, %R241
  %R244 = fcmp olt float %R242, %R243
  %R245 = select i1 %R244, float 0.000000e+00, float 1.000000e+00
  %R246 = select i1 %R244, float 1.000000e+00, float 0.000000e+00
  %R247 = fadd fast float %R242, 0x3FCB0CB180000000
  %R248 = fadd fast float %R243, 0x3FCB0CB180000000
  %R249 = fadd fast float %R242, 0xBFE279A740000000
  %R250 = fadd fast float %R243, 0xBFE279A740000000
  %R251 = fsub fast float %R247, %R245
  %R252 = fsub fast float %R248, %R246
  %R253 = fmul fast float %R237, 0x3F6C5894E0000000
  %R254 = fmul fast float %R238, 0x3F6C5894E0000000
  %R255 = call float @dx.op.unary.f32(i32 29, float %R253)  ; Round_z(value)
  %R256 = call float @dx.op.unary.f32(i32 29, float %R254)  ; Round_z(value)
  %R257 = fmul fast float %R255, 2.890000e+02
  %R258 = fmul fast float %R256, 2.890000e+02
  %R259 = fsub fast float %R237, %R257
  %R260 = fsub fast float %R238, %R258
  %R261 = fadd fast float %R260, %R246
  %R262 = fadd fast float %R260, 1.000000e+00
  %R263 = fmul fast float %R260, 3.400000e+01
  %R264 = fmul fast float %R261, 3.400000e+01
  %R265 = fmul fast float %R262, 3.400000e+01
  %R266 = fadd fast float %R263, 1.000000e+00
  %R267 = fadd fast float %R264, 1.000000e+00
  %R268 = fadd fast float %R265, 1.000000e+00
  %R269 = fmul fast float %R266, %R260
  %R270 = fmul fast float %R267, %R261
  %R271 = fmul fast float %R268, %R262
  %R272 = fmul fast float %R269, 0x3F6C5894E0000000
  %R273 = fmul fast float %R270, 0x3F6C5894E0000000
  %R274 = fmul fast float %R271, 0x3F6C5894E0000000
  %R275 = call float @dx.op.unary.f32(i32 29, float %R272)  ; Round_z(value)
  %R276 = call float @dx.op.unary.f32(i32 29, float %R273)  ; Round_z(value)
  %R277 = call float @dx.op.unary.f32(i32 29, float %R274)  ; Round_z(value)
  %R278 = fmul fast float %R275, 2.890000e+02
  %R279 = fmul fast float %R276, 2.890000e+02
  %R280 = fmul fast float %R277, 2.890000e+02
  %R281 = fsub fast float %R269, %R278
  %R282 = fadd fast float %R281, %R259
  %R283 = fadd fast float %R245, %R259
  %R284 = fsub fast float %R283, %R279
  %R285 = fadd fast float %R284, %R270
  %R286 = fadd fast float %R259, 1.000000e+00
  %R287 = fsub fast float %R286, %R280
  %R288 = fadd fast float %R287, %R271
  %R289 = fmul fast float %R282, 3.400000e+01
  %R290 = fmul fast float %R285, 3.400000e+01
  %R291 = fmul fast float %R288, 3.400000e+01
  %R292 = fadd fast float %R289, 1.000000e+00
  %R293 = fadd fast float %R290, 1.000000e+00
  %R294 = fadd fast float %R291, 1.000000e+00
  %R295 = fmul fast float %R292, %R282
  %R296 = fmul fast float %R293, %R285
  %R297 = fmul fast float %R294, %R288
  %R298 = fmul fast float %R295, 0x3F6C5894E0000000
  %R299 = fmul fast float %R296, 0x3F6C5894E0000000
  %R300 = fmul fast float %R297, 0x3F6C5894E0000000
  %R301 = call float @dx.op.unary.f32(i32 29, float %R298)  ; Round_z(value)
  %R302 = call float @dx.op.unary.f32(i32 29, float %R299)  ; Round_z(value)
  %R303 = call float @dx.op.unary.f32(i32 29, float %R300)  ; Round_z(value)
  %R304 = fmul fast float %R301, 2.890000e+02
  %R305 = fmul fast float %R302, 2.890000e+02
  %R306 = fmul fast float %R303, 2.890000e+02
  %R307 = fsub fast float %R295, %R304
  %R308 = fsub fast float %R296, %R305
  %R309 = fsub fast float %R297, %R306
  %R310 = call float @dx.op.dot2.f32(i32 54, float %R242, float %R243, float %R242, float %R243)  ; Dot2(ax,ay,bx,by)
  %R311 = call float @dx.op.dot2.f32(i32 54, float %R251, float %R252, float %R251, float %R252)  ; Dot2(ax,ay,bx,by)
  %R312 = call float @dx.op.dot2.f32(i32 54, float %R249, float %R250, float %R249, float %R250)  ; Dot2(ax,ay,bx,by)
  %R313 = fsub fast float 5.000000e-01, %R310
  %R314 = fsub fast float 5.000000e-01, %R311
  %R315 = fsub fast float 5.000000e-01, %R312
  %R316 = call float @dx.op.binary.f32(i32 35, float %R313, float 0.000000e+00)  ; FMax(a,b)
  %R317 = call float @dx.op.binary.f32(i32 35, float %R314, float 0.000000e+00)  ; FMax(a,b)
  %R318 = call float @dx.op.binary.f32(i32 35, float %R315, float 0.000000e+00)  ; FMax(a,b)
  %R319 = fmul fast float %R316, %R316
  %R320 = fmul fast float %R317, %R317
  %R321 = fmul fast float %R318, %R318
  %R322 = fmul fast float %R319, %R319
  %R323 = fmul fast float %R320, %R320
  %R324 = fmul fast float %R321, %R321
  %R325 = fmul fast float %R307, 0x3F98F9C180000000
  %R326 = fmul fast float %R308, 0x3F98F9C180000000
  %R327 = fmul fast float %R309, 0x3F98F9C180000000
  %R328 = call float @dx.op.unary.f32(i32 22, float %R325)  ; Frc(value)
  %R329 = call float @dx.op.unary.f32(i32 22, float %R326)  ; Frc(value)
  %R330 = call float @dx.op.unary.f32(i32 22, float %R327)  ; Frc(value)
  %R331 = fmul fast float %R328, 2.000000e+00
  %R332 = fmul fast float %R329, 2.000000e+00
  %R333 = fmul fast float %R330, 2.000000e+00
  %R334 = fadd fast float %R331, -1.000000e+00
  %R335 = fadd fast float %R332, -1.000000e+00
  %R336 = fadd fast float %R333, -1.000000e+00
  %R337 = call float @dx.op.unary.f32(i32 6, float %R334)  ; FAbs(value)
  %R338 = call float @dx.op.unary.f32(i32 6, float %R335)  ; FAbs(value)
  %R339 = call float @dx.op.unary.f32(i32 6, float %R336)  ; FAbs(value)
  %R340 = fadd fast float %R337, -5.000000e-01
  %R341 = fadd fast float %R338, -5.000000e-01
  %R342 = fadd fast float %R339, -5.000000e-01
  %R343 = fadd fast float %R331, -5.000000e-01
  %R344 = fadd fast float %R332, -5.000000e-01
  %R345 = fadd fast float %R333, -5.000000e-01
  %R346 = call float @dx.op.unary.f32(i32 27, float %R343)  ; Round_ni(value)
  %R347 = call float @dx.op.unary.f32(i32 27, float %R344)  ; Round_ni(value)
  %R348 = call float @dx.op.unary.f32(i32 27, float %R345)  ; Round_ni(value)
  %R349 = fsub fast float %R334, %R346
  %R350 = fsub fast float %R335, %R347
  %R351 = fsub fast float %R336, %R348
  %R352 = fmul fast float %R349, %R349
  %R353 = fmul fast float %R350, %R350
  %R354 = fmul fast float %R351, %R351
  %R355 = fmul fast float %R340, %R340
  %R356 = fmul fast float %R341, %R341
  %R357 = fmul fast float %R342, %R342
  %R358 = fadd fast float %R352, %R355
  %R359 = fadd fast float %R353, %R356
  %R360 = fadd fast float %R354, %R357
  %R361 = fmul fast float %R358, 0x3FEB51CB80000000
  %R362 = fmul fast float %R359, 0x3FEB51CB80000000
  %R363 = fmul fast float %R360, 0x3FEB51CB80000000
  %R364 = fsub fast float 0x3FFCAF7C00000000, %R361
  %R365 = fsub fast float 0x3FFCAF7C00000000, %R362
  %R366 = fsub fast float 0x3FFCAF7C00000000, %R363
  %R367 = fmul fast float %R322, %R364
  %R368 = fmul fast float %R323, %R365
  %R369 = fmul fast float %R324, %R366
  %R370 = fmul fast float %R349, %R242
  %R371 = fmul fast float %R340, %R243
  %R372 = fadd fast float %R370, %R371
  %R373 = fmul fast float %R350, %R251
  %R374 = fmul fast float %R351, %R249
  %R375 = fmul fast float %R341, %R252
  %R376 = fmul fast float %R342, %R250
  %R377 = fadd fast float %R373, %R375
  %R378 = fadd fast float %R374, %R376
  %R379 = call float @dx.op.dot3.f32(i32 55, float %R367, float %R368, float %R369, float %R372, float %R377, float %R378)  ; Dot3(ax,ay,az,bx,by,bz)
  %R380 = fmul fast float %R221, 1.300000e+02
  %R381 = fmul fast float %R380, %R379
  %R220 = fadd fast float %R381, %R219
  %R382 = fmul fast float %R215, 0x3FEC152800000000
  %R383 = call float @dx.op.tertiary.f32(i32 46, float %R217, float 0xBFDEAEE880000000, float %R382)  ; FMad(a,b,c)
  %R384 = fmul fast float %R215, 0x3FDEAEE880000000
  %R385 = call float @dx.op.tertiary.f32(i32 46, float %R217, float 0x3FEC152800000000, float %R384)  ; FMad(a,b,c)
  %R386 = fmul fast float %R383, 2.000000e+00
  %R387 = fmul fast float %R385, 2.000000e+00
  %R216 = fadd fast float %R386, 1.000000e+02
  %R218 = fadd fast float %R387, 1.000000e+02
  %R222 = fmul fast float %R221, 5.000000e-01
  %R388 = icmp eq i32 %R225, %R231
  %R389 = icmp eq i32 %R228, 0
  %R390 = and i1 %R389, %R388
  br i1 %R390, label %R233, label %R213

; <label>:390                                     ; preds = %R214, %R213
  %R391 = phi float [ %R220, %R214 ], [ %R219, %R213 ]
  %R392 = fsub fast float %R391, %R207
  %R393 = fmul fast float %R208, %R392
  %R394 = fadd fast float %R22, 0x3FB99999A0000000
  %R395 = fmul fast float %R394, 0x3F847AE140000000
  br label %R396

; <label>:396                                     ; preds = %R397, %R233
  %R398 = phi float [ %R399, %R397 ], [ %R26, %R233 ]
  %R400 = phi float [ %R401, %R397 ], [ %R395, %R233 ]
  %R402 = phi float [ %R403, %R397 ], [ 0.000000e+00, %R233 ]
  %R404 = phi float [ %R405, %R397 ], [ 5.000000e-01, %R233 ]
  %R406 = phi i32 [ %R407, %R397 ], [ 0, %R233 ]
  %R408 = phi i32 [ %R409, %R397 ], [ -1, %R233 ]
  %R410 = phi i32 [ %R411, %R397 ], [ -1, %R233 ]
  %R412 = phi i32 [ 1, %R397 ], [ 0, %R233 ]
  %R413 = icmp eq i32 %R410, 0
  %R414 = zext i1 %R413 to i32
  %R411 = add i32 %R410, -1
  %R407 = add i32 %R412, %R406
  %R415 = icmp ult i32 %R407, 5
  br i1 %R415, label %R397, label %R416

; <label>:410                                     ; preds = %R396
  %R409 = sub i32 %R408, %R414
  %R417 = call float @dx.op.dot2.f32(i32 54, float %R398, float %R400, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R418 = fadd fast float %R417, %R398
  %R419 = fadd fast float %R417, %R400
  %R420 = call float @dx.op.unary.f32(i32 27, float %R418)  ; Round_ni(value)
  %R421 = call float @dx.op.unary.f32(i32 27, float %R419)  ; Round_ni(value)
  %R422 = fsub fast float %R398, %R420
  %R423 = fsub fast float %R400, %R421
  %R424 = call float @dx.op.dot2.f32(i32 54, float %R420, float %R421, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R425 = fadd fast float %R424, %R422
  %R426 = fadd fast float %R423, %R424
  %R427 = fcmp olt float %R425, %R426
  %R428 = select i1 %R427, float 0.000000e+00, float 1.000000e+00
  %R429 = select i1 %R427, float 1.000000e+00, float 0.000000e+00
  %R430 = fadd fast float %R425, 0x3FCB0CB180000000
  %R431 = fadd fast float %R426, 0x3FCB0CB180000000
  %R432 = fadd fast float %R425, 0xBFE279A740000000
  %R433 = fadd fast float %R426, 0xBFE279A740000000
  %R434 = fsub fast float %R430, %R428
  %R435 = fsub fast float %R431, %R429
  %R436 = fmul fast float %R420, 0x3F6C5894E0000000
  %R437 = fmul fast float %R421, 0x3F6C5894E0000000
  %R438 = call float @dx.op.unary.f32(i32 29, float %R436)  ; Round_z(value)
  %R439 = call float @dx.op.unary.f32(i32 29, float %R437)  ; Round_z(value)
  %R440 = fmul fast float %R438, 2.890000e+02
  %R441 = fmul fast float %R439, 2.890000e+02
  %R442 = fsub fast float %R420, %R440
  %R443 = fsub fast float %R421, %R441
  %R444 = fadd fast float %R443, %R429
  %R445 = fadd fast float %R443, 1.000000e+00
  %R446 = fmul fast float %R443, 3.400000e+01
  %R447 = fmul fast float %R444, 3.400000e+01
  %R448 = fmul fast float %R445, 3.400000e+01
  %R449 = fadd fast float %R446, 1.000000e+00
  %R450 = fadd fast float %R447, 1.000000e+00
  %R451 = fadd fast float %R448, 1.000000e+00
  %R452 = fmul fast float %R449, %R443
  %R453 = fmul fast float %R450, %R444
  %R454 = fmul fast float %R451, %R445
  %R455 = fmul fast float %R452, 0x3F6C5894E0000000
  %R456 = fmul fast float %R453, 0x3F6C5894E0000000
  %R457 = fmul fast float %R454, 0x3F6C5894E0000000
  %R458 = call float @dx.op.unary.f32(i32 29, float %R455)  ; Round_z(value)
  %R459 = call float @dx.op.unary.f32(i32 29, float %R456)  ; Round_z(value)
  %R460 = call float @dx.op.unary.f32(i32 29, float %R457)  ; Round_z(value)
  %R461 = fmul fast float %R458, 2.890000e+02
  %R462 = fmul fast float %R459, 2.890000e+02
  %R463 = fmul fast float %R460, 2.890000e+02
  %R464 = fsub fast float %R452, %R461
  %R465 = fadd fast float %R464, %R442
  %R466 = fadd fast float %R428, %R442
  %R467 = fsub fast float %R466, %R462
  %R468 = fadd fast float %R467, %R453
  %R469 = fadd fast float %R442, 1.000000e+00
  %R470 = fsub fast float %R469, %R463
  %R471 = fadd fast float %R470, %R454
  %R472 = fmul fast float %R465, 3.400000e+01
  %R473 = fmul fast float %R468, 3.400000e+01
  %R474 = fmul fast float %R471, 3.400000e+01
  %R475 = fadd fast float %R472, 1.000000e+00
  %R476 = fadd fast float %R473, 1.000000e+00
  %R477 = fadd fast float %R474, 1.000000e+00
  %R478 = fmul fast float %R475, %R465
  %R479 = fmul fast float %R476, %R468
  %R480 = fmul fast float %R477, %R471
  %R481 = fmul fast float %R478, 0x3F6C5894E0000000
  %R482 = fmul fast float %R479, 0x3F6C5894E0000000
  %R483 = fmul fast float %R480, 0x3F6C5894E0000000
  %R484 = call float @dx.op.unary.f32(i32 29, float %R481)  ; Round_z(value)
  %R485 = call float @dx.op.unary.f32(i32 29, float %R482)  ; Round_z(value)
  %R486 = call float @dx.op.unary.f32(i32 29, float %R483)  ; Round_z(value)
  %R487 = fmul fast float %R484, 2.890000e+02
  %R488 = fmul fast float %R485, 2.890000e+02
  %R489 = fmul fast float %R486, 2.890000e+02
  %R490 = fsub fast float %R478, %R487
  %R491 = fsub fast float %R479, %R488
  %R492 = fsub fast float %R480, %R489
  %R493 = call float @dx.op.dot2.f32(i32 54, float %R425, float %R426, float %R425, float %R426)  ; Dot2(ax,ay,bx,by)
  %R494 = call float @dx.op.dot2.f32(i32 54, float %R434, float %R435, float %R434, float %R435)  ; Dot2(ax,ay,bx,by)
  %R495 = call float @dx.op.dot2.f32(i32 54, float %R432, float %R433, float %R432, float %R433)  ; Dot2(ax,ay,bx,by)
  %R496 = fsub fast float 5.000000e-01, %R493
  %R497 = fsub fast float 5.000000e-01, %R494
  %R498 = fsub fast float 5.000000e-01, %R495
  %R499 = call float @dx.op.binary.f32(i32 35, float %R496, float 0.000000e+00)  ; FMax(a,b)
  %R500 = call float @dx.op.binary.f32(i32 35, float %R497, float 0.000000e+00)  ; FMax(a,b)
  %R501 = call float @dx.op.binary.f32(i32 35, float %R498, float 0.000000e+00)  ; FMax(a,b)
  %R502 = fmul fast float %R499, %R499
  %R503 = fmul fast float %R500, %R500
  %R504 = fmul fast float %R501, %R501
  %R505 = fmul fast float %R502, %R502
  %R506 = fmul fast float %R503, %R503
  %R507 = fmul fast float %R504, %R504
  %R508 = fmul fast float %R490, 0x3F98F9C180000000
  %R509 = fmul fast float %R491, 0x3F98F9C180000000
  %R510 = fmul fast float %R492, 0x3F98F9C180000000
  %R511 = call float @dx.op.unary.f32(i32 22, float %R508)  ; Frc(value)
  %R512 = call float @dx.op.unary.f32(i32 22, float %R509)  ; Frc(value)
  %R513 = call float @dx.op.unary.f32(i32 22, float %R510)  ; Frc(value)
  %R514 = fmul fast float %R511, 2.000000e+00
  %R515 = fmul fast float %R512, 2.000000e+00
  %R516 = fmul fast float %R513, 2.000000e+00
  %R517 = fadd fast float %R514, -1.000000e+00
  %R518 = fadd fast float %R515, -1.000000e+00
  %R519 = fadd fast float %R516, -1.000000e+00
  %R520 = call float @dx.op.unary.f32(i32 6, float %R517)  ; FAbs(value)
  %R521 = call float @dx.op.unary.f32(i32 6, float %R518)  ; FAbs(value)
  %R522 = call float @dx.op.unary.f32(i32 6, float %R519)  ; FAbs(value)
  %R523 = fadd fast float %R520, -5.000000e-01
  %R524 = fadd fast float %R521, -5.000000e-01
  %R525 = fadd fast float %R522, -5.000000e-01
  %R526 = fadd fast float %R514, -5.000000e-01
  %R527 = fadd fast float %R515, -5.000000e-01
  %R528 = fadd fast float %R516, -5.000000e-01
  %R529 = call float @dx.op.unary.f32(i32 27, float %R526)  ; Round_ni(value)
  %R530 = call float @dx.op.unary.f32(i32 27, float %R527)  ; Round_ni(value)
  %R531 = call float @dx.op.unary.f32(i32 27, float %R528)  ; Round_ni(value)
  %R532 = fsub fast float %R517, %R529
  %R533 = fsub fast float %R518, %R530
  %R534 = fsub fast float %R519, %R531
  %R535 = fmul fast float %R532, %R532
  %R536 = fmul fast float %R533, %R533
  %R537 = fmul fast float %R534, %R534
  %R538 = fmul fast float %R523, %R523
  %R539 = fmul fast float %R524, %R524
  %R540 = fmul fast float %R525, %R525
  %R541 = fadd fast float %R535, %R538
  %R542 = fadd fast float %R536, %R539
  %R543 = fadd fast float %R537, %R540
  %R544 = fmul fast float %R541, 0x3FEB51CB80000000
  %R545 = fmul fast float %R542, 0x3FEB51CB80000000
  %R546 = fmul fast float %R543, 0x3FEB51CB80000000
  %R547 = fsub fast float 0x3FFCAF7C00000000, %R544
  %R548 = fsub fast float 0x3FFCAF7C00000000, %R545
  %R549 = fsub fast float 0x3FFCAF7C00000000, %R546
  %R550 = fmul fast float %R505, %R547
  %R551 = fmul fast float %R506, %R548
  %R552 = fmul fast float %R507, %R549
  %R553 = fmul fast float %R532, %R425
  %R554 = fmul fast float %R523, %R426
  %R555 = fadd fast float %R553, %R554
  %R556 = fmul fast float %R533, %R434
  %R557 = fmul fast float %R534, %R432
  %R558 = fmul fast float %R524, %R435
  %R559 = fmul fast float %R525, %R433
  %R560 = fadd fast float %R556, %R558
  %R561 = fadd fast float %R557, %R559
  %R562 = call float @dx.op.dot3.f32(i32 55, float %R550, float %R551, float %R552, float %R555, float %R560, float %R561)  ; Dot3(ax,ay,az,bx,by,bz)
  %R563 = fmul fast float %R404, 1.300000e+02
  %R564 = fmul fast float %R563, %R562
  %R403 = fadd fast float %R564, %R402
  %R565 = fmul fast float %R398, 0x3FEC152800000000
  %R566 = call float @dx.op.tertiary.f32(i32 46, float %R400, float 0xBFDEAEE880000000, float %R565)  ; FMad(a,b,c)
  %R567 = fmul fast float %R398, 0x3FDEAEE880000000
  %R568 = call float @dx.op.tertiary.f32(i32 46, float %R400, float 0x3FEC152800000000, float %R567)  ; FMad(a,b,c)
  %R569 = fmul fast float %R566, 2.000000e+00
  %R570 = fmul fast float %R568, 2.000000e+00
  %R399 = fadd fast float %R569, 1.000000e+02
  %R401 = fadd fast float %R570, 1.000000e+02
  %R405 = fmul fast float %R404, 5.000000e-01
  %R571 = icmp eq i32 %R408, %R414
  %R572 = icmp eq i32 %R411, 0
  %R573 = and i1 %R572, %R571
  br i1 %R573, label %R416, label %R396

; <label>:573                                     ; preds = %R397, %R396
  %R574 = phi float [ %R403, %R397 ], [ %R402, %R396 ]
  %R575 = fsub fast float %R574, %R207
  %R576 = fmul fast float %R208, %R575
  %R577 = fadd fast float %R21, 0xBFB99999A0000000
  %R578 = fmul fast float %R577, 0x3F847AE140000000
  br label %R579

; <label>:579                                     ; preds = %R580, %R416
  %R581 = phi float [ %R582, %R580 ], [ %R578, %R416 ]
  %R583 = phi float [ %R584, %R580 ], [ %R27, %R416 ]
  %R585 = phi float [ %R586, %R580 ], [ 0.000000e+00, %R416 ]
  %R587 = phi float [ %R588, %R580 ], [ 5.000000e-01, %R416 ]
  %R589 = phi i32 [ %R590, %R580 ], [ 0, %R416 ]
  %R591 = phi i32 [ %R592, %R580 ], [ -1, %R416 ]
  %R593 = phi i32 [ %R594, %R580 ], [ -1, %R416 ]
  %R595 = phi i32 [ 1, %R580 ], [ 0, %R416 ]
  %R596 = icmp eq i32 %R593, 0
  %R597 = zext i1 %R596 to i32
  %R594 = add i32 %R593, -1
  %R590 = add i32 %R595, %R589
  %R598 = icmp ult i32 %R590, 5
  br i1 %R598, label %R580, label %R599

; <label>:593                                     ; preds = %R579
  %R592 = sub i32 %R591, %R597
  %R600 = call float @dx.op.dot2.f32(i32 54, float %R581, float %R583, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R601 = fadd fast float %R600, %R581
  %R602 = fadd fast float %R600, %R583
  %R603 = call float @dx.op.unary.f32(i32 27, float %R601)  ; Round_ni(value)
  %R604 = call float @dx.op.unary.f32(i32 27, float %R602)  ; Round_ni(value)
  %R605 = fsub fast float %R581, %R603
  %R606 = fsub fast float %R583, %R604
  %R607 = call float @dx.op.dot2.f32(i32 54, float %R603, float %R604, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R608 = fadd fast float %R607, %R605
  %R609 = fadd fast float %R606, %R607
  %R610 = fcmp olt float %R608, %R609
  %R611 = select i1 %R610, float 0.000000e+00, float 1.000000e+00
  %R612 = select i1 %R610, float 1.000000e+00, float 0.000000e+00
  %R613 = fadd fast float %R608, 0x3FCB0CB180000000
  %R614 = fadd fast float %R609, 0x3FCB0CB180000000
  %R615 = fadd fast float %R608, 0xBFE279A740000000
  %R616 = fadd fast float %R609, 0xBFE279A740000000
  %R617 = fsub fast float %R613, %R611
  %R618 = fsub fast float %R614, %R612
  %R619 = fmul fast float %R603, 0x3F6C5894E0000000
  %R620 = fmul fast float %R604, 0x3F6C5894E0000000
  %R621 = call float @dx.op.unary.f32(i32 29, float %R619)  ; Round_z(value)
  %R622 = call float @dx.op.unary.f32(i32 29, float %R620)  ; Round_z(value)
  %R623 = fmul fast float %R621, 2.890000e+02
  %R624 = fmul fast float %R622, 2.890000e+02
  %R625 = fsub fast float %R603, %R623
  %R626 = fsub fast float %R604, %R624
  %R627 = fadd fast float %R626, %R612
  %R628 = fadd fast float %R626, 1.000000e+00
  %R629 = fmul fast float %R626, 3.400000e+01
  %R630 = fmul fast float %R627, 3.400000e+01
  %R631 = fmul fast float %R628, 3.400000e+01
  %R632 = fadd fast float %R629, 1.000000e+00
  %R633 = fadd fast float %R630, 1.000000e+00
  %R634 = fadd fast float %R631, 1.000000e+00
  %R635 = fmul fast float %R632, %R626
  %R636 = fmul fast float %R633, %R627
  %R637 = fmul fast float %R634, %R628
  %R638 = fmul fast float %R635, 0x3F6C5894E0000000
  %R639 = fmul fast float %R636, 0x3F6C5894E0000000
  %R640 = fmul fast float %R637, 0x3F6C5894E0000000
  %R641 = call float @dx.op.unary.f32(i32 29, float %R638)  ; Round_z(value)
  %R642 = call float @dx.op.unary.f32(i32 29, float %R639)  ; Round_z(value)
  %R643 = call float @dx.op.unary.f32(i32 29, float %R640)  ; Round_z(value)
  %R644 = fmul fast float %R641, 2.890000e+02
  %R645 = fmul fast float %R642, 2.890000e+02
  %R646 = fmul fast float %R643, 2.890000e+02
  %R647 = fsub fast float %R635, %R644
  %R648 = fadd fast float %R647, %R625
  %R649 = fadd fast float %R611, %R625
  %R650 = fsub fast float %R649, %R645
  %R651 = fadd fast float %R650, %R636
  %R652 = fadd fast float %R625, 1.000000e+00
  %R653 = fsub fast float %R652, %R646
  %R654 = fadd fast float %R653, %R637
  %R655 = fmul fast float %R648, 3.400000e+01
  %R656 = fmul fast float %R651, 3.400000e+01
  %R657 = fmul fast float %R654, 3.400000e+01
  %R658 = fadd fast float %R655, 1.000000e+00
  %R659 = fadd fast float %R656, 1.000000e+00
  %R660 = fadd fast float %R657, 1.000000e+00
  %R661 = fmul fast float %R658, %R648
  %R662 = fmul fast float %R659, %R651
  %R663 = fmul fast float %R660, %R654
  %R664 = fmul fast float %R661, 0x3F6C5894E0000000
  %R665 = fmul fast float %R662, 0x3F6C5894E0000000
  %R666 = fmul fast float %R663, 0x3F6C5894E0000000
  %R667 = call float @dx.op.unary.f32(i32 29, float %R664)  ; Round_z(value)
  %R668 = call float @dx.op.unary.f32(i32 29, float %R665)  ; Round_z(value)
  %R669 = call float @dx.op.unary.f32(i32 29, float %R666)  ; Round_z(value)
  %R670 = fmul fast float %R667, 2.890000e+02
  %R671 = fmul fast float %R668, 2.890000e+02
  %R672 = fmul fast float %R669, 2.890000e+02
  %R673 = fsub fast float %R661, %R670
  %R674 = fsub fast float %R662, %R671
  %R675 = fsub fast float %R663, %R672
  %R676 = call float @dx.op.dot2.f32(i32 54, float %R608, float %R609, float %R608, float %R609)  ; Dot2(ax,ay,bx,by)
  %R677 = call float @dx.op.dot2.f32(i32 54, float %R617, float %R618, float %R617, float %R618)  ; Dot2(ax,ay,bx,by)
  %R678 = call float @dx.op.dot2.f32(i32 54, float %R615, float %R616, float %R615, float %R616)  ; Dot2(ax,ay,bx,by)
  %R679 = fsub fast float 5.000000e-01, %R676
  %R680 = fsub fast float 5.000000e-01, %R677
  %R681 = fsub fast float 5.000000e-01, %R678
  %R682 = call float @dx.op.binary.f32(i32 35, float %R679, float 0.000000e+00)  ; FMax(a,b)
  %R683 = call float @dx.op.binary.f32(i32 35, float %R680, float 0.000000e+00)  ; FMax(a,b)
  %R684 = call float @dx.op.binary.f32(i32 35, float %R681, float 0.000000e+00)  ; FMax(a,b)
  %R685 = fmul fast float %R682, %R682
  %R686 = fmul fast float %R683, %R683
  %R687 = fmul fast float %R684, %R684
  %R688 = fmul fast float %R685, %R685
  %R689 = fmul fast float %R686, %R686
  %R690 = fmul fast float %R687, %R687
  %R691 = fmul fast float %R673, 0x3F98F9C180000000
  %R692 = fmul fast float %R674, 0x3F98F9C180000000
  %R693 = fmul fast float %R675, 0x3F98F9C180000000
  %R694 = call float @dx.op.unary.f32(i32 22, float %R691)  ; Frc(value)
  %R695 = call float @dx.op.unary.f32(i32 22, float %R692)  ; Frc(value)
  %R696 = call float @dx.op.unary.f32(i32 22, float %R693)  ; Frc(value)
  %R697 = fmul fast float %R694, 2.000000e+00
  %R698 = fmul fast float %R695, 2.000000e+00
  %R699 = fmul fast float %R696, 2.000000e+00
  %R700 = fadd fast float %R697, -1.000000e+00
  %R701 = fadd fast float %R698, -1.000000e+00
  %R702 = fadd fast float %R699, -1.000000e+00
  %R703 = call float @dx.op.unary.f32(i32 6, float %R700)  ; FAbs(value)
  %R704 = call float @dx.op.unary.f32(i32 6, float %R701)  ; FAbs(value)
  %R705 = call float @dx.op.unary.f32(i32 6, float %R702)  ; FAbs(value)
  %R706 = fadd fast float %R703, -5.000000e-01
  %R707 = fadd fast float %R704, -5.000000e-01
  %R708 = fadd fast float %R705, -5.000000e-01
  %R709 = fadd fast float %R697, -5.000000e-01
  %R710 = fadd fast float %R698, -5.000000e-01
  %R711 = fadd fast float %R699, -5.000000e-01
  %R712 = call float @dx.op.unary.f32(i32 27, float %R709)  ; Round_ni(value)
  %R713 = call float @dx.op.unary.f32(i32 27, float %R710)  ; Round_ni(value)
  %R714 = call float @dx.op.unary.f32(i32 27, float %R711)  ; Round_ni(value)
  %R715 = fsub fast float %R700, %R712
  %R716 = fsub fast float %R701, %R713
  %R717 = fsub fast float %R702, %R714
  %R718 = fmul fast float %R715, %R715
  %R719 = fmul fast float %R716, %R716
  %R720 = fmul fast float %R717, %R717
  %R721 = fmul fast float %R706, %R706
  %R722 = fmul fast float %R707, %R707
  %R723 = fmul fast float %R708, %R708
  %R724 = fadd fast float %R718, %R721
  %R725 = fadd fast float %R719, %R722
  %R726 = fadd fast float %R720, %R723
  %R727 = fmul fast float %R724, 0x3FEB51CB80000000
  %R728 = fmul fast float %R725, 0x3FEB51CB80000000
  %R729 = fmul fast float %R726, 0x3FEB51CB80000000
  %R730 = fsub fast float 0x3FFCAF7C00000000, %R727
  %R731 = fsub fast float 0x3FFCAF7C00000000, %R728
  %R732 = fsub fast float 0x3FFCAF7C00000000, %R729
  %R733 = fmul fast float %R688, %R730
  %R734 = fmul fast float %R689, %R731
  %R735 = fmul fast float %R690, %R732
  %R736 = fmul fast float %R715, %R608
  %R737 = fmul fast float %R706, %R609
  %R738 = fadd fast float %R736, %R737
  %R739 = fmul fast float %R716, %R617
  %R740 = fmul fast float %R717, %R615
  %R741 = fmul fast float %R707, %R618
  %R742 = fmul fast float %R708, %R616
  %R743 = fadd fast float %R739, %R741
  %R744 = fadd fast float %R740, %R742
  %R745 = call float @dx.op.dot3.f32(i32 55, float %R733, float %R734, float %R735, float %R738, float %R743, float %R744)  ; Dot3(ax,ay,az,bx,by,bz)
  %R746 = fmul fast float %R587, 1.300000e+02
  %R747 = fmul fast float %R746, %R745
  %R586 = fadd fast float %R747, %R585
  %R748 = fmul fast float %R581, 0x3FEC152800000000
  %R749 = call float @dx.op.tertiary.f32(i32 46, float %R583, float 0xBFDEAEE880000000, float %R748)  ; FMad(a,b,c)
  %R750 = fmul fast float %R581, 0x3FDEAEE880000000
  %R751 = call float @dx.op.tertiary.f32(i32 46, float %R583, float 0x3FEC152800000000, float %R750)  ; FMad(a,b,c)
  %R752 = fmul fast float %R749, 2.000000e+00
  %R753 = fmul fast float %R751, 2.000000e+00
  %R582 = fadd fast float %R752, 1.000000e+02
  %R584 = fadd fast float %R753, 1.000000e+02
  %R588 = fmul fast float %R587, 5.000000e-01
  %R754 = icmp eq i32 %R591, %R597
  %R755 = icmp eq i32 %R594, 0
  %R756 = and i1 %R755, %R754
  br i1 %R756, label %R599, label %R579

; <label>:756                                     ; preds = %R580, %R579
  %R757 = phi float [ %R586, %R580 ], [ %R585, %R579 ]
  %R758 = fsub fast float %R757, %R207
  %R759 = fmul fast float %R208, %R758
  %R760 = fadd fast float %R22, 0xBFB99999A0000000
  %R761 = fmul fast float %R760, 0x3F847AE140000000
  br label %R762

; <label>:762                                     ; preds = %R763, %R599
  %R764 = phi float [ %R765, %R763 ], [ %R26, %R599 ]
  %R766 = phi float [ %R767, %R763 ], [ %R761, %R599 ]
  %R768 = phi float [ %R769, %R763 ], [ 0.000000e+00, %R599 ]
  %R770 = phi float [ %R771, %R763 ], [ 5.000000e-01, %R599 ]
  %R772 = phi i32 [ %R773, %R763 ], [ 0, %R599 ]
  %R774 = phi i32 [ %R775, %R763 ], [ -1, %R599 ]
  %R776 = phi i32 [ %R777, %R763 ], [ -1, %R599 ]
  %R778 = phi i32 [ 1, %R763 ], [ 0, %R599 ]
  %R779 = icmp eq i32 %R776, 0
  %R780 = zext i1 %R779 to i32
  %R777 = add i32 %R776, -1
  %R773 = add i32 %R778, %R772
  %R781 = icmp ult i32 %R773, 5
  br i1 %R781, label %R763, label %R782

; <label>:776                                     ; preds = %R762
  %R775 = sub i32 %R774, %R780
  %R783 = call float @dx.op.dot2.f32(i32 54, float %R764, float %R766, float 0x3FD76CF5E0000000, float 0x3FD76CF5E0000000)  ; Dot2(ax,ay,bx,by)
  %R784 = fadd fast float %R783, %R764
  %R785 = fadd fast float %R783, %R766
  %R786 = call float @dx.op.unary.f32(i32 27, float %R784)  ; Round_ni(value)
  %R787 = call float @dx.op.unary.f32(i32 27, float %R785)  ; Round_ni(value)
  %R788 = fsub fast float %R764, %R786
  %R789 = fsub fast float %R766, %R787
  %R790 = call float @dx.op.dot2.f32(i32 54, float %R786, float %R787, float 0x3FCB0CB180000000, float 0x3FCB0CB180000000)  ; Dot2(ax,ay,bx,by)
  %R791 = fadd fast float %R790, %R788
  %R792 = fadd fast float %R789, %R790
  %R793 = fcmp olt float %R791, %R792
  %R794 = select i1 %R793, float 0.000000e+00, float 1.000000e+00
  %R795 = select i1 %R793, float 1.000000e+00, float 0.000000e+00
  %R796 = fadd fast float %R791, 0x3FCB0CB180000000
  %R797 = fadd fast float %R792, 0x3FCB0CB180000000
  %R798 = fadd fast float %R791, 0xBFE279A740000000
  %R799 = fadd fast float %R792, 0xBFE279A740000000
  %R800 = fsub fast float %R796, %R794
  %R801 = fsub fast float %R797, %R795
  %R802 = fmul fast float %R786, 0x3F6C5894E0000000
  %R803 = fmul fast float %R787, 0x3F6C5894E0000000
  %R804 = call float @dx.op.unary.f32(i32 29, float %R802)  ; Round_z(value)
  %R805 = call float @dx.op.unary.f32(i32 29, float %R803)  ; Round_z(value)
  %R806 = fmul fast float %R804, 2.890000e+02
  %R807 = fmul fast float %R805, 2.890000e+02
  %R808 = fsub fast float %R786, %R806
  %R809 = fsub fast float %R787, %R807
  %R810 = fadd fast float %R809, %R795
  %R811 = fadd fast float %R809, 1.000000e+00
  %R812 = fmul fast float %R809, 3.400000e+01
  %R813 = fmul fast float %R810, 3.400000e+01
  %R814 = fmul fast float %R811, 3.400000e+01
  %R815 = fadd fast float %R812, 1.000000e+00
  %R816 = fadd fast float %R813, 1.000000e+00
  %R817 = fadd fast float %R814, 1.000000e+00
  %R818 = fmul fast float %R815, %R809
  %R819 = fmul fast float %R816, %R810
  %R820 = fmul fast float %R817, %R811
  %R821 = fmul fast float %R818, 0x3F6C5894E0000000
  %R822 = fmul fast float %R819, 0x3F6C5894E0000000
  %R823 = fmul fast float %R820, 0x3F6C5894E0000000
  %R824 = call float @dx.op.unary.f32(i32 29, float %R821)  ; Round_z(value)
  %R825 = call float @dx.op.unary.f32(i32 29, float %R822)  ; Round_z(value)
  %R826 = call float @dx.op.unary.f32(i32 29, float %R823)  ; Round_z(value)
  %R827 = fmul fast float %R824, 2.890000e+02
  %R828 = fmul fast float %R825, 2.890000e+02
  %R829 = fmul fast float %R826, 2.890000e+02
  %R830 = fsub fast float %R818, %R827
  %R831 = fadd fast float %R830, %R808
  %R832 = fadd fast float %R794, %R808
  %R833 = fsub fast float %R832, %R828
  %R834 = fadd fast float %R833, %R819
  %R835 = fadd fast float %R808, 1.000000e+00
  %R836 = fsub fast float %R835, %R829
  %R837 = fadd fast float %R836, %R820
  %R838 = fmul fast float %R831, 3.400000e+01
  %R839 = fmul fast float %R834, 3.400000e+01
  %R840 = fmul fast float %R837, 3.400000e+01
  %R841 = fadd fast float %R838, 1.000000e+00
  %R842 = fadd fast float %R839, 1.000000e+00
  %R843 = fadd fast float %R840, 1.000000e+00
  %R844 = fmul fast float %R841, %R831
  %R845 = fmul fast float %R842, %R834
  %R846 = fmul fast float %R843, %R837
  %R847 = fmul fast float %R844, 0x3F6C5894E0000000
  %R848 = fmul fast float %R845, 0x3F6C5894E0000000
  %R849 = fmul fast float %R846, 0x3F6C5894E0000000
  %R850 = call float @dx.op.unary.f32(i32 29, float %R847)  ; Round_z(value)
  %R851 = call float @dx.op.unary.f32(i32 29, float %R848)  ; Round_z(value)
  %R852 = call float @dx.op.unary.f32(i32 29, float %R849)  ; Round_z(value)
  %R853 = fmul fast float %R850, 2.890000e+02
  %R854 = fmul fast float %R851, 2.890000e+02
  %R855 = fmul fast float %R852, 2.890000e+02
  %R856 = fsub fast float %R844, %R853
  %R857 = fsub fast float %R845, %R854
  %R858 = fsub fast float %R846, %R855
  %R859 = call float @dx.op.dot2.f32(i32 54, float %R791, float %R792, float %R791, float %R792)  ; Dot2(ax,ay,bx,by)
  %R860 = call float @dx.op.dot2.f32(i32 54, float %R800, float %R801, float %R800, float %R801)  ; Dot2(ax,ay,bx,by)
  %R861 = call float @dx.op.dot2.f32(i32 54, float %R798, float %R799, float %R798, float %R799)  ; Dot2(ax,ay,bx,by)
  %R862 = fsub fast float 5.000000e-01, %R859
  %R863 = fsub fast float 5.000000e-01, %R860
  %R864 = fsub fast float 5.000000e-01, %R861
  %R865 = call float @dx.op.binary.f32(i32 35, float %R862, float 0.000000e+00)  ; FMax(a,b)
  %R866 = call float @dx.op.binary.f32(i32 35, float %R863, float 0.000000e+00)  ; FMax(a,b)
  %R867 = call float @dx.op.binary.f32(i32 35, float %R864, float 0.000000e+00)  ; FMax(a,b)
  %R868 = fmul fast float %R865, %R865
  %R869 = fmul fast float %R866, %R866
  %R870 = fmul fast float %R867, %R867
  %R871 = fmul fast float %R868, %R868
  %R872 = fmul fast float %R869, %R869
  %R873 = fmul fast float %R870, %R870
  %R874 = fmul fast float %R856, 0x3F98F9C180000000
  %R875 = fmul fast float %R857, 0x3F98F9C180000000
  %R876 = fmul fast float %R858, 0x3F98F9C180000000
  %R877 = call float @dx.op.unary.f32(i32 22, float %R874)  ; Frc(value)
  %R878 = call float @dx.op.unary.f32(i32 22, float %R875)  ; Frc(value)
  %R879 = call float @dx.op.unary.f32(i32 22, float %R876)  ; Frc(value)
  %R880 = fmul fast float %R877, 2.000000e+00
  %R881 = fmul fast float %R878, 2.000000e+00
  %R882 = fmul fast float %R879, 2.000000e+00
  %R883 = fadd fast float %R880, -1.000000e+00
  %R884 = fadd fast float %R881, -1.000000e+00
  %R885 = fadd fast float %R882, -1.000000e+00
  %R886 = call float @dx.op.unary.f32(i32 6, float %R883)  ; FAbs(value)
  %R887 = call float @dx.op.unary.f32(i32 6, float %R884)  ; FAbs(value)
  %R888 = call float @dx.op.unary.f32(i32 6, float %R885)  ; FAbs(value)
  %R889 = fadd fast float %R886, -5.000000e-01
  %R890 = fadd fast float %R887, -5.000000e-01
  %R891 = fadd fast float %R888, -5.000000e-01
  %R892 = fadd fast float %R880, -5.000000e-01
  %R893 = fadd fast float %R881, -5.000000e-01
  %R894 = fadd fast float %R882, -5.000000e-01
  %R895 = call float @dx.op.unary.f32(i32 27, float %R892)  ; Round_ni(value)
  %R896 = call float @dx.op.unary.f32(i32 27, float %R893)  ; Round_ni(value)
  %R897 = call float @dx.op.unary.f32(i32 27, float %R894)  ; Round_ni(value)
  %R898 = fsub fast float %R883, %R895
  %R899 = fsub fast float %R884, %R896
  %R900 = fsub fast float %R885, %R897
  %R901 = fmul fast float %R898, %R898
  %R902 = fmul fast float %R899, %R899
  %R903 = fmul fast float %R900, %R900
  %R904 = fmul fast float %R889, %R889
  %R905 = fmul fast float %R890, %R890
  %R906 = fmul fast float %R891, %R891
  %R907 = fadd fast float %R901, %R904
  %R908 = fadd fast float %R902, %R905
  %R909 = fadd fast float %R903, %R906
  %R910 = fmul fast float %R907, 0x3FEB51CB80000000
  %R911 = fmul fast float %R908, 0x3FEB51CB80000000
  %R912 = fmul fast float %R909, 0x3FEB51CB80000000
  %R913 = fsub fast float 0x3FFCAF7C00000000, %R910
  %R914 = fsub fast float 0x3FFCAF7C00000000, %R911
  %R915 = fsub fast float 0x3FFCAF7C00000000, %R912
  %R916 = fmul fast float %R871, %R913
  %R917 = fmul fast float %R872, %R914
  %R918 = fmul fast float %R873, %R915
  %R919 = fmul fast float %R898, %R791
  %R920 = fmul fast float %R889, %R792
  %R921 = fadd fast float %R919, %R920
  %R922 = fmul fast float %R899, %R800
  %R923 = fmul fast float %R900, %R798
  %R924 = fmul fast float %R890, %R801
  %R925 = fmul fast float %R891, %R799
  %R926 = fadd fast float %R922, %R924
  %R927 = fadd fast float %R923, %R925
  %R928 = call float @dx.op.dot3.f32(i32 55, float %R916, float %R917, float %R918, float %R921, float %R926, float %R927)  ; Dot3(ax,ay,az,bx,by,bz)
  %R929 = fmul fast float %R770, 1.300000e+02
  %R930 = fmul fast float %R929, %R928
  %R769 = fadd fast float %R930, %R768
  %R931 = fmul fast float %R764, 0x3FEC152800000000
  %R932 = call float @dx.op.tertiary.f32(i32 46, float %R766, float 0xBFDEAEE880000000, float %R931)  ; FMad(a,b,c)
  %R933 = fmul fast float %R764, 0x3FDEAEE880000000
  %R934 = call float @dx.op.tertiary.f32(i32 46, float %R766, float 0x3FEC152800000000, float %R933)  ; FMad(a,b,c)
  %R935 = fmul fast float %R932, 2.000000e+00
  %R936 = fmul fast float %R934, 2.000000e+00
  %R765 = fadd fast float %R935, 1.000000e+02
  %R767 = fadd fast float %R936, 1.000000e+02
  %R771 = fmul fast float %R770, 5.000000e-01
  %R937 = icmp eq i32 %R774, %R780
  %R938 = icmp eq i32 %R777, 0
  %R939 = and i1 %R938, %R937
  br i1 %R939, label %R782, label %R762

; <label>:939                                     ; preds = %R763, %R762
  %R940 = phi float [ %R768, %R762 ], [ %R769, %R763 ]
  %R941 = fsub fast float %R940, %R207
  %R942 = fmul fast float %R208, %R941
  %R943 = fmul fast float %R393, 0xBFB99999A0000000
  %R944 = fmul fast float %R576, 0xBFB99999A0000000
  %R945 = call float @dx.op.dot3.f32(i32 55, float %R943, float 0x3F847AE160000000, float %R944, float %R943, float 0x3F847AE160000000, float %R944)  ; Dot3(ax,ay,az,bx,by,bz)
  %R946 = call float @dx.op.unary.f32(i32 25, float %R945)  ; Rsqrt(value)
  %R947 = fmul fast float %R946, %R943
  %R948 = fmul fast float %R946, %R944
  %R949 = fmul fast float %R759, 0x3FB99999A0000000
  %R950 = fmul fast float %R942, 0x3FB99999A0000000
  %R951 = call float @dx.op.dot3.f32(i32 55, float %R949, float 0x3F847AE160000000, float %R950, float %R949, float 0x3F847AE160000000, float %R950)  ; Dot3(ax,ay,az,bx,by,bz)
  %R952 = call float @dx.op.unary.f32(i32 25, float %R951)  ; Rsqrt(value)
  %R953 = fmul fast float %R952, %R949
  %R954 = fmul fast float %R952, %R950
  %R955 = fadd fast float %R953, %R947
  %R956 = fadd fast float %R952, %R946
  %R957 = fadd fast float %R954, %R948
  %R958 = fmul fast float %R955, 5.000000e-01
  %R959 = fmul fast float %R956, 0x3F747AE160000000
  %R960 = fmul fast float %R957, 5.000000e-01
  %R961 = bitcast float %R21 to i32
  %R962 = bitcast float %R210 to i32
  %R963 = bitcast float %R22 to i32
  %R964 = shl i32 %R3, 5
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R1, i32 %R964, i32 undef, i32 %R961, i32 %R962, i32 %R963, i32 undef, i8 7)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R965 = bitcast float %R958 to i32
  %R966 = bitcast float %R959 to i32
  %R967 = bitcast float %R960 to i32
  %R968 = or i32 %R964, 16
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R1, i32 %R968, i32 undef, i32 %R965, i32 %R966, i32 %R967, i32 undef, i8 7)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R969 = mul i32 %R3, 6
  %R970 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R971 = extractvalue %dx.types.CBufRet.i32 %R970, 0
  %R972 = extractvalue %dx.types.CBufRet.i32 %R970, 1
  %R973 = mul i32 %R971, 6
  %R974 = mul i32 %R973, %R972
  %R975 = icmp ult i32 %R969, %R974
  br i1 %R975, label %R976, label %R977

; <label>:976                                     ; preds = %R782
  %R978 = icmp eq i32 %R971, 0
  %R979 = select i1 %R978, i32 1, i32 %R971
  %R980 = udiv i32 %R3, %R979
  %R981 = add i32 %R980, %R3
  %R982 = add i32 %R981, 1
  %R983 = add i32 %R981, %R971
  %R984 = add i32 %R983, 1
  %R985 = add i32 %R983, 2
  %R986 = mul i32 %R3, 24
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R986, i32 undef, i32 %R981, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R987 = or i32 %R986, 4
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R987, i32 undef, i32 %R984, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R988 = add i32 %R986, 8
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R988, i32 undef, i32 %R985, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R989 = add i32 %R986, 12
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R989, i32 undef, i32 %R981, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R990 = add i32 %R986, 16
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R990, i32 undef, i32 %R985, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R991 = add i32 %R986, 20
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R991, i32 undef, i32 %R982, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  br label %R977

; <label>:991                                     ; preds = %R976, %R782
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A2

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32, %dx.types.Handle, i32) #A2

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

; Function Attrs: nounwind readnone
declare float @dx.op.dot2.f32(i32, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.dot3.f32(i32, float, float, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.tertiary.f32(i32, float, float, float) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.unary.f32(i32, float) #A0

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }
attributes #A2 = { nounwind readonly }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{null, !M6, !M7, null}
!M6 = !{!M8, !M9}
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M9 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 2, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M7 = !{!M10}
!M10 = !{i32 0, %chunk_data* undef, !"", i32 0, i32 0, i32 1, i32 24, null}
!M5 = !{void ()* @gen_terrain_compute, !"gen_terrain_compute", null, !M4, !M11}
!M11 = !{i32 0, i64 16, i32 4, !M12}
!M12 = !{i32 64, i32 1, i32 1}

