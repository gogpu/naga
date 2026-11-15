// Command glslgoldens generates Rust naga's backend outputs for the vendored
// GLSL input corpus (card P2-GLSLIN-001).
//
// Rust naga's own snapshot harness writes WGSL text for these inputs, and our
// WGSL backend is post-v1.0, so the MSL, HLSL and SPIR-V reference outputs are
// generated here with the pinned naga-cli and committed under
// snapshot/testdata/rust-derived/glsl (see that tree's README.md).
//
// Usage:
//
//	go run ./internal/cmd/glslgoldens -naga <naga-cli> \
//	    -in snapshot/testdata/rust/in/glsl \
//	    -out snapshot/testdata/rust-derived/glsl
//
// The tool needs the pinned naga-cli locally; CI never runs it. It replaces the
// msl, hlsl and spv subdirectories of -out and writes SKIPPED.md, which names
// every shader the pinned naga could not translate for a backend and the first
// line of its error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// backends are the outputs to generate, in the order they are tried. The
// pinned naga-cli picks the backend from the output file's extension.
var backends = []struct {
	name string // subdirectory under -out
	ext  string // output file extension
}{
	{"msl", ".metal"},
	{"hlsl", ".hlsl"},
	{"spv", ".spv"},
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "glslgoldens:", err)
		os.Exit(1)
	}
}

// translator runs the pinned naga-cli on one input for one backend and returns
// the tool's error output. The output file exists only on success.
type translator func(naga, stage, input, output string) (string, error)

func run(args []string, stdout io.Writer) error {
	fl := flag.NewFlagSet("glslgoldens", flag.ContinueOnError)
	naga := fl.String("naga", "", "path to the pinned naga-cli")
	in := fl.String("in", "", "vendored GLSL corpus directory (in/glsl)")
	out := fl.String("out", "", "derived GLSL tree to write (rust-derived/glsl)")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if *naga == "" || *in == "" || *out == "" {
		return errors.New("glslgoldens: -naga, -in and -out are required")
	}
	return generate(*naga, *in, *out, translateImpl, stdout)
}

// translateImpl is the translator run calls: the real naga-cli in the command,
// a fake in tests.
var translateImpl translator = translateNaga

// generate writes every input's outputs into the msl, hlsl and spv
// subdirectories of out, replacing them wholesale, and records the refusals in
// out/SKIPPED.md. translate is the naga-cli under the caller's control: the
// command passes translateNaga, tests pass a fake.
func generate(naga, in, out string, translate translator, stdout io.Writer) error {
	inputs, err := glslInputs(in)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return fmt.Errorf("glslgoldens: no .vert/.frag/.comp inputs under %s", in)
	}

	// Generate into a temporary tree on the destination's volume, so a failed
	// run never leaves a half-updated corpus behind.
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(out), ".glslgoldens-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	for _, be := range backends {
		if err := os.MkdirAll(filepath.Join(work, be.name), 0o750); err != nil {
			return err
		}
	}

	var skips []skip
	generated := 0
	for _, input := range inputs {
		stage, ok := stageOf(input)
		if !ok {
			continue
		}
		base := filepath.Base(input)
		for _, be := range backends {
			dst := filepath.Join(work, be.name, "glsl-"+base+be.ext)
			stderr, err := translate(naga, stage, input, dst)
			if err != nil {
				skips = append(skips, skip{Shader: base, Backend: be.name, Error: firstError(stderr)})
				continue
			}
			generated++
		}
	}

	for _, be := range backends {
		dst := filepath.Join(out, be.name)
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(work, be.name), dst); err != nil {
			return err
		}
	}
	if err := writeSkipped(filepath.Join(out, "SKIPPED.md"), skips); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "generated %d outputs for %d shaders; %d skipped (SKIPPED.md)\n",
		generated, len(inputs), len(skips))
	return nil
}

// skip is one shader/backend pair the pinned naga could not translate.
type skip struct {
	Shader  string
	Backend string
	Error   string
}

// glslInputs lists the GLSL sources under dir, sorted. Parameters (.toml) and
// anything else are ignored.
func glslInputs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var inputs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := stageOf(e.Name()); ok {
			inputs = append(inputs, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(inputs)
	return inputs, nil
}

// stageOf maps a GLSL source file name to the naga-cli shader stage.
func stageOf(name string) (string, bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".vert":
		return "vert", true
	case ".frag":
		return "frag", true
	case ".comp":
		return "comp", true
	}
	return "", false
}

// command runs name with args and returns the process's standard error output.
// It is a variable so tests can replace the process launch.
var command = func(name string, args ...string) (string, error) {
	var stderr strings.Builder
	cmd := exec.Command(name, args...) //nolint:gosec // the tool runs the caller-named binary
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

// translateNaga runs the pinned naga-cli on one input for one backend and
// returns its combined error output. The output file exists only on success.
func translateNaga(naga, stage, input, output string) (string, error) {
	return command(naga,
		"--input-kind", "glsl",
		"--shader-stage", stage,
		"--compact",
		input, output,
	)
}

// ansiEscape matches the terminal color codes naga's error writer emits.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// firstError returns the first non-empty line of naga's error output with the
// terminal colors removed, shortened for a Markdown table cell.
func firstError(stderr string) string {
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(stderr, ""), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > 200 {
			line = string([]rune(line)[:200]) + "…"
		}
		return line
	}
	return "(no message on stderr)"
}

// writeSkipped writes SKIPPED.md, the record of what the pinned naga could not
// translate. The file is deterministic: skips are sorted by shader and backend.
func writeSkipped(path string, skips []skip) error {
	sort.Slice(skips, func(i, j int) bool {
		if skips[i].Shader != skips[j].Shader {
			return skips[i].Shader < skips[j].Shader
		}
		return skips[i].Backend < skips[j].Backend
	})
	var b strings.Builder
	b.WriteString("# Outputs the pinned naga does not produce\n\n")
	b.WriteString("Generated by `internal/cmd/glslgoldens`. Every shader not listed here has all\n")
	b.WriteString("three outputs. A row is a shader the pinned naga-cli could not translate for\n")
	b.WriteString("that backend, with the first line of its error: the reason is the reference\n")
	b.WriteString("implementation's, recorded so a comparison test can skip it knowingly.\n\n")
	if len(skips) == 0 {
		b.WriteString("No skips: the pinned naga produced all three outputs for every shader.\n")
		return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // generated corpus file, public test data
	}
	b.WriteString("| Shader | Backend | Error |\n|---|---|---|\n")
	for _, s := range skips {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", s.Shader, s.Backend, strings.ReplaceAll(s.Error, "|", "\\|"))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // generated corpus file, public test data
}
