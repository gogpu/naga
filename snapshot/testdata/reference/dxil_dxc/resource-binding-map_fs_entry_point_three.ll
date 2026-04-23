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
; EntryFunctionName: entry_point_three
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
; uniformOne                        cbuffer      NA          NA     CB0            cb4     1
; uniformTwo                        cbuffer      NA          NA     CB1     cb0,space1     1
; nagaSamplerHeap                   sampler      NA          NA      S0             s0  2048
; t1_                               texture     f32          2d      T0             t0     1
; t2_                               texture     f32          2d      T1             t1     1
; nagaGroup0SamplerIndexArray       texture  struct         r/o      T2    t0,space255     1
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
%dx.types.CBufRet.f32 = type { float, float, float, float }
%dx.types.ResRet.f32 = type { float, float, float, float, i32 }
%"class.Texture2D<vector<float, 4> >" = type { <4 x float>, %"class.Texture2D<vector<float, 4> >::mips_type" }
%"class.Texture2D<vector<float, 4> >::mips_type" = type { i32 }
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%uniformOne = type { <2 x float> }
%uniformTwo = type { <2 x float> }
%struct.S0 = type { i32 }

define void @entry_point_three() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 2, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 1, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 4, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R5 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 2, i32 0)  ; BufferLoad(srv,index,wot)
  %R6 = extractvalue %dx.types.ResRet.i32 %R5, 0
  %R7 = add i32 %R6, 0
  %R8 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R7, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R9 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 3, i32 0)  ; BufferLoad(srv,index,wot)
  %R10 = extractvalue %dx.types.ResRet.i32 %R9, 0
  %R11 = add i32 %R10, 0
  %R12 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R11, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R13 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R3, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R14 = extractvalue %dx.types.CBufRet.f32 %R13, 0
  %R15 = extractvalue %dx.types.CBufRet.f32 %R13, 1
  %R16 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R4, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R17 = extractvalue %dx.types.CBufRet.f32 %R16, 0
  %R18 = extractvalue %dx.types.CBufRet.f32 %R16, 1
  %R19 = fadd fast float %R17, %R14
  %R20 = fadd fast float %R18, %R15
  %R21 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R2, %dx.types.Handle %R8, float %R19, float %R20, float undef, float undef, i32 0, i32 0, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R22 = extractvalue %dx.types.ResRet.f32 %R21, 0
  %R23 = extractvalue %dx.types.ResRet.f32 %R21, 1
  %R24 = extractvalue %dx.types.ResRet.f32 %R21, 2
  %R25 = extractvalue %dx.types.ResRet.f32 %R21, 3
  %R26 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R4, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R27 = extractvalue %dx.types.CBufRet.f32 %R26, 0
  %R28 = extractvalue %dx.types.CBufRet.f32 %R26, 1
  %R29 = call %dx.types.ResRet.f32 @dx.op.sample.f32(i32 60, %dx.types.Handle %R1, %dx.types.Handle %R12, float %R27, float %R28, float undef, float undef, i32 0, i32 0, i32 undef, float undef)  ; Sample(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,clamp)
  %R30 = extractvalue %dx.types.ResRet.f32 %R29, 0
  %R31 = extractvalue %dx.types.ResRet.f32 %R29, 1
  %R32 = extractvalue %dx.types.ResRet.f32 %R29, 2
  %R33 = extractvalue %dx.types.ResRet.f32 %R29, 3
  %R34 = fadd fast float %R30, %R22
  %R35 = fadd fast float %R31, %R23
  %R36 = fadd fast float %R32, %R24
  %R37 = fadd fast float %R33, %R25
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R34)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R35)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R36)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R37)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sample.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

attributes #A0 = { nounwind readonly }
attributes #A1 = { nounwind }

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
!M10 = !{i32 0, %"class.Texture2D<vector<float, 4> >"* undef, !"", i32 0, i32 0, i32 1, i32 2, i32 0, !M13}
!M13 = !{i32 0, i32 9}
!M11 = !{i32 1, %"class.Texture2D<vector<float, 4> >"* undef, !"", i32 0, i32 1, i32 1, i32 2, i32 0, !M13}
!M12 = !{i32 2, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 0, i32 1, i32 12, i32 0, !M14}
!M14 = !{i32 1, i32 4}
!M8 = !{!M15, !M16}
!M15 = !{i32 0, %uniformOne* undef, !"", i32 0, i32 4, i32 1, i32 8, null}
!M16 = !{i32 1, %uniformTwo* undef, !"", i32 1, i32 0, i32 1, i32 8, null}
!M9 = !{!M17}
!M17 = !{i32 0, [2048 x %struct.S0]* undef, !"", i32 0, i32 0, i32 2048, i32 0, null}
!M5 = !{[2 x i32] [i32 0, i32 4]}
!M6 = !{void ()* @entry_point_three, !"entry_point_three", !M18, !M4, !M19}
!M18 = !{null, !M20, null}
!M20 = !{!M21}
!M21 = !{i32 0, !"SV_Target", i8 9, i8 16, !M22, i8 0, i32 1, i8 4, i32 0, i8 0, !M23}
!M22 = !{i32 0}
!M23 = !{i32 3, i32 15}
!M19 = !{i32 0, i64 16}

