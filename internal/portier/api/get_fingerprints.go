package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type GetFingerPrintRequest struct {

	// Device ID of the device to upload the fingerprint for
	DeviceIDs []string `json:"DeviceIDs"`
}

type GetFingerPrintResponse struct {
	Username     string            `json:"username"`
	Fingerprints map[string]string `json:"fingerprints"`
}

func GetFingerprint(home, baseURL string, deviceIDs []string) (map[string]string, error) {
	accessToken, err := LoadAccessToken(home)
	if err != nil {
		return nil, err
	}

	// Create the request
	payload := GetFingerPrintRequest{
		DeviceIDs: deviceIDs,
	}

	// Convert the request to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Make the POST request
	url := fmt.Sprintf("%s/fingerprints", baseURL)
	log.Printf("Gettings fingerprints from %s\n", url)
	fmt.Println()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Set Authorization header if access token available
	if accessToken.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken.AccessToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get fingerprints: %v", err)
	}
	defer resp.Body.Close()
	body := &bytes.Buffer{}
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get fingerprints: %s. Response: %v", resp.Status, body.String())
	}

	// Decode the response
	var response GetFingerPrintResponse
	err = json.NewDecoder(body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return response.Fingerprints, nil
}
