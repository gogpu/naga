package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// A switch's merge block is reachable whenever a case leaves the switch:
// by falling off its end or by a break, which in a switch case targets the
// switch's own merge. Rust naga (back/spv/block.rs, Statement::Switch)
// always continues writing in the merge block. Until v0.21.11 a case that
// ended in break counted as terminated, so a switch whose every case ended
// in break got an OpUnreachable merge and every statement after it was
// dropped, with spirv-val passing: a silent miscompile since v0.15.0.
//
// A switch whose every case ends in continue, return or kill leaves through
// another construct, so its merge is unreachable and stays OpUnreachable.
func TestSwitchMergeReachability(t *testing.T) {
	tests := []struct {
		name string
		src  string
		// marker is the constant stored by the statement after the switch.
		marker      uint32
		wantMarker  bool
		unreachable int
	}{
		{
			name: "every case breaks",
			src: `
@group(0) @binding(0) var<storage, read_write> b: array<i32, 2>;
@compute @workgroup_size(1) fn main() {
  let x = b[0];
  switch x {
    case 1: { b[1] = 20; break; }
    default: { b[1] = 40; break; }
  }
  b[1] = 50;
}`,
			marker: 50, wantMarker: true,
		},
		{
			name: "every case breaks inside a loop",
			src: `
@group(0) @binding(0) var<storage, read_write> b: array<i32, 3>;
@compute @workgroup_size(1) fn main() {
  var i = 0;
  loop {
    if i >= 4 { break; }
    switch i {
      case 1: { b[0] = b[0] + 1; break; }
      default: { b[1] = b[1] + 1; break; }
    }
    b[2] = 77;
    continuing { i = i + 1; }
  }
}`,
			marker: 77, wantMarker: true,
		},
		{
			name: "some cases break, others return",
			src: `
@group(0) @binding(0) var<storage, read_write> b: array<i32, 2>;
@compute @workgroup_size(1) fn main() {
  let x = b[0];
  switch x {
    case 1: { b[1] = 20; break; }
    case 2: { return; }
    default: { b[1] = 40; break; }
  }
  b[1] = 50;
}`,
			marker: 50, wantMarker: true,
		},
		{
			name: "every case continues",
			src: `
@group(0) @binding(0) var<storage, read_write> b: array<i32, 3>;
@compute @workgroup_size(1) fn main() {
  var i = 0;
  loop {
    if i >= 4 { break; }
    switch i {
      case 1: { b[0] = 1; continue; }
      default: { b[1] = 2; continue; }
    }
    b[2] = 77;
    continuing { i = i + 1; }
  }
}`,
			marker: 77, wantMarker: false, unreachable: 1,
		},
		{
			name: "every case returns",
			src: `
@group(0) @binding(0) var<storage, read_write> b: array<i32, 2>;
@compute @workgroup_size(1) fn main() {
  let x = b[0];
  switch x {
    case 1: { b[1] = 20; return; }
    default: { b[1] = 40; return; }
  }
  b[1] = 50;
}`,
			marker: 50, wantMarker: false, unreachable: 1,
		},
		{
			name: "every case discards",
			src: `
@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
  var c = vec4<f32>(0.0);
  switch i32(p.x) {
    case 1: { discard; }
    default: { discard; }
  }
  c.x = 0.75;
  return c;
}`,
			marker: 0x3F400000, wantMarker: false, unreachable: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spv := compileWGSLToSPIRV(t, tt.name, tt.src)
			instrs := decodeSPIRVInstructions(spv)
			if got := storesConstant(instrs, tt.marker); got != tt.wantMarker {
				t.Errorf("store of the constant %#x after the switch: %v, want %v", tt.marker, got, tt.wantMarker)
			}
			if got := countOpcode(instrs, spvenums.OpUnreachable); got != tt.unreachable {
				t.Errorf("OpUnreachable count = %d, want %d", got, tt.unreachable)
			}
			spirvValidate(t, spv)
		})
	}
}

// storesConstant reports whether the module stores an OpConstant whose
// single-word value is value.
func storesConstant(instrs []spirvInstruction, value uint32) bool {
	ids := map[uint32]bool{}
	for _, in := range instrs {
		if in.opcode == spvenums.OpConstant && in.wordCount == 4 && in.words[3] == value {
			ids[in.words[2]] = true
		}
	}
	for _, in := range instrs {
		if in.opcode == spvenums.OpStore && in.wordCount >= 3 && ids[in.words[2]] {
			return true
		}
	}
	return false
}

// spirvValidate runs spirv-val on the module; the tool is required when
// NAGA_REQUIRE_TOOLS names it and skipped otherwise.
func spirvValidate(t *testing.T, spv []byte) {
	t.Helper()
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	path := filepath.Join(t.TempDir(), "m.spv")
	if err := os.WriteFile(path, spv, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(spirvVal, "--target-env", "vulkan1.1", path).CombinedOutput(); err != nil {
		t.Fatalf("spirv-val:\n%s", out)
	}
}
