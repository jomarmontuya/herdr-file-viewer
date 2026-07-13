package viewer

import (
	"net/url"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

const (
	osc8Prefix = "\x1b]8;;"
	osc8ST     = "\x1b\\"
	osc8Close  = "\x1b]8;;\x1b\\"
)

func terminalHyperlinks(s string) string {
	return urlPattern.ReplaceAllStringFunc(s, func(match string) string {
		link, trailing := splitTrailingURLPunctuation(match)
		if !safeURL(link) {
			return match
		}
		return osc8Prefix + link + osc8ST + link + osc8Close + trailing
	})
}

func splitTrailingURLPunctuation(s string) (string, string) {
	end := len(s)
	for end > 0 && strings.ContainsRune(".,;:!?)]}", rune(s[end-1])) {
		end--
	}
	return s[:end], s[end:]
}

func safeURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
