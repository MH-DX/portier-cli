package webwizard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	portier "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/skratchdot/open-golang/open"
	"gopkg.in/yaml.v3"
)

//go:embed templates/* static/*
var staticFiles embed.FS

// WizardStep represents the current step in the setup wizard
type WizardStep string

const (
	StepWelcome    WizardStep = "welcome"
	StepLogin      WizardStep = "login"
	StepRegister   WizardStep = "register"
	StepService    WizardStep = "service"
	StepComplete   WizardStep = "complete"
)

// WizardMessage represents a message sent to the client
type WizardMessage struct {
	Type    string      `json:"type"`
	Step    WizardStep  `json:"step,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Success bool        `json:"success,omitempty"`
}

// LoginData contains the login flow information
type LoginData struct {
	UserCode                 string `json:"userCode"`
	VerificationURL          string `json:"verificationUrl"`
	VerificationURLComplete  string `json:"verificationUrlComplete"`
	QRCodeURL                string `json:"qrCodeUrl"`
}

// DeviceRegistrationRequest contains device registration info
type DeviceRegistrationRequest struct {
	DeviceName string `json:"deviceName"`
}

// WizardServer manages the setup wizard web server
type WizardServer struct {
	server    *http.Server
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	broadcast chan WizardMessage
	ctx       context.Context
	cancel    context.CancelFunc
	port      int
}

// NewWizardServer creates a new wizard server
func NewWizardServer() *WizardServer {
	ctx, cancel := context.WithCancel(context.Background())

	ws := &WizardServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local development
			},
		},
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan WizardMessage),
		ctx:       ctx,
		cancel:    cancel,
	}

	return ws
}

// Start starts the wizard server and opens the browser
func (ws *WizardServer) Start() error {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %v", err)
	}
	defer listener.Close()

	ws.port = listener.Addr().(*net.TCPAddr).Port

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/ws", ws.handleWebSocket)
	mux.HandleFunc("/api/login", ws.handleLogin)
	mux.HandleFunc("/api/register", ws.handleRegister)
	mux.HandleFunc("/api/service/install", ws.handleServiceInstall)
	mux.HandleFunc("/api/service/start", ws.handleServiceStart)
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	ws.server = &http.Server{
		Handler: mux,
	}

	// Start message broadcaster
	go ws.handleMessages()

	// Start server
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", ws.port))
		if err != nil {
			log.Printf("Failed to listen on port %d: %v", ws.port, err)
			return
		}

		log.Printf("Starting wizard server on http://127.0.0.1:%d", ws.port)
		if err := ws.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Open browser after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		url := fmt.Sprintf("http://127.0.0.1:%d", ws.port)
		log.Printf("Opening browser at %s", url)
		if err := open.Run(url); err != nil {
			log.Printf("Failed to open browser: %v", err)
			log.Printf("Please manually open: %s", url)
		}
	}()

	return nil
}

// Stop stops the wizard server
func (ws *WizardServer) Stop() error {
	ws.cancel()
	if ws.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return ws.server.Shutdown(ctx)
	}
	return nil
}

// Wait waits for the wizard to complete or be cancelled
func (ws *WizardServer) Wait() {
	<-ws.ctx.Done()
}

// handleIndex serves the main wizard page
func (ws *WizardServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Failed to load wizard page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

// handleWebSocket handles WebSocket connections
func (ws *WizardServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ws.clients[conn] = true
	defer delete(ws.clients, conn)

	// Send welcome message
	ws.sendToClient(conn, WizardMessage{
		Type: "step",
		Step: StepWelcome,
	})

	// Read messages from client
	for {
		var msg WizardMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle client messages
		ws.handleClientMessage(conn, msg)
	}
}

// handleMessages handles broadcasting messages to all clients
func (ws *WizardServer) handleMessages() {
	for {
		select {
		case msg := <-ws.broadcast:
			for client := range ws.clients {
				ws.sendToClient(client, msg)
			}
		case <-ws.ctx.Done():
			return
		}
	}
}

// sendToClient sends a message to a specific client
func (ws *WizardServer) sendToClient(conn *websocket.Conn, msg WizardMessage) {
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Failed to send message to client: %v", err)
		conn.Close()
		delete(ws.clients, conn)
	}
}

// handleClientMessage handles messages from clients
func (ws *WizardServer) handleClientMessage(conn *websocket.Conn, msg WizardMessage) {
	switch msg.Type {
	case "nextStep":
		switch msg.Step {
		case StepLogin:
			ws.sendToClient(conn, WizardMessage{
				Type: "step",
				Step: StepLogin,
			})
		case StepRegister:
			ws.sendToClient(conn, WizardMessage{
				Type: "step",
				Step: StepRegister,
			})
		case StepService:
			ws.sendToClient(conn, WizardMessage{
				Type: "step",
				Step: StepService,
			})
		case StepComplete:
			ws.sendToClient(conn, WizardMessage{
				Type: "step",
				Step: StepComplete,
			})
		}
	case "exit":
		ws.cancel()
	}
}

// handleLogin handles the login API endpoint
func (ws *WizardServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Start login process in a goroutine
	go func() {
		if err := ws.performWizardLogin(); err != nil {
			ws.broadcast <- WizardMessage{
				Type:  "loginResult",
				Error: fmt.Sprintf("Login failed: %v", err),
			}
			return
		}

		ws.broadcast <- WizardMessage{
			Type:    "loginResult",
			Success: true,
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handleRegister handles the device registration API endpoint
func (ws *WizardServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeviceRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DeviceName == "" {
		http.Error(w, "Device name is required", http.StatusBadRequest)
		return
	}

	// Start registration process in a goroutine
	go func() {
		home, err := utils.Home()
		if err != nil {
			ws.broadcast <- WizardMessage{
				Type:  "registerResult",
				Error: fmt.Sprintf("Failed to get home directory: %v", err),
			}
			return
		}

		if err := portier.Register(req.DeviceName, "https://api.portier.dev/api", home, "credentials_device.yaml"); err != nil {
			ws.broadcast <- WizardMessage{
				Type:  "registerResult",
				Error: fmt.Sprintf("Registration failed: %v", err),
			}
			return
		}

		ws.broadcast <- WizardMessage{
			Type:    "registerResult",
			Success: true,
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handleServiceInstall handles the service installation API endpoint
func (ws *WizardServer) handleServiceInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate service installation (this would need to be implemented based on the existing service command)
	go func() {
		// TODO: Integrate with actual service installation logic
		time.Sleep(2 * time.Second) // Simulate installation time

		ws.broadcast <- WizardMessage{
			Type:    "serviceInstallResult",
			Success: true,
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handleServiceStart handles the service start API endpoint
func (ws *WizardServer) handleServiceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate service start (this would need to be implemented based on the existing service command)
	go func() {
		// TODO: Integrate with actual service start logic
		time.Sleep(1 * time.Second) // Simulate start time

		ws.broadcast <- WizardMessage{
			Type:    "serviceStartResult",
			Success: true,
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// CheckCredentialsExist checks if device credentials already exist
func CheckCredentialsExist() (bool, error) {
	home, err := utils.Home()
	if err != nil {
		return false, err
	}

	credentialsFile := filepath.Join(home, "credentials_device.yaml")
	_, err = os.Stat(credentialsFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// performWizardLogin performs the login process with wizard-specific messaging
func (ws *WizardServer) performWizardLogin() error {
	home, err := utils.Home()
	if err != nil {
		return err
	}

	// define the endpoint
	deviceURL := "https://portier-spider.eu.auth0.com/oauth/device/code"

	// define the data
	data := url.Values{}
	data.Set("client_id", "jE4nxZ6miTLOS4OWGLzoyVlOnkxAiHqb")
	data.Set("scope", "openid email profile offline_access")
	data.Set("audience", "https://api.portier.dev")

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

	// Send login data to the client
	ws.broadcast <- WizardMessage{
		Type: "loginResult",
		Data: LoginData{
			UserCode:                userCode,
			VerificationURL:         verificationURL,
			VerificationURLComplete: verificationURLComplete,
			QRCodeURL:               verificationURLComplete,
		},
	}

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

		return nil
	}
}