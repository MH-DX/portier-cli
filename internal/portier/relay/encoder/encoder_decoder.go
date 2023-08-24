package encoder

import (
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/vmihailenco/msgpack"
)

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

type encoderDecoder struct {
}

// NewEncoderDecoder creates a new encoder/decoder
func NewEncoderDecoder() EncoderDecoder {
	return &encoderDecoder{}
}

// Encode encodes a message
func (e *encoderDecoder) Encode(msg messages.Message) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode decodes a message
func (e *encoderDecoder) Decode(msg []byte) (messages.Message, error) {
	// use msgpack to decode the message
	var message messages.Message
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.Message{}, err
	}
	return message, nil
}

// DecodeConnectionOpenMessage decodes a connection open message
func (e *encoderDecoder) DecodeConnectionOpenMessage(msg []byte) (messages.ConnectionOpenMessage, error) {
	// use msgpack to decode the message
	var message messages.ConnectionOpenMessage
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.ConnectionOpenMessage{}, err
	}
	return message, nil
}

// EncodeConnectionOpenMessage encodes a connection open message
func (e *encoderDecoder) EncodeConnectionOpenMessage(msg messages.ConnectionOpenMessage) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode ConnectionAcceptMessage decodes a connection accept message
func (e *encoderDecoder) DecodeConnectionAcceptMessage(msg []byte) (messages.ConnectionAcceptMessage, error) {
	// use msgpack to decode the message
	var message messages.ConnectionAcceptMessage
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.ConnectionAcceptMessage{}, err
	}
	return message, nil
}

// EncodeConnectionAcceptMessage encodes a connection accept message
func (e *encoderDecoder) EncodeConnectionAcceptMessage(msg messages.ConnectionAcceptMessage) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode ConnectionFailedMessage decodes a connection failed message
func (e *encoderDecoder) DecodeConnectionFailedMessage(msg []byte) (messages.ConnectionFailedMessage, error) {
	// use msgpack to decode the message
	var message messages.ConnectionFailedMessage
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.ConnectionFailedMessage{}, err
	}
	return message, nil
}

// EncodeConnectionFailedMessage encodes a connection failed message
func (e *encoderDecoder) EncodeConnectionFailedMessage(msg messages.ConnectionFailedMessage) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}
