package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type FingerPrintUploadRequest struct {

	// Device ID of the device to upload the fingerprint for
	DeviceID string `json:"device_id"`

	// The fingerprint of the TLS certificate in DER format
	Fingerprint string `json:"fingerprint"`
}

func UploadFingerprint(home, baseURL, deviceID, fingerprint string) error {
	accessToken, err := LoadAccessToken(home)
	if err != nil {
		return err
	}

	// Create the request
	payload := FingerPrintUploadRequest{
		DeviceID:    deviceID,
		Fingerprint: fingerprint,
	}

	// Convert the request to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Make the POST request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/device/%s/fingerprint", baseURL, deviceID), bytes.NewBuffer(payloadJSON))
	if err != nil {
		return err
	}

	// Set Authorization header if access token available
	if accessToken.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken.AccessToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload fingerprint: %s", resp.Status)
	}

	defer resp.Body.Close()

	return nil
}
