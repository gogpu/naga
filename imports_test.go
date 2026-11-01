package naga_test

import (
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/gogpu/naga"

// TestPackageLayering pins the layering of Rust naga (lib.rs): the IR is
// data that depends on nothing else of naga; the validator (valid) builds on
// the IR and the typifier only; and a backend types the module it writes but
// never validates it, so no backend reaches valid.
func TestPackageLayering(t *testing.T) {
	imports := moduleImports(t)
	if _, ok := imports[modulePath+"/valid"]; !ok {
		t.Fatalf("package %s/valid does not exist", modulePath)
	}

	for _, imp := range imports[modulePath+"/ir"] {
		t.Errorf("ir imports %s; the IR depends on no other naga package", imp)
	}

	allowed := []string{modulePath + "/ir", modulePath + "/internal/typifier"}
	for _, imp := range imports[modulePath+"/valid"] {
		if !slices.Contains(allowed, imp) {
			t.Errorf("valid imports %s; it may import only %v of naga", imp, allowed)
		}
	}

	for pkg := range imports {
		backend := false
		for _, b := range []string{"spirv", "msl", "glsl", "hlsl", "dxil"} {
			if pkg == modulePath+"/"+b || strings.HasPrefix(pkg, modulePath+"/"+b+"/") {
				backend = true
			}
		}
		if !backend {
			continue
		}
		if slices.Contains(transitive(imports, pkg), modulePath+"/valid") {
			t.Errorf("%s reaches valid; a backend's Compile types the module, never validates it", pkg)
		}
	}
}

// moduleImports returns the naga packages each package of the module
// imports, outside its tests, on every target system: go/build reads the
// sources, nothing is run.
func moduleImports(t *testing.T) map[string][]string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root {
			if name == "testdata" || name == "tmp" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir // a module of its own
			}
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		pkg := modulePath
		if rel != "." {
			pkg += "/" + filepath.ToSlash(rel)
		}
		for _, goos := range []string{"windows", "linux", "darwin"} {
			ctx := build.Default
			ctx.GOOS, ctx.GOARCH = goos, "amd64"
			p, err := ctx.ImportDir(path, 0)
			if err != nil {
				continue // no Go files for this system
			}
			if _, ok := out[pkg]; !ok {
				out[pkg] = nil
			}
			for _, imp := range p.Imports {
				if (imp == modulePath || strings.HasPrefix(imp, modulePath+"/")) && !slices.Contains(out[pkg], imp) {
					out[pkg] = append(out[pkg], imp)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// transitive returns every naga package pkg reaches.
func transitive(imports map[string][]string, pkg string) []string {
	var seen []string
	var visit func(string)
	visit = func(p string) {
		for _, imp := range imports[p] {
			if !slices.Contains(seen, imp) {
				seen = append(seen, imp)
				visit(imp)
			}
		}
	}
	visit(pkg)
	return seen
}
