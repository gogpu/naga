// Package irutil holds the IR helpers and passes that naga's own frontends,
// backends and pipeline-constant processing share and that are not part of
// the public ir package: ir keeps the data model, validation, type
// resolution and the few helpers a frontend or backend outside naga needs
// (ADR-010 B7). Everything here works on the ir types through their public
// API.
//
// Every pass that rewrites a module lives here, as in Rust naga, where
// compact is a module of its own and not part of the IR: compaction
// (Compact, CompactUnused; public as naga.Compact), function inlining for
// DXIL (InlineUserFunctions), type reordering (ReorderTypes), and the one
// walker over the expression handles of an expression and of a statement
// tree that they share with pipeline-constant processing
// (RemapExpressionHandles, RemapStatementExpressionHandles). These passes
// modify the module they are given in place.
package irutil
