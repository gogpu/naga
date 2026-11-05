package run

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// epFailure is an entry point dxil.Compile refused.
type epFailure struct {
	name  string
	stage ir.ShaderStage
	err   error
}

// wgslBlobs is what compileWGSL produced for one .wgsl file: the blobs of the
// entry points that compiled (parallel slices) and the entry points that did
// not, with their errors.
type wgslBlobs struct {
	blobs    [][]byte
	names    []string
	stages   []ir.ShaderStage
	failures []epFailure
}

// compileWGSL parses a .wgsl file and emits one DXIL blob per entry point.
// An entry point that fails to compile is returned in failures with its
// error, never dropped. When entry is non-empty only that single entry point
// is compiled.
//
// When a sibling .toml file declares `[[hlsl.binding_map]]` entries they
// are parsed and passed to dxil.Compile via dxil.Options.BindingMap so
// the emitted DXIL honors explicit (register, space) placement. This is
// required for shaders that declare overlapping @group/@binding pairs
// (binding-arrays.wgsl puts every resource in its own space via the map)
// and for unbounded binding arrays where the TOML carries the concrete
// array size the runtime will use (binding_array_size override).
func compileWGSL(srcPath, entry string) (wgslBlobs, error) {
	var out wgslBlobs
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return out, err
	}
	module, _, err := wgsl.Parse(string(src), wgsl.ParseOptions{})
	if err != nil {
		return out, fmt.Errorf("parse: %w", err)
	}

	opts := dxil.DefaultOptions()
	// Immediate data at b0 in space 0, Rust naga's immediates_target in the
	// test parameters that give one.
	opts.ImmediatesTarget = &dxil.BindTarget{}
	if bm := loadHLSLBindingMap(srcPath); bm != nil {
		opts.BindingMap = bm
	}

	found := false
	for j := range module.EntryPoints {
		ep := module.EntryPoints[j]
		if entry != "" && ep.Name != entry {
			continue
		}
		found = true
		opts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
		bytes, _, cerr := dxil.Compile(module, opts)
		// An entry point that needs a newer shader model is validated at it,
		// the model a device that supports it is asked for.
		var vErr *backend.VersionError
		if errors.As(cerr, &vErr) && vErr.Space == backend.VersionShaderModel {
			epOpts := opts
			epOpts.ShaderModel = backend.ShaderModel(vErr.Required)
			bytes, _, cerr = dxil.Compile(module, epOpts)
		}
		if cerr != nil {
			out.failures = append(out.failures, epFailure{name: ep.Name, stage: ep.Stage, err: cerr})
			continue
		}
		out.blobs = append(out.blobs, bytes)
		out.names = append(out.names, ep.Name)
		out.stages = append(out.stages, ep.Stage)
	}
	if entry != "" && !found {
		return out, fmt.Errorf("entry point %q not found", entry)
	}
	return out, nil
}

// reportCompileFailures prints one line per entry point that did not
// compile and reports whether there was any.
func reportCompileFailures(w io.Writer, failures []epFailure) bool {
	for _, f := range failures {
		fmt.Fprintf(w, "%s/%s: COMPILE_ERROR %v\n", stageString(f.stage), f.name, f.err)
	}
	return len(failures) > 0
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
