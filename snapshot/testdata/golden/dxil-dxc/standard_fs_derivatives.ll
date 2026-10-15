;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Position              0   xyzw        0      POS   float   xyzw
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
; EntryFunctionName: derivatives
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

define void @derivatives() {
  %R0 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 3, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call float @dx.op.unary.f32(i32 83, float %R0)  ; DerivCoarseX(value)
  %R5 = call float @dx.op.unary.f32(i32 83, float %R1)  ; DerivCoarseX(value)
  %R6 = call float @dx.op.unary.f32(i32 83, float %R2)  ; DerivCoarseX(value)
  %R7 = call float @dx.op.unary.f32(i32 83, float %R3)  ; DerivCoarseX(value)
  %R8 = call float @dx.op.unary.f32(i32 84, float %R0)  ; DerivCoarseY(value)
  %R9 = call float @dx.op.unary.f32(i32 84, float %R1)  ; DerivCoarseY(value)
  %R10 = call float @dx.op.unary.f32(i32 84, float %R2)  ; DerivCoarseY(value)
  %R11 = call float @dx.op.unary.f32(i32 84, float %R3)  ; DerivCoarseY(value)
  %R12 = call float @dx.op.unary.f32(i32 6, float %R4)  ; FAbs(value)
  %R13 = call float @dx.op.unary.f32(i32 6, float %R5)  ; FAbs(value)
  %R14 = call float @dx.op.unary.f32(i32 6, float %R6)  ; FAbs(value)
  %R15 = call float @dx.op.unary.f32(i32 6, float %R7)  ; FAbs(value)
  %R16 = call float @dx.op.unary.f32(i32 6, float %R8)  ; FAbs(value)
  %R17 = call float @dx.op.unary.f32(i32 6, float %R9)  ; FAbs(value)
  %R18 = call float @dx.op.unary.f32(i32 6, float %R10)  ; FAbs(value)
  %R19 = call float @dx.op.unary.f32(i32 6, float %R11)  ; FAbs(value)
  %R20 = fadd fast float %R16, %R12
  %R21 = fadd fast float %R17, %R13
  %R22 = fadd fast float %R18, %R14
  %R23 = fadd fast float %R19, %R15
  %R24 = fadd fast float %R8, %R4
  %R25 = fadd fast float %R9, %R5
  %R26 = fadd fast float %R10, %R6
  %R27 = fadd fast float %R11, %R7
  %R28 = fmul fast float %R20, %R24
  %R29 = fmul fast float %R21, %R25
  %R30 = fmul fast float %R22, %R26
  %R31 = fmul fast float %R23, %R27
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R28)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R29)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R30)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R31)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.loadInput.f32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A1

; Function Attrs: nounwind readnone
declare float @dx.op.unary.f32(i32, float) #A0

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
!M4 = !{[6 x i32] [i32 4, i32 4, i32 1, i32 2, i32 4, i32 8]}
!M5 = !{void ()* @derivatives, !"derivatives", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9}
!M9 = !{i32 0, !"SV_Position", i8 9, i8 3, !M10, i8 4, i32 1, i8 4, i32 0, i8 0, !M11}
!M10 = !{i32 0}
!M11 = !{i32 3, i32 15}
!M8 = !{!M12}
!M12 = !{i32 0, !"SV_Target", i8 9, i8 16, !M10, i8 0, i32 1, i8 4, i32 0, i8 0, !M11}

