package relay

import (
	"net/url"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

// ServiceOptions defines the local options for the service
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

type Router interface {
	// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
	// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
	// Returns an error if the message could not be routed.
	HandleMessage(msg messages.Message) error

	// AddConnection adds a connection to the router
	AddConnection(messages.ConnectionId, ConnectionAdapter)

	// RemoveConnection removes a connection from the router
	RemoveConnection(messages.ConnectionId)
}

// EncoderDecoder is the interface for encoding and decoding messages (using msgpack)
type EncoderDecoder interface {
	// Encode encodes a message
	Encode(messages.Message) ([]byte, error)

	// Decode decodes a message
	Decode([]byte) (messages.Message, error)

	// DecodeConnectionOpenMessage decodes a connection open message
	DecodeConnectionOpenMessage([]byte) (messages.ConnectionOpenMessage, error)

	// EncodeConnectionOpenMessage encodes a connection open message
	EncodeConnectionOpenMessage(messages.ConnectionOpenMessage) ([]byte, error)

	// Decode ConnectionAcceptMessage decodes a connection accept message
	DecodeConnectionAcceptMessage([]byte) (messages.ConnectionAcceptMessage, error)

	// EncodeConnectionAcceptMessage encodes a connection accept message
	EncodeConnectionAcceptMessage(messages.ConnectionAcceptMessage) ([]byte, error)

	// Decode ConnectionFailedMessage decodes a connection failed message
	DecodeConnectionFailedMessage([]byte) (messages.ConnectionFailedMessage, error)

	// EncodeConnectionFailedMessage encodes a connection failed message
	EncodeConnectionFailedMessage(messages.ConnectionFailedMessage) ([]byte, error)
}

// State is the state of the relay
type UplinkState string

const (
	// UplinkStateDisconnected is the state when the uplink is disconnected
	UplinkStateDisconnected UplinkState = "disconnected"

	// UplinkStateConnected is the state when the uplink is connected
	UplinkStateConnected UplinkState = "connected"
)

type UplinkEvent struct {
	// State is the state of the uplink
	State UplinkState
	// Event
	Event string
}

// Uplink is the uplink interface to the portier server. It is used to send messages to the portier server and to receive messages from the portier server.
// Moreover, it has to handle connection loss and reconnect to the portier server.
type Uplink interface {
	// Connect connects to the portier server return recv channel to receive messages from the portier server.
	// The channels will be to be closed by the uplink when the connection to the portier server is closed.
	// The recv channel will have no buffer and it is mandatory that the Router processes messages in a non-blocking way.
	Connect() (chan []byte, error)

	// Send enqueues a message to the portier server.
	// The Uplink has only a small buffer to realize backpressure in case the uplink cannot keep up with the messages, i.e. it will block.
	// This blocking must be effectively throttling the Service.
	Send(messages.Message) error

	// Close closes the uplink, the connection to the portier server and expects the uplink to close the recv channel
	Close() error

	// Returns a recv channel to listen for events
	Events() <-chan UplinkEvent
}

// Relay is the portier relay to bridging TCP / UDP traffic via websocket to the portier server
type Relay struct {
	//The portier server URL
	ServerURL string

	//The service document
	Services []Service

	//Router is the router that is used to route traffic to the correct service
	Router Router

	// Uplink is the uplink that is used to send traffic to the portier server
	Uplink Uplink
}

type Connector interface {
	// CreateConnection creates a new connection
	CreateInboundConnection(header messages.MessageHeader, options messages.BridgeOptions, pcKey string) error
}
