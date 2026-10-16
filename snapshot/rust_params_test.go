package snapshot_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// rustTargets mirrors naga-test's Targets bitflags: the outputs Rust naga's
// snapshot driver writes for a shader (naga-test/src/lib.rs).
type rustTargets uint16

const (
	rustTargetIR rustTargets = 1 << iota
	rustTargetANALYSIS
	rustTargetSPIRV
	rustTargetMETAL
	rustTargetGLSL
	rustTargetDOT
	rustTargetHLSL
	rustTargetWGSL
	rustTargetNoValidation
)

// rustTargetsWGSLDefault is Targets::wgsl_default(): the targets of a WGSL
// snapshot whose parameters do not declare any.
const rustTargetsWGSLDefault = rustTargetHLSL | rustTargetSPIRV | rustTargetGLSL | rustTargetMETAL | rustTargetWGSL

// rustTargetNames lists the flags in declaration order, as spelled in TOML.
var rustTargetNames = []struct {
	flag rustTargets
	name string
}{
	{rustTargetIR, "IR"},
	{rustTargetANALYSIS, "ANALYSIS"},
	{rustTargetSPIRV, "SPIRV"},
	{rustTargetMETAL, "METAL"},
	{rustTargetGLSL, "GLSL"},
	{rustTargetDOT, "DOT"},
	{rustTargetHLSL, "HLSL"},
	{rustTargetWGSL, "WGSL"},
	{rustTargetNoValidation, "NO_VALIDATION"},
}

func (t rustTargets) has(flag rustTargets) bool { return t&flag != 0 }

func (t rustTargets) String() string {
	var names []string
	for _, n := range rustTargetNames {
		if t.has(n.flag) {
			names = append(names, n.name)
		}
	}
	return strings.Join(names, " | ")
}

// parseRustTargets parses bitflags syntax ("SPIRV | METAL"). An empty string
// is the empty set, as serde's bitflags deserializer reads it.
func parseRustTargets(s string) (rustTargets, error) {
	var targets rustTargets
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	for item := range strings.SplitSeq(s, "|") {
		name := strings.TrimSpace(item)
		if name == "" {
			return 0, fmt.Errorf("targets %q: empty flag", s)
		}
		found := false
		for _, n := range rustTargetNames {
			if n.name == name {
				targets |= n.flag
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("targets %q: unknown flag %q", s, name)
		}
	}
	return targets, nil
}

// rustParams holds the parts of a shader's Rust test parameters (the TOML
// next to it) that decide which reference outputs exist. Backend options are
// read by the per-backend config readers.
type rustParams struct {
	targets             rustTargets
	separateEntryPoints bool            // [spv] separate_entry_points
	glslExcludeList     map[string]bool // entry points without GLSL output
	boundsCheckPolicies rustBoundsCheckPolicies
}

// rustBoundsCheckPolicy is naga::proc::BoundsCheckPolicy, as the snapshot
// TOML spells its variants.
type rustBoundsCheckPolicy uint8

const (
	rustUnchecked rustBoundsCheckPolicy = iota
	rustRestrict
	rustReadZeroSkipWrite
)

// rustBoundsCheckPolicies is naga::proc::BoundsCheckPolicies, the
// [bounds_check_policies] section. Like the serde(default) struct, a missing
// field is Unchecked; an unknown field or variant is an error.
type rustBoundsCheckPolicies struct {
	index, buffer, imageLoad, bindingArray rustBoundsCheckPolicy
}

func parseRustBoundsCheckPolicy(value string) (rustBoundsCheckPolicy, error) {
	s, ok := tomlString(value)
	if !ok {
		return 0, fmt.Errorf("bounds check policy must be a string, got %s", value)
	}
	switch s {
	case "Unchecked":
		return rustUnchecked, nil
	case "Restrict":
		return rustRestrict, nil
	case "ReadZeroSkipWrite":
		return rustReadZeroSkipWrite, nil
	}
	return 0, fmt.Errorf("unknown bounds check policy %q", s)
}

func (p *rustBoundsCheckPolicies) set(key, value string) error {
	policy, err := parseRustBoundsCheckPolicy(value)
	if err != nil {
		return fmt.Errorf("[bounds_check_policies] %s: %w", key, err)
	}
	switch key {
	case "index":
		p.index = policy
	case "buffer":
		p.buffer = policy
	case "image_load":
		p.imageLoad = policy
	case "binding_array":
		p.bindingArray = policy
	default:
		if !rustIgnoredBoundsCheckFields[key] {
			return fmt.Errorf("[bounds_check_policies]: unknown field %q", key)
		}
	}
	return nil
}

// rustIgnoredBoundsCheckFields are keys in the Rust corpus that are not
// fields of naga::proc::BoundsCheckPolicies. Rust's serde(default) structs
// do not deny unknown fields, so these have no effect there and none here.
// Any other unknown key is an error, to be reviewed when the corpus syncs.
var rustIgnoredBoundsCheckFields = map[string]bool{
	"image": true, // f16-native.toml, f16-polyfill.toml
}

// mustRustParams parses a shader's TOML for the option readers, which have
// no error path; TestRustParamsOfEveryCorpusShader reports parse errors.
func mustRustParams(content string) rustParams {
	p, err := parseRustParams(content)
	if err != nil {
		panic(fmt.Sprintf("snapshot TOML: %v", err))
	}
	return p
}

// loadRustParams reads the parameters of a vendored shader. A shader without
// a TOML file uses Rust's defaults, as in naga-test's read_parameters.
func loadRustParams(shaderName string) (rustParams, error) {
	path := rustParamsPath(shaderName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return rustParams{targets: rustTargetsWGSLDefault}, nil
	}
	if err != nil {
		return rustParams{}, err
	}
	p, err := parseRustParams(string(data))
	if err != nil {
		return rustParams{}, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// parseRustParams extracts the classification parameters from a naga-test
// TOML file. It understands the TOML subset those files use — sections,
// `key = value` pairs, comments, multi-line arrays and inline tables — and
// rejects anything it cannot interpret instead of guessing.
func parseRustParams(content string) (rustParams, error) {
	p := rustParams{}
	targetsSet := false
	err := scanTOML(content, func(section, key, value string) error {
		switch {
		case section == "" && key == "targets":
			s, ok := tomlString(value)
			if !ok {
				return fmt.Errorf("targets must be a string, got %s", value)
			}
			targets, err := parseRustTargets(s)
			if err != nil {
				return err
			}
			p.targets, targetsSet = targets, true
		case section == "" && key == "glsl_exclude_list":
			list, ok := tomlStringArray(value)
			if !ok {
				return fmt.Errorf("glsl_exclude_list must be an array of strings, got %s", value)
			}
			if len(list) > 0 {
				p.glslExcludeList = make(map[string]bool, len(list))
				for _, ep := range list {
					p.glslExcludeList[ep] = true
				}
			}
		case section == "spv" && key == "separate_entry_points":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("[spv] separate_entry_points must be a bool, got %s", value)
			}
			p.separateEntryPoints = b
		case section == "bounds_check_policies":
			return p.boundsCheckPolicies.set(key, value)
		}
		return nil
	})
	if err != nil {
		return rustParams{}, err
	}
	if !targetsSet {
		p.targets = rustTargetsWGSLDefault
	}
	return p, nil
}

// scanTOML calls fn for every `key = value` pair with its section ("" for
// the top level). Values spanning lines (arrays, inline tables) are joined.
// Duplicate keys within a section are an error, as in TOML.
func scanTOML(content string, fn func(section, key, value string) error) error {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	section := ""
	seen := map[string]bool{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripTOMLComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			name, isArray, err := tomlSectionName(line)
			if err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
			section = name
			if isArray {
				// Each [[table]] starts a new element: its keys may repeat.
				for k := range seen {
					if strings.HasPrefix(k, name+".") {
						delete(seen, k)
					}
				}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: expected key = value, got %q", i+1, line)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		start := i
		for depth := bracketDepth(value); depth > 0; depth = bracketDepth(value) {
			i++
			if i >= len(lines) {
				return fmt.Errorf("line %d: unterminated value for %s", start+1, key)
			}
			value += " " + strings.TrimSpace(stripTOMLComment(lines[i]))
		}
		full := section + "." + key
		if seen[full] {
			return fmt.Errorf("line %d: duplicate key %s", start+1, strings.TrimPrefix(full, "."))
		}
		seen[full] = true
		if err := fn(section, key, value); err != nil {
			return fmt.Errorf("line %d: %w", start+1, err)
		}
	}
	return nil
}

// tomlSectionName parses a `[table]` or `[[array.of.tables]]` header.
func tomlSectionName(line string) (name string, isArray bool, err error) {
	isArray = strings.HasPrefix(line, "[[")
	open, closing := "[", "]"
	if isArray {
		open, closing = "[[", "]]"
	}
	inner, ok := strings.CutPrefix(line, open)
	if ok {
		inner, ok = strings.CutSuffix(inner, closing)
	}
	name = strings.TrimSpace(inner)
	if !ok || name == "" || strings.ContainsAny(name, "[]") {
		return "", false, fmt.Errorf("malformed section header %q", line)
	}
	return name, isArray, nil
}

// stripTOMLComment removes a `#` comment that is not inside a string.
func stripTOMLComment(line string) string {
	inString := false
	for i, c := range line {
		switch {
		case c == '"' && (i == 0 || line[i-1] != '\\'):
			inString = !inString
		case c == '#' && !inString:
			return line[:i]
		}
	}
	return line
}

// bracketDepth returns the number of unclosed [ and { outside strings.
func bracketDepth(value string) int {
	depth, inString := 0, false
	for i, c := range value {
		switch {
		case c == '"' && (i == 0 || value[i-1] != '\\'):
			inString = !inString
		case inString:
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		}
	}
	return depth
}

// tomlString unquotes a basic TOML string without escapes.
func tomlString(value string) (string, bool) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	s := value[1 : len(value)-1]
	if strings.ContainsAny(s, `"\`) {
		return "", false
	}
	return s, true
}

// tomlStringArray parses an array of basic strings.
func tomlStringArray(value string) ([]string, bool) {
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, false
	}
	var out []string
	for item := range strings.SplitSeq(value[1:len(value)-1], ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue // trailing comma
		}
		s, ok := tomlString(item)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
