;
; Note: shader requires additional functionality:
;       Use native low precision
;
;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   x           0     NONE    fp16
; LOC                      2    yz         0     NONE    fp16    yz
; LOC                      1   x           1     NONE   float
; LOC                      3    yz         1     NONE   float
; LOC                      4   xyz         2     NONE    fp16
; LOC                      5   xyz         3     NONE   float
; LOC                      6   xyzw        4     NONE    fp16
; LOC                      7   xyzw        5     NONE   float
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Target                0   x           0   TARGET    fp16   x
; SV_Target                1   x           1   TARGET   float   x
; SV_Target                2   xy          2   TARGET    fp16   xy
; SV_Target                3   xy          3   TARGET   float   xy
; SV_Target                4   xyz         4   TARGET    fp16   xyz
; SV_Target                5   xyz         5   TARGET   float   xyz
; SV_Target                6   xyzw        6   TARGET    fp16   xyzw
; SV_Target                7   xyzw        7   TARGET   float   xyzw
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
; SigOutputElements: 8
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 6
; SigOutputVectors[0]: 8
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: test_component_access
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
; SV_Target                1
; SV_Target                2
; SV_Target                3
; SV_Target                4
; SV_Target                5
; SV_Target                6
; SV_Target                7
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
; Number of inputs: 24, outputs: 32
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 8 depends on inputs: { 2 }
;   output 9 depends on inputs: { 1 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:8-i16:16-i32:32-i64:64-f16:16-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @test_component_access() {
  %R0 = call half @dx.op.loadInput.f16(i32 4, i32 2, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = call half @dx.op.loadInput.f16(i32 4, i32 2, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  call void @dx.op.storeOutput.f16(i32 5, i32 0, i32 0, i8 0, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 2, i32 0, i8 0, half %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 2, i32 0, i8 1, half %R0)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 1, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 4, i32 0, i8 0, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 4, i32 0, i8 1, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 4, i32 0, i8 2, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 1, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 2, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 6, i32 0, i8 0, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 6, i32 0, i8 1, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 6, i32 0, i8 2, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f16(i32 5, i32 6, i32 0, i8 3, half 0xH0000)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 0, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 1, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 2, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 3, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare half @dx.op.loadInput.f16(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f16(i32, i32, i32, i8, half) #A1

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
!M4 = !{[26 x i32] [i32 24, i32 32, i32 0, i32 512, i32 256, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0]}
!M5 = !{void ()* @test_component_access, !"test_component_access", !M6, null, !M7}
!M6 = !{!M8, !M9, null}
!M8 = !{!M10, !M11, !M12, !M13, !M14, !M15, !M16, !M17}
!M10 = !{i32 0, !"LOC", i8 8, i8 0, !M18, i8 2, i32 1, i8 1, i32 0, i8 0, null}
!M18 = !{i32 0}
!M11 = !{i32 1, !"LOC", i8 9, i8 0, !M19, i8 2, i32 1, i8 1, i32 1, i8 0, null}
!M19 = !{i32 1}
!M12 = !{i32 2, !"LOC", i8 8, i8 0, !M20, i8 2, i32 1, i8 2, i32 0, i8 1, !M21}
!M20 = !{i32 2}
!M21 = !{i32 3, i32 3}
!M13 = !{i32 3, !"LOC", i8 9, i8 0, !M22, i8 2, i32 1, i8 2, i32 1, i8 1, null}
!M22 = !{i32 3}
!M14 = !{i32 4, !"LOC", i8 8, i8 0, !M23, i8 2, i32 1, i8 3, i32 2, i8 0, null}
!M23 = !{i32 4}
!M15 = !{i32 5, !"LOC", i8 9, i8 0, !M24, i8 2, i32 1, i8 3, i32 3, i8 0, null}
!M24 = !{i32 5}
!M16 = !{i32 6, !"LOC", i8 8, i8 0, !M25, i8 2, i32 1, i8 4, i32 4, i8 0, null}
!M25 = !{i32 6}
!M17 = !{i32 7, !"LOC", i8 9, i8 0, !M26, i8 2, i32 1, i8 4, i32 5, i8 0, null}
!M26 = !{i32 7}
!M9 = !{!M27, !M28, !M29, !M30, !M31, !M32, !M33, !M34}
!M27 = !{i32 0, !"SV_Target", i8 8, i8 16, !M18, i8 0, i32 1, i8 1, i32 0, i8 0, !M35}
!M35 = !{i32 3, i32 1}
!M28 = !{i32 1, !"SV_Target", i8 9, i8 16, !M19, i8 0, i32 1, i8 1, i32 1, i8 0, !M35}
!M29 = !{i32 2, !"SV_Target", i8 8, i8 16, !M20, i8 0, i32 1, i8 2, i32 2, i8 0, !M21}
!M30 = !{i32 3, !"SV_Target", i8 9, i8 16, !M22, i8 0, i32 1, i8 2, i32 3, i8 0, !M21}
!M31 = !{i32 4, !"SV_Target", i8 8, i8 16, !M23, i8 0, i32 1, i8 3, i32 4, i8 0, !M36}
!M36 = !{i32 3, i32 7}
!M32 = !{i32 5, !"SV_Target", i8 9, i8 16, !M24, i8 0, i32 1, i8 3, i32 5, i8 0, !M36}
!M33 = !{i32 6, !"SV_Target", i8 8, i8 16, !M25, i8 0, i32 1, i8 4, i32 6, i8 0, !M37}
!M37 = !{i32 3, i32 15}
!M34 = !{i32 7, !"SV_Target", i8 9, i8 16, !M26, i8 0, i32 1, i8 4, i32 7, i8 0, !M37}
!M7 = !{i32 0, i64 8388640}

