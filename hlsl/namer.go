// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"
	"strings"
)

// namer generates unique identifiers for HLSL output.
// Matches Rust naga's proc::Namer behavior:
// - Per-base-name conflict counters
// - Appends trailing underscore when base ends with digit or is a keyword
// - Case-insensitive keyword detection
type namer struct {
	// unique tracks each sanitized base name → conflict count.
	// Count 0 means first use, 1 means one collision (_1), etc.
	unique map[string]int

	// keywords is the set of reserved HLSL keywords (case-sensitive).
	keywords map[string]struct{}

	// keywordsCaseInsensitive is the set of case-insensitive HLSL keywords (stored lowercase).
	keywordsCaseInsensitive map[string]struct{}

	// reservedPrefixes are prefixes that should be avoided.
	reservedPrefixes []string
}

// newNamer creates a new namer instance.
func newNamer() *namer {
	n := &namer{
		unique:                  make(map[string]int),
		keywords:                make(map[string]struct{}),
		keywordsCaseInsensitive: make(map[string]struct{}),
	}

	// Register HLSL keywords (case-sensitive, matching Rust naga's KeywordSet)
	for kw := range reservedKeywords {
		n.keywords[kw] = struct{}{}
	}
	// Register case-insensitive keywords (stored lowercase, matching Rust naga's CaseInsensitiveKeywordSet)
	for kw := range caseInsensitiveKeywords {
		n.keywordsCaseInsensitive[strings.ToLower(kw)] = struct{}{}
	}

	// Pre-register all naga helper function names to avoid conflicts
	helperNames := []string{
		NagaModfFunction,
		NagaFrexpFunction,
		NagaExtractBitsFunction,
		NagaInsertBitsFunction,
		SamplerHeapVar,
		ComparisonSamplerHeapVar,
		SampleExternalTextureFunction,
		NagaAbsFunction,
		NagaDivFunction,
		NagaModFunction,
		NagaNegFunction,
		NagaF2I32Function,
		NagaF2U32Function,
		NagaF2I64Function,
		NagaF2U64Function,
		ImageLoadExternalFunction,
		ImageSampleBaseClampToEdgeFunc,
		DynamicBufferOffsetsPrefix,
		ImageStorageLoadScalarWrapper,
	}

	for _, name := range helperNames {
		// Register in unique so they'll be avoided
		sanitized := n.sanitize(name)
		n.unique[sanitized] = 0
	}

	return n
}

// sanitize cleans a label into a valid identifier base.
// Matches Rust naga's Namer::sanitize:
// - Drop leading digits
// - Trim trailing underscores
// - Keep only alphanumeric and '_'
// - Collapse consecutive underscores
func (n *namer) sanitize(label string) string {
	if label == "" {
		return "unnamed"
	}

	// Drop leading digits
	start := 0
	for start < len(label) && label[start] >= '0' && label[start] <= '9' {
		start++
	}
	s := label[start:]

	// Trim trailing underscores
	s = strings.TrimRight(s, "_")

	if s == "" {
		return "unnamed"
	}

	// Check if sanitization is needed
	// Rust naga uses is_ascii_alphanumeric — only ASCII letters and digits are valid
	needsSanitize := false
	if strings.Contains(s, "__") {
		needsSanitize = true
	}
	if !needsSanitize {
		for _, c := range s {
			if !isASCIIAlphanumeric(c) && c != '_' {
				needsSanitize = true
				break
			}
		}
	}

	if !needsSanitize {
		// Check for reserved prefixes
		for _, prefix := range n.reservedPrefixes {
			if strings.HasPrefix(s, prefix) {
				return "gen_" + s
			}
		}
		return s
	}

	// Filter characters
	var buf strings.Builder
	for _, c := range s {
		switch c {
		case ':', '<', '>', ',':
			// Common C++ type chars become separator
			if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "_") {
				buf.WriteByte('_')
			}
		default:
			if isASCIIAlphanumeric(c) || c == '_' {
				if c == '_' && strings.HasSuffix(buf.String(), "_") {
					continue // collapse consecutive underscores
				}
				buf.WriteRune(c)
			} else {
				if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "_") {
					buf.WriteByte('_')
				}
				fmt.Fprintf(&buf, "u%04x_", c)
			}
		}
	}

	result := strings.TrimRight(buf.String(), "_")
	if result == "" {
		return "unnamed"
	}

	// Check for reserved prefixes
	for _, prefix := range n.reservedPrefixes {
		if strings.HasPrefix(result, prefix) {
			return "gen_" + result
		}
	}

	return result
}

// call generates a unique name based on the given label.
// Matches Rust naga's Namer::call:
// - Sanitizes the label
// - If first occurrence AND (ends with digit or is keyword), appends '_'
// - If collision, appends '_{count}'
func (n *namer) call(label string) string {
	base := n.sanitize(label)

	if count, exists := n.unique[base]; exists {
		// Collision: increment count and append suffix
		count++
		n.unique[base] = count
		return fmt.Sprintf("%s_%d", base, count)
	}

	// First occurrence
	suffixed := base
	if endsWithDigit(base) || n.isKeyword(base) {
		suffixed = base + "_"
	}
	n.unique[base] = 0
	return suffixed
}

// callOr generates a unique name from the given base, or uses the fallback if base is empty/nil.
func (n *namer) callOr(base, fallback string) string {
	if base == "" {
		return n.call(fallback)
	}
	return n.call(base)
}

// isKeyword checks if a name is a reserved keyword.
// Matches Rust naga: case-sensitive check against keywords, case-insensitive check against keywordsCaseInsensitive.
func (n *namer) isKeyword(name string) bool {
	if _, found := n.keywords[name]; found {
		return true
	}
	_, found := n.keywordsCaseInsensitive[strings.ToLower(name)]
	return found
}

// endsWithDigit checks if a string ends with an ASCII digit.
func endsWithDigit(s string) bool {
	if s == "" {
		return false
	}
	c := s[len(s)-1]
	return c >= '0' && c <= '9'
}

// reserve marks a name as used without returning it.
func (n *namer) reserve(name string) {
	base := n.sanitize(name)
	if _, exists := n.unique[base]; !exists {
		n.unique[base] = 0
	}
}

// namespace temporarily enters a fresh naming scope for the duration of body.
// Used for struct members which only need to be unique among themselves.
// Matches Rust naga's Namer::namespace.
func (n *namer) namespace(body func()) {
	outer := n.unique
	n.unique = make(map[string]int)
	body()
	n.unique = outer
}

// reset clears all tracked names and resets state.
func (n *namer) reset() {
	n.unique = make(map[string]int)
}

// count returns the number of unique base names tracked.
func (n *namer) count() int {
	return len(n.unique)
}

// isUsed checks if a name has already been used.
func (n *namer) isUsed(name string) bool {
	base := n.sanitize(name)
	_, exists := n.unique[base]
	return exists
}

// callWithPrefix generates a unique name with a specific prefix.
func (n *namer) callWithPrefix(prefix, base string) string {
	return n.call(prefix + base)
}

// isASCIIAlphanumeric checks if a rune is an ASCII letter or digit.
// Matches Rust's char::is_ascii_alphanumeric().
func isASCIIAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// NeedsTrailingUnderscore reports whether a variable name requires a
// trailing underscore suffix in HLSL output. The HLSL namer (matching
// Rust naga's proc::Namer) appends "_" when the sanitized name ends
// with an ASCII digit or collides with an HLSL reserved keyword. The
// DXIL backend uses this to produce metadata resource names that match
// what DXC would generate from the HLSL roundtrip.
func NeedsTrailingUnderscore(name string) bool {
	if name == "" {
		return false
	}
	if endsWithDigit(name) {
		return true
	}
	if _, found := reservedKeywords[name]; found {
		return true
	}
	_, found := caseInsensitiveKeywords[strings.ToLower(name)]
	return found
}
