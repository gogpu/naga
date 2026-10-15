package snapshot_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// rustOutput is one reference file Rust naga's snapshot driver writes for a
// WGSL shader (naga/tests/naga/snapshots.rs).
type rustOutput struct {
	kind       string         // rustOutSPIRV, rustOutMSL, rustOutHLSL or rustOutGLSL
	file       string         // file name under out/<kind>/
	entryPoint string         // set for per-entry-point outputs
	stage      ir.ShaderStage // stage of entryPoint
}

// rustStageName is Rust's Debug spelling of a shader stage, used in GLSL
// output file names.
func rustStageName(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "Vertex"
	case ir.StageTask:
		return "Task"
	case ir.StageMesh:
		return "Mesh"
	case ir.StageFragment:
		return "Fragment"
	case ir.StageCompute:
		return "Compute"
	default:
		return "ShaderStage(" + strconv.Itoa(int(stage)) + ")"
	}
}

// expectedRustOutputs lists the backend outputs Rust writes for a shader, in
// the driver's order (SPIR-V, MSL, GLSL, HLSL):
//
//   - SPIR-V: wgsl-N.spvasm, or wgsl-N.<ep>.spvasm per entry point when
//     [spv] separate_entry_points is set;
//   - MSL: wgsl-N.metal; HLSL: wgsl-N.hlsl (even without entry points);
//   - GLSL: wgsl-N.<ep>.<Stage>.glsl per entry point not in glsl_exclude_list.
//
// IR snapshots are listed by expectedRustIROutputs.
func expectedRustOutputs(shader string, p rustParams, eps []ir.EntryPoint) []rustOutput {
	prefix := "wgsl-" + shader
	var out []rustOutput
	if p.targets.has(rustTargetSPIRV) {
		if p.separateEntryPoints {
			for _, ep := range eps {
				out = append(out, rustOutput{kind: rustOutSPIRV, file: prefix + "." + ep.Name + ".spvasm",
					entryPoint: ep.Name, stage: ep.Stage})
			}
		} else {
			out = append(out, rustOutput{kind: rustOutSPIRV, file: prefix + ".spvasm"})
		}
	}
	if p.targets.has(rustTargetMETAL) {
		out = append(out, rustOutput{kind: rustOutMSL, file: prefix + ".metal"})
	}
	if p.targets.has(rustTargetGLSL) {
		for _, ep := range eps {
			if p.glslExcludeList[ep.Name] {
				continue
			}
			out = append(out, rustOutput{kind: rustOutGLSL,
				file:       prefix + "." + ep.Name + "." + rustStageName(ep.Stage) + ".glsl",
				entryPoint: ep.Name, stage: ep.Stage})
		}
	}
	if p.targets.has(rustTargetHLSL) {
		out = append(out, rustOutput{kind: rustOutHLSL, file: prefix + ".hlsl"})
	}
	return out
}

// expectedRustIROutputs lists the IR snapshots of a shader: the module after
// the front end and after compaction.
func expectedRustIROutputs(shader string) []string {
	return []string{"wgsl-" + shader + ".ron", "wgsl-" + shader + ".compact.ron"}
}

// isRustAuxiliaryOutput reports files Rust writes next to its outputs that
// our tests do not compare: SPIRV-Cross GLSL of the SPIR-V (a review aid) and
// the DXC profile config of the HLSL. They may be present; they are never
// required.
func isRustAuxiliaryOutput(kind, file string) bool {
	switch kind {
	case rustOutSPIRV:
		return strings.HasSuffix(file, ".spvasm.glsl")
	case rustOutHLSL:
		return strings.HasSuffix(file, ".ron")
	default:
		return false
	}
}

// inventoryGap is a required reference file known to be missing, and the file
// that stands in for it until the gap is closed ("" if none).
type inventoryGap struct {
	substitute string
	reason     string
}

// knownInventoryGaps are required reference files (keyed by path under
// testdata/) known to be missing. Each entry must still be a gap, and its
// substitute must exist — a resolved entry fails the test, so the list only
// shrinks.
var knownInventoryGaps = map[string]inventoryGap{
	// types_with_comments targets IR upstream, but the legacy import did not
	// vendor its snapshots; ours were generated from the older input. The
	// sync to a single upstream commit (TEST-CORPUS-001 phase 4) resolves it.
	"rust/out/ir/wgsl-types_with_comments.ron": {
		substitute: "rust-derived/out/ir/wgsl-types_with_comments.ron",
		reason:     "IR target snapshot not vendored by the legacy import",
	},
	"rust/out/ir/wgsl-types_with_comments.compact.ron": {
		substitute: "rust-derived/out/ir/wgsl-types_with_comments.compact.ron",
		reason:     "IR target snapshot not vendored by the legacy import",
	},
}

// TestRustReferenceInventory checks that the reference trees hold exactly the
// files Rust's driver writes for our corpus, in the right tree:
//
//   - every output a Rust corpus shader's targets produce is vendored;
//   - every Rust corpus shader has IR snapshots: vendored if it targets IR,
//     derived otherwise;
//   - our own shaders may have derived outputs, under the same names;
//   - nothing else is present except Rust's auxiliary files.
//
// A missing file means a sync or naming defect; an extra file means a
// reference no test would ever compare.
func TestRustReferenceInventory(t *testing.T) {
	type location struct{ tree, kind, file string }
	expected := map[location]bool{} // value: required
	addExpected := func(loc location, required bool) {
		if required || !expected[loc] {
			expected[loc] = required
		}
	}

	for _, shader := range loadCorpus(t) {
		module, err := tryCompileToIR(shader.source)
		if err != nil {
			t.Errorf("%s: lowering failed: %v", shader.name, err)
			continue
		}
		params := rustParams{targets: rustTargetsWGSLDefault}
		if shader.origin == originRust {
			if params, err = loadRustParams(shader.name); err != nil {
				t.Errorf("%s: %v", shader.name, err)
				continue
			}
		}
		for _, o := range expectedRustOutputs(shader.name, params, module.EntryPoints) {
			if shader.origin == originRust {
				addExpected(location{rustCorpusDir, o.kind, o.file}, true)
			} else {
				addExpected(location{rustDerivedDir, o.kind, o.file}, false)
			}
		}
		for _, file := range expectedRustIROutputs(shader.name) {
			switch {
			case shader.origin != originRust:
				addExpected(location{rustDerivedDir, rustOutIR, file}, false)
			case params.targets.has(rustTargetIR):
				addExpected(location{rustCorpusDir, rustOutIR, file}, true)
			default:
				addExpected(location{rustDerivedDir, rustOutIR, file}, true)
			}
		}
	}

	present := map[location]bool{}
	for _, tree := range []string{rustCorpusDir, rustDerivedDir} {
		for _, kind := range []string{rustOutSPIRV, rustOutMSL, rustOutHLSL, rustOutGLSL, rustOutIR} {
			entries, err := os.ReadDir(filepath.Join(tree, "out", kind))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				present[location{tree, kind, e.Name()}] = true
			}
		}
	}

	// Compare as paths relative to testdata/ ("rust/out/ir/wgsl-x.ron").
	pathOf := func(loc location) string {
		return strings.TrimPrefix(loc.tree, "testdata/") + "/out/" + loc.kind + "/" + loc.file
	}
	allowed := map[string]bool{}  // may be present
	required := map[string]bool{} // must be present
	for loc, req := range expected {
		p := pathOf(loc)
		allowed[p] = true
		if !req {
			continue
		}
		if gap, ok := knownInventoryGaps[p]; ok {
			if gap.substitute != "" {
				allowed[gap.substitute], required[gap.substitute] = true, true
			}
			continue
		}
		required[p] = true
	}
	var problems []string
	existing := map[string]bool{}
	for loc := range present {
		existing[pathOf(loc)] = true
		if !allowed[pathOf(loc)] && !isRustAuxiliaryOutput(loc.kind, loc.file) {
			problems = append(problems, "unexpected "+pathOf(loc)+" (no test would compare it)")
		}
	}
	for p := range required {
		if !existing[p] {
			problems = append(problems, "missing "+p)
		}
	}
	for p, gap := range knownInventoryGaps {
		if existing[p] || !allowed[p] {
			problems = append(problems, "knownInventoryGaps entry no longer describes a gap, remove it: "+p+" ("+gap.reason+")")
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}
