// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package irutil_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
)

// TestInlinePreservesControlFlow runs each function before and after
// InlineUserFunctions on a small IR interpreter and requires the same result
// for every input. The shapes are the ones the DXIL path inlines: a return
// inside a switch, a loop, an if, a switch inside a loop, a void helper that
// returns early, and helpers inlined into helpers (P0-DXIL-052, P0-DXIL-055,
// P0-DXIL-057). Before the returns were lowered with a flag, a return inside a
// switch became a break that only left the switch, so the synthetic wrap loop
// ran again forever, and a return inside a loop only left that loop.
func TestInlinePreservesControlFlow(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"return in switch", `
fn inner(a: u32, b: u32) -> u32 { return a * 10u + b; }
fn outer(a: u32, b: u32, mode: u32) -> u32 {
    switch mode {
        case 1u: { return a; }
        case 3u: { return inner(a, b); }
        default: { return b; }
    }
}
fn f(a: u32, b: u32) -> u32 { return outer(a, b, a % 4u) + outer(b, a, b % 4u); }`},
		{"return in switch, code after it", `
fn g(a: u32, b: u32) -> u32 {
    var r = 1u;
    switch a % 3u {
        case 0u: { return b; }
        case 1u: { r = 5u; }
        default: {}
    }
    r = r + b;
    return r;
}
fn f(a: u32, b: u32) -> u32 { return g(a, b) * 2u + g(b, a); }`},
		{"return in loop", `
fn first_at_least(a: u32, t: u32) -> u32 {
    for (var i = 0u; i < 8u; i++) {
        if (a + i * 3u) % 8u >= t { return i; }
    }
    return 99u;
}
fn f(a: u32, b: u32) -> u32 { return first_at_least(a, b % 9u) + 100u * first_at_least(b, a % 9u); }`},
		{"return in switch in loop", `
fn h(a: u32, b: u32) -> u32 {
    var acc = 0u;
    for (var i = 0u; i < 6u; i++) {
        switch (a + i) % 4u {
            case 2u: { if i >= b % 4u { return acc + 1000u; } }
            case 3u: { break; }
            default: { acc = acc + i; }
        }
        acc = acc + 10u;
    }
    return acc;
}
fn f(a: u32, b: u32) -> u32 { return h(a, b); }`},
		{"break in switch in loop", `
fn k(a: u32, b: u32) -> u32 {
    var n = 0u;
    var i = 0u;
    loop {
        if i >= 3u { break; }
        switch a % 2u {
            case 1u: { break; }
            default: {}
        }
        n = n + b;
        i = i + 1u;
    }
    return n;
}
fn f(a: u32, b: u32) -> u32 { return k(a, b); }`},
		{"void helper returns early", `
fn bump(p: ptr<function, u32>, a: u32) {
    if a % 2u == 0u { return; }
    *p = *p + a;
}
fn f(a: u32, b: u32) -> u32 {
    var x = 0u;
    bump(&x, a);
    bump(&x, b);
    bump(&x, a + b);
    return x;
}`},
		{"if-else returns, code after", `
fn m(a: u32, b: u32) -> u32 {
    if a > b {
        if a > 2u * b { return 1u; }
    } else {
        return 2u;
    }
    let c = a - b;
    if c == 1u { return 3u; }
    return c + 10u;
}
fn f(a: u32, b: u32) -> u32 { return m(a, b) + 100u * m(b, a); }`},
		{"helper chain through a var", `
fn inner(a: u32, b: u32) -> u32 { return a * 10u + b; }
fn twice(a: u32, b: u32) -> u32 { return inner(a, b) * 2u; }
fn f(a: u32, b: u32) -> u32 {
    var c = b;
    switch a % 4u {
        case 1u: { c = a; }
        case 3u: { c = inner(a, b); }
        default: {}
    }
    return c + twice(b, a);
}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := lowerWGSL(t, tt.src)
			inlined := orig.Clone()
			if err := irutil.InlineUserFunctions(inlined, func(*ir.Function) bool { return true }); err != nil {
				t.Fatal(err)
			}
			fo, fi := functionByName(t, orig, "f"), functionByName(t, inlined, "f")
			if hasCall(fi.Body) {
				t.Fatalf("f still calls a helper after inlining")
			}
			for a := uint32(0); a < 7; a++ {
				for b := uint32(0); b < 7; b++ {
					want, err := newIREval(orig).call(fo, []uint32{a, b})
					if err != nil {
						t.Fatalf("f(%d, %d) before inlining: %v", a, b, err)
					}
					got, err := newIREval(inlined).call(fi, []uint32{a, b})
					if err != nil {
						t.Fatalf("f(%d, %d) after inlining: %v", a, b, err)
					}
					if got != want {
						t.Fatalf("f(%d, %d) = %d after inlining, %d before", a, b, got, want)
					}
				}
			}
		})
	}
}

// TestInlineReturnFlagIsClearedPerCall: when a guard reads the returned
// flag, the inlined body clears it before anything else stores it, so a call
// inside a loop does not see the flag its previous iteration set. (The
// semantics test above runs one call per inlined body and would not notice.)
func TestInlineReturnFlagIsClearedPerCall(t *testing.T) {
	mod := lowerWGSL(t, `
fn g(a: u32) -> u32 {
    switch a {
        case 0u: { return 7u; }
        default: {}
    }
    return a + 1u;
}
fn f(n: u32) -> u32 {
    var s = 0u;
    for (var i = 0u; i < n; i++) { s = s + g(i); }
    return s;
}`)
	orig := mod.Clone()
	if err := irutil.InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	f := functionByName(t, mod, "f")
	flag := -1
	for i, lv := range f.LocalVars {
		if lv.Name == "_inline_returned_g" {
			flag = i
		}
	}
	if flag < 0 {
		t.Fatalf("no returned flag: the code after g's switch needs a guard")
	}
	var first *ir.StmtStore
	walkStatements(f.Body, func(s ir.StatementKind) {
		st, ok := s.(ir.StmtStore)
		if !ok || first != nil {
			return
		}
		if lv, ok := f.Expressions[st.Pointer].Kind.(ir.ExprLocalVariable); ok && int(lv.Variable) == flag {
			first = &st
		}
	})
	if first == nil {
		t.Fatalf("the flag is never stored")
	}
	if lit, ok := f.Expressions[first.Value].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralBool(false) {
		t.Errorf("the flag's first store is %+v, want false", f.Expressions[first.Value].Kind)
	}
	for n := uint32(0); n < 5; n++ {
		want, err := newIREval(orig).call(functionByName(t, orig, "f"), []uint32{n})
		if err != nil {
			t.Fatal(err)
		}
		got, err := newIREval(mod).call(f, []uint32{n})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("f(%d) = %d after inlining, %d before", n, got, want)
		}
	}
}

// walkStatements calls fn for every statement in b, in order, depth first.
func walkStatements(b ir.Block, fn func(ir.StatementKind)) {
	for _, s := range b {
		fn(s.Kind)
		switch k := s.Kind.(type) {
		case ir.StmtBlock:
			walkStatements(k.Block, fn)
		case ir.StmtIf:
			walkStatements(k.Accept, fn)
			walkStatements(k.Reject, fn)
		case ir.StmtLoop:
			walkStatements(k.Body, fn)
			walkStatements(k.Continuing, fn)
		case ir.StmtSwitch:
			for _, c := range k.Cases {
				walkStatements(c.Body, fn)
			}
		}
	}
}

func functionByName(t *testing.T, m *ir.Module, name string) *ir.Function {
	t.Helper()
	for i := range m.Functions {
		if m.Functions[i].Name == name {
			return &m.Functions[i]
		}
	}
	t.Fatalf("no function %q", name)
	return nil
}

func hasCall(b ir.Block) bool {
	for _, s := range b {
		switch k := s.Kind.(type) {
		case ir.StmtCall:
			return true
		case ir.StmtBlock:
			if hasCall(k.Block) {
				return true
			}
		case ir.StmtIf:
			if hasCall(k.Accept) || hasCall(k.Reject) {
				return true
			}
		case ir.StmtLoop:
			if hasCall(k.Body) || hasCall(k.Continuing) {
				return true
			}
		case ir.StmtSwitch:
			for _, c := range k.Cases {
				if hasCall(c.Body) {
					return true
				}
			}
		}
	}
	return false
}

// irEval interprets the scalar u32/bool subset of the IR that these tests use.
// It follows the IR's own rules: an expression that needs an Emit has a value
// only after its StmtEmit ran (a Load reads memory at that point), Break leaves
// the innermost loop or switch, Continue goes to the loop's continuing block.
// Anything outside the subset is an error, never a guess.
type irEval struct {
	mod   *ir.Module
	steps int
}

func newIREval(m *ir.Module) *irEval { return &irEval{mod: m} }

const irEvalStepLimit = 100000

var errStepLimit = errors.New("step limit reached (the IR loops forever)")

type irFlow uint8

const (
	flowNext irFlow = iota
	flowBreak
	flowContinue
	flowReturn
)

type irFrame struct {
	ev     *irEval
	fn     *ir.Function
	args   []uint32
	ptrs   map[uint32]*uint32 // pointer arguments, by argument index
	vals   map[ir.ExpressionHandle]uint32
	locals []uint32
	ret    uint32
}

func (e *irEval) call(fn *ir.Function, args []uint32) (uint32, error) {
	return e.callWithPointers(fn, args, nil)
}

func (e *irEval) callWithPointers(fn *ir.Function, args []uint32, ptrs map[uint32]*uint32) (uint32, error) {
	f := &irFrame{ev: e, fn: fn, args: args, ptrs: ptrs, vals: map[ir.ExpressionHandle]uint32{}, locals: make([]uint32, len(fn.LocalVars))}
	for i, lv := range fn.LocalVars {
		if lv.Init != nil {
			v, err := f.value(*lv.Init)
			if err != nil {
				return 0, fmt.Errorf("init of local %q: %w", lv.Name, err)
			}
			f.locals[i] = v
		}
	}
	if _, err := f.block(fn.Body); err != nil {
		return 0, err
	}
	return f.ret, nil
}

func (f *irFrame) block(b ir.Block) (irFlow, error) {
	for i := range b {
		f.ev.steps++
		if f.ev.steps > irEvalStepLimit {
			return flowNext, errStepLimit
		}
		fl, err := f.stmt(b[i].Kind)
		if err != nil || fl != flowNext {
			return fl, err
		}
	}
	return flowNext, nil
}

func (f *irFrame) stmt(s ir.StatementKind) (irFlow, error) {
	switch k := s.(type) {
	case ir.StmtEmit:
		for h := k.Range.Start; h < k.Range.End; h++ {
			v, err := f.eval(h)
			if err != nil {
				return flowNext, err
			}
			f.vals[h] = v
		}
	case ir.StmtBlock:
		return f.block(k.Block)
	case ir.StmtStore:
		p, err := f.pointer(k.Pointer)
		if err != nil {
			return flowNext, err
		}
		v, err := f.value(k.Value)
		if err != nil {
			return flowNext, err
		}
		*p = v
	case ir.StmtIf:
		c, err := f.value(k.Condition)
		if err != nil {
			return flowNext, err
		}
		if c != 0 {
			return f.block(k.Accept)
		}
		return f.block(k.Reject)
	case ir.StmtSwitch:
		return f.switchStmt(k)
	case ir.StmtLoop:
		return f.loop(k)
	case ir.StmtBreak:
		return flowBreak, nil
	case ir.StmtContinue:
		return flowContinue, nil
	case ir.StmtReturn:
		if k.Value != nil {
			v, err := f.value(*k.Value)
			if err != nil {
				return flowNext, err
			}
			f.ret = v
		}
		return flowReturn, nil
	case ir.StmtCall:
		args := make([]uint32, len(k.Arguments))
		ptrs := map[uint32]*uint32{}
		for i, a := range k.Arguments {
			switch f.fn.Expressions[a].Kind.(type) {
			case ir.ExprLocalVariable, ir.ExprFunctionArgument:
				if p, err := f.pointer(a); err == nil {
					ptrs[uint32(i)] = p
					continue
				}
			}
			v, err := f.value(a)
			if err != nil {
				return flowNext, err
			}
			args[i] = v
		}
		r, err := f.ev.callWithPointers(&f.ev.mod.Functions[k.Function], args, ptrs)
		if err != nil {
			return flowNext, err
		}
		if k.Result != nil {
			f.vals[*k.Result] = r
		}
	default:
		return flowNext, fmt.Errorf("statement %T is outside the interpreter's subset", s)
	}
	return flowNext, nil
}

func (f *irFrame) switchStmt(k ir.StmtSwitch) (irFlow, error) {
	sel, err := f.value(k.Selector)
	if err != nil {
		return flowNext, err
	}
	start := -1
	for i, c := range k.Cases {
		switch v := c.Value.(type) {
		case ir.SwitchValueU32:
			if uint32(v) == sel {
				start = i
			}
		case ir.SwitchValueI32:
			if uint32(v) == sel {
				start = i
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		for i, c := range k.Cases {
			if _, ok := c.Value.(ir.SwitchValueDefault); ok {
				start = i
			}
		}
	}
	for i := start; i >= 0 && i < len(k.Cases); i++ {
		fl, err := f.block(k.Cases[i].Body)
		if err != nil {
			return flowNext, err
		}
		switch fl {
		case flowBreak:
			return flowNext, nil
		case flowContinue, flowReturn:
			return fl, nil
		}
		if !k.Cases[i].FallThrough {
			break
		}
	}
	return flowNext, nil
}

func (f *irFrame) loop(k ir.StmtLoop) (irFlow, error) {
	for {
		fl, err := f.block(k.Body)
		if err != nil {
			return flowNext, err
		}
		switch fl {
		case flowBreak:
			return flowNext, nil
		case flowReturn:
			return fl, nil
		}
		if fl, err := f.block(k.Continuing); err != nil || fl != flowNext {
			return fl, err
		}
		if k.BreakIf != nil {
			c, err := f.value(*k.BreakIf)
			if err != nil {
				return flowNext, err
			}
			if c != 0 {
				return flowNext, nil
			}
		}
		f.ev.steps++
		if f.ev.steps > irEvalStepLimit {
			return flowNext, errStepLimit
		}
	}
}

// value returns an expression's value at a use: an emitted value, or one
// that needs no Emit (literals, arguments, call results set by StmtCall).
func (f *irFrame) value(h ir.ExpressionHandle) (uint32, error) {
	if v, ok := f.vals[h]; ok {
		return v, nil
	}
	switch k := f.fn.Expressions[h].Kind.(type) {
	case ir.Literal, ir.ExprFunctionArgument, ir.ExprZeroValue:
		return f.eval(h)
	case ir.ExprConstant:
		c := f.ev.mod.Constants[k.Constant]
		return f.globalValue(c.Init)
	}
	return 0, fmt.Errorf("expression %d (%T) used before its Emit", h, f.fn.Expressions[h].Kind)
}

func (f *irFrame) globalValue(h ir.ExpressionHandle) (uint32, error) {
	switch k := f.ev.mod.GlobalExpressions[h].Kind.(type) {
	case ir.Literal:
		return literalValue(k)
	case ir.ExprZeroValue:
		return 0, nil
	}
	return 0, fmt.Errorf("global expression %T is outside the interpreter's subset", f.ev.mod.GlobalExpressions[h].Kind)
}

func literalValue(l ir.Literal) (uint32, error) {
	switch v := l.Value.(type) {
	case ir.LiteralU32:
		return uint32(v), nil
	case ir.LiteralI32:
		return uint32(v), nil
	case ir.LiteralBool:
		if v {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("literal %T is outside the interpreter's subset", l.Value)
}

// pointer resolves a pointer expression: a local variable, or a pointer
// argument the caller passed.
func (f *irFrame) pointer(h ir.ExpressionHandle) (*uint32, error) {
	switch k := f.fn.Expressions[h].Kind.(type) {
	case ir.ExprLocalVariable:
		return &f.locals[k.Variable], nil
	case ir.ExprFunctionArgument:
		if p, ok := f.ptrs[k.Index]; ok {
			return p, nil
		}
	}
	return nil, fmt.Errorf("pointer %d (%T) is outside the interpreter's subset", h, f.fn.Expressions[h].Kind)
}

func (f *irFrame) eval(h ir.ExpressionHandle) (uint32, error) {
	switch k := f.fn.Expressions[h].Kind.(type) {
	case ir.Literal:
		return literalValue(k)
	case ir.ExprZeroValue:
		return 0, nil
	case ir.ExprFunctionArgument:
		return f.args[k.Index], nil
	case ir.ExprLocalVariable:
		return 0, nil // a pointer; only Load and Store read it, through pointer
	case ir.ExprLoad:
		p, err := f.pointer(k.Pointer)
		if err != nil {
			return 0, err
		}
		return *p, nil
	case ir.ExprAlias:
		return f.value(k.Source)
	case ir.ExprUnary:
		x, err := f.value(k.Expr)
		if err != nil {
			return 0, err
		}
		if k.Op == ir.UnaryLogicalNot {
			return x ^ 1, nil
		}
		return 0, fmt.Errorf("unary op %d is outside the interpreter's subset", k.Op)
	case ir.ExprBinary:
		l, err := f.value(k.Left)
		if err != nil {
			return 0, err
		}
		r, err := f.value(k.Right)
		if err != nil {
			return 0, err
		}
		return binaryU32(k.Op, l, r)
	case ir.ExprSelect:
		c, err := f.value(k.Condition)
		if err != nil {
			return 0, err
		}
		if c != 0 {
			return f.value(k.Accept)
		}
		return f.value(k.Reject)
	}
	return 0, fmt.Errorf("expression %T is outside the interpreter's subset", f.fn.Expressions[h].Kind)
}

func binaryU32(op ir.BinaryOperator, l, r uint32) (uint32, error) {
	b := func(c bool) uint32 {
		if c {
			return 1
		}
		return 0
	}
	switch op {
	case ir.BinaryAdd:
		return l + r, nil
	case ir.BinarySubtract:
		return l - r, nil
	case ir.BinaryMultiply:
		return l * r, nil
	case ir.BinaryDivide, ir.BinaryModulo:
		if r == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		if op == ir.BinaryDivide {
			return l / r, nil
		}
		return l % r, nil
	case ir.BinaryEqual:
		return b(l == r), nil
	case ir.BinaryNotEqual:
		return b(l != r), nil
	case ir.BinaryLess:
		return b(l < r), nil
	case ir.BinaryLessEqual:
		return b(l <= r), nil
	case ir.BinaryGreater:
		return b(l > r), nil
	case ir.BinaryGreaterEqual:
		return b(l >= r), nil
	case ir.BinaryLogicalAnd, ir.BinaryAnd:
		return l & r, nil
	case ir.BinaryLogicalOr, ir.BinaryInclusiveOr:
		return l | r, nil
	}
	return 0, fmt.Errorf("binary op %d is outside the interpreter's subset", op)
}
