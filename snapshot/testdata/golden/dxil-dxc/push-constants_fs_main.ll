;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyzw        0     NONE   float   xyzw
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Target                0   xyzw        0   TARGET   float   xyzw
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
; SigInputElements: 1
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 1
; SigOutputVectors[0]: 1
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
; LOC                      0                 linear
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
; im                                cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 4, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0 }
;   output 1 depends on inputs: { 1 }
;   output 2 depends on inputs: { 2 }
;   output 3 depends on inputs: { 3 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.f32 = type { float, float, float, float }
%im = type { %struct.S0 }
%struct.S0 = type { float }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 3, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R5 = call %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32 59, %dx.types.Handle %R0, i32 0)  ; CBufferLoadLegacy(handle,regIndex)
  %R6 = extractvalue %dx.types.CBufRet.f32 %R5, 0
  %R7 = fmul fast float %R6, %R1
  %R8 = fmul fast float %R6, %R2
  %R9 = fmul fast float %R6, %R3
  %R10 = fmul fast float %R6, %R4
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R7)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R8)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R9)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R10)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.f32 @dx.op.cbufferLoadLegacy.f32(i32, %dx.types.Handle, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A0

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A2

attributes #A0 = { nounwind readonly }
attributes #A1 = { nounwind readnone }
attributes #A2 = { nounwind }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.viewIdState = !{!M5}
!dx.entryPoints = !{!M6}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{null, null, !M7, null}
!M7 = !{!M8}
!M8 = !{i32 0, %im* undef, !"", i32 0, i32 0, i32 1, i32 4, null}
!M5 = !{[6 x i32] [i32 4, i32 4, i32 1, i32 2, i32 4, i32 8]}
!M6 = !{void ()* @main, !"main", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12}
!M12 = !{i32 0, !"LOC", i8 9, i8 0, !M13, i8 2, i32 1, i8 4, i32 0, i8 0, !M14}
!M13 = !{i32 0}
!M14 = !{i32 3, i32 15}
!M11 = !{!M15}
!M15 = !{i32 0, !"SV_Target", i8 9, i8 16, !M13, i8 0, i32 1, i8 4, i32 0, i8 0, !M14}

