package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

func compileWGSLForInt64AtomicPolicy(t *testing.T, source string) (string, error) {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize WGSL: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse WGSL: %v", err)
	}
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		t.Fatalf("lower WGSL: %v", err)
	}

	options := DefaultOptions()
	options.LangVersion = Version2_4
	options.FakeMissingBindings = true
	code, _, err := Compile(module, options)
	return code, err
}

func TestMSLInt64AtomicPolicy(t *testing.T) {
	t.Run("allows result-discarded storage min max", func(t *testing.T) {
		const source = `
@group(0) @binding(0)
var<storage, read_write> unsigned_value: atomic<u64>;
@group(0) @binding(1)
var<storage, read_write> signed_value: atomic<i64>;

@compute @workgroup_size(1)
fn main() {
    atomicMin(&unsigned_value, 1lu);
    atomicMax(&unsigned_value, 2lu);
    atomicMin(&signed_value, 1li);
    atomicMax(&signed_value, 2li);
}
`

		code, err := compileWGSLForInt64AtomicPolicy(t, source)
		if err != nil {
			t.Fatalf("compile supported 64-bit storage min/max: %v", err)
		}
		for _, want := range []string{"atomic_min_explicit", "atomic_max_explicit"} {
			if !strings.Contains(code, want) {
				t.Errorf("MSL does not contain %q:\n%s", want, code)
			}
		}
		for _, unwanted := range []string{"atomic_fetch_min_explicit", "atomic_fetch_max_explicit"} {
			if strings.Contains(code, unwanted) {
				t.Errorf("MSL contains unsupported result-producing intrinsic %q:\n%s", unwanted, code)
			}
		}
	})

	t.Run("allows nested storage min max", func(t *testing.T) {
		const source = `
struct AtomicPair {
    values: array<atomic<u64>, 2>,
}

@group(0) @binding(0)
var<storage, read_write> pairs: array<AtomicPair, 2>;

@compute @workgroup_size(1)
fn main() {
    atomicMin(&pairs[1].values[0], 1lu);
    atomicMax(&pairs[0].values[1], 2lu);
}
`

		code, err := compileWGSLForInt64AtomicPolicy(t, source)
		if err != nil {
			t.Fatalf("compile nested 64-bit storage min/max: %v", err)
		}
		if !strings.Contains(code, "atomic_min_explicit") || !strings.Contains(code, "atomic_max_explicit") {
			t.Fatalf("nested storage min/max intrinsics missing:\n%s", code)
		}
	})

	rejected := []struct {
		name      string
		operation string
		source    string
	}{
		{
			name:      "load",
			operation: "load",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    let old = atomicLoad(&value);
    _ = old;
}
`,
		},
		{
			name:      "store",
			operation: "store",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicStore(&value, 1lu);
}
`,
		},
		{
			name:      "add",
			operation: "add",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicAdd(&value, 1lu);
}
`,
		},
		{
			name:      "subtract",
			operation: "subtract",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicSub(&value, 1lu);
}
`,
		},
		{
			name:      "and",
			operation: "and",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicAnd(&value, 1lu);
}
`,
		},
		{
			name:      "or",
			operation: "or",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicOr(&value, 1lu);
}
`,
		},
		{
			name:      "xor",
			operation: "xor",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicXor(&value, 1lu);
}
`,
		},
		{
			name:      "exchange",
			operation: "exchange",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicExchange(&value, 1lu);
}
`,
		},
		{
			name:      "compare exchange",
			operation: "compare exchange",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    let result = atomicCompareExchangeWeak(&value, 0lu, 1lu);
    _ = result.old_value;
}
`,
		},
		{
			name:      "result-producing min",
			operation: "min",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    let old = atomicMin(&value, 1lu);
    _ = old;
}
`,
		},
		{
			name:      "result-producing max",
			operation: "max",
			source: `
@group(0) @binding(0) var<storage, read_write> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    let old = atomicMax(&value, 1lu);
    _ = old;
}
`,
		},
		{
			name:      "workgroup min",
			operation: "min",
			source: `
var<workgroup> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicMin(&value, 1lu);
}
`,
		},
		{
			name:      "workgroup max",
			operation: "max",
			source: `
var<workgroup> value: atomic<u64>;
@compute @workgroup_size(1) fn main() {
    atomicMax(&value, 1lu);
}
`,
		},
	}

	for _, test := range rejected {
		t.Run("rejects "+test.name, func(t *testing.T) {
			code, err := compileWGSLForInt64AtomicPolicy(t, test.source)
			if err == nil {
				t.Fatalf("expected 64-bit atomic %s rejection, got MSL:\n%s", test.name, code)
			}
			for _, want := range []string{
				"64-bit atomic " + test.operation,
				"only result-discarded min/max in the storage address space",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			if code != "" {
				t.Errorf("Compile returned source with error:\n%s", code)
			}
		})
	}

	t.Run("preserves 32-bit atomics", func(t *testing.T) {
		const source = `
@group(0) @binding(0) var<storage, read_write> value: atomic<u32>;
@compute @workgroup_size(1) fn main() {
    atomicStore(&value, 1u);
    let loaded = atomicLoad(&value);
    let added = atomicAdd(&value, loaded);
    let exchanged = atomicExchange(&value, added);
    _ = exchanged;
}
`

		code, err := compileWGSLForInt64AtomicPolicy(t, source)
		if err != nil {
			t.Fatalf("compile 32-bit atomic control: %v", err)
		}
		for _, want := range []string{
			"atomic_store_explicit",
			"atomic_load_explicit",
			"atomic_fetch_add_explicit",
			"atomic_exchange_explicit",
		} {
			if !strings.Contains(code, want) {
				t.Errorf("32-bit MSL does not contain %q:\n%s", want, code)
			}
		}
	})
}
