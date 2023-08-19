package encoder

import (
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/vmihailenco/msgpack"
)

type EncoderDecoder struct {
}

// Encode encodes a message
func (e *EncoderDecoder) Encode(msg messages.Message) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode decodes a message
func (e *EncoderDecoder) Decode(msg []byte) (messages.Message, error) {
	// use msgpack to decode the message
	var message messages.Message
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.Message{}, err
	}
	return message, nil
}

// DecodeConnectionOpenMessage decodes a connection open message
func (e *EncoderDecoder) DecodeConnectionOpenMessage(msg []byte) (messages.ConnectionOpenMessage, error) {
	// use msgpack to decode the message
	var message messages.ConnectionOpenMessage
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.ConnectionOpenMessage{}, err
	}
	return message, nil
}

// EncodeConnectionOpenMessage encodes a connection open message
func (e *EncoderDecoder) EncodeConnectionOpenMessage(msg messages.ConnectionOpenMessage) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode ConnectionAcceptMessage decodes a connection accept message
func (e *EncoderDecoder) DecodeConnectionAcceptMessage(msg []byte) (messages.ConnectionAcceptMessage, error) {
	// use msgpack to decode the message
	var message messages.ConnectionAcceptMessage
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.ConnectionAcceptMessage{}, err
	}
	return message, nil
}

// EncodeConnectionAcceptMessage encodes a connection accept message
func (e *EncoderDecoder) EncodeConnectionAcceptMessage(msg messages.ConnectionAcceptMessage) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode ConnectionFailedMessage decodes a connection failed message
func (e *EncoderDecoder) DecodeConnectionFailedMessage(msg []byte) (messages.ConnectionFailedMessage, error) {
	// use msgpack to decode the message
	var message messages.ConnectionFailedMessage
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return messages.ConnectionFailedMessage{}, err
	}
	return message, nil
}

// EncodeConnectionFailedMessage encodes a connection failed message
func (e *EncoderDecoder) EncodeConnectionFailedMessage(msg messages.ConnectionFailedMessage) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}
