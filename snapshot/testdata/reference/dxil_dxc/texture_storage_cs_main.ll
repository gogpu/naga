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
; NumThreads=(8,8,1)
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
; output_tex                            UAVunorm_f32          2d      U0             u0     1
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

%dx.types.Handle = type { i8* }
%dx.types.Dimensions = type { i32, i32, i32, i32 }
%"class.RWTexture2D<vector<float, 4> >" = type { <4 x float> }

define void @main() {
  %R0 = call %dx.types.Handle @dx.op.createHandle(i32 57, i8 1, i32 0, i32 0, i1 false)  ; CreateHandle(resourceClass,rangeId,index,nonUniformIndex)
  %R1 = call i32 @dx.op.threadId.i32(i32 93, i32 0)  ; ThreadId(component)
  %R2 = call i32 @dx.op.threadId.i32(i32 93, i32 1)  ; ThreadId(component)
  %R3 = call %dx.types.Dimensions @dx.op.getDimensions(i32 72, %dx.types.Handle %R0, i32 0)  ; GetDimensions(handle,mipLevel)
  %R4 = extractvalue %dx.types.Dimensions %R3, 0
  %R5 = extractvalue %dx.types.Dimensions %R3, 1
  %R6 = icmp uge i32 %R1, %R4
  %R7 = icmp uge i32 %R2, %R5
  %R8 = or i1 %R6, %R7
  br i1 %R8, label %R9, label %R10

; <label>:10                                      ; preds = %R11
  %R12 = uitofp i32 %R1 to float
  %R13 = uitofp i32 %R4 to float
  %R14 = fdiv fast float %R12, %R13
  %R15 = uitofp i32 %R2 to float
  %R16 = uitofp i32 %R5 to float
  %R17 = fdiv fast float %R15, %R16
  call void @dx.op.textureStore.f32(i32 67, %dx.types.Handle %R0, i32 %R1, i32 %R2, i32 undef, float %R14, float %R17, float 5.000000e-01, float 1.000000e+00, i8 15)  ; TextureStore(srv,coord0,coord1,coord2,value0,value1,value2,value3,mask)
  br label %R9

; <label>:17                                      ; preds = %R10, %R11
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.threadId.i32(i32, i32) #A0

; Function Attrs: nounwind
declare void @dx.op.textureStore.f32(i32, %dx.types.Handle, i32, i32, i32, float, float, float, float, i8) #A1

; Function Attrs: nounwind readonly
declare %dx.types.Dimensions @dx.op.getDimensions(i32, %dx.types.Handle, i32) #A2

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1) #A2

attributes #A0 = { nounwind readnone }
attributes #A1 = { nounwind }
attributes #A2 = { nounwind readonly }

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
!M4 = !{null, !M6, null, null}
!M6 = !{!M7}
!M7 = !{i32 0, %"class.RWTexture2D<vector<float, 4> >"* undef, !"", i32 0, i32 0, i32 1, i32 2, i1 false, i1 false, i1 false, !M8}
!M8 = !{i32 0, i32 14}
!M5 = !{void ()* @main, !"main", null, !M4, !M9}
!M9 = !{i32 4, !M10}
!M10 = !{i32 8, i32 8, i32 1}

