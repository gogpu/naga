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
; EntryFunctionName: texture_sample_comparison
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
; nagaGroup1SamplerIndexArray       texture  struct         r/o      T0    t1,space255     1
; image_2d_depth                    texture     f32          2d      T1      t2,space1     1
; image_2d_array_depth              texture     f32     2darray      T2      t3,space1     1
; image_cube_depth                  texture     f32        cube      T3      t4,space1     1
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
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%"class.Texture2D<float>" = type { float, %"class.Texture2D<float>::mips_type" }
%"class.Texture2D<float>::mips_type" = type { i32 }
%"class.Texture2DArray<float>" = type { float, %"class.Texture2DArray<float>::mips_type" }
%"class.Texture2DArray<float>::mips_type" = type { i32 }
%"class.TextureCube<float>" = type { float }
%struct.S0 = type { i32 }

define void @texture_sample_comparison() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 3, i32 4, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 2, i32 3, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R3, i32 1, i32 0)  ; BufferLoad(srv,index,wot)
  %R5 = extractvalue %dx.types.ResRet.i32 %R4, 0
  %R6 = add i32 %R5, 0
  %R7 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R6, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R8 = call %dx.types.ResRet.f32 @dx.op.sampleCmp.f32(i32 64, %dx.types.Handle %R2, %dx.types.Handle %R7, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 undef, float 5.000000e-01, float undef)  ; SampleCmp(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue,clamp)
  %R9 = extractvalue %dx.types.ResRet.f32 %R8, 0
  %R10 = call %dx.types.ResRet.f32 @dx.op.sampleCmp.f32(i32 64, %dx.types.Handle %R1, %dx.types.Handle %R7, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 0, i32 0, i32 undef, float 5.000000e-01, float undef)  ; SampleCmp(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue,clamp)
  %R11 = extractvalue %dx.types.ResRet.f32 %R10, 0
  %R12 = call %dx.types.ResRet.f32 @dx.op.sampleCmp.f32(i32 64, %dx.types.Handle %R0, %dx.types.Handle %R7, float 5.000000e-01, float 5.000000e-01, float 5.000000e-01, float undef, i32 undef, i32 undef, i32 undef, float 5.000000e-01, float undef)  ; SampleCmp(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue,clamp)
  %R13 = extractvalue %dx.types.ResRet.f32 %R12, 0
  %R14 = call %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32 65, %dx.types.Handle %R2, %dx.types.Handle %R7, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 undef, float 5.000000e-01)  ; SampleCmpLevelZero(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue)
  %R15 = extractvalue %dx.types.ResRet.f32 %R14, 0
  %R16 = call %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32 65, %dx.types.Handle %R1, %dx.types.Handle %R7, float 5.000000e-01, float 5.000000e-01, float 0.000000e+00, float undef, i32 0, i32 0, i32 undef, float 5.000000e-01)  ; SampleCmpLevelZero(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue)
  %R17 = extractvalue %dx.types.ResRet.f32 %R16, 0
  %R18 = call %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32 65, %dx.types.Handle %R0, %dx.types.Handle %R7, float 5.000000e-01, float 5.000000e-01, float 5.000000e-01, float undef, i32 undef, i32 undef, i32 undef, float 5.000000e-01)  ; SampleCmpLevelZero(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,offset2,compareValue)
  %R19 = extractvalue %dx.types.ResRet.f32 %R18, 0
  %R20 = fmul fast float %R17, 2.000000e+00
  %R21 = fmul fast float %R11, 2.000000e+00
  %R22 = fadd fast float %R13, %R9
  %R23 = fadd fast float %R22, %R15
  %R24 = fadd fast float %R23, %R19
  %R25 = fadd fast float %R24, %R20
  %R26 = fadd fast float %R25, %R21
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R26)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sampleCmpLevelZero.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.sampleCmp.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A1

attributes #A0 = { nounwind }
attributes #A1 = { nounwind readonly }

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
!M7 = !{!M9, !M10, !M11, !M12}
!M9 = !{i32 0, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 1, i32 1, i32 12, i32 0, !M13}
!M13 = !{i32 1, i32 4}
!M10 = !{i32 1, %"class.Texture2D<float>"* undef, !"", i32 1, i32 2, i32 1, i32 2, i32 0, !M14}
!M14 = !{i32 0, i32 9}
!M11 = !{i32 2, %"class.Texture2DArray<float>"* undef, !"", i32 1, i32 3, i32 1, i32 7, i32 0, !M14}
!M12 = !{i32 3, %"class.TextureCube<float>"* undef, !"", i32 1, i32 4, i32 1, i32 5, i32 0, !M14}
!M8 = !{!M15}
!M15 = !{i32 0, [2048 x %struct.S0]* undef, !"", i32 1, i32 0, i32 2048, i32 1, null}
!M5 = !{[2 x i32] [i32 0, i32 1]}
!M6 = !{void ()* @texture_sample_comparison, !"texture_sample_comparison", !M16, !M4, !M17}
!M16 = !{null, !M18, null}
!M18 = !{!M19}
!M19 = !{i32 0, !"SV_Target", i8 9, i8 16, !M20, i8 0, i32 1, i8 1, i32 0, i8 0, !M21}
!M20 = !{i32 0}
!M21 = !{i32 3, i32 1}
!M17 = !{i32 0, i64 16}

