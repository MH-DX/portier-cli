package devicecert

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Material struct {
	PrivateKey    crypto.PrivateKey
	PrivateKeyPEM []byte
	CSR           *x509.CertificateRequest
	CSRPEM        []byte
}

func Generate(deviceGUID string, requiredDNSSANs, requiredURISANs []string) (*Material, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	uris := make([]*url.URL, 0, len(requiredURISANs))
	for _, rawURI := range requiredURISANs {
		parsedURI, err := url.Parse(rawURI)
		if err != nil {
			return nil, fmt.Errorf("failed to parse required URI SAN %q: %w", rawURI, err)
		}
		uris = append(uris, parsedURI)
	}

	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: "portier-device-" + deviceGUID,
		},
		DNSNames: append([]string(nil), requiredDNSSANs...),
		URIs:     uris,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privateKey)
	if err != nil {
		return nil, err
	}

	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, err
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, err
	}
	if err := ValidateCSRContainsExactSANs(csr, requiredDNSSANs, requiredURISANs); err != nil {
		return nil, err
	}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})
	if privateKeyPEM == nil {
		return nil, fmt.Errorf("failed to encode private key PEM")
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})
	if csrPEM == nil {
		return nil, fmt.Errorf("failed to encode CSR PEM")
	}

	return &Material{
		PrivateKey:    privateKey,
		PrivateKeyPEM: privateKeyPEM,
		CSR:           csr,
		CSRPEM:        csrPEM,
	}, nil
}

func ValidateCSRContainsExactSANs(csr *x509.CertificateRequest, requiredDNSSANs, requiredURISANs []string) error {
	if !stringSetEquals(csr.DNSNames, requiredDNSSANs) {
		return fmt.Errorf("generated CSR DNS SANs do not exactly match required DNS SANs")
	}
	if !stringSetEquals(extractURIs(csr.URIs), requiredURISANs) {
		return fmt.Errorf("generated CSR URI SANs do not exactly match required URI SANs")
	}

	return nil
}

func ValidateIssuedMaterials(certificatePEM, chainPEM []byte, privateKey crypto.PrivateKey, expectedNotBefore, expectedNotAfter time.Time, requiredDNSSANs, requiredURISANs []string) error {
	leafCertificate, err := parseSingleCertificatePEM(certificatePEM)
	if err != nil {
		return fmt.Errorf("invalid certificate PEM: %w", err)
	}

	chainCertificates, err := parseCertificateChainPEM(chainPEM)
	if err != nil {
		return fmt.Errorf("invalid certificate chain PEM: %w", err)
	}

	if err := validatePrivateKeyMatchesCertificate(privateKey, leafCertificate); err != nil {
		return err
	}

	if !leafCertificate.NotBefore.UTC().Equal(expectedNotBefore.UTC()) {
		return fmt.Errorf("issued certificate not_before %s does not match expected %s", leafCertificate.NotBefore.UTC().Format(time.RFC3339), expectedNotBefore.UTC().Format(time.RFC3339))
	}
	if !leafCertificate.NotAfter.UTC().Equal(expectedNotAfter.UTC()) {
		return fmt.Errorf("issued certificate not_after %s does not match expected %s", leafCertificate.NotAfter.UTC().Format(time.RFC3339), expectedNotAfter.UTC().Format(time.RFC3339))
	}

	if !stringSetEquals(leafCertificate.DNSNames, requiredDNSSANs) {
		return fmt.Errorf("issued certificate DNS SANs do not exactly match required DNS SANs")
	}
	if !stringSetEquals(extractURIs(leafCertificate.URIs), requiredURISANs) {
		return fmt.Errorf("issued certificate URI SANs do not exactly match required URI SANs")
	}
	if !containsExtKeyUsage(leafCertificate.ExtKeyUsage, x509.ExtKeyUsageServerAuth) {
		return fmt.Errorf("issued certificate is missing serverAuth extended key usage")
	}
	if !containsExtKeyUsage(leafCertificate.ExtKeyUsage, x509.ExtKeyUsageClientAuth) {
		return fmt.Errorf("issued certificate is missing clientAuth extended key usage")
	}

	roots := x509.NewCertPool()
	for _, certificate := range chainCertificates {
		roots.AddCert(certificate)
	}

	if _, err := leafCertificate.Verify(x509.VerifyOptions{
		CurrentTime: expectedNotBefore.UTC().Add(time.Second),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		Roots:       roots,
	}); err != nil {
		return fmt.Errorf("issued certificate failed chain validation: %w", err)
	}

	return nil
}

func parseSingleCertificatePEM(certificatePEM []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(certificatePEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unexpected PEM block %q", block.Type)
	}
	if strings.TrimSpace(string(rest)) != "" {
		return nil, fmt.Errorf("expected a single certificate PEM block")
	}

	return x509.ParseCertificate(block.Bytes)
}

func parseCertificateChainPEM(chainPEM []byte) ([]*x509.Certificate, error) {
	remaining := chainPEM
	certificates := make([]*x509.Certificate, 0)
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("unexpected PEM block %q", block.Type)
		}

		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, certificate)
		remaining = rest
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificate PEM blocks found")
	}
	if strings.TrimSpace(string(remaining)) != "" {
		return nil, fmt.Errorf("trailing non-PEM data in certificate chain")
	}

	return certificates, nil
}

func validatePrivateKeyMatchesCertificate(privateKey crypto.PrivateKey, certificate *x509.Certificate) error {
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("generated private key does not implement crypto.Signer")
	}

	privateKeyPublicKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return fmt.Errorf("could not marshal generated private key public key: %w", err)
	}
	certificatePublicKey, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return fmt.Errorf("could not marshal issued certificate public key: %w", err)
	}
	if !bytes.Equal(privateKeyPublicKey, certificatePublicKey) {
		return fmt.Errorf("issued certificate public key does not match the generated private key")
	}

	return nil
}

func containsExtKeyUsage(usages []x509.ExtKeyUsage, targetUsage x509.ExtKeyUsage) bool {
	for _, usage := range usages {
		if usage == targetUsage {
			return true
		}
	}

	return false
}

func extractURIs(uris []*url.URL) []string {
	values := make([]string, 0, len(uris))
	for _, uriValue := range uris {
		values = append(values, uriValue.String())
	}
	return values
}

func stringSetEquals(actual []string, expected []string) bool {
	actualSet := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		actualSet[value] = struct{}{}
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, value := range expected {
		expectedSet[value] = struct{}{}
	}

	if len(actualSet) != len(expectedSet) {
		return false
	}
	for value := range expectedSet {
		if _, ok := actualSet[value]; !ok {
			return false
		}
	}

	return true
}
