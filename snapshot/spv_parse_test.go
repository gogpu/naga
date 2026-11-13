package snapshot_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// TestSPVCorpusParseValidate is the corpus-wide gate: every module of the
// derived corpus either parses into IR that validates or is refused with a
// *ParseError (never a panic, never invalid IR). The modules the implemented
// phases cover must parse and validate, so a regression in the decoder, the
// control flow or the wrapper shows here even when a hand-built test misses
// it.
func TestSPVCorpusParseValidate(t *testing.T) {
	phaseOne := map[string]bool{
		"barrier":               true,
		"degrees":               true,
		"empty-global-name":     true,
		"non-semantic-debug":    true,
		"unnamed-gl-per-vertex": true,
		"f16-spv":               true,
		// Phase 1 decodes these as well; they were not pinned before.
		"dual-source-blending":          true,
		"inv-hyperbolic-trig-functions": true,
		"quad-vert":                     true,
		"spec-constants":                true,
		"subgroup-barrier":              true,
		// Phase 2, structured control flow: selections, loops, phis.
		"8151-barrier-reorder":      true,
		"per-vertex":                true,
		"spec-constants-issue-5598": true,
		// Phase 2, function calls and the call graph.
		"builtin-accessed-outside-entrypoint": true,
		"do-while":                            true,
	}
	binaries, err := filepath.Glob(filepath.Join(rustDerivedDir, "spv", "*.spv"))
	if err != nil {
		t.Fatal(err)
	}
	// The corpus is pinned; a module that disappears must fail its pin, not
	// silently leave the sweep.
	const corpusSize = 33
	if len(binaries) != corpusSize {
		t.Fatalf("binaries = %d, want %d: a corpus change updates this test", len(binaries), corpusSize)
	}
	seen := make(map[string]bool, len(binaries))
	for _, path := range binaries {
		name := strings.TrimSuffix(filepath.Base(path), ".spv")
		seen[name] = true
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		module, _, err := spirv.Parse(data, spirv.ParseOptions{})
		if err != nil {
			var parseErr *spirv.ParseError
			if !errors.As(err, &parseErr) {
				t.Errorf("%s: error %v (%T), want a *ParseError", name, err, err)
			}
			if phaseOne[name] {
				t.Errorf("%s: an implemented phase must parse it: %v", name, err)
			}
			t.Logf("%s: refused: %v", name, err)
			continue
		}
		if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
			t.Errorf("%s: parses but does not validate: %v", name, err)
		}
	}
	for name := range phaseOne {
		if !seen[name] {
			t.Errorf("%s: the Phase 1 module is missing from the corpus", name)
		}
	}
}

// TestSPVParseCallOrder: main calls a function defined after it. Rust naga
// orders Module.Functions callees first, so its outputs for these modules
// (tests/out/{msl,hlsl}/spv-do-while.*) write the callee before main, and
// every call names a function earlier in the arena.
func TestSPVParseCallOrder(t *testing.T) {
	for name, want := range map[string][]string{
		"do-while":                            {"f(b1;", "main"},
		"builtin-accessed-outside-entrypoint": {"builtin_usage(", "main"},
	} {
		t.Run(name, func(t *testing.T) {
			module := parseSPVCorpus(t, name)
			var got []string
			for i := range module.Functions {
				got = append(got, module.Functions[i].Name)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("functions = %q, want %q", got, want)
			}
			for i := range module.Functions {
				for _, statement := range module.Functions[i].Body {
					if call, ok := statement.Kind.(ir.StmtCall); ok && int(call.Function) >= i {
						t.Errorf("%s calls function %d, not before it", module.Functions[i].Name, call.Function)
					}
				}
			}
			if len(module.EntryPoints) != 1 {
				t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
			}
		})
	}
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

// TestSPVParseBarrierReorder: gfx-rs/wgpu#8151. The workgroup load between
// two barriers must stay between them, after the first selection, as in Rust
// naga's output (tests/out/wgsl/spv-8151-barrier-reorder.wgsl):
//
//	if _e8 { global_2 = 1u; }
//	workgroupBarrier(); let _e9 = global_2; workgroupBarrier();
//	global_1.member[_e6.x] = _e9;
//	if _e8 { global_2 = 2u; }
//	return;
func TestSPVParseBarrierReorder(t *testing.T) {
	module := parseSPVCorpus(t, "8151-barrier-reorder")
	fn := module.Functions[0]
	var kinds []string
	for _, statement := range fn.Body {
		switch kind := statement.Kind.(type) {
		case ir.StmtEmit:
			loads := 0
			for h := kind.Range.Start; h < kind.Range.End; h++ {
				if _, ok := fn.Expressions[h].Kind.(ir.ExprLoad); ok {
					loads++
				}
			}
			kinds = append(kinds, "emit"+strings.Repeat("+load", loads))
		case ir.StmtIf:
			kinds = append(kinds, "if")
		case ir.StmtControlBarrier:
			kinds = append(kinds, "barrier")
		case ir.StmtStore:
			kinds = append(kinds, "store")
		case ir.StmtReturn:
			kinds = append(kinds, "return")
		default:
			kinds = append(kinds, "other")
		}
	}
	got := strings.Join(kinds, " ")
	const want = "emit+load if barrier emit+load barrier emit store if return"
	if got != want {
		t.Errorf("statements = %s\nwant         %s", got, want)
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

// TestSPVParsePerVertexCulling: glslang declares gl_PointSize, gl_ClipDistance
// and gl_CullDistance on gl_PerVertex even when a shader never writes them.
// Rust naga records the built-in members an OpAccessChain reaches and drops
// the binding of every other one in the wrapper result (front/spv/function.rs,
// process_entry_point, and next_block.rs, gl_per_vertex_builtin_access), so the
// spec-constants wrapper returns 2 members where the module declares 5.
func TestSPVParsePerVertexCulling(t *testing.T) {
	module := parseSPVCorpus(t, "spec-constants")
	if len(module.EntryPoints) != 1 {
		t.Fatalf("entry points = %d, want 1", len(module.EntryPoints))
	}
	result := module.EntryPoints[0].Function.Result
	if result == nil {
		t.Fatal("the vertex entry point has no result")
	}
	structType, ok := module.Types[result.Type].Inner.(ir.StructType)
	if !ok {
		t.Fatalf("result type = %T, want a struct", module.Types[result.Type].Inner)
	}
	if len(structType.Members) != 2 {
		t.Fatalf("result members = %d, want 2 (v_Uv and gl_Position)", len(structType.Members))
	}
	location, ok := (*structType.Members[0].Binding).(ir.LocationBinding)
	if !ok || location.Location != 0 {
		t.Errorf("member 0 binding = %#v, want location 0", structType.Members[0].Binding)
	}
	builtin, ok := (*structType.Members[1].Binding).(ir.BuiltinBinding)
	if !ok || builtin.Builtin != ir.BuiltinPosition {
		t.Errorf("member 1 binding = %#v, want the position built-in", structType.Members[1].Binding)
	}
	// The module's gl_PerVertex type keeps every member; the culled ones lose
	// their binding, as Rust naga rewrites the type in place.
	perVertex := 0
	for _, ty := range module.Types {
		inner, ok := ty.Inner.(ir.StructType)
		if !ok || ty.Name != "gl_PerVertex" {
			continue
		}
		perVertex++
		for index, want := range []ir.BuiltinValue{
			ir.BuiltinPosition, ir.BuiltinPointSize, ir.BuiltinClipDistances, ir.BuiltinCullDistance,
		} {
			binding := inner.Members[index].Binding
			if want == ir.BuiltinPosition {
				if binding == nil {
					t.Errorf("gl_PerVertex member %d lost its binding", index)
				}
				continue
			}
			if binding != nil {
				t.Errorf("gl_PerVertex member %d keeps binding %#v, want none", index, binding)
			}
		}
	}
	if perVertex != 1 {
		t.Fatalf("gl_PerVertex types = %d, want 1", perVertex)
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
		// Structured control flow (Phase 2): our SPIR-V writer's selections,
		// loops, switches and break-ifs read back into IR that validates.
		{"if-else", `@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
			var c = vec4<f32>(0.0);
			if p.x > 1.0 { c.x = 1.0; } else if p.y > 2.0 { c.y = 1.0; } else { return c; }
			return c;
		}`},
		{"for-loop", `@compute @workgroup_size(1) fn main() {
			var s = 0;
			for (var i = 0; i < 10; i++) {
				if i == 3 { continue; }
				if i == 7 { break; }
				s += i;
			}
		}`},
		{"loop-break-if", `@compute @workgroup_size(1) fn main() {
			var i = 0;
			loop { i += 1; continuing { break if i >= 4; } }
		}`},
		{"switch", `@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) x: u32) {
			var s = 0;
			switch x {
				case 0u, 1u: { s = 1; }
				case 2u: { s = 2; return; }
				default: { s = 3; }
			}
		}`},
		{"nested", `@compute @workgroup_size(1) fn main() {
			var s = 0;
			for (var i = 0; i < 4; i++) {
				var j = 0;
				while j < i {
					switch j { case 1: { s += 1; } default: { s -= 1; } }
					j++;
				}
			}
		}`},
		// Function calls (Phase 2): results, arguments, a pointer argument
		// and a call chain whose callees come after their callers.
		{"calls", `fn twice(x: i32) -> i32 { return add(x, x); }
		fn add(a: i32, b: i32) -> i32 { return a + b; }
		fn bump(p: ptr<function, i32>) { *p = twice(*p); }
		@compute @workgroup_size(1) fn main() {
			var s = twice(3);
			bump(&s);
			if s > 4 { bump(&s); }
		}`},
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

// TestSPVRoundTripCorpus sends every Rust WGSL corpus shader our SPIR-V
// writer compiles back through the frontend: each module either parses into
// IR that validates or is refused with a *ParseError, never a panic and never
// invalid IR. minParsed only rises as the frontend phases land; Phase 2 (calls
// and control flow) brought it from 55 to 91.
func TestSPVRoundTripCorpus(t *testing.T) {
	const minParsed = 91
	sources, err := filepath.Glob(filepath.Join(rustInputDir, "*.wgsl"))
	if err != nil {
		t.Fatal(err)
	}
	parsed := 0
	for _, path := range sources {
		name := strings.TrimSuffix(filepath.Base(path), ".wgsl")
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		module, _, err := wgsl.Parse(string(source), wgsl.ParseOptions{})
		if err != nil {
			continue
		}
		binary, _, err := spirv.Compile(module, spirv.DefaultOptions())
		if err != nil {
			continue
		}
		back, _, err := spirv.Parse(binary, spirv.ParseOptions{})
		if err != nil {
			var parseErr *spirv.ParseError
			if !errors.As(err, &parseErr) {
				t.Errorf("%s: error %v (%T), want a *ParseError", name, err, err)
			}
			continue
		}
		if _, err := valid.Validate(back, valid.Options{Capabilities: valid.CapAll}); err != nil {
			t.Errorf("%s: parses into invalid IR: %v", name, err)
			continue
		}
		parsed++
	}
	if parsed < minParsed {
		t.Errorf("parsed and validated %d modules, want at least %d", parsed, minParsed)
	}
	t.Logf("%d of %d shaders round-trip", parsed, len(sources))
}

// walkExpressions calls visit for every expression of the arena.
func walkExpressions(expressions []ir.Expression, visit func(ir.Expression)) {
	for _, expression := range expressions {
		visit(expression)
	}
}
