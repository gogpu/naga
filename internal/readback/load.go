package readback

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// ReadmeName is the corpus README at the root of the tree, the only file
// outside an area directory.
const ReadmeName = "README.md"

// nameRe matches an area or a case name: lowercase words joined by '-'.
var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Case is one corpus case: <area>/<name>.wgsl and its <area>/<name>.json.
type Case struct {
	// Name is "<area>/<name>", the case path without an extension.
	Name string
	// WGSL is the shader source.
	WGSL string
	// Spec is the decoded and checked JSON file.
	Spec Spec
}

// Load reads every case of the corpus tree fsys, sorted by name. Every file
// but the README must belong to a case, and every case needs both files: a
// stray or half-added file is an error, not a skipped case.
func Load(fsys fs.FS) ([]Case, error) {
	shaders := map[string]bool{}
	specs := map[string]bool{}
	var errs []error
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == ReadmeName {
			return err
		}
		if strings.Count(p, "/") != 1 {
			errs = append(errs, fmt.Errorf("%s: not in an area directory (<area>/<name>.wgsl)", p))
			return nil
		}
		ext := path.Ext(p)
		name := strings.TrimSuffix(p, ext)
		switch ext {
		case ".wgsl":
			shaders[name] = true
		case ".json":
			specs[name] = true
		default:
			errs = append(errs, fmt.Errorf("%s: not a .wgsl or .json file", p))
			return nil
		}
		area, base, _ := strings.Cut(name, "/")
		if !nameRe.MatchString(area) || !nameRe.MatchString(base) {
			errs = append(errs, fmt.Errorf("%s: area and case name must be lowercase words joined by '-'", p))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(specs))
	for name := range specs {
		if !shaders[name] {
			errs = append(errs, fmt.Errorf("%s: no %s.wgsl", name, name))
			continue
		}
		names = append(names, name)
	}
	for name := range shaders {
		if !specs[name] {
			errs = append(errs, fmt.Errorf("%s: no %s.json", name, name))
		}
	}
	sort.Strings(names)

	cases := make([]Case, 0, len(names))
	for _, name := range names {
		c, err := loadCase(fsys, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cases = append(cases, c)
	}
	if len(errs) == 0 && len(cases) == 0 {
		errs = append(errs, errors.New("no cases"))
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cases, nil
}

func loadCase(fsys fs.FS, name string) (Case, error) {
	src, err := fs.ReadFile(fsys, name+".wgsl")
	if err != nil {
		return Case{}, err
	}
	data, err := fs.ReadFile(fsys, name+".json")
	if err != nil {
		return Case{}, err
	}
	spec, err := ParseSpec(data)
	if err != nil {
		return Case{}, fmt.Errorf("%s.json: %w", name, err)
	}
	return Case{Name: name, WGSL: string(src), Spec: spec}, nil
}
