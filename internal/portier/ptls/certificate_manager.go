package ptls

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
)

type PTLSCertificateManager interface {

	// CreateCertificate creates a new TLS certificate
	CreateCertificate(commonName string) (*tls.Certificate, error)

	// ConvertCertificateToPEM converts the certificate to PEM format
	ConvertCertificateToPEM(cert *tls.Certificate) (certPEM []byte, keyPEM []byte, err error)

	// GetFingerprint returns the fingerprint of the certificate in ASN.1 format (DER)
	GetFingerprint(cert *tls.Certificate) (fingerprint string, err error)

	// UploadFingerprint uploads the fingerprint of the certificate to the portier server
	UploadFingerprint(ctx context.Context, fingerprint string) error
}

type ptlsCertMan struct {
}

func NewPTLSCertificateManager() PTLSCertificateManager {
	return &ptlsCertMan{}
}

func (p *ptlsCertMan) CreateCertificate(commonName string) (*tls.Certificate, error) {
	// from https://golang.org/src/crypto/tls/generate_cert.go
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}
	x509Cert, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)

	if err != nil {
		return nil, err
	}
	return &tls.Certificate{
		Certificate: [][]byte{x509Cert},
		PrivateKey:  privKey,
	}, nil
}

func (p *ptlsCertMan) ConvertCertificateToPEM(cert *tls.Certificate) (certPEM []byte, keyPEM []byte, err error) {
	// convert the certificate to PEM format
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	if certPEM == nil {
		return nil, nil, err
	}

	// convert the private key to PEM format
	priv, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
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

func (p *ptlsCertMan) GetFingerprint(cert *tls.Certificate) (fingerprint string, err error) {
	fp := sha256.Sum256(cert.Certificate[0])
	return fmt.Sprintf("%x", fp), nil
}

func (p *ptlsCertMan) UploadFingerprint(ctx context.Context, fingerprint string) error {
	return nil
}
