package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeviceByNameResponse represents the response structure of GET /deviceByName/<name>
type DeviceByNameResponse struct {
	GUID string `json:"GUID"`
}

// GetDeviceByName fetches the device GUID for a given device name from the API.
func GetDeviceByName(baseURL, name string, apiKey string) (string, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := fmt.Sprintf("%s/spider/deviceByName/%s", baseURL, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get device by name: %s. Response: %s", resp.Status, string(body))
	}
	var d DeviceByNameResponse
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return "", err
	}
	if d.GUID == "" {
		return "", fmt.Errorf("device not found")
	}
	return d.GUID, nil
}
