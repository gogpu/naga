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
; EntryFunctionName: csStore
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; s_r_w                                 UAV     f32          2d      U0      u0,space1     1
; s_rg_w                                UAV     f32          2d      U1      u1,space1     1
; s_rgba_w                              UAV     f32          2d      U2      u2,space1     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%"class.RWTexture2D<float>" = type { float }
%"class.RWTexture2D<vector<float, 4> >" = type { <4 x float> }

define void @csStore() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 2, i32 2, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  call void @dx.op.textureStore.f32(i32 67, %dx.types.Handle %R2, i32 0, i32 0, i32 undef, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, i8 15)  ; TextureStore(srv,coord0,coord1,coord2,value0,value1,value2,value3,mask)
  call void @dx.op.textureStore.f32(i32 67, %dx.types.Handle %R1, i32 0, i32 0, i32 undef, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, i8 15)  ; TextureStore(srv,coord0,coord1,coord2,value0,value1,value2,value3,mask)
  call void @dx.op.textureStore.f32(i32 67, %dx.types.Handle %R0, i32 0, i32 0, i32 undef, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, i8 15)  ; TextureStore(srv,coord0,coord1,coord2,value0,value1,value2,value3,mask)
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.textureStore.f32(i32, %dx.types.Handle, i32, i32, i32, float, float, float, float, i8) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

attributes #A0 = { nounwind }
attributes #A1 = { nounwind readonly }

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
!M4 = !{null, !M6, null, null}
!M6 = !{!M7, !M8, !M9}
!M7 = !{i32 0, %"class.RWTexture2D<float>"* undef, !"", i32 1, i32 0, i32 1, i32 2, i1 false, i1 false, i1 false, !M10}
!M10 = !{i32 0, i32 9}
!M8 = !{i32 1, %"class.RWTexture2D<vector<float, 4> >"* undef, !"", i32 1, i32 1, i32 1, i32 2, i1 false, i1 false, i1 false, !M10}
!M9 = !{i32 2, %"class.RWTexture2D<vector<float, 4> >"* undef, !"", i32 1, i32 2, i32 1, i32 2, i1 false, i1 false, i1 false, !M10}
!M5 = !{void ()* @csStore, !"csStore", null, !M4, !M11}
!M11 = !{i32 4, !M12}
!M12 = !{i32 1, i32 1, i32 1}

