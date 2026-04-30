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
; EntryFunctionName: gather
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
; nagaComparisonSamplerHeap         sampler      NA          NA      S1      s0,space1  2048
; image_2d                          texture     f32          2d      T0             t1     1
; image_2d_u32_                     texture     u32          2d      T1             t2     1
; image_2d_i32_                     texture     i32          2d      T2             t3     1
; nagaGroup1SamplerIndexArray       texture  struct         r/o      T3    t1,space255     1
; image_2d_depth                    texture     f32          2d      T4      t2,space1     1
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
%"class.Texture2D<vector<float, 4> >" = type { <4 x float>, %"class.Texture2D<vector<float, 4> >::mips_type" }
%"class.Texture2D<vector<float, 4> >::mips_type" = type { i32 }
%"class.Texture2D<vector<unsigned int, 4> >" = type { <4 x i32>, %"class.Texture2D<vector<unsigned int, 4> >::mips_type" }
%"class.Texture2D<vector<unsigned int, 4> >::mips_type" = type { i32 }
%"class.Texture2D<vector<int, 4> >" = type { <4 x i32>, %"class.Texture2D<vector<int, 4> >::mips_type" }
%"class.Texture2D<vector<int, 4> >::mips_type" = type { i32 }
%"class.StructuredBuffer<unsigned int>" = type { i32 }
%"class.Texture2D<float>" = type { float, %"class.Texture2D<float>::mips_type" }
%"class.Texture2D<float>::mips_type" = type { i32 }
%struct.S0 = type { i32 }
%struct.S1 = type { i32 }

define void @gather() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 4, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 3, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 2, i32 3, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R5 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 0, i32 0)  ; BufferLoad(srv,index,wot)
  %R6 = extractvalue %dx.types.ResRet.i32 %R5, 0
  %R7 = add i32 %R6, 0
  %R8 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 0, i32 %R7, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R9 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 1, i32 0)  ; BufferLoad(srv,index,wot)
  %R10 = extractvalue %dx.types.ResRet.i32 %R9, 0
  %R11 = add i32 %R10, 0
  %R12 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 3, i32 1, i32 %R11, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R13 = call %dx.types.ResRet.f32 @dx.op.textureGather.f32(i32 73, %dx.types.Handle %R4, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 1)  ; TextureGather(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,channel)
  %R14 = extractvalue %dx.types.ResRet.f32 %R13, 0
  %R15 = extractvalue %dx.types.ResRet.f32 %R13, 1
  %R16 = extractvalue %dx.types.ResRet.f32 %R13, 2
  %R17 = extractvalue %dx.types.ResRet.f32 %R13, 3
  %R18 = call %dx.types.ResRet.f32 @dx.op.textureGather.f32(i32 73, %dx.types.Handle %R4, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 3, i32 1, i32 3)  ; TextureGather(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,channel)
  %R19 = extractvalue %dx.types.ResRet.f32 %R18, 0
  %R20 = extractvalue %dx.types.ResRet.f32 %R18, 1
  %R21 = extractvalue %dx.types.ResRet.f32 %R18, 2
  %R22 = extractvalue %dx.types.ResRet.f32 %R18, 3
  %R23 = call %dx.types.ResRet.f32 @dx.op.textureGatherCmp.f32(i32 74, %dx.types.Handle %R0, %dx.types.Handle %R12, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 0, float 5.000000e-01)  ; TextureGatherCmp(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,channel,compareValue)
  %R24 = extractvalue %dx.types.ResRet.f32 %R23, 0
  %R25 = extractvalue %dx.types.ResRet.f32 %R23, 1
  %R26 = extractvalue %dx.types.ResRet.f32 %R23, 2
  %R27 = extractvalue %dx.types.ResRet.f32 %R23, 3
  %R28 = call %dx.types.ResRet.f32 @dx.op.textureGatherCmp.f32(i32 74, %dx.types.Handle %R0, %dx.types.Handle %R12, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 3, i32 1, i32 0, float 5.000000e-01)  ; TextureGatherCmp(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,channel,compareValue)
  %R29 = extractvalue %dx.types.ResRet.f32 %R28, 0
  %R30 = extractvalue %dx.types.ResRet.f32 %R28, 1
  %R31 = extractvalue %dx.types.ResRet.f32 %R28, 2
  %R32 = extractvalue %dx.types.ResRet.f32 %R28, 3
  %R33 = call %dx.types.ResRet.i32 @dx.op.textureGather.i32(i32 73, %dx.types.Handle %R3, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 0)  ; TextureGather(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,channel)
  %R34 = extractvalue %dx.types.ResRet.i32 %R33, 0
  %R35 = extractvalue %dx.types.ResRet.i32 %R33, 1
  %R36 = extractvalue %dx.types.ResRet.i32 %R33, 2
  %R37 = extractvalue %dx.types.ResRet.i32 %R33, 3
  %R38 = call %dx.types.ResRet.i32 @dx.op.textureGather.i32(i32 73, %dx.types.Handle %R2, %dx.types.Handle %R8, float 5.000000e-01, float 5.000000e-01, float undef, float undef, i32 0, i32 0, i32 0)  ; TextureGather(srv,sampler,coord0,coord1,coord2,coord3,offset0,offset1,channel)
  %R39 = extractvalue %dx.types.ResRet.i32 %R38, 0
  %R40 = extractvalue %dx.types.ResRet.i32 %R38, 1
  %R41 = extractvalue %dx.types.ResRet.i32 %R38, 2
  %R42 = extractvalue %dx.types.ResRet.i32 %R38, 3
  %R43 = uitofp i32 %R34 to float
  %R44 = uitofp i32 %R35 to float
  %R45 = uitofp i32 %R36 to float
  %R46 = uitofp i32 %R37 to float
  %R47 = sitofp i32 %R39 to float
  %R48 = sitofp i32 %R40 to float
  %R49 = sitofp i32 %R41 to float
  %R50 = sitofp i32 %R42 to float
  %R51 = fadd fast float %R19, %R14
  %R52 = fadd fast float %R51, %R24
  %R53 = fadd fast float %R52, %R29
  %R54 = fadd fast float %R53, %R43
  %R55 = fadd fast float %R54, %R47
  %R56 = fadd fast float %R20, %R15
  %R57 = fadd fast float %R56, %R25
  %R58 = fadd fast float %R57, %R30
  %R59 = fadd fast float %R58, %R44
  %R60 = fadd fast float %R59, %R48
  %R61 = fadd fast float %R21, %R16
  %R62 = fadd fast float %R61, %R26
  %R63 = fadd fast float %R62, %R31
  %R64 = fadd fast float %R63, %R45
  %R65 = fadd fast float %R64, %R49
  %R66 = fadd fast float %R22, %R17
  %R67 = fadd fast float %R66, %R27
  %R68 = fadd fast float %R67, %R32
  %R69 = fadd fast float %R68, %R46
  %R70 = fadd fast float %R69, %R50
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R55)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R60)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R65)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R70)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.textureGather.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.textureGather.i32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.f32 @dx.op.textureGatherCmp.f32(i32, %dx.types.Handle, %dx.types.Handle, float, float, float, float, i32, i32, i32, float) #A0

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
!M2 = !{i32 1, i32 0}
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{!M7, null, null, !M8}
!M7 = !{!M9, !M10, !M11, !M12, !M13}
!M9 = !{i32 0, %"class.Texture2D<vector<float, 4> >"* undef, !"", i32 0, i32 1, i32 1, i32 2, i32 0, !M14}
!M14 = !{i32 0, i32 9}
!M10 = !{i32 1, %"class.Texture2D<vector<unsigned int, 4> >"* undef, !"", i32 0, i32 2, i32 1, i32 2, i32 0, !M15}
!M15 = !{i32 0, i32 5}
!M11 = !{i32 2, %"class.Texture2D<vector<int, 4> >"* undef, !"", i32 0, i32 3, i32 1, i32 2, i32 0, !M16}
!M16 = !{i32 0, i32 4}
!M12 = !{i32 3, %"class.StructuredBuffer<unsigned int>"* undef, !"", i32 255, i32 1, i32 1, i32 12, i32 0, !M17}
!M17 = !{i32 1, i32 0}
!M13 = !{i32 4, %"class.Texture2D<float>"* undef, !"", i32 1, i32 2, i32 1, i32 2, i32 0, !M14}
!M8 = !{!M18, !M19}
!M18 = !{i32 0, [2048 x %struct.S0]* undef, !"", i32 0, i32 0, i32 2048, i32 0, null}
!M19 = !{i32 1, [2048 x %struct.S1]* undef, !"", i32 1, i32 0, i32 2048, i32 1, null}
!M5 = !{[2 x i32] [i32 0, i32 4]}
!M6 = !{void ()* @gather, !"gather", !M20, !M4, !M21}
!M20 = !{null, !M22, null}
!M22 = !{!M23}
!M23 = !{i32 0, !"SV_Target", i8 9, i8 16, !M24, i8 0, i32 1, i8 4, i32 0, i8 0, !M25}
!M24 = !{i32 0}
!M25 = !{i32 3, i32 15}
!M21 = !{i32 0, i64 16}

