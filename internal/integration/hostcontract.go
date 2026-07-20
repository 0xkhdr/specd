package integration

import "github.com/0xkhdr/specd/internal/core"

// The host contract is assurance policy, so it lives in internal/core alongside
// the assurance lattice it resolves against. These aliases keep integration as
// the host-facing surface without duplicating the policy — and without an import
// cycle, since internal/cmd needs the contract and this package's tests import
// internal/cmd.

type (
	HostContract    = core.HostContract
	HostConformance = core.HostConformance
)

func EvaluateHostContract(contract HostContract) HostConformance {
	return core.EvaluateHostContract(contract)
}
func ReferenceHostContract() HostContract { return core.ReferenceHostContract() }
func HumanOnlyOperations() []string       { return core.HumanOnlyOperations() }
