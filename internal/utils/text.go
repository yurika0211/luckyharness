package utils

const ellipsis = "..."

// Truncate truncates by byte length and appends "..." when overflow happens.
// The returned string may be longer than maxLen because of the suffix.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + ellipsis
}

// TruncateKeepLength truncates by byte length and keeps the final length <= maxLen.
func TruncateKeepLength(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= len(ellipsis) {
		return ellipsis[:maxLen]
	}
	return s[:maxLen-len(ellipsis)] + ellipsis
}
