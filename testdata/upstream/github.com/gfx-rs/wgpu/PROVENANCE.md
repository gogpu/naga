# gfx-rs/wgpu example shaders (vendored)

Verbatim copies of five WGSL shaders from the examples of
[gfx-rs/wgpu](https://github.com/gfx-rs/wgpu). The SPIR-V backend tests in
`spirv/internal/codegen/shader_test.go` compile them as real-world inputs.

**Do not edit files in this directory.** Every vendored file is listed with its
SHA-256 and git blob id in `MANIFEST.json`, and `TestUpstreamTrees`
(`internal/testcorpus`) fails on any modified, missing or unlisted file.

## Source

- Repository: https://github.com/gfx-rs/wgpu
- Commit: `14328f49918cce19f7cc8bf703435968c8293035` (2026-10-14), the commit our
  Rust naga corpus (`snapshot/testdata/rust`) is pinned to.
- Taken: 2026-10-30, read from git objects (`git cat-file blob <commit>:<path>`),
  so the bytes do not depend on a checkout's line-ending settings.

| Here | Upstream |
|---|---|
| `examples/features/src/cube/shader.wgsl` | same path |
| `examples/features/src/hello_triangle/shader.wgsl` | same path |
| `examples/features/src/msaa_line/shader.wgsl` | same path |
| `examples/features/src/render_to_texture/shader.wgsl` | same path |
| `examples/standalone/01_hello_compute/src/shader.wgsl` | same path |
| `LICENSE.APACHE`, `LICENSE.MIT` | repository root |

## License

These files are © The gfx-rs developers, licensed under either of

- Apache License, Version 2.0 ([LICENSE.APACHE](LICENSE.APACHE))
- MIT license ([LICENSE.MIT](LICENSE.MIT))

at your option, as in the upstream repository.

## Refreshing

From a gfx-rs/wgpu clone that has the new commit, for each path in the table:

```sh
git -C <wgpu> cat-file blob <commit>:<path> > testdata/upstream/github.com/gfx-rs/wgpu/<path>
```

then rewrite the manifest and record the commit:

```sh
go run ./internal/cmd/corpussync manifest -dir testdata/upstream/github.com/gfx-rs/wgpu \
  -kind vendored -repo https://github.com/gfx-rs/wgpu -commit <commit> -commit-date <YYYY-MM-DD>
```

Update the commit and date above, and review every resulting test difference.
