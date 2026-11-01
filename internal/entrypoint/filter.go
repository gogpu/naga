package entrypoint

import "github.com/gogpu/naga/ir"

// stages is a set of shader stages, Rust naga's valid::ShaderStages.
type stages uint8

const (
	stageVertex stages = 1 << iota
	stageFragment
	stageCompute
	stageTask
	stageMesh

	stagesAll         = stageVertex | stageFragment | stageCompute | stageTask | stageMesh
	stagesComputeLike = stageCompute | stageTask | stageMesh
)

// Filter is what an output holding one entry point writes of the module:
// the globals that entry point uses and the functions that can serve it.
// It is Rust naga's SPIR-V writer with PipelineOptions
// (back/spv/writer.rs write_logical_layout):
//
//   - a global the entry point and its callees never reference is left out;
//   - a function that references a global the entry point does not, or that
//     is unavailable in a stage the entry point is available in (Rust's
//     FunctionInfo::available_stages), is left out.
//
// Rust compares global uses as flag sets (read, write, query, atomic); Filter
// compares whether a global is referenced at all. The two differ only for a
// function the entry point does not call that uses a global the entry point
// uses, with more flags: Filter keeps it, Rust leaves it out. The output is
// valid either way.
type Filter struct {
	// Globals[h] is set when the entry point references global h.
	Globals []bool
	// Functions[h] is set when function h is written.
	Functions []bool
}

// NewFilter returns the filter for m.EntryPoints[ep].
func NewFilter(m *ir.Module, ep int) Filter {
	a := &analysis{
		m:       m,
		globals: make([][]bool, len(m.Functions)),
		stages:  make([]stages, len(m.Functions)),
		done:    make([]bool, len(m.Functions)),
	}
	epGlobals, epStages := a.scan(&m.EntryPoints[ep].Function)
	f := Filter{Globals: epGlobals, Functions: make([]bool, len(m.Functions))}
	for h := range m.Functions {
		globals, st := a.function(ir.FunctionHandle(h))
		f.Functions[h] = st&epStages == epStages && subset(globals, epGlobals)
	}
	return f
}

func subset(a, b []bool) bool {
	for i, used := range a {
		if used && !b[i] {
			return false
		}
	}
	return true
}

// analysis computes, per function, the globals it references and the stages
// it is available in, its callees included.
type analysis struct {
	m       *ir.Module
	globals [][]bool
	stages  []stages
	done    []bool
}

func (a *analysis) function(h ir.FunctionHandle) ([]bool, stages) {
	if !a.done[h] {
		// WGSL and the IR forbid recursion, so the callee graph is acyclic;
		// marking first only guards a malformed module from looping.
		a.done[h] = true
		a.stages[h] = stagesAll
		a.globals[h], a.stages[h] = a.scan(&a.m.Functions[h])
	}
	return a.globals[h], a.stages[h]
}

func (a *analysis) scan(fn *ir.Function) ([]bool, stages) {
	used := make([]bool, len(a.m.GlobalVariables))
	st := stagesAll
	for _, e := range fn.Expressions {
		switch k := e.Kind.(type) {
		case ir.ExprGlobalVariable:
			used[k.Variable] = true
		case ir.ExprDerivative:
			st &= stageFragment
		case ir.ExprImageSample:
			// Implicit derivatives (valid/expression.rs ImageSample).
			switch k.Level.(type) {
			case ir.SampleLevelAuto, ir.SampleLevelBias:
				st &= stageFragment
			}
		case ir.ExprWorkGroupUniformLoadResult:
			st &= stagesComputeLike
		}
	}
	a.scanBlock(fn.Body, used, &st)
	return used, st
}

func (a *analysis) scanBlock(block ir.Block, used []bool, st *stages) {
	for _, stmt := range block {
		switch s := stmt.Kind.(type) {
		case ir.StmtKill:
			*st &= stageFragment
		// Rust valid/function.rs:1038: both barriers are compute-like only.
		case ir.StmtControlBarrier, ir.StmtMemoryBarrier, ir.StmtWorkGroupUniformLoad:
			*st &= stagesComputeLike
		case ir.StmtCall:
			if int(s.Function) >= len(a.m.Functions) {
				continue
			}
			globals, calleeStages := a.function(s.Function)
			for i, g := range globals {
				used[i] = used[i] || g
			}
			*st &= calleeStages
		case ir.StmtBlock:
			a.scanBlock(s.Block, used, st)
		case ir.StmtIf:
			a.scanBlock(s.Accept, used, st)
			a.scanBlock(s.Reject, used, st)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				a.scanBlock(c.Body, used, st)
			}
		case ir.StmtLoop:
			a.scanBlock(s.Body, used, st)
			a.scanBlock(s.Continuing, used, st)
		}
	}
}
