# Rust naga reference outputs (derived)

Reference outputs that upstream does not commit, produced by running
[Rust naga](https://github.com/gfx-rs/wgpu/tree/trunk/naga):

- `out/ir/wgsl-<name>{,.compact}.ron` — IR dumps (RON) of corpus shaders whose
  upstream test parameters do not request IR output. Same format and pipeline
  as upstream's IR snapshots: the module after the WGSL front end (`.ron`) and
  after compaction (`.compact.ron`).
- `out/{spv,msl,hlsl}/wgsl-ptr-deref-test.*` — Rust naga output for our own
  shader `own/in/ptr-deref-test.wgsl`.
- `spv/<name>.spv` — the vendored `../rust/in/spv/<name>.spvasm` assembled
  with `spirv-as`, one binary per assembly file, so a parse test needs no
  external tool. The vendored tree itself stays verbatim.
- `row-major/` — our own row-major matrix fixtures for the SPIR-V frontend:
  GLSL sources, the SPIR-V compiled from them, and Rust naga's MSL, HLSL and
  GLSL output. See `row-major/PROVENANCE.md`.

**Do not edit files in this directory.** They are listed in `MANIFEST.json` and
checked by `TestRustCorpusIntegrity`; regenerate them with Rust naga and rewrite
the manifest with `go run ./internal/cmd/corpussync manifest`.

## License

Derived from the gfx-rs/wgpu test corpus, © The gfx-rs developers, licensed
under Apache-2.0 or MIT at your option — see `../rust/LICENSE.APACHE` and
`../rust/LICENSE.MIT`.

## Provenance

All files are generated at the upstream commit the vendored tree `../rust` is
synced to; `MANIFEST.json` records the commit and the generator. When the
corpus is synced to a new commit, regenerate every file here at that commit
and rewrite the manifest.

The SPIR-V binaries are regenerated from the vendored assembly (spirv-as from
Vulkan SDK 1.4.341.1, SPIRV-Tools v2026.1):

```sh
for f in ../rust/in/spv/*.spvasm; do
  spirv-as --target-env spv1.6 -o "spv/$(basename "$f" .spvasm).spv" "$f"
done
```

`--target-env spv1.6` pins the version word in the SPIR-V header. Without it
spirv-as uses the default of the built tools (SPIR-V 1.3 with SPIRV-Tools
v2026.1, another version in another build), while upstream's harness assembles
without a target env at all. The assembly uses no version-dependent construct,
so the outputs differ only in that header word; naga's frontend reads the
version but only logs it. An explicit target keeps the committed bytes
independent of the local assembler's default.

`TestSPVCorpusBinaries` checks that every `.spvasm` has its `.spv` and that
each starts with the SPIR-V magic; `TestSPVCorpusAssembly` re-assembles every
file with spirv-as and compares the bytes exactly. CI installs the assembler
and requires it through `NAGA_REQUIRE_TOOLS`; locally the byte check skips
when spirv-as is absent, with the reason.
