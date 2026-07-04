package core

func ForbiddenTool(name string) bool {
	switch name {
	case "report", "decision", "memory":
		return true
	default:
		return false
	}
}
