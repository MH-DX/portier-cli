package daemon

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kardianos/service"
)

var logger *log.Logger

type program struct {
	exit chan struct{}
}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}
func (p *program) run() {
	// Load config
	logFile, err := os.OpenFile("/var/log/portier.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening file: %v", err)
		return
	}
	defer logFile.Close()

	logger = log.New(logFile, "Portier: ", log.LstdFlags)

	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case tm := <-ticker.C:
			logger.Printf("Still running at %v...", tm)
		case <-p.exit:
			ticker.Stop()
			return
		}
	}
}
func (p *program) Stop(s service.Service) error {
	close(p.exit)
	return nil
}
func StartDaemon(svcFlag string) error {

	svcConfig := &service.Config{
		Name:        "portier",
		DisplayName: "Portier.dev service",
		Description: "The portier.dev service is your local relay to the portier.dev cloud service.",
	}

	prg := &program{}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	if len(svcFlag) != 0 {
		err := service.Control(s, svcFlag)
		if err != nil {
			fmt.Printf("Valid actions: %q\n", service.ControlAction)
			return err
		}
		return nil
	}

	err = s.Run()
	if err != nil {
		return err
	}

	return nil
}
