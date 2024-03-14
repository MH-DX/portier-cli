package relay

import (
	"net/url"
	"time"

	"github.com/google/uuid"
)

// ServiceOptions are options for the service that need to be known beforehand.
type ServiceOptions struct {
	// The local URL
	URLLocal url.URL

	// The remote URL the bridge has to connect to
	URLRemote url.URL

	// The remote device id
	PeerDeviceID uuid.UUID

	// The remote device public key
	PeerDevicePublicKey string

	// The Cipher that is used to encrypt the data
	Cipher string

	// The Curve that is used to generate the keys
	Curve string

	// The local public key
	LocalPublicKey string

	// The local private key
	LocalPrivateKey string

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
	Name string

	// ServiceOptions defines the options for the service
	Options ServiceOptions
}

type RelayConfig struct {
	// The portier server URL
	ServerURL string

	// The services that are exposed by the relay
	Services []Service
}
