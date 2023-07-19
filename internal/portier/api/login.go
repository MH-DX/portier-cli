package portier

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/marinator86/portier-cli/internal/utils"
	"gopkg.in/yaml.v3"
)

type Verifier struct {
	// The verifier is a random string that is used to generate the challenge
	// It is a url-safe base64 encoded string
	// See https://tools.ietf.org/html/rfc7636#section-4.1
	Verifier  string
	Challenge string
}

type AuthRequest struct {
	// The authorization URL is the URL that the user is redirected to
	AuthURL string
	// The client ID is a random string that is used to identify the client
	ClientID string
	// The redirect URI is the URI that the user is redirected to after login
	RedirectURI string
	// The scope is a list of scopes that the client requests
	Scope string
	// The state is a random string that is used to prevent CSRF attacks
	State string
	// The nonce is a random string that is used to prevent replay attacks
	Nonce string
	// The code challenge is a hash of the verifier
	Verifier Verifier
	// The code challenge method is the method that is used to generate the code challenge
	CodeChallengeMethod string
}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
}

func NewVerifier() (Verifier, error) {
	// create a new random url-safe base64 encoded string
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return Verifier{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	s256 := sha256.New()
	s256.Write([]byte(verifier))
	hash := s256.Sum(nil)

	codeChallenge := base64.RawURLEncoding.EncodeToString(hash)
	return Verifier{
		Verifier:  verifier,
		Challenge: codeChallenge,
	}, nil
}

func NewAuthRequest(verifier Verifier) (AuthRequest, error) {
	// create a new auth request
	// url encode the verifier
	var authRequest = AuthRequest{
		AuthURL:             "https://dev-eky3r60icvtk7a30.eu.auth0.com/authorize",
		ClientID:            "jj2dZqnIaL682XQiVIalh05diy8cha99",
		RedirectURI:         "http://localhost:5555/callback",
		Scope:               url.QueryEscape("openid email offline_access"),
		State:               url.QueryEscape("portier-cli"),
		Nonce:               url.QueryEscape("portier-cli"),
		Verifier:            verifier,
		CodeChallengeMethod: "S256",
	}
	return authRequest, nil
}

func CreateAuthUrl(authRequest AuthRequest) string {
	return fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=%s&state=%s&nonce=%s&code_challenge=%s&code_challenge_method=%s&response_type=code", authRequest.AuthURL, authRequest.ClientID, authRequest.RedirectURI, authRequest.Scope, authRequest.State, authRequest.Nonce, authRequest.Verifier.Challenge, authRequest.CodeChallengeMethod)
}

func WaitForAuthCode(authRequest AuthRequest) (string, error) {
	// start a http server on localhost:5555
	// wait for the user to login
	// return the authorization code
	var server http.Server
	var code string
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// This is where you would handle the callback and extract the code
		// For now, we'll just print the entire request URL
		fmt.Fprintf(w, "Thanks for using portier.dev! You can now close this window.")

		// You can get individual query parameters like this:
		code = r.URL.Query().Get("code")
		// Stop the server after handling the callback
		go func() {
			<-time.After(100 * time.Millisecond) // delay to allow the HTTP response
			if err := server.Shutdown(context.Background()); err != nil {
				log.Fatal(err)
			}
		}()
	})

	server = http.Server{Addr: ":5555"}

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	return code, nil
}

func ExchangeAuthCode(authRequest AuthRequest, authCode string) (AuthResponse, error) {
	// define the endpoint
	tokenURL := "https://dev-eky3r60icvtk7a30.eu.auth0.com/oauth/token"

	// define the data
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", authRequest.ClientID)
	data.Set("code_verifier", authRequest.Verifier.Verifier)
	data.Set("code", authCode)
	data.Set("redirect_uri", authRequest.RedirectURI)

	// create the request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return AuthResponse{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return AuthResponse{}, err
	}
	defer resp.Body.Close()

	// parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AuthResponse{}, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return AuthResponse{}, err
	}

	// TODO check the nonce in the id token

	// extract the access token and refresh token
	accessToken, ok := result["access_token"].(string)
	if !ok {
		return AuthResponse{}, fmt.Errorf("access_token not found in response")
	}
	refreshToken, ok := result["refresh_token"].(string)
	if !ok {
		return AuthResponse{}, fmt.Errorf("refresh_token not found in response")
	}

	return AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func StoreAccessToken(authResponse AuthResponse, home string) error {
	// store the access token in ~/.portier/credentials.json
	// create the file if it does not exist
	var credentialsFile = fmt.Sprintf("%s/credentials.json", home)
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		// create the file
		_, err := os.Create(credentialsFile)
		if err != nil {
			return err
		}
	}
	// store  the authResponse in the file as yaml
	credentials := map[string]string{
		"stored_at":     time.Now().Format(time.RFC3339),
		"access_token":  authResponse.AccessToken,
		"refresh_token": authResponse.RefreshToken,
	}

	yamlCredentials, err := yaml.Marshal(credentials)
	if err != nil {
		return err
	}
	// write the yaml to the file
	err = os.WriteFile(credentialsFile, yamlCredentials, 0644)
	if err != nil {
		return err
	}
	return nil
}

// login is a function that logs the user in. It uses a file to store the user's jwt token in ~/.portier/credentials.json
// As interactive login we offer Authorization Code Flow with Proof Key for Code Exchange (PKCE).
// This is the recommended way to authenticate users.
// See https://tools.ietf.org/html/rfc7636
func Login() error {
	var home, err = utils.Home()
	if err != nil {
		return err
	}

	// Create a new PKCE verifier and challenge
	verifier, err := NewVerifier()
	if err != nil {
		return err
	}

	// Create a new authorization request
	authRequest, err := NewAuthRequest(verifier)
	if err != nil {
		return err
	}

	// Open the browser and let the user login
	url := CreateAuthUrl(authRequest)
	fmt.Printf("Open the following link in your browser:\n%s\n", url)

	// Wait for the user to login and get the authorization code
	authCode, err := WaitForAuthCode(authRequest)
	if err != nil {
		return err
	}

	// Exchange the authorization code for an access token
	authResponse, err := ExchangeAuthCode(authRequest, authCode)
	if err != nil {
		return err
	}

	// Store the access token in the credentials file
	err = StoreAccessToken(authResponse, home)
	if err != nil {
		return err
	}

	fmt.Println("You are now logged in.")
	return nil
}
