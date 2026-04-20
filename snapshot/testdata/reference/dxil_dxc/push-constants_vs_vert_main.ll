;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xy          0     NONE   float   xy
; SV_InstanceID            0   x           1   INSTID    uint   x
; SV_VertexID              0   x           2   VERTID    uint   x
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
; SigInputElements: 3
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 3
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: vert_main
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0
; SV_InstanceID            0
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
; im                                cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 9, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0, 4, 8 }
;   output 1 depends on inputs: { 1, 4, 8 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%im = type { %struct.S0 }
%struct.S0 = type { float }

define void @vert_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.loadInput.i32(i32 4, i32 2, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call i32 @dx.op.loadInput.i32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R5 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R6 = extractvalue %dx.types.CBufRet.f32 %R5, 0
  %R7 = uitofp i32 %R2 to float
  %R8 = uitofp i32 %R1 to float
  %R9 = fmul fast float %R7, %R8
  %R10 = fmul fast float %R9, %R6
  %R11 = fmul fast float %R10, %R3
  %R12 = fmul fast float %R10, %R4
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R11)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R12)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A2

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }
attributes #A2 = { nounwind readonly }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.viewIdState = !{!M5}
!dx.entryPoints = !{!M6}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 8}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{null, null, !M7, null}
!M7 = !{!M8}
!M8 = !{i32 0, %im* undef, !"", i32 0, i32 0, i32 1, i32 4, null}
!M5 = !{[11 x i32] [i32 9, i32 4, i32 1, i32 2, i32 0, i32 0, i32 3, i32 0, i32 0, i32 0, i32 3]}
!M6 = !{void ()* @vert_main, !"vert_main", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12, !M13, !M14}
!M12 = !{i32 0, !"LOC", i8 9, i8 0, !M15, i8 0, i32 1, i8 2, i32 0, i8 0, !M16}
!M15 = !{i32 0}
!M16 = !{i32 3, i32 3}
!M13 = !{i32 1, !"SV_InstanceID", i8 5, i8 2, !M15, i8 0, i32 1, i8 1, i32 1, i8 0, !M17}
!M17 = !{i32 3, i32 1}
!M14 = !{i32 2, !"SV_VertexID", i8 5, i8 1, !M15, i8 0, i32 1, i8 1, i32 2, i8 0, !M17}
!M11 = !{!M18}
!M18 = !{i32 0, !"SV_Position", i8 9, i8 3, !M15, i8 4, i32 1, i8 4, i32 0, i8 0, !M19}
!M19 = !{i32 3, i32 15}

