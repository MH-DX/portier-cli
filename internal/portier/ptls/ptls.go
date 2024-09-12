package ptls

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/google/uuid"
	"github.com/marinator86/portier-cli/internal/utils"
	"gopkg.in/yaml.v2"
)

type PTLSConfig struct {
	// PeerDeviceID is the deviceID of the peer device.
	// Required
	PeerDeviceID uuid.UUID

	// cert file path, containing this device's certificate
	// default: "{home}/cert.pem"
	CertFile func() ([]byte, error)

	// key file path, containing this device's private key
	// default: "{home}/key.pem"
	KeyFile func() ([]byte, error)

	// CA cert file path, to verify the peer's certificate
	// If set, server and client will verify the peer's certificate.
	// If not set, server and client will try the KnownHosts file to verify the peer's certificate.
	// default: not set
	CAFile func() ([]byte, error)

	// KnownHosts is a local file containing a map of known certificate fingerprints to deviceID, for
	// verifying the peer's certificate. Only used if CAFile is not set.
	// default: {home}/known_hosts
	KnownHostsFile func() ([]byte, error)
}

func defaultPTLSConfig() (*PTLSConfig, error) {
	home, err := utils.Home()
	if err != nil {
		return nil, err
	}

	result := PTLSConfig{
		CertFile:       FileProvider(fmt.Sprintf("%s/cert.pem", home)),
		KeyFile:        FileProvider(fmt.Sprintf("%s/key.pem", home)),
		CAFile:         nil,
		KnownHostsFile: FileProvider(fmt.Sprintf("%s/known_hosts", home)),
	}

	return &result, nil
}

func FileProvider(file string) func() ([]byte, error) {
	return func() ([]byte, error) {
		file, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}

// CreateClientAndBridge creates a new TLS client and decorates the connection with it.
// Returns a new connection and a TLS handshaker function.
func CreateClientAndBridge(conn net.Conn, config PTLSConfig) (net.Conn, func() error, error) {

	conn1, conn2 := net.Pipe()

	completedConfig, err := completeConfig(config)
	if err != nil {
		return nil, nil, err
	}

	conn1, handshaker, err := decorateTLSClient(conn1, completedConfig)
	if err != nil {
		return nil, nil, err
	}

	// io.Copy from conn to conn1 and vice versa
	go func() {
		io.Copy(conn1, conn)
		conn1.Close()
	}()
	go func() {
		io.Copy(conn, conn1)
		conn.Close()
	}()

	return conn2, handshaker, nil
}

func completeConfig(config PTLSConfig) (PTLSConfig, error) {
	defaultPTLSConfig, err := defaultPTLSConfig()
	if err != nil {
		return config, err
	}

	if config.PeerDeviceID == uuid.Nil {
		return config, fmt.Errorf("PeerDeviceID is required")
	}

	if config.CertFile == nil {
		config.CertFile = defaultPTLSConfig.CertFile
	}

	if config.KeyFile == nil {
		config.KeyFile = defaultPTLSConfig.KeyFile
	}

	if config.KnownHostsFile == nil {
		config.KnownHostsFile = defaultPTLSConfig.KnownHostsFile
	}

	return config, nil
}

func decorateTLSClient(conn net.Conn, config PTLSConfig) (net.Conn, func() error, error) {

	// create a new TLS client

	// load the client's certificate and private key
	cert, err := config.CertFile()
	if err != nil {
		return nil, nil, err
	}
	key, err := config.KeyFile()
	if err != nil {
		return nil, nil, err
	}

	// create a new TLS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{cert},
				PrivateKey:  key,
			},
		},
	}

	if config.CAFile != nil {
		tlsConfig.InsecureSkipVerify = false
		tlsConfig.ServerName = config.PeerDeviceID.String()

		// load the CA certificate
		caCert, err := config.CAFile()
		if err != nil {
			return nil, nil, err
		}

		// add the CA certificate to the TLS client
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	} else {
		tlsConfig.InsecureSkipVerify = true

		// load the known hosts file
		knownHosts, err := config.KnownHostsFile()
		if err != nil {
			return nil, nil, err
		}

		// add the known hosts to the TLS client
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			peerCert := verifiedChains[0][0]
			peerDeviceID := peerCert.Subject.CommonName
			peerCertFingerprint := sha256.Sum256(peerCert.Raw)
			if peerDeviceID != config.PeerDeviceID.String() {
				return fmt.Errorf("common name %s does not match expected peer device %s", peerDeviceID, config.PeerDeviceID)
			}

			knownHostsMap := make(map[string][]byte)
			err := yaml.Unmarshal(knownHosts, knownHostsMap)
			if err != nil {
				return err
			}

			if knownHostsMap[peerDeviceID] == nil {
				return fmt.Errorf("unknown peer device: %s", peerDeviceID)
			}

			if !bytes.Equal(knownHostsMap[peerDeviceID], peerCertFingerprint[:]) {
				return fmt.Errorf("peer device %s has an unknown certificate", peerDeviceID)
			}

			return nil
		}
	}

	// create a new TLS client
	tlsConn := tls.Client(conn, tlsConfig)

	// create the TLS handshaker
	handshaker := func() error {
		err = tlsConn.Handshake()
		if err != nil {
			return err
		}
		return nil
	}

	return tlsConn, handshaker, nil
}
