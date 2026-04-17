package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
)

// DeviceByNameResponse represents the response structure of GET /deviceByName/<name>
type DeviceByNameResponse struct {
	GUID string `json:"GUID"`
}

// GetDeviceByName fetches the device GUID for a given device name from the API
// using the configured Portier base URL.
func GetDeviceByName(home, baseURL, name string) (string, error) {
	accessToken, err := LoadAccessToken(home)
	useAPIKey := false
	var apiKey string
	if err != nil || accessToken.AccessToken == "" {
		creds, derr := LoadDeviceCredentials(home, "credentials_device.yaml", baseURL)
		if derr != nil {
			if err != nil {
				return "", err
			}
			return "", derr
		}
		useAPIKey = true
		apiKey = creds.APIKey
	}

	var url string
	if useAPIKey {
		url, err = endpoints.SpiderURL(baseURL, "/deviceByName/"+name)
	} else {
		url, err = endpoints.APIURL(baseURL, "/deviceByName/"+name)
	}
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if useAPIKey {
		req.Header.Set("Authorization", apiKey)
	} else if accessToken.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken.AccessToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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
