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
; unsized_comma                         UAV    byte         r/w      U0             u0     1
; unsized_no_comma                      UAV    byte         r/w      U1             u1     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.ResRet.i32 = type { i32, i32, i32, i32, i32 }
%struct.S0 = type { i32 }

@"\01?sized_comma@@3PAIA" = external addrspace(3) global [1 x i32], align 4
@"\01?sized_no_comma@@3PAIA" = external addrspace(3) global [1 x i32], align 4

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 1, i32 1, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R2 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 0)  ; ThreadIdInGroup(component)
  %R3 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 1)  ; ThreadIdInGroup(component)
  %R4 = call i32 @dx.op.threadIdInGroup.i32(i32 95, i32 2)  ; ThreadIdInGroup(component)
  %R5 = or i32 %R3, %R2
  %R6 = or i32 %R5, %R4
  %R7 = icmp eq i32 %R6, 0
  br i1 %R7, label %R8, label %R9

; <label>:9                                       ; preds = %R10
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([1 x i32], [1 x i32] addrspace(3)* @"\01?sized_comma@@3PAIA", i32 0, i32 0), align 4
  store i32 0, i32 addrspace(3)* getelementptr inbounds ([1 x i32], [1 x i32] addrspace(3)* @"\01?sized_no_comma@@3PAIA", i32 0, i32 0), align 4
  br label %R9

; <label>:10                                      ; preds = %R8, %R10
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R11 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R1, i32 0, i32 undef)  ; BufferLoad(srv,index,wot)
  %R12 = extractvalue %dx.types.ResRet.i32 %R11, 0
  store i32 %R12, i32 addrspace(3)* getelementptr inbounds ([1 x i32], [1 x i32] addrspace(3)* @"\01?sized_comma@@3PAIA", i32 0, i32 0), align 4, !tbaa !M0
  %R13 = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32 68, %dx.types.Handle %R0, i32 0, i32 undef)  ; BufferLoad(srv,index,wot)
  %R14 = extractvalue %dx.types.ResRet.i32 %R13, 0
  store i32 %R14, i32 addrspace(3)* getelementptr inbounds ([1 x i32], [1 x i32] addrspace(3)* @"\01?sized_no_comma@@3PAIA", i32 0, i32 0), align 4, !tbaa !M0
  %R15 = add i32 %R14, %R12
  call void @dx.op.bufferStore.i32(i32 69, %dx.types.Handle %R0, i32 0, i32 undef, i32 %R15, i32 undef, i32 undef, i32 undef, i8 1)  ; BufferStore(uav,coord0,coord1,value0,value1,value2,value3,mask)
  ret void
}

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A0

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32, %dx.types.Handle, i32, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32, %dx.types.Handle, i32, i32, i32, i32, i32, i32, i8) #A2

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadIdInGroup.i32(i32, i32) #A3

attributes #A0 = { noduplicate nounwind }
attributes #A1 = { nounwind readonly }
attributes #A2 = { nounwind }
attributes #A3 = { nounwind readnone }

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
!M7 = !{!M8, !M9}
!M8 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M9 = !{i32 1, %struct.S0* undef, !"", i32 0, i32 1, i32 1, i32 11, i1 false, i1 false, i1 false, null}
!M6 = !{void ()* @main, !"main", null, !M5, !M10}
!M10 = !{i32 0, i64 16, i32 4, !M11}
!M11 = !{i32 1, i32 1, i32 1}
!M0 = !{!M12, !M12, i64 0}
!M12 = !{!"int", !M13, i64 0}
!M13 = !{!"omnipotent char", !M14, i64 0}
!M14 = !{!"<ident>"}

