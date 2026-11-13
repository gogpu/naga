package front

import (
	"github.com/gogpu/naga/ir"
)

// lookupFunction is a parsed function: its handle in Module.Functions and its
// OpTypeFunction, which every call of it is checked against.
type lookupFunction struct {
	handle ir.FunctionHandle
	typeID uint32
	// parameterSampling is how each argument of the function is sampled, as
	// Rust naga's LookupFunction::parameters_sampling: the flags its callers
	// pass on to the globals behind the arguments.
	parameterSampling []samplingFlags
}

// callNode is a node of the call graph: a function's id, or the wrapper of
// the entry point with that index. Rust naga names a wrapper by the fake id
// !(entry point index), which a real id near 2^32 could collide with; the
// wrapper flag keeps the two apart.
type callNode struct {
	id      uint32
	wrapper bool
}

// callNeighbor is one adjacency entry: an edge to or from another node, in
// the order the edges were added.
type callNeighbor struct {
	node     int
	outgoing bool
}

// callGraph is the graph of every call in the module (Rust naga's
// function_call_graph, a petgraph GraphMap). Its node and adjacency order
// reproduces GraphMap's, so the topological order below, and with it the
// order of Module.Functions, matches Rust naga's.
type callGraph struct {
	nodes     []callNode
	index     map[callNode]int
	adjacency [][]callNeighbor
	edges     map[[2]int]struct{}
}

// addNode adds a node unless the graph has it, and returns its index.
func (g *callGraph) addNode(n callNode) int {
	if i, ok := g.index[n]; ok {
		return i
	}
	if g.index == nil {
		g.index = make(map[callNode]int)
		g.edges = make(map[[2]int]struct{})
	}
	i := len(g.nodes)
	g.index[n] = i
	g.nodes = append(g.nodes, n)
	g.adjacency = append(g.adjacency, nil)
	return i
}

// addEdge adds the edge from a to b, and either node the graph lacks. A
// repeated edge changes nothing; a self loop has no incoming entry, as in
// GraphMap::add_edge.
func (g *callGraph) addEdge(from, to callNode) {
	a := g.addNode(from)
	b := g.addNode(to)
	key := [2]int{a, b}
	if _, ok := g.edges[key]; ok {
		return
	}
	g.edges[key] = struct{}{}
	g.adjacency[a] = append(g.adjacency[a], callNeighbor{node: b, outgoing: true})
	if a != b {
		g.adjacency[b] = append(g.adjacency[b], callNeighbor{node: a, outgoing: false})
	}
}

// toposort is petgraph 0.8's algo::toposort on this graph: callers before
// callees. On a cycle it returns false and the node petgraph reports. Both
// searches use an explicit stack, so a long call chain cannot exhaust the
// goroutine stack.
func (g *callGraph) toposort() ([]callNode, callNode, bool) {
	discovered := make([]bool, len(g.nodes))
	finished := make([]bool, len(g.nodes))
	finishStack := make([]int, 0, len(g.nodes))
	var stack []int
	for i := range g.nodes {
		if discovered[i] {
			continue
		}
		stack = append(stack, i)
		for len(stack) > 0 {
			nx := stack[len(stack)-1]
			if !discovered[nx] {
				discovered[nx] = true
				for _, neighbor := range g.adjacency[nx] {
					if !neighbor.outgoing {
						continue
					}
					if neighbor.node == nx {
						return nil, g.nodes[nx], false
					}
					if !discovered[neighbor.node] {
						stack = append(stack, neighbor.node)
					}
				}
				continue
			}
			stack = stack[:len(stack)-1]
			if !finished[nx] {
				finished[nx] = true
				finishStack = append(finishStack, nx)
			}
		}
	}
	for i, j := 0, len(finishStack)-1; i < j; i, j = i+1, j-1 {
		finishStack[i], finishStack[j] = finishStack[j], finishStack[i]
	}

	// A second search on the reversed graph, from each node in that order:
	// reaching any node beyond the start means a cycle (Kosaraju).
	clear(discovered)
	for _, start := range finishStack {
		stack = append(stack[:0], start)
		reached := false
		for len(stack) > 0 {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if discovered[node] {
				continue
			}
			discovered[node] = true
			for _, neighbor := range g.adjacency[node] {
				// GraphMap's incoming neighbors: incoming entries, and a self
				// loop's outgoing one.
				if (neighbor.outgoing && neighbor.node != node) || discovered[neighbor.node] {
					continue
				}
				stack = append(stack, neighbor.node)
			}
			if reached {
				return nil, g.nodes[node], false
			}
			reached = true
		}
	}

	order := make([]callNode, len(finishStack))
	for i, node := range finishStack {
		order[i] = g.nodes[node]
	}
	return order, callNode{}, true
}

// deferredCall is one call site: the callee's id, and what the call expects
// of the callee's type, checked once every function is parsed. Its index is
// the placeholder function handle the call's IR holds until then (Rust naga's
// deferred_function_calls and dummy_functions).
type deferredCall struct {
	callee uint32
	// argumentTypeIDs are the types of the arguments passed.
	argumentTypeIDs []uint32
	// resultTypeID is the call's result type; a wrapper's call has none and
	// needs a void callee.
	resultTypeID uint32
	wrapper      bool
	offset       uint32
}

// addCall records a call from one node to a function and returns the
// placeholder handle its IR refers to (Rust naga's add_call).
func (p *parser) addCall(from callNode, call deferredCall) ir.FunctionHandle {
	handle := ir.FunctionHandle(len(p.deferredCalls))
	p.deferredCalls = append(p.deferredCalls, call)
	p.callGraph.addEdge(from, callNode{id: call.callee})
	return handle
}

// orderFunctions puts Module.Functions in the order Rust naga gives them:
// every callee before its callers, from the reversed topological order of the
// call graph. A recursive call is refused, as in Rust naga.
func (p *parser) orderFunctions() error {
	order, cycle, ok := p.callGraph.toposort()
	if !ok {
		return errFunctionCallCycle(cycle.id)
	}
	functions := p.module.Functions
	ordered := make([]ir.Function, 0, len(functions))
	for i := len(order) - 1; i >= 0; i-- {
		node := order[i]
		if node.wrapper {
			continue
		}
		lookup, ok := p.lookupFunction[node.id]
		if !ok {
			// Only a call adds a node that is not a parsed function; Rust
			// naga panics here.
			return p.callError(node.id, errInvalidID(node.id))
		}
		ordered = append(ordered, functions[lookup.handle])
		lookup.handle = ir.FunctionHandle(len(ordered) - 1)
		p.lookupFunction[node.id] = lookup
	}
	p.module.Functions = ordered
	return nil
}

// callError places err at the first call of callee.
func (p *parser) callError(callee uint32, err *Error) *Error {
	for i := range p.deferredCalls {
		if p.deferredCalls[i].callee == callee {
			err.Offset = p.deferredCalls[i].offset
			break
		}
	}
	return err
}

// checkCalls refuses a call that does not match its callee's type: the
// argument count and types and the result type. Rust naga does not check
// them and builds IR its validator rejects.
func (p *parser) checkCalls() error {
	for i := range p.deferredCalls {
		call := &p.deferredCalls[i]
		lookup := p.lookupFunction[call.callee]
		funType := p.lookupFunctionType[lookup.typeID]
		if len(call.argumentTypeIDs) != len(funType.parameterTypeIDs) {
			return &Error{
				Message: errWrongArgumentCount(call.callee, len(funType.parameterTypeIDs), len(call.argumentTypeIDs)).Message,
				Offset:  call.offset,
			}
		}
		for j, typeID := range call.argumentTypeIDs {
			if typeID != funType.parameterTypeIDs[j] {
				return &Error{Message: errWrongFunctionArgumentType(typeID).Message, Offset: call.offset}
			}
		}
		if call.wrapper {
			if !p.hasVoidType || funType.returnTypeID != p.voidTypeID {
				return &Error{Message: errWrongFunctionResultType(funType.returnTypeID).Message, Offset: call.offset}
			}
			continue
		}
		if call.resultTypeID != funType.returnTypeID {
			return &Error{Message: errWrongFunctionResultType(call.resultTypeID).Message, Offset: call.offset}
		}
	}
	return nil
}

// patchCalls replaces every placeholder handle with the callee's handle in
// Module.Functions (Rust naga's patch_function and patch_statements).
func (p *parser) patchCalls() {
	for i := range p.module.Functions {
		p.patchFunction(&p.module.Functions[i])
	}
	for i := range p.module.EntryPoints {
		p.patchFunction(&p.module.EntryPoints[i].Function)
	}
}

// patchFunction patches one function's call results and call statements.
func (p *parser) patchFunction(fn *ir.Function) {
	for i := range fn.Expressions {
		if result, ok := fn.Expressions[i].Kind.(ir.ExprCallResult); ok {
			result.Function = p.calleeHandle(result.Function)
			fn.Expressions[i].Kind = result
		}
	}
	p.patchBlock(fn.Body)
}

// calleeHandle returns the function a placeholder handle stands for. Every
// callee was checked to be a parsed function by orderFunctions.
func (p *parser) calleeHandle(placeholder ir.FunctionHandle) ir.FunctionHandle {
	return p.lookupFunction[p.deferredCalls[placeholder].callee].handle
}

// patchBlock patches the call statements of a block and of the blocks nested
// in it. The nesting is the frontend's own lowering, bounded by maxNesting.
func (p *parser) patchBlock(block ir.Block) {
	for i := range block {
		switch kind := block[i].Kind.(type) {
		case ir.StmtCall:
			kind.Function = p.calleeHandle(kind.Function)
			block[i].Kind = kind
		case ir.StmtBlock:
			p.patchBlock(kind.Block)
		case ir.StmtIf:
			p.patchBlock(kind.Reject)
			p.patchBlock(kind.Accept)
		case ir.StmtSwitch:
			for _, c := range kind.Cases {
				p.patchBlock(c.Body)
			}
		case ir.StmtLoop:
			p.patchBlock(kind.Body)
			p.patchBlock(kind.Continuing)
		}
	}
}
