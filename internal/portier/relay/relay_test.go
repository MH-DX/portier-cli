package relay

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/connector"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
	"github.com/stretchr/testify/mock"
)

var spider = Spider{
	channels: make(map[uuid.UUID]chan messages.Message),
	encoder:  encoder.NewEncoderDecoder(),
}

type Spider struct {
	// map from device id to channels
	channels map[uuid.UUID]chan messages.Message

	// encoder
	encoder encoder.EncoderDecoder
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	// create channels
	outChannel := make(chan messages.Message)
	// get device id from Authorization header
	header := r.Header.Get("Authorization")
	deviceId := uuid.MustParse(header)

	spider.channels[deviceId] = outChannel

	// start goroutine to read from in channel and write to target device channel
	go func() {
		for {
			_, message, _ := c.ReadMessage()
			msg, _ := spider.encoder.Decode(message)
			toDeviceId := msg.Header.To
			toChannel := spider.channels[toDeviceId]
			toChannel <- msg
		}
	}()

	// start goroutine to read from out channel and write to websocket
	go func() {
		for {
			msg := <-outChannel
			encoded, _ := spider.encoder.Encode(msg)
			c.WriteMessage(websocket.BinaryMessage, encoded)
		}
	}()
}

func createUplink(deviceId string, url string) uplink.Uplink {
	options := uplink.Options{
		APIToken:   deviceId,
		PortierURL: url,
	}

	return uplink.NewWebsocketUplink(options, nil)
}

func TestConnectAndBridging(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echo))

	device1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	device2, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	url := "ws" + server.URL[4:]

	uplink1 := createUplink(device1.String(), url)
	uplink1.Connect()
	downlink2, _ := createUplink(device2.String(), url).Connect()

	// WHEN
	msg := messages.Message{
		Header: messages.MessageHeader{
			From: device1,
			To:   device2,
		},
		Message: []byte("Hello, world!"),
	}
	uplink1.Send(msg) // send message to the uplink

	// THEN
	response := <-downlink2
	if response.Header != msg.Header {
		testing.Errorf("expected %v, got %v", msg, response)
	}
}

func TestForwarding(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echo))

	device1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	device2, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")
	cid := messages.ConnectionId("test-connection-id")
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	ws_url := "ws" + server.URL[4:]

	// create forwarding tcp server
	ln, _ := net.Listen("tcp", "127.0.0.1:18080")
	defer ln.Close()
	fromOptions := createConnectionAdapterOptions(cid, device1, device2, "tcp://localhost:18081")

	inboundEvents := make(chan adapter.AdapterEvent)
	outboundEvents := make(chan adapter.AdapterEvent)

	forwarded, _ := net.Listen("tcp", "127.0.0.1:18081")
	defer forwarded.Close()

	router2 := createInboundRelay(device2, ws_url, inboundEvents)
	router2.Start()

	router1, adapter1, listenerConn := createOutboundRelay(device1, fromOptions, ws_url, outboundEvents, ln)
	router1.Start()
	router1.AddConnection(cid, adapter1)

	// Starting the outbound adapter will initiate sending the connection open message
	err := adapter1.Start()
	if err != nil {
		testing.Errorf("error starting adapter: %v", err)
	}

	// WHEN
	// wait for the forwardED listener to accept a connection from the inbound adapter
	forwardedConn, _ := forwarded.Accept()
	// then write to the forwardING connection
	listenerConn.Write([]byte("Hello, world!"))

	// THEN
	// wait till the forwarded connection receives the message, i.e. the inbound adapter has forwarded the message
	buf := make([]byte, 1024)
	n, _ := forwardedConn.Read(buf)
	if string(buf[:n]) != "Hello, world!" {
		testing.Errorf("expected %v, got %v", "Hello, world!", string(buf[:n]))
	}
}

func createOutboundRelay(deviceId uuid.UUID, opts adapter.ConnectionAdapterOptions, url string, events chan<- adapter.AdapterEvent, ln net.Listener) (router.Router, adapter.ConnectionAdapter, net.Conn) {
	uplink := createUplink(deviceId.String(), url)
	controller := &ControllerMock{}
	controller.On("GetEventChannel").Return(events)
	connector := connector.NewConnector(uplink, controller)
	messageChannel, _ := uplink.Connect()
	router := router.NewRouter(connector, uplink, messageChannel)

	// dial tcp server
	cConn, _ := net.Dial("tcp", "127.0.0.1:18080")
	sConn, _ := ln.Accept()

	adapter := adapter.NewOutboundConnectionAdapter(opts, sConn, uplink, events)
	return router, adapter, cConn
}

func createInboundRelay(deviceId uuid.UUID, url string, events chan<- adapter.AdapterEvent) router.Router {
	uplink := createUplink(deviceId.String(), url)
	controller := &ControllerMock{}
	controller.On("GetEventChannel").Return(events)
	connector := connector.NewConnector(uplink, controller)
	messageChannel, _ := uplink.Connect()
	router := router.NewRouter(connector, uplink, messageChannel)
	controller.On("AddConnection", mock.MatchedBy(func(cid messages.ConnectionId) bool {
		return true
	}), mock.MatchedBy(func(adapter adapter.ConnectionAdapter) bool {
		router.AddConnection("test-connection-id", adapter)
		return true
	})).Return(nil)
	return router
}

func createConnectionAdapterOptions(cid messages.ConnectionId, from uuid.UUID, to uuid.UUID, rawUrl string) adapter.ConnectionAdapterOptions {
	url, _ := url.Parse(rawUrl)
	return adapter.ConnectionAdapterOptions{
		ConnectionId:  cid,
		LocalDeviceId: from,
		PeerDeviceId:  to,
		BridgeOptions: messages.BridgeOptions{
			URLRemote: *url,
		},
		ResponseInterval:      time.Millisecond * 1000,
		ConnectionReadTimeout: time.Millisecond * 1000,
		ReadBufferSize:        1024,
	}
}

type ControllerMock struct {
	mock.Mock
}

func (c *ControllerMock) AddConnection(cid messages.ConnectionId, adapter adapter.ConnectionAdapter) error {
	c.Called(cid, adapter)
	return nil
}

func (c *ControllerMock) GetEventChannel() chan<- adapter.AdapterEvent {
	args := c.Called()
	return args.Get(0).(chan<- adapter.AdapterEvent)
}
