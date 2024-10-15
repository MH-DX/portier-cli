package uplink

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
)

// State is the state of the relay.
type State string

const (
	// UplinkStateDisconnected is the state when the uplink is disconnected.
	Disconnected State = "disconnected"

	// Connected is the state when the uplink is connected.
	Connected State = "connected"
)

type Event struct {
	// State is the state of the uplink
	State State
	// Event
	Event string
}

// Uplink is the uplink interface to the portier server. It is used to send messages to the portier server and to receive messages from the portier server.
// Moreover, it has to handle connection loss and reconnect to the portier server.
type Uplink interface {
	// Connect connects to the portier server return recv channel to receive messages from the portier server.
	// The channels will be to be closed by the uplink when the connection to the portier server is closed.
	// The recv channel will have no buffer and it is mandatory that the Router processes messages in a non-blocking way.
	Connect() (<-chan messages.Message, error)

	// Send enqueues a message to the portier server.
	// The Uplink has only a small buffer to realize backpressure in case the uplink cannot keep up with the messages, i.e. it will block.
	// This blocking must be effectively throttling the Service.
	Send(messages.Message) error

	// Close closes the uplink, the connection to the portier server and expects the uplink to close the recv channel
	Close() error

	// Returns a recv channel to listen for events
	Events() <-chan Event
}

// dialer is the websocket dialer.
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
	recv chan messages.Message

	// send is the channel to send messages to the portier server
	send chan []byte

	// events is the channel to receive events from the uplink
	events chan Event

	// encoderdecoder is the encoder / decoder for the uplink
	encoderDecoder encoder.EncoderDecoder

	// context is the context to close the uplink
	context context.Context

	// cancel is the cancel function to close the uplink
	cancel context.CancelFunc
}

func defaultOptions() Options {
	return Options{
		MaxReconnectInterval: 5 * time.Second,
		ReconnectRetries:     0,
	}
}

// NewWebsocketUplink creates a new websocket uplink.
func NewWebsocketUplink(options Options, encoderDecoder encoder.EncoderDecoder) *WebsocketUplink {
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
		encoderDecoder = encoder.NewEncoderDecoder()
	}

	return &WebsocketUplink{
		Options:        options,
		recv:           make(chan messages.Message, 1000),
		send:           make(chan []byte),
		events:         make(chan Event, 100),
		encoderDecoder: encoderDecoder,
	}
}

// Connect connects to the portier server return recv channel to receive messages from the portier server.
func (u *WebsocketUplink) Connect() (<-chan messages.Message, error) {
	// Connect to the portier server

	err := u.connectWebsocket()
	if err != nil {
		return nil, err
	}
	return u.recv, nil
}

// Send enqueues a message to the portier server.
func (u *WebsocketUplink) Send(message messages.Message) error {
	payload, err := u.encoderDecoder.Encode(message)
	if err != nil {
		return err
	}
	u.send <- payload
	return nil
}

// Close closes the uplink, the connection to the portier server and expects the uplink to close the recv channel.
func (u *WebsocketUplink) Close() error {
	if u.cancel == nil {
		u.cancel()
	}
	u.connection.Close()
	return nil
}

func (u *WebsocketUplink) Events() <-chan Event {
	return u.events
}

func (u *WebsocketUplink) connectWebsocket() error {
	// Create a header with the API token
	header := make(http.Header)
	header.Add("Authorization", u.Options.APIToken)
	u.events <- Event{
		State: Disconnected,
		Event: "connecting to portier server: " + u.Options.PortierURL,
	}

	// Establish a websocket connection to the portier server
	connection, _, err := dialer.Dial(u.Options.PortierURL, header)
	if err != nil {
		u.events <- Event{
			State: Disconnected,
			Event: "error connecting to portier server: " + err.Error(),
		}
		if u.retries < u.Options.ReconnectRetries || u.Options.ReconnectRetries == 0 {
			u.retries++
		} else {
			u.events <- Event{
				State: Disconnected,
				Event: "maximum number of retries reached",
			}
			return fmt.Errorf("maximum number of retries reached after: %v", err)
		}
		time.Sleep(u.calculateBackoff())
		return u.connectWebsocket()
	}
	u.events <- Event{
		State: Connected,
		Event: fmt.Sprintf("Connected to portier server: %s", u.Options.PortierURL),
	}

	u.retries = 0
	u.context, u.cancel = context.WithCancel(context.Background())

	// receive messages from the portier server and forward them to the recv channel
	go func() {
		for {
			select {
			case <-u.context.Done():
				return
			default:
			}

			_, frame, err := connection.ReadMessage()
			if err != nil {
				connection.Close()
				u.cancel()
				u.events <- Event{
					State: Disconnected,
					Event: fmt.Sprintf("read - websocket closed after error: %v\n", err),
				}

				time.Sleep(u.calculateBackoff())
				err = u.connectWebsocket()
				if err != nil {
					panic("error reconnecting to portier server: " + err.Error())
				}
				return
			}
			message, err := u.encoderDecoder.Decode(frame)
			if err != nil {
				u.events <- Event{
					State: Connected,
					Event: fmt.Sprintf("error decoding message: %v", err),
				}
				continue
			}
			select {
			case u.recv <- message:
			default:
				u.events <- Event{
					State: Connected,
					Event: "recv channel full, dropping message",
				}
			}
		}
	}()

	mutex := &sync.Mutex{}

	// send messages to the portier server
	go func() {
		for {
			select {
			case payload := <-u.send:
				mutex.Lock()
				connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = connection.WriteMessage(websocket.BinaryMessage, payload)
				if err != nil {
					u.events <- Event{
						State: Disconnected,
						Event: fmt.Sprintf("send - websocket error: %v", err),
					}
					mutex.Unlock()
					return
				}
				mutex.Unlock()
			case <-u.context.Done():
				return
			}
		}
	}()

	// setup ping
	connection.SetPingHandler(func(appData string) error {
		mutex.Lock()
		defer mutex.Unlock()
		err := connection.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
		if err != nil {
			u.events <- Event{
				State: Disconnected,
				Event: fmt.Sprintf("ping - websocket error: %v", err),
			}
			return err
		}
		connection.SetReadDeadline(time.Now().Add(10 * time.Second))
		return nil
	})

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
