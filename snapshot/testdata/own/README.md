# Own test shaders

WGSL shaders written for this project, not taken from Rust naga. They cover
cases the upstream corpus does not (storage read-modify-write patterns,
pointer dereferences, basic pipeline shapes used as regression tests).

They go through the same pipeline as the Rust corpus — golden snapshots
(`../golden/`), SPIR-V validation, DXIL validation — but do not count toward
Rust parity. Rust naga output exists only for `ptr-deref-test`
(`../rust-derived/`).

Add new shaders here, not under `../rust/`.
