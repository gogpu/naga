;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Position              0   xyzw        0      POS   float     z
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Depth                 0    N/A   oDepth    DEPTH   float    YES
;
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Pixel Shader
; DepthOutput=1
; SampleFrequency=0
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 1
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 1
; SigOutputVectors[0]: 0
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: main
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Position              0          noperspective
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Depth                 0
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
; Number of inputs: 4, outputs: 0
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @main() {
  %R0 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = fadd fast float %R0, 0xBFB99999A0000000
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

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
!M4 = !{[2 x i32] [i32 4, i32 0]}
!M5 = !{void ()* @main, !"main", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9}
!M9 = !{i32 0, !"SV_Position", i8 9, i8 3, !M10, i8 4, i32 1, i8 4, i32 0, i8 0, !M11}
!M10 = !{i32 0}
!M11 = !{i32 3, i32 4}
!M8 = !{!M12}
!M12 = !{i32 0, !"SV_Depth", i8 9, i8 17, !M10, i8 0, i32 1, i8 1, i32 -1, i8 -1, !M13}
!M13 = !{i32 3, i32 1}

