package run

import (
	"fmt"
	"os"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// compileWGSL parses a .wgsl file and emits one DXIL blob per entry point.
// Returns parallel slices of (blob, entry-point name, stage). When entry is
// non-empty only that single entry point is compiled.
//
// When a sibling .toml file declares `[[hlsl.binding_map]]` entries they
// are parsed and passed to dxil.Compile via dxil.Options.BindingMap so
// the emitted DXIL honors explicit (register, space) placement. This is
// required for shaders that declare overlapping @group/@binding pairs
// (binding-arrays.wgsl puts every resource in its own space via the map)
// and for unbounded binding arrays where the TOML carries the concrete
// array size the runtime will use (binding_array_size override).
func compileWGSL(srcPath, entry string) (blobs [][]byte, names []string, stages []ir.ShaderStage, err error) {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, nil, nil, err
	}
	tokens, err := wgsl.NewLexer(string(src)).Tokenize()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tokenize: %w", err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse: %w", err)
	}
	module, err := wgsl.LowerWithSource(ast, string(src))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("lower: %w", err)
	}
	if len(module.EntryPoints) == 0 {
		return nil, nil, nil, nil
	}

	opts := dxil.DefaultOptions()
	if bm := loadHLSLBindingMap(srcPath); bm != nil {
		opts.BindingMap = bm
	}

	for j := range module.EntryPoints {
		ep := module.EntryPoints[j]
		if entry != "" && ep.Name != entry {
			continue
		}
		single := &ir.Module{
			Types:             module.Types,
			Constants:         module.Constants,
			GlobalVariables:   module.GlobalVariables,
			GlobalExpressions: module.GlobalExpressions,
			Functions:         module.Functions,
			EntryPoints:       []ir.EntryPoint{ep},
			Overrides:         module.Overrides,
			SpecialTypes:      module.SpecialTypes,
		}
		bytes, cerr := dxil.Compile(single, opts)
		if cerr != nil {
			// Skip individual EP failures; corpus mode tallies these
			// separately and single mode reports the first error.
			continue
		}
		blobs = append(blobs, bytes)
		names = append(names, ep.Name)
		stages = append(stages, ep.Stage)
	}
	if entry != "" && len(blobs) == 0 {
		return nil, nil, nil, fmt.Errorf("entry point %q not found or failed to compile", entry)
	}
	return blobs, names, stages, nil
}

func stageString(s ir.ShaderStage) string {
	switch s {
	case ir.StageVertex:
		return "VS"
	case ir.StageFragment:
		return "FS"
	case ir.StageCompute:
		return "CS"
	}
	return fmt.Sprintf("s%d", s)
}
