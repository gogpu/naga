// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package gen generates the SPIR-V enum set of spirv/internal/spvenums from
// the vendored SPIR-V grammar. It lives in its own package so the drift test
// can run exactly the same code: the production package must not pull
// encoding/json and go/format into its importers just to be testable.
//
// The generator takes an explicit list of operand kinds, so the package
// carries only the enums this repository uses. It fails on anything it cannot
// represent — an unknown category, a value that is not a 32-bit number, two
// enumerants sharing a value without an alias, or a name that is not a Go
// identifier — because a silent skip is the drift the generated package
// exists to prevent.
package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// HeadersRevision names the SPIR-V headers revision the vendored grammar
// comes from. The generated file records it, and the drift test compares the
// whole file including this line.
const HeadersRevision = "vulkan-sdk-1.4.341.0 (04f10f650d514df88b76d25e83db360142c7b174)"

// GlslStd450Revision is the version of the GLSL extended instruction set
// grammar, from its "version" field.
const GlslStd450Revision = 100

// kinds are the operand kinds the generator emits. The order is the order of
// the declarations in the generated file.
var kinds = []string{
	"Capability",
	"Decoration",
	"BuiltIn",
	"ExecutionModel",
	"ExecutionMode",
	"StorageClass",
	"AddressingModel",
	"MemoryModel",
	"FunctionControl",
	"SelectionControl",
	"LoopControl",
	"Dim",
	"ImageFormat",
	"ImageOperands",
	"MemoryAccess",
	"MemorySemantics",
	"Scope",
	"GroupOperation",
	"PackedVectorFormat",
	"RayFlags",
}

// Report counts what Generate read from the grammars.
type Report struct {
	Opcodes    int // instructions in spirv.core.grammar.json
	Enumerants int // canonical enumerants across the generated kinds
	Aliases    int // further names the grammar gives them
	GlslStd450 int // instructions in extinst.glsl.std.450.grammar.json
}

// grammar is the part of spirv.core.grammar.json this generator reads.
type grammar struct {
	Instructions []instruction `json:"instructions"`
	OperandKinds []operandKind `json:"operand_kinds"`
}

// extinstGrammar is the part of extinst.glsl.std.450.grammar.json it reads.
type extinstGrammar struct {
	Version      int           `json:"version"`
	Instructions []instruction `json:"instructions"`
}

type instruction struct {
	Opname  string   `json:"opname"`
	Opcode  uint32   `json:"opcode"`
	Aliases []string `json:"aliases"`
}

type operandKind struct {
	Category   string      `json:"category"`
	Kind       string      `json:"kind"`
	Enumerants []enumerant `json:"enumerants"`
}

type enumerant struct {
	Enumerant string          `json:"enumerant"`
	Value     json.RawMessage `json:"value"`
	Aliases   []string        `json:"aliases"`
}

// constant is one generated name with its value.
type constant struct {
	Name  string
	Value uint32
}

// enumType is one generated Go type.
type enumType struct {
	Type      string            // Go type name
	Constants []constant        // every name, canonical and alias, sorted
	Names     map[uint32]string // canonical name per value, for String
}

// Generate reads the SPIR-V grammar and the GLSL.std.450 extended instruction
// grammar and returns the Go source of enums.go.
func Generate(core, extinst io.Reader) ([]byte, Report, error) {
	var g grammar
	if err := json.NewDecoder(core).Decode(&g); err != nil {
		return nil, Report{}, fmt.Errorf("grammar: %w", err)
	}
	if len(g.Instructions) == 0 {
		return nil, Report{}, errors.New("grammar: no instructions")
	}

	types := make([]enumType, 0, len(kinds)+2)
	ops, err := collectOps(g.Instructions)
	if err != nil {
		return nil, Report{}, err
	}
	types = append(types, ops)

	report := Report{Opcodes: len(g.Instructions)}
	for _, kind := range kinds {
		found := false
		for _, k := range g.OperandKinds {
			if k.Kind != kind {
				continue
			}
			found = true
			et, err := collectKind(k)
			if err != nil {
				return nil, Report{}, err
			}
			report.Enumerants += len(k.Enumerants)
			for _, e := range k.Enumerants {
				report.Aliases += len(e.Aliases)
			}
			types = append(types, et)
			break
		}
		if !found {
			return nil, Report{}, fmt.Errorf("grammar: no %s operand kind", kind)
		}
	}

	var ext extinstGrammar
	if err := json.NewDecoder(extinst).Decode(&ext); err != nil {
		return nil, Report{}, fmt.Errorf("extinst grammar: %w", err)
	}
	if ext.Version != GlslStd450Revision {
		return nil, Report{}, fmt.Errorf("extinst grammar: version %d, want %d", ext.Version, GlslStd450Revision)
	}
	glsl, err := collectExtInst(ext.Instructions)
	if err != nil {
		return nil, Report{}, err
	}
	report.GlslStd450 = len(ext.Instructions)
	types = append(types, glsl)

	src, err := writeSource(types)
	if err != nil {
		return nil, Report{}, err
	}
	return src, report, nil
}

// collectOps turns the instructions of the core grammar into the Op type.
// Opnames already carry the Op prefix, so they are used as they are.
func collectOps(instructions []instruction) (enumType, error) {
	et := enumType{Type: "Op", Names: make(map[uint32]string, len(instructions))}
	seenValue := make(map[uint32]string, len(instructions))
	seenName := make(map[string]bool, len(instructions))
	for _, instr := range instructions {
		if !strings.HasPrefix(instr.Opname, "Op") {
			return enumType{}, fmt.Errorf("grammar: instruction %q does not start with Op", instr.Opname)
		}
		if err := addEntry(&et, seenValue, seenName, instr.Opname, instr.Opcode, ""); err != nil {
			return enumType{}, err
		}
		for _, alias := range instr.Aliases {
			if err := addEntry(&et, seenValue, seenName, alias, instr.Opcode, instr.Opname); err != nil {
				return enumType{}, err
			}
		}
	}
	sortConstants(&et)
	return et, nil
}

// collectKind turns one operand kind into its Go type. Constants are the kind
// name followed by the enumerant or alias name, which is the specification's
// spelling (Dim + 2D, ImageFormat + Rgba32f).
func collectKind(k operandKind) (enumType, error) {
	if k.Category != "ValueEnum" && k.Category != "BitEnum" {
		return enumType{}, fmt.Errorf("grammar: %s has unsupported category %q", k.Kind, k.Category)
	}
	et := enumType{Type: k.Kind, Names: make(map[uint32]string, len(k.Enumerants))}
	seenValue := make(map[uint32]string, len(k.Enumerants))
	seenName := make(map[string]bool, len(k.Enumerants))
	for _, e := range k.Enumerants {
		value, err := parseValue(e)
		if err != nil {
			return enumType{}, fmt.Errorf("grammar: %s.%s: %w", k.Kind, e.Enumerant, err)
		}
		name := kindName(k.Kind, e.Enumerant)
		if err := addEntry(&et, seenValue, seenName, name, value, ""); err != nil {
			return enumType{}, err
		}
		for _, alias := range e.Aliases {
			if err := addEntry(&et, seenValue, seenName, kindName(k.Kind, alias), value, name); err != nil {
				return enumType{}, err
			}
		}
	}
	sortConstants(&et)
	return et, nil
}

// kindName is the constant name of one enumerant: the kind name followed by
// the specification's spelling. An enumerant that already begins with its kind
// (PackedVectorFormat4x8Bit) is not prefixed twice.
func kindName(kind, enumerant string) string {
	if strings.HasPrefix(enumerant, kind) {
		return enumerant
	}
	return kind + enumerant
}

// collectExtInst turns the GLSL.std.450 instructions into their type; the
// opnames have no prefix, unlike the core instructions.
func collectExtInst(instructions []instruction) (enumType, error) {
	et := enumType{Type: "GlslStd450", Names: make(map[uint32]string, len(instructions))}
	seenValue := make(map[uint32]string, len(instructions))
	seenName := make(map[string]bool, len(instructions))
	for _, instr := range instructions {
		name := "GlslStd450" + instr.Opname
		if err := addEntry(&et, seenValue, seenName, name, instr.Opcode, ""); err != nil {
			return enumType{}, err
		}
		for _, alias := range instr.Aliases {
			if err := addEntry(&et, seenValue, seenName, "GlslStd450"+alias, instr.Opcode, name); err != nil {
				return enumType{}, err
			}
		}
	}
	sortConstants(&et)
	return et, nil
}

// addEntry records one constant. A value is canonical once: a second
// enumerant with the same value is refused, because the specification only
// shares values through aliases; the alias paths pass aliasOf.
func addEntry(et *enumType, seenValue map[uint32]string, seenName map[string]bool, name string, value uint32, aliasOf string) error {
	if !identifierPattern.MatchString(name) {
		return fmt.Errorf("%s: %q is not a Go identifier", et.Type, name)
	}
	if seenName[name] {
		return fmt.Errorf("%s: %q appears twice", et.Type, name)
	}
	seenName[name] = true
	if aliasOf != "" {
		canonical, ok := seenValue[value]
		if !ok || canonical != aliasOf {
			return fmt.Errorf("%s: alias %q has no target at value %d", et.Type, name, value)
		}
	} else {
		if prev, ok := seenValue[value]; ok {
			return fmt.Errorf("%s: %q and %q share value %d without an alias", et.Type, prev, name, value)
		}
		seenValue[value] = name
		et.Names[value] = name
	}
	et.Constants = append(et.Constants, constant{Name: name, Value: value})
	return nil
}

// sortConstants orders the names by value, the canonical one first, then the
// aliases alphabetically.
func sortConstants(et *enumType) {
	sort.Slice(et.Constants, func(i, j int) bool {
		a, b := et.Constants[i], et.Constants[j]
		if a.Value != b.Value {
			return a.Value < b.Value
		}
		if a.Name == et.Names[a.Value] {
			return true
		}
		if b.Name == et.Names[b.Value] {
			return false
		}
		return a.Name < b.Name
	})
}

// parseValue reads an enumerant value, which the grammar spells either as a
// JSON number or as a string with a decimal or hexadecimal number.
func parseValue(e enumerant) (uint32, error) {
	raw := strings.TrimSpace(string(e.Value))
	if strings.HasPrefix(raw, `"`) {
		var s string
		if err := json.Unmarshal(e.Value, &s); err != nil {
			return 0, fmt.Errorf("value %s: %w", raw, err)
		}
		v, err := strconv.ParseUint(s, 0, 32)
		if err != nil {
			var numErr *strconv.NumError
			if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
				return 0, fmt.Errorf("value %q does not fit 32 bits", s)
			}
			return 0, fmt.Errorf("value %q: %w", s, err)
		}
		return uint32(v), nil
	}
	var n uint64
	if err := json.Unmarshal(e.Value, &n); err != nil {
		return 0, fmt.Errorf("value %s: %w", raw, err)
	}
	if n > math.MaxUint32 {
		return 0, fmt.Errorf("value %d does not fit 32 bits", n)
	}
	return uint32(n), nil
}

// identifierPattern accepts the names the grammar may generate.
var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// writeSource renders and formats the generated file.
func writeSource(types []enumType) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Copyright 2026 The GoGPU Authors\n")
	b.WriteString("// SPDX-License-Identifier: MIT\n\n")
	fmt.Fprintf(&b, "// Code generated by gen.go from the SPIR-V grammar %s and the GLSL.std.450 grammar (version %d); DO NOT EDIT.\n\n",
		HeadersRevision, GlslStd450Revision)
	b.WriteString("package spvenums\n\n")
	b.WriteString("import \"fmt\"\n\n")
	for i, et := range types {
		if i > 0 {
			b.WriteString("\n")
		}
		writeType(&b, et)
	}
	b.WriteString("\n// enumString names one value from a generated table.\n")
	b.WriteString("func enumString[T ~uint32](names map[T]string, v T, typeName string) string {\n")
	b.WriteString("\tif name, ok := names[v]; ok {\n")
	b.WriteString("\t\treturn name\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn fmt.Sprintf(\"%s(%d)\", typeName, uint32(v))\n")
	b.WriteString("}\n")

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("generated source: %w", err)
	}
	return src, nil
}

// writeType renders one type: the declaration, its constants, the name table
// and the String method.
func writeType(b *strings.Builder, et enumType) {
	fmt.Fprintf(b, "// %s is a SPIR-V %s.\n", et.Type, et.Type)
	fmt.Fprintf(b, "type %s uint32\n\n", et.Type)
	b.WriteString("const (\n")
	for _, c := range et.Constants {
		fmt.Fprintf(b, "\t%s %s = %d\n", c.Name, et.Type, c.Value)
	}
	b.WriteString(")\n\n")

	values := make([]uint32, 0, len(et.Names))
	for value := range et.Names {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	fmt.Fprintf(b, "var %sNames = map[%s]string{\n", lowerFirst(et.Type), et.Type)
	for _, value := range values {
		fmt.Fprintf(b, "\t%d: %q,\n", value, et.Names[value])
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(b, "// String names the value for diagnostics; an unknown value prints as a\n")
	fmt.Fprintf(b, "// number.\n")
	fmt.Fprintf(b, "func (v %s) String() string { return enumString(%sNames, v, %q) }\n", et.Type, lowerFirst(et.Type), et.Type)
}

// lowerFirst lowercases the first letter of a type name for its table.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
