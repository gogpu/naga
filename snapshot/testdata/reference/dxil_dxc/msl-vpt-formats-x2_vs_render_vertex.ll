;
; Input signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; LOC                      0   xy          0     NONE    uint   x
; LOC                      1   xy          1     NONE    uint
; LOC                      2   xy          2     NONE    uint
; LOC                      3   xy          3     NONE     int
; LOC                      4   xy          4     NONE     int
; LOC                      5   xy          5     NONE     int
; LOC                      6   xy          6     NONE   float
; LOC                      7   xy          7     NONE   float
; LOC                      8   xy          8     NONE   float
; LOC                      9   xy          9     NONE   float
; LOC                     10   xy         10     NONE   float
; LOC                     11   xy         11     NONE   float
; LOC                     12   xy         12     NONE    uint
; LOC                     13   xy         13     NONE    uint
; LOC                     14   xy         14     NONE    uint
; LOC                     15   xy         15     NONE     int
; LOC                     16   xy         16     NONE     int
; LOC                     17   xy         17     NONE     int
; LOC                     18   xy         18     NONE   float
; LOC                     19   xy         19     NONE   float
; LOC                     20   xy         20     NONE   float
; LOC                     21   xy         21     NONE   float
; LOC                     22   xy         22     NONE   float
; LOC                     23   xy         23     NONE   float
; LOC                     24   xy         24     NONE   float
; LOC                     25   xy         25     NONE   float
; LOC                     26   xy         26     NONE   float
; LOC                     27   xy         27     NONE   float
; LOC                     28   xy         28     NONE   float
; LOC                     29   xy         29     NONE   float
; LOC                     30   xy         30     NONE   float
; LOC                     31   xy         31     NONE    uint
; LOC                     32   xy         32     NONE    uint
; LOC                     33   xy         33     NONE    uint
; LOC                     34   xy         34     NONE    uint
; LOC                     35   xy         35     NONE     int
; LOC                     36   xy         36     NONE     int
; LOC                     37   xy         37     NONE     int
; LOC                     38   xy         38     NONE     int
; LOC                     39   xy         39     NONE   float
; LOC                     40   xy         40     NONE   float
;
;
; Output signature:
;
; Name                 Index   Mask Register SysValue  Format   Used
; -------------------- ----- ------ -------- -------- ------- ------
; SV_Position              0   xyzw        0      POS   float   xyzw
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
; SigInputElements: 41
; SigOutputElements: 1
; SigPatchConstOrPrimElements: 0
; SigInputVectors: 41
; SigOutputVectors[0]: 1
; SigOutputVectors[1]: 0
; SigOutputVectors[2]: 0
; SigOutputVectors[3]: 0
; EntryFunctionName: render_vertex
;
;
; Input signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; LOC                      0
; LOC                      1
; LOC                      2
; LOC                      3
; LOC                      4
; LOC                      5
; LOC                      6
; LOC                      7
; LOC                      8
; LOC                      9
; LOC                     10
; LOC                     11
; LOC                     12
; LOC                     13
; LOC                     14
; LOC                     15
; LOC                     16
; LOC                     17
; LOC                     18
; LOC                     19
; LOC                     20
; LOC                     21
; LOC                     22
; LOC                     23
; LOC                     24
; LOC                     25
; LOC                     26
; LOC                     27
; LOC                     28
; LOC                     29
; LOC                     30
; LOC                     31
; LOC                     32
; LOC                     33
; LOC                     34
; LOC                     35
; LOC                     36
; LOC                     37
; LOC                     38
; LOC                     39
; LOC                     40
;
; Output signature:
;
; Name                 Index             InterpMode DynIdx
; -------------------- ----- ---------------------- ------
; SV_Position              0          noperspective
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
; Number of inputs: 162, outputs: 4
; Outputs dependent on ViewId: {  }
; Inputs contributing to computation of Outputs:
;   output 0 depends on inputs: { 0 }
;   output 1 depends on inputs: { 0 }
;   output 2 depends on inputs: { 0 }
;   output 3 depends on inputs: { 0 }
;
target datalayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
target triple = "dxil-ms-dx"

define void @render_vertex() {
  %R0 = call i32 @dx.op.loadInput.i32(i32 4, i32 0, i32 0, i8 0, i32 undef)  ; LoadInput(inputSigId,rowIndex,colIndex,gsVertexAxis)
  %R1 = uitofp i32 %R0 to float
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float %R1)  ; StoreOutput(outputSigId,rowIndex,colIndex,value)
  ret void
}

; Function Attrs: nounwind readnone
declare i32 @dx.op.loadInput.i32(i32, i32, i32, i8, i32) #A0

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
!M3 = !{!"vs", i32 6, i32 0}
!M4 = !{[164 x i32] [i32 162, i32 4, i32 15, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0, i32 0]}
!M5 = !{void ()* @render_vertex, !"render_vertex", !M6, null, null}
!M6 = !{!M7, !M8, null}
!M7 = !{!M9, !M10, !M11, !M12, !M13, !M14, !M15, !M16, !M17, !M18, !M19, !M20, !M21, !M22, !M23, !M24, !M25, !M26, !M27, !M28, !M29, !M30, !M31, !M32, !M33, !M34, !M35, !M36, !M37, !M38, !M39, !M40, !M41, !M42, !M43, !M44, !M45, !M46, !M47, !M48, !M49}
!M9 = !{i32 0, !"LOC", i8 5, i8 0, !M50, i8 0, i32 1, i8 2, i32 0, i8 0, !M51}
!M50 = !{i32 0}
!M51 = !{i32 3, i32 1}
!M10 = !{i32 1, !"LOC", i8 5, i8 0, !M52, i8 0, i32 1, i8 2, i32 1, i8 0, null}
!M52 = !{i32 1}
!M11 = !{i32 2, !"LOC", i8 5, i8 0, !M53, i8 0, i32 1, i8 2, i32 2, i8 0, null}
!M53 = !{i32 2}
!M12 = !{i32 3, !"LOC", i8 4, i8 0, !M54, i8 0, i32 1, i8 2, i32 3, i8 0, null}
!M54 = !{i32 3}
!M13 = !{i32 4, !"LOC", i8 4, i8 0, !M55, i8 0, i32 1, i8 2, i32 4, i8 0, null}
!M55 = !{i32 4}
!M14 = !{i32 5, !"LOC", i8 4, i8 0, !M56, i8 0, i32 1, i8 2, i32 5, i8 0, null}
!M56 = !{i32 5}
!M15 = !{i32 6, !"LOC", i8 9, i8 0, !M57, i8 0, i32 1, i8 2, i32 6, i8 0, null}
!M57 = !{i32 6}
!M16 = !{i32 7, !"LOC", i8 9, i8 0, !M58, i8 0, i32 1, i8 2, i32 7, i8 0, null}
!M58 = !{i32 7}
!M17 = !{i32 8, !"LOC", i8 9, i8 0, !M59, i8 0, i32 1, i8 2, i32 8, i8 0, null}
!M59 = !{i32 8}
!M18 = !{i32 9, !"LOC", i8 9, i8 0, !M60, i8 0, i32 1, i8 2, i32 9, i8 0, null}
!M60 = !{i32 9}
!M19 = !{i32 10, !"LOC", i8 9, i8 0, !M61, i8 0, i32 1, i8 2, i32 10, i8 0, null}
!M61 = !{i32 10}
!M20 = !{i32 11, !"LOC", i8 9, i8 0, !M62, i8 0, i32 1, i8 2, i32 11, i8 0, null}
!M62 = !{i32 11}
!M21 = !{i32 12, !"LOC", i8 5, i8 0, !M63, i8 0, i32 1, i8 2, i32 12, i8 0, null}
!M63 = !{i32 12}
!M22 = !{i32 13, !"LOC", i8 5, i8 0, !M64, i8 0, i32 1, i8 2, i32 13, i8 0, null}
!M64 = !{i32 13}
!M23 = !{i32 14, !"LOC", i8 5, i8 0, !M65, i8 0, i32 1, i8 2, i32 14, i8 0, null}
!M65 = !{i32 14}
!M24 = !{i32 15, !"LOC", i8 4, i8 0, !M66, i8 0, i32 1, i8 2, i32 15, i8 0, null}
!M66 = !{i32 15}
!M25 = !{i32 16, !"LOC", i8 4, i8 0, !M67, i8 0, i32 1, i8 2, i32 16, i8 0, null}
!M67 = !{i32 16}
!M26 = !{i32 17, !"LOC", i8 4, i8 0, !M68, i8 0, i32 1, i8 2, i32 17, i8 0, null}
!M68 = !{i32 17}
!M27 = !{i32 18, !"LOC", i8 9, i8 0, !M69, i8 0, i32 1, i8 2, i32 18, i8 0, null}
!M69 = !{i32 18}
!M28 = !{i32 19, !"LOC", i8 9, i8 0, !M70, i8 0, i32 1, i8 2, i32 19, i8 0, null}
!M70 = !{i32 19}
!M29 = !{i32 20, !"LOC", i8 9, i8 0, !M71, i8 0, i32 1, i8 2, i32 20, i8 0, null}
!M71 = !{i32 20}
!M30 = !{i32 21, !"LOC", i8 9, i8 0, !M72, i8 0, i32 1, i8 2, i32 21, i8 0, null}
!M72 = !{i32 21}
!M31 = !{i32 22, !"LOC", i8 9, i8 0, !M73, i8 0, i32 1, i8 2, i32 22, i8 0, null}
!M73 = !{i32 22}
!M32 = !{i32 23, !"LOC", i8 9, i8 0, !M74, i8 0, i32 1, i8 2, i32 23, i8 0, null}
!M74 = !{i32 23}
!M33 = !{i32 24, !"LOC", i8 9, i8 0, !M75, i8 0, i32 1, i8 2, i32 24, i8 0, null}
!M75 = !{i32 24}
!M34 = !{i32 25, !"LOC", i8 9, i8 0, !M76, i8 0, i32 1, i8 2, i32 25, i8 0, null}
!M76 = !{i32 25}
!M35 = !{i32 26, !"LOC", i8 9, i8 0, !M77, i8 0, i32 1, i8 2, i32 26, i8 0, null}
!M77 = !{i32 26}
!M36 = !{i32 27, !"LOC", i8 9, i8 0, !M78, i8 0, i32 1, i8 2, i32 27, i8 0, null}
!M78 = !{i32 27}
!M37 = !{i32 28, !"LOC", i8 9, i8 0, !M79, i8 0, i32 1, i8 2, i32 28, i8 0, null}
!M79 = !{i32 28}
!M38 = !{i32 29, !"LOC", i8 9, i8 0, !M80, i8 0, i32 1, i8 2, i32 29, i8 0, null}
!M80 = !{i32 29}
!M39 = !{i32 30, !"LOC", i8 9, i8 0, !M81, i8 0, i32 1, i8 2, i32 30, i8 0, null}
!M81 = !{i32 30}
!M40 = !{i32 31, !"LOC", i8 5, i8 0, !M82, i8 0, i32 1, i8 2, i32 31, i8 0, null}
!M82 = !{i32 31}
!M41 = !{i32 32, !"LOC", i8 5, i8 0, !M83, i8 0, i32 1, i8 2, i32 32, i8 0, null}
!M83 = !{i32 32}
!M42 = !{i32 33, !"LOC", i8 5, i8 0, !M84, i8 0, i32 1, i8 2, i32 33, i8 0, null}
!M84 = !{i32 33}
!M43 = !{i32 34, !"LOC", i8 5, i8 0, !M85, i8 0, i32 1, i8 2, i32 34, i8 0, null}
!M85 = !{i32 34}
!M44 = !{i32 35, !"LOC", i8 4, i8 0, !M86, i8 0, i32 1, i8 2, i32 35, i8 0, null}
!M86 = !{i32 35}
!M45 = !{i32 36, !"LOC", i8 4, i8 0, !M87, i8 0, i32 1, i8 2, i32 36, i8 0, null}
!M87 = !{i32 36}
!M46 = !{i32 37, !"LOC", i8 4, i8 0, !M88, i8 0, i32 1, i8 2, i32 37, i8 0, null}
!M88 = !{i32 37}
!M47 = !{i32 38, !"LOC", i8 4, i8 0, !M89, i8 0, i32 1, i8 2, i32 38, i8 0, null}
!M89 = !{i32 38}
!M48 = !{i32 39, !"LOC", i8 9, i8 0, !M90, i8 0, i32 1, i8 2, i32 39, i8 0, null}
!M90 = !{i32 39}
!M49 = !{i32 40, !"LOC", i8 9, i8 0, !M91, i8 0, i32 1, i8 2, i32 40, i8 0, null}
!M91 = !{i32 40}
!M8 = !{!M92}
!M92 = !{i32 0, !"SV_Position", i8 9, i8 3, !M50, i8 4, i32 1, i8 4, i32 0, i8 0, !M93}
!M93 = !{i32 3, i32 15}

