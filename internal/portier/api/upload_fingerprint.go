package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type FingerPrintUploadRequest struct {

	// Device ID of the device to upload the fingerprint for
	DeviceID string `json:"DeviceID"`

	// The fingerprint of the TLS certificate in DER format
	SHA256Fingerprint string `json:"SHA256Fingerprint"`
}

func UploadFingerprint(home, baseURL, deviceID, fingerprint string) error {
	accessToken, err := LoadAccessToken(home)
	if err != nil {
		return err
	}

	// Create the request
	payload := FingerPrintUploadRequest{
		DeviceID:          deviceID,
		SHA256Fingerprint: fingerprint,
	}

	// Convert the request to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Make the POST request
	url := fmt.Sprintf("%s/fingerprintupsert", baseURL)
	log.Printf("Uploading fingerprint to %s\n", url)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Set Authorization header if access token available
	if accessToken.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken.AccessToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	body := new(bytes.Buffer)
	body.ReadFrom(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to upload fingerprint: %v. Response: %v", err, body.String())
	}

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload fingerprint: %s. Response: %v", resp.Status, body.String())
	}

	defer resp.Body.Close()

	return nil
}
