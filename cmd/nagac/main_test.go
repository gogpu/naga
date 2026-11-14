package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// minimalShader is the smallest WGSL module nagac compiles to SPIR-V.
const minimalShader = "@compute @workgroup_size(1)\nfn main() {}\n"

// spirvMagic is the first word of every SPIR-V binary, little-endian.
var spirvMagic = []byte{0x03, 0x02, 0x23, 0x07}

var (
	buildOnce sync.Once
	builtPath string
	errBuild  error
)

// nagacBinary builds the command once per test run and returns its path. The
// tests drive the real binary, so they see exactly what a shell user sees:
// the exit status, stderr and stdout, and the flag package's own behavior.
func nagacBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "nagac-test-")
		if err != nil {
			errBuild = err
			return
		}
		builtPath = filepath.Join(dir, "nagac-test.exe")
		out, err := exec.Command("go", "build", "-o", builtPath, ".").CombinedOutput()
		if err != nil {
			errBuild = errors.New("go build: " + err.Error() + "\n" + string(out))
		}
	})
	if errBuild != nil {
		t.Fatalf("building nagac: %v", errBuild)
	}
	return builtPath
}

// runNagac runs the built command and returns stdout, stderr and the exit status.
func runNagac(t *testing.T, args ...string) (stdout, stderr []byte, status int) {
	t.Helper()
	cmd := exec.Command(nagacBinary(t), args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		status = 0
	case errors.As(err, &exitErr):
		status = exitErr.ExitCode()
	default:
		t.Fatalf("running nagac: %v", err)
	}
	return out.Bytes(), errOut.Bytes(), status
}

func writeShader(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.wgsl")
	if err := os.WriteFile(path, []byte(minimalShader), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFlagsBeforeInputCompileToFile(t *testing.T) {
	in := writeShader(t)
	out := filepath.Join(t.TempDir(), "a.spv")

	stdout, stderr, status := runNagac(t, "-o", out, in)
	if status != 0 {
		t.Fatalf("exit status = %d, want 0; stderr: %s", status, stderr)
	}
	if bytes.HasPrefix(stdout, spirvMagic) {
		t.Error("binary SPIR-V written to stdout although -o was given")
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output file was not written: %v", err)
	}
	if !bytes.HasPrefix(got, spirvMagic) {
		t.Errorf("output file does not start with the SPIR-V magic number: % x", got[:min(len(got), 4)])
	}
	// The header word after the magic is the module version, 1.3 by default
	// (0x00010300 little-endian).
	if len(got) < 8 || got[4] != 0x00 || got[5] != 0x03 || got[6] != 0x01 || got[7] != 0x00 {
		t.Errorf("SPIR-V header version = % x, want 1.3 (0x00010300 little-endian)", got[4:min(len(got), 8)])
	}
}

func TestArgumentsAfterInputAreRejected(t *testing.T) {
	in := writeShader(t)
	out := filepath.Join(t.TempDir(), "a.spv")

	tests := []struct {
		name string
		args []string
		// named lists the stray arguments the message must mention.
		named []string
	}{
		{"flag after input", []string{in, "-o", out}, []string{"-o", out}},
		{"second input", []string{in, in}, []string{in}},
		{"debug flag after input", []string{in, "-debug"}, []string{"-debug"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, status := runNagac(t, tt.args...)
			if status != exitUsage {
				t.Errorf("exit status = %d, want %d", status, exitUsage)
			}
			if len(stdout) != 0 {
				t.Errorf("stdout has %d bytes, want none (a usage error must not compile)", len(stdout))
			}
			for _, want := range tt.named {
				if !strings.Contains(string(stderr), want) {
					t.Errorf("stderr does not name the stray argument %q:\n%s", want, stderr)
				}
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("output file %s was written despite the usage error", out)
			}
		})
	}
}

func TestNoInputIsAnError(t *testing.T) {
	_, stderr, status := runNagac(t)
	if status == 0 {
		t.Error("exit status = 0, want non-zero")
	}
	if !strings.Contains(string(stderr), "no input file") {
		t.Errorf("stderr does not say the input is missing:\n%s", stderr)
	}
}

func TestVersionFlag(t *testing.T) {
	stdout, _, status := runNagac(t, "-version")
	if status != 0 {
		t.Fatalf("exit status = %d, want 0", status)
	}
	if !strings.HasPrefix(string(stdout), "nagac version ") {
		t.Errorf("stdout = %q, want a \"nagac version\" line", stdout)
	}
}

// TestMain removes the binary built by nagacBinary.
func TestMain(m *testing.M) {
	code := m.Run()
	if builtPath != "" {
		if err := os.RemoveAll(filepath.Dir(builtPath)); err != nil {
			fmt.Fprintf(os.Stderr, "removing the test binary: %v\n", err)
			code = 1
		}
	}
	os.Exit(code)
}

func TestBinaryToTerminalIsRefused(t *testing.T) {
	in := writeShader(t)
	out := filepath.Join(t.TempDir(), "a.spv")

	t.Run("stdout terminal without -o", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		status := run([]string{in}, &stdout, &stderr, true)
		if status != exitUsage {
			t.Errorf("exit status = %d, want %d", status, exitUsage)
		}
		if stdout.Len() != 0 {
			t.Errorf("wrote %d bytes to the terminal", stdout.Len())
		}
		if !strings.Contains(stderr.String(), "-o") {
			t.Errorf("stderr does not ask for -o:\n%s", &stderr)
		}
	})

	t.Run("stdout terminal with -o is fine", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if status := run([]string{"-o", out, in}, &stdout, &stderr, true); status != exitOK {
			t.Fatalf("exit status = %d, want 0; stderr: %s", status, &stderr)
		}
		if _, err := os.Stat(out); err != nil {
			t.Errorf("output file was not written: %v", err)
		}
	})

	t.Run("redirected stdout gets the binary", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if status := run([]string{in}, &stdout, &stderr, false); status != exitOK {
			t.Fatalf("exit status = %d, want 0; stderr: %s", status, &stderr)
		}
		if !bytes.HasPrefix(stdout.Bytes(), spirvMagic) {
			t.Error("stdout does not start with the SPIR-V magic number")
		}
	})
}

func TestIsTerminal(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		f, err := os.Create(filepath.Join(t.TempDir(), "f"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if isTerminal(f) {
			t.Error("a regular file is not a terminal")
		}
	})
	t.Run("null device", func(t *testing.T) {
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if isTerminal(f) {
			t.Error("the null device must not count as a terminal")
		}
	})
}

// fragmentShader compiles to a stage every text backend can express, unlike
// the compute-only minimalShader that GLSL 3.30 cannot.
const fragmentShader = "@fragment\nfn main() -> @location(0) vec4<f32> {\n\treturn vec4<f32>(1.0);\n}\n"

// writeSource writes a shader to a temporary file and returns its path.
func writeSource(t *testing.T, name, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFromSPIRVToTextBackends compiles WGSL to SPIR-V and feeds the binary
// back through -from-spirv into every text backend: the round trip a CLI user
// runs, which the frontend's corpus tests cannot see.
func TestFromSPIRVToTextBackends(t *testing.T) {
	in := writeSource(t, "t.wgsl", fragmentShader)
	dir := t.TempDir()
	spvPath := filepath.Join(dir, "a.spv")
	if _, stderr, status := runNagac(t, "-o", spvPath, in); status != exitOK {
		t.Fatalf("compiling WGSL: exit status = %d; stderr: %s", status, stderr)
	}

	tests := []struct {
		backend string
		want    string
	}{
		{"msl", "fragment"},
		{"hlsl", "SV_Target"},
		{"glsl", "void main"},
	}
	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			out := filepath.Join(dir, "a."+tt.backend)
			stdout, stderr, status := runNagac(t, "-from-spirv", "-backend", tt.backend, "-o", out, spvPath)
			if status != exitOK {
				t.Fatalf("exit status = %d, want %d; stderr: %s", status, exitOK, stderr)
			}
			if bytes.HasPrefix(stdout, spirvMagic) {
				t.Error("SPIR-V written although a text backend was selected")
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("output file was not written: %v", err)
			}
			if !strings.Contains(string(data), tt.want) {
				t.Errorf("%s output does not contain %q:\n%s", tt.backend, tt.want, data)
			}
		})
	}
}

// TestFromSPIRVTruncated checks a malformed binary is a compilation failure,
// not a panic and not a silent success.
func TestFromSPIRVTruncated(t *testing.T) {
	path := writeSource(t, "bad.spv", string(append(append([]byte{}, spirvMagic...), 0, 0, 0, 0)))
	_, stderr, status := runNagac(t, "-from-spirv", path)
	if status != exitFailure {
		t.Errorf("exit status = %d, want %d", status, exitFailure)
	}
	if !strings.Contains(string(stderr), "Compilation error") {
		t.Errorf("stderr does not report a compilation error:\n%s", stderr)
	}
}

// TestUnknownBackendIsRejected checks a misspelled backend is a usage error
// that names the accepted values, not a failed compile.
func TestUnknownBackendIsRejected(t *testing.T) {
	in := writeShader(t)
	stdout, stderr, status := runNagac(t, "-backend", "dxil", in)
	if status != exitUsage {
		t.Errorf("exit status = %d, want %d", status, exitUsage)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout has %d bytes, want none", len(stdout))
	}
	if !strings.Contains(string(stderr), "unknown backend") {
		t.Errorf("stderr does not name the unknown backend:\n%s", stderr)
	}
}

// TestTextBackendMayWriteToTerminal checks the terminal guard covers only the
// binary SPIR-V output, not text.
func TestTextBackendMayWriteToTerminal(t *testing.T) {
	in := writeShader(t)
	var stdout, stderr bytes.Buffer
	if status := run([]string{"-backend", "msl", in}, &stdout, &stderr, true); status != exitOK {
		t.Fatalf("exit status = %d, want %d; stderr: %s", status, exitOK, &stderr)
	}
	if !strings.Contains(stdout.String(), "kernel") {
		t.Errorf("stdout is not MSL kernel source:\n%s", &stdout)
	}
}

// volatileShader is a storage buffer declared @volatile, which only the
// backends whose supported list has MEMORY_DECORATION_VOLATILE may write.
const volatileShader = "@group(0) @binding(0) @volatile var<storage, read_write> data: array<u32>;\n" +
	"@compute @workgroup_size(1) fn main() { data[0] = 1u; }\n"

// TestCapabilitiesFlag: -capabilities bounds what the input may use, and a
// text backend intersects it with its supported list — MSL has no volatile,
// the SPIR-V backend has.
func TestCapabilitiesFlag(t *testing.T) {
	in := writeSource(t, "volatile.wgsl", volatileShader)

	t.Run("spirv accepts volatile", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "a.spv")
		_, stderr, status := runNagac(t, "-o", out, in)
		if status != exitOK {
			t.Fatalf("exit status = %d; stderr: %s", status, stderr)
		}
	})
	t.Run("msl refuses volatile", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "a.msl")
		_, stderr, status := runNagac(t, "-backend", "msl", "-o", out, in)
		if status != exitFailure {
			t.Fatalf("exit status = %d, want %d; stderr: %s", status, exitFailure, stderr)
		}
		if !strings.Contains(string(stderr), "MEMORY_DECORATION_VOLATILE") {
			t.Errorf("stderr does not name the missing capability:\n%s", stderr)
		}
	})
	t.Run("a bounded set", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "a.spv")
		if _, stderr, status := runNagac(t, "-capabilities", "MEMORY_DECORATION_COHERENT", "-o", out, in); status != exitFailure {
			t.Fatalf("exit status = %d, want %d; stderr: %s", status, exitFailure, stderr)
		}
		if _, stderr, status := runNagac(t, "-capabilities", "all", "-o", out, in); status != exitOK {
			t.Fatalf("all: exit status = %d; stderr: %s", status, stderr)
		}
	})
	t.Run("unknown name", func(t *testing.T) {
		_, stderr, status := runNagac(t, "-capabilities", "NOPE", in)
		if status != exitUsage {
			t.Errorf("exit status = %d, want %d", status, exitUsage)
		}
		if !strings.Contains(string(stderr), "unknown capability") {
			t.Errorf("stderr does not name the unknown capability:\n%s", stderr)
		}
	})
}
