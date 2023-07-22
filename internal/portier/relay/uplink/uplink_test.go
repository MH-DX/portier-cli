package uplink

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay"
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
		if string(message) == "close" {
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
	msg := []byte("Hello, world!")

	uplink := NewWebsocketUplink(options)
	channel, err := uplink.Connect()
	if err != nil {
		testing.Errorf("error connecting to websocket: %v", err)
	}

	// WHEN
	uplink.Send(msg) // send message to the uplink

	// THEN
	response := <-channel
	if string(response) != string(msg) {
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
	msg := []byte("Hello, world!")

	uplink := NewWebsocketUplink(options)
	eventChannel := uplink.Events()
	channel, err := uplink.Connect()
	if err != nil {
		testing.Errorf("error connecting to websocket: %v", err)
	}
	// expect connected event
	event := <-eventChannel
	if event.State != relay.UplinkStateConnected {
		testing.Errorf("expected %v, got %v", relay.UplinkStateConnected, event.State)
	}

	// WHEN
	uplink.Send([]byte("close")) // send message to the uplink to close the connection

	// THEN
	event = <-eventChannel
	if event.State != relay.UplinkStateDisconnected {
		testing.Errorf("expected %v, got %v", relay.UplinkStateDisconnected, event.State)
	}
	event = <-eventChannel
	if event.State != relay.UplinkStateConnected {
		testing.Errorf("expected %v, got %v", relay.UplinkStateConnected, event.State)
	}

	// WHEN
	uplink.Send(msg) // send message to the uplink

	// THEN
	response := <-channel
	if string(response) != string(msg) {
		testing.Errorf("expected %v, got %v", msg, response)
	}
}
