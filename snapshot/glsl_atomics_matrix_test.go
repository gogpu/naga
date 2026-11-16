package snapshot_test

// The atomic matrix of the GLSL frontend (card P1-GLSLIN-015): the atomicAdd
// family on shared variables and buffer-block members (GLSL 4.60 §8.11) and the
// imageAtomic* family on storage images (§8.12), each end to end.
//
// An "ok" row must parse, validate, compile to SPIR-V that spirv-val accepts,
// and compile through the MSL and HLSL backends; before any of that its source
// is compiled by glslangValidator when the tool is installed, so a row cannot
// assert support for a form the reference GLSL compiler rejects. A "refuse" row
// must fail with a *glsl.ParseError whose message carries the recorded text,
// and its reason names what justifies the refusal.
//
// Every name atomicBuiltinFunction (glsl/internal/front/atomics.go) handles has
// a row here, and TestGLSLAtomicsMatrixCoverage reads that switch's own source
// to check it, so a builtin added to the frontend fails the test until a row
// drives it. The IR shape, the SPIR-V opcodes, the scopes and the semantics of
// these forms are pinned in glsl/internal/front/atomics_test.go; this matrix is
// the end-to-end gate.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
)

// glslAtomicsMatrix is the matrix: the eight atomic memory functions on a
// shared variable and on buffer-block members, the eight image atomic functions
// on the integer image formats, and the refusals.
var glslAtomicsMatrix = []glslMatrixRow{
	// --- the atomic memory functions on shared variables --------------------

	{
		name:      "atomic-add-shared-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		source: `#version 450
layout(local_size_x = 1) in;
shared uint counter;
void main() { atomicAdd(counter, 1u); }
`,
	},
	{
		name:      "atomic-min-shared-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicMin"},
		source: `#version 450
layout(local_size_x = 1) in;
shared int value;
void main() { atomicMin(value, 1); }
`,
	},
	{
		name:      "atomic-max-shared-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicMax"},
		source: `#version 450
layout(local_size_x = 1) in;
shared uint value;
void main() { atomicMax(value, 1u); }
`,
	},
	{
		name:      "atomic-and-shared-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAnd"},
		// GLSL 4.60 §8.11 gives atomicAnd an int overload as well as a uint
		// one; glslang compiles both.
		source: `#version 450
layout(local_size_x = 1) in;
shared int value;
void main() { atomicAnd(value, 1); }
`,
	},
	{
		name:      "atomic-or-shared-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicOr"},
		source: `#version 450
layout(local_size_x = 1) in;
shared uint value;
void main() { atomicOr(value, 1u); }
`,
	},
	{
		name:      "atomic-xor-shared-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicXor"},
		source: `#version 450
layout(local_size_x = 1) in;
shared int value;
void main() { atomicXor(value, 1); }
`,
	},
	{
		name:      "atomic-exchange-shared-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicExchange"},
		source: `#version 450
layout(local_size_x = 1) in;
shared uint value;
void main() { atomicExchange(value, 1u); }
`,
	},
	{
		name:      "atomic-compare-swap-shared-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicCompSwap"},
		source: `#version 450
layout(local_size_x = 1) in;
shared int value;
void main() { atomicCompSwap(value, 1, 2); }
`,
	},
	{
		name:      "atomic-old-value-used",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd", "atomicOr"},
		// The old value every atomic returns is an ordinary value: the first
		// one feeds the second call, so the result expression is exercised.
		source: `#version 450
layout(local_size_x = 1) in;
shared uint counter;
void main() {
    uint old = atomicAdd(counter, 1u);
    atomicOr(counter, old);
}
`,
	},
	{
		name:      "atomic-implicit-conversion",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		// GLSL's implicit int-to-uint conversion applies to the value operand,
		// through an explicit constructor here; the row below covers the
		// implicit form, which glslang accepts too.
		source: `#version 450
layout(local_size_x = 64) in;
shared uint counter;
void main() {
    uint i = gl_GlobalInvocationID.x;
    atomicAdd(counter, int(i));
}
`,
	},
	{
		name:      "atomic-implicit-int-literal",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		// A bare int literal as a uint atomic's operand: the implicit
		// conversion the spec's implicit-conversion rules allow, measured with
		// glslangValidator 11:16.2.0 (`atomicAdd(counter, 1);` compiles,
		// `atomicAdd(int_counter, 1u);` does not).
		source: `#version 450
layout(local_size_x = 1) in;
shared uint counter;
void main() { atomicAdd(counter, 1); }
`,
	},
	{
		name:      "atomic-buffer-member",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd", "atomicMin"},
		// A buffer-block member and a dynamically indexed array member, the
		// two storage forms GLSL 4.60 §8.11 allows besides shared memory.
		source: `#version 450
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Data {
    uint counter;
    uint values[];
} data;
void main() {
    uint i = gl_GlobalInvocationID.x;
    atomicAdd(data.counter, 1u);
    atomicMin(data.values[i], 7u);
}
`,
	},
	{
		name:      "atomic-helper-function",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd", "atomicMax"},
		// The accessed variable is upgraded for the whole module, so an atomic
		// in a helper function and one in main share the type.
		source: `#version 450
layout(local_size_x = 1) in;
shared uint counter;
void bump() { atomicAdd(counter, 1u); }
void main() {
    bump();
    atomicMax(counter, 2u);
}
`,
	},

	// --- the image atomic functions -----------------------------------------

	{
		name:      "image-atomic-add-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicAdd"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2D img;
void main() { imageAtomicAdd(img, ivec2(0), 1u); }
`,
	},
	{
		name:      "image-atomic-min-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicMin"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32i, set = 0, binding = 0) uniform iimage2D img;
void main() { imageAtomicMin(img, ivec2(0), 1); }
`,
	},
	{
		name:      "image-atomic-max-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicMax"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2D img;
void main() { imageAtomicMax(img, ivec2(0), 1u); }
`,
	},
	{
		name:      "image-atomic-and-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicAnd"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32i, set = 0, binding = 0) uniform iimage2D img;
void main() { imageAtomicAnd(img, ivec2(0), 1); }
`,
	},
	{
		name:      "image-atomic-or-uint",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicOr"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2D img;
void main() { imageAtomicOr(img, ivec2(0), 1u); }
`,
	},
	{
		name:      "image-atomic-xor-int",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicXor"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32i, set = 0, binding = 0) uniform iimage2D img;
void main() { imageAtomicXor(img, ivec2(0), 1); }
`,
	},
	{
		name:      "image-atomic-exchange-arrayed",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicExchange"},
		// An arrayed image takes (x, y, layer) as its coordinate; the frontend
		// splits the layer out the way imageLoad does.
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2DArray img;
void main() { imageAtomicExchange(img, ivec3(0), 1u); }
`,
	},
	{
		name:      "image-atomic-compare-swap",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicCompSwap"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2D img;
void main() { imageAtomicCompSwap(img, ivec2(0), 1u, 2u); }
`,
	},
	{
		name:      "image-atomic-1d",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicAdd"},
		// A 1D image takes a scalar coordinate; the split handles it.
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage1D img;
void main() { imageAtomicAdd(img, 0, 1u); }
`,
	},
	{
		name:      "image-atomic-3d",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicMax"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32i, set = 0, binding = 0) uniform iimage3D img;
void main() { imageAtomicMax(img, ivec3(0), 1); }
`,
	},
	{
		name:      "image-and-buffer-atomics",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd", "imageAtomicAdd", "atomicOr", "imageAtomicOr"},
		// Both families in one shader, on a buffer member and an image texel.
		source: `#version 450
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Data { uint count; } data;
layout(r32ui, set = 0, binding = 1) uniform uimage2D img;
void main() {
    uint i = gl_GlobalInvocationID.x;
    imageAtomicOr(img, ivec2(int(i), 0), 1u);
    atomicAdd(data.count, 1u);
}
`,
	},

	{
		name:      "atomic-nested-member",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		// The upgrade walks a nested struct member as well as a direct one.
		source: `#version 450
layout(local_size_x = 1) in;
struct Inner { uint x; };
layout(set = 0, binding = 0) buffer B { Inner inner; } b;
void main() { atomicAdd(b.inner.x, 1u); }
`,
	},
	{
		name:      "atomic-shared-array",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		// A shared array with a dynamic index; the element type becomes atomic.
		source: `#version 450
layout(local_size_x = 64) in;
shared uint bins[64];
void main() { atomicAdd(bins[gl_GlobalInvocationID.x], 1u); }
`,
	},
	{
		name:      "atomic-plain-access-mixed",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		// A plain read and write of a variable that is also atomically accessed:
		// the variable becomes atomic for the whole module, so the backends emit
		// an atomic load and store for these, which is stronger than — never
		// weaker than — the plain access glslang compiles.
		source: `#version 450
layout(local_size_x = 1) in;
shared uint counter;
void main() {
    atomicAdd(counter, 1u);
    counter = counter + 1u;
}
`,
	},

	// --- refusals ------------------------------------------------------------

	{
		name:      "refuse-local-variable",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		refuse:    "Only l-values corresponding to shader block storage or shared variables",
		reason: "GLSL 4.60 §8.11 restricts every atomic memory function to a shared variable or a buffer-block " +
			"member; glslangValidator rejects the same source with the same sentence",
		source: `#version 450
layout(local_size_x = 1) in;
void main() { uint counter = 0u; atomicAdd(counter, 1u); }
`,
	},
	{
		name:      "refuse-uniform-block-member",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		refuse:    "Only l-values corresponding to shader block storage or shared variables",
		reason:    "a uniform block is read-only storage; glslangValidator reports the same l-value rule for it",
		source: `#version 450
layout(local_size_x = 1) in;
layout(set = 0, binding = 0) uniform Limits { uint n; } limits;
void main() { atomicAdd(limits.n, 1u); }
`,
	},
	{
		name:      "refuse-value-type-mismatch",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		refuse:    "Unknown function 'atomicAdd'",
		reason: "the value must convert to the variable's type, and GLSL has no implicit uint-to-int " +
			"conversion; glslangValidator reports 'no matching overloaded function found'",
		source: `#version 450
layout(local_size_x = 1) in;
shared int counter;
void main() { atomicAdd(counter, 1u); }
`,
	},
	{
		name:      "refuse-image-format",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicAdd"},
		refuse:    "only supported on image with format r32i or r32ui",
		reason: "GLSL 4.60 §8.12 defines the image atomics on integer images whose format is r32i or " +
			"r32ui alone; glslangValidator prints the same sentence for an rgba8ui image",
		source: `#version 450
layout(local_size_x = 1) in;
layout(rgba8ui, set = 0, binding = 0) uniform uimage2D img;
void main() { imageAtomicAdd(img, ivec2(0), 1u); }
`,
	},
	{
		name:      "refuse-image-result-used",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicAdd", "imageAtomicOr"},
		refuse:    "ir.StmtImageAtomic carries no result expression",
		reason: "GLSL's image atomics return the value they replaced, but ir.StmtImageAtomic has no result " +
			"expression (the WGSL texture atomics are statements); the frontend supports the statement form " +
			"and refuses the value, rather than inventing an IR expression (card P1-GLSLIN-015)",
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2D img;
void main() {
    uint old = imageAtomicAdd(img, ivec2(0), 1u);
    imageAtomicOr(img, ivec2(1), old);
}
`,
	},
	{
		name:      "refuse-image-float-exchange",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageAtomicExchange"},
		refuse:    "an image atomic exchange on a float image (r32f) is not implemented",
		reason: "imageAtomicExchange is core GLSL on an r32f image and glslangValidator accepts it " +
			"(OpAtomicExchange %float, accepted by spirv-val), so the refusal is the IR's: it has no float " +
			"image atomic, and MSL and HLSL cannot write a float texture atomically (card P1-GLSLIN-015). " +
			"glslang's integer-format sentence would be false for this form and is kept for the others",
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32f, set = 0, binding = 0) uniform image2D img;
void main() { imageAtomicExchange(img, ivec2(0), 1.0); }
`,
	},
	{
		name:      "refuse-vector-component",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"atomicAdd"},
		refuse:    "a vector component is not a valid atomic operand",
		reason: "glslangValidator accepts atomicAdd on a component of a shared uvec4, but ir.StmtAtomic's " +
			"pointer must point to an ir.AtomicType and a vector has no atomic element type, so the IR " +
			"cannot express it without a new type (card P1-GLSLIN-015)",
		source: `#version 450
layout(local_size_x = 1) in;
shared uvec4 lanes;
void main() { atomicAdd(lanes.x, 1u); }
`,
	},
	{
		name:      "refuse-atomic-counter-function",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "atomic counters are not implemented",
		reason: "the IR has no atomic-counter resource class, so `layout(binding) uniform atomic_uint` " +
			"and its atomicCounter* functions cannot be represented; the card's own decision refuses them " +
			"rather than inventing a resource model (card P1-GLSLIN-015)",
		source: `#version 450
layout(local_size_x = 1) in;
shared uint counter;
void main() { atomicCounterIncrement(counter); }
`,
	},
	{
		name:      "refuse-atomic-counter-type",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "atomic counters are not implemented",
		reason: "an atomic_uint declaration is the atomic counter itself; the same missing resource class " +
			"is reported at the type (card P1-GLSLIN-015)",
		source: `#version 450
layout(local_size_x = 1) in;
layout(binding = 0) uniform atomic_uint counter;
void main() { }
`,
	},
	{
		name:      "refuse-gl330-atomic",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'atomicAdd' : not supported by GLSL 3.30 core",
		reason: "GLSL 4.30 introduced the atomic memory functions; glslangValidator in 330 core reports " +
			"'atomicAdd' : no matching overloaded function found",
		source: `#version 330
void main() { uint counter = 0u; atomicAdd(counter, 1u); }
`,
	},
	{
		name:      "refuse-es300-image-atomic",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'imageAtomicAdd' : not supported by GLSL ES 3.00",
		reason: "the image atomic functions are GLSL 4.20 / ES 3.10; ES 3.00 has neither the image types " +
			"nor the functions (design §15)",
		source: `#version 300 es
precision highp float;
void main() { uint counter = 0u; imageAtomicAdd(counter, 0, 1u); }
`,
	},
}

// glslAtomicBuiltinNames reads the atomic builtin names out of
// atomicBuiltinFunction's switch in glsl/internal/front/atomics.go. The scan is
// the point: a name added to the catalog fails the coverage test until a row
// drives it.
func glslAtomicBuiltinNames(t *testing.T) []string {
	t.Helper()
	const path = "../glsl/internal/front/atomics.go"
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "atomicBuiltinFunction" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, clause := range sw.Body.List {
				cc, ok := clause.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range cc.List {
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if name, uerr := strconv.Unquote(lit.Value); uerr == nil {
						names = append(names, name)
					}
				}
			}
			return false
		})
		return false
	})
	if len(names) == 0 {
		t.Fatalf("no builtin names found in atomicBuiltinFunction in %s; its switch shape changed, update this scan", path)
	}
	sort.Strings(names)
	return names
}

// TestGLSLAtomicsMatrix runs every row through the frontend and, for an ok row,
// through the reference GLSL compiler and all the backends.
func TestGLSLAtomicsMatrix(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)

	for _, row := range glslAtomicsMatrix {
		t.Run(row.name, func(t *testing.T) {
			if row.refuse == "" {
				checkGLSLSourceWithGLSLang(t, glslang, haveGLSLang, row)
			}

			module, _, err := glsl.Parse(row.source, glsl.ParseOptions{Stage: row.stage})
			if row.refuse != "" {
				if err == nil {
					t.Fatalf("parsed, want a refusal containing %q (%s)\n\nsource:\n%s",
						row.refuse, row.reason, row.source)
				}
				var parseErr *glsl.ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("error is %T, want *glsl.ParseError: %v", err, err)
				}
				if !strings.Contains(parseErr.Message, row.refuse) {
					t.Fatalf("refusal is %q, want it to contain %q\nreason: %s\n\nsource:\n%s",
						parseErr.Message, row.refuse, row.reason, row.source)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v\n\nsource:\n%s", err, row.source)
			}
			if verr := validateGLSL(t, module); verr != nil {
				t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, row.source)
			}
			if step, cerr := compileGLSLModule(t, row.name, module); cerr != nil {
				t.Fatalf("%s step: %v\n\nsource:\n%s", step, cerr, row.source)
			}
			// The frontend's own output goes back through the GLSL backend at
			// Vulkan GLSL's version, where these compute-only forms exist: the
			// round trip catches a form the writer cannot express (a compare
			// exchange, say) before a consumer meets it.
			if _, _, gerr := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450}); gerr != nil {
				t.Fatalf("glsl backend round trip: %v\n\nsource:\n%s", gerr, row.source)
			}
		})
	}
}

// TestGLSLAtomicsMatrixCoverage audits the matrix itself: every row states
// exactly one outcome, every builtin it names is one atomicBuiltinFunction
// handles, and every builtin that switch handles has at least one row.
func TestGLSLAtomicsMatrixCoverage(t *testing.T) {
	names := glslAtomicBuiltinNames(t)
	catalog := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, duplicate := catalog[name]; duplicate {
			t.Errorf("atomicBuiltinFunction's switch lists %q twice", name)
		}
		catalog[name] = struct{}{}
	}

	covered := make(map[string]string, len(catalog))
	seen := make(map[string]struct{}, len(glslAtomicsMatrix))
	for _, row := range glslAtomicsMatrix {
		if _, duplicate := seen[row.name]; duplicate {
			t.Errorf("two matrix rows are named %q", row.name)
		}
		seen[row.name] = struct{}{}

		switch {
		case row.refuse == "" && row.reason != "":
			t.Errorf("%s: states no refusal but records a reason: %s", row.name, row.reason)
		case row.refuse != "" && row.reason == "":
			t.Errorf("%s: refuses with %q but records no reason", row.name, row.refuse)
		}
		wantStage, known := glslStageByExtension[row.extension]
		if !known {
			t.Errorf("%s: extension %q is not .vert, .frag or .comp", row.name, row.extension)
		} else if row.stage != wantStage {
			t.Errorf("%s: stage %v does not match the source's %q extension", row.name, row.stage, row.extension)
		}

		for _, name := range row.builtins {
			if _, known := catalog[name]; !known {
				t.Errorf("%s: drives %q, which atomicBuiltinFunction does not handle", row.name, name)
				continue
			}
			if _, taken := covered[name]; !taken {
				covered[name] = row.name
			}
		}
	}

	for _, name := range names {
		if _, ok := covered[name]; !ok {
			t.Errorf("%s is handled by atomicBuiltinFunction but no matrix row drives it; add a row to glslAtomicsMatrix", name)
		}
	}
}
