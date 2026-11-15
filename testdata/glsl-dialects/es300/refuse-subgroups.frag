#version 300 es

// design §15: no subgroups in ES 3.00 (they arrive with ES 3.10), and ES
// 3.00 has no extension that could enable them.
precision highp float;

out vec4 color;

void main() {
    color = subgroupBallotBitCount(subgroupBallot(true)) > 0u ? vec4(1.0) : vec4(0.0);
}
