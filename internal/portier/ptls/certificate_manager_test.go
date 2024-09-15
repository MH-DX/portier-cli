package ptls

import (
	"os"
	"testing"
)

func TestCreateCert(t *testing.T) {
	// GIVEN
	underTest := NewPTLSCertificateManager()

	// WHEN
	cert, err := underTest.CreateCertificate("00000000-0000-0000-0000-000000000001")

	// THEN
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert.PrivateKey == nil {
		t.Errorf("expected private key to be non-nil")
	}

	// print the cert in PEM format
	certPEM, keyPEM, err := underTest.ConvertCertificateToPEM(cert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// get fingerprint
	fp, err := underTest.GetFingerprint(cert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// write cert and key to files
	os.WriteFile("cert.pem", certPEM, 0644)
	os.WriteFile("key.pem", keyPEM, 0644)
	os.WriteFile("fingerprint", []byte(fp), 0644)
}
