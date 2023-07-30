package relay

import (
	"net/url"
	"time"
)

// ServiceOptions defines the local options for the service
type ServiceOptions struct {
	// The max queue size of messages to fetch from the connection
	MaxQueueSize int
}

// Cipher is the cipher type and can be AES-256-GCM
type Cipher string

// Curve is the curve type and can be P256
type Curve string

// BridgeOptions defines the options for the bridge, which are shared with the relay on the other side of the bridge
// when this relay attempts to open a connection to the other relay.
type BridgeOptions struct {
	// Timestamp is the timestamp of the connection opening
	Timestamp time.Time

	// The remote URL
	URLRemote url.URL

	// RateLimit is the rate limit in bytes per second that is applied to the connection
	RateLimitBytesPerSecond int

	// AckWindowSize is the size of the ack window, i.e. the number of messages that are sent before an ack is expected
	AckWindowSize int

	// Cipher is the cipher that is used to encrypt the data
	Cipher Cipher

	// Curve is the canonical name of the curve that is used to generate the keys
	Curve Curve
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

	// BridgeOptions defines the options for the bridge, which are shared
	BridgeOptions BridgeOptions
}

type ServiceDocument struct {
	// The services
	Services []Service
}

type MessageType string

type ConnectionId string

const (
	// ConnectionOpenMessage is a message that is sent when a connection is opened
	CO MessageType = "CO"

	// ConnectionCloseMessage is a message that is sent when a connection is closed
	CC MessageType = "CC"

	// ConnectionAcceptMessage is a message that is sent when a connection is accepted
	CA MessageType = "CA"

	// ConnectionFailedMessage is a message that is sent when a connection open attempt failed
	CF MessageType = "CF"

	// DataMessage is a message that contains data
	D MessageType = "D"

	// DataAckMessage is a message that is sent when data with a sequence number is received
	DA MessageType = "DA"
)

type MessageHeader struct {
	// From is the spider device Id of the sender of the message
	From string

	// To is the spider device Id of the recipient of the message
	To string

	// The type of this message
	Type MessageType

	// CID is a uuid for the connection
	CID ConnectionId
}

// Message is a message that is sent to the portier server
type Message struct {
	// Header is the plaintext, but authenticated header of the message
	Header MessageHeader

	// Message is the serialized and encrypted message, i.e. a DataMessage
	Message []byte
}

// ConnectionOpenMessage is a message that is sent when a connection is opened
type ConnectionOpenMessage struct {
	// BridgeOptions defines the options for the bridge, which are shared
	BridgeOptions BridgeOptions

	// PCKey is the ephemeral public connection key, used to encrypt&sign the data for this connection
	PCKey string
}

type ConnectionAcceptMessage struct {
	// PCKey is the ephemeral public connection key, used to encrypt&sign the data for this connection
	PCKey string
}

// ConnectionFailedMessage is a message that is sent when a connection open attempt failed
type ConnectionFailedMessage struct {
	// Reason is the reason why the connection failed
	Reason string
}

// DataMessage is a message that contains data
type DataMessage struct {
	// Seq is the sequence number of the data
	Seq uint64

	// Data is the actual payload from the bridged connection
	Data []byte
}

// DataAckMessage is a message that is sent when data with a sequence number is received
type DataAckMessage struct {
	// Seq is the sequence number of the data
	Seq uint64
}

type Router interface {
	// HandleMessage handles a message, i.e. creates a new service if necessary and routes the message to the service,
	// or routes the message to the existing service, or shuts down the service if the message is a shutdown message.
	// Returns an error if the message could not be routed.
	HandleMessage(msg Message) error

	// AddConnection adds a connection to the router
	AddConnection(ConnectionId, ConnectionAdapter)

	// RemoveConnection removes a connection from the router
	RemoveConnection(ConnectionId)
}

// EncoderDecoder is the interface for encoding and decoding messages (using msgpack)
type EncoderDecoder interface {
	// Encode encodes a message
	Encode(Message) ([]byte, error)

	// Decode decodes a message
	Decode([]byte) (Message, error)

	// DecodeConnectionOpenMessage decodes a connection open message
	DecodeConnectionOpenMessage([]byte) (ConnectionOpenMessage, error)
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
	Send([]byte) error

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
	Services ServiceDocument

	//Router is the router that is used to route traffic to the correct service
	Router Router

	// Uplink is the uplink that is used to send traffic to the portier server
	Uplink Uplink
}

type ConnectionAdapter interface {
	// Start starts the connection
	Start() error

	// Send sends a message to the connection
	Send(payload []byte) error
}

type ConnectionProvider interface {
	// CreateConnection creates a new connection
	CreateInboundConnection(header MessageHeader, options BridgeOptions, pcKey string, router Router) error
}
