package snapshot_test

// The GLSL text reference of design §18.2: for every derived MSL and HLSL
// golden of the pinned naga-cli, our own pipeline — glsl.Parse, validation,
// compaction, override processing and a text backend — must write the same
// text, or differ in a way recorded in glslTextDeviations with a reason and a
// fingerprint of the compared pair.
//
// The goldens come from `internal/cmd/glslgoldens`, which runs the pinned
// naga-cli with `--compact` and its default backend options; the pipeline here
// mirrors the CLI (naga-cli/src/bin/naga.rs): validate, compact with
// KeepUnused::No, revalidate, process overrides with their default values,
// write. Every difference is a frontend or backend defect until the entry
// recording it says otherwise.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/overrides"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/valid"
)

// glslTextBackend is one text backend of the comparison.
type glslTextBackend struct {
	name string // subdirectory of the derived tree and the SKIPPED.md name
	ext  string // extension of the golden files
}

var glslTextBackends = []glslTextBackend{
	{name: "msl", ext: ".metal"},
	{name: "hlsl", ext: ".hlsl"},
}

// glslTextGolden resolves the golden of one backend and source name.
func glslTextGolden(b glslTextBackend, shader string) string {
	return filepath.Join(rustDerivedDir, "glsl", b.name, "glsl-"+shader+b.ext)
}

// TestGLSLTextReference compares our text backends with Rust naga's derived
// outputs for the vendored GLSL corpus.
func TestGLSLTextReference(t *testing.T) {
	inputDir := filepath.Join(rustCorpusDir, "in", "glsl")
	skipped := readGLSLSkipped(t)

	usedDeviations := map[*rustDeviation]bool{}
	total, compared, refused, notProduced := 0, 0, 0, 0
	for _, b := range glslTextBackends {
		goldens, err := filepath.Glob(filepath.Join(rustDerivedDir, "glsl", b.name, "glsl-*"+b.ext))
		if err != nil {
			t.Fatal(err)
		}
		if len(goldens) == 0 {
			t.Fatalf("no %s goldens under %s", b.name, filepath.Join(rustDerivedDir, "glsl", b.name))
		}
		for _, golden := range goldens {
			total++
			shader := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(golden), "glsl-"), b.ext)
			file := filepath.Base(golden)
			t.Run(shader+"/"+b.name, func(t *testing.T) {
				if reason, ok := skipped[glslSkipKey{shader: shader, backend: b.name}]; ok {
					t.Fatalf("the pinned naga produced no %s output (%s), but a golden exists", b.name, reason)
				}
				sourcePath := filepath.Join(inputDir, shader)
				source, err := os.ReadFile(sourcePath)
				if err != nil {
					t.Fatalf("no vendored source for the golden: %v", err)
				}
				module, _, parseErr := glsl.Parse(string(source), glsl.ParseOptions{Stage: glslStageOf(t, sourcePath)})
				if parseErr != nil {
					if reason, known := glslKnownRefusals[shader]; known {
						refused++
						t.Logf("refused as recorded (%s): %v", reason, parseErr)
						return
					}
					t.Fatalf("Parse: %v", parseErr)
				}
				ours, compileErr := glslTextOutput(module, b)

				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatal(err)
				}
				expected := normalizeExprNames(strings.ReplaceAll(string(want), "\r\n", "\n"))
				actual := normalizeExprNames(strings.ReplaceAll(ours, "\r\n", "\n"))

				dev := findGLSLTextDeviation(shader, file)
				if dev != nil {
					usedDeviations[dev] = true
				}
				differs := compileErr == nil && expected != actual
				var fingerprint string
				if differs {
					fingerprint = referenceFingerprint(expected, actual)
				}
				verdict := judgeReference(dev, compileErr, differs, fingerprint)
				switch verdict.result {
				case resultPass:
					compared++
				case resultDeviation:
					t.Logf("%v deviation: %s", verdict.kind, dev.Reason)
				default:
					msg := verdict.message
					if differs {
						msg += "\n" + firstDiff(expected, actual)
					}
					t.Error(msg)
				}
			})
		}

		// Every source/backend pair without a golden must be a recorded
		// refusal of the pinned naga, and every recorded refusal must lack its
		// golden: nothing is skipped silently.
		sources, err := glslSources(inputDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, shader := range sources {
			if _, err := os.Stat(glslTextGolden(b, shader)); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			notProduced++
			if _, ok := skipped[glslSkipKey{shader: shader, backend: b.name}]; !ok {
				t.Errorf("%s/%s: no golden and no SKIPPED.md row; regenerate the goldens or record the refusal",
					shader, b.name)
			}
		}
	}

	for i := range glslTextDeviations {
		d := &glslTextDeviations[i]
		if !usedDeviations[d] {
			t.Errorf("deviation %s/%s was not exercised: remove it or fix its shader/file", d.Shader, d.File)
		}
	}
	for key, reason := range skipped {
		if _, err := os.Stat(glslTextGolden(glslTextBackendFor(t, key.backend), key.shader)); err == nil {
			t.Errorf("SKIPPED.md records %s/%s (%s) but the golden exists", key.shader, key.backend, reason)
		}
	}
	t.Logf("%d goldens: %d match Rust, %d deviations, %d refused as recorded, %d pairs without a golden",
		total, compared, len(usedDeviations), refused, notProduced)
}

// glslTextOutput runs our pipeline for one backend and returns its text. The
// steps and their order are the pinned naga-cli's (naga-cli/src/bin/naga.rs:
// validate, --compact, revalidate, process_overrides, write_string).
func glslTextOutput(module *ir.Module, b glslTextBackend) (string, error) {
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		return "", fmt.Errorf("validate: %w", err)
	}
	naga.Compact(module, naga.KeepUnusedNo)
	info, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll})
	if err != nil {
		return "", fmt.Errorf("validate after compact: %w", err)
	}
	// The CLI calls process_overrides before every text backend, with an empty
	// override map — the defaults of the module's pipeline-overridable
	// constants (pipeline_constants.rs does its own compaction).
	module, _, err = overrides.Process(module, info, nil, nil)
	if err != nil {
		return "", fmt.Errorf("process overrides: %w", err)
	}
	switch b.name {
	case "msl":
		src, _, err := msl.Compile(module, glslMSLOptions())
		return src, err
	case "hlsl":
		src, _, err := hlsl.Compile(module, hlsl.Options{})
		return src, err
	default:
		return "", fmt.Errorf("unknown backend %q", b.name)
	}
}

// glslMSLOptions returns the options of the pinned naga-cli, which are Rust
// naga's msl::Options::default() (back/msl/mod.rs:333-350): Metal 1.0, fake
// missing bindings, and Unchecked bounds policies everywhere. The GLSL corpus
// carries no harness parameters, so the CLI's defaults are the whole story.
func glslMSLOptions() msl.Options {
	unchecked := backend.BoundsCheckUnchecked
	return msl.Options{
		LangVersion:         msl.Version1_0,
		PerEntryPointMap:    map[string]msl.EntryPointResources{},
		FakeMissingBindings: true,
		BoundsCheckPolicies: backend.BoundsCheckPolicies{
			Index:        unchecked,
			Buffer:       unchecked,
			ImageLoad:    unchecked,
			BindingArray: unchecked,
		},
	}
}

// glslSkipKey identifies one row of the derived tree's SKIPPED.md.
type glslSkipKey struct {
	shader  string
	backend string
}

// skippedRow matches one data row of SKIPPED.md.
var skippedRow = regexp.MustCompile("^\\| `([^`]+)` \\| ([a-z]+) \\|")

// readGLSLSkipped parses the derived tree's SKIPPED.md: the shader/backend
// pairs the pinned naga could not translate. An unparsable row fails the test,
// so a format change cannot silently empty the record.
func readGLSLSkipped(t *testing.T) map[glslSkipKey]string {
	t.Helper()
	path := filepath.Join(rustDerivedDir, "glsl", "SKIPPED.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the record of the pinned naga's refusals is missing: %v", err)
	}
	rows := make(map[glslSkipKey]string)
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") || strings.Contains(line, "---") {
			continue
		}
		m := skippedRow.FindStringSubmatch(line)
		if m == nil {
			if strings.Contains(line, "| Shader |") {
				continue
			}
			t.Fatalf("unparsable SKIPPED.md row: %q", line)
		}
		rows[glslSkipKey{shader: m[1], backend: m[2]}] = line
	}
	return rows
}

// glslSources lists the vendored GLSL sources, sorted.
func glslSources(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var sources []string
	for _, entry := range entries {
		switch filepath.Ext(entry.Name()) {
		case ".vert", ".frag", ".comp":
			sources = append(sources, entry.Name())
		}
	}
	return sources, nil
}

// glslTextBackendFor maps a SKIPPED.md backend name to the backend record.
func glslTextBackendFor(t *testing.T, name string) glslTextBackend {
	t.Helper()
	for _, b := range glslTextBackends {
		if b.name == name {
			return b
		}
	}
	t.Fatalf("SKIPPED.md names the unknown backend %q", name)
	return glslTextBackend{}
}

// findGLSLTextDeviation returns the recorded deviation for a golden, if any.
func findGLSLTextDeviation(shader, file string) *rustDeviation {
	for i := range glslTextDeviations {
		if glslTextDeviations[i].Shader == shader && glslTextDeviations[i].File == file {
			return &glslTextDeviations[i]
		}
	}
	return nil
}
