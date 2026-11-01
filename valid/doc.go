// Package valid validates an IR module: Rust naga's valid module. Validate
// checks a module against the capabilities a WebGPU implementation allows
// and returns, per function and entry point, the globals it uses (ModuleInfo).
//
// The layering is Rust naga's (lib.rs): package ir is the data, the typifier
// types its expressions, and valid builds on both. The backends type the
// module they write but never validate it; validate first, then compile.
package valid
