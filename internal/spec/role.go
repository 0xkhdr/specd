package spec

// RoleDef is one role contract: permission, budget, phase affinity, allowed
// tools, file policy, and prompt class.
type RoleDef struct {
	Name          string
	RW            string
	BudgetTier    string
	PhaseAffinity []Phase
	Tools         []string
	FilePolicy    string
	PromptClass   string
}

// Roles is single registry of declared role contracts, in stable order.
var Roles = []RoleDef{
	{
		Name:          "scout",
		RW:            "readonly",
		BudgetTier:    "minimal",
		PhaseAffinity: []Phase{PhasePerceive, PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect},
		Tools:         []string{"specd_inspect", "specd_read", "specd_query", "specd_status", "specd_context"},
		FilePolicy:    "no writes",
		PromptClass:   "gate-only",
	},
	{
		Name:          "researcher",
		RW:            "readonly",
		BudgetTier:    "large",
		PhaseAffinity: []Phase{PhasePerceive, PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect},
		Tools:         []string{"specd_inspect", "specd_read", "specd_query", "specd_status", "specd_context", "specd_diff", "specd_report", "specd_check"},
		FilePolicy:    "no writes",
		PromptClass:   "gate-only",
	},
	{
		Name:          "reviewer",
		RW:            "readonly",
		BudgetTier:    "medium",
		PhaseAffinity: []Phase{PhasePerceive, PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect},
		Tools:         []string{"specd_inspect", "specd_read", "specd_query", "specd_status", "specd_context", "specd_diff", "specd_report", "specd_check", "specd_waves"},
		FilePolicy:    "no writes",
		PromptClass:   "gate-only",
	},
	{
		Name:          "architect",
		RW:            "readonly",
		BudgetTier:    "medium",
		PhaseAffinity: []Phase{PhaseAnalyze, PhasePlan},
		Tools:         []string{"specd_inspect", "specd_read", "specd_query", "specd_status", "specd_context", "specd_check", "specd_approve", "specd_waves", "specd_report", "specd_diff"},
		FilePolicy:    "no writes",
		PromptClass:   "gate-only",
	},
	{
		Name:          "builder",
		RW:            "readwrite",
		BudgetTier:    "focused",
		PhaseAffinity: []Phase{PhaseExecute},
		Tools:         []string{"specd_inspect", "specd_read", "specd_context", "specd_status", "specd_next", "specd_dispatch", "specd_verify", "specd_task", "specd_report"},
		FilePolicy:    "task scope only",
		PromptClass:   "card",
	},
	{
		Name:          "tester",
		RW:            "readwrite",
		BudgetTier:    "focused",
		PhaseAffinity: []Phase{PhaseExecute, PhaseVerify},
		Tools:         []string{"specd_inspect", "specd_read", "specd_context", "specd_status", "specd_check", "specd_verify", "specd_task", "specd_report"},
		FilePolicy:    "task scope only",
		PromptClass:   "card",
	},
	{
		Name:          "documenter",
		RW:            "readwrite",
		BudgetTier:    "focused",
		PhaseAffinity: []Phase{PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect},
		Tools:         []string{"specd_inspect", "specd_read", "specd_context", "specd_status", "specd_memory", "specd_decision", "specd_report", "specd_diff", "specd_check"},
		FilePolicy:    "unrestricted within files:",
		PromptClass:   "card",
	},
	{
		Name:          "verifier",
		RW:            "readwrite",
		BudgetTier:    "medium",
		PhaseAffinity: []Phase{PhaseExecute, PhaseVerify},
		Tools:         []string{"specd_check", "specd_status", "specd_state_read", "specd_doctor"},
		FilePolicy:    "specd state only",
		PromptClass:   "contract",
	},
	// Legacy alias; keep until later deprecation cycle.
	{
		Name:          "investigator",
		RW:            "readonly",
		BudgetTier:    "minimal",
		PhaseAffinity: []Phase{PhasePerceive, PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect},
		Tools:         []string{"specd_inspect", "specd_read", "specd_query", "specd_status", "specd_context"},
		FilePolicy:    "no writes",
		PromptClass:   "gate-only",
	},
}

var rolesByName = func() map[string]RoleDef {
	m := make(map[string]RoleDef, len(Roles))
	for _, r := range Roles {
		m[r.Name] = r
	}
	return m
}()

// RoleByName looks up a role contract.
func RoleByName(name string) (RoleDef, bool) { r, ok := rolesByName[name]; return r, ok }

// RoleNames returns stable registry order.
func RoleNames() []string {
	out := make([]string, 0, len(Roles))
	for _, r := range Roles {
		out = append(out, r.Name)
	}
	return out
}

// ReadonlyRoleNames returns stable readonly-role names.
func ReadonlyRoleNames() []string {
	out := make([]string, 0, len(Roles))
	for _, r := range Roles {
		if r.RW == "readonly" {
			out = append(out, r.Name)
		}
	}
	return out
}

// IsReadonlyRole reports whether r is a read-only task role.
func IsReadonlyRole(r string) bool {
	def, ok := RoleByName(r)
	return ok && def.RW == "readonly"
}

// RoleTools returns allowed MCP tool names for role.
func RoleTools(name string) []string {
	def, ok := RoleByName(name)
	if !ok {
		return nil
	}
	return append([]string(nil), def.Tools...)
}

// RoleToolSet returns the allowed MCP tool names for role as a set.
func RoleToolSet(name string) map[string]bool {
	tools := RoleTools(name)
	if len(tools) == 0 {
		return nil
	}
	set := make(map[string]bool, len(tools))
	for _, tool := range tools {
		set[tool] = true
	}
	return set
}

// RoleAllowsTool reports whether role may expose tool.
func RoleAllowsTool(name, tool string) bool {
	set := RoleToolSet(name)
	return set != nil && set[tool]
}

// RoleBudgetTier returns budget tier for role.
func RoleBudgetTier(name string) string {
	def, ok := RoleByName(name)
	if !ok {
		return ""
	}
	return def.BudgetTier
}

// RolePromptClass returns prompt class for role.
func RolePromptClass(name string) string {
	def, ok := RoleByName(name)
	if !ok {
		return ""
	}
	return def.PromptClass
}

// RoleFilePolicy returns file policy for role.
func RoleFilePolicy(name string) string {
	def, ok := RoleByName(name)
	if !ok {
		return ""
	}
	return def.FilePolicy
}

// RolePhaseAffinity returns phase affinity for role.
func RolePhaseAffinity(name string) []Phase {
	def, ok := RoleByName(name)
	if !ok {
		return nil
	}
	return append([]Phase(nil), def.PhaseAffinity...)
}

// RoleAllowsPhase reports whether role may operate in phase.
func RoleAllowsPhase(name string, phase Phase) bool {
	def, ok := RoleByName(name)
	if !ok {
		return false
	}
	for _, p := range def.PhaseAffinity {
		if p == phase {
			return true
		}
	}
	return false
}
