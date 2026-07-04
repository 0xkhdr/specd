package embed_templates

import "embed"

//go:embed roles/*.md steering/*.md
var FS embed.FS
