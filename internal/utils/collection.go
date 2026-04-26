package utils

import "strings"

// DedupStrings keeps insertion order and removes duplicates.
func DedupStrings(items []string) []string {
	return DedupStringsLimit(items, len(items))
}

// DedupStringsLimit keeps insertion order, removes duplicates, and applies an output limit.
// If limit <= 0, all items are considered.
func DedupStringsLimit(items []string, limit int) []string {
	if limit <= 0 {
		limit = len(items)
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, MinInt(len(items), limit))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// DedupNonEmptyStrings removes empty entries and de-duplicates while preserving order.
func DedupNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

// FilterNonEmptyTrimmed trims each entry and keeps non-empty values.
func FilterNonEmptyTrimmed(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
