package cmd

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type taskEnrollmentMetadata struct {
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

func TestTaskEnrollSuccess(t *testing.T) {
	notBefore := time.Date(2026, 4, 6, 8, 25, 0, 0, time.UTC)
	notAfter := time.Date(2026, 4, 6, 8, 35, 0, 0, time.UTC)
	scope := "ssh://operatoruser@localhost:22"
	requiredURISANs := []string{
		"urn:portier:task:e3f8e542-1841-4cc8-a463-592be38e3d3f",
		"urn:portier:scope:" + base64.RawURLEncoding.EncodeToString([]byte(scope)),
		"urn:portier:not-before:" + notBefore.Format(time.RFC3339),
		"urn:portier:not-after:" + notAfter.Format(time.RFC3339),
	}

	caCertificate, caPrivateKey, caPEM := newTestCertificateAuthority(t)

	taskGUID := "e3f8e542-1841-4cc8-a463-592be38e3d3f"
	taskToken := "task-secret"
	customerGUID := "5f0244a6-d1ce-4b15-8b11-dff1f002c64c"
	deviceGUIDs := []string{
		"c5610ffa-317b-4f33-ab9c-e0d8e8824cc5",
		"2b1071cf-8421-4538-bb51-3e6d6e66ff03",
	}

	var privateKeySent bool
	var capturedCSR *x509.CertificateRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public/tasks/" + taskGUID + "/client-certificate-info":
			var request map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, taskToken, request["task_token"])
			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     customerGUID,
				"device_guids":      deviceGUIDs,
				"scope":             scope,
				"not_before":        notBefore.Format(time.RFC3339),
				"not_after":         notAfter.Format(time.RFC3339),
				"required_uri_sans": requiredURISANs,
			})
		case "/public/tasks/" + taskGUID + "/client-certificate":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			privateKeySent = strings.Contains(string(body), "PRIVATE KEY")

			var request map[string]string
			require.NoError(t, json.Unmarshal(body, &request))
			require.Equal(t, taskToken, request["task_token"])

			capturedCSR = parseCSRFromPEM(t, request["csr"])
			assertCSRContainsURIs(t, capturedCSR, requiredURISANs)

			leafPEM := signTestTaskCertificate(t, caCertificate, caPrivateKey, capturedCSR, notBefore, notAfter, customerGUID, deviceGUIDs, requiredURISANs)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     customerGUID,
				"device_guids":      deviceGUIDs,
				"scope":             scope,
				"certificate":       string(leafPEM),
				"certificate_chain": string(caPEM),
				"not_before":        notBefore.Format(time.RFC3339),
				"not_after":         notAfter.Format(time.RFC3339),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	homeDir := t.TempDir()
	outputDir := filepath.Join(homeDir, "task-output")
	output := &bytes.Buffer{}

	command := newTaskEnrollCmd()
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{
		"--apiUrl", server.URL,
		"--task-guid", taskGUID,
		"--task-token", taskToken,
		"--home", homeDir,
		"--output-dir", outputDir,
	})

	require.NoError(t, command.Execute())
	require.NotNil(t, capturedCSR)
	require.False(t, privateKeySent)

	privateKeyPEM, err := os.ReadFile(filepath.Join(outputDir, "private-key.pem"))
	require.NoError(t, err)
	require.Contains(t, string(privateKeyPEM), "BEGIN PRIVATE KEY")

	certificatePEM, err := os.ReadFile(filepath.Join(outputDir, "certificate.pem"))
	require.NoError(t, err)
	leafCertificate := parseCertificateFromPEM(t, certificatePEM)
	require.True(t, leafCertificate.NotBefore.Equal(notBefore))
	require.True(t, leafCertificate.NotAfter.Equal(notAfter))

	metadataBytes, err := os.ReadFile(filepath.Join(outputDir, "metadata.yaml"))
	require.NoError(t, err)

	var metadata taskEnrollmentMetadata
	require.NoError(t, yaml.Unmarshal(metadataBytes, &metadata))
	require.Equal(t, taskGUID, metadata.TaskGUID)
	require.Equal(t, taskToken, metadata.TaskToken)
	require.Equal(t, requiredURISANs, metadata.RequiredURISANs)
	require.Equal(t, filepath.Join(outputDir, "private-key.pem"), metadata.PrivateKeyFile)
	require.Equal(t, filepath.Join(outputDir, "certificate.pem"), metadata.CertificateFile)
	require.Equal(t, filepath.Join(outputDir, "certificate-chain.pem"), metadata.CertificateChainFile)

	stdout := output.String()
	require.Contains(t, stdout, "Task certificate enrolled.")
	require.Contains(t, stdout, "Scope: "+scope)
	require.Contains(t, stdout, "Devices: 2")
	require.Contains(t, stdout, "Validity: "+notBefore.Format(time.RFC3339)+" to "+notAfter.Format(time.RFC3339))
	require.Contains(t, stdout, "Stored in: "+outputDir)
}

func TestTaskEnrollUnauthorized(t *testing.T) {
	taskGUID := "e3f8e542-1841-4cc8-a463-592be38e3d3f"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]any{
			"error": "invalid task GUID and token combination",
		})
	}))
	defer server.Close()

	command := newTaskEnrollCmd()
	command.SetArgs([]string{
		"--apiUrl", server.URL,
		"--task-guid", taskGUID,
		"--task-token", "bad-token",
		"--home", t.TempDir(),
		"--output-dir", filepath.Join(t.TempDir(), "task-output"),
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
	require.Contains(t, err.Error(), "invalid task GUID and token combination")
}

func TestTaskEnrollSurfacesCSRClaimErrors(t *testing.T) {
	taskGUID := "e3f8e542-1841-4cc8-a463-592be38e3d3f"
	taskToken := "task-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public/tasks/" + taskGUID + "/client-certificate-info":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     "5f0244a6-d1ce-4b15-8b11-dff1f002c64c",
				"device_guids":      []string{"c5610ffa-317b-4f33-ab9c-e0d8e8824cc5"},
				"scope":             "ssh://operatoruser@localhost:22",
				"not_before":        "2026-04-06T08:25:00Z",
				"not_after":         "2026-04-06T08:35:00Z",
				"required_uri_sans": []string{"urn:portier:task:" + taskGUID},
			})
		case "/public/tasks/" + taskGUID + "/client-certificate":
			writeJSON(t, w, http.StatusUnprocessableEntity, map[string]any{
				"error": "csr does not contain the required task claims",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	command := newTaskEnrollCmd()
	command.SetArgs([]string{
		"--apiUrl", server.URL,
		"--task-guid", taskGUID,
		"--task-token", taskToken,
		"--home", t.TempDir(),
		"--output-dir", filepath.Join(t.TempDir(), "task-output"),
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "rejected")
	require.Contains(t, err.Error(), "csr does not contain the required task claims")
}

func TestTaskEnrollRejectsMalformedCertificateMaterial(t *testing.T) {
	taskGUID := "e3f8e542-1841-4cc8-a463-592be38e3d3f"
	taskToken := "task-secret"
	outputDir := filepath.Join(t.TempDir(), "task-output")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public/tasks/" + taskGUID + "/client-certificate-info":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     "5f0244a6-d1ce-4b15-8b11-dff1f002c64c",
				"device_guids":      []string{"c5610ffa-317b-4f33-ab9c-e0d8e8824cc5"},
				"scope":             "ssh://operatoruser@localhost:22",
				"not_before":        "2026-04-06T08:25:00Z",
				"not_after":         "2026-04-06T08:35:00Z",
				"required_uri_sans": []string{"urn:portier:task:" + taskGUID, "urn:portier:not-before:2026-04-06T08:25:00Z"},
			})
		case "/public/tasks/" + taskGUID + "/client-certificate":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"task_guid":         taskGUID,
				"customer_guid":     "5f0244a6-d1ce-4b15-8b11-dff1f002c64c",
				"device_guids":      []string{"c5610ffa-317b-4f33-ab9c-e0d8e8824cc5"},
				"scope":             "ssh://operatoruser@localhost:22",
				"certificate":       "not a certificate",
				"certificate_chain": "not a chain",
				"not_before":        "2026-04-06T08:25:00Z",
				"not_after":         "2026-04-06T08:35:00Z",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	command := newTaskEnrollCmd()
	command.SetArgs([]string{
		"--apiUrl", server.URL,
		"--task-guid", taskGUID,
		"--task-token", taskToken,
		"--home", t.TempDir(),
		"--output-dir", outputDir,
	})

	err := command.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid certificate PEM")
	_, statErr := os.Stat(outputDir)
	require.Error(t, statErr)
}

func newTestCertificateAuthority(t *testing.T) (*x509.Certificate, crypto.Signer, []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "task-enrollment-test-ca",
		},
		NotBefore:             time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	require.NoError(t, err)

	certificate, err := x509.ParseCertificate(certificateDER)
	require.NoError(t, err)

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})
	require.NotNil(t, certificatePEM)

	return certificate, privateKey, certificatePEM
}

func signTestTaskCertificate(t *testing.T, caCertificate *x509.Certificate, caPrivateKey crypto.Signer, csr *x509.CertificateRequest, notBefore, notAfter time.Time, customerGUID string, deviceGUIDs, requiredURISANs []string) []byte {
	t.Helper()

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	uris := make([]*url.URL, 0, len(requiredURISANs)+len(deviceGUIDs)+1)
	for _, rawURI := range requiredURISANs {
		parsedURI, parseErr := url.Parse(rawURI)
		require.NoError(t, parseErr)
		uris = append(uris, parsedURI)
	}
	customerURI, err := url.Parse("urn:portier:customer:" + customerGUID)
	require.NoError(t, err)
	uris = append(uris, customerURI)
	for _, deviceGUID := range deviceGUIDs {
		deviceURI, parseErr := url.Parse("urn:portier:device:" + deviceGUID)
		require.NoError(t, parseErr)
		uris = append(uris, deviceURI)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      csr.Subject,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         uris,
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, caCertificate, csr.PublicKey, caPrivateKey)
	require.NoError(t, err)

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})
	require.NotNil(t, certificatePEM)

	return certificatePEM
}

func parseCSRFromPEM(t *testing.T, csrPEM string) *x509.CertificateRequest {
	t.Helper()

	block, _ := pem.Decode([]byte(csrPEM))
	require.NotNil(t, block)

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	require.NoError(t, err)
	require.NoError(t, csr.CheckSignature())

	return csr
}

func parseCertificateFromPEM(t *testing.T, certificatePEM []byte) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode(certificatePEM)
	require.NotNil(t, block)

	certificate, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return certificate
}

func assertCSRContainsURIs(t *testing.T, csr *x509.CertificateRequest, expectedURIs []string) {
	t.Helper()

	available := make(map[string]struct{}, len(csr.URIs))
	for _, uriValue := range csr.URIs {
		available[uriValue.String()] = struct{}{}
	}

	for _, expectedURI := range expectedURIs {
		_, ok := available[expectedURI]
		require.Truef(t, ok, "missing URI SAN %s", expectedURI)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, statusCode int, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}
