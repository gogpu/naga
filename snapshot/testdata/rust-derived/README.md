# Rust naga reference outputs (derived)

Reference outputs that upstream does not commit, produced by running
[Rust naga](https://github.com/gfx-rs/wgpu/tree/trunk/naga):

- `out/ir/wgsl-<name>{,.compact}.ron` — IR dumps (RON) of corpus shaders whose
  upstream test parameters do not request IR output. Same format and pipeline
  as upstream's IR snapshots: the module after the WGSL front end (`.ron`) and
  after compaction (`.compact.ron`).
- `out/{spv,msl,hlsl}/wgsl-ptr-deref-test.*` — Rust naga output for our own
  shader `own/in/ptr-deref-test.wgsl`.

**Do not edit files in this directory.** They are listed in `MANIFEST.json` and
checked by `TestRustCorpusIntegrity`; regenerate them with Rust naga and rewrite
the manifest with `go run ./internal/cmd/corpussync manifest`.

## License

Derived from the gfx-rs/wgpu test corpus, © The gfx-rs developers, licensed
under Apache-2.0 or MIT at your option — see `../rust/LICENSE.APACHE` and
`../rust/LICENSE.MIT`.

## Provenance

These files were generated before provenance was recorded; the manifest notes
this. The IR dumps will be regenerated at the vendored upstream commit when the
corpus is synced (TEST-CORPUS-001), and the generator will be recorded then.
