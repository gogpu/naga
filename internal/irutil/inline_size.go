package irutil

import "github.com/gogpu/naga/ir"

// InlinedEntryPointSizes returns, for each entry point of the module, the
// size its body would have after InlineUserFunctions: its own expressions
// and statements plus, for every call site, the inlined size of the callee.
// Every count saturates at limit, so the result is exact up to limit and
// limit for anything larger.
//
// The size grows with the product of call fan-out along a call chain: a
// chain of helpers each calling the next one twice inlines into 2^depth
// copies of the leaf from source of linear size. The estimate is computed
// bottom-up with one value per function, so it costs time linear in the
// module, while the inlining it predicts may not finish in practice.
//
// A call cycle (forbidden by WGSL, possible only in malformed IR) counts as
// limit: such a module can't be inlined at all.
func InlinedEntryPointSizes(module *ir.Module, limit int) []int {
	if module == nil {
		return nil
	}
	s := inlinedSizer{
		module: module,
		limit:  limit,
		size:   make([]int, len(module.Functions)),
		state:  make([]uint8, len(module.Functions)),
	}
	sizes := make([]int, len(module.EntryPoints))
	for i := range module.EntryPoints {
		sizes[i] = s.function(&module.EntryPoints[i].Function)
	}
	return sizes
}

// Function sizing states of inlinedSizer.state.
const (
	sizeUnseen uint8 = iota
	sizeInProgress
	sizeDone
)

// inlinedSizer memoizes the inlined size of each module function.
type inlinedSizer struct {
	module *ir.Module
	limit  int
	size   []int
	state  []uint8
}

// add returns a+b saturated at the limit; both are in [0, limit].
func (s *inlinedSizer) add(a, b int) int {
	if a >= s.limit-b {
		return s.limit
	}
	return a + b
}

// callee returns the inlined size of module function h.
func (s *inlinedSizer) callee(h ir.FunctionHandle) int {
	if int(h) >= len(s.module.Functions) {
		return 0
	}
	switch s.state[h] {
	case sizeDone:
		return s.size[h]
	case sizeInProgress:
		return s.limit
	}
	s.state[h] = sizeInProgress
	s.size[h] = s.function(&s.module.Functions[h])
	s.state[h] = sizeDone
	return s.size[h]
}

// function returns the inlined size of fn's body.
func (s *inlinedSizer) function(fn *ir.Function) int {
	n := min(len(fn.Expressions), s.limit)
	return s.add(n, s.block(fn.Body))
}

// block returns the inlined size of the statements of b.
func (s *inlinedSizer) block(b ir.Block) int {
	n := 0
	for i := range b {
		n = s.add(n, 1)
		switch sk := b[i].Kind.(type) {
		case ir.StmtCall:
			n = s.add(n, s.callee(sk.Function))
		case ir.StmtBlock:
			n = s.add(n, s.block(sk.Block))
		case ir.StmtIf:
			n = s.add(n, s.block(sk.Accept))
			n = s.add(n, s.block(sk.Reject))
		case ir.StmtLoop:
			n = s.add(n, s.block(sk.Body))
			n = s.add(n, s.block(sk.Continuing))
		case ir.StmtSwitch:
			for j := range sk.Cases {
				n = s.add(n, s.block(sk.Cases[j].Body))
			}
		}
		if n == s.limit {
			return n
		}
	}
	return n
}
