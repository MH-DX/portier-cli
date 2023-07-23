package encoder

import (
	"github.com/marinator86/portier-cli/internal/portier/relay"
	"github.com/vmihailenco/msgpack"
)

type EncoderDecoder struct {
}

// Encode encodes a message
func (e *EncoderDecoder) Encode(msg relay.Message) ([]byte, error) {
	// use msgpack to encode the message
	msgpack, err := msgpack.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return msgpack, nil
}

// Decode decodes a message
func (e *EncoderDecoder) Decode(msg []byte) (relay.Message, error) {
	// use msgpack to decode the message
	var message relay.Message
	err := msgpack.Unmarshal(msg, &message)
	if err != nil {
		return relay.Message{}, err
	}
	return message, nil
}
