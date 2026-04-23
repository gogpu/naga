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
; SV_Position              0   xyzw        0      POS   float   xyzw
;
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Vertex Shader
; OutputPositionPresent=1
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
; EntryFunctionName: queries
;
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Position              0          noperspective
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; image_1d                          texture     f32          1d      T0             t0     1
; image_2d                          texture     f32          2d      T1             t1     1
; image_2d_array                    texture     f32     2darray      T2             t4     1
; image_cube                        texture     f32        cube      T3             t5     1
; image_cube_array                  texture     f32   cubearray      T4             t6     1
; image_3d                          texture     f32          3d      T5             t7     1
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
%dx.types.Dimensions = type { i32, i32, i32, i32 }
%"class.Texture1D<vector<float, 4> >" = type { <4 x float>, %"class.Texture1D<vector<float, 4> >::mips_type" }
%"class.Texture1D<vector<float, 4> >::mips_type" = type { i32 }
%"class.Texture2D<vector<float, 4> >" = type { <4 x float>, %"class.Texture2D<vector<float, 4> >::mips_type" }
%"class.Texture2D<vector<float, 4> >::mips_type" = type { i32 }
%"class.Texture2DArray<vector<float, 4> >" = type { <4 x float>, %"class.Texture2DArray<vector<float, 4> >::mips_type" }
%"class.Texture2DArray<vector<float, 4> >::mips_type" = type { i32 }
%"class.TextureCube<vector<float, 4> >" = type { <4 x float> }
%"class.TextureCubeArray<vector<float, 4> >" = type { <4 x float> }
%"class.Texture3D<vector<float, 4> >" = type { <4 x float>, %"class.Texture3D<vector<float, 4> >::mips_type" }
%"class.Texture3D<vector<float, 4> >::mips_type" = type { i32 }

define void @queries() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 5, i32 7, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 4, i32 6, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 3, i32 5, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R3 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 2, i32 4, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R4 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R5 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R6 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R5, i32 0)  ; GetDimensions(handle,mipLevel)
  %R7 = extractvalue %dx.types.Dimensions %R6, 0
  %R8 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R4, i32 0)  ; GetDimensions(handle,mipLevel)
  %R9 = extractvalue %dx.types.Dimensions %R8, 1
  %R10 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R4, i32 1)  ; GetDimensions(handle,mipLevel)
  %R11 = extractvalue %dx.types.Dimensions %R10, 1
  %R12 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R3, i32 0)  ; GetDimensions(handle,mipLevel)
  %R13 = extractvalue %dx.types.Dimensions %R12, 1
  %R14 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R3, i32 1)  ; GetDimensions(handle,mipLevel)
  %R15 = extractvalue %dx.types.Dimensions %R14, 1
  %R16 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R2, i32 0)  ; GetDimensions(handle,mipLevel)
  %R17 = extractvalue %dx.types.Dimensions %R16, 1
  %R18 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R2, i32 1)  ; GetDimensions(handle,mipLevel)
  %R19 = extractvalue %dx.types.Dimensions %R18, 1
  %R20 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R1, i32 0)  ; GetDimensions(handle,mipLevel)
  %R21 = extractvalue %dx.types.Dimensions %R20, 1
  %R22 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R1, i32 1)  ; GetDimensions(handle,mipLevel)
  %R23 = extractvalue %dx.types.Dimensions %R22, 1
  %R24 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R0, i32 0)  ; GetDimensions(handle,mipLevel)
  %R25 = extractvalue %dx.types.Dimensions %R24, 2
  %R26 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R0, i32 1)  ; GetDimensions(handle,mipLevel)
  %R27 = extractvalue %dx.types.Dimensions %R26, 2
  %R28 = add i32 %R9, %R7
  %R29 = add i32 %R28, %R11
  %R30 = add i32 %R29, %R13
  %R31 = add i32 %R30, %R15
  %R32 = add i32 %R31, %R17
  %R33 = add i32 %R32, %R19
  %R34 = add i32 %R33, %R21
  %R35 = add i32 %R34, %R23
  %R36 = add i32 %R35, %R25
  %R37 = add i32 %R36, %R27
  %R38 = uitofp i32 %R37 to float
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R38)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R38)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R38)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R38)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Dimensions @dx.op.getDimensions(i32, %dx.types.Handle, i32) #A0

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
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{!M7, null, null, null}
!M7 = !{!M8, !M9, !M10, !M11, !M12, !M13}
!M8 = !{i32 0, %"class.Texture1D<vector<float, 4> >"* undef, !"", i32 0, i32 0, i32 1, i32 1, i32 0, !M14}
!M14 = !{i32 0, i32 9}
!M9 = !{i32 1, %"class.Texture2D<vector<float, 4> >"* undef, !"", i32 0, i32 1, i32 1, i32 2, i32 0, !M14}
!M10 = !{i32 2, %"class.Texture2DArray<vector<float, 4> >"* undef, !"", i32 0, i32 4, i32 1, i32 7, i32 0, !M14}
!M11 = !{i32 3, %"class.TextureCube<vector<float, 4> >"* undef, !"", i32 0, i32 5, i32 1, i32 5, i32 0, !M14}
!M12 = !{i32 4, %"class.TextureCubeArray<vector<float, 4> >"* undef, !"", i32 0, i32 6, i32 1, i32 9, i32 0, !M14}
!M13 = !{i32 5, %"class.Texture3D<vector<float, 4> >"* undef, !"", i32 0, i32 7, i32 1, i32 4, i32 0, !M14}
!M5 = !{[2 x i32] [i32 0, i32 4]}
!M6 = !{void ()* @queries, !"queries", !M15, !M4, null}
!M15 = !{null, !M16, null}
!M16 = !{!M17}
!M17 = !{i32 0, !"SV_Position", i8 9, i8 3, !M18, i8 4, i32 1, i8 4, i32 0, i8 0, !M19}
!M18 = !{i32 0}
!M19 = !{i32 3, i32 15}

