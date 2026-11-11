//go:build ignore

// gen generates table.go from the SPIR-V grammar and checks every name it
// emits against spirv.h of the same SPIR-V headers package:
//
//	go run gen.go -header <spirv-headers>/spirv.h \
//	              -grammar <spirv-headers>/spirv.core.grammar.json
//
// The files ship together in the SPIR-V headers. Every capability of the
// grammar must be in spirv.h with the same number, and every capability of
// spirv.h must be in the grammar, aliases included; a mismatch stops
// generation. Run it again when the SPIR-V headers are updated: a capability
// the table does not know prints as a bare number, and its name is rejected as
// unknown.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

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

func main() {
	headerPath := flag.String("header", "", "path to spirv.h of the SPIR-V headers")
	grammarPath := flag.String("grammar", "", "path to spirv.core.grammar.json of the SPIR-V headers")
	outPath := flag.String("out", "table.go", "Go file to write")
	flag.Parse()
	if *headerPath == "" || *grammarPath == "" {
		fmt.Fprintln(os.Stderr, "gen: -header and -grammar are required")
		os.Exit(2)
	}
	if err := run(*headerPath, *grammarPath, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

// run reads the grammar and spirv.h, checks one against the other and writes
// the table.
func run(headerPath, grammarPath, outPath string) error {
	table, err := readGrammar(grammarPath)
	if err != nil {
		return err
	}
	header, err := readHeader(headerPath)
	if err != nil {
		return err
	}
	if err := check(table, header); err != nil {
		return err
	}
	aliases := 0
	for _, c := range table {
		aliases += len(c.aliases)
	}
	fmt.Printf("grammar: %d capabilities, %d aliases; spirv.h: %d names, %d numbers; mismatches: 0\n",
		len(table), aliases, len(header), len(table))
	return writeTable(outPath, table)
}

// readGrammar returns the capability enumerants of the grammar, by number.
func readGrammar(path string) ([]capEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g grammar
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, kind := range g.OperandKinds {
		if kind.Kind != "Capability" {
			continue
		}
		table := make([]capEntry, 0, len(kind.Enumerants))
		for _, e := range kind.Enumerants {
			var value uint32
			if err := json.Unmarshal(e.Value, &value); err != nil {
				return nil, fmt.Errorf("%s: capability %q value: %w", path, e.Enumerant, err)
			}
			aliases := append([]string(nil), e.Aliases...)
			sort.Strings(aliases)
			table = append(table, capEntry{value: value, name: e.Enumerant, aliases: aliases})
		}
		sort.Slice(table, func(i, j int) bool { return table[i].value < table[j].value })
		return table, nil
	}
	return nil, fmt.Errorf("%s: no Capability operand kind", path)
}

// spvCapabilityPattern matches one SpvCapability<Name> = <number> line of the
// SpvCapability enum in spirv.h.
var spvCapabilityPattern = regexp.MustCompile(`^\s*SpvCapability([A-Za-z0-9_]+) = (0[xX][0-9a-fA-F]+|[0-9]+),?$`)

// readHeader returns the name-to-number map of the SpvCapability enum in
// spirv.h. SpvCapabilityMax, the sentinel that ends the enum, is not a
// capability and is left out.
func readHeader(path string) (map[string]uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo+1, err)
		}
		header[m[1]] = uint32(value)
	}
	return nil, fmt.Errorf("%s: no SpvCapability enum", path)
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

// writeTable writes the Go source of the capability table and formats it.
func writeTable(path string, table []capEntry) error {
	var b strings.Builder
	b.WriteString("// Copyright 2026 The GoGPU Authors\n")
	b.WriteString("// SPDX-License-Identifier: MIT\n\n")
	b.WriteString("// Code generated by gen.go from the SPIR-V grammar, checked against\n")
	b.WriteString("// spirv.h of the same SPIR-V headers; DO NOT EDIT.\n\n")
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
		return fmt.Errorf("generated source: %w", err)
	}
	return os.WriteFile(path, src, 0o644)
}
