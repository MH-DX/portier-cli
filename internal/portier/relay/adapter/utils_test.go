package adapter

import (
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"github.com/stretchr/testify/mock"
)

type MockUplink struct {
	mock.Mock
}

func (m *MockUplink) Connect() (<-chan messages.Message, error) {
	m.Called()
	return nil, nil
}

func (m *MockUplink) Send(message messages.Message) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockUplink) Close() error {
	m.Called()
	return nil
}

func (m *MockUplink) Events() <-chan uplink.UplinkEvent {
	m.Called()
	return nil
}

type MockEncryption struct {
	mock.Mock
}

func (m *MockEncryption) Decrypt(header messages.MessageHeader, data []byte) ([]byte, error) {
	args := m.Called(header, data)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockEncryption) Encrypt(header messages.MessageHeader, data []byte) ([]byte, error) {
	args := m.Called(header, data)
	return args.Get(0).([]byte), args.Error(1)
}
