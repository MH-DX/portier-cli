package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DeviceByNameResponse represents the response structure of GET /deviceByName/<name>
type DeviceByNameResponse struct {
	GUID string `json:"GUID"`
}

// GetDeviceByName fetches the device GUID for a given device name from the API.
func GetDeviceByName(baseURL, name string) (string, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := fmt.Sprintf("%s/spider/deviceByName/%s", baseURL, name)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
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
