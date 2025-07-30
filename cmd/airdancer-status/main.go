package main

import (
	"github.com/larsks/airdancer/internal/cli"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/status"
)

func main() {
	cli.StandardMain(
		func() cli.Configurable { return status.NewConfig() },
		status.NewHandler(nil),
	)
}
