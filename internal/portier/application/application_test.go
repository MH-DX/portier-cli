package application

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/utils"
)

func TestApplicationStartupAndForwardingWithTLS(t *testing.T) {
	// GIVEN
	server := httptest.NewServer(http.HandlerFunc(utils.EchoWithLoss(5)))

	local, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	peer, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")

	// get free port
	localURL, _ := url.Parse("tcp://localhost:" + fmt.Sprintf("%d", GetFreePort()))
	remoteURL, _ := url.Parse("tcp://localhost:" + fmt.Sprintf("%d", GetFreePort()))
	defer server.Close()
	// Replace "http" with "ws" in our URL.
	ws_url := "ws" + server.URL[4:]

	localServices := []config.Service{
		{
			Name: "service1",
			Options: config.ServiceOptions{
				URLLocal:     utils.YAMLURL{URL: localURL},
				URLRemote:    utils.YAMLURL{URL: remoteURL},
				PeerDeviceID: peer,
				TLSEnabled:   true,
			},
		},
	}
	configLocal, credsLocal := createConfigs(ws_url, local, localServices, "local")
	configPeer, credsPeer := createConfigs(ws_url, peer, []config.Service{}, "peer")
	appLocal := NewPortierApplication()
	appRemote := NewPortierApplication()
	// listen at remote URL
	remoteListener, _ := net.Listen("tcp", remoteURL.Host)
	defer remoteListener.Close()

	// WHEN
	appLocal.StartServices(configLocal, credsLocal)
	appRemote.StartServices(configPeer, credsPeer)

	// THEN
	// connect to local URL
	localConn, _ := net.Dial("tcp", localURL.Host)
	defer localConn.Close()
	// accept connection
	remoteConn, err := remoteListener.Accept()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	defer remoteConn.Close()

	// create message with X bytes
	msg := make([]byte, 100000)

	n, err := localConn.Write(msg)
	if err != nil {
		t.Errorf("error writing to connection: %v\n", err)
	}
	if n != len(msg) {
		t.Errorf("expected %d bytes, got %d", len(msg), n)
	}

	// read message from remote connection
	total, err := readUntil(remoteConn, len(msg))
	if err != nil {
		t.Errorf("error reading from connection: %v\n", err)
	}

	// assert that remote connection received the message
	if total != len(msg) {
		t.Errorf("expected %d bytes, got %d", len(msg), total)
	}
}

type fakeConnectionAdapter struct {
	closeCalls chan struct{}
}

func (f *fakeConnectionAdapter) Start() error { return nil }

func (f *fakeConnectionAdapter) Close() error {
	select {
	case f.closeCalls <- struct{}{}:
	default:
	}
	return nil
}

func (f *fakeConnectionAdapter) Send(messages.Message) {}

func TestStartTLSHandshakeDoesNotBlockAcceptLoop(t *testing.T) {
	app := NewPortierApplication()
	connectionAdapter := &fakeConnectionAdapter{closeCalls: make(chan struct{}, 1)}
	handshakeStarted := make(chan struct{}, 1)
	releaseHandshake := make(chan struct{})

	done := make(chan struct{})
	go func() {
		app.startTLSHandshake(connectionAdapter, func() error {
			handshakeStarted <- struct{}{}
			<-releaseHandshake
			return errors.New("handshake failed")
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected startTLSHandshake to return immediately")
	}

	select {
	case <-handshakeStarted:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected handshake goroutine to start")
	}

	close(releaseHandshake)

	select {
	case <-connectionAdapter.closeCalls:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected handshake failure to close the adapter")
	}
}

func createConfigs(ws_url string, deviceID uuid.UUID, services []config.Service, suffix string) (*config.PortierConfig, *config.DeviceCredentials) {
	portierConfig, err := config.DefaultPortierConfig()
	if err != nil {
		panic(err)
	}
	portierURL, _ := url.Parse(ws_url)
	normalizedBaseURL := endpoints.NormalizeBaseURL(ws_url)
	baseURL, _ := url.Parse(normalizedBaseURL)

	portierConfig.TLSEnabled = true
	portierConfig.PTLSConfig = config.PTLSConfig{
		CertFile:       fmt.Sprintf("testdata/cert%s.pem", suffix),
		KeyFile:        fmt.Sprintf("testdata/key%s.pem", suffix),
		KnownHostsFile: fmt.Sprintf("testdata/known_hosts.%s", suffix),
	}

	portierConfig.BaseURL = utils.YAMLURL{URL: baseURL}
	portierConfig.RelayPath = portierURL.Path
	if portierConfig.RelayPath == "" {
		portierConfig.RelayPath = "/"
	}
	portierConfig.Services = services

	credentials := &config.DeviceCredentials{
		DeviceID: deviceID,
		ApiToken: deviceID.String(),
	}

	return portierConfig, credentials
}

func readUntil(conn net.Conn, totalBytes int) (int, error) {
	totalRead := 0
	buf := make([]byte, 100000)
	for totalRead < totalBytes {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Reached EOF after reading %d bytes\n", totalRead)
				break
			}
			return totalRead, err
		}
		totalRead += n
		fmt.Printf("Read %d bytes, total read: %d\n", n, totalRead)
	}
	return totalRead, nil
}

func GetFreePort() int {
	if a, err := net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port
		}
	}
	panic("no free ports")
}
