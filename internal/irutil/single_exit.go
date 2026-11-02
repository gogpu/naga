package irutil

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// SingleExitSwitchReturns returns m with one exit in every helper function
// that returns from inside a switch inside a loop. Each return stores its
// value into a local and sets a "returned" flag, a break leaves each switch
// and loop it is in, guards skip what follows them, and the one return is at
// the end of the body: the rewrite the inliner gives an inlined callee
// (returnLowering, Mesa's nir_lower_returns).
//
// It is for FXC, the HLSL compiler of shader models 5.x: FXC inlines helpers
// and turns such a return into the switch's break, dropping the value and the
// loop exit (fxc 10.0.26100: the readback case flow/return-in-switch-in-loop-rt
// read wrong values on Intel and NVIDIA), while it compiles a break out of a
// switch and a loop right. Entry points keep their returns: FXC writes them as
// `ret`, which is right. m is not modified; a module with no such helper is
// returned as it is.
func SingleExitSwitchReturns(m *ir.Module) (*ir.Module, error) {
	var rewrite []int
	for i := range m.Functions {
		if returnInSwitchInLoop(m.Functions[i].Body) {
			rewrite = append(rewrite, i)
		}
	}
	if len(rewrite) == 0 {
		return m, nil
	}
	out := m.Clone()
	for _, i := range rewrite {
		if err := singleExit(out, &out.Functions[i]); err != nil {
			return nil, fmt.Errorf("function %q: %w", out.Functions[i].Name, err)
		}
	}
	return out, nil
}

// returnInSwitchInLoop reports whether b returns from inside a switch that
// is inside a loop.
func returnInSwitchInLoop(b ir.Block) bool {
	for i := range b {
		switch sk := b[i].Kind.(type) {
		case ir.StmtBlock:
			if returnInSwitchInLoop(sk.Block) {
				return true
			}
		case ir.StmtIf:
			if returnInSwitchInLoop(sk.Accept) || returnInSwitchInLoop(sk.Reject) {
				return true
			}
		case ir.StmtSwitch:
			for j := range sk.Cases {
				if returnInSwitchInLoop(sk.Cases[j].Body) {
					return true
				}
			}
		case ir.StmtLoop:
			// WGSL has no return in a continuing block.
			if returnInSwitch(sk.Body) {
				return true
			}
		}
	}
	return false
}

// returnInSwitch reports whether b, a block inside a loop, returns from
// inside a switch.
func returnInSwitch(b ir.Block) bool {
	for i := range b {
		switch sk := b[i].Kind.(type) {
		case ir.StmtBlock:
			if returnInSwitch(sk.Block) {
				return true
			}
		case ir.StmtIf:
			if returnInSwitch(sk.Accept) || returnInSwitch(sk.Reject) {
				return true
			}
		case ir.StmtSwitch:
			for j := range sk.Cases {
				if blockContainsReturn(sk.Cases[j].Body) {
					return true
				}
			}
		case ir.StmtLoop:
			if returnInSwitch(sk.Body) {
				return true
			}
		}
	}
	return false
}

// singleExit rewrites fn's returns into stores of a result local and breaks,
// and ends its body with the one return of that local.
func singleExit(m *ir.Module, fn *ir.Function) error {
	var resultPtr *ir.ExpressionHandle
	if fn.Result != nil {
		slot := appendLocal(fn, ir.LocalVariable{Name: "result", Type: fn.Result.Type})
		ptr := appendExpression(fn, ir.ExprLocalVariable{Variable: slot})
		resultPtr = &ptr
	}
	lowering := returnLowering{module: m, caller: fn, flagName: "returned", retPtr: resultPtr}
	body, err := lowering.lower(fn.Body)
	if err != nil {
		return err
	}
	if resultPtr == nil {
		body = append(body, ir.Statement{Kind: ir.StmtReturn{}})
	} else {
		load := appendExpression(fn, ir.ExprLoad{Pointer: *resultPtr})
		body = append(body,
			ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: load, End: load + 1}}},
			ir.Statement{Kind: ir.StmtReturn{Value: &load}},
		)
	}
	fn.Body = body
	return nil
}
