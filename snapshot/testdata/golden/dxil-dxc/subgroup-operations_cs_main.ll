;
; Note: shader requires additional functionality:
;       Wave level operations
;
;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; no parameters
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; no parameters
; shader hash: <stripped>
;
; Pipeline Runtime Information:
;
;PSVRuntimeInfo:
; Compute Shader
; NumThreads=(1,1,1)
; MinimumExpectedWaveLaneCount: 0
; MaximumExpectedWaveLaneCount: 4294967295
; UsesViewID: false
; SigInputElements: 0
; SigOutputElements: 0
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 0
; SigOutputVectors[0]: 0
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: main
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.fouri32 = type { i32, i32, i32, i32 }

define void @main() {
  %R0 = call i32 @dx.op.waveGetLaneCount(i32 112)  ; WaveGetLaneCount()
  %R1 = call i32 @dx.op.waveGetLaneIndex(i32 111)  ; WaveGetLaneIndex()
  %R2 = and i32 %R1, 1
  %R3 = icmp ne i32 %R2, 0
  %R4 = call %dx.types.fouri32 @dx.op.waveActiveBallot(i32 116, i1 %R3)  ; WaveActiveBallot(cond)
  %R5 = call %dx.types.fouri32 @dx.op.waveActiveBallot(i32 116, i1 true)  ; WaveActiveBallot(cond)
  %R6 = icmp ne i32 %R1, 0
  %R7 = call i1 @dx.op.waveAllTrue(i32 114, i1 %R6)  ; WaveAllTrue(cond)
  %R8 = icmp eq i32 %R1, 0
  %R9 = call i1 @dx.op.waveAnyTrue(i32 113, i1 %R8)  ; WaveAnyTrue(cond)
  %R10 = call i32 @dx.op.waveActiveOp.i32(i32 119, i32 %R1, i8 0, i8 1)  ; WaveActiveOp(value,op,sop)
  %R11 = call i32 @dx.op.waveActiveOp.i32(i32 119, i32 %R1, i8 1, i8 1)  ; WaveActiveOp(value,op,sop)
  %R12 = call i32 @dx.op.waveActiveOp.i32(i32 119, i32 %R1, i8 2, i8 1)  ; WaveActiveOp(value,op,sop)
  %R13 = call i32 @dx.op.waveActiveOp.i32(i32 119, i32 %R1, i8 3, i8 1)  ; WaveActiveOp(value,op,sop)
  %R14 = call i32 @dx.op.waveActiveBit.i32(i32 120, i32 %R1, i8 0)  ; WaveActiveBit(value,op)
  %R15 = call i32 @dx.op.waveActiveBit.i32(i32 120, i32 %R1, i8 1)  ; WaveActiveBit(value,op)
  %R16 = call i32 @dx.op.waveActiveBit.i32(i32 120, i32 %R1, i8 2)  ; WaveActiveBit(value,op)
  %R17 = call i32 @dx.op.wavePrefixOp.i32(i32 121, i32 %R1, i8 0, i8 1)  ; WavePrefixOp(value,op,sop)
  %R18 = call i32 @dx.op.wavePrefixOp.i32(i32 121, i32 %R1, i8 1, i8 1)  ; WavePrefixOp(value,op,sop)
  %R19 = call i32 @dx.op.wavePrefixOp.i32(i32 121, i32 %R1, i8 0, i8 1)  ; WavePrefixOp(value,op,sop)
  %R20 = call i32 @dx.op.wavePrefixOp.i32(i32 121, i32 %R1, i8 1, i8 1)  ; WavePrefixOp(value,op,sop)
  %R21 = call i32 @dx.op.waveReadLaneFirst.i32(i32 118, i32 %R1)  ; WaveReadLaneFirst(value)
  %R22 = call i32 @dx.op.waveReadLaneAt.i32(i32 117, i32 %R1, i32 4)  ; WaveReadLaneAt(value,lane)
  %R23 = add i32 %R0, -1
  %R24 = sub i32 %R23, %R1
  %R25 = call i32 @dx.op.waveReadLaneAt.i32(i32 117, i32 %R1, i32 %R24)  ; WaveReadLaneAt(value,lane)
  %R26 = call i32 @dx.op.waveGetLaneIndex(i32 111)  ; WaveGetLaneIndex()
  %R27 = add i32 %R26, 1
  %R28 = call i32 @dx.op.waveReadLaneAt.i32(i32 117, i32 %R1, i32 %R27)  ; WaveReadLaneAt(value,lane)
  %R29 = call i32 @dx.op.waveGetLaneIndex(i32 111)  ; WaveGetLaneIndex()
  %R30 = add i32 %R29, -1
  %R31 = call i32 @dx.op.waveReadLaneAt.i32(i32 117, i32 %R1, i32 %R30)  ; WaveReadLaneAt(value,lane)
  %R32 = call i32 @dx.op.waveGetLaneIndex(i32 111)  ; WaveGetLaneIndex()
  %R33 = xor i32 %R32, %R23
  %R34 = call i32 @dx.op.waveReadLaneAt.i32(i32 117, i32 %R1, i32 %R33)  ; WaveReadLaneAt(value,lane)
  %R35 = call i32 @dx.op.quadReadLaneAt.i32(i32 122, i32 %R1, i32 4)  ; QuadReadLaneAt(value,quadLane)
  %R36 = call i32 @dx.op.quadOp.i32(i32 123, i32 %R1, i8 0)  ; QuadOp(value,op)
  %R37 = call i32 @dx.op.quadOp.i32(i32 123, i32 %R1, i8 1)  ; QuadOp(value,op)
  %R38 = call i32 @dx.op.quadOp.i32(i32 123, i32 %R1, i8 2)  ; QuadOp(value,op)
  ret void
}

; Function Attrs: nounwind
declare i32 @dx.op.quadOp.i32(i32, i32, i8) #A0

; Function Attrs: nounwind
declare i32 @dx.op.quadReadLaneAt.i32(i32, i32, i32) #A0

; Function Attrs: nounwind
declare %dx.types.fouri32 @dx.op.waveActiveBallot(i32, i1) #A0

; Function Attrs: nounwind
declare i32 @dx.op.waveActiveBit.i32(i32, i32, i8) #A0

; Function Attrs: nounwind
declare i32 @dx.op.waveActiveOp.i32(i32, i32, i8, i8) #A0

; Function Attrs: nounwind
declare i1 @dx.op.waveAllTrue(i32, i1) #A0

; Function Attrs: nounwind
declare i1 @dx.op.waveAnyTrue(i32, i1) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.waveGetLaneCount(i32) #A1

; Function Attrs: nounwind readonly
declare i32 @dx.op.waveGetLaneIndex(i32) #A2

; Function Attrs: nounwind
declare i32 @dx.op.wavePrefixOp.i32(i32, i32, i8, i8) #A0

; Function Attrs: nounwind
declare i32 @dx.op.waveReadLaneAt.i32(i32, i32, i32) #A0

; Function Attrs: nounwind
declare i32 @dx.op.waveReadLaneFirst.i32(i32, i32) #A0

attributes #A0 = { nounwind }
attributes #A1 = { nounwind readnone }
attributes #A2 = { nounwind readonly }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.entryPoints = !{!M4}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{void ()* @main, !"main", null, null, !M5}
!M5 = !{i32 0, i64 524288, i32 4, !M6}
!M6 = !{i32 1, i32 1, i32 1}

