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
; out_                                  UAV    byte         r/w      U0             u0     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%struct.S0 = type { i32 }

@ret.i.hca = internal unnamed_addr constant [2 x i32] [i32 1, i32 2]

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  br label %R1

; <label>:2                                       ; preds = %R2, %R3
  %R4 = phi i32 [ %R5, %R2 ], [ 0, %R3 ]
  %R6 = phi i32 [ %R7, %R2 ], [ -1, %R3 ]
  %R8 = phi i32 [ %R9, %R2 ], [ -1, %R3 ]
  %R10 = phi i32 [ 1, %R2 ], [ 0, %R3 ]
  %R11 = icmp eq i32 %R8, 0
  %R12 = zext i1 %R11 to i32
  %R9 = add i32 %R8, -1
  %R5 = add i32 %R10, %R4
  %R13 = icmp slt i32 %R5, 2
  br i1 %R13, label %R2, label %R14

; <label>:12                                      ; preds = %R1
  %R7 = sub i32 %R6, %R12
  %R15 = call i32 @dx.op.binary.i32(i32 40, i32 %R5, i32 1)  ; UMin(a,b)
  %R16 = getelementptr inbounds [2 x i32], [2 x i32]* @ret.i.hca, i32 0, i32 %R15
  %R17 = load i32, i32* %R16, align 4, !tbaa !M0
  %R18 = shl nsw i32 %R5, 2
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 %R18, i32 undef, i32 %R17, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  %R19 = icmp eq i32 %R6, %R12
  %R20 = icmp eq i32 %R9, 0
  %R21 = and i1 %R20, %R19
  br i1 %R21, label %R14, label %R1

; <label>:21                                      ; preds = %R2, %R1
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.binary.i32(i32, i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A2

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind readonly }
attributes #A2 = { nounwind }

!llvm.ident = !{!M1}
!dx.version = !{!M2}
!dx.valver = !{!M3}
!dx.shaderModel = !{!M4}
!dx.resources = !{!M5}
!dx.entryPoints = !{!M6}

!M1 = !{!"<ident>"}
!M2 = !{i32 1, i32 0}
!M3 = !{i32 1, i32 8}
!M4 = !{!"cs", i32 6, i32 0}
!M5 = !{null, !M7, null, null}
!M7 = !{!M8}
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M6 = !{void ()* @main, !"main", null, !M5, !M9}
!M9 = !{i32 0, i64 16, i32 4, !M10}
!M10 = !{i32 1, i32 1, i32 1}
!M0 = !{!M11, !M11, i64 0}
!M11 = !{!"int", !M12, i64 0}
!M12 = !{!"omnipotent char", !M13, i64 0}
!M13 = !{!"<ident>"}

