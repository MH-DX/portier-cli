package config

import (
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/relay"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/utils"
)

type PortierConfig struct {
	PortierURL                  utils.YAMLURL         `yaml:"portierUrl"`
	Services                    []relay.Service       `yaml:"services"`
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
	Service  relay.Service
	Listener net.Listener
}
