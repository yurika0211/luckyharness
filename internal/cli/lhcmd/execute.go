package lhcmd

import "github.com/yurika0211/luckyharness/internal/logger"

func Execute() error {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		logger.Error("command failed", "error", err)
		return err
	}
	return nil
}
