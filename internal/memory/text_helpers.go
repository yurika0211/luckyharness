package memory

import "github.com/yurika0211/luckyharness/internal/utils"

func truncateField(s string, maxLen int) string {
	return utils.Truncate(s, maxLen)
}

func dedupSlice(items []string) []string {
	return utils.DedupNonEmptyStrings(items)
}
