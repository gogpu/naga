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
; LOC                      0   x           0     NONE    uint   x
; LOC                      2    y          0     NONE    uint    y
; LOC                      3   x           1     NONE   float   x
; LOC                      7    yzw        1     NONE   float    yzw
; LOC                      4   xy          2     NONE   float   xy
; LOC                      6   xyz         3     NONE   float   xyz
; LOC                      8   xyzw        4     NONE   float   xyzw
; LOC                      9   x           5     NONE   float   x
; LOC                     10   x           6     NONE   float   x
; LOC                     11   x           7     NONE   float   x
; SV_Position              0   xyzw        8      POS   float   xyzw
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
; SigOutputElements: 11
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 9
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vert_main
;
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0        nointerpolation
; LOC                      2        nointerpolation
; LOC                      3          noperspective
; LOC                      4 noperspective centroid
; LOC                      6   noperspective sample
; LOC                      7          noperspective
; LOC                      8                 linear
; LOC                      9               centroid
; LOC                     10                 sample
; LOC                     11                 linear
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
; Number of inputs: 0, outputs: 36
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @vert_main() {
  call void @dx.op.storeOutput.i32(i32 5, i32 0, i32 0, i8 0, i32 8)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 1, i32 0, i8 0, i32 10)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float 2.700000e+01)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 0, float 6.400000e+01)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 1, float 1.250000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 0, float 2.160000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 1, float 3.430000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 2, float 5.120000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 0, float 2.550000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 1, float 5.110000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 2, float 1.024000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 0, float 7.290000e+02)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 1, float 1.000000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 2, float 1.331000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 3, float 1.728000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 0, float 2.197000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 8, i32 0, i8 0, float 2.744000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 9, i32 0, i8 0, float 2.812000e+03)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 10, i32 0, i8 0, float 2.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 10, i32 0, i8 1, float 4.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 10, i32 0, i8 2, float 5.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 10, i32 0, i8 3, float 6.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
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
!M2 = !{i32 1, i32 0}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{[2 x i32] [i32 0, i32 36]}
!M5 = !{void ()* @vert_main, !"vert_main", !M6, null, null}
!M6 = !{null, !M7, null}
!M7 = !{!M8, !M9, !M10, !M11, !M12, !M13, !M14, !M15, !M16, !M17, !M18}
!M8 = !{i32 0, !"LOC", i8 5, i8 0, !M19, i8 1, i32 1, i8 1, i32 0, i8 0, !M20}
!M19 = !{i32 0}
!M20 = !{i32 3, i32 1}
!M9 = !{i32 1, !"LOC", i8 5, i8 0, !M21, i8 1, i32 1, i8 1, i32 0, i8 1, !M20}
!M21 = !{i32 2}
!M10 = !{i32 2, !"LOC", i8 9, i8 0, !M22, i8 4, i32 1, i8 1, i32 1, i8 0, !M20}
!M22 = !{i32 3}
!M11 = !{i32 3, !"LOC", i8 9, i8 0, !M23, i8 5, i32 1, i8 2, i32 2, i8 0, !M24}
!M23 = !{i32 4}
!M24 = !{i32 3, i32 3}
!M12 = !{i32 4, !"LOC", i8 9, i8 0, !M25, i8 7, i32 1, i8 3, i32 3, i8 0, !M26}
!M25 = !{i32 6}
!M26 = !{i32 3, i32 7}
!M13 = !{i32 5, !"LOC", i8 9, i8 0, !M27, i8 4, i32 1, i8 3, i32 1, i8 1, !M26}
!M27 = !{i32 7}
!M14 = !{i32 6, !"LOC", i8 9, i8 0, !M28, i8 2, i32 1, i8 4, i32 4, i8 0, !M29}
!M28 = !{i32 8}
!M29 = !{i32 3, i32 15}
!M15 = !{i32 7, !"LOC", i8 9, i8 0, !M30, i8 3, i32 1, i8 1, i32 5, i8 0, !M20}
!M30 = !{i32 9}
!M16 = !{i32 8, !"LOC", i8 9, i8 0, !M31, i8 6, i32 1, i8 1, i32 6, i8 0, !M20}
!M31 = !{i32 10}
!M17 = !{i32 9, !"LOC", i8 9, i8 0, !M32, i8 2, i32 1, i8 1, i32 7, i8 0, !M20}
!M32 = !{i32 11}
!M18 = !{i32 10, !"SV_Position", i8 9, i8 3, !M19, i8 4, i32 1, i8 4, i32 8, i8 0, !M29}

