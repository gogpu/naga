;
; Note: shader requires additional functionality:
;       64-Bit integer
;       64-bit Atomics on Typed Resources
;       64-bit Atomics on Heap Resources
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
; NumThreads=(2,1,1)
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
; EntryFunctionName: cs_main
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; image                                 UAV     u32          2d      U0             u0     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%"class.RWTexture2D<unsigned long long>" = type { i64 }

define void @cs_main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R0, i32 7, i32 0, i32 0, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  call void @dx.op.barrier(i32 80, i32 9)  ; Barrier(barrierMode)
  %R2 = call i64 @dx.op.atomicBinOp.i64(i32 78, %dx.types.Handle %R0, i32 6, i32 0, i32 0, i32 undef, i64 1)  ; AtomicBinOp(handle,atomicOp,offset0,offset1,offset2,newValue)
  ret void
}

; Function Attrs: nounwind
declare i64 @dx.op.atomicBinOp.i64(i32, %dx.types.Handle, i32, i32, i32, i32, i64) #A0

; Function Attrs: noduplicate nounwind
declare void @dx.op.barrier(i32, i32) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

attributes #A0 = { nounwind }
attributes #A1 = { noduplicate nounwind }
attributes #A2 = { nounwind readonly }

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
!M7 = !{i32 0, %"class.RWTexture2D<unsigned long long>"* undef, !"", i32 0, i32 0, i32 1, i32 2, i1 false, i1 false, i1 false, !M8}
!M8 = !{i32 0, i32 5}
!M5 = !{void ()* @cs_main, !"cs_main", null, !M4, !M9}
!M9 = !{i32 0, i64 4430233600, i32 4, !M10}
!M10 = !{i32 2, i32 1, i32 1}

