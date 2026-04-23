;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_VertexID              0   x           0   VERTID    uint   x
;
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
; SigInputElements: 1
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 1
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vs_main
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_VertexID              0
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
;
;
; ViewId state:
;
; Number of inputs: 1, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0 }
;   output 1 depends on inputs: { 0 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

@ret.i.0.hca = internal unnamed_addr constant [3 x float] [float 0.000000e+00, float -5.000000e-01, float 5.000000e-01]
@ret.i.1.hca = internal unnamed_addr constant [3 x float] [float 5.000000e-01, float -5.000000e-01, float -5.000000e-01]

define void @vs_main() {
  %R0 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = call i32 @dx.op.binary.i32(i32 40, i32 %R0, i32 2)  ; UMin(a,b)
  %R2 = getelementptr [3 x float], [3 x float]* @ret.i.0.hca, i32 0, i32 %R1
  %R3 = getelementptr [3 x float], [3 x float]* @ret.i.1.hca, i32 0, i32 %R1
  %R4 = load float, float* %R2, align 4
  %R5 = load float, float* %R3, align 4
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R4)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R5)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.binary.i32(i32, i32, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

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
!M2 = !{i32 1, i32 8}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{[3 x i32] [i32 1, i32 4, i32 3]}
!M5 = !{void ()* @vs_main, !"vs_main", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9}
!M9 = !{i32 0, !"SV_VertexID", i8 5, i8 1, !M10, i8 0, i32 1, i8 1, i32 0, i8 0, !M11}
!M10 = !{i32 0}
!M11 = !{i32 3, i32 1}
!M8 = !{!M12}
!M12 = !{i32 0, !"SV_Position", i8 9, i8 3, !M10, i8 4, i32 1, i8 4, i32 0, i8 0, !M13}
!M13 = !{i32 3, i32 15}

