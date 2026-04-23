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
; LOC                      0   x           0     NONE    uint   x
; LOC                      1   xy          1     NONE   float   xy
; SV_Position              0   xyzw        2      POS   float   xyzw
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
; SigOutputElements: 3
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 1
; SigOutputVectors[0]: 3
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: gen_terrain_vertex
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
; LOC                      0        nointerpolation
; LOC                      1                 linear
; SV_Position              0          noperspective
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; gen_data                          cbuffer      NA          NA     CB0            cb0     1
;
;
; ViewId state:
;
; Number of inputs: 1, outputs: 12
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0 }
;   output 4 depends on inputs: { 0 }
;   output 5 depends on inputs: { 0 }
;   output 8 depends on inputs: { 0 }
;   output 9 depends on inputs: { 0 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.CBufRet.i32 = type { i32, i32, i32, i32 }
%gen_data = type { %struct.S0 }
%struct.S0 = type { <2 x i32>, <2 x i32>, <2 x float>, i32, i32 }

define void @gen_terrain_vertex() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 2, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R2 = add i32 %R1, 2
  %R3 = udiv i32 %R2, 3
  %R4 = and i32 %R3, 1
  %R5 = uitofp i32 %R4 to float
  %R6 = add i32 %R1, 1
  %R7 = udiv i32 %R6, 3
  %R8 = and i32 %R7, 1
  %R9 = uitofp i32 %R8 to float
  %R10 = fmul fast float %R5, 2.000000e+00
  %R11 = fmul fast float %R9, 2.000000e+00
  %R12 = fadd fast float %R10, -1.000000e+00
  %R13 = fadd fast float %R11, -1.000000e+00
  %R14 = call %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32 59, %dx.types.Handle %R0, i32 1)  ; CBufferLoadLegacy(handle,regIndex)
  %R15 = extractvalue %dx.types.CBufRet.i32 %R14, 2
  %R16 = extractvalue %dx.types.CBufRet.i32 %R14, 3
  %R17 = uitofp i32 %R15 to float
  %R18 = fadd fast float %R9, %R5
  %R19 = fmul fast float %R18, %R17
  %R20 = call float @dx.op.binary.f32(i32 35, float %R19, float 0.000000e+00)  ; FMax(a,b)
  %R21 = call float @dx.op.binary.f32(i32 36, float %R20, float 0x41EFFFFFE0000000)  ; FMin(a,b)
  %R22 = fptoui float %R21 to i32
  %R23 = add i32 %R22, %R16
  call void @dx.op.storeOutput.i32(i32 5, i32 0, i32 0, i8 0, i32 %R23)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 0, float %R5)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 1, i32 0, i8 1, float %R9)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 0, float %R12)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 1, float %R13)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 2, float 0.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 2, i32 0, i8 3, float 1.000000e+00)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare float @dx.op.binary.f32(i32, float, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.CBufRet.i32 @dx.op.cbufferLoadLegacy.i32(i32, %dx.types.Handle, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.storeOutput.f32(i32, i32, i32, i8, float) #A2

; Function Attrs: nounwind
declare void @dx.op.storeOutput.i32(i32, i32, i32, i8, i32) #A2

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind readonly }
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
!M2 = !{i32 1, i32 8}
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{null, null, !M7, null}
!M7 = !{!M8}
!M8 = !{i32 0, %gen_data* undef, !"", i32 0, i32 0, i32 1, i32 32, null}
!M5 = !{[3 x i32] [i32 1, i32 12, i32 817]}
!M6 = !{void ()* @gen_terrain_vertex, !"gen_terrain_vertex", !M9, !M4, null}
!M9 = !{!M10, !M11, null}
!M10 = !{!M12}
!M12 = !{i32 0, !"SV_VertexID", i8 5, i8 1, !M13, i8 0, i32 1, i8 1, i32 0, i8 0, !M14}
!M13 = !{i32 0}
!M14 = !{i32 3, i32 1}
!M11 = !{!M15, !M16, !M17}
!M15 = !{i32 0, !"LOC", i8 5, i8 0, !M13, i8 1, i32 1, i8 1, i32 0, i8 0, !M14}
!M16 = !{i32 1, !"LOC", i8 9, i8 0, !M18, i8 2, i32 1, i8 2, i32 1, i8 0, !M19}
!M18 = !{i32 1}
!M19 = !{i32 3, i32 3}
!M17 = !{i32 2, !"SV_Position", i8 9, i8 3, !M13, i8 4, i32 1, i8 4, i32 2, i8 0, !M20}
!M20 = !{i32 3, i32 15}

