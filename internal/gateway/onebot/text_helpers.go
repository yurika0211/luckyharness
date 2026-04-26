package onebot

import "github.com/yurika0211/luckyharness/internal/utils"

func truncateStr(s string, maxLen int) string {
	return utils.TruncateKeepLength(s, maxLen)
}
