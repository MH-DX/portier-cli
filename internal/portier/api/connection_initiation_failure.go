package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ConnectionInitiationFailureRequest struct {
	ConnectingDeviceGUID string `json:"connectingDeviceGUID"`
	ConnectionID         string `json:"connectionID"`
	ErrorCode            string `json:"errorCode"`
	ErrorMessage         string `json:"errorMessage"`
	RecommendedAction    string `json:"recommendedAction"`
	RemoteURL            string `json:"remoteURL"`
}

func ReportConnectionInitiationFailure(baseURL, apiKey string, request ConnectionInitiationFailureRequest) error {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := fmt.Sprintf("%s/spider/connection-initiation-failure", baseURL)

	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	httpRequest, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Authorization", apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpRequest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("connection initiation failure report failed: %s", resp.Status)
	}
	return nil
}
