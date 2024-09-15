package ptls

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

type PTLSCertificateManager interface {

	// CreateCertificate creates a new TLS certificate
	CreateCertificate(commonName string) (*x509.Certificate, crypto.PrivateKey, error)

	// ConvertCertificateToPEM converts the certificate to PEM format
	ConvertCertificateToPEM(cert *x509.Certificate, privateKey crypto.PrivateKey) (certPEM []byte, keyPEM []byte, err error)

	// GetFingerprint returns the fingerprint of the certificate in ASN.1 format (DER)
	GetFingerprint(cert *x509.Certificate) (fingerprint string, err error)
}

type ptlsCertMan struct {
}

func NewPTLSCertificateManager() PTLSCertificateManager {
	return &ptlsCertMan{}
}

func (p *ptlsCertMan) CreateCertificate(commonName string) (*x509.Certificate, crypto.PrivateKey, error) {
	// from https://golang.org/src/crypto/tls/generate_cert.go
	// from https://gist.github.com/rorycl/d300f3ab942fd79e6cc1f37db0c6260f
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(20, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}
	x509Cert, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)

	if err != nil {
		return nil, nil, err
	}

	parsedCert, err := x509.ParseCertificate(x509Cert)
	if err != nil {
		return nil, nil, err
	}

	return parsedCert, privKey, nil
}

func (p *ptlsCertMan) ConvertCertificateToPEM(cert *x509.Certificate, privateKey crypto.PrivateKey) (certPEM []byte, keyPEM []byte, err error) {
	// convert the certificate to PEM format
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if certPEM == nil {
		return nil, nil, err
	}

	// convert the private key to PEM format
	priv, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY",
		Bytes: priv})
	if keyPEM == nil {
		return nil, nil, err
	}

	return certPEM, keyPEM, nil
}

func (p *ptlsCertMan) GetFingerprint(cert *x509.Certificate) (fingerprint string, err error) {
	fp := sha256.Sum256(cert.Raw)
	return fmt.Sprintf("%x", fp), nil
}

func (p *ptlsCertMan) UploadFingerprint(ctx context.Context, fingerprint string) error {
	return nil
}
