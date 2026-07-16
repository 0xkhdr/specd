package integration

import "fmt"

func Snippet(host, slug, taskID string) string {
	if host == "" {
		host = "agent"
	}
	return fmt.Sprintf("%s: run `specd context %s %s --json`, implement declared files only, `specd verify %s %s` to record evidence, `specd complete-task %s %s` to complete, then `specd check %s`.", host, slug, taskID, slug, taskID, slug, taskID, slug)
}
