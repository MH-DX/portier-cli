package application

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/portier/config"
	"github.com/marinator86/portier-cli/internal/portier/relay"
	"github.com/marinator86/portier-cli/internal/utils"
	"gopkg.in/yaml.v2"
)

func TestApplicationStartupAndForwarding(t *testing.T) {
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

	localServices := []relay.Service{
		{
			Name: "service1",
			Options: relay.ServiceOptions{
				URLLocal:     utils.YAMLURL{URL: localURL},
				URLRemote:    utils.YAMLURL{URL: remoteURL},
				PeerDeviceID: peer,
				TLSDisabled:  false,
			},
		},
	}
	appLocal := createApp(ws_url, local, localServices)
	appRemote := createApp(ws_url, peer, []relay.Service{})
	// listen at remote URL
	remoteListener, _ := net.Listen("tcp", remoteURL.Host)
	defer remoteListener.Close()

	// WHEN
	appLocal.StartServices()
	appRemote.StartServices()

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

func createApp(ws_url string, deviceID uuid.UUID, services []relay.Service) *PortierApplication {
	configPath := "config.yaml"
	credentialsPath := "credentials.yaml"
	defer func() {
		// delete config.yaml and credentials.yaml
		os.Remove(configPath)
		os.Remove(credentialsPath)
	}()

	// create config
	portierConfig := defaultPortierConfig()
	portierURL, _ := url.Parse(ws_url)

	portierConfig.PortierURL.URL = portierURL
	portierConfig.Services = services

	// write config to file
	configFile, _ := os.Create(configPath)
	encoder := yaml.NewEncoder(configFile)
	encoder.Encode(portierConfig)

	credentials := &config.DeviceCredentials{
		DeviceID: deviceID,
		ApiToken: deviceID.String(),
	}
	// write credentials to file
	credentialsFile, _ := os.Create(credentialsPath)
	encoder = yaml.NewEncoder(credentialsFile)
	encoder.Encode(credentials)

	app := NewPortierApplication()

	app.LoadConfig(configPath)
	app.LoadApiToken(credentialsPath)

	return app
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
