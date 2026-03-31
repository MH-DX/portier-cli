package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DeviceWhoAmIResponse struct {
	GUID     string   `json:"GUID"`
	Name     string   `json:"Name"`
	UserGUID string   `json:"UserGUID"`
	Networks []string `json:"Networks"`
}

// WhoAmI calls the /spider/whoami endpoint using the provided api key
// and returns the device GUID.
func WhoAmI(baseURL, apiKey string) (uuid.UUID, error) {
	response, err := WhoAmIDevice(baseURL, apiKey)
	if err != nil {
		return uuid.Nil, err
	}

	guid, err := uuid.Parse(response.GUID)
	if err != nil {
		return uuid.Nil, err
	}

	return guid, nil
}

// WhoAmIDevice calls the /spider/whoami endpoint using the provided api key
// and returns the full device response.
func WhoAmIDevice(baseURL, apiKey string) (*DeviceWhoAmIResponse, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := fmt.Sprintf("%s/spider/whoami", baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whoami failed: %s - %s", resp.Status, string(body))
	}

	response := &DeviceWhoAmIResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, err
	}

	if response.GUID == "" {
		return nil, fmt.Errorf("GUID not found in whoami response")
	}

	return response, nil
}
