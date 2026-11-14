# Row-major matrix corpus — provenance

Fixtures for reads and stores through a `layout(row_major)` matrix, fed to the
SPIR-V frontend, with Rust naga's output as the reference (card P3-TEST-018).
Upstream has no such rows: neither the vendored WGSL inputs nor the vendored
SPIR-V assembly contains a `RowMajor` decoration.

| Fixture | What it exercises |
|---|---|
| `rm-member` | Reads the whole matrix member, `data.m`, and indexes it in a local copy |
| `rm-column` | Reads one column of the member at a constant index, `data.m[3]` |
| `rm-array-element` | Reads one matrix of an array member, `data.m[1]`, then a column of it |
| `rm-whole-array` | Passes the whole row-major array member to a function as a parameter |
| `rm-store` | Writes the member, `data.m = mat4(1.0)`, through a `std430` buffer |

## Layout

| Path | What it is |
|---|---|
| `in/spv/<name>.glsl` | The sources, written for this corpus |
| `spv/<name>.spv` | SPIR-V 1.3 binaries compiled from them |
| `out/msl/<name>.metal`, `out/hlsl/<name>.hlsl`, `out/glsl/<name>.glsl` | Rust naga's output for the four read fixtures |

`snapshot/row_major_corpus_test.go` reads every fixture, compares the MSL,
HLSL and GLSL our backends write with the Rust outputs, and pins the store
fixture's refusal.

## SPIR-V binaries

Compiled with glslc from Vulkan SDK 1.4.341.1 (shaderc v2026.1, SPIRV-Tools
v2026.1), from the fixture directory:

```sh
for f in in/spv/*.glsl; do
  glslc --target-env=vulkan1.1 -fshader-stage=frag -o "spv/$(basename "$f" .glsl).spv" "$f"
done
```

`--target-env=vulkan1.1` pins the SPIR-V version word. Re-running the command
with this SDK reproduces every committed binary byte for byte (checked
2026-11-14).

## Rust outputs

Rust naga at the commit the vendored tree `../../rust` is synced to (see
`../../MANIFEST.json`): `gfx-rs/wgpu` 14328f49. The driver parses each binary
with `naga::front::spv::parse_u8_slice` and `Options::default()`, compacts with
`compact(KeepUnused::No)`, validates with `ValidationFlags::all()` and
`Capabilities::all()`, then writes:

- MSL: `back::msl::write_string` with `Options::default()` (language 1.0,
  `fake_missing_bindings`, so a resource gets `[[user(fake0)]]`) and
  `PipelineOptions::default()`;
- HLSL: `back::hlsl::Writer::new` with `Options::default()` and
  `PipelineOptions::default()`, then `write(module, info, None)`;
- GLSL: `back::glsl::Writer::new` with `Options::default()` (GLES 3.10, no
  bounds checks: the GLSL options have no policy field) and
  `PipelineOptions` of the module's only entry point.

The test passes the equivalent options explicitly on our side: an empty
per-entry-point MSL map with `FakeMissingBindings` (a nil map would take our
automatic slot numbering, a convenience Rust does not have), the HLSL writer's
fake bindings, and GLES 3.10 with unchecked bounds for GLSL.

## The store fixture

`rm-store` has no Rust output on purpose. Rust's SPIR-V frontend side-loads a
`Transpose` for a row-major member on the load path only (the
`lookup_load_override` map applied in `front/spv/next_block.rs`); its store
path has no counterpart, so it writes the value untransposed — a miscompile,
not a reference. Our frontend refuses the store instead (card P3-SPVIN-007), and
the refusal is this fixture's expected result until the spec-correct store
lands (SPIR-V spec, `OpStore` through a row-major matrix member stores the
matrix in row-major order).

## Matrix stride

Both readers require `MatrixStride` to be the alignment of the matrix's row
count times the scalar width (Rust `front/spv/mod.rs`, our
`spirv/internal/front/types.go`). glslang emits 16 for a `std140` row-major
2x2 matrix, where that rule expects 8, so a `mat2` fixture would be refused by
Rust and by us alike. The fixtures use `mat4`, whose `std140` stride of 16 is
exactly 4 x 4.

## License

The GLSL sources are written for this repository and licensed under MIT OR
Apache-2.0, as the repository is. The Rust outputs are (c) The gfx-rs
developers, licensed under Apache-2.0 or MIT at your option, as the vendored
tree `../../rust` — see `../../rust/LICENSE.APACHE` and
`../../rust/LICENSE.MIT`.

## Refresh

After changing a fixture: recompile the binaries with the command above,
regenerate the read fixtures' outputs with the driver, then rewrite the
manifest from the repository root:

```sh
go run ./internal/cmd/corpussync manifest -dir snapshot/testdata/rust-derived -kind derived
```

Every file under `row-major/`, this one included, is hashed in
`../../MANIFEST.json` and checked by `TestRustCorpusIntegrity`; do not edit
them by hand. A refresh that changes any file rewrites the manifest in the
same change.
