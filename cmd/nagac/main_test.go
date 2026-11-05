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
