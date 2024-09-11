package portier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marinator86/portier-cli/internal/utils"
	"gopkg.in/yaml.v3"
)

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
}

// Login uses device flow to log the user in. It uses a file to store the user's jwt token in ~/.portier/credentials.json
// Device flow is a way to authenticate users on devices that do not have a browser.
// See https://tools.ietf.org/html/rfc8628
func Login() error {
	home, err := utils.Home()
	if err != nil {
		return err
	}

	// define the endpoint
	deviceURL := "https://portier-spider.eu.auth0.com/oauth/device/code"

	// define the data
	data := url.Values{}
	data.Set("client_id", "jE4nxZ6miTLOS4OWGLzoyVlOnkxAiHqb")
	data.Set("scope", "openid email offline_access")

	// create the request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, deviceURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	// check the response code
	if resp.StatusCode != 200 {
		return fmt.Errorf("error: %s", result["error_description"])
	}

	// extract the device code and user code
	deviceCode, ok := result["device_code"].(string)
	if !ok {
		return fmt.Errorf("device_code not found in response")
	}
	userCode, ok := result["user_code"].(string)
	if !ok {
		return fmt.Errorf("user_code not found in response")
	}
	verificationURL, _ := result["verification_uri"].(string)
	verificationURLComplete, _ := result["verification_uri_complete"].(string)
	pollingInterval, _ := result["interval"].(float64)

	// log a nice multiline message to the user repeating the instructions
	// to log in
	log.Println(`

Logging in to portier.dev
-------------------------
Steps:

1. Open the following link in your browser to authenticate:
` + verificationURLComplete + `

2. Alternatively, open ` + verificationURL + ` in your browser and enter the code ` + userCode + `

Waiting for user to log in...
`)

	// poll the token endpoint until the user has logged in
	for {
		// define the endpoint
		tokenURL := "https://portier-spider.eu.auth0.com/oauth/token"

		// define the data
		data := url.Values{}
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		data.Set("client_id", "jE4nxZ6miTLOS4OWGLzoyVlOnkxAiHqb")
		data.Set("device_code", deviceCode)

		// create the request
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		// perform the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// parse the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}

		// check if the user has not logged in yet, if so wait
		if resp.StatusCode != 200 {
			error := result["error"].(string)
			if error == "authorization_pending" {
				time.Sleep(time.Duration(pollingInterval) * time.Second)
				continue
			}
			if error == "slow_down" {
				pollingInterval = pollingInterval + 1
				time.Sleep(time.Duration(pollingInterval) * time.Second)
				continue
			}
			return fmt.Errorf("%s", result["error_description"])
		}

		// extract the access token and refresh token
		log.Printf("Log in successful, storing access token in ~/.portier/credentials.yaml")
		accessToken, ok := result["access_token"].(string)
		if !ok {
			return fmt.Errorf("access_token not found in response")
		}
		refreshToken, ok := result["refresh_token"].(string)
		if !ok {
			return fmt.Errorf("refresh_token not found in response")
		}

		// store the access token in the credentials file
		credentials := map[string]string{
			"stored_at":     time.Now().Format(time.RFC3339),
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		}

		yamlCredentials, err := yaml.Marshal(credentials)
		if err != nil {
			return err
		}
		// write the yaml to the file
		credentialsFile := filepath.Join(home, "credentials.yaml")
		err = os.WriteFile(credentialsFile, yamlCredentials, 0o644)
		if err != nil {
			return err
		}

		log.Println("Login successful.")
		return nil
	}
}
