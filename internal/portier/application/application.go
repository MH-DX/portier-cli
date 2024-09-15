package application

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/config"
	"github.com/marinator86/portier-cli/internal/portier/ptls"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"github.com/marinator86/portier-cli/internal/utils"
)

type ServiceContext struct {
	Service  config.Service
	Listener net.Listener
}

type PortierApplication struct {
	config *config.PortierConfig

	deviceCredentials *config.DeviceCredentials

	contexts []ServiceContext

	router router.Router

	uplink uplink.Uplink

	ptls ptls.PTLS
}

func NewPortierApplication() *PortierApplication {

	return &PortierApplication{
		contexts: []ServiceContext{},
	}
}

func (p *PortierApplication) StartServices(portierConfig *config.PortierConfig, creds *config.DeviceCredentials) error {

	log.Println("Creating relay...")
	p.config = portierConfig
	p.deviceCredentials = creds

	p.ptls = ptls.NewPTLS(p.config.TLSEnabled, p.config.PTLSConfig.CertFile, p.config.PTLSConfig.KeyFile, p.config.PTLSConfig.CAFile, p.config.PTLSConfig.KnownHostsFile, nil)

	router, uplink, err := p.createRelay()
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
			if context.Listener != nil {
				p.handleAccept(context, context.Listener)
			}
		}(c)
	}

	go func() {
		for event := range uplink.Events() {
			log.Printf("uplink event received: %v\n", event)
		}
	}()

	err = router.Start()
	if err != nil {
		return err
	}

	log.Println("All Services started...")
	return nil
}

func (p *PortierApplication) handleAccept(context ServiceContext, listener net.Listener) error {
	for {
		conn, err := context.Listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			return err
		}

		log.Printf("Accepted connection from: %s\n", conn.RemoteAddr().String())

		// Now we create a new connection adapter for the outbound connection
		// First, we define the options for the connection adapter

		cID := messages.ConnectionID(uuid.New().String())
		options := adapter.ConnectionAdapterOptions{
			ConnectionId:  cID,
			LocalDeviceId: p.deviceCredentials.DeviceID,
			PeerDeviceId:  context.Service.Options.PeerDeviceID,
			BridgeOptions: messages.BridgeOptions{
				Timestamp: time.Now(),
				URLRemote: *context.Service.Options.URLRemote.URL,
			},
			ConnectionReadTimeout: context.Service.Options.ConnectionReadTimeout,
			ReadBufferSize:        context.Service.Options.ReadBufferSize,
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

		log.Println(utils.PrettyPrint(options))

		// If encryption is enabled globally and for this service, we need to create a TLS client
		var tlsHandshaker func() error = nil
		if p.config.TLSEnabled && context.Service.Options.TLSEnabled {
			tlsConn, handshaker, err := p.ptls.CreateClientAndBridge(conn, context.Service.Options.PeerDeviceID)
			if err != nil {
				log.Printf("Error in TLS handshake: %v", err)
				return err
			}
			conn = tlsConn
			tlsHandshaker = handshaker
		}

		adapter := adapter.NewOutboundConnectionAdapter(options, conn, p.uplink, p.router.EventChannel())
		p.router.AddConnection(cID, adapter)
		adapter.Start()

		// If we have a handshaker, we need to call it now
		if tlsHandshaker != nil {
			err := tlsHandshaker()
			if err != nil {
				log.Printf("Error in TLS handshake: %v", err)
				adapter.Close()
				return err
			}
		}

		log.Printf("Started connection adapter for service: %s\n", context.Service.Name)
	}
}

func (p *PortierApplication) StopServices() error {
	errors := []error{}
	for _, c := range p.contexts {
		err := c.Listener.Close()
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
		// log separator
		log.Printf("--------------------------------------------------\n")
		log.Printf("Starting service: %s\n", service.Name)
		log.Println(utils.PrettyPrint(service))
		switch service.Options.URLLocal.Scheme {
		case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
			listener, err := net.Listen(service.Options.URLLocal.Scheme, service.Options.URLLocal.Host)
			if err != nil {
				return err
			}
			p.contexts = append(p.contexts, ServiceContext{
				Service:  service,
				Listener: listener,
			})
			continue
		case "udp", "udp4", "udp6", "unixgram", "ip", "ip4", "ip6":
			return fmt.Errorf("scheme yet unsupported: %s. Contact contact@portier.dev", service.Options.URLLocal.Scheme)
		default:
			return fmt.Errorf("unrecognized scheme: %s", service.Options.URLLocal.Scheme)
		}
	}
	return nil
}

func (p *PortierApplication) createRelay() (router.Router, uplink.Uplink, error) {
	log.Printf("Creating relay for device: %s\n", p.deviceCredentials.DeviceID)
	log.Printf("Portier URL: %s\n", p.config.PortierURL.String())

	uplinkOptions := uplink.Options{
		APIToken:   p.deviceCredentials.ApiToken,
		PortierURL: p.config.PortierURL.String(),
	}
	uplink := uplink.NewWebsocketUplink(uplinkOptions, nil)
	messageChannel, err := uplink.Connect()
	if err != nil {
		log.Printf("Error connecting to portier server: %v", err)
		return nil, nil, err
	}

	events := make(chan adapter.AdapterEvent, 100)
	router := router.NewRouter(uplink, messageChannel, events, p.ptls)

	return router, uplink, nil
}
