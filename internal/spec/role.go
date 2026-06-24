package spec

// ReadonlyRoles are the task roles that may read context but never mutate the
// working tree. The engine widens or narrows the context budget for them.
var ReadonlyRoles = []string{"investigator", "reviewer"}

var readonlyRoleSet = sliceToSet(ReadonlyRoles)

// IsReadonlyRole reports whether r is a read-only task role.
func IsReadonlyRole(r string) bool { return readonlyRoleSet[r] }

func sliceToSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
