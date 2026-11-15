package front

import "testing"

// FuzzPreprocess checks that arbitrary untrusted source is parsed into tokens
// or diagnostics, never a panic, and that the limits bound the result.
func FuzzPreprocess(f *testing.F) {
	for _, seed := range []string{
		"#version 450 core\nvoid main() {}\n",
		"#version 300 es\nprecision highp float;\nvoid main() {}\n",
		"#define A B\n#define B A\nA\n",
		"#define F(x, y) x * y\nF(1, 2)\n",
		"#if defined(X) && __VERSION__ > 400\n#endif\n",
		"/* unterminated",
		"0x 09 1l 1.5h\n",
		"\\\n",
		"#line 5\n__LINE__\n#error stop\n",
		"#extension GL_EXT_x : require\n#pragma whatever\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		res := preprocess(source, map[string]string{"FOO": "1"})
		if len(res.Tokens) > maxPPTokens {
			t.Fatalf("preprocess produced %d tokens, over the %d cap", len(res.Tokens), maxPPTokens)
		}
	})
}
