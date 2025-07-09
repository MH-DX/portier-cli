package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	api "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/utils"
	"gopkg.in/yaml.v2"
)

type PortierConfig struct {
	PortierURL                  utils.YAMLURL         `yaml:"portierUrl"`
	TLSEnabled                  bool                  `yaml:"tlsEnabled"`
	PTLSConfig                  PTLSConfig            `yaml:"tlsConfig"`
	Services                    []Service             `yaml:"services"`
	DefaultResponseInterval     time.Duration         `yaml:"defaultResponseInterval"`
	DefaultReadTimeout          time.Duration         `yaml:"defaultReadTimeout"`
	DefaultThroughputLimit      int                   `yaml:"defaultThroughputLimit"`
	DefaultReadBufferSize       int                   `yaml:"defaultReadBufferSize"`
	DefaultDatagramConnectionID messages.ConnectionID `yaml:"defaultDatagramConnectionId"`
}

type DeviceCredentials struct {
	// DeviceID is filled at runtime using the whoami endpoint and is not persisted
	DeviceID uuid.UUID `yaml:"-"`
	ApiToken string    `yaml:"APIKey"`
}

// ServiceOptions are options for the service that need to be known beforehand.
type ServiceOptions struct {
	// The local URL
	URLLocal utils.YAMLURL `yaml:"urlLocal" validate:"required"`

	// The remote URL the bridge has to connect to
	URLRemote utils.YAMLURL `yaml:"urlRemote" validate:"required"`

	// The remote device id
	PeerDeviceID uuid.UUID `yaml:"peerDeviceID" validate:"required,uuid"`

	// IsSecure indicates whether the connection is secured with TLS
	TLSEnabled bool `yaml:"tlsEnabled"`

	// The connection adapter's read timeout
	ConnectionReadTimeout time.Duration `yaml:"connectionReadTimeout"`

	// The TCP read buffer size
	ReadBufferSize int `yaml:"readBufferSize"`
}

// Service is a service that is exposed by the portier server as a TCP or UDP service. Each Service
// has an internal queue that is used to queue messages that are sent to the service. The queue has a max size,
// in case the queue is full, messages are not received from the underlying connection anymore (backpressure).
//
// A service decides itself when to ack messages, i.e. when to send an ack message to the portier server, and
// it implements rate-limiting, i.e. it limits the number of bytes that are sent per second to the portier server.
// A service also implements encryption, i.e. it encrypts the data that is sent to the portier server after exchanging the public keys.
type Service struct {
	// The service name
	Name string `yaml:"name"`

	// ServiceOptions defines the options for the service
	Options ServiceOptions `yaml:"options"`
}

type PTLSConfig struct {
	// cert file path, containing this device's certificate
	// default: "{home}/cert.pem"
	CertFile string `yaml:"certFile"`

	// key file path, containing this device's private key
	// default: "{home}/key.pem"
	KeyFile string `yaml:"keyFile"`

	// CA cert file path, to verify the peer's certificate
	// If set, server and client will verify the peer's certificate.
	// If not set, server and client will try the KnownHosts file to verify the peer's certificate.
	// default: not set
	CAFile string `yaml:"caFile"`

	// KnownHosts is a local file containing a map of known certificate fingerprints to deviceID, for
	// verifying the peer's certificate. Only used if CAFile is not set.
	// default: {home}/known_hosts
	KnownHostsFile string `yaml:"knownHostsFile"`
}

func defaultPTLSConfig(home string) *PTLSConfig {
	result := PTLSConfig{
		CertFile:       fmt.Sprintf("%s/cert.pem", home),
		KeyFile:        fmt.Sprintf("%s/key.pem", home),
		CAFile:         fmt.Sprintf("%s/cacert.pem", home),
		KnownHostsFile: fmt.Sprintf("%s/known_hosts", home),
	}

	return &result
}

// LoadConfig loads the config from the given file path.
func LoadConfig(filePath string) (*PortierConfig, error) {
	config, err := DefaultPortierConfig()
	if err != nil {
		log.Printf("Error getting default config: %v", err)
		return nil, err
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Error getting file info: %v. Using default config only.", err)
		return config, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return nil, err
	}
	defer file.Close()

	fileContent := make([]byte, stat.Size())
	_, err = file.Read(fileContent)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return nil, err
	}

	err = yaml.Unmarshal(fileContent, config)
	if err != nil {
		log.Printf("Error unmarshalling yaml: %v", err)
		return nil, err
	}

	return config, nil
}

func LoadApiToken(filePath string) (*DeviceCredentials, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Error getting file info: %v. Exiting", err)
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v. Exiting", err)
		return nil, err
	}
	defer file.Close()

	fileContent := make([]byte, stat.Size())
	_, err = file.Read(fileContent)
	if err != nil {
		log.Printf("Error reading file: %v. Exiting", err)
		return nil, err
	}

	type fileCreds struct {
		ApiToken string `yaml:"APIKey"`
	}

	fc := fileCreds{}
	err = yaml.Unmarshal(fileContent, &fc)
	if err != nil {
		log.Printf("Error unmarshalling yaml: %v. Exiting", err)
		return nil, err
	}

	guid, err := api.WhoAmI("https://api.portier.dev", fc.ApiToken)
	if err != nil {
		return nil, fmt.Errorf("could not get device ID: %w", err)
	}

	credentials := DeviceCredentials{
		DeviceID: guid,
		ApiToken: fc.ApiToken,
	}

	return &credentials, nil
}

// SaveConfig saves the config to the given file path.
func SaveConfig(filePath string, config *PortierConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func DefaultPortierConfig() (*PortierConfig, error) {
	home, err := utils.Home()
	if err != nil {
		return nil, err
	}

	return &PortierConfig{
		PortierURL: utils.YAMLURL{
			URL: &url.URL{
				Scheme: "wss",
				Host:   "api.portier.dev",
				Path:   "/spider",
			},
		},
		Services:                    []Service{},
		TLSEnabled:                  false,
		PTLSConfig:                  *defaultPTLSConfig(home),
		DefaultResponseInterval:     1 * time.Second,
		DefaultReadTimeout:          1 * time.Second,
		DefaultThroughputLimit:      0,
		DefaultReadBufferSize:       4096,
		DefaultDatagramConnectionID: messages.ConnectionID("00000000-1111-0000-0000-000000000000"),
	}, nil
}
