package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/endpoints"
)

type DeviceRegistrationTokenExchangeRequest struct {
	Token string `json:"token"`
}

type DeviceRegistrationTokenExchangeResponse struct {
	DeviceGUID   string `json:"device_guid"`
	APIKey       string `json:"api_key"`
	CustomerGUID string `json:"customer_guid,omitempty"`
}

func ExchangeDeviceRegistrationToken(baseURL, token string) (*DeviceRegistrationTokenExchangeResponse, error) {
	requestBody, err := json.Marshal(DeviceRegistrationTokenExchangeRequest{
		Token: token,
	})
	if err != nil {
		return nil, err
	}

	exchangeURL, err := endpoints.APIURL(baseURL, "/device-registration-tokens/exchange")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", exchangeURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
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
		return nil, fmt.Errorf("registration token exchange failed: %s - %s", resp.Status, string(body))
	}

	response := &DeviceRegistrationTokenExchangeResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, err
	}
	if response.DeviceGUID == "" {
		return nil, fmt.Errorf("registration token exchange response did not include device_guid")
	}
	if response.APIKey == "" {
		return nil, fmt.Errorf("registration token exchange response did not include api_key")
	}

	return response, nil
}
