package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
)

type DeviceCertificateInfoResponse struct {
	DeviceGUID         string   `json:"device_guid"`
	CustomerGUID       string   `json:"customer_guid"`
	NotBefore          string   `json:"not_before"`
	NotAfter           string   `json:"not_after"`
	RequiredDNSSANs    []string `json:"required_dns_sans"`
	RequiredURISANs    []string `json:"required_uri_sans"`
	CertificateProfile string   `json:"certificate_profile"`
}

type DeviceCertificateRequest struct {
	CSR string `json:"csr"`
}

type DeviceCertificateResponse struct {
	DeviceGUID         string `json:"device_guid"`
	CustomerGUID       string `json:"customer_guid"`
	Certificate        string `json:"certificate"`
	CertificateChain   string `json:"certificate_chain"`
	NotBefore          string `json:"not_before"`
	NotAfter           string `json:"not_after"`
	CertificateProfile string `json:"certificate_profile"`
}

type DeviceCertificateAPIError struct {
	Operation  string
	StatusCode int
	Message    string
}

func (e *DeviceCertificateAPIError) Error() string {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Sprintf("%s unauthorized: %s", e.Operation, e.Message)
	case http.StatusUnprocessableEntity:
		return fmt.Sprintf("%s rejected: %s", e.Operation, e.Message)
	default:
		return fmt.Sprintf("%s failed: %s", e.Operation, e.Message)
	}
}

type deviceCertificateErrorResponse struct {
	Error string `json:"error"`
}

func GetDeviceCertificateInfo(baseURL, deviceGUID, apiKey string) (*DeviceCertificateInfoResponse, error) {
	endpoint, err := endpoints.APIURL(baseURL, fmt.Sprintf("/devices/%s/certificate-info", deviceGUID))
	if err != nil {
		return nil, err
	}

	var response DeviceCertificateInfoResponse
	if err := doDeviceCertificateRequest(
		"device certificate info request",
		endpoint,
		apiKey,
		struct{}{},
		&response,
	); err != nil {
		return nil, err
	}

	return &response, nil
}

func IssueDeviceCertificate(baseURL, deviceGUID, apiKey, csr string) (*DeviceCertificateResponse, error) {
	endpoint, err := endpoints.APIURL(baseURL, fmt.Sprintf("/devices/%s/certificate", deviceGUID))
	if err != nil {
		return nil, err
	}

	var response DeviceCertificateResponse
	if err := doDeviceCertificateRequest(
		"device certificate signing request",
		endpoint,
		apiKey,
		DeviceCertificateRequest{CSR: csr},
		&response,
	); err != nil {
		return nil, err
	}

	return &response, nil
}

func doDeviceCertificateRequest(operation, endpoint, apiKey string, requestBody any, responseBody any) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s failed: %w", operation, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s failed to read response body: %w", operation, err)
	}

	if resp.StatusCode != http.StatusOK {
		return &DeviceCertificateAPIError{
			Operation:  operation,
			StatusCode: resp.StatusCode,
			Message:    decodeDeviceCertificateError(body, resp.Status),
		}
	}

	if err := json.Unmarshal(body, responseBody); err != nil {
		return fmt.Errorf("%s failed to decode response: %w", operation, err)
	}

	return nil
}

func decodeDeviceCertificateError(body []byte, fallback string) string {
	var response deviceCertificateErrorResponse
	if err := json.Unmarshal(body, &response); err == nil && strings.TrimSpace(response.Error) != "" {
		return response.Error
	}

	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody != "" {
		return trimmedBody
	}

	return fallback
}
