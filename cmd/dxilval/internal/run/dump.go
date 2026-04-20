package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// runDump compiles a WGSL file and writes the per-entry-point DXIL blobs
// to disk WITHOUT calling the validator. Designed for offline inspection
// (dxc -dumpbin, hex viewers, byte diff against a reference) and for
// reproducing dxil.dll AV crashes that would otherwise kill the
// dxilval process before any blob is materialized.
//
// Output paths:
//
//   - If `out` ends in ".dxil" and there is exactly one entry point,
//     writes that single blob to `out`.
//   - Otherwise treats `out` as a prefix and writes one file per entry
//     point as `<prefix>_<stage>_<name>.dxil`.
func runDump(path, entry, out string, stdout, stderr io.Writer) int {
	blobs, names, stages, err := compileWGSL(path, entry)
	if err != nil {
		fmt.Fprintf(stderr, "dxilval: compile %s: %v\n", path, err)
		return ExitFail
	}
	if len(blobs) == 0 {
		fmt.Fprintf(stderr, "dxilval: %s has no compilable entry points\n", path)
		return ExitFail
	}

	// Single-blob shortcut when the output path looks like a concrete
	// .dxil file and there is exactly one entry point to emit.
	if len(blobs) == 1 && strings.HasSuffix(strings.ToLower(out), ".dxil") {
		if err := writeBlob(out, blobs[0]); err != nil {
			fmt.Fprintf(stderr, "dxilval: write %s: %v\n", out, err)
			return ExitFail
		}
		fmt.Fprintf(stdout, "%s: %d bytes (%s/%s)\n", out, len(blobs[0]), stageString(stages[0]), names[0])
		return ExitOK
	}

	// Multi-entry-point: treat `out` as a prefix, strip a trailing
	// .dxil if present, then emit one file per entry point.
	prefix := strings.TrimSuffix(out, filepath.Ext(out))
	if prefix == "" {
		prefix = "blob"
	}
	for i, blob := range blobs {
		p := fmt.Sprintf("%s_%s_%s.dxil", prefix, stageString(stages[i]), names[i])
		if err := writeBlob(p, blob); err != nil {
			fmt.Fprintf(stderr, "dxilval: write %s: %v\n", p, err)
			return ExitFail
		}
		fmt.Fprintf(stdout, "%s: %d bytes\n", p, len(blob))
	}
	return ExitOK
}

func writeBlob(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}
