;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   x           0     NONE   float
; LOC                      1    y          0     NONE   float
; LOC                      2     zw        0     NONE   float
; LOC                      3   xy          1     NONE   float
; LOC                      4   xyz         2     NONE   float
; LOC                      5   xyz         3     NONE   float
; LOC                      6   xyzw        4     NONE   float
; LOC                      7   xyzw        5     NONE   float
;
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
; SigInputElements: 8
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 6
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: test_return_partial
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0                 linear
; LOC                      1                 linear
; LOC                      2                 linear
; LOC                      3                 linear
; LOC                      4                 linear
; LOC                      5                 linear
; LOC                      6                 linear
; LOC                      7                 linear
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
;
;
; ViewId state:
;
; Number of inputs: 24, outputs: 1
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @test_return_partial() {
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
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
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{[26 x i32] [i32 24, i32 1, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0]}
!M5 = !{void ()* @test_return_partial, !"test_return_partial", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9, !M10, !M11, !M12, !M13, !M14, !M15, !M16}
!M9 = !{i32 0, !"LOC", i8 9, i8 0, !M17, i8 2, i32 1, i8 1, i32 0, i8 0, null}
!M17 = !{i32 0}
!M10 = !{i32 1, !"LOC", i8 9, i8 0, !M18, i8 2, i32 1, i8 1, i32 0, i8 1, null}
!M18 = !{i32 1}
!M11 = !{i32 2, !"LOC", i8 9, i8 0, !M19, i8 2, i32 1, i8 2, i32 0, i8 2, null}
!M19 = !{i32 2}
!M12 = !{i32 3, !"LOC", i8 9, i8 0, !M20, i8 2, i32 1, i8 2, i32 1, i8 0, null}
!M20 = !{i32 3}
!M13 = !{i32 4, !"LOC", i8 9, i8 0, !M21, i8 2, i32 1, i8 3, i32 2, i8 0, null}
!M21 = !{i32 4}
!M14 = !{i32 5, !"LOC", i8 9, i8 0, !M22, i8 2, i32 1, i8 3, i32 3, i8 0, null}
!M22 = !{i32 5}
!M15 = !{i32 6, !"LOC", i8 9, i8 0, !M23, i8 2, i32 1, i8 4, i32 4, i8 0, null}
!M23 = !{i32 6}
!M16 = !{i32 7, !"LOC", i8 9, i8 0, !M24, i8 2, i32 1, i8 4, i32 5, i8 0, null}
!M24 = !{i32 7}
!M8 = !{!M25}
!M25 = !{i32 0, !"SV_Target", i8 9, i8 16, !M17, i8 0, i32 1, i8 1, i32 0, i8 0, !M26}
!M26 = !{i32 3, i32 1}

