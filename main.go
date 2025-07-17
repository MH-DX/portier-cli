package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/mh-dx/portier-cli/cmd"
	"github.com/mh-dx/portier-cli/internal/utils"
	"gopkg.in/natefinch/lumberjack.v2"
)

var version = "0.0.1"
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")
var logfile = flag.String("logfile", "", "path to log file")

func main() {
	flag.Parse()

	home, err := utils.Home()
	if err != nil {
		log.Fatalf("Failed to get portier home directory: %v", err)
	}

	if *logfile == "" {
		*logfile = filepath.Join(home, "portier-cli.log")
	}

	log.SetOutput(&lumberjack.Logger{
		Filename:   *logfile,
		MaxSize:    1, // megabytes
		MaxBackups: 3,
		MaxAge:     28,    // days
		Compress:   false, // disabled by default
	})

	if *cpuprofile != "" {
		log.Println("Profiling CPU...")
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	runErr := cmd.Execute(version)

	if *memprofile != "" {
		log.Println("Profiling memory...")
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}

	if runErr != nil {
		os.Exit(1)
	}
}
