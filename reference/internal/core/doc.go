// Package core holds specd's domain logic: the layer between the CLI
// commands in internal/cmd and the on-disk spec/state representation. It
// owns state.json load/save with compare-and-swap, the per-spec advisory
// lock, the wave DAG and frontier/critical-path computation, the EARS
// requirements linter, spec-artifact accessors and traceability gates, and
// the embedded scaffold templates used by `specd init`/`new`.
//
// core has no dependency on internal/cmd or internal/cli — commands call
// into core, never the reverse — and it performs no flag parsing or
// process-level argument handling of its own.
package core
