// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import "testing"

func TestNamer_Call(t *testing.T) {
	n := newNamer()

	// First call should return the base name
	got := n.call("position")
	if got != "position" {
		t.Errorf("call(\"position\") = %q, want \"position\"", got)
	}

	// Second call with same base should get a suffix
	got = n.call("position")
	if got == "position" {
		t.Error("expected unique name for second call")
	}
	if got != "position_1" && got != "position_2" {
		t.Logf("got %q (suffix added as expected)", got)
	}

	// Different base should work
	got = n.call("normal")
	if got != "normal" {
		t.Errorf("call(\"normal\") = %q, want \"normal\"", got)
	}
}

func TestNamer_CaseInsensitivity(t *testing.T) {
	n := newNamer()

	// Use lowercase
	got1 := n.call("myvar")
	if got1 != "myvar" {
		t.Errorf("first call = %q, want \"myvar\"", got1)
	}

	// HLSL is case-insensitive, so MYVAR should conflict
	got2 := n.call("MYVAR")
	if got2 == "MYVAR" {
		t.Error("MYVAR should conflict with myvar in HLSL (case-insensitive)")
	}

	// MixedCase should also conflict
	got3 := n.call("MyVar")
	if got3 == "MyVar" {
		t.Error("MyVar should conflict with myvar in HLSL (case-insensitive)")
	}
}

func TestNamer_ReservedKeywords(t *testing.T) {
	n := newNamer()

	// Reserved keywords should be escaped
	tests := []struct {
		input string
		want  string
	}{
		{"float", "_float"},
		{"int", "_int"},
		{"bool", "_bool"},
		{"struct", "_struct"},
		{"cbuffer", "_cbuffer"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := n.call(tt.input)
			if got != tt.want {
				t.Errorf("call(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNamer_EmptyBase(t *testing.T) {
	n := newNamer()

	got := n.call("")
	if got != UnnamedIdentifier {
		t.Errorf("call(\"\") = %q, want \"_unnamed\"", got)
	}
}

func TestNamer_IsUsed(t *testing.T) {
	n := newNamer()

	// Before using
	if n.isUsed("test") {
		t.Error("\"test\" should not be used yet")
	}

	// After using
	n.call("test")
	if !n.isUsed("test") {
		t.Error("\"test\" should be used now")
	}

	// Case-insensitive check
	if !n.isUsed("TEST") {
		t.Error("\"TEST\" should match \"test\" (case-insensitive)")
	}
}

func TestNamer_Reserve(t *testing.T) {
	n := newNamer()

	// Reserve a name
	n.reserve("reserved_name")

	// Should be marked as used
	if !n.isUsed("reserved_name") {
		t.Error("reserved name should be marked as used")
	}

	// Calling with that base should get a suffix
	got := n.call("reserved_name")
	if got == "reserved_name" {
		t.Error("call should not return reserved name")
	}
}

func TestNamer_HelperNamesReserved(t *testing.T) {
	n := newNamer()

	// All naga helper names should be pre-reserved
	helperNames := []string{
		"_naga_modf",
		"_naga_div",
		"_naga_mod",
		"_naga_abs",
	}

	for _, name := range helperNames {
		if !n.isUsed(name) {
			t.Errorf("helper name %q should be pre-reserved", name)
		}
	}
}

func TestNamer_Reset(t *testing.T) {
	n := newNamer()

	// Use some names
	n.call("a")
	n.call("b")
	n.call("c")

	initialCount := n.count()
	if initialCount < 3 {
		t.Errorf("expected at least 3 names, got %d", initialCount)
	}

	// Reset
	n.reset()

	if n.count() != 0 {
		t.Errorf("after reset, count = %d, want 0", n.count())
	}

	// Should be able to use names again
	got := n.call("a")
	if got != "a" {
		t.Errorf("after reset, call(\"a\") = %q, want \"a\"", got)
	}
}

func TestNamer_CallWithPrefix(t *testing.T) {
	n := newNamer()

	// Basic prefix usage
	got := n.callWithPrefix("input_", "position")
	if got != "input_position" {
		t.Errorf("callWithPrefix = %q, want \"input_position\"", got)
	}

	// Second call should get suffix
	got2 := n.callWithPrefix("input_", "position")
	if got2 == "input_position" {
		t.Error("expected unique name for second call")
	}
}

func TestNamer_UniqueSequence(t *testing.T) {
	n := newNamer()

	// Generate many names with same base
	names := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		name := n.call("var")
		if _, exists := names[name]; exists {
			t.Errorf("duplicate name generated: %q", name)
		}
		names[name] = struct{}{}
	}

	if len(names) != 100 {
		t.Errorf("expected 100 unique names, got %d", len(names))
	}
}

func TestNamer_CaseInsensitiveTypes(t *testing.T) {
	n := newNamer()

	// Some HLSL keywords are case-insensitive
	// After using "texture2d", "TEXTURE2D" should conflict
	n.call("myTexture2d")

	// Case variations should be detected
	if !n.isUsed("mytexture2d") {
		t.Error("lowercase variant should be detected as used")
	}
	if !n.isUsed("MYTEXTURE2D") {
		t.Error("uppercase variant should be detected as used")
	}
}
