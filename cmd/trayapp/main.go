package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mh-dx/portier-cli/cmd"
	"github.com/mh-dx/portier-cli/internal/utils"
)

var version = "0.0.1"

func main() {
	// Set up file logging for errors and panics in .portier home dir
	home, err := utils.Home()
	if err != nil {
		log.Printf("Failed to get .portier home directory: %v", err)
		home = os.TempDir()
	}
	logFilePath := filepath.Join(home, "portier-tray.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	} else {
		log.Printf("Failed to open log file: %v", err)
	}

	// Log working directory and environment
	if wd, err := os.Getwd(); err == nil {
		log.Printf("Working directory: %s", wd)
	} else {
		log.Printf("Failed to get working directory: %v", err)
	}
	log.Printf("Started at: %s", time.Now().Format(time.RFC3339))

	// Catch panics and log them
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v", r)
			os.Exit(2)
		}
	}()

	// Always run in tray mode for this binary
	os.Args = []string{"portier-tray", "tray"}

	// Execute the command
	runErr := cmd.Execute(version)
	if runErr != nil {
		log.Printf("Error running tray application: %v", runErr)
		os.Exit(1)
	}
}
