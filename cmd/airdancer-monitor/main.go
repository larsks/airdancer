package main

import (
	"github.com/larsks/airdancer/internal/cli"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/monitor"
)

func main() {
	cli.StandardMain(
		func() cli.Configurable { return monitor.NewConfig() },
		monitor.NewMonitorHandler(),
	)
}
