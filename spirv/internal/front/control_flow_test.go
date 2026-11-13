package front

import (
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/valid"
)

// The structured control flow tests build functions on bodyModule (ids in its
// doc comment; the function's first block is 101, and the builder ends the
// last open block with OpReturn). Every accepted module must also validate:
// the frontend never hands out invalid IR for structured input.

const fnSpace = uint32(spvenums.StorageClassFunction)

// parseValid parses the function and validates the module it belongs to.
func parseValid(t *testing.T, m *bodyModule, body ...[]uint32) *ir.Function {
	t.Helper()
	module, _, err := Parse(m.build(body...), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return &module.Functions[0]
}

// stripEmits drops the Emit statements, leaving the control-flow shape.
func stripEmits(block ir.Block) ir.Block {
	var out ir.Block
	for _, statement := range block {
		if _, ok := statement.Kind.(ir.StmtEmit); !ok {
			out = append(out, statement)
		}
	}
	return out
}

// shape renders the control-flow skeleton of a block, so a test can state
// the structure it expects in one line.
func shape(block ir.Block) string {
	parts := make([]string, 0, len(block))
	for _, statement := range stripEmits(block) {
		switch kind := statement.Kind.(type) {
		case ir.StmtIf:
			parts = append(parts, "if{"+shape(kind.Accept)+"}else{"+shape(kind.Reject)+"}")
		case ir.StmtLoop:
			s := "loop{" + shape(kind.Body) + "}continuing{" + shape(kind.Continuing) + "}"
			if kind.BreakIf != nil {
				s += "breakif"
			}
			parts = append(parts, s)
		case ir.StmtSwitch:
			cases := make([]string, 0, len(kind.Cases))
			for _, c := range kind.Cases {
				label := "default"
				if v, ok := c.Value.(ir.SwitchValueI32); ok {
					label = itoaTest(uint32(v))
				}
				if c.FallThrough {
					label += "+"
				}
				cases = append(cases, label+":{"+shape(c.Body)+"}")
			}
			parts = append(parts, "switch{"+strings.Join(cases, " ")+"}")
		case ir.StmtBreak:
			parts = append(parts, "break")
		case ir.StmtContinue:
			parts = append(parts, "continue")
		case ir.StmtReturn:
			parts = append(parts, "return")
		case ir.StmtStore:
			parts = append(parts, "store")
		default:
			parts = append(parts, "stmt")
		}
	}
	return strings.Join(parts, ";")
}

func TestControlFlowSelection(t *testing.T) {
	cases := []struct {
		name string
		body [][]uint32
		want string
	}{
		{
			name: "if without else",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 103, 0),
				instWords(spvenums.OpBranchConditional, 24, 102, 103),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
			},
			want: "if{}else{};return",
		},
		{
			name: "if with else",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 104, 0),
				instWords(spvenums.OpBranchConditional, 24, 102, 103),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpBranch, 104),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranch, 104),
				instWords(spvenums.OpLabel, 104),
			},
			want: "if{}else{};return",
		},
		{
			name: "both targets the same block",
			body: [][]uint32{
				instWords(spvenums.OpBranchConditional, 24, 102, 102),
				instWords(spvenums.OpLabel, 102),
			},
			want: "return",
		},
		{
			name: "an arm that returns",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 103, 0),
				instWords(spvenums.OpBranchConditional, 24, 102, 103),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpReturn),
				instWords(spvenums.OpLabel, 103),
			},
			want: "if{return}else{};return",
		},
		{
			name: "a plain branch continues the body",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
			},
			want: "return",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := parseValid(t, newBodyModule(), tc.body...)
			if got := shape(fn.Body); got != tc.want {
				t.Errorf("shape = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestControlFlowSelectionCondition(t *testing.T) {
	// A computed condition is emitted in the header and used by the If.
	fn := parseValid(t, newBodyModule(),
		instWords(spvenums.OpSLessThan, 6, 200, 20, 26),
		instWords(spvenums.OpSelectionMerge, 103, 0),
		instWords(spvenums.OpBranchConditional, 200, 102, 103),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpBranch, 103),
		instWords(spvenums.OpLabel, 103),
	)
	body := stripEmits(fn.Body)
	stmt, ok := body[0].Kind.(ir.StmtIf)
	if !ok {
		t.Fatalf("body[0] = %#v, want an If", body[0].Kind)
	}
	if _, ok := fn.Expressions[stmt.Condition].Kind.(ir.ExprBinary); !ok {
		t.Errorf("condition = %#v, want the comparison", fn.Expressions[stmt.Condition].Kind)
	}
}

func TestControlFlowLoop(t *testing.T) {
	cases := []struct {
		name      string
		body      [][]uint32
		want      string
		breakIf   bool
		negated   bool
		checkCond bool
	}{
		{
			name: "loop with a conditional break in the header",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpLoopMerge, 104, 103, 0),
				instWords(spvenums.OpBranchConditional, 24, 105, 104),
				instWords(spvenums.OpLabel, 105),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 104),
			},
			want: "loop{if{continue}else{break}}continuing{};return",
		},
		{
			// Rust naga's do-while.spvasm: the back edge is the true target,
			// so the loop breaks when the condition is false.
			name: "do-while, back edge on true",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpLoopMerge, 104, 103, 0),
				instWords(spvenums.OpBranch, 105),
				instWords(spvenums.OpLabel, 105),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranchConditional, 24, 102, 104),
				instWords(spvenums.OpLabel, 104),
			},
			want:      "loop{continue}continuing{}breakif;return",
			breakIf:   true,
			negated:   true,
			checkCond: true,
		},
		{
			name: "do-while, back edge on false",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpLoopMerge, 104, 103, 0),
				instWords(spvenums.OpBranch, 105),
				instWords(spvenums.OpLabel, 105),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranchConditional, 24, 104, 102),
				instWords(spvenums.OpLabel, 104),
			},
			want:      "loop{continue}continuing{}breakif;return",
			breakIf:   true,
			checkCond: true,
		},
		{
			name: "single-block loop whose continue target is its header",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpLoopMerge, 104, 102, 0),
				instWords(spvenums.OpBranchConditional, 24, 102, 104),
				instWords(spvenums.OpLabel, 104),
			},
			want: "loop{if{continue}else{break}}continuing{};return",
		},
		{
			name: "loop control parameters are skipped",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				// DependencyLength (0x8) takes one literal.
				instWords(spvenums.OpLoopMerge, 104, 103, 8, 4),
				instWords(spvenums.OpBranchConditional, 24, 103, 104, 1, 1),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 104),
			},
			want: "loop{if{continue}else{break}}continuing{};return",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := parseValid(t, newBodyModule(), tc.body...)
			if got := shape(fn.Body); got != tc.want {
				t.Fatalf("shape = %s, want %s", got, tc.want)
			}
			if !tc.checkCond {
				return
			}
			loop := stripEmits(fn.Body)[0].Kind.(ir.StmtLoop)
			cond := fn.Expressions[*loop.BreakIf].Kind
			unary, isNot := cond.(ir.ExprUnary)
			if tc.negated != (isNot && unary.Op == ir.UnaryLogicalNot) {
				t.Errorf("break-if condition = %#v, negated want %v", cond, tc.negated)
			}
		})
	}
}

func TestControlFlowSwitch(t *testing.T) {
	cases := []struct {
		name     string
		selector uint32
		targets  []uint32
		want     string
		bitcast  bool
	}{
		{
			name:     "shared targets and a case at the merge",
			selector: 20,
			targets:  []uint32{1, 102, 2, 102, 3, 103, 4, 110},
			// Literal 2 shares 1's target: an empty fall-through case before it.
			want: "switch{2+:{} 1:{break} 3:{break} 4:{break} default:{}};return",
		},
		{
			name:     "an unsigned selector is bitcast to i32",
			selector: 21,
			targets:  []uint32{1, 102, 2, 103},
			want:     "switch{1:{break} 2:{break} default:{}};return",
			bitcast:  true,
		},
		{
			name:     "only a default",
			selector: 20,
			want:     "switch{default:{}};return",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			operands := append([]uint32{tc.selector, 110}, tc.targets...)
			body := [][]uint32{
				instWords(spvenums.OpSelectionMerge, 110, 0),
				instWords(spvenums.OpSwitch, operands...),
			}
			// Each case block the switch names branches to the merge.
			for _, label := range []uint32{102, 103} {
				for i := 1; i < len(tc.targets); i += 2 {
					if tc.targets[i] == label {
						body = append(body, instWords(spvenums.OpLabel, label), instWords(spvenums.OpBranch, 110))
						break
					}
				}
			}
			body = append(body, instWords(spvenums.OpLabel, 110))
			fn := parseValid(t, newBodyModule(), body...)
			if got := shape(fn.Body); got != tc.want {
				t.Fatalf("shape = %s, want %s", got, tc.want)
			}
			sw := stripEmits(fn.Body)[0].Kind.(ir.StmtSwitch)
			as, isAs := fn.Expressions[sw.Selector].Kind.(ir.ExprAs)
			if tc.bitcast != isAs || (isAs && (as.Kind != ir.ScalarSint || as.Convert != nil)) {
				t.Errorf("selector = %#v, bitcast want %v", fn.Expressions[sw.Selector].Kind, tc.bitcast)
			}
		})
	}
}

func TestControlFlowSwitchFallThroughToNextCase(t *testing.T) {
	// Case 1 branches into case 2's block: it falls through, as in Rust naga.
	fn := parseValid(t, newBodyModule(),
		instWords(spvenums.OpSelectionMerge, 110, 0),
		instWords(spvenums.OpSwitch, 20, 110, 1, 102, 2, 103),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpBranch, 103),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpBranch, 110),
		instWords(spvenums.OpLabel, 110),
	)
	if got, want := shape(fn.Body), "switch{1+:{} 2:{break} default:{}};return"; got != want {
		t.Errorf("shape = %s, want %s", got, want)
	}
}

// switchStores runs a switch on one selector value as the IR defines it: the
// case whose value matches (the default when none does), then each next case
// while the one before falls through. It returns the i32 constants the cases
// store, in order, up to the first Break or other terminator.
func switchStores(t *testing.T, module *ir.Module, fn *ir.Function, sw ir.StmtSwitch, selector int32) []int32 {
	t.Helper()
	start := -1
	for i, c := range sw.Cases {
		if v, ok := c.Value.(ir.SwitchValueI32); ok && int32(v) == selector {
			start = i
		}
	}
	if start < 0 {
		for i, c := range sw.Cases {
			if _, ok := c.Value.(ir.SwitchValueDefault); ok {
				start = i
			}
		}
	}
	var stores []int32
	for i := start; i < len(sw.Cases); i++ {
		for _, statement := range sw.Cases[i].Body {
			if isTerminator(statement) {
				return stores
			}
			if store, ok := statement.Kind.(ir.StmtStore); ok {
				stores = append(stores, i32Constant(t, module, fn, store.Value))
			}
		}
		if !sw.Cases[i].FallThrough {
			return stores
		}
	}
	return stores
}

// i32Constant reads the i32 constant a function expression stands for.
func i32Constant(t *testing.T, module *ir.Module, fn *ir.Function, handle ir.ExpressionHandle) int32 {
	t.Helper()
	kind := fn.Expressions[handle].Kind
	if c, ok := kind.(ir.ExprConstant); ok {
		kind = module.GlobalExpressions[module.Constants[c.Constant].Init].Kind
	}
	if literal, ok := kind.(ir.Literal); ok {
		if v, ok := literal.Value.(ir.LiteralI32); ok {
			return int32(v)
		}
	}
	t.Fatalf("stored value %#v is not an i32 constant", kind)
	return 0
}

// TestControlFlowSwitchLiteralAtDefault: a literal whose target is the
// default target runs the default body. glslang emits this for
// `case 3: default: …`. Deviation from Rust naga, which miscompiles it
// (next_block.rs:2140-2142 and 2187-2199): the default body takes the block
// first, the literal's case stays empty with fall-through set and falls into
// whatever case follows it, so x == 3 ran case 5's body.
func TestControlFlowSwitchLiteralAtDefault(t *testing.T) {
	// OpSwitch %x default:%102 3:%102 5:%103 7:%102. %102 stores 1, %103
	// stores 5; both break.
	m := newBodyModule()
	module, _, err := Parse(m.build(
		instWords(spvenums.OpVariable, 11, 60, fnSpace),
		instWords(spvenums.OpSelectionMerge, 110, 0),
		instWords(spvenums.OpSwitch, 20, 102, 3, 102, 5, 103, 7, 102),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpStore, 60, 26),
		instWords(spvenums.OpBranch, 110),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpStore, 60, 20),
		instWords(spvenums.OpBranch, 110),
		instWords(spvenums.OpLabel, 110),
	), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	fn := &module.Functions[0]
	sw := stripEmits(fn.Body)[0].Kind.(ir.StmtSwitch)
	for _, tc := range []struct {
		selector int32
		want     []int32
	}{
		{selector: 3, want: []int32{1}},
		{selector: 5, want: []int32{5}},
		{selector: 7, want: []int32{1}},
		{selector: 9, want: []int32{1}},
	} {
		if got := switchStores(t, module, fn, sw, tc.selector); !slices.Equal(got, tc.want) {
			t.Errorf("selector %d stores %v, want %v (switch %s)", tc.selector, got, tc.want, shape(fn.Body))
		}
	}
}

// Labels and stores of the fall-through tests: the default target %102 stores
// 1 (%26), case 5's target %103 stores 5 (%20), case 7's target %104 stores
// -1 (%23); the switch merge is %110.
const (
	ftDefault uint32 = 102
	ftCase5   uint32 = 103
	ftCase7   uint32 = 104
	ftMerge   uint32 = 110
)

// ftBlock is a case block that stores value into %60 and branches to next.
func ftBlock(label, value, next uint32) [][]uint32 {
	return [][]uint32{
		instWords(spvenums.OpLabel, label),
		instWords(spvenums.OpStore, 60, value),
		instWords(spvenums.OpBranch, next),
	}
}

// ftBody is the function body of a fall-through test: a variable %60, the
// switch on %20 with the given case operands (default %102), the case blocks,
// and the merge block.
func ftBody(caseOps []uint32, blocks ...[][]uint32) [][]uint32 {
	body := [][]uint32{
		instWords(spvenums.OpVariable, 11, 60, fnSpace),
		instWords(spvenums.OpSelectionMerge, ftMerge, 0),
		instWords(spvenums.OpSwitch, append([]uint32{20, ftDefault}, caseOps...)...),
	}
	for _, b := range blocks {
		body = append(body, b...)
	}
	return append(body, instWords(spvenums.OpLabel, ftMerge))
}

// TestControlFlowSwitchNonEmptyFallThrough: SPIR-V case constructs that branch
// into another case construct, with statements on the way. The IR expresses
// fall-through only into the next case, so the cases are ordered by their
// fall-through edges: a default that falls into a case sits directly before
// it (not last), a case that falls into the default directly before the
// default. Each test runs the lowered switch per selector value.
//
// Deviation from Rust naga: it keeps the default last with no fall-through
// and never reorders (next_block.rs:2128-2240), so a default that branches
// into a case loses that case's statements, and a case that branches into the
// default falls into whatever case follows it instead.
func TestControlFlowSwitchNonEmptyFallThrough(t *testing.T) {
	type run struct {
		selector int32
		want     []int32
	}
	cases := []struct {
		name string
		body [][]uint32
		runs []run
	}{
		{
			name: "default falls into case",
			body: ftBody([]uint32{5, ftCase5},
				ftBlock(ftDefault, 26, ftCase5),
				ftBlock(ftCase5, 20, ftMerge)),
			runs: []run{{5, []int32{5}}, {9, []int32{1, 5}}},
		},
		{
			name: "default falls into a case that is not the first literal",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				ftBlock(ftDefault, 26, ftCase7),
				ftBlock(ftCase5, 20, ftMerge),
				ftBlock(ftCase7, 23, ftMerge)),
			runs: []run{{5, []int32{5}}, {7, []int32{-1}}, {9, []int32{1, -1}}},
		},
		{
			name: "case falls into default",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				ftBlock(ftCase5, 20, ftDefault),
				ftBlock(ftDefault, 26, ftMerge),
				ftBlock(ftCase7, 23, ftMerge)),
			runs: []run{{5, []int32{5, 1}}, {7, []int32{-1}}, {9, []int32{1}}},
		},
		{
			name: "case falls into default, which falls into a case",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				ftBlock(ftCase5, 20, ftDefault),
				ftBlock(ftDefault, 26, ftCase7),
				ftBlock(ftCase7, 23, ftMerge)),
			runs: []run{{5, []int32{5, 1, -1}}, {7, []int32{-1}}, {9, []int32{1, -1}}},
		},
		{
			name: "case falls into the next case",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				ftBlock(ftCase5, 20, ftCase7),
				ftBlock(ftCase7, 23, ftMerge),
				ftBlock(ftDefault, 26, ftMerge)),
			runs: []run{{5, []int32{5, -1}}, {7, []int32{-1}}, {9, []int32{1}}},
		},
		{
			// The case block is placed before the default that falls into it,
			// so the branch goes back to a block already parsed.
			name: "default placed after the case it falls into",
			body: ftBody([]uint32{5, ftCase5},
				ftBlock(ftCase5, 20, ftMerge),
				ftBlock(ftDefault, 26, ftCase5)),
			runs: []run{{5, []int32{5}}, {9, []int32{1, 5}}},
		},
		{
			name: "literal at the default target and a case falling into the default",
			body: ftBody([]uint32{3, ftDefault, 5, ftCase5},
				ftBlock(ftCase5, 20, ftDefault),
				ftBlock(ftDefault, 26, ftMerge)),
			runs: []run{{3, []int32{1}}, {5, []int32{5, 1}}, {9, []int32{1}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := Parse(newBodyModule().build(tc.body...), Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			fn := &module.Functions[0]
			sw := stripEmits(fn.Body)[0].Kind.(ir.StmtSwitch)
			if last := sw.Cases[len(sw.Cases)-1]; last.FallThrough {
				t.Errorf("last case falls through (switch %s)", shape(fn.Body))
			}
			for _, r := range tc.runs {
				if got := switchStores(t, module, fn, sw, r.selector); !slices.Equal(got, r.want) {
					t.Errorf("selector %d stores %v, want %v (switch %s)", r.selector, got, r.want, shape(fn.Body))
				}
			}
		})
	}
}

// TestControlFlowSwitchCaseIsLoopHeader: a case or default target that is a
// loop header whose continue construct ends in an unconditional back edge
// (valid SPIR-V, spirv-val accepts it). The back edge is a branch to the case
// label, but from the loop's continuing body: it repeats the loop and is not a
// fall-through. Rust naga handles it the same way (next_block.rs OpBranch
// checks only the merge blocks).
func TestControlFlowSwitchCaseIsLoopHeader(t *testing.T) {
	// loopCase is a loop with header label whose body stores value and breaks
	// (the condition is the constant true), and whose merge block stores after
	// before it branches to the switch merge.
	loopCase := func(label, value uint32, after []uint32) [][]uint32 {
		const body, cont, merge, selMerge = 120, 121, 122, 123
		out := [][]uint32{
			instWords(spvenums.OpLabel, label),
			instWords(spvenums.OpLoopMerge, merge, cont, 0),
			instWords(spvenums.OpBranch, body),
			instWords(spvenums.OpLabel, body),
			instWords(spvenums.OpStore, 60, value),
			instWords(spvenums.OpSelectionMerge, selMerge, 0),
			instWords(spvenums.OpBranchConditional, 24, merge, selMerge),
			instWords(spvenums.OpLabel, selMerge),
			instWords(spvenums.OpBranch, cont),
			instWords(spvenums.OpLabel, cont),
			instWords(spvenums.OpBranch, label),
			instWords(spvenums.OpLabel, merge),
		}
		if after != nil {
			out = append(out, after)
		}
		return append(out, instWords(spvenums.OpBranch, ftMerge))
	}
	type run struct {
		selector int32
		want     []int32
	}
	cases := []struct {
		name string
		body [][]uint32
		runs []run
	}{
		{
			// The reviewer's A12b: case 5 is the loop header.
			name: "case",
			body: ftBody([]uint32{5, ftCase5},
				loopCase(ftCase5, 26, instWords(spvenums.OpStore, 60, 23)),
				ftBlock(ftDefault, 20, ftMerge)),
			runs: []run{{5, []int32{1, -1}}, {9, []int32{5}}},
		},
		{
			// The reviewer's B13: the default is the loop header.
			name: "default",
			body: ftBody([]uint32{5, ftCase5},
				ftBlock(ftCase5, 26, ftMerge),
				loopCase(ftDefault, 20, nil)),
			runs: []run{{5, []int32{1}}, {9, []int32{5}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := Parse(newBodyModule().build(tc.body...), Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			fn := &module.Functions[0]
			sw := stripEmits(fn.Body)[0].Kind.(ir.StmtSwitch)
			for _, r := range tc.runs {
				if got := runSwitch(t, module, fn, sw, r.selector); !slices.Equal(got, r.want) {
					t.Errorf("selector %d stores %v, want %v (switch %s)", r.selector, got, r.want, shape(fn.Body))
				}
			}
		})
	}
}

// exit is how running a block of statements ended.
type exit uint8

const (
	exitNone exit = iota
	exitBreak
	exitContinue
	exitReturn
)

// runSwitch is switchStores for case bodies with control flow: it runs the
// switch on one selector value with runBlock, following fall-through, and
// returns the i32 constants stored, in order.
func runSwitch(t *testing.T, module *ir.Module, fn *ir.Function, sw ir.StmtSwitch, selector int32) []int32 {
	t.Helper()
	start := -1
	for i, c := range sw.Cases {
		if v, ok := c.Value.(ir.SwitchValueI32); ok && int32(v) == selector {
			start = i
		}
	}
	if start < 0 {
		for i, c := range sw.Cases {
			if _, ok := c.Value.(ir.SwitchValueDefault); ok {
				start = i
			}
		}
	}
	var stores []int32
	for i := start; i < len(sw.Cases); i++ {
		if runBlock(t, module, fn, sw.Cases[i].Body, &stores) != exitNone || !sw.Cases[i].FallThrough {
			break
		}
	}
	return stores
}

// runBlock runs stores, Ifs and Loops whose conditions are bool constants,
// appending each stored i32 constant to stores. A loop runs at most 8
// iterations, so a test that loops forever fails instead of hanging.
func runBlock(t *testing.T, module *ir.Module, fn *ir.Function, block ir.Block, stores *[]int32) exit {
	t.Helper()
	for _, statement := range block {
		switch kind := statement.Kind.(type) {
		case ir.StmtStore:
			*stores = append(*stores, i32Constant(t, module, fn, kind.Value))
		case ir.StmtIf:
			arm := kind.Reject
			if boolConstant(t, module, fn, kind.Condition) {
				arm = kind.Accept
			}
			if e := runBlock(t, module, fn, arm, stores); e != exitNone {
				return e
			}
		case ir.StmtLoop:
			if e := runLoop(t, module, fn, kind, stores); e != exitNone {
				return e
			}
		case ir.StmtBreak:
			return exitBreak
		case ir.StmtContinue:
			return exitContinue
		case ir.StmtReturn, ir.StmtKill:
			return exitReturn
		}
	}
	return exitNone
}

// runLoop runs one Loop for runBlock; it returns exitReturn when the loop
// body returns and exitNone when the loop ends.
func runLoop(t *testing.T, module *ir.Module, fn *ir.Function, loop ir.StmtLoop, stores *[]int32) exit {
	t.Helper()
	for range 8 {
		switch runBlock(t, module, fn, loop.Body, stores) {
		case exitBreak:
			return exitNone
		case exitReturn:
			return exitReturn
		}
		if e := runBlock(t, module, fn, loop.Continuing, stores); e == exitReturn {
			return e
		}
		if loop.BreakIf != nil && boolConstant(t, module, fn, *loop.BreakIf) {
			return exitNone
		}
	}
	t.Fatalf("loop did not end after 8 iterations")
	return exitReturn
}

// boolConstant reads the bool constant a function expression stands for.
func boolConstant(t *testing.T, module *ir.Module, fn *ir.Function, handle ir.ExpressionHandle) bool {
	t.Helper()
	kind := fn.Expressions[handle].Kind
	if c, ok := kind.(ir.ExprConstant); ok {
		kind = module.GlobalExpressions[module.Constants[c.Constant].Init].Kind
	}
	if literal, ok := kind.(ir.Literal); ok {
		if v, ok := literal.Value.(ir.LiteralBool); ok {
			return bool(v)
		}
	}
	t.Fatalf("condition %#v is not a bool constant", kind)
	return false
}

// TestControlFlowSwitchConditionalFallThrough: a case construct whose
// OpBranchConditional goes either to the next case or to the switch merge
// (valid SPIR-V, spirv-val accepts it). The arm into the next case stays
// empty and the case falls through; the other arm is a Break. Deviation from
// Rust naga: its If arm takes the next case's block (next_block.rs
// OpBranchConditional), so that case's own body is empty and selector 7 runs
// nothing.
func TestControlFlowSwitchConditionalFallThrough(t *testing.T) {
	module, _, err := Parse(newBodyModule().build(ftBody([]uint32{5, ftCase5, 7, ftCase7},
		[][]uint32{
			instWords(spvenums.OpLabel, ftCase5),
			instWords(spvenums.OpStore, 60, 20),
			instWords(spvenums.OpBranchConditional, 24, ftCase7, ftMerge),
		},
		ftBlock(ftCase7, 23, ftMerge),
		ftBlock(ftDefault, 26, ftMerge))...), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	fn := &module.Functions[0]
	want := "switch{5+:{store;if{}else{break}} 7:{store;break} default:{store;break}};return"
	if got := shape(fn.Body); got != want {
		t.Errorf("shape = %s, want %s", got, want)
	}
	sw := stripEmits(fn.Body)[0].Kind.(ir.StmtSwitch)
	if got := switchStores(t, module, fn, sw, 7); !slices.Equal(got, []int32{-1}) {
		t.Errorf("selector 7 stores %v, want [-1]", got)
	}
}

// TestControlFlowSwitchFallThroughRefused: fall-through shapes the IR cannot
// express by case order are refused instead of miscompiled. spirv-val rejects
// each of them too (multiple case constructs branching to one; a back edge to
// a case that is not a loop header; a nested selection exiting to a case; a
// switch whose default is the loop's continue target). Rust naga accepts them
// and loses or misplaces case bodies.
func TestControlFlowSwitchFallThroughRefused(t *testing.T) {
	cases := []struct {
		name string
		body [][]uint32
		want string
	}{
		{
			name: "two cases fall into one",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				ftBlock(ftCase5, 20, ftCase7),
				ftBlock(ftDefault, 26, ftCase7),
				ftBlock(ftCase7, 23, ftMerge)),
			want: "more than one switch case falls through into %104, which is not supported",
		},
		{
			name: "cycle",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				ftBlock(ftCase5, 20, ftCase7),
				ftBlock(ftCase7, 23, ftCase5),
				ftBlock(ftDefault, 26, ftMerge)),
			want: "the fall-through of switch case %103 forms a cycle, which is not supported",
		},
		{
			name: "fall-through from a nested selection",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				[][]uint32{
					instWords(spvenums.OpLabel, ftCase5),
					instWords(spvenums.OpSelectionMerge, 105, 0),
					instWords(spvenums.OpBranchConditional, 24, 106, 105),
					instWords(spvenums.OpLabel, 106),
					instWords(spvenums.OpBranch, ftCase7),
					instWords(spvenums.OpLabel, 105),
					instWords(spvenums.OpBranch, ftMerge),
				},
				ftBlock(ftCase7, 23, ftMerge),
				ftBlock(ftDefault, 26, ftMerge)),
			want: "a branch into switch case %104 from outside a case construct of its switch is not supported",
		},
		{
			name: "conditional branch into two cases",
			body: ftBody([]uint32{5, ftCase5, 7, ftCase7},
				[][]uint32{
					instWords(spvenums.OpLabel, ftDefault),
					instWords(spvenums.OpBranchConditional, 24, ftCase5, ftCase7),
				},
				ftBlock(ftCase5, 20, ftMerge),
				ftBlock(ftCase7, 23, ftMerge)),
			want: "a conditional branch into two switch cases %103 and %104 is not supported",
		},
		{
			name: "default at the loop's continue target",
			body: switchInLoop(103, []uint32{5, 107},
				instWords(spvenums.OpLabel, 107),
				instWords(spvenums.OpBranch, 106)),
			want: "a switch default at the enclosing loop's continue target %103 is not supported",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newBodyModule().parseError(t, tc.body...).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// switchInLoop builds a loop (header 102, merge 104, continue target 103)
// whose body is a switch on %20 with merge 106; caseOps are the switch's case
// operands and caseBlocks the blocks after it, before the switch merge.
func switchInLoop(defaultTarget uint32, caseOps []uint32, caseBlocks ...[]uint32) [][]uint32 {
	body := [][]uint32{
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpLoopMerge, 104, 103, 0),
		instWords(spvenums.OpBranch, 105),
		instWords(spvenums.OpLabel, 105),
		instWords(spvenums.OpSelectionMerge, 106, 0),
		instWords(spvenums.OpSwitch, append([]uint32{20, defaultTarget}, caseOps...)...),
	}
	body = append(body, caseBlocks...)
	return append(body,
		instWords(spvenums.OpLabel, 106),
		instWords(spvenums.OpBranch, 103),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 104),
	)
}

// TestControlFlowSwitchExitsLoop: a switch case that branches to the merge
// block of the loop around the switch leaves the loop. The IR's Break inside
// a Switch leaves only the switch, so the shape cannot be expressed without
// a flag variable and is refused at the branch. Deviation from Rust naga:
// its merger turns the branch into a switch Break (next_block.rs merger),
// so the loop never exits. A case that branches to the loop's continue
// target stays a Continue, which the IR allows inside a switch.
func TestControlFlowSwitchExitsLoop(t *testing.T) {
	const want = "a switch case that exits the enclosing loop is not supported"
	cases := []struct {
		name string
		body [][]uint32
	}{
		{
			name: "a case block branches to the loop merge",
			body: switchInLoop(106, []uint32{1, 107},
				instWords(spvenums.OpLabel, 107),
				instWords(spvenums.OpBranch, 104)),
		},
		{
			name: "a case target is the loop merge",
			body: switchInLoop(106, []uint32{1, 104}),
		},
		{
			name: "the default target is the loop merge",
			body: switchInLoop(104, []uint32{1, 106}),
		},
		{
			name: "a conditional branch in a case to the loop merge",
			body: switchInLoop(106, []uint32{1, 107},
				instWords(spvenums.OpLabel, 107),
				instWords(spvenums.OpBranchConditional, 24, 104, 106)),
		},
		{
			name: "an if arm inside a case branches to the loop merge",
			body: switchInLoop(106, []uint32{1, 107},
				instWords(spvenums.OpLabel, 107),
				instWords(spvenums.OpSelectionMerge, 108, 0),
				instWords(spvenums.OpBranchConditional, 24, 109, 108),
				instWords(spvenums.OpLabel, 109),
				instWords(spvenums.OpBranch, 104),
				instWords(spvenums.OpLabel, 108),
				instWords(spvenums.OpBranch, 106)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newBodyModule().parseError(t, tc.body...).Message; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}

	t.Run("a case branches to the loop's continue target", func(t *testing.T) {
		fn := parseValid(t, newBodyModule(), switchInLoop(106, []uint32{1, 107},
			instWords(spvenums.OpLabel, 107),
			instWords(spvenums.OpBranch, 103))...)
		if got, want := shape(fn.Body), "loop{switch{1:{continue} default:{}};continue}continuing{};return"; got != want {
			t.Errorf("shape = %s, want %s", got, want)
		}
	})
}

// TestControlFlowLoopMergeBeforeContinue: a loop whose merge block comes
// before its continue construct in the module, with a conditional back edge,
// is refused with an error that says so. spirv-val accepts the order; Rust
// naga fails on it too (its validator: "break used outside of a loop"). The
// merge block's statements land after the Loop in the enclosing body, so the
// back edge can no longer be matched to the loop as a break-if.
func TestControlFlowLoopMergeBeforeContinue(t *testing.T) {
	err := newBodyModule().parseError(t,
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpLoopMerge, 104, 103, 0),
		instWords(spvenums.OpBranchConditional, 24, 104, 103),
		instWords(spvenums.OpLabel, 104),
		instWords(spvenums.OpBranch, 106),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpBranchConditional, 24, 104, 102),
		instWords(spvenums.OpLabel, 106),
	)
	if want := "loop merge block %104 comes before the loop's continue construct, which is not supported"; err.Message != want {
		t.Errorf("message = %q, want %q", err.Message, want)
	}
}

func TestControlFlowPhi(t *testing.T) {
	// %51 = phi [%50 from 102] [5 from 103]: a local variable named after the
	// phi, stored at the end of each predecessor and loaded in the merge.
	fn := parseValid(t, newBodyModule(),
		instWords(spvenums.OpVariable, 11, 60, fnSpace),
		instWords(spvenums.OpSelectionMerge, 104, 0),
		instWords(spvenums.OpBranchConditional, 24, 102, 103),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpIAdd, 3, 50, 20, 26),
		instWords(spvenums.OpBranch, 104),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpBranch, 104),
		instWords(spvenums.OpLabel, 104),
		instWords(spvenums.OpPhi, 3, 51, 50, 102, 20, 103),
		instWords(spvenums.OpStore, 60, 51),
	)
	if got, want := shape(fn.Body), "if{store}else{store};store;return"; got != want {
		t.Errorf("shape = %s, want %s", got, want)
	}
	var phi *ir.LocalVariable
	for i := range fn.LocalVars {
		if fn.LocalVars[i].Name == "phi_51" {
			phi = &fn.LocalVars[i]
		}
	}
	if phi == nil || phi.Type != 0 {
		t.Fatalf("locals = %#v, want an i32 phi_51", fn.LocalVars)
	}
}

func TestControlFlowPhiFromLoop(t *testing.T) {
	// A loop counter: phi [0 from 101] [%52 from the continue block 103].
	fn := parseValid(t, newBodyModule(),
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpPhi, 3, 51, 20, 101, 52, 103),
		instWords(spvenums.OpLoopMerge, 104, 103, 0),
		instWords(spvenums.OpSLessThan, 6, 53, 51, 20),
		instWords(spvenums.OpBranchConditional, 53, 103, 104),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpIAdd, 3, 52, 51, 26),
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 104),
	)
	if got, want := shape(fn.Body), "store;loop{if{continue}else{break}}continuing{store};return"; got != want {
		t.Errorf("shape = %s, want %s", got, want)
	}
}

func TestControlFlowSpillOutOfScope(t *testing.T) {
	// %50 is defined in the loop header, which belongs to the loop body, and
	// used after the loop: it is spilled to a temporary inside the loop and
	// reloaded at the use.
	fn := parseValid(t, newBodyModule(),
		instWords(spvenums.OpVariable, 11, 60, fnSpace),
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 102),
		instWords(spvenums.OpLoopMerge, 104, 103, 0),
		instWords(spvenums.OpIAdd, 3, 50, 20, 26),
		instWords(spvenums.OpBranchConditional, 24, 104, 103),
		instWords(spvenums.OpLabel, 103),
		instWords(spvenums.OpBranch, 102),
		instWords(spvenums.OpLabel, 104),
		instWords(spvenums.OpStore, 60, 50),
	)
	if got, want := shape(fn.Body), "loop{store;if{break}else{continue}}continuing{};store;return"; got != want {
		t.Errorf("shape = %s, want %s", got, want)
	}
	if len(fn.LocalVars) != 2 || fn.LocalVars[1].Name != "" {
		t.Errorf("locals = %#v, want the variable and one unnamed spill temporary", fn.LocalVars)
	}
}

func TestControlFlowErrors(t *testing.T) {
	cases := []struct {
		name string
		body [][]uint32
		want string
	}{
		{
			name: "branch to an undefined label",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 150),
				instWords(spvenums.OpLabel, 102),
			},
			want: "undefined label %150",
		},
		{
			name: "label defined twice",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpBranch, 101),
				instWords(spvenums.OpLabel, 101),
			},
			want: "label %101 is defined twice",
		},
		{
			name: "merge block declared twice",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 104, 0),
				instWords(spvenums.OpBranchConditional, 24, 102, 104),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpSelectionMerge, 104, 0),
				instWords(spvenums.OpBranchConditional, 24, 103, 104),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranch, 104),
				instWords(spvenums.OpLabel, 104),
			},
			want: "merge block %104 is declared twice",
		},
		{
			name: "phi predecessor that is not a block",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpPhi, 3, 51, 20, 20),
			},
			want: "phi predecessor %20 is not a block of the function",
		},
		{
			name: "phi with a dangling operand",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpPhi, 3, 51, 20, 101, 20),
			},
			want: "invalid operand count 6 for Phi",
		},
		{
			name: "switch with a dangling operand",
			body: [][]uint32{
				instWords(spvenums.OpSwitch, 20, 102, 1),
				instWords(spvenums.OpLabel, 102),
			},
			want: "invalid operand count 4 for Switch",
		},
		{
			name: "switch on a float",
			body: [][]uint32{
				instWords(spvenums.OpSwitch, 22, 102),
				instWords(spvenums.OpLabel, 102),
			},
			want: "unsupported type [2]",
		},
		{
			name: "switch with a repeated literal",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 110, 0),
				instWords(spvenums.OpSwitch, 20, 110, 1, 102, 1, 110),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpBranch, 110),
				instWords(spvenums.OpLabel, 110),
			},
			want: "switch case 1 is repeated",
		},
		{
			name: "branch on an integer",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 103, 0),
				instWords(spvenums.OpBranchConditional, 20, 102, 103),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
			},
			want: "invalid branch condition type [0]",
		},
		{
			name: "switch on an unknown id",
			body: [][]uint32{
				instWords(spvenums.OpSwitch, 99, 102),
				instWords(spvenums.OpLabel, 102),
			},
			want: "invalid id %99",
		},
		{
			name: "break out of a loop from outside it",
			body: [][]uint32{
				instWords(spvenums.OpSelectionMerge, 106, 0),
				instWords(spvenums.OpBranchConditional, 24, 102, 105),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpLoopMerge, 104, 103, 0),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 105),
				instWords(spvenums.OpBranch, 104),
				instWords(spvenums.OpLabel, 104),
				instWords(spvenums.OpBranch, 106),
				instWords(spvenums.OpLabel, 106),
			},
			want: "invalid terminator",
		},
		{
			name: "return inside a continuing block",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpLoopMerge, 104, 103, 0),
				instWords(spvenums.OpBranch, 103),
				instWords(spvenums.OpLabel, 103),
				instWords(spvenums.OpReturn),
				instWords(spvenums.OpLabel, 104),
			},
			want: "invalid terminator",
		},
		{
			name: "conditional branch back to a parsed block",
			body: [][]uint32{
				instWords(spvenums.OpBranch, 102),
				instWords(spvenums.OpLabel, 102),
				instWords(spvenums.OpSelectionMerge, 103, 0),
				instWords(spvenums.OpBranchConditional, 24, 101, 103),
				instWords(spvenums.OpLabel, 103),
			},
			want: "invalid branch target %101",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newBodyModule().parseError(t, tc.body...).Message; got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestControlFlowMutations rewrites every operand word of structured
// functions with labels, ids and edge values: malformed control flow must be
// refused with an *Error or parse into IR that validates, never panic.
func TestControlFlowMutations(t *testing.T) {
	bodies := [][][]uint32{
		{ // loop with a phi, a break-if candidate and a selection
			instWords(spvenums.OpBranch, 102),
			instWords(spvenums.OpLabel, 102),
			instWords(spvenums.OpPhi, 3, 51, 20, 101, 52, 103),
			instWords(spvenums.OpLoopMerge, 104, 103, 0),
			instWords(spvenums.OpSelectionMerge, 105, 0),
			instWords(spvenums.OpBranchConditional, 24, 106, 105),
			instWords(spvenums.OpLabel, 106),
			instWords(spvenums.OpBranch, 105),
			instWords(spvenums.OpLabel, 105),
			instWords(spvenums.OpBranch, 103),
			instWords(spvenums.OpLabel, 103),
			instWords(spvenums.OpIAdd, 3, 52, 51, 26),
			instWords(spvenums.OpBranchConditional, 24, 102, 104),
			instWords(spvenums.OpLabel, 104),
		},
		{ // switch with shared targets
			instWords(spvenums.OpSelectionMerge, 110, 0),
			instWords(spvenums.OpSwitch, 21, 110, 1, 102, 2, 102, 3, 103),
			instWords(spvenums.OpLabel, 102),
			instWords(spvenums.OpBranch, 103),
			instWords(spvenums.OpLabel, 103),
			instWords(spvenums.OpBranch, 110),
			instWords(spvenums.OpLabel, 110),
		},
	}
	values := []uint32{0, 1, 3, 20, 24, 51, 101, 102, 103, 104, 105, 110, 0xffff, 0xffffffff}
	parses := 0
	for b, body := range bodies {
		for i := range body {
			// Binary operand types are not checked by the frontend (nor by
			// Rust naga's); that is not control flow, so its operands stay.
			if spvenums.Op(body[i][0]&0xffff) == spvenums.OpIAdd {
				continue
			}
			// Operands only: the opcode word keeps the instruction count.
			for w := 1; w < len(body[i]); w++ {
				for _, v := range values {
					mutated := make([][]uint32, len(body))
					copy(mutated, body)
					inst := append([]uint32(nil), body[i]...)
					inst[w] = v
					mutated[i] = inst
					func() {
						defer func() {
							if r := recover(); r != nil {
								t.Fatalf("body %d, instruction %d word %d = %d: panic %v", b, i, w, v, r)
							}
						}()
						module, _, err := Parse(newBodyModule().build(mutated...), Options{})
						if err != nil {
							requireError(t, err)
							return
						}
						parses++
						if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
							t.Errorf("body %d, instruction %d word %d = %d: parses into invalid IR: %v", b, i, w, v, err)
						}
					}()
				}
			}
		}
	}
	if parses == 0 {
		t.Error("no mutation parsed: the bodies themselves are broken")
	}
}

// nestedSelections builds depth selection constructs, each nested in the
// accept arm of the one before it.
func nestedSelections(depth int) [][]uint32 {
	var body [][]uint32
	// Header i is block 1000+i (the first is the function's block 101), its
	// merge block 3000+i.
	header := func(i int) uint32 {
		if i == 0 {
			return 101
		}
		return uint32(1000 + i)
	}
	for i := range depth {
		if i > 0 {
			body = append(body, instWords(spvenums.OpLabel, header(i)))
		}
		next := header(i + 1)
		body = append(body,
			instWords(spvenums.OpSelectionMerge, uint32(3000+i), 0),
			instWords(spvenums.OpBranchConditional, 24, next, uint32(3000+i)),
		)
	}
	body = append(body, instWords(spvenums.OpLabel, header(depth)), instWords(spvenums.OpBranch, uint32(3000+depth-1)))
	for i := depth - 1; i >= 0; i-- {
		body = append(body, instWords(spvenums.OpLabel, uint32(3000+i)))
		if i > 0 {
			body = append(body, instWords(spvenums.OpBranch, uint32(3000+i-1)))
		}
	}
	return body
}

func TestControlFlowNestingLimit(t *testing.T) {
	// At the limit the module parses; one level more is refused before the
	// body tree (and lowering's recursion) grows past it.
	fn := parseValid(t, newBodyModule(), nestedSelections(maxNesting)...)
	depth := 0
	for block := fn.Body; ; depth++ {
		stripped := stripEmits(block)
		if len(stripped) == 0 {
			break
		}
		stmt, ok := stripped[0].Kind.(ir.StmtIf)
		if !ok {
			break
		}
		block = stmt.Accept
	}
	if depth != maxNesting {
		t.Errorf("nesting = %d, want %d", depth, maxNesting)
	}

	err := newBodyModule().parseError(t, nestedSelections(maxNesting+1)...)
	if want := "control flow is nested deeper than 128"; err.Message != want {
		t.Errorf("message = %q, want %q", err.Message, want)
	}
}
