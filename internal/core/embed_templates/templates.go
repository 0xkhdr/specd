package embed_templates

import "embed"

//go:embed roles/*.md steering/*.md reports/*.md AGENTS.md project.yml
var FS embed.FS
