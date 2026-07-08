package core

import (
	"fmt"
	"strings"
)

func RenderMetrics(model ReportModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "specd_tasks_total{spec=%q} %d\n", model.Slug, model.Total)
	fmt.Fprintf(&b, "specd_tasks_complete{spec=%q} %d\n", model.Slug, model.Complete)
	fmt.Fprintf(&b, "specd_tasks_running{spec=%q} %d\n", model.Slug, model.Running)
	fmt.Fprintf(&b, "specd_tasks_blocked{spec=%q} %d\n", model.Slug, model.Blocked)
	fmt.Fprintf(&b, "specd_tasks_pending{spec=%q} %d\n", model.Slug, model.Pending)
	return b.String()
}
