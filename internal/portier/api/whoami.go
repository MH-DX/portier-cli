package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
)

type DeviceWhoAmIResponse struct {
	GUID     string   `json:"GUID"`
	Name     string   `json:"Name"`
	UserGUID string   `json:"UserGUID"`
	Networks []string `json:"Networks"`
}

// WhoAmI calls the device identity endpoint on the provided Portier base URL
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

// WhoAmIDevice calls the device identity endpoint on the provided Portier base URL
// and returns the full device response.
func WhoAmIDevice(baseURL, apiKey string) (*DeviceWhoAmIResponse, error) {
	url, err := endpoints.SpiderURL(baseURL, "/whoami")
	if err != nil {
		return nil, err
	}

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
