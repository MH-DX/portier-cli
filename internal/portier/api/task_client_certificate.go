package portier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
)

type TaskClientCertificateInfoRequest struct {
	TaskToken string `json:"task_token"`
}

type TaskClientCertificateRequest struct {
	TaskToken string `json:"task_token"`
	CSR       string `json:"csr"`
}

type TaskClientCertificateInfoResponse struct {
	TaskGUID        string   `json:"task_guid"`
	CustomerGUID    string   `json:"customer_guid"`
	DeviceGUIDs     []string `json:"device_guids"`
	Scope           string   `json:"scope"`
	NotBefore       string   `json:"not_before"`
	NotAfter        string   `json:"not_after"`
	RequiredURISANs []string `json:"required_uri_sans"`
}

type TaskClientCertificateResponse struct {
	TaskGUID         string   `json:"task_guid"`
	CustomerGUID     string   `json:"customer_guid"`
	DeviceGUIDs      []string `json:"device_guids"`
	Scope            string   `json:"scope"`
	Certificate      string   `json:"certificate"`
	CertificateChain string   `json:"certificate_chain"`
	NotBefore        string   `json:"not_before"`
	NotAfter         string   `json:"not_after"`
}

type TaskClientCertificateAPIError struct {
	Operation  string
	StatusCode int
	Message    string
}

func (e *TaskClientCertificateAPIError) Error() string {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Sprintf("%s unauthorized: %s", e.Operation, e.Message)
	case http.StatusUnprocessableEntity:
		return fmt.Sprintf("%s rejected: %s", e.Operation, e.Message)
	default:
		return fmt.Sprintf("%s failed: %s", e.Operation, e.Message)
	}
}

func GetTaskClientCertificateInfo(baseURL, taskGUID, taskToken string) (*TaskClientCertificateInfoResponse, error) {
	requestBody := TaskClientCertificateInfoRequest{
		TaskToken: taskToken,
	}

	var response TaskClientCertificateInfoResponse
	err := doTaskClientCertificateRequest(
		"task certificate info request",
		taskClientCertificateEndpoints(baseURL, taskGUID, "client-certificate-info"),
		requestBody,
		&response,
	)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func IssueTaskClientCertificate(baseURL, taskGUID, taskToken, csr string) (*TaskClientCertificateResponse, error) {
	requestBody := TaskClientCertificateRequest{
		TaskToken: taskToken,
		CSR:       csr,
	}

	var response TaskClientCertificateResponse
	err := doTaskClientCertificateRequest(
		"task certificate signing request",
		taskClientCertificateEndpoints(baseURL, taskGUID, "client-certificate"),
		requestBody,
		&response,
	)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

type taskCertificateErrorResponse struct {
	Error string `json:"error"`
}

func doTaskClientCertificateRequest(operation string, endpoints []string, requestBody any, responseBody any) error {
	var lastError error
	for _, endpoint := range endpoints {
		err := doTaskClientCertificateRequestOnce(operation, endpoint, requestBody, responseBody)
		if err == nil {
			return nil
		}

		var apiError *TaskClientCertificateAPIError
		if errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound {
			lastError = err
			continue
		}

		return err
	}

	if lastError != nil {
		return lastError
	}

	return fmt.Errorf("%s failed: no task certificate endpoints configured", operation)
}

func doTaskClientCertificateRequestOnce(operation, endpoint string, requestBody any, responseBody any) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
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
		return &TaskClientCertificateAPIError{
			Operation:  operation,
			StatusCode: resp.StatusCode,
			Message:    decodeTaskCertificateError(body, resp.Status),
		}
	}

	if err := json.Unmarshal(body, responseBody); err != nil {
		return fmt.Errorf("%s failed to decode response: %w", operation, err)
	}

	return nil
}

func decodeTaskCertificateError(body []byte, fallback string) string {
	var response taskCertificateErrorResponse
	if err := json.Unmarshal(body, &response); err == nil && strings.TrimSpace(response.Error) != "" {
		return response.Error
	}

	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody != "" {
		return trimmedBody
	}

	return fallback
}

func taskClientCertificateEndpoints(baseURL, taskGUID, suffix string) []string {
	resourcePath := fmt.Sprintf("/tasks/%s/%s", taskGUID, suffix)

	publicURL, err := endpoints.PublicURL(baseURL, resourcePath)
	if err != nil {
		return nil
	}

	apiURL, err := endpoints.APIURL(baseURL, resourcePath)
	if err != nil {
		return []string{publicURL}
	}

	return []string{publicURL, apiURL}
}
