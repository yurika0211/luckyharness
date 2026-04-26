package utils

import (
	"regexp"
	"strings"
)

var (
	htmlTagPattern    = regexp.MustCompile(`<[^>]*>`)
	whitespacePattern = regexp.MustCompile(`\s+`)
)

// StripHTMLTags removes HTML tags with a lightweight regex.
func StripHTMLTags(s string) string {
	return htmlTagPattern.ReplaceAllString(s, "")
}

// NormalizeWhitespace collapses repeated whitespace into one space and trims edges.
func NormalizeWhitespace(s string) string {
	s = whitespacePattern.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
