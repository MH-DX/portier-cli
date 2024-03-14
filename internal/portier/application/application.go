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
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"gopkg.in/yaml.v2"
)

type PortierConfig struct {
	PortierURL                  url.URL               `yaml:"portierUrl"`
	DeviceID                    uuid.UUID             `yaml:"deviceId"`
	RelayConfig                 relay.RelayConfig     `yaml:"relay"`
	DefaultLocalPublicKey       string                `yaml:"defaultLocalPublicKey" default:"not-implemented-yet"`
	DefaultLocalPrivateKey      string                `yaml:"defaultLocalPrivateKey" default:"not-implemented-yet"`
	DefaultCipher               string                `yaml:"defaultCipher" default:"aes-256-gcm"`
	DefaultCurve                string                `yaml:"defaultCurve" default:"curve25519"`
	DefaultResponseInterval     time.Duration         `yaml:"defaultResponseInterval" default:"1s"`
	DefaultReadTimeout          time.Duration         `yaml:"defaultReadTimeout" default:"1s"`
	DefaultThroughputLimit      int                   `yaml:"defaultThroughputLimit" default:"0"`
	DefaultReadBufferSize       int                   `yaml:"defaultReadBufferSize" default:"4096"`
	DefaultDatagramConnectionID messages.ConnectionID `yaml:"defaultDatagramConnectionId" default:"00000000-1111-0000-0000-000000000000"`
}

type ServiceContext struct {
	service *relay.Service

	packetConn net.PacketConn

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
			if context.listener != nil {
				p.handleAccept(context, context.listener)
			} else {
				p.handlePacket(context, context.packetConn)
			}
		}(c)
	}

	return nil
}

func (p *PortierApplication) handleAccept(context ServiceContext, listener net.Listener) adapter.ConnectionAdapter {
	for {
		conn, err := context.listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v", err)
			continue
		}

		// Now we create a new connection adapter for the outbound connection
		// First, we define the options for the connection adapter

		cID := messages.ConnectionID(uuid.New().String())
		options := adapter.ConnectionAdapterOptions{
			ConnectionId:        cID,
			LocalDeviceId:       p.config.DeviceID,
			PeerDeviceId:        context.service.Options.PeerDeviceID,
			PeerDevicePublicKey: context.service.Options.PeerDevicePublicKey,
			BridgeOptions: messages.BridgeOptions{
				Timestamp: time.Now(),
				URLRemote: context.service.Options.URLRemote,
				Cipher:    context.service.Options.Cipher,
				Curve:     context.service.Options.Curve,
			},
			LocalPublicKey:        context.service.Options.LocalPublicKey,
			LocalPrivateKey:       context.service.Options.LocalPrivateKey,
			ResponseInterval:      context.service.Options.ResponseInterval,
			ConnectionReadTimeout: context.service.Options.ConnectionReadTimeout,
			ThroughputLimit:       context.service.Options.ThroughputLimit,
			ReadBufferSize:        context.service.Options.ReadBufferSize,
		}

		if options.LocalPublicKey == "" {
			options.LocalPublicKey = p.config.DefaultLocalPublicKey
		}
		if options.LocalPrivateKey == "" {
			options.LocalPrivateKey = p.config.DefaultLocalPrivateKey
		}
		if options.BridgeOptions.Cipher == "" {
			options.BridgeOptions.Cipher = p.config.DefaultCipher
		}
		if options.BridgeOptions.Curve == "" {
			options.BridgeOptions.Curve = p.config.DefaultCurve
		}
		if options.ResponseInterval == 0 {
			options.ResponseInterval = p.config.DefaultResponseInterval
		}
		if options.ConnectionReadTimeout == 0 {
			options.ConnectionReadTimeout = p.config.DefaultReadTimeout
		}
		if options.ThroughputLimit == 0 {
			options.ThroughputLimit = p.config.DefaultThroughputLimit
		}
		if options.ReadBufferSize == 0 {
			options.ReadBufferSize = p.config.DefaultReadBufferSize
		}

		context.adapter = adapter.NewOutboundConnectionAdapter(options, conn, p.uplink, p.controller.EventChannel())
		p.controller.AddConnection(cID, context.adapter)
		context.adapter.Start()
	}
}

func (p *PortierApplication) handlePacket(context ServiceContext, packetConn net.PacketConn) {
	for {
		buffer := make([]byte, 4096)
		n, addr, err := packetConn.ReadFrom(buffer)
		if err != nil {
			fmt.Printf("Error reading from packet connection: %v", err)
			continue
		}

		datagram := messages.DatagramMessage{
			Source: addr.String(),
			Target: context.service.Options.URLRemote.String(),
			Data:   buffer[:n],
		}

		datagramEncoded, err := encoder.NewEncoderDecoder().EncodeDatagramMessage(datagram) // TODO avoid creating a new encoder/decoder for each message
		if err != nil {
			fmt.Printf("Error encoding datagram message: %v", err)
			continue
		}

		// send the packet to the portier server via the uplink
		p.uplink.Send(messages.Message{
			Header: messages.MessageHeader{
				From: p.config.DeviceID,
				To:   context.service.Options.PeerDeviceID,
				Type: messages.DG,
				CID:  p.config.DefaultDatagramConnectionID,
			},
			Message: datagramEncoded, // TODO encrypt the message
		})
	}
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

func (p *PortierApplication) startListeners() error {
	for _, service := range p.config.RelayConfig.Services {
		switch service.Options.URLLocal.Scheme {
		case "udp", "udp4", "udp6", "unixgram", "ip", "ip4", "ip6":
			conn, err := net.ListenPacket(service.Options.URLLocal.Scheme, service.Options.URLLocal.Host)
			if err != nil {
				return err
			}
			p.contexts = append(p.contexts, ServiceContext{
				service:    &service,
				packetConn: conn,
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
		APIToken:   apiToken,
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
