package embed_templates

import "embed"

//go:embed roles/*.md steering/*.md reports/*.md AGENTS.md
var FS embed.FS
