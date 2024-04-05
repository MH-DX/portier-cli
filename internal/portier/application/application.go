package application

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"github.com/marinator86/portier-cli/internal/utils"
	"gopkg.in/yaml.v3"
)

type PortierConfig struct {
	PortierURL                  utils.YAMLURL         `yaml:"portierUrl"`
	Services                    []relay.Service       `yaml:"services"`
	DefaultLocalPublicKey       string                `yaml:"defaultLocalPublicKey"`
	DefaultLocalPrivateKey      string                `yaml:"defaultLocalPrivateKey"`
	DefaultCipher               string                `yaml:"defaultCipher"`
	DefaultCurve                string                `yaml:"defaultCurve"`
	DefaultResponseInterval     time.Duration         `yaml:"defaultResponseInterval"`
	DefaultReadTimeout          time.Duration         `yaml:"defaultReadTimeout"`
	DefaultThroughputLimit      int                   `yaml:"defaultThroughputLimit"`
	DefaultReadBufferSize       int                   `yaml:"defaultReadBufferSize"`
	DefaultDatagramConnectionID messages.ConnectionID `yaml:"defaultDatagramConnectionId"`
}

type DeviceCredentials struct {
	DeviceID uuid.UUID `yaml:"deviceID"`
	ApiToken string    `yaml:"APIKey"`
}

type ServiceContext struct {
	service relay.Service

	packetConn net.PacketConn

	listener net.Listener
}

type PortierApplication struct {
	config *PortierConfig

	deviceCredentials *DeviceCredentials

	contexts []ServiceContext

	router router.Router

	uplink uplink.Uplink
}

func NewPortierApplication() *PortierApplication {

	return &PortierApplication{
		contexts: []ServiceContext{},
	}
}

func defaultPortierConfig() *PortierConfig {
	return &PortierConfig{
		PortierURL: utils.YAMLURL{
			URL: &url.URL{
				Scheme: "wss",
				Host:   "api.portier.dev/spider",
			},
		},
		Services:                    []relay.Service{},
		DefaultLocalPublicKey:       "not-implemented-yet",
		DefaultLocalPrivateKey:      "not-implemented-yet",
		DefaultCipher:               "aes-256-gcm",
		DefaultCurve:                "curve25519",
		DefaultResponseInterval:     1 * time.Second,
		DefaultReadTimeout:          1 * time.Second,
		DefaultThroughputLimit:      0,
		DefaultReadBufferSize:       4096,
		DefaultDatagramConnectionID: messages.ConnectionID("00000000-1111-0000-0000-000000000000"),
	}
}

// LoadConfig loads the config from the given file path.
func (p *PortierApplication) LoadConfig(filePath string) error {
	stat, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Error getting file info: %v", err)
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return err
	}
	defer file.Close()

	fileContent := make([]byte, stat.Size())
	_, err = file.Read(fileContent)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return err
	}

	config := defaultPortierConfig()

	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		log.Printf("Error unmarshalling yaml: %v", err)
		return err
	}

	p.config = config

	return nil
}

func (p *PortierApplication) LoadApiToken(filePath string) error {
	stat, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Error getting file info: %v", err)
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return err
	}
	defer file.Close()

	fileContent := make([]byte, stat.Size())
	_, err = file.Read(fileContent)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return err
	}

	credentials := DeviceCredentials{}

	err = yaml.Unmarshal(fileContent, &credentials)
	if err != nil {
		log.Printf("Error unmarshalling yaml: %v", err)
		return err
	}

	p.deviceCredentials = &credentials

	return nil
}

func (p *PortierApplication) StartServices() error {

	log.Println("Creating relay...")

	router, uplink, err := createRelay(p.deviceCredentials.DeviceID, *p.config.PortierURL.URL, p.deviceCredentials.ApiToken)
	if err != nil {
		log.Printf("Error creating outbound relay: %v", err)
		os.Exit(1)
	}
	p.router = router
	p.uplink = uplink

	log.Println("Starting services...")

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

	log.Println("All Services started...")

	err = router.Start()
	if err != nil {
		return err
	}

	return nil
}

func (p *PortierApplication) handleAccept(context ServiceContext, listener net.Listener) error {
	for {
		conn, err := context.listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			return err
		}

		log.Printf("Accepted connection from: %s\n", conn.RemoteAddr().String())

		// Now we create a new connection adapter for the outbound connection
		// First, we define the options for the connection adapter

		cID := messages.ConnectionID(uuid.New().String())
		options := adapter.ConnectionAdapterOptions{
			ConnectionId:        cID,
			LocalDeviceId:       p.deviceCredentials.DeviceID,
			PeerDeviceId:        context.service.Options.PeerDeviceID,
			PeerDevicePublicKey: context.service.Options.PeerDevicePublicKey,
			BridgeOptions: messages.BridgeOptions{
				Timestamp: time.Now(),
				URLRemote: *context.service.Options.URLRemote.URL,
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

		// print the options to the console in pretty format
		log.Printf("Connection adapter options: %v\n", options)

		adapter := adapter.NewOutboundConnectionAdapter(options, conn, p.uplink, p.router.EventChannel())
		p.router.AddConnection(cID, adapter)
		adapter.Start()

		log.Printf("Started connection adapter for service: %s\n", context.service.Name)
	}
}

func (p *PortierApplication) handlePacket(context ServiceContext, packetConn net.PacketConn) {
	for {
		buffer := make([]byte, 4096)
		n, addr, err := packetConn.ReadFrom(buffer)
		if err != nil {
			log.Printf("Error reading from packet connection: %v", err)
			continue
		}

		datagram := messages.DatagramMessage{
			Source: addr.String(),
			Target: context.service.Options.URLRemote.String(),
			Data:   buffer[:n],
		}

		datagramEncoded, err := encoder.NewEncoderDecoder().EncodeDatagramMessage(datagram) // TODO avoid creating a new encoder/decoder for each message
		if err != nil {
			log.Printf("Error encoding datagram message: %v", err)
			continue
		}

		// send the packet to the portier server via the uplink
		p.uplink.Send(messages.Message{
			Header: messages.MessageHeader{
				From: p.deviceCredentials.DeviceID,
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
		err := c.listener.Close()
		if err != nil {
			log.Printf("Error closing connection listener: %v", err)
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors while closing listeners: %v", errors)
	}
	return nil
}

func (p *PortierApplication) startListeners() error {
	for _, service := range p.config.Services {
		log.Printf("Starting service: %s\n", service.Name)
		switch service.Options.URLLocal.Scheme {
		case "udp", "udp4", "udp6", "unixgram", "ip", "ip4", "ip6":
			log.Printf("Starting gram listener on %s\n", service.Options.URLLocal.Host)
			conn, err := net.ListenPacket(service.Options.URLLocal.Scheme, service.Options.URLLocal.Host)
			if err != nil {
				return err
			}
			p.contexts = append(p.contexts, ServiceContext{
				service:    service,
				packetConn: conn,
			})
			continue
		case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
			log.Printf("Starting listener for service: %v\n", service)
			listener, err := net.Listen(service.Options.URLLocal.Scheme, service.Options.URLLocal.Host)
			if err != nil {
				return err
			}
			p.contexts = append(p.contexts, ServiceContext{
				service:  service,
				listener: listener,
			})
			continue
		default:
			return fmt.Errorf("unsupported scheme: %s", service.Options.URLLocal.Scheme)
		}
	}
	return nil
}

func createRelay(deviceID uuid.UUID, portierUrl url.URL, apiToken string) (router.Router, uplink.Uplink, error) {
	uplinkOptions := uplink.Options{
		APIToken:   apiToken,
		PortierURL: portierUrl.String(),
	}
	uplink := uplink.NewWebsocketUplink(uplinkOptions, nil)
	messageChannel, err := uplink.Connect()
	if err != nil {
		log.Printf("Error connecting to portier server: %v", err)
		return nil, nil, err
	}

	events := make(chan adapter.AdapterEvent, 100)
	router := router.NewRouter(uplink, messageChannel, events)

	return router, uplink, nil
}
