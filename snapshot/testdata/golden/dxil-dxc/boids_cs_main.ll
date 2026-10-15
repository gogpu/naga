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
; EntryFunctionName: main
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; params                            cbuffer      NA          NA     CB0            cb0     1
; particlesSrc                      texture    byte         r/o      T0             t1     1
; particlesDst                          UAV    byte         r/w      U0             u2     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%struct.S0 = type { i32 }
%struct.S1 = type { i32 }
%params = type { %struct.S2 }
%struct.S2 = type { float, float, float, float, float, float, float }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R4 = icmp ugt i32 %R3, 1499
  br i1 %R4, label %R5, label %R6

; <label>:6                                       ; preds = %R7
  %R8 = shl i32 %R3, 4
  %R9 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R8, i32 undef)  ; BufferLoad(srv,index,wot)
  %R10 = extractvalue %dx.types.ResRet.i32 %R9, 0
  %R11 = extractvalue %dx.types.ResRet.i32 %R9, 1
  %R12 = bitcast i32 %R10 to float
  %R13 = bitcast i32 %R11 to float
  %R14 = or i32 %R8, 8
  %R15 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R14, i32 undef)  ; BufferLoad(srv,index,wot)
  %R16 = extractvalue %dx.types.ResRet.i32 %R15, 0
  %R17 = extractvalue %dx.types.ResRet.i32 %R15, 1
  %R18 = bitcast i32 %R16 to float
  %R19 = bitcast i32 %R17 to float
  br label %R20

; <label>:19                                      ; preds = %R21, %R6
  %R22 = phi i32 [ %R23, %R21 ], [ -1, %R6 ]
  %R24 = phi i32 [ %R25, %R21 ], [ -1, %R6 ]
  %R26 = phi i32 [ %R27, %R21 ], [ 0, %R6 ]
  %R28 = phi i32 [ %R29, %R21 ], [ 0, %R6 ]
  %R30 = phi i32 [ %R31, %R21 ], [ 0, %R6 ]
  %R32 = phi float [ %R33, %R21 ], [ 0.000000e+00, %R6 ]
  %R34 = phi float [ %R35, %R21 ], [ 0.000000e+00, %R6 ]
  %R36 = phi float [ %R37, %R21 ], [ 0.000000e+00, %R6 ]
  %R38 = phi float [ %R39, %R21 ], [ 0.000000e+00, %R6 ]
  %R40 = phi float [ %R41, %R21 ], [ 0.000000e+00, %R6 ]
  %R42 = phi float [ %R43, %R21 ], [ 0.000000e+00, %R6 ]
  %R44 = phi i32 [ 1, %R21 ], [ 0, %R6 ]
  %R45 = icmp eq i32 %R24, 0
  %R46 = zext i1 %R45 to i32
  %R23 = sub i32 %R22, %R46
  %R25 = add i32 %R24, -1
  %R27 = add i32 %R44, %R26
  %R47 = icmp ugt i32 %R27, 1499
  br i1 %R47, label %R48, label %R49

; <label>:38                                      ; preds = %R20
  %R50 = icmp eq i32 %R27, %R3
  br i1 %R50, label %R21, label %R51

; <label>:40                                      ; preds = %R52, %R53, %R49
  %R29 = phi i32 [ %R28, %R49 ], [ %R54, %R52 ], [ %R28, %R53 ]
  %R31 = phi i32 [ %R30, %R49 ], [ %R55, %R52 ], [ %R55, %R53 ]
  %R33 = phi float [ %R32, %R49 ], [ %R56, %R52 ], [ %R56, %R53 ]
  %R35 = phi float [ %R34, %R49 ], [ %R57, %R52 ], [ %R57, %R53 ]
  %R37 = phi float [ %R36, %R49 ], [ %R58, %R52 ], [ %R36, %R53 ]
  %R39 = phi float [ %R38, %R49 ], [ %R59, %R52 ], [ %R38, %R53 ]
  %R41 = phi float [ %R40, %R49 ], [ %R60, %R52 ], [ %R60, %R53 ]
  %R43 = phi float [ %R42, %R49 ], [ %R61, %R52 ], [ %R61, %R53 ]
  %R62 = icmp eq i32 %R22, %R46
  %R63 = icmp eq i32 %R25, 0
  %R64 = and i1 %R63, %R62
  br i1 %R64, label %R48, label %R20

; <label>:52                                      ; preds = %R49
  %R65 = shl i32 %R27, 4
  %R66 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R65, i32 undef)  ; BufferLoad(srv,index,wot)
  %R67 = extractvalue %dx.types.ResRet.i32 %R66, 0
  %R68 = extractvalue %dx.types.ResRet.i32 %R66, 1
  %R69 = bitcast i32 %R67 to float
  %R70 = bitcast i32 %R68 to float
  %R71 = or i32 %R65, 8
  %R72 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R71, i32 undef)  ; BufferLoad(srv,index,wot)
  %R73 = extractvalue %dx.types.ResRet.i32 %R72, 0
  %R74 = extractvalue %dx.types.ResRet.i32 %R72, 1
  %R75 = bitcast i32 %R73 to float
  %R76 = bitcast i32 %R74 to float
  %R77 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R78 = extractvalue %dx.types.CBufRet.f32 %R77, 1
  %R79 = fsub fast float %R69, %R12
  %R80 = fsub fast float %R70, %R13
  %R81 = fmul fast float %R79, %R79
  %R82 = fmul fast float %R80, %R80
  %R83 = fadd fast float %R81, %R82
  %R84 = call float @dx.op.unary.f32(i32 24, float %R83)  ; Sqrt(value)
  %R85 = fcmp olt float %R84, %R78
  br i1 %R85, label %R86, label %R53

; <label>:74                                      ; preds = %R51
  %R87 = fadd fast float %R69, %R40
  %R88 = fadd fast float %R70, %R42
  %R89 = add i32 %R30, 1
  br label %R53

; <label>:78                                      ; preds = %R86, %R51
  %R55 = phi i32 [ %R89, %R86 ], [ %R30, %R51 ]
  %R60 = phi float [ %R87, %R86 ], [ %R40, %R51 ]
  %R61 = phi float [ %R88, %R86 ], [ %R42, %R51 ]
  %R90 = extractvalue %dx.types.CBufRet.f32 %R77, 2
  %R91 = fcmp olt float %R84, %R90
  %R92 = fadd fast float %R32, %R12
  %R93 = fsub fast float %R92, %R69
  %R94 = fadd fast float %R34, %R13
  %R95 = fsub fast float %R94, %R70
  %R56 = select i1 %R91, float %R93, float %R32
  %R57 = select i1 %R91, float %R95, float %R34
  %R96 = extractvalue %dx.types.CBufRet.f32 %R77, 3
  %R97 = fcmp olt float %R84, %R96
  br i1 %R97, label %R52, label %R21

; <label>:92                                      ; preds = %R53
  %R58 = fadd fast float %R75, %R36
  %R59 = fadd fast float %R76, %R38
  %R54 = add i32 %R28, 1
  br label %R21

; <label>:96                                      ; preds = %R21, %R20
  %R98 = phi i32 [ %R29, %R21 ], [ %R28, %R20 ]
  %R99 = phi i32 [ %R31, %R21 ], [ %R30, %R20 ]
  %R100 = phi float [ %R33, %R21 ], [ %R32, %R20 ]
  %R101 = phi float [ %R35, %R21 ], [ %R34, %R20 ]
  %R102 = phi float [ %R37, %R21 ], [ %R36, %R20 ]
  %R103 = phi float [ %R39, %R21 ], [ %R38, %R20 ]
  %R104 = phi float [ %R41, %R21 ], [ %R40, %R20 ]
  %R105 = phi float [ %R43, %R21 ], [ %R42, %R20 ]
  %R106 = icmp sgt i32 %R99, 0
  br i1 %R106, label %R107, label %R108

; <label>:106                                     ; preds = %R48
  %R109 = sitofp i32 %R99 to float
  %R110 = fdiv fast float %R104, %R109
  %R111 = fdiv fast float %R105, %R109
  %R112 = fsub fast float %R110, %R12
  %R113 = fsub fast float %R111, %R13
  br label %R108

; <label>:112                                     ; preds = %R107, %R48
  %R114 = phi float [ %R112, %R107 ], [ %R104, %R48 ]
  %R115 = phi float [ %R113, %R107 ], [ %R105, %R48 ]
  %R116 = icmp sgt i32 %R98, 0
  br i1 %R116, label %R117, label %R118

; <label>:116                                     ; preds = %R108
  %R119 = sitofp i32 %R98 to float
  %R120 = fdiv fast float %R102, %R119
  %R121 = fdiv fast float %R103, %R119
  br label %R118

; <label>:120                                     ; preds = %R117, %R108
  %R122 = phi float [ %R120, %R117 ], [ %R102, %R108 ]
  %R123 = phi float [ %R121, %R117 ], [ %R103, %R108 ]
  %R124 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R125 = extractvalue %dx.types.CBufRet.f32 %R124, 0
  %R126 = extractvalue %dx.types.CBufRet.f32 %R124, 1
  %R127 = extractvalue %dx.types.CBufRet.f32 %R124, 2
  %R128 = fmul fast float %R125, %R114
  %R129 = fmul fast float %R125, %R115
  %R130 = fadd fast float %R128, %R18
  %R131 = fadd fast float %R129, %R19
  %R132 = fmul fast float %R126, %R100
  %R133 = fmul fast float %R126, %R101
  %R134 = fadd fast float %R130, %R132
  %R135 = fadd fast float %R131, %R133
  %R136 = fmul fast float %R127, %R122
  %R137 = fmul fast float %R127, %R123
  %R138 = fadd fast float %R134, %R136
  %R139 = fadd fast float %R135, %R137
  %R140 = call float @dx.op.dot2.f32(i32 54, float %R138, float %R139, float %R138, float %R139)  ; Dot2(ax,ay,bx,by)
  %R141 = call float @dx.op.unary.f32(i32 25, float %R140)  ; Rsqrt(value)
  %R142 = fmul fast float %R138, %R138
  %R143 = fmul fast float %R139, %R139
  %R144 = fadd fast float %R142, %R143
  %R145 = call float @dx.op.unary.f32(i32 24, float %R144)  ; Sqrt(value)
  %R146 = call float @dx.op.binary.f32(i32 35, float %R145, float 0.000000e+00)  ; FMax(a,b)
  %R147 = call float @dx.op.binary.f32(i32 36, float %R146, float 0x3FB99999A0000000)  ; FMin(a,b)
  %R148 = fmul fast float %R147, %R141
  %R149 = fmul fast float %R148, %R138
  %R150 = fmul fast float %R148, %R139
  %R151 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R152 = extractvalue %dx.types.CBufRet.f32 %R151, 0
  %R153 = fmul fast float %R152, %R149
  %R154 = fmul fast float %R152, %R150
  %R155 = fadd fast float %R153, %R12
  %R156 = fadd fast float %R154, %R13
  %R157 = fcmp olt float %R155, -1.000000e+00
  %R158 = select i1 %R157, float 1.000000e+00, float %R155
  %R159 = fcmp ogt float %R158, 1.000000e+00
  %R160 = fcmp olt float %R156, -1.000000e+00
  %R161 = select i1 %R160, float 1.000000e+00, float %R156
  %R162 = fcmp ogt float %R161, 1.000000e+00
  %R163 = bitcast float %R158 to i32
  %R164 = select i1 %R159, i32 -1082130432, i32 %R163
  %R165 = bitcast float %R161 to i32
  %R166 = select i1 %R162, i32 -1082130432, i32 %R165
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R8, i32 undef, i32 %R164, i32 %R166, i32 undef, i32 undef, i8 3)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R167 = bitcast float %R149 to i32
  %R168 = bitcast float %R150 to i32
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R14, i32 undef, i32 %R167, i32 %R168, i32 undef, i32 undef, i8 3)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  br label %R5

; <label>:168                                     ; preds = %R118, %R7
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A2

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readnone
declare float @dx.op.dot2.f32(i32, float, float, float, float) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A0

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
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{!M6, !M7, !M8, null}
!M6 = !{!M9}
!M9 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i32 0, null}
!M7 = !{!M10}
!M10 = !{i32 0, %struct.S1* undef, !"", i32 0, i32 2, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M8 = !{!M11}
!M11 = !{i32 0, %params* undef, !"", i32 0, i32 0, i32 1, i32 28, null}
!M5 = !{void ()* @main, !"main", null, !M4, !M12}
!M12 = !{i32 0, i64 16, i32 4, !M13}
!M13 = !{i32 64, i32 1, i32 1}

