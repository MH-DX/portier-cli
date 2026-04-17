package cmd

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type storedDeviceCredentials struct {
	APIKey string `yaml:"APIKey"`
}

type customerSetupTestServerOptions struct {
	customerGUID       string
	expectedAPIKey     string
	deviceGUID         string
	networks           []string
	requiredDNSSANs    []string
	requiredURISANs    []string
	notBefore          time.Time
	notAfter           time.Time
	certificateProfile string
	caCertificate      *x509.Certificate
	caPrivateKey       crypto.Signer
	caPEM              []byte
	signStatusCode     int
	signError          string
	signCertificatePEM []byte
	signChainPEM       []byte
	onCSR              func(*testing.T, *x509.CertificateRequest)
}

func TestCustomerSetupSuccess(t *testing.T) {
	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	notBefore := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC)
	server := newCustomerSetupTestServer(t, customerSetupTestServerOptions{
		customerGUID:       customerGUID,
		expectedAPIKey:     deviceAPIKey,
		deviceGUID:         deviceGUID,
		networks:           []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"},
		requiredDNSSANs:    []string{deviceGUID},
		requiredURISANs:    []string{"urn:portier:device:" + deviceGUID},
		notBefore:          notBefore,
		notAfter:           notAfter,
		certificateProfile: "portier-device-v1",
		caCertificate:      caCertificate,
		caPrivateKey:       caPrivateKey,
		caPEM:              caPEM,
		onCSR: func(t *testing.T, csr *x509.CertificateRequest) {
			require.Equal(t, []string{deviceGUID}, csr.DNSNames)
			require.Len(t, csr.URIs, 1)
			require.Equal(t, "urn:portier:device:"+deviceGUID, csr.URIs[0].String())
		},
	})
	defer server.Close()

	homeDir := t.TempDir()
	output := &bytes.Buffer{}

	command := newCustomerSetupCmd()
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
	})

	require.NoError(t, command.Execute())

	var credentials storedDeviceCredentials
	credentialsBytes, err := os.ReadFile(filepath.Join(homeDir, "credentials_device.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(credentialsBytes, &credentials))
	require.Equal(t, deviceAPIKey, credentials.APIKey)

	caBundleBytes, err := os.ReadFile(filepath.Join(homeDir, "cacert.pem"))
	require.NoError(t, err)
	require.Equal(t, string(caPEM), string(caBundleBytes))

	certificateBytes, err := os.ReadFile(filepath.Join(homeDir, "cert.pem"))
	require.NoError(t, err)
	deviceCertificate := parseTestCertificatePEM(t, certificateBytes)
	require.Equal(t, []string{deviceGUID}, deviceCertificate.DNSNames)
	require.Len(t, deviceCertificate.URIs, 1)
	require.Equal(t, "urn:portier:device:"+deviceGUID, deviceCertificate.URIs[0].String())
	require.Equal(t, notBefore, deviceCertificate.NotBefore.UTC())
	require.Equal(t, notAfter, deviceCertificate.NotAfter.UTC())
	require.NotEqual(t, deviceCertificate.Issuer.String(), deviceCertificate.Subject.String())

	keyBytes, err := os.ReadFile(filepath.Join(homeDir, "key.pem"))
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(keyBytes)
	require.NotNil(t, keyBlock)
	require.Equal(t, "PRIVATE KEY", keyBlock.Type)

	cfg, err := config.LoadConfig(filepath.Join(homeDir, "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, server.URL, cfg.APIBaseURL())
	require.True(t, cfg.TLSEnabled)
	require.Equal(t, filepath.Join(homeDir, "cacert.pem"), cfg.PTLSConfig.CAFile)
	require.Equal(t, filepath.Join(homeDir, "cert.pem"), cfg.PTLSConfig.CertFile)
	require.Equal(t, filepath.Join(homeDir, "key.pem"), cfg.PTLSConfig.KeyFile)

	metadataBytes, err := os.ReadFile(filepath.Join(homeDir, "customer_setup.yaml"))
	require.NoError(t, err)
	var metadata customerSetupMetadata
	require.NoError(t, yaml.Unmarshal(metadataBytes, &metadata))
	require.Equal(t, customerGUID, metadata.CustomerGUID)
	require.Equal(t, deviceGUID, metadata.DeviceGUID)
	require.Equal(t, filepath.Join(homeDir, "cacert.pem"), metadata.CACertFile)
	require.Equal(t, filepath.Join(homeDir, "cert.pem"), metadata.DeviceCertFile)
	require.Equal(t, filepath.Join(homeDir, "key.pem"), metadata.DeviceKeyFile)
	require.Equal(t, notBefore.Format(time.RFC3339), metadata.NotBefore)
	require.Equal(t, notAfter.Format(time.RFC3339), metadata.NotAfter)
	require.Equal(t, "portier-device-v1", metadata.CertificateProfile)

	stdout := output.String()
	require.Contains(t, stdout, "Customer setup complete.")
	require.Contains(t, stdout, "Customer: "+customerGUID)
	require.Contains(t, stdout, "Certificate profile: portier-device-v1")
	require.Contains(t, stdout, "Validity: "+notBefore.Format(time.RFC3339)+" to "+notAfter.Format(time.RFC3339))
	require.Contains(t, stdout, "CA certificates: 1")
	require.Contains(t, stdout, "PTLS is ready to validate task client certificates on the next run.")
}

func TestCustomerSetupReplacesLegacySelfSignedDeviceCertificate(t *testing.T) {
	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerSetupTestServerOptions{
		customerGUID:       customerGUID,
		expectedAPIKey:     deviceAPIKey,
		deviceGUID:         deviceGUID,
		networks:           []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"},
		requiredDNSSANs:    []string{deviceGUID},
		requiredURISANs:    []string{"urn:portier:device:" + deviceGUID},
		notBefore:          time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		notAfter:           time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC),
		certificateProfile: "portier-device-v1",
		caCertificate:      caCertificate,
		caPrivateKey:       caPrivateKey,
		caPEM:              caPEM,
	})
	defer server.Close()

	homeDir := t.TempDir()
	legacyCertificate := newLegacySelfSignedDeviceCertificate(t, deviceGUID)
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "cert.pem"), legacyCertificate, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "key.pem"), []byte("legacy"), 0o600))

	command := newCustomerSetupCmd()
	command.SetArgs([]string{
		"--home", homeDir,
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
	})

	require.NoError(t, command.Execute())

	updatedCertificateBytes, err := os.ReadFile(filepath.Join(homeDir, "cert.pem"))
	require.NoError(t, err)
	require.NotEqual(t, string(legacyCertificate), string(updatedCertificateBytes))

	updatedCertificate := parseTestCertificatePEM(t, updatedCertificateBytes)
	require.Equal(t, []string{deviceGUID}, updatedCertificate.DNSNames)

	rootPool := x509.NewCertPool()
	require.True(t, rootPool.AppendCertsFromPEM(caPEM))
	_, err = updatedCertificate.Verify(x509.VerifyOptions{
		Roots:     rootPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	require.NoError(t, err)
}

func TestCustomerSetupRejectsCustomerGUIDMismatch(t *testing.T) {
	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerSetupTestServerOptions{
		customerGUID:       customerGUID,
		expectedAPIKey:     deviceAPIKey,
		deviceGUID:         deviceGUID,
		networks:           []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"},
		requiredDNSSANs:    []string{deviceGUID},
		requiredURISANs:    []string{"urn:portier:device:" + deviceGUID},
		notBefore:          time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		notAfter:           time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC),
		certificateProfile: "portier-device-v1",
		caCertificate:      caCertificate,
		caPrivateKey:       caPrivateKey,
		caPEM:              caPEM,
	})
	defer server.Close()

	command := newCustomerSetupCmd()
	command.SetArgs([]string{
		"--home", t.TempDir(),
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
		"--customer-guid", "different-customer-guid",
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "device certificate info response customer_guid")
}

func TestCustomerSetupRejectsMalformedCertificateChain(t *testing.T) {
	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	const (
		deviceAPIKey = "device-api-key"
		deviceGUID   = "d1b9c2af-ff76-43f1-8347-3b5db88d67bc"
		customerGUID = "0a0cc2b5-8c53-4c08-bff9-a2f655b6dd0a"
	)

	server := newCustomerSetupTestServer(t, customerSetupTestServerOptions{
		customerGUID:       customerGUID,
		expectedAPIKey:     deviceAPIKey,
		deviceGUID:         deviceGUID,
		networks:           []string{"8f29bf1f-6c3a-4ee2-9c5d-526f7894c9d7"},
		requiredDNSSANs:    []string{deviceGUID},
		requiredURISANs:    []string{"urn:portier:device:" + deviceGUID},
		notBefore:          time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		notAfter:           time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC),
		certificateProfile: "portier-device-v1",
		caCertificate:      caCertificate,
		caPrivateKey:       caPrivateKey,
		caPEM:              caPEM,
		signChainPEM:       []byte("not a certificate"),
	})
	defer server.Close()

	command := newCustomerSetupCmd()
	command.SetArgs([]string{
		"--home", t.TempDir(),
		"--apiUrl", server.URL,
		"--apiKey", deviceAPIKey,
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "customer CA bundle did not contain any certificates")
}

func newCustomerSetupTestServer(t *testing.T, options customerSetupTestServerOptions) *httptest.Server {
	t.Helper()

	requiredDNSSANs := append([]string(nil), options.requiredDNSSANs...)
	if len(requiredDNSSANs) == 0 {
		requiredDNSSANs = []string{options.deviceGUID}
	}
	requiredURISANs := append([]string(nil), options.requiredURISANs...)
	if len(requiredURISANs) == 0 {
		requiredURISANs = []string{"urn:portier:device:" + options.deviceGUID}
	}

	notBefore := options.notBefore.UTC()
	if notBefore.IsZero() {
		notBefore = time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	}
	notAfter := options.notAfter.UTC()
	if notAfter.IsZero() {
		notAfter = notBefore.AddDate(1, 0, 0)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spider/whoami":
			require.Equal(t, options.expectedAPIKey, r.Header.Get("Authorization"))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"GUID":     options.deviceGUID,
				"Networks": options.networks,
			})
		case "/api/devices/" + options.deviceGUID + "/certificate-info":
			require.Equal(t, options.expectedAPIKey, r.Header.Get("Authorization"))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"device_guid":         options.deviceGUID,
				"customer_guid":       options.customerGUID,
				"not_before":          notBefore.Format(time.RFC3339),
				"not_after":           notAfter.Format(time.RFC3339),
				"required_dns_sans":   requiredDNSSANs,
				"required_uri_sans":   requiredURISANs,
				"certificate_profile": options.certificateProfile,
			})
		case "/api/devices/" + options.deviceGUID + "/certificate":
			require.Equal(t, options.expectedAPIKey, r.Header.Get("Authorization"))

			if options.signStatusCode != 0 && options.signStatusCode != http.StatusOK {
				writeJSON(t, w, options.signStatusCode, map[string]any{
					"error": options.signError,
				})
				return
			}

			var request struct {
				CSR string `json:"csr"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.NotEmpty(t, request.CSR)

			csr := parseTestCSRPEM(t, []byte(request.CSR))
			if options.onCSR != nil {
				options.onCSR(t, csr)
			}

			certificatePEM := options.signCertificatePEM
			if len(certificatePEM) == 0 {
				certificatePEM = signTestDeviceCertificate(t, options.caCertificate, options.caPrivateKey, csr, notBefore, notAfter, requiredDNSSANs, requiredURISANs)
			}

			chainPEM := options.signChainPEM
			if len(chainPEM) == 0 {
				chainPEM = options.caPEM
			}

			writeJSON(t, w, http.StatusOK, map[string]any{
				"device_guid":         options.deviceGUID,
				"customer_guid":       options.customerGUID,
				"certificate":         string(certificatePEM),
				"certificate_chain":   string(chainPEM),
				"not_before":          notBefore.Format(time.RFC3339),
				"not_after":           notAfter.Format(time.RFC3339),
				"certificate_profile": options.certificateProfile,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func signTestDeviceCertificate(t *testing.T, caCertificate *x509.Certificate, caPrivateKey crypto.Signer, csr *x509.CertificateRequest, notBefore, notAfter time.Time, requiredDNSSANs, requiredURISANs []string) []byte {
	t.Helper()

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	uris := make([]*url.URL, 0, len(requiredURISANs))
	for _, rawURI := range requiredURISANs {
		parsedURI, err := url.Parse(rawURI)
		require.NoError(t, err)
		uris = append(uris, parsedURI)
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               csr.Subject,
		NotBefore:             notBefore.UTC(),
		NotAfter:              notAfter.UTC(),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              append([]string(nil), requiredDNSSANs...),
		URIs:                  uris,
	}, caCertificate, csr.PublicKey, caPrivateKey)
	require.NoError(t, err)

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})
	require.NotNil(t, certificatePEM)

	return certificatePEM
}

func parseTestCSRPEM(t *testing.T, csrPEM []byte) *x509.CertificateRequest {
	t.Helper()

	block, _ := pem.Decode(csrPEM)
	require.NotNil(t, block)
	require.Equal(t, "CERTIFICATE REQUEST", block.Type)

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	require.NoError(t, err)
	require.NoError(t, csr.CheckSignature())

	return csr
}

func parseTestCertificatePEM(t *testing.T, certificatePEM []byte) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode(certificatePEM)
	require.NotNil(t, block)
	require.Equal(t, "CERTIFICATE", block.Type)

	certificate, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return certificate
}

func newLegacySelfSignedDeviceCertificate(t *testing.T, commonName string) []byte {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	require.NoError(t, err)

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})
	require.NotNil(t, certificatePEM)

	return certificatePEM
}
