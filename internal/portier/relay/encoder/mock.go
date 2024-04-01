package encoder

import (
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/stretchr/testify/mock"
)

type MockEncoderDecoder struct {
	mock.Mock
}

func (m *MockEncoderDecoder) EncodeDataMessage(dm messages.DataMessage) ([]byte, error) {
	args := m.Called(dm)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) DecodeDataMessage(data []byte) (messages.DataMessage, error) {
	args := m.Called(data)
	return args.Get(0).(messages.DataMessage), args.Error(1)
}

func (m *MockEncoderDecoder) EncodeDatagramMessage(dm messages.DatagramMessage) ([]byte, error) {
	args := m.Called(dm)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) DecodeDatagramMessage(data []byte) (messages.DatagramMessage, error) {
	args := m.Called(data)
	return args.Get(0).(messages.DatagramMessage), args.Error(1)
}

func (m *MockEncoderDecoder) EncodeConnectionOpenMessage(dm messages.ConnectionOpenMessage) ([]byte, error) {
	args := m.Called(dm)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) DecodeConnectionOpenMessage(data []byte) (messages.ConnectionOpenMessage, error) {
	args := m.Called(data)
	return args.Get(0).(messages.ConnectionOpenMessage), args.Error(1)
}

func (m *MockEncoderDecoder) EncodeConnectionAcceptMessage(dm messages.ConnectionAcceptMessage) ([]byte, error) {
	args := m.Called(dm)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) DecodeConnectionAcceptMessage(data []byte) (messages.ConnectionAcceptMessage, error) {
	args := m.Called(data)
	return args.Get(0).(messages.ConnectionAcceptMessage), args.Error(1)
}

func (m *MockEncoderDecoder) EncodeConnectionFailedMessage(dm messages.ConnectionFailedMessage) ([]byte, error) {
	args := m.Called(dm)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) DecodeConnectionFailedMessage(data []byte) (messages.ConnectionFailedMessage, error) {
	args := m.Called(data)
	return args.Get(0).(messages.ConnectionFailedMessage), args.Error(1)
}

func (m *MockEncoderDecoder) EncodeDataAckMessage(dm messages.DataAckMessage) ([]byte, error) {
	args := m.Called(dm)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) DecodeDataAckMessage(data []byte) (messages.DataAckMessage, error) {
	args := m.Called(data)
	return args.Get(0).(messages.DataAckMessage), args.Error(1)
}

func (m *MockEncoderDecoder) Encode(msg messages.Message) ([]byte, error) {
	args := m.Called(msg)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncoderDecoder) Decode(msg []byte) (messages.Message, error) {
	args := m.Called(msg)
	return args.Get(0).(messages.Message), args.Error(1)
}
