package main

import (
	"github.com/larsks/airdancer/internal/cli"
	"github.com/larsks/airdancer/internal/dancerctl"
	_ "github.com/larsks/airdancer/internal/logsetup"
)

func main() {
	cli.SubCommandMain(
		func() cli.Configurable { return dancerctl.NewConfig() },
		dancerctl.NewHandler(),
	)
}
