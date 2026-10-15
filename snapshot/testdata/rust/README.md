# Rust naga test corpus (vendored)

Verbatim copies of the WGSL test inputs and reference outputs of
[Rust naga](https://github.com/gfx-rs/wgpu/tree/trunk/naga), the shader
translator of gfx-rs/wgpu. Our backends are compared against these outputs:
Rust naga is the ground truth for this project.

**Do not edit files in this directory.** Every file is listed with its SHA-256
and git blob id in `MANIFEST.json`, and `TestRustCorpusIntegrity` fails on any
modified, missing or unlisted file.

## License

These files are © The gfx-rs developers, licensed under either of

- Apache License, Version 2.0 ([LICENSE.APACHE](LICENSE.APACHE))
- MIT license ([LICENSE.MIT](LICENSE.MIT))

at your option, as in the upstream repository.

## Layout

| Here | Upstream (`gfx-rs/wgpu`) |
|---|---|
| `in/wgsl/<name>.wgsl`, `<name>.toml` | `naga/tests/in/wgsl/` (shader + test parameters) |
| `out/spv/wgsl-<name>.spvasm` | `naga/tests/out/spv/` (rspirv disassembly) |
| `out/msl/wgsl-<name>.metal` | `naga/tests/out/msl/` |
| `out/hlsl/wgsl-<name>.hlsl` | `naga/tests/out/hlsl/` |
| `out/glsl/wgsl-<name>.<entry>.<Stage>.glsl` | `naga/tests/out/glsl/` |
| `out/ir/wgsl-<name>{,.compact}.ron` | `naga/tests/out/ir/` |
| `LICENSE.APACHE`, `LICENSE.MIT` | repository root |

Upstream also writes `out/spv/*.spvasm.glsl` (SPIRV-Cross output of its SPIR-V,
a review aid); a sync vendors them as well. Outputs of non-WGSL inputs are not
vendored.

## Provenance

`MANIFEST.json` records the upstream commit. The current files predate the
sync tool: they were imported from several upstream commits between 2026-02
and 2026-04, and the manifest records no single commit. Each file's git blob id
is listed, and every one of them exists in the upstream history, so the origin
of every file can be traced with `git log --find-object=<gitBlob>` in a
gfx-rs/wgpu clone. The license files are from 14328f49.

## Updating

From a clean gfx-rs/wgpu checkout at the commit to vendor:

```sh
go run ./internal/cmd/corpussync sync -wgpu <path-to-wgpu> -out snapshot/testdata/rust
```

The tool reads the files from git objects (byte-exact, whatever the checkout's
line-ending settings), replaces this tree, writes `MANIFEST.json` and prints
what was added, removed and changed. Commit the result as one change
(`chore(testdata): sync Rust corpus to <commit>`), and review every resulting
test difference.
