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
; NumThreads=(1,1,1)
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
; in_data_uniform                   cbuffer      NA          NA     CB0            cb2     1
; in_                                   UAV    byte         r/w      U0             u0     1
; out_                                  UAV    byte         r/w      U1             u1     1
; in_data_storage_g0_b3_                UAV    byte         r/w      U2             u3     1
; in_data_storage_g0_b4_                UAV    byte         r/w      U3             u4     1
; in_data_storage_g1_b0_                UAV    byte         r/w      U4      u0,space1     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.CBufRet.i32 = type { i32, i32, i32, i32 }
%struct.S0 = type { i32 }
%in_data_uniform = type { [1 x %struct.S1] }
%struct.S1 = type { i32, i32, i32, i32 }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 4, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 3, i32 4, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 2, i32 3, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R5 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R6 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R4, i32 0, i32 undef)  ; BufferLoad(srv,index,wot)
  %R7 = extractvalue %dx.types.ResRet.i32 %R6, 0
  %R8 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R5, i32 %R7)  ; CBufferLoadLegacy(handle,regIndex)
  %R9 = extractvalue %dx.types.CBufRet.i32 %R8, 0
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R3, i32 0, i32 undef, i32 %R9, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R10 = shl i32 %R7, 4
  %R11 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R2, i32 %R10, i32 undef)  ; BufferLoad(srv,index,wot)
  %R12 = extractvalue %dx.types.ResRet.i32 %R11, 0
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R3, i32 4, i32 undef, i32 %R12, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R13 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 %R10, i32 undef)  ; BufferLoad(srv,index,wot)
  %R14 = extractvalue %dx.types.ResRet.i32 %R13, 0
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R3, i32 8, i32 undef, i32 %R14, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R15 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 %R10, i32 undef)  ; BufferLoad(srv,index,wot)
  %R16 = extractvalue %dx.types.ResRet.i32 %R15, 0
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R3, i32 12, i32 undef, i32 %R16, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

attributes #A0 = { nounwind readonly }
attributes #A1 = { nounwind }

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
!M4 = !{null, !M6, !M7, null}
!M6 = !{!M8, !M9, !M10, !M11, !M12}
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M9 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M10 = !{i32 2, %struct.S0* undef, !"", i32 0, i32 3, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M11 = !{i32 3, %struct.S0* undef, !"", i32 0, i32 4, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M12 = !{i32 4, %struct.S0* undef, !"", i32 1, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M7 = !{!M13}
!M13 = !{i32 0, %in_data_uniform* undef, !"", i32 0, i32 2, i32 1, i32 16, null}
!M5 = !{void ()* @main, !"main", null, !M4, !M14}
!M14 = !{i32 0, i64 16, i32 4, !M15}
!M15 = !{i32 1, i32 1, i32 1}

