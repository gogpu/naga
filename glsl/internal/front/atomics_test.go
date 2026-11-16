package front

// This file covers the GLSL atomic memory functions (GLSL 4.60 §8.11) and the
// image atomic functions (§8.12). Rust naga's GLSL frontend has neither, so
// every expectation here was measured with glslangValidator and read back with
// spirv-dis; the tests below pin the IR shape the frontend builds, and the
// end-to-end matrix in snapshot/glsl_atomics_matrix_test.go runs each driver
// through glslang, validation, SPIR-V + spirv-val and the text backends.

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// parseAtomicCompute parses a #version 450 compute shader and validates it.
func parseAtomicCompute(t *testing.T, body string) *ir.Module {
	t.Helper()
	source := "#version 450\nlayout(local_size_x = 1) in;\n" + body
	mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
	if err != nil {
		t.Fatalf("Parse: %v\n\nsource:\n%s", err, source)
	}
	if _, verr := valid.Validate(mod, valid.Options{Capabilities: valid.CapAll}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, source)
	}
	return mod
}

// mainFunction returns the module function the entry point wraps.
func mainFunction(t *testing.T, mod *ir.Module) ir.Function {
	t.Helper()
	for _, fn := range mod.Functions {
		if fn.Name == "main" {
			return fn
		}
	}
	t.Fatal("no module function named main")
	return ir.Function{}
}

// collectAtomicStatements returns every StmtAtomic in a function, walking the
// nested blocks a test source may use.
func collectAtomicStatements(fn ir.Function) []ir.StmtAtomic {
	var out []ir.StmtAtomic
	var walk func(block ir.Block)
	walk = func(block ir.Block) {
		for _, stmt := range block {
			switch kind := stmt.Kind.(type) {
			case ir.StmtAtomic:
				out = append(out, kind)
			case ir.StmtBlock:
				walk(kind.Block)
			case ir.StmtIf:
				walk(kind.Accept)
				walk(kind.Reject)
			case ir.StmtLoop:
				walk(kind.Body)
				walk(kind.Continuing)
			case ir.StmtSwitch:
				for _, c := range kind.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(fn.Body)
	return out
}

// collectImageAtomicStatements returns every StmtImageAtomic in a function.
func collectImageAtomicStatements(fn ir.Function) []ir.StmtImageAtomic {
	var out []ir.StmtImageAtomic
	var walk func(block ir.Block)
	walk = func(block ir.Block) {
		for _, stmt := range block {
			switch kind := stmt.Kind.(type) {
			case ir.StmtImageAtomic:
				out = append(out, kind)
			case ir.StmtBlock:
				walk(kind.Block)
			case ir.StmtIf:
				walk(kind.Accept)
				walk(kind.Reject)
			case ir.StmtLoop:
				walk(kind.Body)
				walk(kind.Continuing)
			case ir.StmtSwitch:
				for _, c := range kind.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(fn.Body)
	return out
}

// TestAtomicSharedFamilies drives the eight atomic memory functions on a shared
// uint and, where GLSL has the overload, a shared int, and checks the IR each
// one lowers to: one StmtAtomic with the mapped AtomicFunction, a result
// expression, and the global's type upgraded to an atomic.
func TestAtomicSharedFamilies(t *testing.T) {
	cases := []struct {
		name string
		call string
		want ir.AtomicFunction
	}{
		{"atomicAdd-uint", "atomicAdd(c, 1u)", ir.AtomicAdd{}},
		{"atomicAdd-int", "atomicAdd(i, 1)", ir.AtomicAdd{}},
		{"atomicMin-uint", "atomicMin(c, 1u)", ir.AtomicMin{}},
		{"atomicMin-int", "atomicMin(i, 1)", ir.AtomicMin{}},
		{"atomicMax-uint", "atomicMax(c, 1u)", ir.AtomicMax{}},
		{"atomicMax-int", "atomicMax(i, 1)", ir.AtomicMax{}},
		{"atomicAnd-uint", "atomicAnd(c, 1u)", ir.AtomicAnd{}},
		{"atomicAnd-int", "atomicAnd(i, 1)", ir.AtomicAnd{}},
		{"atomicOr-uint", "atomicOr(c, 1u)", ir.AtomicInclusiveOr{}},
		{"atomicOr-int", "atomicOr(i, 1)", ir.AtomicInclusiveOr{}},
		{"atomicXor-uint", "atomicXor(c, 1u)", ir.AtomicExclusiveOr{}},
		{"atomicXor-int", "atomicXor(i, 1)", ir.AtomicExclusiveOr{}},
		{"atomicExchange-uint", "atomicExchange(c, 1u)", ir.AtomicExchange{}},
		{"atomicExchange-int", "atomicExchange(i, 1)", ir.AtomicExchange{}},
		{"atomicCompSwap-uint", "atomicCompSwap(c, 1u, 2u)", ir.AtomicExchange{Compare: exprPtr(0)}},
		{"atomicCompSwap-int", "atomicCompSwap(i, 1, 2)", ir.AtomicExchange{Compare: exprPtr(0)}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mod := parseAtomicCompute(t, "shared uint c;\nshared int i;\nvoid main() { "+tt.call+"; }\n")
			fn := mainFunction(t, mod)
			stmts := collectAtomicStatements(fn)
			if len(stmts) != 1 {
				t.Fatalf("lowered %d atomic statements, want one", len(stmts))
			}
			stmt := stmts[0]
			if got, want := atomicFunctionName(stmt.Fun), atomicFunctionName(tt.want); got != want {
				t.Errorf("atomic function = %s, want %s", got, want)
			}
			if stmt.Result == nil {
				t.Fatal("the atomic statement has no result expression")
			}
			if _, ok := fn.Expressions[*stmt.Result].Kind.(ir.ExprAtomicResult); !ok {
				t.Errorf("result %d is %T, want ExprAtomicResult", *stmt.Result, fn.Expressions[*stmt.Result].Kind)
			}
		})
	}

	// The accessed variables are the atomics; the one the shader never touches
	// with an atomic stays a plain scalar, which is what makes the upgrade
	// per-variable rather than per-type.
	mod := parseAtomicCompute(t, "shared uint c;\nshared uint untouched;\nvoid main() { atomicAdd(c, 1u); }\n")
	c := findGlobal(t, mod, "c")
	if _, ok := mod.Types[c.Type].Inner.(ir.AtomicType); !ok {
		t.Errorf("the shared variable c is %T, want an AtomicType", mod.Types[c.Type].Inner)
	}
	plain := findGlobal(t, mod, "untouched")
	if got := mod.Types[plain.Type].Inner; got != (ir.ScalarType{Kind: ir.ScalarUint, Width: 4}) {
		t.Errorf("the untouched shared variable is %T, want a plain u32", got)
	}
}

// atomicFunctionName renders an AtomicFunction with its kind, for the table.
func atomicFunctionName(fun ir.AtomicFunction) string {
	switch f := fun.(type) {
	case ir.AtomicAdd:
		return "AtomicAdd"
	case ir.AtomicSubtract:
		return "AtomicSubtract"
	case ir.AtomicAnd:
		return "AtomicAnd"
	case ir.AtomicExclusiveOr:
		return "AtomicExclusiveOr"
	case ir.AtomicInclusiveOr:
		return "AtomicInclusiveOr"
	case ir.AtomicMin:
		return "AtomicMin"
	case ir.AtomicMax:
		return "AtomicMax"
	case ir.AtomicExchange:
		if f.Compare != nil {
			return "AtomicExchangeCompare"
		}
		return "AtomicExchange"
	}
	return "unknown"
}

// TestAtomicCompareSwapResult checks the compare-swap surface: GLSL returns the
// old value alone, while ir.StmtAtomic's compare-exchange result is the
// predeclared {old_value, exchanged} struct, so the frontend extracts member 0.
func TestAtomicCompareSwapResult(t *testing.T) {
	mod := parseAtomicCompute(t, `
shared uint c;
void main() {
    uint old = atomicCompSwap(c, 1u, 2u);
    atomicOr(c, old);
}
`)
	fn := mainFunction(t, mod)
	stmts := collectAtomicStatements(fn)
	if len(stmts) != 2 {
		t.Fatalf("lowered %d atomic statements, want two", len(stmts))
	}
	var swap ir.StmtAtomic
	for _, stmt := range stmts {
		if swapFun, ok := stmt.Fun.(ir.AtomicExchange); ok && swapFun.Compare != nil {
			swap = stmt
		}
	}
	if swap.Result == nil {
		t.Fatal("the compare-swap has no result")
	}
	result, ok := fn.Expressions[*swap.Result].Kind.(ir.ExprAtomicResult)
	if !ok {
		t.Fatalf("the compare-swap result is %T, want ExprAtomicResult", fn.Expressions[*swap.Result].Kind)
	}
	if !result.Comparison {
		t.Error("the compare-swap result is not marked Comparison")
	}
	name := mod.Types[result.Ty].Name
	if name != "__atomic_compare_exchange_result<Uint,4>" {
		t.Errorf("the compare-swap result type is %q, want __atomic_compare_exchange_result<Uint,4>", name)
	}

	// The old value the shader sees is member 0 of that struct.
	found := false
	for _, expr := range fn.Expressions {
		access, ok := expr.Kind.(ir.ExprAccessIndex)
		if ok && access.Base == *swap.Result && access.Index == 0 {
			found = true
		}
	}
	if !found {
		t.Error("the frontend never extracts old_value from the compare-swap result struct")
	}
}

// TestAtomicStorageBufferMembers checks the buffer-block forms: a scalar member
// and a dynamically indexed array member, both upgraded in place.
func TestAtomicStorageBufferMembers(t *testing.T) {
	mod := parseAtomicCompute(t, `
layout(set = 0, binding = 0) buffer B { uint counter; uint values[]; } b;
void main() {
    uint i = gl_GlobalInvocationID.x;
    atomicAdd(b.counter, 1u);
    atomicAdd(b.values[i], 2u);
}
`)
	stmts := collectAtomicStatements(mainFunction(t, mod))
	if len(stmts) != 2 {
		t.Fatalf("lowered %d atomic statements, want two", len(stmts))
	}
	b := findGlobal(t, mod, "b")
	block, ok := mod.Types[b.Type].Inner.(ir.StructType)
	if !ok {
		t.Fatalf("the buffer block is %T, want a struct", mod.Types[b.Type].Inner)
	}
	if _, ok := mod.Types[block.Members[0].Type].Inner.(ir.AtomicType); !ok {
		t.Errorf("counter member is %T, want an AtomicType", mod.Types[block.Members[0].Type].Inner)
	}
	array, ok := mod.Types[block.Members[1].Type].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("the values member is %T, want an array", mod.Types[block.Members[1].Type].Inner)
	}
	if _, ok := mod.Types[array.Base].Inner.(ir.AtomicType); !ok {
		t.Errorf("the values element is %T, want an AtomicType", mod.Types[array.Base].Inner)
	}
	// The stride is the scalar's, so the upgrade leaves the layout alone.
	if array.Stride != 4 {
		t.Errorf("the upgraded array stride is %d, want 4", array.Stride)
	}
}

// TestAtomicCrossFunction checks the upgrade reaches a variable used from more
// than one function: the whole module is lowered before any type changes.
func TestAtomicCrossFunction(t *testing.T) {
	mod := parseAtomicCompute(t, `
shared uint c;
void bump() { atomicAdd(c, 1u); }
void main() { bump(); atomicMin(c, 2u); }
`)
	total := 0
	for _, fn := range mod.Functions {
		total += len(collectAtomicStatements(fn))
	}
	if total != 2 {
		t.Fatalf("lowered %d atomic statements across the module, want two", total)
	}
	c := findGlobal(t, mod, "c")
	if _, ok := mod.Types[c.Type].Inner.(ir.AtomicType); !ok {
		t.Errorf("the shared variable c is %T, want an AtomicType", mod.Types[c.Type].Inner)
	}
	binary, _, err := spirv.Compile(mod, spirv.Options{})
	if err != nil {
		t.Fatalf("spirv.Compile: %v", err)
	}
	if len(binary) == 0 {
		t.Fatal("spirv.Compile produced no binary")
	}
}

// TestAtomicImageStatement checks the image atomic IR: a statement, with the
// coordinate split the image load path uses and the value converted to the
// image format's scalar. A compare-swap keeps its comparator as well: it is
// part of the operation (ir.AtomicExchange.Compare), and an exchange emitted
// without it is a different, but equally valid, instruction.
func TestAtomicImageStatement(t *testing.T) {
	cases := []struct {
		name string
		body string
		fun  string
		// wantValue is the kind of expression the value operand lowered to:
		// a literal of the image's scalar, or an ExprAs for GLSL's implicit
		// int-to-uint conversion.
		wantValue any
		// wantCompare is the literal the comparator lowered to, nil for every
		// operation that takes none.
		wantCompare any
	}{
		{
			name: "uint-2d",
			body: "layout(r32ui, set = 0, binding = 0) uniform uimage2D img;\n" +
				"void main() { imageAtomicAdd(img, ivec2(0), 1u); }\n",
			fun:       "AtomicAdd",
			wantValue: ir.LiteralU32(1),
		},
		{
			name: "int-2d",
			body: "layout(r32i, set = 0, binding = 0) uniform iimage2D img;\n" +
				"void main() { imageAtomicMax(img, ivec2(0), 1); }\n",
			fun:       "AtomicMax",
			wantValue: ir.LiteralI32(1),
		},
		{
			name: "uint-arrayed",
			body: "layout(r32ui, set = 0, binding = 0) uniform uimage2DArray img;\n" +
				"void main() { imageAtomicExchange(img, ivec3(0), 1u); }\n",
			fun:       "AtomicExchange",
			wantValue: ir.LiteralU32(1),
		},
		{
			name: "uint-compare",
			body: "layout(r32ui, set = 0, binding = 0) uniform uimage2D img;\n" +
				"void main() { imageAtomicCompSwap(img, ivec2(0), 1u, 2u); }\n",
			fun:         "AtomicExchangeCompare",
			wantValue:   ir.LiteralU32(2),
			wantCompare: ir.LiteralU32(1),
		},
		{
			name: "uint-converted",
			body: "layout(r32ui, set = 0, binding = 0) uniform uimage2D img;\n" +
				"void main() { uint i = gl_GlobalInvocationID.x; imageAtomicAdd(img, ivec2(0), int(i)); }\n",
			fun:       "AtomicAdd",
			wantValue: ir.ExprAs{Kind: ir.ScalarUint},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mod := parseAtomicCompute(t, tt.body)
			fn := mainFunction(t, mod)
			stmts := collectImageAtomicStatements(fn)
			if len(stmts) != 1 {
				t.Fatalf("lowered %d image atomic statements, want one", len(stmts))
			}
			if got := atomicFunctionName(stmts[0].Fun); got != tt.fun {
				t.Errorf("image atomic function = %s, want %s", got, tt.fun)
			}
			assertAtomicOperand(t, fn, stmts[0].Value, tt.wantValue)
			compare := atomicCompareHandle(stmts[0])
			if tt.wantCompare == nil {
				if compare != nil {
					t.Errorf("the operation carries a comparator (expression %d), want none", *compare)
				}
				return
			}
			if compare == nil {
				t.Fatalf("the operation carries no comparator, want the compare operand %v", tt.wantCompare)
			}
			assertAtomicOperand(t, fn, *compare, tt.wantCompare)
		})
	}
}

// atomicCompareHandle returns the comparator of an image atomic operation, or
// nil when it takes none.
func atomicCompareHandle(stmt ir.StmtImageAtomic) *ir.ExpressionHandle {
	if xchg, ok := stmt.Fun.(ir.AtomicExchange); ok {
		return xchg.Compare
	}
	return nil
}

// assertAtomicOperand checks that h is the expression the test expects: a
// literal with the given value, or an ExprAs of the given scalar kind.
func assertAtomicOperand(t *testing.T, fn ir.Function, h ir.ExpressionHandle, want any) {
	t.Helper()
	switch want := want.(type) {
	case ir.LiteralValue:
		lit, ok := fn.Expressions[h].Kind.(ir.Literal)
		if !ok {
			t.Fatalf("expression %d is %T, want a literal", h, fn.Expressions[h].Kind)
		}
		if lit.Value != want {
			t.Errorf("expression %d is %v, want %v", h, lit.Value, want)
		}
	case ir.ExprAs:
		as, ok := fn.Expressions[h].Kind.(ir.ExprAs)
		if !ok {
			t.Fatalf("expression %d is %T, want ExprAs", h, fn.Expressions[h].Kind)
		}
		if as.Kind != want.Kind {
			t.Errorf("expression %d conversion kind = %v, want %v", h, as.Kind, want.Kind)
		}
	default:
		t.Fatalf("unsupported expectation %T", want)
	}
}

// TestAtomicRefusals pins every deliberate refusal with the text the author
// reads, so a change cannot soften one silently.
func TestAtomicRefusals(t *testing.T) {
	cases := []struct {
		name   string
		stage  ir.ShaderStage
		source string
		want   string
	}{
		{
			name:   "local-variable",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nvoid main() { uint c = 0u; atomicAdd(c, 1u); }\n",
			want:   "Only l-values corresponding to shader block storage or shared variables",
		},
		{
			name:  "uniform-block-member",
			stage: ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\n" +
				"layout(set = 0, binding = 0) uniform U { uint n; } u;\n" +
				"void main() { atomicAdd(u.n, 1u); }\n",
			want: "Only l-values corresponding to shader block storage or shared variables",
		},
		{
			name:   "non-lvalue",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nvoid main() { atomicAdd(1u, 2u); }\n",
			want:   "Expression cannot be in the left hand side",
		},
		{
			name:   "shared-initializer",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nshared uint c = 0u;\nvoid main() { atomicAdd(c, 1u); }\n",
			want:   "an atomic variable cannot have an initializer",
		},
		{
			name:   "value-type-mismatch",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nshared int i;\nvoid main() { atomicAdd(i, 1u); }\n",
			want:   "Unknown function 'atomicAdd'",
		},
		{
			name:   "aggregate-operand",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nshared uint a[2];\nvoid main() { atomicAdd(a, 1u); }\n",
			want:   "Unknown function 'atomicAdd'",
		},
		{
			name:   "vector-component",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nshared uvec4 v;\nvoid main() { atomicAdd(v.x, 1u); }\n",
			want:   "a vector component is not a valid atomic operand",
		},
		{
			name:   "image-as-memory-operand",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nlayout(r32ui, set = 0, binding = 0) uniform uimage2D img;\nvoid main() { atomicAdd(img, 1u); }\n",
			want:   "Unknown function 'atomicAdd'",
		},
		{
			name:   "image-format-rgba8ui",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nlayout(rgba8ui, set = 0, binding = 0) uniform uimage2D img;\nvoid main() { imageAtomicAdd(img, ivec2(0), 1u); }\n",
			want:   "'imageAtomicAdd' : only supported on image with format r32i or r32ui",
		},
		{
			// imageAtomicExchange on r32f is core GLSL and glslang compiles it
			// (`OpAtomicExchange %float`, accepted by spirv-val), so it gets a
			// reason of our own: the IR has no float image atomic. The other
			// functions stay on glslang's format sentence, which is the rule
			// that refuses them (an r32f imageAtomicAdd needs
			// GL_EXT_shader_atomic_float at glslang, an extension this
			// frontend does not implement).
			name:   "image-format-float-exchange",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nlayout(r32f, set = 0, binding = 0) uniform image2D img;\nvoid main() { imageAtomicExchange(img, ivec2(0), 1.0); }\n",
			want:   "an image atomic exchange on a float image (r32f) is not implemented",
		},
		{
			name:   "image-format-float-add",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nlayout(r32f, set = 0, binding = 0) uniform image2D img;\nvoid main() { imageAtomicAdd(img, ivec2(0), 1.0); }\n",
			want:   "'imageAtomicAdd' : only supported on image with format r32i or r32ui",
		},
		{
			name:   "image-sampled-texture",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nlayout(set = 0, binding = 0) uniform sampler2D s;\nvoid main() { imageAtomicAdd(s, ivec2(0), 1u); }\n",
			want:   "Unknown function 'imageAtomicAdd'",
		},
		{
			name:   "image-result-used",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nlayout(r32ui, set = 0, binding = 0) uniform uimage2D img;\nvoid main() { uint old = imageAtomicAdd(img, ivec2(0), 1u); imageAtomicOr(img, ivec2(1), old); }\n",
			want:   "ir.StmtImageAtomic carries no result expression",
		},
		{
			name:   "atomic-counter-function",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(local_size_x = 1) in;\nshared uint c;\nvoid main() { atomicCounterIncrement(c); }\n",
			want:   "atomic counters are not implemented",
		},
		{
			name:   "atomic-counter-type",
			stage:  ir.StageCompute,
			source: "#version 450\nlayout(binding = 0) uniform atomic_uint counter;\nvoid main() { }\n",
			want:   "atomic counters are not implemented",
		},
		{
			name:   "gl330-shared",
			stage:  ir.StageFragment,
			source: "#version 330\nvoid main() { uint x = 0u; atomicAdd(x, 1u); }\n",
			want:   "'atomicAdd' : not supported by GLSL 3.30 core",
		},
		{
			name:   "es300-image",
			stage:  ir.StageFragment,
			source: "#version 300 es\nprecision highp float;\nvoid main() { uint x = 0u; imageAtomicAdd(x, 0, 1u); }\n",
			want:   "'imageAtomicAdd' : not supported by GLSL ES 3.00",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.source, Options{Stage: tt.stage})
			if err == nil {
				t.Fatalf("parsed, want a refusal containing %q\n\nsource:\n%s", tt.want, tt.source)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("refusal is %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}

// TestAtomicStructuralForms checks the variable shapes the upgrade walks: a
// nested struct member inside a buffer block, a shared array with a dynamic
// index, and a plain read and write of a variable that is also atomically
// accessed (all three accepted by glslangValidator).
func TestAtomicStructuralForms(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		verify func(t *testing.T, mod *ir.Module)
	}{
		{
			name: "nested-struct-member",
			body: `
struct Inner { uint x; };
layout(set = 0, binding = 0) buffer B { Inner inner; } b;
void main() { atomicAdd(b.inner.x, 1u); }
`,
			verify: func(t *testing.T, mod *ir.Module) {
				b := findGlobal(t, mod, "b")
				block := mod.Types[b.Type].Inner.(ir.StructType)
				inner := mod.Types[block.Members[0].Type].Inner.(ir.StructType)
				if _, ok := mod.Types[inner.Members[0].Type].Inner.(ir.AtomicType); !ok {
					t.Errorf("the nested member is %T, want an AtomicType", mod.Types[inner.Members[0].Type].Inner)
				}
			},
		},
		{
			name: "shared-array-dynamic-index",
			body: `
shared uint a[4];
void main() { atomicAdd(a[gl_GlobalInvocationID.x], 1u); }
`,
			verify: func(t *testing.T, mod *ir.Module) {
				a := findGlobal(t, mod, "a")
				array, ok := mod.Types[a.Type].Inner.(ir.ArrayType)
				if !ok {
					t.Fatalf("the shared array is %T, want an array", mod.Types[a.Type].Inner)
				}
				if _, ok := mod.Types[array.Base].Inner.(ir.AtomicType); !ok {
					t.Errorf("the array element is %T, want an AtomicType", mod.Types[array.Base].Inner)
				}
				if array.Stride != 4 {
					t.Errorf("the array stride is %d, want 4", array.Stride)
				}
			},
		},
		{
			name: "plain-access-of-an-atomic-variable",
			body: `
shared uint c;
void main() { atomicAdd(c, 1u); c = c + 1u; }
`,
			verify: func(t *testing.T, mod *ir.Module) {
				// The plain read and write become a load and a store through the
				// atomic pointer; the IR allows both, and every backend emits an
				// atomic load or store for them, which is stronger than, never
				// weaker than, the plain access glslang compiles.
				c := findGlobal(t, mod, "c")
				if _, ok := mod.Types[c.Type].Inner.(ir.AtomicType); !ok {
					t.Errorf("the shared variable is %T, want an AtomicType", mod.Types[c.Type].Inner)
				}
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mod := parseAtomicCompute(t, tt.body)
			tt.verify(t, mod)
		})
	}
}

// TestAtomicNameDoesNotShadowUserFunction checks that a shader that declares its
// own function with an atomic name reaches that function, never the builtin —
// in both declaration orders, and for a prototype of a builtin the shader could
// not otherwise call (a float atomicAdd). A declared function is what the
// intercept must see: on db33bc04 it required a definition, so before the
// definition appeared a call went to the atomic path and was lowered as an
// atomic statement (or refused as a bad lvalue).
func TestAtomicNameDoesNotShadowUserFunction(t *testing.T) {
	cases := []struct {
		name   string
		stage  ir.ShaderStage
		source string
		// sharedVariable is the variable the shader's own function is called
		// with in place of an atomic operand; it must keep its scalar type.
		sharedVariable string
	}{
		{
			name:  "definition-before-main",
			stage: ir.StageCompute,
			source: `#version 450
layout(local_size_x = 1) in;
uint atomicAdd(uint a, uint b) { return a + b; }
shared uint c;
void main() { c = atomicAdd(c, 1u); }
`,
			sharedVariable: "c",
		},
		{
			// The prototype is all the frontend has when main is lowered.
			name:  "prototype-before-main",
			stage: ir.StageCompute,
			source: `#version 450
layout(local_size_x = 1) in;
uint atomicAdd(uint a, uint b);
shared uint c;
void main() { c = atomicAdd(c, 1u); }
uint atomicAdd(uint a, uint b) { return a + b; }
`,
			sharedVariable: "c",
		},
		{
			// A float atomicAdd does not exist as a builtin at all, so a call
			// that reached the atomic path could only fail; the shader's own
			// prototype must be the one the call resolves against.
			name:  "float-prototype-before-main",
			stage: ir.StageFragment,
			source: `#version 450
float atomicAdd(float a, float b);
out vec4 o;
void main() { o = vec4(atomicAdd(1.0, 2.0)); }
float atomicAdd(float a, float b) { return a + b; }
`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mod, _, err := Parse(tt.source, Options{Stage: tt.stage})
			if err != nil {
				t.Fatalf("Parse: %v\n\nsource:\n%s", err, tt.source)
			}
			if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, tt.source)
			}
			for _, fn := range mod.Functions {
				if len(collectAtomicStatements(fn)) != 0 {
					t.Fatalf("function %q carries an atomic statement, want none: the shader's own atomicAdd must win", fn.Name)
				}
				if len(collectImageAtomicStatements(fn)) != 0 {
					t.Fatalf("function %q carries an image atomic statement, want none", fn.Name)
				}
			}
			if !callsFunction(t, mod, "main", "atomicAdd") {
				t.Error("main does not call the shader's own atomicAdd")
			}
			if tt.sharedVariable != "" {
				// The variable is only read and written by the user function, so
				// it keeps its scalar type.
				c := findGlobal(t, mod, tt.sharedVariable)
				if _, ok := mod.Types[c.Type].Inner.(ir.AtomicType); ok {
					t.Errorf("the shared variable %s was upgraded although no atomic builtin was called",
						tt.sharedVariable)
				}
			}
		})
	}
}

// callsFunction reports whether the module function named caller has a call of
// the module function named callee.
func callsFunction(t *testing.T, mod *ir.Module, caller, callee string) bool {
	t.Helper()
	var walk func(block ir.Block) bool
	walk = func(block ir.Block) bool {
		for _, stmt := range block {
			switch kind := stmt.Kind.(type) {
			case ir.StmtCall:
				if int(kind.Function) < len(mod.Functions) && mod.Functions[kind.Function].Name == callee {
					return true
				}
			case ir.StmtBlock:
				if walk(kind.Block) {
					return true
				}
			case ir.StmtIf:
				if walk(kind.Accept) || walk(kind.Reject) {
					return true
				}
			case ir.StmtLoop:
				if walk(kind.Body) || walk(kind.Continuing) {
					return true
				}
			case ir.StmtSwitch:
				for _, c := range kind.Cases {
					if walk(c.Body) {
						return true
					}
				}
			}
		}
		return false
	}
	for _, fn := range mod.Functions {
		if fn.Name == caller && walk(fn.Body) {
			return true
		}
	}
	return false
}

// SPIR-V opcodes and operand values, SPIR-V 1.6 §3.32.6 and the Scope and
// Memory Semantics tables.
const (
	opAtomicExchange        = 229
	opAtomicCompareExchange = 230
	opAtomicIAdd            = 234
	opAtomicSMin            = 236
	opAtomicUMin            = 237
	opAtomicUMax            = 239
	opAtomicAnd             = 240
	opAtomicOr              = 241
	opAtomicXor             = 242
)

// atomicInstruction is one decoded atomic instruction: its opcode, the scope
// and equal-semantics operands, and the values of its trailing value and
// comparator operands (the comparator is 0 for every instruction that has
// none).
type atomicInstruction struct {
	opcode    uint32
	scope     uint32
	semantics uint32
	value     uint32
	compare   uint32
}

// decodeAtomicInstructions returns every atomic instruction of a SPIR-V binary
// with its scope and memory-semantics operands resolved through the module's
// OpConstant instructions, as the barrier decoder in builtins_barrier_test.go
// resolves its own operands. The value and comparator operands are resolved the
// same way; every source the tests decode passes them as literals.
func decodeAtomicInstructions(t *testing.T, blob []byte) []atomicInstruction {
	t.Helper()
	if len(blob)%4 != 0 {
		t.Fatalf("SPIR-V binary length %d is not a multiple of 4", len(blob))
	}
	words := make([]uint32, len(blob)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(blob[i*4:])
	}

	constants := make(map[uint32]uint32)
	for i := 5; i < len(words); {
		count := int(words[i] >> 16)
		if count == 0 {
			t.Fatalf("SPIR-V instruction at word %d has zero word count", i)
		}
		if words[i]&0xFFFF == opConstant {
			constants[words[i+2]] = words[i+3]
		}
		i += count
	}
	lookup := func(id uint32) uint32 {
		value, ok := constants[id]
		if !ok {
			t.Fatalf("atomic operand %d is not an OpConstant", id)
		}
		return value
	}

	var out []atomicInstruction
	for i := 5; i < len(words); {
		count := int(words[i] >> 16)
		switch op := words[i] & 0xFFFF; op {
		case opAtomicExchange, opAtomicIAdd, opAtomicSMin, opAtomicUMin, opAtomicUMax,
			opAtomicAnd, opAtomicOr, opAtomicXor:
			out = append(out, atomicInstruction{
				opcode:    op,
				scope:     lookup(words[i+4]),
				semantics: lookup(words[i+5]),
				value:     lookup(words[i+6]),
			})
		case opAtomicCompareExchange:
			// ResultType Result Pointer Scope EqualSemantics UnequalSemantics
			// Value Comparator.
			out = append(out, atomicInstruction{
				opcode:    op,
				scope:     lookup(words[i+4]),
				semantics: lookup(words[i+5]),
				value:     lookup(words[i+7]),
				compare:   lookup(words[i+8]),
			})
		}
		i += count
	}
	return out
}

// TestAtomicSpirvShape pins the SPIR-V the frontend's atomic statements produce,
// and the one glslangValidator produces for the same source: both take Device
// scope and empty (relaxed) memory semantics for a buffer-block or image
// atomic. A workgroup atomic is the one recorded deviation: Rust naga maps
// WorkGroup space to Workgroup scope (back/spv/helpers.rs,
// to_spirv_semantics_and_scope) and glslang uses Device scope; Workgroup scope
// is the narrower, still correct operand, and matching Rust is the floor.
func TestAtomicSpirvShape(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		op    uint32
		scope uint32
		// value and compare are the literals the instruction's operand words
		// resolve to; compare is checked only when hasCompare is set.
		value      uint32
		compare    uint32
		hasCompare bool
	}{
		{
			name: "buffer-member-add",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicAdd(b.n, 1u); }\n",
			op: opAtomicIAdd, scope: scopeDevice,
		},
		{
			name: "buffer-member-max-uint",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicMax(b.n, 1u); }\n",
			op: opAtomicUMax, scope: scopeDevice,
		},
		{
			name: "buffer-member-min-int",
			body: "layout(set = 0, binding = 0) buffer B { int n; } b;\n" +
				"void main() { atomicMin(b.n, 1); }\n",
			op: opAtomicSMin, scope: scopeDevice,
		},
		{
			name: "buffer-member-and",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicAnd(b.n, 1u); }\n",
			op: opAtomicAnd, scope: scopeDevice,
		},
		{
			name: "buffer-member-or",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicOr(b.n, 1u); }\n",
			op: opAtomicOr, scope: scopeDevice,
		},
		{
			name: "buffer-member-xor",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicXor(b.n, 1u); }\n",
			op: opAtomicXor, scope: scopeDevice,
		},
		{
			name: "buffer-member-exchange",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicExchange(b.n, 1u); }\n",
			op: opAtomicExchange, scope: scopeDevice,
		},
		{
			name: "buffer-member-compare-exchange",
			body: "layout(set = 0, binding = 0) buffer B { uint n; } b;\n" +
				"void main() { atomicCompSwap(b.n, 1u, 2u); }\n",
			op: opAtomicCompareExchange, scope: scopeDevice,
			value: 2, compare: 1, hasCompare: true,
		},
		{
			name: "image-atomic-add",
			body: "layout(r32ui, set = 0, binding = 0) uniform uimage2D img;\n" +
				"void main() { imageAtomicAdd(img, ivec2(0), 1u); }\n",
			op: opAtomicIAdd, scope: scopeDevice,
		},
		{
			// The comparator is an operand of the instruction, not of the
			// address computation: imageAtomicCompSwap(img, P, 1u, 2u) must
			// exchange 2u only where the texel already holds 1u.
			name: "image-compare-exchange",
			body: "layout(r32ui, set = 0, binding = 0) uniform uimage2D img;\n" +
				"void main() { imageAtomicCompSwap(img, ivec2(0), 1u, 2u); }\n",
			op: opAtomicCompareExchange, scope: scopeDevice,
			value: 2, compare: 1, hasCompare: true,
		},
		{
			// The recorded deviation: Workgroup scope, Rust's mapping.
			name: "workgroup-min-uint",
			body: "shared uint c;\nvoid main() { atomicMin(c, 1u); }\n",
			op:   opAtomicUMin, scope: scopeWorkgroup,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			source := "#version 450\nlayout(local_size_x = 1) in;\n" + tt.body
			mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, verr := valid.Validate(mod, valid.Options{Capabilities: valid.CapAll}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v", verr)
			}
			blob, _, cerr := spirv.Compile(mod, spirv.Options{})
			if cerr != nil {
				t.Fatalf("spirv.Compile: %v", cerr)
			}
			instructions := decodeAtomicInstructions(t, blob)
			if len(instructions) != 1 {
				t.Fatalf("emitted %d atomic instructions, want one: %v", len(instructions), instructions)
			}
			got := instructions[0]
			if got.opcode != tt.op {
				t.Errorf("opcode = %d, want %d", got.opcode, tt.op)
			}
			if got.scope != tt.scope {
				t.Errorf("scope = %d, want %d", got.scope, tt.scope)
			}
			// Empty memory semantics is relaxed, which is what glslang emits
			// for every one of these calls.
			if got.semantics != 0 {
				t.Errorf("memory semantics = %d, want 0 (relaxed)", got.semantics)
			}
			if tt.hasCompare {
				if got.compare != tt.compare {
					t.Errorf("comparator operand = %d, want %d: the compare-and-swap lost its comparator",
						got.compare, tt.compare)
				}
				if got.value != tt.value {
					t.Errorf("value operand = %d, want %d", got.value, tt.value)
				}
			}
		})
	}
}
