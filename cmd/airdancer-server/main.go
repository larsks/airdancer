package main

import (
	"fmt"
	"github.com/larsks/airdancer/internal/piface"
)

func main() {
	pf, err := piface.NewPiFace(0)
	if err != nil {
		panic(err)
	}

	fmt.Printf("piface: %+v\n", pf)
}
