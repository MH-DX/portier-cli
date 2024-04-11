package portier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type RegistrationRequest struct {
	// The display name for the device
	Name string
}

type Device struct {
	GUID          string   `validate:"required,uuid"`
	Name          string   `validate:"alpha"`
	User          string   `validate:"email"` // FK
	Networks      []string `validate:"omitempty,uuids"`
	OutboundBytes int      `validate:"gte=0"`
	LastSeen      time.Time
}

type ApiKeyRequest struct {
	// A unique identifier for the device in the form of a UUID
	// example: a1b2c3d4-1234-5678-9abc-def012345678
	DeviceGUID string
	// A description for the api key
	Description string
}

type ApiKeyCreation struct {
	GUID        string    `validate:"required,uuid"`
	CreatedAt   time.Time `validate:"required,datetime"`
	DeviceGUID  string    `validate:"required,uuid"`
	Description string    `validate:"required,alphanum"`
	ApiKey      string    `validate:"required,alphanum"`
}

// Function to register a device
func RegisterDevice(baseURL, name, accessToken, home string) (Device, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	regRequest := RegistrationRequest{Name: name}

	// Convert RegistrationRequest to JSON
	regJSON, err := json.Marshal(regRequest)
	if err != nil {
		return Device{}, err
	}

	// Make POST request to register the device
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/device", baseURL), bytes.NewBuffer(regJSON))
	if err != nil {
		return Device{}, err
	}

	// Set Authorization header if access token available
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Device{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// parse the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return Device{}, err
		}
		return Device{}, fmt.Errorf("failed to register device: %s, Response: %s", resp.Status, body)
	}

	var device Device
	// Parse response body
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return Device{}, err
	}

	return device, nil
}

// Function to generate API key for a device
func GenerateApiKey(baseURL, deviceGUID, description, accessToken, home string) (ApiKeyCreation, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	apiKeyRequest := ApiKeyRequest{
		DeviceGUID:  deviceGUID,
		Description: description,
	}

	// Convert ApiKeyRequest to JSON
	apiKeyJSON, err := json.Marshal(apiKeyRequest)
	if err != nil {
		return ApiKeyCreation{}, err
	}

	// Make POST request to generate API key
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/device/%s/apikey", baseURL, deviceGUID), bytes.NewBuffer(apiKeyJSON))
	if err != nil {
		return ApiKeyCreation{}, err
	}

	// Set Authorization header if access token available
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ApiKeyCreation{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// parse the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ApiKeyCreation{}, err
		}
		return ApiKeyCreation{}, fmt.Errorf("failed to register device: %s, Response: %s", resp.Status, body)
	}

	var apiKey ApiKeyCreation
	// Parse response body
	if err := json.NewDecoder(resp.Body).Decode(&apiKey); err != nil {
		return ApiKeyCreation{}, err
	}

	return apiKey, nil
}

// Function to store device credentials
func StoreCredentials(device Device, apiKey ApiKeyCreation, home string, filename string) error {
	// Create YAML file
	file, err := os.Create(filepath.Join(home, filename))
	if err != nil {
		return err
	}
	defer file.Close()

	// Write credentials to file
	_, err = fmt.Fprintf(file, "deviceID: %s\nname: %s\nAPIKey: %s\n", device.GUID, device.Name, apiKey.ApiKey)
	if err != nil {
		return err
	}

	return nil
}

// Function to load access token from credentials file
func LoadAccessToken(home string) (AuthResponse, error) {
	credentialsFile := filepath.Join(home, "credentials.yaml")
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return AuthResponse{}, fmt.Errorf("credentials file does not exist. Please login")
	}

	// Read the file
	fileContent, err := os.ReadFile(credentialsFile)
	if err != nil {
		return AuthResponse{}, err
	}

	// Unmarshal YAML
	var credentials map[string]string
	if err := yaml.Unmarshal(fileContent, &credentials); err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{
		AccessToken:  credentials["access_token"],
		RefreshToken: credentials["refresh_token"],
	}, nil
}

func Register(name string, baseURL string, home string, credentialsFileName string) error {
	// Attempt to load access token from credentials file
	authResponse, err := LoadAccessToken(home)
	if err != nil {
		fmt.Println("Failed to load access token:", err)
		return err
	}

	// If access token is available and not expired, use it to register device
	fmt.Println("Registering Device...")
	device, err := RegisterDevice(baseURL, name, authResponse.AccessToken, home)
	if err != nil {
		fmt.Println("Error registering device:", err)
		return err
	}

	fmt.Println("Generating API key...")
	apiKey, err := GenerateApiKey(baseURL, device.GUID, "Generated by portier CLI", authResponse.AccessToken, home)
	if err != nil {
		fmt.Println("Error generating API key:", err)
		return err
	}

	err = StoreCredentials(device, apiKey, home, credentialsFileName)
	if err != nil {
		fmt.Println("Error storing credentials:", err)
		return err
	}

	fmt.Println("Device registered and credentials stored successfully.")
	fmt.Printf("Device ID: \t%s\nAPI Key: \t%s\n", device.GUID, apiKey.ApiKey)
	return nil
}
