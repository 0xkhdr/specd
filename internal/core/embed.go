package core

import (
	"embed"
	"strings"
)

//go:embed embed_templates
var TemplatesFS embed.FS

func ReadTemplate(rel string) (string, error) {
	b, err := TemplatesFS.ReadFile("embed_templates/" + rel)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ApplyVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}
