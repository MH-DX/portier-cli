package taskcert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Material struct {
	PrivateKey    crypto.PrivateKey
	PrivateKeyPEM []byte
	CSR           *x509.CertificateRequest
	CSRPEM        []byte
}

type Metadata struct {
	APIURL               string   `yaml:"api_url"`
	TaskGUID             string   `yaml:"task_guid"`
	TaskToken            string   `yaml:"task_token"`
	CustomerGUID         string   `yaml:"customer_guid"`
	DeviceGUIDs          []string `yaml:"device_guids"`
	Scope                string   `yaml:"scope"`
	NotBefore            string   `yaml:"not_before"`
	NotAfter             string   `yaml:"not_after"`
	RequiredURISANs      []string `yaml:"required_uri_sans"`
	PrivateKeyFile       string   `yaml:"private_key_file"`
	CertificateFile      string   `yaml:"certificate_file"`
	CertificateChainFile string   `yaml:"certificate_chain_file"`
}

type StoredPaths struct {
	OutputDir            string
	PrivateKeyPath       string
	CertificatePath      string
	CertificateChainPath string
	MetadataPath         string
}

func Generate(taskGUID string, requiredURISANs []string) (*Material, error) {
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
			CommonName: "portier-task-" + taskGUID,
		},
		URIs: uris,
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
	if err := ValidateCSRContainsURISANs(csr, requiredURISANs); err != nil {
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

func ValidateCSRContainsURISANs(csr *x509.CertificateRequest, requiredURISANs []string) error {
	availableURIs := make(map[string]struct{}, len(csr.URIs))
	for _, uriValue := range csr.URIs {
		availableURIs[uriValue.String()] = struct{}{}
	}

	missing := make([]string, 0)
	for _, requiredURI := range requiredURISANs {
		if _, ok := availableURIs[requiredURI]; !ok {
			missing = append(missing, requiredURI)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("generated CSR is missing required URI SANs: %s", strings.Join(missing, ", "))
	}

	return nil
}

func ValidateIssuedMaterials(certificatePEM, chainPEM []byte, privateKey crypto.PrivateKey, expectedNotBefore, expectedNotAfter time.Time, requiredURISANs []string) error {
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

	if err := validateCertificateContainsURISANs(leafCertificate, requiredURISANs); err != nil {
		return err
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
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Roots:       roots,
	}); err != nil {
		return fmt.Errorf("issued certificate failed chain validation: %w", err)
	}

	return nil
}

func Store(outputDir string, privateKeyPEM, certificatePEM, certificateChainPEM []byte, metadata *Metadata) (*StoredPaths, error) {
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return nil, err
	}

	paths := &StoredPaths{
		OutputDir:            outputDir,
		PrivateKeyPath:       filepath.Join(outputDir, "private-key.pem"),
		CertificatePath:      filepath.Join(outputDir, "certificate.pem"),
		CertificateChainPath: filepath.Join(outputDir, "certificate-chain.pem"),
		MetadataPath:         filepath.Join(outputDir, "metadata.yaml"),
	}

	if err := os.WriteFile(paths.PrivateKeyPath, privateKeyPEM, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(paths.CertificatePath, certificatePEM, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(paths.CertificateChainPath, certificateChainPEM, 0o644); err != nil {
		return nil, err
	}

	metadata.PrivateKeyFile = paths.PrivateKeyPath
	metadata.CertificateFile = paths.CertificatePath
	metadata.CertificateChainFile = paths.CertificateChainPath

	metadataBytes, err := yaml.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(paths.MetadataPath, metadataBytes, 0o600); err != nil {
		return nil, err
	}

	return paths, nil
}

func LoadMetadata(metadataPath string) (*Metadata, error) {
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	metadata := &Metadata{}
	if err := yaml.Unmarshal(metadataBytes, metadata); err != nil {
		return nil, err
	}

	return metadata, nil
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
	certificatePublicKey, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return err
	}

	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("private key does not implement crypto.Signer")
	}

	privateKeyPublicKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return err
	}

	if string(certificatePublicKey) != string(privateKeyPublicKey) {
		return fmt.Errorf("issued certificate public key does not match the generated private key")
	}

	return nil
}

func validateCertificateContainsURISANs(certificate *x509.Certificate, requiredURISANs []string) error {
	availableURIs := make(map[string]struct{}, len(certificate.URIs))
	for _, uriValue := range certificate.URIs {
		availableURIs[uriValue.String()] = struct{}{}
	}

	missing := make([]string, 0)
	for _, requiredURI := range requiredURISANs {
		if _, ok := availableURIs[requiredURI]; !ok {
			missing = append(missing, requiredURI)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("issued certificate is missing required URI SANs: %s", strings.Join(missing, ", "))
	}

	return nil
}

func containsExtKeyUsage(values []x509.ExtKeyUsage, expected x509.ExtKeyUsage) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}

	return false
}
