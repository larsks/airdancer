package main

import (
	"github.com/larsks/airdancer/internal/api"
	"github.com/larsks/airdancer/internal/cli"
	_ "github.com/larsks/airdancer/internal/logsetup"
)

func main() {
	cli.StandardMain(
		func() cli.Configurable { return api.NewConfig() },
		api.NewAPIHandler(),
	)
}
