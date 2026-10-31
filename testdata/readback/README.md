# GPU readback corpus

Small compute shaders and the exact values a GPU must write when it runs them.

Static validators (spirv-val, IDxcValidator) check structure, not values: naga v0.20.3 fixed three DXIL loop
miscompiles that every validator accepted and that wrote wrong results on Intel and NVIDIA GPUs. Only running
the shader and reading the buffers back catches that class of bug.

- **naga** owns the corpus and keeps it well formed: `TestReadbackCorpusCompile` (in the root package) loads
  every case, checks its JSON against the shader and compiles it on every naga backend, with spirv-val,
  glslangValidator, dxc and IDxcValidator where they are installed. It runs no GPU.
- **gogpu/wgpu** runs the corpus on every backend and adapter and compares the buffers it reads back.

The reference reader of this format is `internal/readback`; a reader in another repository implements this
README.

## Layout

```
testdata/readback/
  README.md
  <area>/<name>.wgsl    the shader, self-contained and compilable as it is
  <area>/<name>.json    the case
```

- `<area>` and `<name>` are lowercase words joined by `-` (`loops/for-counter`). The case name is
  `<area>/<name>`.
- Every `.wgsl` has its `.json` and the other way round; any other file is an error.
- A value that a compiler could fold is tested twice, as two plain shaders: `<name>-lit.wgsl` with the value as
  a literal and `<name>-rt.wgsl` reading it from a storage buffer (the WebGPU CTS `preventValueOptimizations`
  pattern). The v0.20.3 bugs appeared only with run-time values.

## Case format 1

```json
{
  "format": 1,
  "desc": "for-loop counter declared in the loop header writes 1..4; the sentinel after them stays",
  "entry": "main",
  "workgroups": [1, 1, 1],
  "constants": {"k": 3, "7": 1.5},
  "requires": [],
  "bindings": [
    {"binding": 0, "kind": "storage-read", "words": [3, "0xFFFFFFFF"], "type": "i32"},
    {"binding": 1, "kind": "storage-rw", "len": 5, "fill": "0xDEADBEEF",
     "expect": [[1, 2, 3, 4, "fill"]]}
  ],
  "knownFailures": [
    {"backend": "dx12-dxil", "card": "BUG-LOOP-VAR-REINIT-001", "reason": "loop-local var is not reset"}
  ]
}
```

Unknown fields are errors. Every field below is required unless marked optional.

| Field | Meaning |
|---|---|
| `format` | `1`. A reader rejects any other value: an older reader fails with "corpus format N > supported M" instead of misreading a newer case. |
| `desc` | What the case checks and why it exists (the bug it guards, with its card if any). |
| `entry` | Name of the `@compute` entry point. |
| `workgroups` | Dispatch size: exactly three counts, each ≥ 1. |
| `constants` | Optional. Pipeline-constant values, keyed as WebGPU keys them: the override's `@id` as a decimal string, or its name if it has no `@id`. Values are JSON numbers (`bool` overrides: 0 or 1). |
| `requires` | Optional. WebGPU features the case needs; an adapter without one skips the case. Format 1 knows `shader-f16`; a case requires it exactly when its shader uses f16 (a type, literal or conversion). |
| `bindings` | The buffers, all in `@group(0)`, one per `@binding` the entry point uses — no more, no fewer. |
| `knownFailures` | Optional. Expected failures, below. |

### Bindings

| Field | Meaning |
|---|---|
| `binding` | The `@binding` number; unique in the case. |
| `kind` | `storage-read` (`var<storage, read>`), `storage-rw` (`var<storage, read_write>`) or `uniform` (`var<uniform>`). It must match the shader's declaration. Format 1 has buffers only: no textures or samplers. |
| `words` | Initial contents, one 32-bit word each. Or: |
| `len`, `fill` | `len` words, each the value `fill` (optional, default 0). Exactly one of `words` and `len` is given; `fill` only with `len`. |
| `expect` | Optional, `storage-rw` only. A list of acceptable contents after the dispatch; the binding passes if it equals **any** of them (Rust wgpu `ShaderTest` any-of outputs). Each entry has exactly as many words as the buffer. `"fill"` stands for the fill value: that word must not have been written. A binding without `expect` is not checked; a case needs at least one binding with `expect`. |
| `type` | Optional: `u32` (default), `i32` or `f32`. It only changes how a failure prints the words; the comparison is always bit for bit. |

The buffer size is the word count × 4 bytes and must be at least the size of the shader's type for that binding
(a runtime-sized array counts one element, WebGPU's minimum binding size).

**Words** are JSON integers in [0, 4294967295] or strings `"0x"` followed by one to eight hex digits. Negative
`i32` values and `f32` bit patterns are written in hex (`-1` is `"0xFFFFFFFF"`, `1.5` is `"0x3FC00000"`); f32
cases use exactly representable values, since format 1 has no tolerance.

**Sentinels.** An output buffer starts filled with a sentinel (`"fill": "0xDEADBEEF"`, Rust wgpu uses
`u32::MAX`) and has room past the last word the shader writes; `"fill"` in `expect` checks that nothing was
written there. "Not written" and "wrote 0" are different results.

**Execution order.** Loops, `break`, `continue`, `continuing` and `switch` cases record the order of events as
the WebGPU CTS flow-control harness does: a single invocation appends event ids to
`struct Out { count: u32, data: array<u32> }`, and `expect` lists `[count, id, id, …]` followed by the
untouched sentinels.

## Known failures

A known failure says a case is expected to fail on some runs, as Rust wgpu's `FailureCase`:

| Field | Meaning |
|---|---|
| `backend` | `vulkan`, `dx12-fxc` (HLSL through FXC), `dx12-dxil` (naga's DXIL), `gles`, `metal` or `software`. |
| `adapter`, `driver` | Optional. Case-insensitive substrings of the adapter name and the driver description; empty matches every adapter. |
| `card` | The card that tracks the bug: a naga card ID (`BUG-DXIL-043`) or `gogpu/<repo>:<ID>` for another gogpu repository (`gogpu/wgpu:P1-INFRA-001`). Required: no anonymous failures. |
| `reason` | One line: what goes wrong. |

The runner matches every run against the entries of its case:

- matched and failed → expected failure, reported as such;
- matched and **passed** → **error**: "unexpected pass, close `<card>`". A fixed bug cannot hide behind a stale
  entry;
- not matched and failed → error.

On the naga side, a backend may fail to compile a case only when that backend has an entry with no `adapter` and
no `driver` (it fails everywhere). A case is not compiled at all for a backend that never exposes a feature it
`requires`, since no run reaches it there: `gles` never offers `shader-f16`. `spirv` serves both `vulkan` and `software`, so a SPIR-V compile failure needs
an entry for each.

### Lockstep rule for another repository's cards

The corpus wgpu runs is the naga version wgpu's `go.mod` pins (or the local naga tree under go.work). An entry
whose card is `gogpu/wgpu:<ID>` therefore changes in two steps, in this order:

1. naga removes the entry in a naga release (naga CI only compiles the corpus, so it stays green);
2. the wgpu PR that fixes the bug bumps naga to that release in the same PR.

Any other order leaves a window in which wgpu is red: an "unexpected pass" while wgpu still pins a naga with the
entry, or the old failure while the entry is gone and the fix is not.

A naga card is closed the other way round: the naga fix and the removal of its entry land in the same naga PR,
and the readback run on that branch (through go.work) shows the case passing.

## How wgpu finds the corpus

`NAGA_READBACK_DIR` if set; otherwise `go list -m -f {{.Dir}} github.com/gogpu/naga` + `/testdata/readback`
(under go.work the local naga tree, otherwise the version wgpu pins; `testdata/` ships in the module zip). An
empty `Dir` falls back to `go mod download -json github.com/gogpu/naga@<pinned>`. A naga version without this
directory makes the harness skip with a message, never fail.

## Adding a case

1. Write the shader. Keep it minimal, `@workgroup_size` as small as the case allows, one entry point.
2. Work out the expected words from the WGSL specification, not from a run of naga or of a GPU, and say in
   `desc` (or a comment in the shader) how they follow.
3. Run `go test -run TestReadbackCorpusCompile .` from the module root.
4. A case for an open bug carries a `knownFailures` entry with the bug's card. The fix's PR removes the entry, and
   the readback run shows the case passing.
