// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package gen regenerates the SPIR-V capability table of internal/spvnames
// from the SPIR-V grammar and checks every name against spirv.h of the same
// headers. It lives in its own package so the table's drift test can call it:
// the production package must not pull encoding/json and go/format into its
// importers just to be testable.
package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// HeadersRevision names the SPIR-V headers revision the vendored copy under
// testdata/upstream comes from. The generated table records it, so a
// regeneration has a baseline to compare against, and the drift test compares
// the whole file including this line.
const HeadersRevision = "vulkan-sdk-1.4.341.0 (04f10f650d514df88b76d25e83db360142c7b174)"

// Report counts what Generate read from the two files.
type Report struct {
	Capabilities int // capability enumerants in the grammar
	Aliases      int // further names the grammar gives them
	HeaderNames  int // names in the SpvCapability enum of spirv.h
}

// Generate reads the SPIR-V grammar and spirv.h, checks one against the other
// and returns the Go source of table.go.
func Generate(header, grammar io.Reader) ([]byte, Report, error) {
	table, err := readGrammar(grammar)
	if err != nil {
		return nil, Report{}, err
	}
	headerNames, err := readHeader(header)
	if err != nil {
		return nil, Report{}, err
	}
	if err := check(table, headerNames); err != nil {
		return nil, Report{}, err
	}
	report := Report{Capabilities: len(table), HeaderNames: len(headerNames)}
	for _, c := range table {
		report.Aliases += len(c.aliases)
	}
	src, err := writeTable(table)
	if err != nil {
		return nil, Report{}, err
	}
	return src, report, nil
}

// enumerant is one enumerant of an operand kind in the grammar. The value is
// kept raw: capability values are numbers, other kinds spell theirs as
// strings.
type enumerant struct {
	Enumerant string          `json:"enumerant"`
	Value     json.RawMessage `json:"value"`
	Aliases   []string        `json:"aliases"`
}

// grammar is the part of spirv.core.grammar.json this generator reads.
type grammar struct {
	OperandKinds []struct {
		Kind       string      `json:"kind"`
		Enumerants []enumerant `json:"enumerants"`
	} `json:"operand_kinds"`
}

// capEntry is one capability as the generator holds it. Its mirror in the
// generated package is spvnames.capability.
type capEntry struct {
	value   uint32
	name    string
	aliases []string
}

// readGrammar returns the capability enumerants of the grammar, by number.
func readGrammar(r io.Reader) ([]capEntry, error) {
	var g grammar
	if err := json.NewDecoder(r).Decode(&g); err != nil {
		return nil, fmt.Errorf("grammar: %w", err)
	}
	for _, kind := range g.OperandKinds {
		if kind.Kind != "Capability" {
			continue
		}
		table := make([]capEntry, 0, len(kind.Enumerants))
		for _, e := range kind.Enumerants {
			var value uint32
			if err := json.Unmarshal(e.Value, &value); err != nil {
				return nil, fmt.Errorf("grammar: capability %q value: %w", e.Enumerant, err)
			}
			aliases := append([]string(nil), e.Aliases...)
			sort.Strings(aliases)
			table = append(table, capEntry{value: value, name: e.Enumerant, aliases: aliases})
		}
		sort.Slice(table, func(i, j int) bool { return table[i].value < table[j].value })
		return table, nil
	}
	return nil, errors.New("grammar: no Capability operand kind")
}

// spvCapabilityPattern matches one SpvCapability<Name> = <number> line of the
// SpvCapability enum in spirv.h.
var spvCapabilityPattern = regexp.MustCompile(`^\s*SpvCapability([A-Za-z0-9_]+) = (0[xX][0-9a-fA-F]+|[0-9]+),?$`)

// readHeader returns the name-to-number map of the SpvCapability enum in
// spirv.h. SpvCapabilityMax, the sentinel that ends the enum, is not a
// capability and is left out.
func readHeader(r io.Reader) (map[string]uint32, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("spirv.h: %w", err)
	}
	header := make(map[string]uint32)
	inEnum := false
	for lineNo, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if !inEnum {
			inEnum = trimmed == "typedef enum SpvCapability_ {"
			continue
		}
		if trimmed == "} SpvCapability;" {
			return header, nil
		}
		m := spvCapabilityPattern.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		if m[1] == "Max" {
			continue
		}
		value, err := strconv.ParseUint(m[2], 0, 32)
		if err != nil {
			return nil, fmt.Errorf("spirv.h:%d: %w", lineNo+1, err)
		}
		header[m[1]] = uint32(value)
	}
	return nil, errors.New("spirv.h: no SpvCapability enum")
}

// check verifies the grammar table against spirv.h: the same numbers, the same
// canonical name for every number, every alias at its capability's number, and
// nothing in one file that is missing from the other.
func check(table []capEntry, header map[string]uint32) error {
	seenNumber := make(map[uint32]string, len(table))
	seenName := make(map[string]bool, len(table))
	var problems []string
	for _, c := range table {
		if prev, ok := seenNumber[c.value]; ok {
			problems = append(problems, fmt.Sprintf("%d has two capabilities: %s and %s", c.value, prev, c.name))
		}
		seenNumber[c.value] = c.name
		if seenName[c.name] {
			problems = append(problems, fmt.Sprintf("capability name %s appears twice", c.name))
		}
		seenName[c.name] = true
		if got, ok := header[c.name]; !ok {
			problems = append(problems, fmt.Sprintf("%s (%d) is not in spirv.h", c.name, c.value))
		} else if got != c.value {
			problems = append(problems, fmt.Sprintf("%s: grammar %d, spirv.h %d", c.name, c.value, got))
		}
		for _, alias := range c.aliases {
			if got, ok := header[alias]; !ok {
				problems = append(problems, fmt.Sprintf("alias %s of %s is not in spirv.h", alias, c.name))
			} else if got != c.value {
				problems = append(problems, fmt.Sprintf("alias %s: grammar %d, spirv.h %d", alias, c.value, got))
			}
			if seenName[alias] {
				problems = append(problems, fmt.Sprintf("alias %s of %s appears twice", alias, c.name))
			}
			seenName[alias] = true
		}
	}
	for name, value := range header {
		if !seenName[name] {
			problems = append(problems, fmt.Sprintf("%s (%d) of spirv.h is not in the grammar", name, value))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New("grammar and spirv.h disagree:\n  " + strings.Join(problems, "\n  "))
	}
	return nil
}

// writeTable returns the formatted Go source of the capability table.
func writeTable(table []capEntry) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Copyright 2026 The GoGPU Authors\n")
	b.WriteString("// SPDX-License-Identifier: MIT\n\n")
	fmt.Fprintf(&b, "// Code generated by gen.go from the SPIR-V grammar of the SPIR-V headers\n")
	fmt.Fprintf(&b, "// %s,\n", HeadersRevision)
	b.WriteString("// checked against spirv.h of the same headers; DO NOT EDIT.\n\n")
	b.WriteString("package spvnames\n\n")
	fmt.Fprintf(&b, "// capabilityTable lists every capability of the SPIR-V specification: %d\n", len(table))
	b.WriteString("// numbers with their canonical name and the aliases the specification gives\n")
	b.WriteString("// them.\n")
	b.WriteString("var capabilityTable = []capability{\n")
	for _, c := range table {
		fmt.Fprintf(&b, "\t{value: %d, name: %q", c.value, c.name)
		if len(c.aliases) > 0 {
			quoted := make([]string, len(c.aliases))
			for i, alias := range c.aliases {
				quoted[i] = strconv.Quote(alias)
			}
			fmt.Fprintf(&b, ", aliases: []string{%s}", strings.Join(quoted, ", "))
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("generated source: %w", err)
	}
	return src, nil
}
