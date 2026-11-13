package backend

import "github.com/gogpu/naga/ir"

// EntryPointResultStructName returns the fallback name Rust naga's namer gives
// an anonymous entry point result struct (proc/namer.rs, name_types): the
// stage decides it, so every backend names the struct the same way. It
// returns false for a stage Rust naga has no fallback for (task and mesh,
// where Rust's namer is unreachable); a namer then falls back to its own
// anonymous-type name.
func EntryPointResultStructName(stage ir.ShaderStage) (string, bool) {
	switch stage {
	case ir.StageVertex:
		return "VertexOutput", true
	case ir.StageFragment:
		return "FragmentOutput", true
	case ir.StageCompute:
		return "ComputeOutput", true
	}
	return "", false
}
