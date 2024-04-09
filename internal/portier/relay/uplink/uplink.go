package uplink

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
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

	// mutex is the mutex to lock the connection
	mutex sync.Mutex

	// events is the channel to receive events from the uplink
	events chan Event

	// encoderdecoder is the encoder / decoder for the uplink
	encoderDecoder encoder.EncoderDecoder
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
	u.mutex.Lock()
	defer u.mutex.Unlock()
	payload, err := u.encoderDecoder.Encode(message)
	if err != nil {
		return err
	}
	err = u.connection.WriteMessage(websocket.BinaryMessage, payload)
	if err != nil {
		u.events <- Event{
			State: Disconnected,
			Event: "send - websocket disconnected",
		}
		log.Printf("send - websocket disconnected: %v", err)
		u.connection.Close()
		return err
	}
	return nil
}

// Close closes the uplink, the connection to the portier server and expects the uplink to close the recv channel.
func (u *WebsocketUplink) Close() error {
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
	log.Println("Connecting to portier server: ", u.Options.PortierURL)

	// Establish a websocket connection to the portier server
	connection, resp, err := dialer.Dial(u.Options.PortierURL, header)
	if err != nil {
		fmt.Println("Error connecting to portier server: ", err)
		if u.retries < u.Options.ReconnectRetries || u.Options.ReconnectRetries == 0 {
			u.retries++
		} else {
			log.Println(resp)
			return fmt.Errorf("maximum number of retries reached after: %v", err)
		}
		time.Sleep(u.calculateBackoff())
		return u.connectWebsocket()
	}
	log.Println("Connected to portier server: ", u.Options.PortierURL)
	u.events <- Event{
		State: Connected,
		Event: "websocket connected",
	}

	u.retries = 0

	// setup ping
	connection.SetPingHandler(func(appData string) error {
		u.mutex.Lock()
		defer u.mutex.Unlock()
		err := connection.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
		if err != nil {
			log.Printf("sending pong error - websocket: %v", err)
			connection.Close()
			return err
		}
		connection.SetReadDeadline(time.Now().Add(10 * time.Second))
		return nil
	})

	// receive messages from the portier server and forward them to the recv channel
	go func() {
		for {
			_, frame, err := connection.ReadMessage()
			if err != nil {
				log.Printf("read - websocket: %v", err)
				u.connection.Close()
				u.events <- Event{
					State: Disconnected,
					Event: "read - websocket closed after error",
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
				log.Printf("error decoding message: %v", err)
				continue
			}
			select {
			case u.recv <- message:
			default:
				log.Println("uplink recv channel full, dropping message")
			}
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
