// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package run

import (
	"os"
	"strconv"
	"strings"

	"github.com/gogpu/naga/dxil"
)

// loadHLSLBindingMap reads the sibling .toml of a .wgsl path and returns
// any `[[hlsl.binding_map]]` entries as a dxil.BindingMap. Returns nil if
// the TOML is missing or has no hlsl.binding_map entries.
//
// Supported entry shapes (Rust naga test fixtures use this exact format):
//
//	[[hlsl.binding_map]]
//	bind_target = { binding_array_size = 10, register = 0, space = 0 }
//	resource_binding = { group = 0, binding = 0 }
//
// This is a narrow hand-rolled parser, not a general TOML implementation:
// it recognizes exactly the keys Rust naga emits for HLSL binding-map
// configuration. Adding a real TOML library as a dependency was rejected
// because naga ships with zero external deps.
//
//nolint:gocyclo,cyclop,funlen // line-by-line parser dispatches over key/value/table shapes
func loadHLSLBindingMap(wgslPath string) dxil.BindingMap {
	tomlPath := strings.TrimSuffix(wgslPath, ".wgsl") + ".toml"
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	out := make(dxil.BindingMap)

	inEntry := false
	var target dxil.BindTarget
	var loc dxil.BindingLocation
	haveTarget := false
	haveLoc := false

	flush := func() {
		if haveTarget && haveLoc {
			out[loc] = target
		}
		inEntry = false
		target = dxil.BindTarget{}
		loc = dxil.BindingLocation{}
		haveTarget = false
		haveLoc = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		// New table header of any kind ends the current entry.
		if strings.HasPrefix(line, "[") {
			flush()
			// Only [[hlsl.binding_map]] opens a new entry we care about.
			if line == "[[hlsl.binding_map]]" {
				inEntry = true
			}
			continue
		}
		if !inEntry {
			continue
		}

		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])

		switch key {
		case "bind_target":
			fields := parseInlineTable(val)
			if v, ok := fields["register"]; ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					target.Register = uint32(n)
				}
			}
			if v, ok := fields["space"]; ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					target.Space = uint32(n)
				}
			}
			if v, ok := fields["binding_array_size"]; ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					sz := uint32(n)
					target.BindingArraySize = &sz
				}
			}
			haveTarget = true
		case "resource_binding":
			fields := parseInlineTable(val)
			if v, ok := fields["group"]; ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					loc.Group = uint32(n)
				}
			}
			if v, ok := fields["binding"]; ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					loc.Binding = uint32(n)
				}
			}
			haveLoc = true
		}
	}
	flush()

	if len(out) == 0 {
		return nil
	}
	return out
}

// parseInlineTable parses a TOML inline table literal like
// `{ binding_array_size = 10, register = 0, space = 0 }` into a
// flat key -> raw-value map. Whitespace around tokens is tolerated; keys
// with unsupported value types are silently skipped. Strings and nested
// tables are not supported — we only need integer scalars.
func parseInlineTable(s string) map[string]string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	out := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		eq := strings.Index(part, "=")
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(part[:eq])
		v := strings.TrimSpace(part[eq+1:])
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// stripComment removes a trailing `# ...` comment if any.
func stripComment(s string) string {
	if i := strings.Index(s, "#"); i >= 0 {
		return s[:i]
	}
	return s
}
