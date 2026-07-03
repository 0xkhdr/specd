package core

import "regexp"

var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// StripHTMLComments blanks out the contents of every HTML comment
// (<!-- ... -->) in text, replacing each non-newline byte with a space so
// line numbers and column offsets in the surrounding text are preserved.
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
