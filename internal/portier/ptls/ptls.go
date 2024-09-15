package ptls

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v2"
)

type PTLS interface {
	TestEndpointURL(endpoint url.URL) bool
	CreateClientAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, func() error, error)
	CreateServerAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, error)
}

type ptls struct {

	// enabled indicates whether PTLS is enabled
	Enabled bool

	// The path to the certificate file
	CertFile string

	// The path to the key file
	KeyFile string

	// The path to the CA file
	CAFile string

	// The path to the known hosts file
	KnownHostsFile string

	// Repository is the repository
	Repo func(string) ([]byte, error)
}

type FileLoader func(string) ([]byte, error)

// NewPTLS creates a new PTLS instance
func NewPTLS(enabled bool, certFile, keyFile, caFile, knownHostsFile string, repo FileLoader) PTLS {

	if repo == nil {
		repo = loadFile
	}

	return &ptls{
		Enabled:        enabled,
		CertFile:       certFile,
		KeyFile:        keyFile,
		CAFile:         caFile,
		KnownHostsFile: knownHostsFile,
		Repo:           repo,
	}
}

func (p *ptls) TestEndpointURL(endpoint url.URL) bool {
	// look at config and determine from the endpoint configs if this endpoint must be secured with TLS
	// also determine if TLS is globally enabled

	// TODO: implement this function
	return p.Enabled
}

// CreateClientAndBridge creates a new TLS client and decorates the connection with it.
// Returns a new connection and a TLS handshaker function.
func (p *ptls) CreateClientAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, func() error, error) {
	return p.decorateAndBridge(conn, peerDeviceID, p.decorateTLSClient)
}

// CreateServerAndBridge creates a new TLS server and decorates the connection with it.
// Returns a new connection.
func (p *ptls) CreateServerAndBridge(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, error) {
	conn, _, err := p.decorateAndBridge(conn, peerDeviceID, p.decorateTLSServer)
	return conn, err
}

func (p *ptls) decorateAndBridge(conn net.Conn, peerDeviceID uuid.UUID, decorator func(net.Conn, uuid.UUID) (net.Conn, func() error, error)) (net.Conn, func() error, error) {

	conn1, conn2 := net.Pipe()

	conn1, handshaker, err := decorator(conn1, peerDeviceID)
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

func (p *ptls) decorateTLSClient(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, func() error, error) {

	// create a new TLS client

	// load the client's certificate and private key
	cert, err := p.Repo(p.CertFile)
	if err != nil {
		return nil, nil, err
	}
	key, err := p.Repo(p.KeyFile)
	if err != nil {
		return nil, nil, err
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, nil, err
	}

	// create a new TLS client
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{tlsCert},
	}

	cacert, err := p.Repo(p.CAFile)

	if err == nil {
		tlsConfig.InsecureSkipVerify = false
		tlsConfig.ServerName = peerDeviceID.String()

		// add the CA certificate to the TLS client
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cacert)
		tlsConfig.RootCAs = caCertPool
	} else {
		tlsConfig.InsecureSkipVerify = true

		// load the known hosts file
		knownHosts, err := p.Repo(p.KnownHostsFile)
		if err != nil {
			return nil, nil, err
		}

		// add the known hosts to the TLS client
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			peerCertRaw := rawCerts[0]
			peerCert, err := x509.ParseCertificate(peerCertRaw)
			if err != nil {
				return err
			}
			cName := peerCert.Subject.CommonName
			peerCertFingerprint := fmt.Sprintf("%x", sha256.Sum256(peerCert.Raw))
			if cName != peerDeviceID.String() {
				return fmt.Errorf("common name %s does not match expected peer device %s", cName, peerDeviceID)
			}

			knownHostsMap := make(map[string]string)
			err = yaml.Unmarshal(knownHosts, knownHostsMap)
			if err != nil {
				return err
			}

			if knownHostsMap[cName] == "" {
				return fmt.Errorf("unknown peer device: %s", peerDeviceID)
			}

			if knownHostsMap[cName] != peerCertFingerprint {
				return fmt.Errorf("peer device %s has an unknown certificate", peerDeviceID)
			}

			return nil
		}
	}

	// create a new TLS client
	tlsConn := tls.Client(conn, tlsConfig)

	// create the TLS handshaker
	handshaker := func() error {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*3600)
		err = tlsConn.HandshakeContext(ctx)
		if err != nil {
			return err
		}
		return nil
	}

	return tlsConn, handshaker, nil
}

func (p *ptls) decorateTLSServer(conn net.Conn, peerDeviceID uuid.UUID) (net.Conn, func() error, error) {

	// create a new TLS server

	// load the server's certificate and private key
	cert, err := p.Repo(p.CertFile)
	if err != nil {
		return nil, nil, err
	}
	key, err := p.Repo(p.KeyFile)
	if err != nil {
		return nil, nil, err
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, nil, err
	}

	// create a new TLS server
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{tlsCert},
	}

	cacert, err := p.Repo(p.CAFile)

	if err == nil {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

		// add the CA certificate to the TLS server
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cacert)
		tlsConfig.ClientCAs = caCertPool
	} else {
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
		tlsConfig.InsecureSkipVerify = true

		// load the known hosts file
		knownHosts, err := p.Repo(p.KnownHostsFile)
		if err != nil {
			return nil, nil, err
		}

		// add the known hosts to the TLS server
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			peerCertRaw := rawCerts[0]
			peerCert, err := x509.ParseCertificate(peerCertRaw)
			if err != nil {
				return err
			}
			cName := peerCert.Subject.CommonName
			peerCertFingerprint := fmt.Sprintf("%x", sha256.Sum256(peerCert.Raw))
			if cName != peerDeviceID.String() {
				return fmt.Errorf("common name %s does not match expected peer device %s", cName, peerDeviceID)
			}

			knownHostsMap := make(map[string]string)
			err = yaml.Unmarshal(knownHosts, knownHostsMap)
			if err != nil {
				return err
			}

			if knownHostsMap[cName] == "" {
				return fmt.Errorf("unknown peer device: %s", peerDeviceID)
			}

			if knownHostsMap[cName] != peerCertFingerprint {
				return fmt.Errorf("peer device %s has an unknown certificate", peerDeviceID)
			}

			return nil
		}
	}

	// create a new TLS server
	tlsConn := tls.Server(conn, tlsConfig)

	return tlsConn, nil, nil
}

func loadFile(path string) ([]byte, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}
