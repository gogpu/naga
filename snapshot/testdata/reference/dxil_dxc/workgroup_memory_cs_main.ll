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
; NumThreads=(256,1,1)
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
; output                                UAV    byte         r/w      U0             u1     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%struct.S0 = type { i32 }

@"\01?shared_data@@3PAMA" = external addrspace(3) global [256 x float], align 4

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.flattenedThreadIdInGroup.i32(i32 96)  ; FlattenedThreadIdInGroup()
  %R2 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R3 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R4 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R5 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R6 = or i32 %R4, %R3
  %R7 = or i32 %R6, %R5
  %R8 = icmp eq i32 %R7, 0
  br i1 %R8, label %R9, label %R10

; <label>:10                                      ; preds = %R11
  br label %R12

; <label>:11                                      ; preds = %R12, %R9
  %R13 = phi i32 [ %R14, %R12 ], [ 0, %R9 ]
  %R15 = getelementptr [256 x float], [256 x float] addrspace(3)* @"\01?shared_data@@3PAMA", i32 0, i32 %R13
  store float 0.000000e+00, float addrspace(3)* %R15, align 4
  %R14 = add i32 %R13, 1
  %R16 = icmp eq i32 %R14, 256
  br i1 %R16, label %R17, label %R12

; <label>:16                                      ; preds = %R12
  br label %R10

; <label>:17                                      ; preds = %R17, %R11
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R18 = uitofp i32 %R2 to float
  %R19 = fmul fast float %R18, 5.000000e-01
  %R20 = call i32 @dx.op.binary.i32(i32 40, i32 %R1, i32 255)  ; UMin(a,b)
  %R21 = getelementptr [256 x float], [256 x float] addrspace(3)* @"\01?shared_data@@3PAMA", i32 0, i32 %R20
  store float %R19, float addrspace(3)* %R21, align 4
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R22 = icmp ult i32 %R1, 128
  br i1 %R22, label %R23, label %R24

; <label>:23                                      ; preds = %R10
  %R25 = load float, float addrspace(3)* %R21, align 4
  %R26 = add i32 %R1, 128
  %R27 = call i32 @dx.op.binary.i32(i32 40, i32 %R26, i32 255)  ; UMin(a,b)
  %R28 = getelementptr [256 x float], [256 x float] addrspace(3)* @"\01?shared_data@@3PAMA", i32 0, i32 %R27
  %R29 = load float, float addrspace(3)* %R28, align 4
  %R30 = fadd fast float %R29, %R25
  store float %R30, float addrspace(3)* %R21, align 4
  br label %R24

; <label>:30                                      ; preds = %R23, %R10
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R31 = icmp eq i32 %R1, 0
  br i1 %R31, label %R32, label %R33

; <label>:32                                      ; preds = %R24
  %R34 = load float, float addrspace(3)* getelementptr inbounds ([256 x float], [256 x float] addrspace(3)* @"\01?shared_data@@3PAMA", i32 0, i32 0), align 4
  %R35 = bitcast float %R34 to i32
  %R36 = lshr i32 %R2, 8
  %R37 = shl i32 %R36, 2
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R37, i32 undef, i32 %R35, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  br label %R33

; <label>:37                                      ; preds = %R32, %R24
  ret void
}

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A0

; Function Attrs: nounwind readnone
declare i32 @dx.op.binary.i32(i32, i32, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A2

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A3

; Function Attrs: nounwind readnone
declare i32 @dx.op.flattenedThreadIdInGroup.i32(i32) #A1

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A1

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A1

attributes #A0 = { noduplicate nounwind }
attributes #A1 = { nounwind readnone }
attributes #A2 = { nounwind }
attributes #A3 = { nounwind readonly }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 0}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{null, !M6, null, null}
!M6 = !{!M7}
!M7 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M5 = !{void ()* @main, !"main", null, !M4, !M8}
!M8 = !{i32 0, i64 16, i32 4, !M9}
!M9 = !{i32 256, i32 1, i32 1}
!M10 = !{!M11, !M11, i64 0}
!M11 = !{!"float", !M12, i64 0}
!M12 = !{!"omnipotent char", !M13, i64 0}
!M13 = !{!"<ident>"}

