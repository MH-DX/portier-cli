package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/uplink"
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
