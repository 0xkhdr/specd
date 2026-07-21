package embed_templates

import "embed"

//go:embed roles/*.md steering/*.md skills/*/*.md reports/*.md maintenance/*.md policy/*.md AGENTS.md config.yaml
var FS embed.FS
