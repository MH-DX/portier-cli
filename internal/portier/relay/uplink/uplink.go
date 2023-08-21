package uplink

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

// dialer is the websocket dialer
var dialer = websocket.Dialer{}

type Options struct {
	// PortierURL is the URL of the portier server
	PortierURL string

	// APIToken is the API token of the portier server
	APIToken string

	// MaxReconnectInterval is the maximum time to wait between reconnects
	MaxReconnectInterval time.Duration

	// ReconnectRetries is the number of retries to reconnect to the portier server
	ReconnectRetries int64
}

type WebsocketUplink struct {
	// Options defines the options for the uplink
	Options Options

	// retries is the number of retries to reconnect to the portier server
	retries int64

	// conn is the websocket connection to the portier server
	connection *websocket.Conn

	// recv is the channel to receive messages from the portier server
	recv chan []byte

	// mutex is the mutex to lock the connection
	mutex sync.Mutex

	// events is the channel to receive events from the uplink
	events chan relay.UplinkEvent

	// encoderdecoder is the encoder / decoder for the uplink
	encoderDecoder *encoder.EncoderDecoder
}

func defaultOptions() Options {
	return Options{
		MaxReconnectInterval: 10 * time.Second,
		ReconnectRetries:     0,
	}
}

// NewWebsocketUplink creates a new websocket uplink
func NewWebsocketUplink(options Options, encoderDecoder *encoder.EncoderDecoder) *WebsocketUplink {
	if options.APIToken == "" {
		log.Fatal("API token is required")
	}
	if options.PortierURL == "" {
		log.Fatal("Portier URL is required")
	}
	if options.MaxReconnectInterval == 0 {
		options.MaxReconnectInterval = defaultOptions().MaxReconnectInterval
	}
	if options.ReconnectRetries == 0 {
		options.ReconnectRetries = defaultOptions().ReconnectRetries
	}
	if encoderDecoder == nil {
		encoderDecoder = new(encoder.EncoderDecoder)
	}

	return &WebsocketUplink{
		Options:        options,
		recv:           make(chan []byte, 1),
		events:         make(chan relay.UplinkEvent, 100),
		encoderDecoder: encoderDecoder,
	}
}

// Connect connects to the portier server return recv channel and a send channel to receive / send messages from /to the portier server.
func (u *WebsocketUplink) Connect() (<-chan []byte, error) {
	// Connect to the portier server
	err := u.connectWebsocket()
	if err != nil {
		return nil, err
	}
	return u.recv, nil
}

// Send enqueues a message to the portier server.
func (u *WebsocketUplink) Send(message messages.Message) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	payload, err := u.encoderDecoder.Encode(message)
	if err != nil {
		return err
	}
	err = u.connection.WriteMessage(websocket.BinaryMessage, payload)
	if err != nil {
		if websocket.IsUnexpectedCloseError(err) {
			log.Printf("websocket disconnected: %v", err)
			u.events <- relay.UplinkEvent{
				State: relay.UplinkStateDisconnected,
				Event: "websocket disconnected",
			}
			time.Sleep(u.calculateBackoff())
			u.connectWebsocket()
		}
	}
	return nil
}

// Close closes the uplink, the connection to the portier server and expects the uplink to close the recv channel
func (u *WebsocketUplink) Close() error {
	u.connection.Close()
	return nil
}

func (u *WebsocketUplink) Events() <-chan relay.UplinkEvent {
	return u.events
}

func (u *WebsocketUplink) connectWebsocket() error {
	// Create a header with the API token
	header := make(http.Header)
	header.Add("Authorization", u.Options.APIToken)

	// Establish a websocket connection to the portier server
	connection, resp, err := dialer.Dial(u.Options.PortierURL, header)
	if err != nil {
		if u.retries < u.Options.ReconnectRetries || u.Options.ReconnectRetries == 0 {
			u.retries++
		} else {
			fmt.Println(resp)
			return fmt.Errorf("maximum number of retries reached after: %v", err)
		}
		time.Sleep(u.calculateBackoff())
		return u.connectWebsocket()
	}
	u.events <- relay.UplinkEvent{
		State: relay.UplinkStateConnected,
		Event: "websocket connected",
	}

	u.retries = 0

	// receive messages from the portier server and forward them to the recv channel
	go func() {
		for {
			_, message, err := connection.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) {
					log.Printf("websocket disconnected: %v", err)
					u.events <- relay.UplinkEvent{
						State: relay.UplinkStateDisconnected,
						Event: "websocket disconnected",
					}
					time.Sleep(u.calculateBackoff())
					u.connectWebsocket()
				}
				return
			}
			u.recv <- message
		}
	}()

	u.connection = connection
	return nil
}

func (u *WebsocketUplink) calculateBackoff() time.Duration {
	if u.retries == 0 {
		return 50 * time.Millisecond
	}
	// Calculate the exponential backoff and max it with the maximum reconnect interval
	backoff := time.Duration(math.Pow(2, float64(u.retries))) * 50 * time.Millisecond
	if backoff > u.Options.MaxReconnectInterval {
		backoff = u.Options.MaxReconnectInterval
	}
	return backoff
}
