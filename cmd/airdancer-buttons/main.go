package main

import (
	"github.com/larsks/airdancer/internal/buttonwatcher"
	"github.com/larsks/airdancer/internal/cli"
)

func main() {
	cli.StandardMain(
		func() cli.Configurable { return buttonwatcher.NewConfig() },
		buttonwatcher.NewButtonHandler(),
	)
}
