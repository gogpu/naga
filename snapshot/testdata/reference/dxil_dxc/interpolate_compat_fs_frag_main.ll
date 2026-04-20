;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   x           0     NONE    uint
; LOC                      2    y          0     NONE    uint
; LOC                      3   x           1     NONE   float
; LOC                      7    yzw        1     NONE   float
; LOC                      4   xy          2     NONE   float
; LOC                      6   xyz         3     NONE   float
; LOC                      8   xyzw        4     NONE   float
; LOC                      9   x           5     NONE   float
; LOC                     10   x           6     NONE   float
; LOC                     11   x           7     NONE   float
; SV_Position              0   xyzw        8      POS   float
;
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
; Pixel Shader
; DepthOutput=0
; SampleFrequency=1
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 11
; SigOutputElements: 0
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 9
; SigOutputVectors[0]: 0
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: frag_main
;
;
; Input signature:
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
; Number of inputs: 36, outputs: 0
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @frag_main() {
  ret void
}

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
!M4 = !{[2 x i32] [i32 36, i32 0]}
!M5 = !{void ()* @frag_main, !"frag_main", !M6, null, null}
!M6 = !{!M7, null, null}
!M7 = !{!M8, !M9, !M10, !M11, !M12, !M13, !M14, !M15, !M16, !M17, !M18}
!M8 = !{i32 0, !"LOC", i8 5, i8 0, !M19, i8 1, i32 1, i8 1, i32 0, i8 0, null}
!M19 = !{i32 0}
!M9 = !{i32 1, !"LOC", i8 5, i8 0, !M20, i8 1, i32 1, i8 1, i32 0, i8 1, null}
!M20 = !{i32 2}
!M10 = !{i32 2, !"LOC", i8 9, i8 0, !M21, i8 4, i32 1, i8 1, i32 1, i8 0, null}
!M21 = !{i32 3}
!M11 = !{i32 3, !"LOC", i8 9, i8 0, !M22, i8 5, i32 1, i8 2, i32 2, i8 0, null}
!M22 = !{i32 4}
!M12 = !{i32 4, !"LOC", i8 9, i8 0, !M23, i8 7, i32 1, i8 3, i32 3, i8 0, null}
!M23 = !{i32 6}
!M13 = !{i32 5, !"LOC", i8 9, i8 0, !M24, i8 4, i32 1, i8 3, i32 1, i8 1, null}
!M24 = !{i32 7}
!M14 = !{i32 6, !"LOC", i8 9, i8 0, !M25, i8 2, i32 1, i8 4, i32 4, i8 0, null}
!M25 = !{i32 8}
!M15 = !{i32 7, !"LOC", i8 9, i8 0, !M26, i8 3, i32 1, i8 1, i32 5, i8 0, null}
!M26 = !{i32 9}
!M16 = !{i32 8, !"LOC", i8 9, i8 0, !M27, i8 6, i32 1, i8 1, i32 6, i8 0, null}
!M27 = !{i32 10}
!M17 = !{i32 9, !"LOC", i8 9, i8 0, !M28, i8 2, i32 1, i8 1, i32 7, i8 0, null}
!M28 = !{i32 11}
!M18 = !{i32 10, !"SV_Position", i8 9, i8 3, !M19, i8 4, i32 1, i8 4, i32 8, i8 0, null}

