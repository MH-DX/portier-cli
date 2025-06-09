package portier

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WhoAmI calls the /spider/whoami endpoint using the provided api key
// and returns the device GUID.
func WhoAmI(baseURL, apiKey string) (uuid.UUID, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := fmt.Sprintf("%s/spider/whoami", baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("Authorization", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.Nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, fmt.Errorf("whoami failed: %s - %s", resp.Status, string(body))
	}

	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return uuid.Nil, err
	}

	var guidStr string
	if v, ok := m["GUID"].(string); ok {
		guidStr = v
	} else if v, ok := m["guid"].(string); ok {
		guidStr = v
	} else if v, ok := m["deviceGUID"].(string); ok {
		guidStr = v
	}
	if guidStr == "" {
		return uuid.Nil, fmt.Errorf("GUID not found in whoami response")
	}
	guid, err := uuid.Parse(guidStr)
	if err != nil {
		return uuid.Nil, err
	}
	return guid, nil
}
