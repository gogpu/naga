package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// parseSPVCorpus parses one binary of the derived corpus and validates the
// module it produces.
func parseSPVCorpus(t *testing.T, name string) *ir.Module {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rustDerivedDir, "spv", name+".spv"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	module, _, err := spirv.Parse(data, spirv.ParseOptions{})
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("Validate(%s): %v", name, err)
	}
	return module
}

// TestSPVParseBarrier: a compute shader whose body is barriers parses into the
// barrier statements the SPIR-V instructions name.
func TestSPVParseBarrier(t *testing.T) {
	module := parseSPVCorpus(t, "barrier")
	if len(module.EntryPoints) != 1 {
		t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
	}
	ep := module.EntryPoints[0]
	if ep.Stage != ir.StageCompute || ep.Name != "main" {
		t.Errorf("entry point = %v %q, want compute main", ep.Stage, ep.Name)
	}
	if ep.Workgroup != [3]uint32{64, 1, 1} {
		t.Errorf("workgroup = %v, want [64 1 1]", ep.Workgroup)
	}
	if len(module.Functions) != 1 {
		t.Fatalf("functions = %d, want the parsed entry point function", len(module.Functions))
	}
	body := module.Functions[0].Body
	var control, memory int
	for _, statement := range body {
		switch statement.Kind.(type) {
		case ir.StmtControlBarrier:
			control++
		case ir.StmtMemoryBarrier:
			memory++
		}
	}
	if control != 3 || memory != 3 {
		t.Errorf("barriers = %d control, %d memory; want 3 and 3", control, memory)
	}
}

// TestSPVParseEmptyGlobalName: a storage buffer whose name is "" keeps the
// name and round-trips its struct access.
func TestSPVParseEmptyGlobalName(t *testing.T) {
	module := parseSPVCorpus(t, "empty-global-name")
	if len(module.GlobalVariables) != 1 {
		t.Fatalf("globals = %d, want 1", len(module.GlobalVariables))
	}
	global := module.GlobalVariables[0]
	if global.Name != "" {
		t.Errorf("global name = %q, want an empty name", global.Name)
	}
	if global.Space != ir.SpaceStorage {
		t.Errorf("space = %v, want storage", global.Space)
	}
	if global.Binding == nil || *global.Binding != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Errorf("binding = %v, want (0,0)", global.Binding)
	}
}

// TestSPVParseDegrees: the GLSL.std.450 Radians and Degrees instructions
// become the IR's math functions.
func TestSPVParseDegrees(t *testing.T) {
	module := parseSPVCorpus(t, "degrees")
	var radians, degrees int
	for _, function := range module.Functions {
		walkExpressions(function.Expressions, func(expression ir.Expression) {
			math, ok := expression.Kind.(ir.ExprMath)
			if !ok {
				return
			}
			switch math.Fun {
			case ir.MathRadians:
				radians++
			case ir.MathDegrees:
				degrees++
			}
		})
	}
	if radians != 1 || degrees != 1 {
		t.Errorf("radians = %d, degrees = %d; want 1 and 1", radians, degrees)
	}
	if len(module.EntryPoints) != 1 {
		t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
	}
	result := module.EntryPoints[0].Function.Result
	if result == nil || result.Binding == nil {
		t.Fatal("the fragment entry point has no bound result")
	}
	location, ok := (*result.Binding).(ir.LocationBinding)
	if !ok || location.Location != 0 {
		t.Errorf("result binding = %#v, want location 0", result.Binding)
	}
}

// TestSPVRoundTripTrivial compiles a WGSL shader to SPIR-V, parses it back and
// validates the result: the first end-to-end proof of the input path.
func TestSPVRoundTripTrivial(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"empty-compute", `@compute @workgroup_size(1) fn main() {}`},
		{"fragment-color", `@fragment fn main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := wgsl.Parse(tc.source, wgsl.ParseOptions{})
			if err != nil {
				t.Fatalf("wgsl.Parse: %v", err)
			}
			binary, _, err := spirv.Compile(module, spirv.DefaultOptions())
			if err != nil {
				t.Fatalf("spirv.Compile: %v", err)
			}
			parsed, _, err := spirv.Parse(binary, spirv.ParseOptions{})
			if err != nil {
				t.Fatalf("spirv.Parse: %v", err)
			}
			if _, err := valid.Validate(parsed, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("valid.Validate: %v", err)
			}
		})
	}
}

// walkExpressions calls visit for every expression of the arena.
func walkExpressions(expressions []ir.Expression, visit func(ir.Expression)) {
	for _, expression := range expressions {
		visit(expression)
	}
}
