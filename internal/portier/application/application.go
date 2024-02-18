package application

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/controller"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"gopkg.in/yaml.v2"
)

type PortierConfig struct {
	PortierURL  url.URL           `yaml:"portierUrl"`
	DeviceID    uuid.UUID         `yaml:"deviceId"`
	RelayConfig relay.RelayConfig `yaml:"relay"`
}

type ServiceContext struct {
	service *relay.Service

	listener net.Listener

	adapter adapter.ConnectionAdapter
}

type PortierApplication struct {
	config *PortierConfig

	apiToken string

	contexts []ServiceContext

	router router.Router

	controller controller.Controller

	uplink uplink.Uplink
}

func NewPortierApplication() *PortierApplication {

	return &PortierApplication{
		contexts: []ServiceContext{},
	}
}

// LoadConfig loads the config from the given file path.
func (p *PortierApplication) LoadConfig(filePath string) error {
	stat, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error getting file info: %v", err)
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v", err)
		return err
	}
	defer file.Close()

	fileContent := make([]byte, stat.Size())
	_, err = file.Read(fileContent)
	if err != nil {
		fmt.Printf("Error reading file: %v", err)
		return err
	}

	config := PortierConfig{}

	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		fmt.Printf("Error unmarshalling yaml: %v", err)
		return err
	}

	p.config = &config

	return nil
}

func (p *PortierApplication) LoadApiToken(filePath string) error {
	stat, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error getting file info: %v", err)
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v", err)
		return err
	}
	defer file.Close()

	fileContent := make([]byte, stat.Size())
	_, err = file.Read(fileContent)
	if err != nil {
		fmt.Printf("Error reading file: %v", err)
		return err
	}

	p.apiToken = string(fileContent)

	return nil
}

func (p *PortierApplication) StartServices() error {

	controller, router, uplink, err := createRelay(p.config.DeviceID, p.config.PortierURL, p.apiToken)
	if err != nil {
		fmt.Printf("Error creating outbound relay: %v", err)
		os.Exit(1)
	}
	p.router = router
	p.controller = controller
	p.uplink = uplink

	err = p.startListeners()
	if err != nil {
		return err
	}

	for _, c := range p.contexts {
		go func(context ServiceContext) {
			for {
				conn, err := context.listener.Accept()
				if err != nil {
					fmt.Printf("Error accepting connection: %v", err)
					continue
				}
				cID := messages.ConnectionID(uuid.New().String())
				options := adapter.ConnectionAdapterOptions{
					ConnectionId:  cID,
					LocalDeviceId: p.config.DeviceID,
					PeerDeviceId:  context.service.Options.PeerDeviceID,
					BridgeOptions: messages.BridgeOptions{
						URLRemote: context.service.Options.URLRemote,
					},
					ResponseInterval:      time.Millisecond * 1000,
					ConnectionReadTimeout: time.Millisecond * 1000,
					ReadBufferSize:        1024,
				}

				context.adapter = adapter.NewOutboundConnectionAdapter(options, conn, p.uplink, p.controller.EventChannel())
				p.controller.AddConnection(cID, context.adapter)
				context.adapter.Start()
			}
		}(c)
	}

	return nil
}

func (p *PortierApplication) StopServices() error {
	errors := []error{}
	for _, c := range p.contexts {
		err := c.adapter.Stop()
		if err != nil {
			fmt.Printf("Error stopping adapter: %v", err)
			errors = append(errors, err)
		}
		err = c.listener.Close()
		if err != nil {
			fmt.Printf("Error closing connection listener: %v", err)
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors while closing listeners: %v", errors)
	}
	return nil
}

// wraps the given PacketConn in a Listener
func (p *PortierApplication) wrapInListener(conn net.PacketConn) net.Listener {
	// TODO: implement
	panic("not implemented yet")
}

func (p *PortierApplication) startListeners() error {
	for _, service := range p.config.RelayConfig.Services {
		switch service.Options.URLLocal.Scheme {
		case "udp", "udp4", "udp6", "unixgram", "ip", "ip4", "ip6":
			conn, err := net.ListenPacket(service.Options.URLLocal.Scheme, service.Options.URLLocal.Host)
			if err != nil {
				return err
			}
			p.contexts = append(p.contexts, ServiceContext{
				service:  &service,
				listener: p.wrapInListener(conn),
			})
			break
		case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
			listener, err := net.Listen(service.Options.URLLocal.Scheme, service.Options.URLLocal.Host)
			if err != nil {
				return err
			}
			p.contexts = append(p.contexts, ServiceContext{
				service:  &service,
				listener: listener,
			})
			break
		default:
			return fmt.Errorf("unsupported scheme: %s", service.Options.URLLocal.Scheme)
		}
	}
	return nil
}

func createRelay(deviceID uuid.UUID, portierUrl url.URL, apiToken string) (controller.Controller, router.Router, uplink.Uplink, error) {
	uplinkOptions := uplink.Options{
		APIToken:   "",
		PortierURL: portierUrl.String(),
	}
	uplink := uplink.NewWebsocketUplink(uplinkOptions, nil)
	messageChannel, err := uplink.Connect()
	if err != nil {
		fmt.Printf("Error connecting to portier server: %v", err)
		return nil, nil, nil, err
	}

	routerEventChannel := make(chan router.ConnectionOpenEvent)
	router := router.NewRouter(uplink, messageChannel, routerEventChannel)
	events := make(chan adapter.AdapterEvent, 100)
	controller := controller.NewController(uplink, events, routerEventChannel, router)

	return controller, router, uplink, nil
}
