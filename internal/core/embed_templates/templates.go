package embed_templates

import "embed"

//go:embed roles/*.md steering/*.md AGENTS.md
var FS embed.FS
