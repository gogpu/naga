package ir

import "testing"

// TestNamePresence pins the tri-state the accessor encodes: an absent name,
// a present but empty one, and a present non-empty one.
func TestNamePresence(t *testing.T) {
	tests := []struct {
		name    string
		present bool
		want    string
		hasName bool
	}{
		{"", false, "", false},
		{"", true, "", true},
		{"x", false, "x", true},
		{"x", true, "x", true},
	}
	for _, tt := range tests {
		got, hasName := NamePresence(tt.name, tt.present)
		if got != tt.want || hasName != tt.hasName {
			t.Errorf("NamePresence(%q, %v) = (%q, %v), want (%q, %v)",
				tt.name, tt.present, got, hasName, tt.want, tt.hasName)
		}
	}
}
