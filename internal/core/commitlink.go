package core

import "strings"

func CommitLink(remote, head string) string {
	if remote == "" || head == "" {
		return head
	}
	remote = strings.TrimSuffix(remote, ".git")
	remote = strings.TrimSuffix(remote, "/")
	if strings.HasPrefix(remote, "git@github.com:") {
		remote = "https://github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	}
	if !strings.HasPrefix(remote, "http://") && !strings.HasPrefix(remote, "https://") {
		return head
	}
	return remote + "/commit/" + head
}
