package core

import "regexp"

var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

func StripHTMLComments(text string) string {
	return htmlCommentRe.ReplaceAllStringFunc(text, func(m string) string {
		result := make([]byte, len(m))
		for i, c := range []byte(m) {
			if c == '\n' {
				result[i] = '\n'
			} else {
				result[i] = ' '
			}
		}
		return string(result)
	})
}
