// Package irutil holds the IR helpers and passes that naga's own frontends,
// backends and pipeline-constant processing share and that are not part of
// the public ir package: ir keeps the data model, validation, type
// resolution and the few helpers a frontend or backend outside naga needs
// (ADR-010 B7). Everything here works on the ir types through their public
// API.
package irutil
