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
%dx.types.Dimensions = type { i32, i32, i32, i32 }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%struct.S0 = type { i32 }
%struct.S1 = type { i32 }
%params = type { %struct.S2 }
%struct.S2 = type { float, float }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R4 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R1, i32 undef)  ; GetDimensions(handle,mipLevel)
  %R5 = extractvalue %dx.types.Dimensions %R4, 0
  %R6 = lshr i32 %R5, 4
  %R7 = icmp ult i32 %R3, %R6
  br i1 %R7, label %R8, label %R9

; <label>:9                                       ; preds = %R10
  %R11 = shl i32 %R3, 4
  %R12 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R11, i32 undef)  ; BufferLoad(srv,index,wot)
  %R13 = extractvalue %dx.types.ResRet.i32 %R12, 0
  %R14 = extractvalue %dx.types.ResRet.i32 %R12, 1
  %R15 = extractvalue %dx.types.ResRet.i32 %R12, 2
  %R16 = extractvalue %dx.types.ResRet.i32 %R12, 3
  %R17 = bitcast i32 %R13 to float
  %R18 = bitcast i32 %R14 to float
  %R19 = bitcast i32 %R15 to float
  %R20 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R2, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R21 = extractvalue %dx.types.CBufRet.f32 %R20, 0
  %R22 = extractvalue %dx.types.CBufRet.f32 %R20, 1
  %R23 = fmul fast float %R17, %R21
  %R24 = fmul fast float %R18, %R21
  %R25 = fmul fast float %R19, %R21
  %R26 = fadd fast float %R23, %R22
  %R27 = fadd fast float %R24, %R22
  %R28 = fadd fast float %R25, %R22
  %R29 = bitcast float %R26 to i32
  %R30 = bitcast float %R27 to i32
  %R31 = bitcast float %R28 to i32
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R11, i32 undef, i32 %R29, i32 %R30, i32 %R31, i32 %R16, i8 15)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  br label %R9

; <label>:31                                      ; preds = %R8, %R10
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Dimensions @dx.op.getDimensions(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A2

attributes #A0 = { nounwind readonly }
attributes #A1 = { nounwind }
attributes #A2 = { nounwind readnone }

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

