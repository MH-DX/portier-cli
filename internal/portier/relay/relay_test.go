package relay

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/marinator86/portier-cli/internal/portier/relay/adapter"
	"github.com/marinator86/portier-cli/internal/portier/relay/controller"
	"github.com/marinator86/portier-cli/internal/portier/relay/encoder"
	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
	"github.com/marinator86/portier-cli/internal/portier/relay/router"
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

func echoWithLoss(n int) func(w http.ResponseWriter, r *http.Request) {
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

func createUplink(deviceId string, url string) uplink.Uplink {
	options := uplink.Options{
		APIToken:   deviceId,
		PortierURL: url,
	}

	return uplink.NewWebsocketUplink(options, nil)
}

func TestConnectAndBridging(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echoWithLoss(0)))

	device1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	device2, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	url := "ws" + server.URL[4:]

	uplink1 := createUplink(device1.String(), url)
	_, _ = uplink1.Connect()
	downlink2, _ := createUplink(device2.String(), url).Connect()

	// WHEN
	msg := messages.Message{
		Header: messages.MessageHeader{
			From: device1,
			To:   device2,
		},
		Message: []byte("Hello, world!"),
	}
	_ = uplink1.Send(msg) // send message to the uplink

	// THEN
	response := <-downlink2
	if response.Header != msg.Header {
		testing.Errorf("expected %v, got %v", msg, response)
	}
}

func TestForwarding(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echoWithLoss(0)))

	device1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	device2, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")
	cid := messages.ConnectionID("test-connection-id")
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

	ctrl2, router2 := createInboundRelay(device2, ws_url, inboundEvents)
	_ = router2.Start()
	_ = ctrl2.Start()

	ctrl1, router1, uplink := createOutboundRelay(device1, ws_url, outboundEvents)
	adapter1, listenerConn := createOutboundAdapter(uplink, fromOptions, outboundEvents, ln)
	_ = router1.Start()
	_ = ctrl1.Start()
	_ = ctrl1.AddConnection(cid, adapter1)

	// Starting the outbound adapter will initiate sending the connection open message
	err := adapter1.Start()
	if err != nil {
		testing.Errorf("error starting adapter: %v", err)
	}

	// WHEN
	// wait for the forwardED listener to accept a connection from the inbound adapter
	forwardedConn, _ := forwarded.Accept()
	// then write to the forwardING connection
	_, _ = listenerConn.Write([]byte("Hello, world!"))

	// THEN
	// wait till the forwarded connection receives the message, i.e. the inbound adapter has forwarded the message
	buf := make([]byte, 1024)
	n, _ := forwardedConn.Read(buf)
	if string(buf[:n]) != "Hello, world!" {
		testing.Errorf("expected %v, got %v", "Hello, world!", string(buf[:n]))
	}

	// close the forwarded connection to provoke a connection close message
	listenerConn.Close()

	// expect the listener connection to be closed
	buf = make([]byte, 1024)
	_, err = forwardedConn.Read(buf)
	if err.Error() != "EOF" {
		testing.Errorf("expected %v, got %v", "EOF", err.Error())
	}
}

func TestForwardingLarge(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echoWithLoss(1204)))

	device1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	device2, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")
	cid := messages.ConnectionID("test-connection-id")
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

	ctrl2, router2 := createInboundRelay(device2, ws_url, inboundEvents)
	_ = router2.Start()
	_ = ctrl2.Start()

	ctrl1, router1, uplink := createOutboundRelay(device1, ws_url, outboundEvents)
	adapter1, listenerConn := createOutboundAdapter(uplink, fromOptions, outboundEvents, ln)
	_ = router1.Start()
	_ = ctrl1.Start()
	_ = ctrl1.AddConnection(cid, adapter1)

	// Starting the outbound adapter will initiate sending the connection open message
	err := adapter1.Start()
	if err != nil {
		testing.Errorf("error starting adapter: %v", err)
	}

	size := 1024 * 1024 * 100

	// WHEN
	forwardedConn, _ := forwarded.Accept()

	msg := make([]byte, size)
	_, _ = rand.Read(msg)

	startingTime := time.Now()

	go func() {
		_ = listenerConn.SetWriteDeadline(time.Time{})
		n, err := listenerConn.Write(msg)
		if err != nil {
			testing.Errorf("error writing to listener connection: %v", err)
		}
		if n != len(msg) {
			testing.Errorf("expected %v, got %v", len(msg), n)
		}
	}()

	// THEN
	buf := make([]byte, size)

	// set read deadline to 5 seconds
	totalBytesRead := 0
	for {
		// read from the forwarded connection and append to buf, repeat until EOF
		currentBuf := make([]byte, 100000)
		_ = forwardedConn.SetReadDeadline(time.Time{})
		n, err := forwardedConn.Read(currentBuf)
		if err != nil {
			break
		}
		if n == 0 {
			continue
		}
		copy(buf[totalBytesRead:], currentBuf[:n])
		totalBytesRead += n
		if totalBytesRead == len(msg) {
			since := time.Since(startingTime)
			fmt.Printf("Read %v bytes in %v, speed %f MB/s\n", totalBytesRead, since, float64(totalBytesRead)/(1024*1024*float64(since.Seconds())))
			break
		}
	}

	// compare the received message with the sent message
	if string(buf[:totalBytesRead]) != string(msg) {
		testing.Errorf("expected %v, got %v", string(msg), string(buf[:totalBytesRead]))
	}

	// close the forwarded connection to provoke a connection close message
	listenerConn.Close()

	if totalBytesRead != len(msg) {
		testing.Errorf("expected %v, got %v", len(msg), totalBytesRead)
	}
}

func TestConnOpenUnderStress(testing *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(echoWithLoss(5)))

	device1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	device2, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	ws_url := "ws" + server.URL[4:]

	// create forwarding tcp server
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	inboundEvents := make(chan adapter.AdapterEvent)
	outboundEvents := make(chan adapter.AdapterEvent)

	forwarded, _ := net.Listen("tcp", "127.0.0.1:0")
	fAddr := fmt.Sprintf("%s://%s", forwarded.Addr().Network(), forwarded.Addr().String())
	defer forwarded.Close()

	ctrl2, router2 := createInboundRelay(device2, ws_url, inboundEvents)
	_ = router2.Start()
	_ = ctrl2.Start()

	ctrl1, router1, uplink := createOutboundRelay(device1, ws_url, outboundEvents)
	_ = router1.Start()
	_ = ctrl1.Start()

	for i := 0; i < 50; i++ {
		cid := messages.ConnectionID(fmt.Sprintf("test-connection-id-%d", i))
		fromOptions := createConnectionAdapterOptions(cid, device1, device2, fAddr)
		adapter1, listenerConn := createOutboundAdapter(uplink, fromOptions, outboundEvents, ln)
		_ = ctrl1.AddConnection(cid, adapter1)

		// Starting the outbound adapter will initiate sending the connection open message
		err := adapter1.Start()
		if err != nil {
			testing.Errorf("error starting adapter: %v", err)
		}

		// WHEN
		// wait for the forwardED listener to accept a connection from the inbound adapter
		_, _ = forwarded.Accept()

		// close the forwarded connection to provoke a connection close message
		_ = listenerConn.Close()
	}
}

func createOutboundRelay(deviceId uuid.UUID, url string, events chan adapter.AdapterEvent) (controller.Controller, router.Router, uplink.Uplink) {
	uplink := createUplink(deviceId.String(), url)
	messageChannel, _ := uplink.Connect()
	routerEventChannel := make(chan router.ConnectionOpenEvent)
	router := router.NewRouter(uplink, messageChannel, routerEventChannel)
	controller := controller.NewController(uplink, events, routerEventChannel, router)

	return controller, router, uplink
}

func createOutboundAdapter(uplink uplink.Uplink, opts adapter.ConnectionAdapterOptions, events chan adapter.AdapterEvent, ln net.Listener) (adapter.ConnectionAdapter, net.Conn) {
	// dial tcp server
	cConn, _ := net.Dial(ln.Addr().Network(), ln.Addr().String())
	sConn, _ := ln.Accept()

	adapter := adapter.NewOutboundConnectionAdapter(opts, sConn, uplink, events)
	return adapter, cConn
}

func createInboundRelay(deviceId uuid.UUID, url string, events chan adapter.AdapterEvent) (controller.Controller, router.Router) {
	uplink := createUplink(deviceId.String(), url)
	messageChannel, _ := uplink.Connect()
	routerEventChannel := make(chan router.ConnectionOpenEvent, 100)
	router := router.NewRouter(uplink, messageChannel, routerEventChannel)
	controller := controller.NewController(uplink, events, routerEventChannel, router)
	return controller, router
}

func createConnectionAdapterOptions(cid messages.ConnectionID, from uuid.UUID, to uuid.UUID, rawUrl string) adapter.ConnectionAdapterOptions {
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
