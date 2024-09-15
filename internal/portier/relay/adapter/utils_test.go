package adapter

import (
	"net"
	"net/url"

	"github.com/google/uuid"
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

func (m *MockUplink) Events() <-chan uplink.Event {
	m.Called()
	return nil
}

type MockPTLS struct {
	mock.Mock
}

func (m *MockPTLS) TestEndpointURL(endpoint url.URL) bool {
	args := m.Called(endpoint)
	return args.Bool(0)
}

func (m *MockPTLS) CreateClientAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, func() error, error) {
	args := m.Called(conn, peerDeviceID)
	return args.Get(0).(net.Conn), args.Get(1).(func() error), args.Error(2)
}

func (m *MockPTLS) CreateServerAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, error) {
	args := m.Called(conn, peerDeviceID)
	return args.Get(0).(net.Conn), args.Error(1)
}
