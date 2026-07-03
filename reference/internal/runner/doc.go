// Package runner executes `specd verify` task commands under a pluggable
// isolation backend. The Runner interface separates policy (env scrubbing,
// shell selection, timeout — owned by the cmd/verify caller) from
// mechanism (how the process is actually isolated and observed). The
// default shRunner reproduces specd's historical, non-sandboxed
// `shell -c command` execution exactly; other backends (e.g. bwrap,
// container) implement the same interface for stronger isolation.
package runner
