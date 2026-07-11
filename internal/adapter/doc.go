// Package adapter holds the interoperability seam for specd: the versioned
// request/result envelopes, result-identity validation, data classification,
// and the opt-in adapter runner. It is deliberately kept out of the trusted
// core (internal/core, internal/core/gates, internal/context, and the DAG/
// report paths): core generates adapter requests as data and consumes adapter
// results only after they have been validated and pinned, but core must never
// import this package. That one-way boundary — enforced by the import guard in
// this package's tests, not by convention — is what keeps the binary free of
// runtime dependencies and free of any model, eval-service, deployment,
// telemetry-backend, protocol, or network code in a gate, DAG, or report path.
//
// The W0 baseline ships only the boundary invariant. Later waves add the
// envelope types, identity checks, classification, runner, and conformance
// suite; those additions live here so the guard keeps core clean as they land.
package adapter
