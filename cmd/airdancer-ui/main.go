package main

import (
	"github.com/larsks/airdancer/internal/cli"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/ui"
)

func main() {
	cli.StandardMain(
		func() cli.Configurable { return ui.NewConfig() },
		ui.NewUIHandler(),
	)
}
