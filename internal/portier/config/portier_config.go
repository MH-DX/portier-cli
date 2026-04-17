package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	api "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/utils"
	"gopkg.in/yaml.v2"
)

type PortierConfig struct {
	BaseURL                     utils.YAMLURL         `yaml:"-"`
	RelayPath                   string                `yaml:"-"`
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

type portierConfigYAML struct {
	BaseURL                     string                `yaml:"baseUrl,omitempty"`
	LegacyPortierURL            string                `yaml:"portierUrl,omitempty"`
	TLSEnabled                  bool                  `yaml:"tlsEnabled"`
	PTLSConfig                  PTLSConfig            `yaml:"tlsConfig"`
	Services                    []Service             `yaml:"services"`
	DefaultResponseInterval     time.Duration         `yaml:"defaultResponseInterval"`
	DefaultReadTimeout          time.Duration         `yaml:"defaultReadTimeout"`
	DefaultThroughputLimit      int                   `yaml:"defaultThroughputLimit"`
	DefaultReadBufferSize       int                   `yaml:"defaultReadBufferSize"`
	DefaultDatagramConnectionID messages.ConnectionID `yaml:"defaultDatagramConnectionId"`
}

func defaultPTLSConfig(home string) *PTLSConfig {
	result := PTLSConfig{
		CertFile:       filepath.Join(home, "cert.pem"),
		KeyFile:        filepath.Join(home, "key.pem"),
		CAFile:         filepath.Join(home, "cacert.pem"),
		KnownHostsFile: filepath.Join(home, "known_hosts"),
	}

	return &result
}

func (c *PortierConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := portierConfigYAML{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	baseURL, relayPath, err := parseConfiguredConnection(raw.BaseURL, raw.LegacyPortierURL)
	if err != nil {
		return err
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	c.BaseURL = utils.YAMLURL{URL: parsedBaseURL}
	c.RelayPath = relayPath
	c.TLSEnabled = raw.TLSEnabled
	c.PTLSConfig = raw.PTLSConfig
	c.Services = raw.Services
	c.DefaultResponseInterval = raw.DefaultResponseInterval
	c.DefaultReadTimeout = raw.DefaultReadTimeout
	c.DefaultThroughputLimit = raw.DefaultThroughputLimit
	c.DefaultReadBufferSize = raw.DefaultReadBufferSize
	c.DefaultDatagramConnectionID = raw.DefaultDatagramConnectionID

	return nil
}

func (c PortierConfig) MarshalYAML() (interface{}, error) {
	return portierConfigYAML{
		BaseURL:                     c.APIBaseURL(),
		TLSEnabled:                  c.TLSEnabled,
		PTLSConfig:                  c.PTLSConfig,
		Services:                    c.Services,
		DefaultResponseInterval:     c.DefaultResponseInterval,
		DefaultReadTimeout:          c.DefaultReadTimeout,
		DefaultThroughputLimit:      c.DefaultThroughputLimit,
		DefaultReadBufferSize:       c.DefaultReadBufferSize,
		DefaultDatagramConnectionID: c.DefaultDatagramConnectionID,
	}, nil
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

func APIBaseURLFromPortierURL(portierURL string) string {
	normalizedBaseURL := endpoints.NormalizeBaseURL(portierURL)
	if normalizedBaseURL == "" {
		return "https://api.portier.dev"
	}

	return normalizedBaseURL
}

func normalizeAPIBaseURL(baseURL string) string {
	result := endpoints.NormalizeBaseURL(baseURL)
	if result == "" {
		return ""
	}
	return result
}

func LoadApiTokenWithBaseURL(filePath string, baseURL string) (*DeviceCredentials, error) {
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

	baseURL = normalizeAPIBaseURL(baseURL)
	if baseURL == "" {
		baseURL = "https://api.portier.dev"
	}

	guid, err := api.WhoAmI(baseURL, fc.ApiToken)
	if err != nil {
		return nil, fmt.Errorf("could not get device ID: %w", err)
	}

	credentials := DeviceCredentials{
		DeviceID: guid,
		ApiToken: fc.ApiToken,
	}

	return &credentials, nil
}

func LoadApiToken(filePath string) (*DeviceCredentials, error) {
	return LoadApiTokenWithBaseURL(filePath, "https://api.portier.dev")
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
		BaseURL: utils.YAMLURL{
			URL: &url.URL{
				Scheme: "https",
				Host:   "api.portier.dev",
			},
		},
		RelayPath:                   "/spider",
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

func (c *PortierConfig) APIBaseURL() string {
	if c == nil {
		return "https://api.portier.dev"
	}

	normalizedBaseURL := normalizeAPIBaseURL(c.BaseURL.String())
	if normalizedBaseURL == "" {
		return "https://api.portier.dev"
	}

	return normalizedBaseURL
}

func (c *PortierConfig) RelayPathOrDefault() string {
	if c == nil || strings.TrimSpace(c.RelayPath) == "" {
		return "/spider"
	}

	return strings.TrimSpace(c.RelayPath)
}

func (c *PortierConfig) RelayURL() (string, error) {
	return endpoints.RelayWebsocketURL(c.APIBaseURL(), c.RelayPathOrDefault())
}

func (c *PortierConfig) IsTaskRelay() bool {
	relayPath := c.RelayPathOrDefault()
	return strings.HasPrefix(relayPath, "/task-spider/") || strings.HasPrefix(relayPath, "/api/tasks/")
}

func parseConfiguredConnection(baseURL, legacyPortierURL string) (string, string, error) {
	baseURL = strings.TrimSpace(baseURL)
	legacyPortierURL = strings.TrimSpace(legacyPortierURL)

	if baseURL == "" && legacyPortierURL == "" {
		return "https://api.portier.dev", "/spider", nil
	}

	rawURL := baseURL
	if rawURL == "" {
		rawURL = legacyPortierURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return "", "", fmt.Errorf("invalid base URL %q", rawURL)
	}

	normalizedBaseURL := normalizeAPIBaseURL(rawURL)
	if normalizedBaseURL == "" {
		return "", "", fmt.Errorf("invalid base URL %q", rawURL)
	}

	return normalizedBaseURL, extractRelayPath(parsedURL.Path), nil
}

func extractRelayPath(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	switch {
	case path == "", path == "/", path == "/api", strings.HasSuffix(path, "/api"):
		return "/spider"
	case taskRelayPath(path) != "":
		return taskRelayPath(path)
	case strings.HasSuffix(path, "/spider"):
		index := strings.LastIndex(path, "/api/tasks/")
		if index != -1 {
			return path[index:]
		}
		return "/spider"
	default:
		return "/spider"
	}
}

func taskRelayPath(path string) string {
	if index := strings.LastIndex(path, "/task-spider/"); index != -1 {
		return path[index:]
	}

	return ""
}
