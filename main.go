package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"github.com/marinator86/portier-cli/cmd"
)

var version = "0.0.1"
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

func main() {
	flag.Parse()
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
