;
; Note: shader requires additional functionality:
;       Raytracing tier 1.1 features
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
; EntryFunctionName: main_candidate
;
;
; Buffer Definitions: <stripped>
; Resource Bindings:
;
; Name                                 Type  Format         Dim      ID      HLSL Bind  Count
; ------------------------------ ---------- ------- ----------- ------- -------------- ------
; acc_struct                        texture     i32         ras      T0             t0     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%struct.S0 = type { i32 }

define void @main_candidate() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 0, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.allocateRayQuery(i32 178, i32 0)  ; AllocateRayQuery(constRayFlags)
  call void @dx.op.rayQuery_TraceRayInline(i32 179, i32 %R1, %dx.types.Handle %R0, i32 4, i32 255, float 0.000000e+00, float 0.000000e+00, float 0.000000e+00, float 0x3FB99999A0000000, float 0.000000e+00, float 1.000000e+00, float 0.000000e+00, float 1.000000e+02)  ; RayQuery_TraceRayInline(rayQueryHandle,accelerationStructure,rayFlags,instanceInclusionMask,origin_X,origin_Y,origin_Z,tMin,direction_X,direction_Y,direction_Z,tMax)
  %R2 = call i32 @dx.op.rayQuery_StateScalar.i32(i32 185, i32 %R1)  ; RayQuery_CandidateType(rayQueryHandle)
  %R3 = icmp eq i32 %R2, 0
  br i1 %R3, label %R4, label %R5

; <label>:5                                       ; preds = %R6
  call void @dx.op.rayQuery_CommitProceduralPrimitiveHit(i32 183, i32 %R1, float 1.000000e+01)  ; RayQuery_CommitProceduralPrimitiveHit(rayQueryHandle,t)
  br label %R7

; <label>:6                                       ; preds = %R6
  call void @dx.op.rayQuery_CommitNonOpaqueTriangleHit(i32 182, i32 %R1)  ; RayQuery_CommitNonOpaqueTriangleHit(rayQueryHandle)
  br label %R7

; <label>:7                                       ; preds = %R4, %R5
  ret void
}

; Function Attrs: nounwind
declare i32 @dx.op.allocateRayQuery(i32, i32) #A0

; Function Attrs: nounwind readonly
declare i32 @dx.op.rayQuery_StateScalar.i32(i32, i32) #A1

; Function Attrs: nounwind
declare void @dx.op.rayQuery_TraceRayInline(i32, i32, %dx.types.Handle, i32, i32, float, float, float, float, float, float, float, float) #A0

; Function Attrs: nounwind
declare void @dx.op.rayQuery_CommitNonOpaqueTriangleHit(i32, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.rayQuery_CommitProceduralPrimitiveHit(i32, i32, float) #A0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A1

attributes #A0 = { nounwind }
attributes #A1 = { nounwind readonly }

!llvm.ident = !{!M0}
!dx.version = !{!M1}
!dx.valver = !{!M2}
!dx.shaderModel = !{!M3}
!dx.resources = !{!M4}
!dx.entryPoints = !{!M5}

!M0 = !{!"<ident>"}
!M1 = !{i32 1, i32 0}
!M2 = !{i32 1, i32 8}
!M3 = !{!"cs", i32 6, i32 0}
!M4 = !{!M6, null, null, null}
!M6 = !{!M7}
!M7 = !{i32 0, %struct.S0* undef, !"", i32 0, i32 0, i32 1, i32 16, i32 0, !M8}
!M8 = !{i32 0, i32 4}
!M5 = !{void ()* @main_candidate, !"main_candidate", null, !M4, !M9}
!M9 = !{i32 0, i64 33554432, i32 4, !M10}
!M10 = !{i32 1, i32 1, i32 1}

