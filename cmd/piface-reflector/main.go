package main

import (
	"fmt"
	"log"
	"time"

	"github.com/larsks/airdancer/internal/piface" // Replace with actual module path
)

func main() {
	spiPortName := "/dev/spidev0.0"
	pf, err := piface.NewPiFace(spiPortName)
	if err != nil {
		log.Fatal("Failed to open piface:", err)
	}
	defer pf.Close()

	if err := pf.Init(); err != nil {
		log.Fatal("Failed to initialize piface:", err)
	}

	// Reflect inputs to outputs
	var oldval uint8
	for {
		val, err := pf.ReadInputs()
		if err != nil {
			log.Fatal("failed to read inputs:", err)
		}

		if val != oldval {
			fmt.Printf("INPUTS: ")
			for i := range 8 {
				fmt.Printf("%d ", (val>>(7-i))&0x1)
			}
			fmt.Println("")
			oldval = val
		}

		if err := pf.WriteOutputs(val); err != nil {
			log.Fatal("failed to write outputs:", err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}
