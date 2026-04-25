package main

import (
	"os"

	"github.com/yurika0211/luckyharness/internal/cli/lhcmd"
)

func main() {
	if err := lhcmd.Execute(); err != nil {
		os.Exit(1)
	}
}
