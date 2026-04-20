;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   x           0     NONE   float   x
; LOC                      1    y          0     NONE   float    y
; LOC                      2     zw        0     NONE   float     zw
; LOC                      3   xy          1     NONE   float   xy
; LOC                      4   xyz         2     NONE   float   xyz
; LOC                      5   xyz         3     NONE   float   xyz
; LOC                      6   xyzw        4     NONE   float   xyzw
; LOC                      7   xyzw        5     NONE   float   xyzw
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Target                0   x           0   TARGET   float   x
; SV_Target                1   x           1   TARGET   float   x
; SV_Target                2   xy          2   TARGET   float   xy
; SV_Target                3   xy          3   TARGET   float   xy
; SV_Target                4   xyz         4   TARGET   float   xyz
; SV_Target                5   xyz         5   TARGET   float   xyz
; SV_Target                6   xyzw        6   TARGET   float   xyzw
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
; EntryFunctionName: test_direct
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
;   output 0 depends on inputs: { 0 }
;   output 4 depends on inputs: { 1 }
;   output 8 depends on inputs: { 2 }
;   output 9 depends on inputs: { 3 }
;   output 12 depends on inputs: { 4 }
;   output 13 depends on inputs: { 5 }
;   output 16 depends on inputs: { 8 }
;   output 17 depends on inputs: { 9 }
;   output 18 depends on inputs: { 10 }
;   output 20 depends on inputs: { 12 }
;   output 21 depends on inputs: { 13 }
;   output 22 depends on inputs: { 14 }
;   output 24 depends on inputs: { 16 }
;   output 25 depends on inputs: { 17 }
;   output 26 depends on inputs: { 18 }
;   output 27 depends on inputs: { 19 }
;   output 28 depends on inputs: { 20 }
;   output 29 depends on inputs: { 21 }
;   output 30 depends on inputs: { 22 }
;   output 31 depends on inputs: { 23 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @test_direct() {
  %R0 = call float @dx.op.loadInput.f32(i32 4, i32 7, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = call float @dx.op.loadInput.f32(i32 4, i32 7, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = call float @dx.op.loadInput.f32(i32 4, i32 7, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R3 = call float @dx.op.loadInput.f32(i32 4, i32 7, i32 0, i8 3, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R4 = call float @dx.op.loadInput.f32(i32 4, i32 6, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R5 = call float @dx.op.loadInput.f32(i32 4, i32 6, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R6 = call float @dx.op.loadInput.f32(i32 4, i32 6, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R7 = call float @dx.op.loadInput.f32(i32 4, i32 6, i32 0, i8 3, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R8 = call float @dx.op.loadInput.f32(i32 4, i32 5, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R9 = call float @dx.op.loadInput.f32(i32 4, i32 5, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R10 = call float @dx.op.loadInput.f32(i32 4, i32 5, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R11 = call float @dx.op.loadInput.f32(i32 4, i32 4, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R12 = call float @dx.op.loadInput.f32(i32 4, i32 4, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R13 = call float @dx.op.loadInput.f32(i32 4, i32 4, i32 0, i8 2, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R14 = call float @dx.op.loadInput.f32(i32 4, i32 3, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R15 = call float @dx.op.loadInput.f32(i32 4, i32 3, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R16 = call float @dx.op.loadInput.f32(i32 4, i32 2, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R17 = call float @dx.op.loadInput.f32(i32 4, i32 2, i32 0, i8 1, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R18 = call float @dx.op.loadInput.f32(i32 4, i32 1, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R19 = call float @dx.op.loadInput.f32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R20 = fadd fast float %R19, 1.000000e+00
  %R21 = fadd fast float %R18, 1.000000e+00
  %R22 = fadd fast float %R16, 1.000000e+00
  %R23 = fadd fast float %R17, 1.000000e+00
  %R24 = fadd fast float %R14, 1.000000e+00
  %R25 = fadd fast float %R15, 1.000000e+00
  %R26 = fadd fast float %R11, 1.000000e+00
  %R27 = fadd fast float %R12, 1.000000e+00
  %R28 = fadd fast float %R13, 1.000000e+00
  %R29 = fadd fast float %R8, 1.000000e+00
  %R30 = fadd fast float %R9, 1.000000e+00
  %R31 = fadd fast float %R10, 1.000000e+00
  %R32 = fadd fast float %R4, 1.000000e+00
  %R33 = fadd fast float %R5, 1.000000e+00
  %R34 = fadd fast float %R6, 1.000000e+00
  %R35 = fadd fast float %R7, 1.000000e+00
  %R36 = fadd fast float %R0, 1.000000e+00
  %R37 = fadd fast float %R1, 1.000000e+00
  %R38 = fadd fast float %R2, 1.000000e+00
  %R39 = fadd fast float %R3, 1.000000e+00
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R20)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float %R21)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float %R22)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 1, float %R23)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 0, float %R24)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 3, i32 0, i8 1, float %R25)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 0, float %R26)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 1, float %R27)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 4, i32 0, i8 2, float %R28)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 0, float %R29)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 1, float %R30)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 5, i32 0, i8 2, float %R31)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 0, float %R32)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 1, float %R33)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 2, float %R34)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 6, i32 0, i8 3, float %R35)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 0, float %R36)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 1, float %R37)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 2, float %R38)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 7, i32 0, i8 3, float %R39)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
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
!M4 = !{[26 x i32] [i32 24, i32 32, i32 1, i32 16, i32 256, i32 512, i32 4096, i32 8192, i32 0, i32 0, i32 65536, i32 131072, i32 262144, i32 0, i32 1048576, i32 2097152, i32 4194304, i32 0, i32 16777216, i32 33554432, i32 67108864, i32 134217728, i32 268435456, i32 536870912, i32 1073741824, i32 -2147483648]}
!M5 = !{void ()* @test_direct, !"test_direct", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9, !M10, !M11, !M12, !M13, !M14, !M15, !M16}
!M9 = !{i32 0, !"LOC", i8 9, i8 0, !M17, i8 2, i32 1, i8 1, i32 0, i8 0, !M18}
!M17 = !{i32 0}
!M18 = !{i32 3, i32 1}
!M10 = !{i32 1, !"LOC", i8 9, i8 0, !M19, i8 2, i32 1, i8 1, i32 0, i8 1, !M18}
!M19 = !{i32 1}
!M11 = !{i32 2, !"LOC", i8 9, i8 0, !M20, i8 2, i32 1, i8 2, i32 0, i8 2, !M21}
!M20 = !{i32 2}
!M21 = !{i32 3, i32 3}
!M12 = !{i32 3, !"LOC", i8 9, i8 0, !M22, i8 2, i32 1, i8 2, i32 1, i8 0, !M21}
!M22 = !{i32 3}
!M13 = !{i32 4, !"LOC", i8 9, i8 0, !M23, i8 2, i32 1, i8 3, i32 2, i8 0, !M24}
!M23 = !{i32 4}
!M24 = !{i32 3, i32 7}
!M14 = !{i32 5, !"LOC", i8 9, i8 0, !M25, i8 2, i32 1, i8 3, i32 3, i8 0, !M24}
!M25 = !{i32 5}
!M15 = !{i32 6, !"LOC", i8 9, i8 0, !M26, i8 2, i32 1, i8 4, i32 4, i8 0, !M27}
!M26 = !{i32 6}
!M27 = !{i32 3, i32 15}
!M16 = !{i32 7, !"LOC", i8 9, i8 0, !M28, i8 2, i32 1, i8 4, i32 5, i8 0, !M27}
!M28 = !{i32 7}
!M8 = !{!M29, !M30, !M31, !M32, !M33, !M34, !M35, !M36}
!M29 = !{i32 0, !"SV_Target", i8 9, i8 16, !M17, i8 0, i32 1, i8 1, i32 0, i8 0, !M18}
!M30 = !{i32 1, !"SV_Target", i8 9, i8 16, !M19, i8 0, i32 1, i8 1, i32 1, i8 0, !M18}
!M31 = !{i32 2, !"SV_Target", i8 9, i8 16, !M20, i8 0, i32 1, i8 2, i32 2, i8 0, !M21}
!M32 = !{i32 3, !"SV_Target", i8 9, i8 16, !M22, i8 0, i32 1, i8 2, i32 3, i8 0, !M21}
!M33 = !{i32 4, !"SV_Target", i8 9, i8 16, !M23, i8 0, i32 1, i8 3, i32 4, i8 0, !M24}
!M34 = !{i32 5, !"SV_Target", i8 9, i8 16, !M25, i8 0, i32 1, i8 3, i32 5, i8 0, !M24}
!M35 = !{i32 6, !"SV_Target", i8 9, i8 16, !M26, i8 0, i32 1, i8 4, i32 6, i8 0, !M27}
!M36 = !{i32 7, !"SV_Target", i8 9, i8 16, !M28, i8 0, i32 1, i8 4, i32 7, i8 0, !M27}

