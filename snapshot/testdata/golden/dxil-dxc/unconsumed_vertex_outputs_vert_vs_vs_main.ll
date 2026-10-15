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
; LOC                      0   x           0     NONE   float   x
; LOC                      1    y          0     NONE   float    y
; LOC                      3     z         0     NONE   float     z
; LOC                      2   xyzw        1     NONE   float   xyzw
; SV_Position              0   xyzw        2      POS   float   xyzw
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
; SigOutputElements: 5
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 3
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vs_main
;
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0                 linear
; LOC                      1                 linear
; LOC                      2                 linear
; LOC                      3                 linear
; SV_Position              0          noperspective
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
; Number of inputs: 0, outputs: 12
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @vs_main() {
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float 2.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 1, float 2.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 2, float 2.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 3, float 2.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 0, float 5.000000e-01)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 0, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 1, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 2, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A0

attributes #A0 = { nounwind }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.viewIdState = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{[2 x i32] [i32 0, i32 12]}
!M5 = !{void ()* @vs_main, !"vs_main", !M6, null, null}
!M6 = !{null, !M7, null}
!M7 = !{!M8, !M9, !M10, !M11, !M12}
!M8 = !{i32 0, !"LOC", i8 9, i8 0, !M13, i8 2, i32 1, i8 1, i32 0, i8 0, !M14}
!M13 = !{i32 0}
!M14 = !{i32 3, i32 1}
!M9 = !{i32 1, !"LOC", i8 9, i8 0, !M15, i8 2, i32 1, i8 1, i32 0, i8 1, !M14}
!M15 = !{i32 1}
!M10 = !{i32 2, !"LOC", i8 9, i8 0, !M16, i8 2, i32 1, i8 4, i32 1, i8 0, !M17}
!M16 = !{i32 2}
!M17 = !{i32 3, i32 15}
!M11 = !{i32 3, !"LOC", i8 9, i8 0, !M18, i8 2, i32 1, i8 1, i32 0, i8 2, !M14}
!M18 = !{i32 3}
!M12 = !{i32 4, !"SV_Position", i8 9, i8 3, !M13, i8 4, i32 1, i8 4, i32 2, i8 0, !M17}

