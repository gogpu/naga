;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xyz         0     NONE   float   xyz
; SV_Position              0   xyzw        1      POS   float
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
; SigInputElements: 2
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 2
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: fs_main
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0                 linear
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
; Number of inputs: 8, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0 }
;   output 1 depends on inputs: { 1 }
;   output 2 depends on inputs: { 2 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @fs_main() {
  %R0 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  br label %R3

; <label>:4                                       ; preds = %R4, %R5
  %R6 = phi i32 [ %R7, %R4 ], [ 0, %R5 ]
  %R8 = phi float [ %R9, %R4 ], [ %R0, %R5 ]
  %R10 = phi float [ %R11, %R4 ], [ %R1, %R5 ]
  %R12 = phi i32 [ %R13, %R4 ], [ -1, %R5 ]
  %R14 = phi i32 [ %R15, %R4 ], [ -1, %R5 ]
  %R16 = phi i32 [ 1, %R4 ], [ 0, %R5 ]
  %R17 = icmp eq i32 %R14, 0
  %R18 = zext i1 %R17 to i32
  %R15 = add i32 %R14, -1
  %R7 = add i32 %R16, %R6
  %R19 = icmp slt i32 %R7, 10
  br i1 %R19, label %R4, label %R20

; <label>:16                                      ; preds = %R3
  %R13 = sub i32 %R12, %R18
  %R21 = sitofp i32 %R7 to float
  %R22 = fmul fast float %R21, 0x3F50624DE0000000
  %R9 = fadd fast float %R22, %R8
  %R23 = fmul fast float %R21, 0x3F60624DE0000000
  %R11 = fadd fast float %R23, %R10
  %R24 = icmp eq i32 %R12, %R18
  %R25 = icmp eq i32 %R15, 0
  %R26 = and i1 %R25, %R24
  br i1 %R26, label %R20, label %R3

; <label>:26                                      ; preds = %R4, %R3
  %R27 = phi float [ %R9, %R4 ], [ %R8, %R3 ]
  %R28 = phi float [ %R11, %R4 ], [ %R10, %R3 ]
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R27)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R28)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R2)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
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
!M2 = !{i32 1, i32 8}
!M3 = !{!"ps", i32 6, i32 0}
!M4 = !{[10 x i32] [i32 8, i32 4, i32 1, i32 2, i32 4, i32 0, i32 0, i32 0, i32 0, i32 0]}
!M5 = !{void ()* @fs_main, !"fs_main", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9, !M10}
!M9 = !{i32 0, !"LOC", i8 9, i8 0, !M11, i8 2, i32 1, i8 3, i32 0, i8 0, !M12}
!M11 = !{i32 0}
!M12 = !{i32 3, i32 7}
!M10 = !{i32 1, !"SV_Position", i8 9, i8 3, !M11, i8 4, i32 1, i8 4, i32 1, i8 0, null}
!M8 = !{!M13}
!M13 = !{i32 0, !"SV_Target", i8 9, i8 16, !M11, i8 0, i32 1, i8 4, i32 0, i8 0, !M14}
!M14 = !{i32 3, i32 15}

