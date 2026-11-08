// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// DXC's validator refuses an entry point whose thread group shared memory
// (TGSM, WGSL's workgroup storage) is larger than the shader model allows:
// 32768 bytes for compute, amplification and node shaders
// (DXIL::kMaxTGSMSize, DxilConstants.h:130) and 28672 for mesh shaders
// (kMaxMSSMSize, DxilConstants.h:147), the sums DxilValidation.cpp:3649-3656
// picks and :3762 compares against. Without the check a compute entry point
// with 4 MiB of workgroup variables compiled silently to a container D3D12
// refuses, so Compile refuses it as unsupported, naming the limit.
func TestCompileRefusesOversizedWorkgroupVariables(t *testing.T) {
	const words = maxWorkgroupVariableBytes / 4
	tests := []struct{ name, decl, use string }{
		{"array of scalars", fmt.Sprintf("var<workgroup> a: array<u32, %d>;", words+1), "a[0] = 1u; o[0] = f32(a[0]);"},
		{"array of vectors", fmt.Sprintf("var<workgroup> a: array<vec4<f32>, %d>;", words/4+1), "a[0] = vec4(1.0); o[0] = a[0].x;"},
		{"array of structs", fmt.Sprintf("var<workgroup> a: array<S, %d>;", words/2), "a[0].k = 1.0; o[0] = a[0].k;"},
		{"two variables together", fmt.Sprintf("var<workgroup> a: array<u32, %d>; var<workgroup> b: array<u32, %d>;", words/2+1, words/2+1),
			"a[0] = 1u; b[0] = 2u; o[0] = f32(a[0] + b[0]);"},
		{"four mebibytes", "var<workgroup> a: array<vec4<f32>, 262144>;", "a[0] = vec4(1.0); o[0] = a[0].x;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "@group(0) @binding(0) var<storage, read_write> o: array<f32>;\nstruct S { p: vec2<f32>, k: f32 }\n" +
				tt.decl + "\n@compute @workgroup_size(1) fn main() {\n" + tt.use + "\n}\n"
			_, _, err := Compile(lowerSource(t, src), DefaultOptions())
			if err == nil || !errors.Is(err, backend.ErrUnsupported) || !strings.Contains(err.Error(), "bytes of workgroup") {
				t.Fatalf("err = %v, want the workgroup variables refused as too large", err)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", maxWorkgroupVariableBytes)) {
				t.Errorf("err = %v, want it to name the %d-byte limit", err, maxWorkgroupVariableBytes)
			}
		})
	}
}

// Workgroup variables up to the limit compile and validate: DXC's check is
// TotalSize > MaxSize, so a variable of exactly 32768 bytes is accepted.
func TestCompileAcceptsWorkgroupVariablesAtTheLimit(t *testing.T) {
	words := maxWorkgroupVariableBytes / 4
	compileValidateEach(t, fmt.Sprintf(`@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@group(0) @binding(1) var<storage, read> idx: array<u32>;
var<workgroup> a: array<u32, %d>;
var<workgroup> b: array<vec4<f32>, %d>;
@compute @workgroup_size(1) fn main() {
    let i = idx[0];
    a[i %% %du] = i;
    b[i %% %du] = vec4(f32(a[1]));
    o[0] = f32(a[0]) + b[3].x;
}
`, words/2, words/8, words/2, words/8))
}

// The byte size counted for each workgroup variable is the size of the
// groupshared globals the entry point reaches once scalarized, which is what
// the emitter writes and the validator measures.
func TestWorkgroupVariableBytes(t *testing.T) {
	m := lowerSource(t, `
struct S { p: vec3<f32>, k: f32 }
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
var<workgroup> a: f32;
var<workgroup> b: vec4<f32>;
var<workgroup> c: array<vec2<f32>, 10>;
var<workgroup> d: S;
@compute @workgroup_size(1) fn main() {
    a = 1.0; b = vec4(2.0); c[0] = vec2(3.0); d.k = 4.0;
    o[0] = a + b.x + c[1].y + d.k;
}
`)
	// A struct is split into one variable per member, so its members count,
	// not its padded span.
	want := uint64(4 + 16 + 80 + 16)
	m2, err := prepareModule(m)
	if err != nil {
		t.Fatalf("prepareModule: %v", err)
	}
	if err := prepareEntryPoint(m2, &m2.EntryPoints[0], DefaultOptions()); err != nil {
		t.Fatalf("prepareEntryPoint: %v", err)
	}
	if got := workgroupVariableBytes(m2, &m2.EntryPoints[0]); got != want {
		t.Errorf("%d bytes of workgroup memory, want %d", got, want)
	}
}

// The per-stage limit is DXC's table: kMaxTGSMSize for compute, amplification
// (task) and node shaders, kMaxMSSMSize for mesh, and no thread group shared
// memory at all in the stages DXC's front end refuses it in
// (DxilValidation.cpp:3649-3656).
func TestMaxWorkgroupBytes(t *testing.T) {
	want := map[ir.ShaderStage]uint64{
		ir.StageCompute: maxWorkgroupVariableBytes,
		ir.StageTask:    maxWorkgroupVariableBytes,
		ir.StageMesh:    maxMeshWorkgroupVariableBytes,
	}
	for stage, bytes := range want {
		got, ok := maxWorkgroupBytes(stage)
		if !ok || got != bytes {
			t.Errorf("%s: %d (ok %t), want %d", stage, got, ok, bytes)
		}
	}
	for _, stage := range []ir.ShaderStage{ir.StageVertex, ir.StageFragment} {
		if got, ok := maxWorkgroupBytes(stage); ok {
			t.Errorf("%s: %d bytes, want no workgroup storage", stage, got)
		}
	}
}

// An entry point of a stage with no thread group shared memory is refused
// rather than written as groupshared the validator rejects
// (SM.TGSMUNSUPPORTED, docs/DXIL.rst:3382).
func TestCheckWorkgroupVariableBytesRefusesStageWithoutTGSM(t *testing.T) {
	m := lowerSource(t, `
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
var<workgroup> a: array<f32, 16>;
@compute @workgroup_size(1) fn main() {
    a[0] = 1.0;
    o[0] = a[0];
}
`)
	m2, err := prepareModule(m)
	if err != nil {
		t.Fatalf("prepareModule: %v", err)
	}
	ep := &m2.EntryPoints[0]
	if err := prepareEntryPoint(m2, ep, DefaultOptions()); err != nil {
		t.Fatalf("prepareEntryPoint: %v", err)
	}
	if err := checkWorkgroupVariableBytes(m2, ep); err != nil {
		t.Fatalf("compute entry point of 64 workgroup bytes: %v", err)
	}
	ep.Stage = ir.StageFragment
	err = checkWorkgroupVariableBytes(m2, ep)
	if err == nil || !errors.Is(err, backend.ErrUnsupported) || !strings.Contains(err.Error(), "not supported in a fragment entry point") {
		t.Fatalf("err = %v, want the workgroup variable refused for the stage", err)
	}
}
