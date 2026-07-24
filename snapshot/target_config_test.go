package snapshot_test

import "testing"

func TestRustTargetClassification(t *testing.T) {
	tests := []struct {
		name    string
		content string
		target  string
		want    rustTargetClassification
	}{
		{
			name:    "enabled among pipe separated targets",
			content: `targets = "SPIRV | METAL | HLSL | WGSL"`,
			target:  "METAL",
			want:    rustTargetEnabled,
		},
		{
			name:    "explicitly excluded",
			content: `targets = "SPIRV | HLSL | WGSL"`,
			target:  "METAL",
			want:    rustTargetExcluded,
		},
		{
			name:    "whitespace around declaration",
			content: "  targets = \"METAL | WGSL\"\n",
			target:  "METAL",
			want:    rustTargetEnabled,
		},
		{
			name:    "no targets declaration",
			content: "[msl]\nlang_version = [2, 4]\n",
			target:  "METAL",
			want:    rustTargetUndeclared,
		},
		{
			name:    "comment is not a declaration",
			content: `# targets = "METAL"`,
			target:  "METAL",
			want:    rustTargetUndeclared,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyRustTarget(test.content, test.target); got != test.want {
				t.Fatalf("classifyRustTarget() = %d, want %d", got, test.want)
			}
		})
	}

	t.Run("missing config stays undeclared", func(t *testing.T) {
		got, err := rustTargetForShader("definitely-not-a-real-shader", "METAL")
		if err != nil {
			t.Fatalf("rustTargetForShader() error: %v", err)
		}
		if got != rustTargetUndeclared {
			t.Fatalf("rustTargetForShader() = %d, want undeclared", got)
		}
	})
}
