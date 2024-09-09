package utils

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
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

func EchoWithLoss(n int) func(w http.ResponseWriter, r *http.Request) {
	result := func(w http.ResponseWriter, r *http.Request) {
		i := 0
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
				i++
				if n != 0 && i%n == 0 {
					// fmt.Printf("Dropping message: %s\n", message)
					continue
				}

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
				_ = c.WriteMessage(websocket.BinaryMessage, encoded)
			}
		}()
	}
	return result
}
