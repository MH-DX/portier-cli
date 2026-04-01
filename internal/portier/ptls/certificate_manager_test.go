package ptls

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateCert(t *testing.T) {
	// GIVEN
	underTest := NewPTLSCertificateManager()

	// WHEN
	cert, priv, err := underTest.CreateCertificate("00000000-0000-0000-0000-000000000001")

	// THEN
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	require.Equal(t, "00000000-0000-0000-0000-000000000001", cert.Subject.CommonName)
	require.Contains(t, cert.DNSNames, "00000000-0000-0000-0000-000000000001")

	certPEM, keyPEM, err := underTest.ConvertCertificateToPEM(cert, priv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotEmpty(t, certPEM)
	require.NotEmpty(t, keyPEM)

	// get fingerprint
	fp, err := underTest.GetFingerprint(cert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotEmpty(t, fp)
}
