package integration

import "fmt"

func Snippet(host, slug, taskID string) string {
	if host == "" {
		host = "agent"
	}
	return fmt.Sprintf("%s: run `specd context %s %s --json`, implement declared files only, then `specd verify %s %s`.", host, slug, taskID, slug, taskID)
}
