package lhcmd

import "github.com/yurika0211/luckyharness/internal/utils"

func truncate(s string, maxLen int) string {
	return utils.Truncate(s, maxLen)
}
