package security

import "strings"

type scannerHit struct {
	Scanner string
	Pattern string
}

var scannerPatterns = []scannerHit{
	{Scanner: "secrets", Pattern: "api_key="},
	{Scanner: "secrets", Pattern: "secret_key="},
	{Scanner: "secrets", Pattern: "password="},
	{Scanner: "injection", Pattern: "curl | sh"},
	{Scanner: "injection", Pattern: "curl|sh"},
	{Scanner: "injection", Pattern: "eval $("},
	{Scanner: "slopsquat", Pattern: "github.com/golang/glog"},
	{Scanner: "slopsquat", Pattern: "github.com/sirupsen/logrusx"},
}

func scanText(text string) []scannerHit {
	lower := strings.ToLower(text)
	var hits []scannerHit
	for _, pattern := range scannerPatterns {
		if strings.Contains(lower, strings.ToLower(pattern.Pattern)) {
			hits = append(hits, pattern)
		}
	}
	return hits
}
