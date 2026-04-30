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
; params                            cbuffer      NA          NA     CB0            cb2     1
; pin                               texture    byte         r/o      T0             t0     1
; pout                                  UAV    byte         r/w      U0             u1     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.i32 = type { i32, i32, i32, i32 }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%struct.S0 = type { i32 }
%struct.S1 = type { i32 }
%params = type { %struct.S2 }
%struct.S2 = type { float, i32 }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R4 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R5 = extractvalue %dx.types.CBufRet.i32 %R4, 1
  %R6 = icmp ult i32 %R3, %R5
  br i1 %R6, label %R7, label %R8

; <label>:8                                       ; preds = %R9
  %R10 = shl i32 %R3, 4
  %R11 = or i32 %R10, 8
  %R12 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R11, i32 undef)  ; BufferLoad(srv,index,wot)
  %R13 = extractvalue %dx.types.ResRet.i32 %R12, 0
  %R14 = extractvalue %dx.types.ResRet.i32 %R12, 1
  %R15 = bitcast i32 %R13 to float
  %R16 = bitcast i32 %R14 to float
  %R17 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R10, i32 undef)  ; BufferLoad(srv,index,wot)
  %R18 = extractvalue %dx.types.ResRet.i32 %R17, 0
  %R19 = extractvalue %dx.types.ResRet.i32 %R17, 1
  %R20 = bitcast i32 %R18 to float
  %R21 = bitcast i32 %R19 to float
  %R22 = fmul fast float %R20, %R20
  %R23 = fmul fast float %R21, %R21
  %R24 = fadd fast float %R22, %R23
  %R25 = call float @dx.op.unary.f32(i32 24, float %R24)  ; Sqrt(value)
  %R26 = call float @dx.op.binary.f32(i32 35, float %R25, float 0x3FA99999A0000000)  ; FMax(a,b)
  %R27 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R28 = extractvalue %dx.types.CBufRet.f32 %R27, 0
  %R29 = fsub fast float -0.000000e+00, %R20
  %R30 = fsub fast float -0.000000e+00, %R21
  %R31 = fmul fast float %R26, %R26
  %R32 = fmul fast float %R31, %R26
  %R33 = fdiv fast float %R29, %R32
  %R34 = fdiv fast float %R30, %R32
  %R35 = fmul fast float %R28, 0x3FC3333340000000
  %R36 = fmul fast float %R35, %R33
  %R37 = fmul fast float %R35, %R34
  %R38 = fadd fast float %R36, %R15
  %R39 = fadd fast float %R37, %R16
  %R40 = fmul fast float %R38, 0x3FEFFBE760000000
  %R41 = fmul fast float %R39, 0x3FEFFBE760000000
  %R42 = fmul fast float %R40, %R28
  %R43 = fmul fast float %R41, %R28
  %R44 = fadd fast float %R42, %R20
  %R45 = fadd fast float %R43, %R21
  %R46 = fcmp ogt float %R44, 1.000000e+00
  %R47 = fmul fast float %R38, 0xBFE99652C0000000
  %R48 = select i1 %R46, float 1.000000e+00, float %R44
  %R49 = select i1 %R46, float %R47, float %R40
  %R50 = fcmp olt float %R48, -1.000000e+00
  %R51 = fmul fast float %R49, 0xBFE99999A0000000
  %R52 = select i1 %R50, float %R51, float %R49
  %R53 = fcmp ogt float %R45, 1.000000e+00
  %R54 = fmul fast float %R39, 0xBFE99652C0000000
  %R55 = select i1 %R53, float 1.000000e+00, float %R45
  %R56 = select i1 %R53, float %R54, float %R41
  %R57 = fcmp olt float %R55, -1.000000e+00
  %R58 = fmul fast float %R56, 0xBFE99999A0000000
  %R59 = select i1 %R57, float %R58, float %R56
  %R60 = bitcast float %R48 to i32
  %R61 = select i1 %R50, i32 -1082130432, i32 %R60
  %R62 = bitcast float %R55 to i32
  %R63 = select i1 %R57, i32 -1082130432, i32 %R62
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R10, i32 undef, i32 %R61, i32 %R63, i32 undef, i32 undef, i8 3)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R64 = bitcast float %R52 to i32
  %R65 = bitcast float %R59 to i32
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R11, i32 undef, i32 %R64, i32 %R65, i32 undef, i32 undef, i8 3)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  br label %R8

; <label>:65                                      ; preds = %R7, %R9
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
declare %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

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
!M2 = !{i32 1, i32 8}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{!M6, !M7, !M8, null}
!M6 = !{!M9}
!M9 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i32 0, null}
!M7 = !{!M10}
!M10 = !{i32 0, %struct.S1* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M8 = !{!M11}
!M11 = !{i32 0, %params* undef, !"", i32 0, i32 2, i32 1, i32 8, null}
!M5 = !{void ()* @main, !"main", null, !M4, !M12}
!M12 = !{i32 0, i64 16, i32 4, !M13}
!M13 = !{i32 64, i32 1, i32 1}

