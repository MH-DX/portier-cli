package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/mh-dx/portier-cli/cmd"
	"github.com/mh-dx/portier-cli/internal/utils"
	"gopkg.in/natefinch/lumberjack.v2"
)

var version = "0.0.1"

type globalOptions struct {
	cpuprofile string
	memprofile string
	logfile    string
}

func main() {
	options, args, err := parseGlobalFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if options.logfile == "" {
		home, err := utils.Home()
		if err != nil {
			log.Fatalf("Failed to get portier home directory: %v", err)
		}
		options.logfile = filepath.Join(home, "portier-cli.log")
	}

	lj := &lumberjack.Logger{
		Filename:   options.logfile,
		MaxSize:    1, // megabytes
		MaxBackups: 3,
		MaxAge:     28,    // days
		Compress:   false, // disabled by default
	}
	log.SetOutput(io.MultiWriter(os.Stdout, lj))

	if options.cpuprofile != "" {
		log.Println("Profiling CPU...")
		f, err := os.Create(options.cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	runErr := cmd.ExecuteArgs(version, args)

	if options.memprofile != "" {
		log.Println("Profiling memory...")
		f, err := os.Create(options.memprofile)
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

func parseGlobalFlags(args []string) (*globalOptions, []string, error) {
	options := &globalOptions{}

	flags := flag.NewFlagSet("portier-cli", flag.ContinueOnError)
	flags.StringVar(&options.cpuprofile, "cpuprofile", "", "write cpu profile to file")
	flags.StringVar(&options.memprofile, "memprofile", "", "write memory profile to this file")
	flags.StringVar(&options.logfile, "logfile", "", "path to log file")

	if err := flags.Parse(args); err != nil {
		return nil, nil, err
	}

	return options, flags.Args(), nil
}
