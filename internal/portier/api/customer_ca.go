package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CustomerCABundleError struct {
	StatusCode int
	Message    string
}

func (e *CustomerCABundleError) Error() string {
	return fmt.Sprintf("customer CA bundle request failed: %s", e.Message)
}

func GetCurrentCustomerCABundle(baseURL, customerGUID string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/api/customers/%s/ca-certificates/current.pem", normalizeCustomerCABaseURL(baseURL), customerGUID)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("customer CA bundle request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("customer CA bundle request failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &CustomerCABundleError{
			StatusCode: resp.StatusCode,
			Message:    decodeCustomerCAError(body, resp.Status),
		}
	}

	return body, nil
}

type customerCAErrorResponse struct {
	Error string `json:"error"`
}

func decodeCustomerCAError(body []byte, fallback string) string {
	response := customerCAErrorResponse{}
	if err := json.Unmarshal(body, &response); err == nil && strings.TrimSpace(response.Error) != "" {
		return response.Error
	}

	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody != "" {
		return trimmedBody
	}

	return fallback
}

func normalizeCustomerCABaseURL(baseURL string) string {
	result := strings.TrimSpace(strings.TrimSuffix(baseURL, "/"))
	result = strings.TrimSuffix(result, "/api")
	return result
}
