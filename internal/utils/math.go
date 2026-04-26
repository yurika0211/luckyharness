package utils

// MinInt returns the smaller one.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt returns the larger one.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
