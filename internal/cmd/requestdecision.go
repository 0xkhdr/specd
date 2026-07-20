package cmd

// runRequestDecision records an agent's request for a human decision. It is the
// agent-legal half of the `decision` pair: the agent states the deviation, a
// human answers it with `specd decision`. Recording a request resolves nothing —
// it appends one record and touches neither phase nor evidence (R1.1, R1.2).
func runRequestDecision(root string, args []string, flags map[string]string) error {
	return appendScoped(root, args, flags, "decision-request", "usage: specd request-decision <spec> --text <text> [--scope <scope>]")
}
