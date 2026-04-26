package utils

// SplitLines splits text by '\n' and trims a trailing '\r' from each line.
func SplitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, trimTrailingCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, trimTrailingCR(s[start:]))
	}
	return lines
}

// SplitLinesBytes splits bytes by '\n' and trims a trailing '\r' from each line.
func SplitLinesBytes(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, trimTrailingCR(string(data[start:i])))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, trimTrailingCR(string(data[start:])))
	}
	return lines
}

func trimTrailingCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}
