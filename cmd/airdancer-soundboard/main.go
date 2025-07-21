package main

import (
	"github.com/larsks/airdancer/internal/cli"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/soundboard"
)

func main() {
	cli.StandardMain(
		func() cli.Configurable { return soundboard.NewConfig() },
		soundboard.NewSoundboardHandler(),
	)
}
