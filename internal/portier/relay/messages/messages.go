package messages

import (
	"net/url"
	"time"

	"github.com/google/uuid"
)

type ConnectionId string

type MessageType string

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
	Cipher string

	// Curve is the canonical name of the curve that is used to generate the keys
	Curve string
}

type MessageHeader struct {
	// From is the spider device Id of the sender of the message
	From uuid.UUID

	// To is the spider device Id of the recipient of the message
	To uuid.UUID

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
