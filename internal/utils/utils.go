package utils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Home returns the Portier home directory without a trailing slash.
func Home() (string, error) {
	return home(user.Current)
}

func home(currentUserLookup func() (*user.User, error)) (string, error) {
	home, err := resolveHome(currentUserLookup)
	if err != nil {
		return "", err
	}

	perm := os.FileMode(0o700)
	if err := os.MkdirAll(home, perm); err != nil {
		return "", err
	}

	return home, nil
}

func resolveHome(currentUserLookup func() (*user.User, error)) (string, error) {
	if customHome := strings.TrimSpace(os.Getenv("PORTIER_HOME")); customHome != "" {
		return customHome, nil
	}

	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return filepath.Join(home, ".portier"), nil
	}

	currentUser, err := currentUserLookup()
	if err != nil {
		return "", fmt.Errorf("HOME is not defined and current user lookup failed: %w", err)
	}

	if strings.TrimSpace(currentUser.HomeDir) == "" {
		return "", fmt.Errorf("HOME is not defined and current user home directory is empty")
	}

	return filepath.Join(currentUser.HomeDir, ".portier"), nil
}

type YAMLURL struct {
	*url.URL
}

func (j *YAMLURL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	url, err := url.Parse(s)
	j.URL = url
	return err
}

func (j YAMLURL) MarshalYAML() (interface{}, error) {
	return j.String(), nil
}

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
