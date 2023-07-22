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

	// state is the state of the uplink
	state relay.UplinkState

	// retries is the number of retries to reconnect to the portier server
	retries int64

	// conn is the websocket connection to the portier server
	connection *websocket.Conn

	// recv is the channel to receive messages from the portier server
	recv chan []byte

	// mutex is the mutex to lock the connection
	mutex sync.Mutex
}

func defaultOptions() Options {
	return Options{
		MaxReconnectInterval: 10 * time.Second,
		ReconnectRetries:     0,
	}
}

// NewWebsocketUplink creates a new websocket uplink
func NewWebsocketUplink(options Options) *WebsocketUplink {
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

	return &WebsocketUplink{
		Options: options,
		state:   relay.UplinkStateDisconnected,
		recv:    make(chan []byte, 1),
	}
}

// Connect connects to the portier server return recv channel and a send channel to receive / send messages from /to the portier server.
func (u *WebsocketUplink) Connect() (<-chan []byte, error) {
	// Connect to the portier server
	err := u.connectWebsocket()
	if err != nil {
		return nil, err
	}
	u.state = relay.UplinkStateConnected
	return u.recv, nil
}

// Send enqueues a message to the portier server.
func (u *WebsocketUplink) Send(message []byte) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	err := u.connection.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		if websocket.IsUnexpectedCloseError(err) {
			log.Printf("websocket disconnected: %v", err)
			time.Sleep(u.calculateBackoff())
			u.connectWebsocket()
		}
	}
	return nil
}

// Close closes the uplink, the connection to the portier server and expects the uplink to close the recv channel
func (u *WebsocketUplink) Close() error {
	u.connection.Close()
	u.state = relay.UplinkStateDisconnected
	return nil
}

func (u *WebsocketUplink) connectWebsocket() error {
	// Create a header with the API token
	header := make(http.Header)
	header.Add("Authorization", u.Options.APIToken)

	// Establish a websocket connection to the portier server
	connection, resp, err := dialer.Dial(u.Options.PortierURL, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket connection failed: %v", resp.Status)
		}
		if u.retries < u.Options.ReconnectRetries || u.Options.ReconnectRetries == 0 {
			u.retries++
		} else {
			return fmt.Errorf("maximum number of retries reached after: %v", err)
		}
		time.Sleep(u.calculateBackoff())
		return u.connectWebsocket()
	}

	u.retries = 0

	// receive messages from the portier server and forward them to the recv channel
	go func() {
		for {
			_, message, err := connection.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) {
					log.Printf("websocket disconnected: %v", err)
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
