package version

import (
	"fmt"
)

var (
	BuildVersion = "dev"
	BuildRef     = ""
	BuildDate    = ""
)

func ShowVersion() {
	fmt.Printf("Version: %s\n", BuildVersion)
	fmt.Printf("Build ref: %s\n", BuildRef)
	fmt.Printf("Build date: %s\n", BuildDate)
}
