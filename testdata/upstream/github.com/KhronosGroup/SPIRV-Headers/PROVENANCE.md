# PROVENANCE — SPIRV-Headers

- Repository: https://github.com/KhronosGroup/SPIRV-Headers
- Tag: `vulkan-sdk-1.4.341.0`
- Commit: `04f10f650d514df88b76d25e83db360142c7b174` (committer date 2026-01-21)
- Files: `spirv.h`, `spirv.core.grammar.json` and `extinst.glsl.std.450.grammar.json` from the
  upstream `include/spirv/unified1/` directory; `LICENSE` from the repository root.
- Licence: MIT-style, see `LICENSE` (the Khronos Group).
- How produced: copied from the Vulkan SDK 1.4.341.1 installation
  (`Include/spirv-headers/`) on 2026-11-11; `extinst.glsl.std.450.grammar.json` was copied from the
  same directory on 2026-11-12. The SDK copies carry CRLF line endings and are byte-identical to the
  tag above after CRLF → LF normalisation (SHA-256 checked); this copy keeps the upstream LF line
  endings, so it matches the tag byte for byte — the extinst grammar was already LF in the SDK.
- Used by: `internal/spvnames` — the capability table is generated from these files and
  `TestTableMatchesVendoredHeaders` regenerates it and diffs the result against `table.go`; and
  `spirv/internal/spvenums` — its enum set is generated from `spirv.core.grammar.json` and
  `extinst.glsl.std.450.grammar.json`, and `TestEnumsMatchVendoredGrammar` diffs the result against
  `enums.go`.
- Refresh: take a newer tag, fetch the four files from
  `https://raw.githubusercontent.com/KhronosGroup/SPIRV-Headers/<commit>/…`, rewrite the manifest
  with
  `go run ./internal/cmd/corpussync manifest -dir testdata/upstream/github.com/KhronosGroup/SPIRV-Headers -kind vendored -repo https://github.com/KhronosGroup/SPIRV-Headers -commit <sha> -commit-date <YYYY-MM-DD>`,
  then update `HeadersRevision` in `internal/spvnames/gen` and in `spirv/internal/spvenums/gen`, and
  run `go generate ./internal/spvnames ./spirv/internal/spvenums`.
