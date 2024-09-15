package daemon

import (
	"fmt"
	"log"

	"github.com/kardianos/service"
	"github.com/marinator86/portier-cli/internal/portier/application"
)

var logger *log.Logger

type program struct {
	exit chan struct{}

	daemonService service.Service
}

func (p *program) Start(service service.Service) error {
	p.daemonService = service
	go p.run()
	return nil
}

func (p *program) run() {
	application := application.NewPortierApplication()

	//application.LoadConfig(p.configFilePath)

	//application.LoadApiToken(p.apiTokenFilePath)

	// application.StartServices()

	// wait for exit
	<-p.exit

	application.StopServices()
}

func (p *program) Stop(_ service.Service) error {
	close(p.exit)
	return nil
}

func StartDaemon() error {
	err := controlService("start")
	if err != nil {
		return err
	}
	return nil
}

func StopDaemon() error {
	err := controlService("stop")
	if err != nil {
		return err
	}
	return nil
}

func controlService(svcFlag string) error {
	svcConfig := &service.Config{
		Name:        "portier",
		DisplayName: "Portier.dev service",
		Description: "The portier.dev service is your local relay to the portier.dev cloud service.",
		Arguments:   []string{"-c", "/etc/portier/config.yaml", "-t", "/etc/portier/apiToken.yaml"},
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
