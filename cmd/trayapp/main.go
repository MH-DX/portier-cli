package main

import (
	"log"
	"os"

	"github.com/mh-dx/portier-cli/cmd"
)

var version = "0.0.1"

func main() {
	// Always run in tray mode for this binary
	os.Args = []string{"portier-tray", "tray"}
	
	// Execute the command
	runErr := cmd.Execute(version)
	if runErr != nil {
		log.Printf("Error running tray application: %v", runErr)
		os.Exit(1)
	}
}