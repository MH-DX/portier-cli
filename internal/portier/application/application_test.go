package application

import (
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
	localURL, _ := url.Parse("tcp://localhost:21113")
	remoteURL, _ := url.Parse("tcp://localhost:21114")
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

	// WHEN
	appLocal.StartServices()
	appRemote.StartServices()

	// THEN
	// connect to local URL
	localConn, _ := net.Dial("tcp", localURL.Host)
	// accept connection
	remoteConn, err := remoteListener.Accept()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create message with 1000 bytes
	msg := make([]byte, 10000)

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

func readUntil(remoteConn net.Conn, length int) (int, error) {
	buf := make([]byte, 100000)
	total := 0
	// read until EOF
	for {
		n, err := remoteConn.Read(buf[total:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, err
		}
		total += n
		if total == length {
			break
		}
	}
	return total, nil
}
