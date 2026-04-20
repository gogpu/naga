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
; SV_Target                0   xy          0   TARGET   float   xy
; SV_Target                1   xy          1   TARGET     int   xy
; SV_Target                2   xy          2   TARGET    uint   xy
; SV_Target                3   x           3   TARGET   float   x
; SV_Target                4   x           4   TARGET     int   x
; SV_Target                5   x           5   TARGET    uint   x
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
; SigOutputElements: 6
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 6
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: main_vec2scalar
;
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Target                0
; SV_Target                1
; SV_Target                2
; SV_Target                3
; SV_Target                4
; SV_Target                5
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
;
;
; ViewId state:
;
; Number of inputs: 0, outputs: 21
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @main_vec2scalar() {
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 1, i32 0, i8 0, i32 0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 1, i32 0, i8 1, i32 0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 2, i32 0, i8 0, i32 0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 2, i32 0, i8 1, i32 0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 4, i32 0, i8 0, i32 0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 5, i32 0, i8 0, i32 0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.i32(i32, i32, i32, i8, i32) #A0

attributes #A0 = { nounwind }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.viewIdState = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 8}
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{[2 x i32] [i32 0, i32 21]}
!M5 = !{void ()* @main_vec2scalar, !"main_vec2scalar", !M6, null, null}
!M6 = !{null, !M7, null}
!M7 = !{!M8, !M9, !M10, !M11, !M12, !M13}
!M8 = !{i32 0, !"SV_Target", i8 9, i8 16, !M14, i8 0, i32 1, i8 2, i32 0, i8 0, !M15}
!M14 = !{i32 0}
!M15 = !{i32 3, i32 3}
!M9 = !{i32 1, !"SV_Target", i8 4, i8 16, !M16, i8 0, i32 1, i8 2, i32 1, i8 0, !M15}
!M16 = !{i32 1}
!M10 = !{i32 2, !"SV_Target", i8 5, i8 16, !M17, i8 0, i32 1, i8 2, i32 2, i8 0, !M15}
!M17 = !{i32 2}
!M11 = !{i32 3, !"SV_Target", i8 9, i8 16, !M18, i8 0, i32 1, i8 1, i32 3, i8 0, !M19}
!M18 = !{i32 3}
!M19 = !{i32 3, i32 1}
!M12 = !{i32 4, !"SV_Target", i8 4, i8 16, !M20, i8 0, i32 1, i8 1, i32 4, i8 0, !M19}
!M20 = !{i32 4}
!M13 = !{i32 5, !"SV_Target", i8 5, i8 16, !M21, i8 0, i32 1, i8 1, i32 5, i8 0, !M19}
!M21 = !{i32 5}

