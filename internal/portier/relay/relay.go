package relay

import (
	"time"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/utils"
)

// ServiceOptions are options for the service that need to be known beforehand.
type ServiceOptions struct {
	// The local URL
	URLLocal utils.YAMLURL `yaml:"urlLocal" validate:"required"`

	// The remote URL the bridge has to connect to
	URLRemote utils.YAMLURL `yaml:"urlRemote" validate:"required"`

	// The remote device id
	PeerDeviceID uuid.UUID `yaml:"peerDeviceID" validate:"required,uuid"`

	// IsSecure indicates whether the connection is secured with TLS
	TLSDisabled bool `yaml:"isSecure"`

	// The connection adapter's response interval for re-transmitting control messages
	ResponseInterval time.Duration

	// The connection adapter's read timeout
	ConnectionReadTimeout time.Duration

	// The rate limit in bytes per second that is applied to the connection
	ThroughputLimit int

	// The TCP read buffer size
	ReadBufferSize int
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
