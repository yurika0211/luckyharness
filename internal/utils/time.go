package utils

import (
	"fmt"
	"time"
)

// FormatDurationCompact formats duration in d/h/m/s for status text.
func FormatDurationCompact(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm %ds", mins, int(d.Seconds())%60)
}
