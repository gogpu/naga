package front

// Contracts of the statement shapes Rust naga's GLSL frontend builds
// (front/glsl/lower.rs / parser/functions.rs:41-583): the for-loop clauses, the
// while and do-while guards, the if/else-if chain, the switch with fall-through
// and the break depth inside nested loops.

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// TestLoopShapes checks that the loop forms parse into a module the validator
// accepts and that each keeps its loop statement.
func TestLoopShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "for with declaration",
			body: `    int sum = 0;
    for (int i = 0; i < 4; i = i + 1) {
        if (i == 2) { continue; }
        if (i == 3) { break; }
        sum = sum + i;
    }
    o = vec4(float(sum));
`,
		},
		{
			name: "while with guard",
			body: `    int n = 0;
    while (n < 3) { n = n + 1; }
    o = vec4(float(n));
`,
		},
		{
			name: "while true with break",
			body: `    int n = 0;
    while (true) {
        n = n + 1;
        if (n > 3) { break; }
    }
    o = vec4(float(n));
`,
		},
		{
			name: "do while",
			body: `    int n = 0;
    do { n = n + 1; } while (n < 2);
    o = vec4(float(n));
`,
		},
		{
			name: "nested loops with break",
			body: `    int n = 0;
    for (int i = 0; i < 2; i = i + 1) {
        for (int j = 0; j < 2; j = j + 1) {
            if (j == 1) { break; }
            n = n + 1;
        }
    }
    o = vec4(float(n));
`,
		},
		{
			name: "else if chain",
			body: `    float v = u.x;
    if (v > 1.0) {
        o = vec4(1.0);
    } else if (v > 0.0) {
        o = vec4(0.5);
    } else {
        o = vec4(0.0);
    }
`,
		},
		{
			name: "switch fall through",
			body: `    switch (u.k) {
    case 0:
    case 1:
        o = vec4(1.0);
        break;
    default:
        o = vec4(0.0);
        break;
    }
`,
		},
		{
			name: "discard",
			body: `    if (u.x < 0.0) { discard; }
    o = vec4(1.0);
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `#version 450
layout(set = 0, binding = 0) uniform U { float x; int k; } u;
layout(location = 0) out vec4 o;
void main() {
` + tt.body + `}
`
			mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v", verr)
			}
			main := functionNamed(t, mod, "main")
			hasLoop, hasSwitch := false, false
			for _, stmt := range main.Body {
				switch stmt.Kind.(type) {
				case ir.StmtLoop:
					hasLoop = true
				case ir.StmtSwitch:
					hasSwitch = true
				}
			}
			isLoop := tt.name == "for with declaration" || tt.name == "while with guard" ||
				tt.name == "while true with break" || tt.name == "do while" || tt.name == "nested loops with break"
			if isLoop && !hasLoop {
				t.Fatalf("%s lowered no loop statement", tt.name)
			}
			if tt.name == "switch fall through" && !hasSwitch {
				t.Fatal("the switch lowered no switch statement")
			}
		})
	}
}
