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
; SV_Target                0   x           0   TARGET   float   x
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
; EntryFunctionName: main
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
; nagaComparisonSamplerHeap         sampler      NA          NA      S0      s0,space1  2048
; texture_                          texture     f32     2darray      T0             t0     1
; nagaGroup0SamplerIndexArray       texture  struct         r/o      T1    t0,space255     1
;
;
; ViewId state:
;
; Number of inputs: 0, outputs: 1
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%dx.types.ResRet.f32 = type { float, float, float, float, i32 }
%"class.Texture2DArray<float>" = type { float, %"class.Texture2DArray<float>::mips_type" }
%"class.Texture2DArray<float>::mips_type" = type { i32 }
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%struct.S0 = type { i32 }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 1, i32 0)  ; BufferLoad(srv,index,wot)
  %R3 = extractvalue %dx.types.ResRet.i32 %R2, 0
  %R4 = add i32 %R3, 0
  %R5 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R4, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R6 = call %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32 65, %dx.types.Handle %R1, %dx.types.Handle %R5, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, float undef, i32 0, i32 0, i32 undef, float 0.000000e+00)  ; SampleCmpLevelZero(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue)
  %R7 = extractvalue %dx.types.ResRet.f32 %R6, 0
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R7)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A0

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
!M4 = !{!M7, null, null, !M8}
!M7 = !{!M9, !M10}
!M9 = !{i32 0, %"class.Texture2DArray<float>"* undef, !"", i32 0, i32 0, i32 1, i32 7, i32 0, !M11}
!M11 = !{i32 0, i32 9}
!M10 = !{i32 1, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 0, i32 1, i32 12, i32 0, !M12}
!M12 = !{i32 1, i32 4}
!M8 = !{!M13}
!M13 = !{i32 0, [2048 x %struct.S0]* undef, !"", i32 1, i32 0, i32 2048, i32 1, null}
!M5 = !{[2 x i32] [i32 0, i32 1]}
!M6 = !{void ()* @main, !"main", !M14, !M4, !M15}
!M14 = !{null, !M16, null}
!M16 = !{!M17}
!M17 = !{i32 0, !"SV_Target", i8 9, i8 16, !M18, i8 0, i32 1, i8 1, i32 0, i8 0, !M19}
!M18 = !{i32 0}
!M19 = !{i32 3, i32 1}
!M15 = !{i32 0, i64 16}

