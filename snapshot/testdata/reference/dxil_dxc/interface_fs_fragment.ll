;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      1   x           0     NONE   float   x
; SV_Position              0   xyzw        1      POS   float
; SV_IsFrontFace           0   x           2    FFACE    uint   x
; SV_SampleIndex           0    N/A  special   SAMPLE    uint     NO
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Target                0   x           0   TARGET   float   x
; SV_Depth                 0    N/A   oDepth    DEPTH   float    YES
; SV_Coverage              0    N/A    oMask COVERAGE    uint    YES
;
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Pixel Shader
; DepthOutput=1
; SampleFrequency=1
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 4
; SigOutputElements: 3
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 3
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: fragment
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      1                 linear
; SV_Position              0          noperspective
; SV_IsFrontFace           0        nointerpolation
; SV_SampleIndex           0        nointerpolation
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Depth                 0
; SV_Coverage              0
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
; Number of inputs: 9, outputs: 1
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 8 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @fragment() {
  %R0 = call i32 @dx.op.loadInput.i32(i32 4, i32 2, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = icmp ne i32 %R0, 0
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call i32 @dx.op.coverage.i32(i32 91)  ; Coverage()
  %R4 = call i32 @dx.op.sampleIndex.i32(i32 90)  ; SampleIndex()
  %R5 = and i32 %R4, 31
  %R6 = shl i32 1, %R5
  %R7 = and i32 %R6, %R3
  %R8 = select i1 %R1, float 1.000000e+00, float 0.000000e+00
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R2)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.i32(i32 5, i32 1, i32 0, i8 0, i32 %R7)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float %R8)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.coverage.i32(i32) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.sampleIndex.i32(i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

; Function Attrs: nounwind
declare void @dx.op.storeOutput.i32(i32, i32, i32, i8, i32) #A1

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }

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
!M4 = !{[11 x i32] [i32 9, i32 1, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 1]}
!M5 = !{void ()* @fragment, !"fragment", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9, !M10, !M11, !M12}
!M9 = !{i32 0, !"LOC", i8 9, i8 0, !M13, i8 2, i32 1, i8 1, i32 0, i8 0, !M14}
!M13 = !{i32 1}
!M14 = !{i32 3, i32 1}
!M10 = !{i32 1, !"SV_Position", i8 9, i8 3, !M15, i8 4, i32 1, i8 4, i32 1, i8 0, null}
!M15 = !{i32 0}
!M11 = !{i32 2, !"SV_IsFrontFace", i8 5, i8 13, !M15, i8 1, i32 1, i8 1, i32 2, i8 0, !M14}
!M12 = !{i32 3, !"SV_SampleIndex", i8 5, i8 12, !M15, i8 1, i32 1, i8 1, i32 -1, i8 -1, null}
!M8 = !{!M16, !M17, !M18}
!M16 = !{i32 0, !"SV_Depth", i8 9, i8 17, !M15, i8 0, i32 1, i8 1, i32 -1, i8 -1, !M14}
!M17 = !{i32 1, !"SV_Coverage", i8 5, i8 14, !M15, i8 0, i32 1, i8 1, i32 -1, i8 -1, !M14}
!M18 = !{i32 2, !"SV_Target", i8 9, i8 16, !M15, i8 0, i32 1, i8 1, i32 0, i8 0, !M14}

