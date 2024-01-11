package relay

import (
	"net/url"

	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
)

// ServiceOptions defines the local options for the service.
type ServiceOptions struct {
	// The max queue size of messages to fetch from the connection
	MaxQueueSize int

	// The remote URL the bridge has to connect to
	URLRemote url.URL

	// RateLimit is the rate limit in bytes per second that is applied to the connection
	RateLimitBytesPerSecond int

	// AckWindowSize is the size of the ack window, i.e. the number of messages that are sent before an ack is expected
	AckWindowSize int
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

	// The local URL
	URLLocal string

	// ServiceOptions defines the options for the service
	Options ServiceOptions
}

// Relay is the portier relay to bridging TCP / UDP traffic via websocket to the portier server.
type Relay struct {
	ServerURL string

	Services []Service
	Router   router.Router

	// Uplink is the uplink that is used to send traffic to the portier server
	Uplink uplink.Uplink
}
