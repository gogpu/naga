package snapshot_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// ronModuleWithNewFields is a minimal Rust IR module using the fields Rust
// added after our IR model (14328f49): GlobalVariable.memory_decorations and
// EntryPoint.incoming_ray_payload.
func ronModuleWithNewFields(decorations, payload string) string {
	return `(
    global_variables: [
        (
            name: Some("v"),
            space: Storage(
                access: ("LOAD | STORE"),
            ),
            binding: Some((
                group: 0,
                binding: 0,
            )),
            ty: 0,
            init: None,
            memory_decorations: (` + decorations + `),
        ),
    ],
    entry_points: [
        (
            name: "main",
            stage: Compute,
            early_depth_test: None,
            workgroup_size: (1, 1, 1),
            workgroup_size_overrides: None,
            function: (
                name: Some("main"),
                arguments: [],
                result: None,
                local_variables: [],
                expressions: [],
                named_expressions: {},
                body: [],
                diagnostic_filter_leaf: None,
            ),
            mesh_info: None,
            task_payload: None,
            incoming_ray_payload: ` + payload + `,
        ),
    ],
)`
}

func TestRONParserNewFields(t *testing.T) {
	mod, err := parseRONModule(ronModuleWithNewFields(`"COHERENT | VOLATILE"`, "Some(0)"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mod.GlobalVariables[0].MemoryDecorations; got != "COHERENT | VOLATILE" {
		t.Errorf("memory_decorations = %q, want %q", got, "COHERENT | VOLATILE")
	}
	if p := mod.EntryPoints[0].IncomingRayPayload; p == nil || *p != 0 {
		t.Errorf("incoming_ray_payload = %v, want Some(0)", p)
	}

	mod, err = parseRONModule(ronModuleWithNewFields(`""`, "None"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mod.GlobalVariables[0].MemoryDecorations; got != "" {
		t.Errorf("memory_decorations = %q, want empty", got)
	}
	if p := mod.EntryPoints[0].IncomingRayPayload; p != nil {
		t.Errorf("incoming_ray_payload = %v, want None", *p)
	}
}

// Rust's cooperative matrix type (14328f49 corpus) has no counterpart in our
// IR; the parser must still read it so the smoke test covers the whole corpus.
func TestRONParserCooperativeMatrix(t *testing.T) {
	mod, err := parseRONModule(`(
    types: [
        (
            name: None,
            inner: CooperativeMatrix(
                columns: Eight,
                rows: Eight,
                scalar: (
                    kind: Float,
                    width: 4,
                ),
                role: A,
            ),
        ),
    ],
)`)
	if err != nil {
		t.Fatal(err)
	}
	if got := mod.Types[0].Inner.Tag; got != "CooperativeMatrix" {
		t.Errorf("type inner tag = %q, want CooperativeMatrix", got)
	}
}

// Our IR has neither field, so a Rust module that sets one cannot match ours:
// the comparison must report it instead of ignoring the field.
func TestCompareIRStructureReportsUnsupportedFields(t *testing.T) {
	ours := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{{Name: "v"}},
		EntryPoints:     []ir.EntryPoint{{Name: "main"}},
	}
	for _, tc := range []struct {
		decorations, payload string
		want                 []string
	}{
		{`""`, "None", nil},
		{`"COHERENT"`, "None", []string{"memory_decorations"}},
		{`""`, "Some(0)", []string{"incoming_ray_payload"}},
	} {
		rust, err := parseRONModule(ronModuleWithNewFields(tc.decorations, tc.payload))
		if err != nil {
			t.Fatal(err)
		}
		var found []string
		for _, d := range compareIRStructure(rust, ours) {
			for _, field := range []string{"memory_decorations", "incoming_ray_payload"} {
				if strings.Contains(d, field) {
					found = append(found, field)
				}
			}
		}
		if strings.Join(found, ",") != strings.Join(tc.want, ",") {
			t.Errorf("decorations %s, payload %s: reported %v, want %v", tc.decorations, tc.payload, found, tc.want)
		}
	}
}
