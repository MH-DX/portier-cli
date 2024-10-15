package uplink

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
)

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
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}
		// decode message
		encoder := encoder.NewEncoderDecoder()
		msg, _ := encoder.Decode(message)
		if msg.Header.Type == "close" {
			return
		}
		err = c.WriteMessage(mt, message)
		if err != nil {
			break
		}
	}
}

func TestConnectAndEcho(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echo))
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	url := "ws" + server.URL[4:]
	options := defaultOptions()
	options.PortierURL = url
	options.APIToken = "80451937-0625-4ffe-b97c-b2ec9e75a0a5"
	msg := messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
		},
		Message: []byte("Hello, world!"),
	}

	uplink := NewWebsocketUplink(options, nil)
	channel, err := uplink.Connect()
	if err != nil {
		testing.Errorf("error connecting to websocket: %v", err)
	}

	// WHEN
	_ = uplink.Send(msg) // send message to the uplink

	// THEN
	response := <-channel
	if response.Header != msg.Header {
		testing.Errorf("expected %v, got %v", msg, response)
	}
}

func TestReconnect(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echo))
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	url := "ws" + server.URL[4:]
	options := defaultOptions()
	options.PortierURL = url
	options.APIToken = "80451937-0625-4ffe-b97c-b2ec9e75a0a5"
	closeMsg := messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: "close",
		},
		Message: []byte("Hello, world!"),
	}

	okayMsg := messages.Message{
		Header: messages.MessageHeader{
			From: uuid.New(),
			To:   uuid.New(),
			Type: messages.D,
		},
		Message: []byte("Hello, world!"),
	}

	uplink := NewWebsocketUplink(options, nil)
	eventChannel := uplink.Events()
	channel, err := uplink.Connect()
	if err != nil {
		testing.Errorf("error connecting to websocket: %v", err)
	}
	// expect connected event
	event := <-eventChannel
	if event.State != Disconnected && event.Event != "connecting to portier server: "+url {
		testing.Errorf("expected %v, got %v", Disconnected, event.State)
	}
	event = <-eventChannel
	if event.State != Connected {
		testing.Errorf("expected %v, got %v", Connected, event.State)
	}

	// WHEN
	_ = uplink.Send(closeMsg) // send message to the uplink to close the connection

	// THEN
	event = <-eventChannel
	if event.State != Disconnected {
		testing.Errorf("expected %v, got %v", Disconnected, event.State)
	}
	event = <-eventChannel
	if event.State != Disconnected && event.Event != "connecting to portier server: "+url {
		testing.Errorf("expected %v, got %v", Disconnected, event.State)
	}
	event = <-eventChannel
	if event.State != Connected {
		testing.Errorf("expected %v, got %v", Connected, event.State)
	}

	// WHEN
	_ = uplink.Send(okayMsg) // send message to the uplink

	// THEN
	response := <-channel
	if response.Header != okayMsg.Header {
		testing.Errorf("expected %v, got %v", okayMsg, response)
	}
}
