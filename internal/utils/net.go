package utils

import (
	"net/url"
	"strings"
)

// URLEncode encodes query text in RFC3986-friendly form (space => %20).
func URLEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
