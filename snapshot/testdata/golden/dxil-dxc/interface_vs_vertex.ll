;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_VertexID              0   x           0   VERTID    uint   x
; SV_InstanceID            0   x           1   INSTID    uint   x
; LOC                     10   x           2     NONE    uint   x
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      1   x           0     NONE   float   x
; SV_Position              0   xyzw        1      POS   float   xyzw
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
; SigInputElements: 3
; SigOutputElements: 2
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 3
; SigOutputVectors[0]: 2
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vertex
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_VertexID              0
; SV_InstanceID            0
; LOC                     10
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      1                 linear
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
; Number of inputs: 9, outputs: 8
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0, 4, 8 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @vertex() {
  %R0 = call i32 @dx.op.loadInput.i32(i32 4, i32 2, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = call i32 @dx.op.loadInput.i32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = add i32 %R1, %R0
  %R4 = add i32 %R3, %R2
  %R5 = uitofp i32 %R4 to float
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R5)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 1, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 2, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

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
!M2 = !{i32 1, i32 0}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{[11 x i32] [i32 9, i32 8, i32 1, i32 0, i32 0, i32 0, i32 1, i32 0, i32 0, i32 0, i32 1]}
!M5 = !{void ()* @vertex, !"vertex", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9, !M10, !M11}
!M9 = !{i32 0, !"SV_VertexID", i8 5, i8 1, !M12, i8 0, i32 1, i8 1, i32 0, i8 0, !M13}
!M12 = !{i32 0}
!M13 = !{i32 3, i32 1}
!M10 = !{i32 1, !"SV_InstanceID", i8 5, i8 2, !M12, i8 0, i32 1, i8 1, i32 1, i8 0, !M13}
!M11 = !{i32 2, !"LOC", i8 5, i8 0, !M14, i8 0, i32 1, i8 1, i32 2, i8 0, !M13}
!M14 = !{i32 10}
!M8 = !{!M15, !M16}
!M15 = !{i32 0, !"LOC", i8 9, i8 0, !M17, i8 2, i32 1, i8 1, i32 0, i8 0, !M13}
!M17 = !{i32 1}
!M16 = !{i32 1, !"SV_Position", i8 9, i8 3, !M12, i8 4, i32 1, i8 4, i32 1, i8 0, !M18}
!M18 = !{i32 3, i32 15}

