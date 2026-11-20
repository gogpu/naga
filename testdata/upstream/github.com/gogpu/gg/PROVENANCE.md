# gogpu/gg production shaders (vendored)

Verbatim copies of the 25 WGSL shaders that [gogpu/gg](https://github.com/gogpu/gg)
compiles with naga in production: the GPU accelerator and compositor shaders
(`internal/gpu/shaders`) and the Vello-derived compute pipeline
(`internal/gpu/tilecompute/shaders`). The snapshot tests compile them as
real-world inputs: every entry point must give DXIL that DXC's validator
accepts (`TestDxilValGGProduction`), and every backend must give the same
bytes on every run (`TestDeterministicOutput`).

**Do not edit files in this directory.** Every vendored file is listed with its
SHA-256 and git blob id in `MANIFEST.json`, and `TestUpstreamTrees`
(`internal/testcorpus`) fails on any modified, missing or unlisted file.

## Source

- Repository: https://github.com/gogpu/gg
- Commit: `3b2cd102efd8439c4e171934e1a638a40f051e11` (2026-11-02) — the copy was
  first taken at `bd563f4c293fd1f70afe56276c444a38302bac15` (2026-09-01, the head
  of `main` then) and refreshed here for `blend.wgsl`, whose `blend_soft_light`
  got a `vecN<bool>` `select()` condition in `3b2cd10` (the old source passed
  `step()`, invalid WGSL, and failed spirv-val; card P2-TEST-010).
- Taken: 2026-11-01, read from git objects (`git cat-file blob <commit>:<path>`),
  so the bytes do not depend on a checkout's line-ending settings; refreshed
  2026-11-20 to the commit above.

| Here | Upstream |
|---|---|
| `internal/gpu/shaders/*.wgsl` (16 files) | same paths |
| `internal/gpu/tilecompute/shaders/*.wgsl` (9 files) | same paths |
| `LICENSE` | repository root |

## License

These files are © Andrey Kolkov and GoGPU Contributors, MIT license
([LICENSE](LICENSE)), as in the upstream repository.

## Refreshing

From a gogpu/gg clone that has the new commit, for each `.wgsl` file under the
two directories in the table (add new ones, delete removed ones):

```sh
git -C <gg> cat-file blob <commit>:<path> > testdata/upstream/github.com/gogpu/gg/<path>
```

then rewrite the manifest and record the commit:

```sh
go run ./internal/cmd/corpussync manifest -dir testdata/upstream/github.com/gogpu/gg \
  -kind vendored -repo https://github.com/gogpu/gg -commit <commit> -commit-date <YYYY-MM-DD>
```

Update the commit and date above, and review every resulting test difference.
