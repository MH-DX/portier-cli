package utils

import (
	"encoding/json"
	"net/url"
	"os"
)

// Home returns the home directory of the current user withouth a trailing slash.
func Home() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	home += "/.portier"

	if customHome := os.Getenv("PORTIER_HOME"); customHome != "" {
		home = customHome
	}

	if _, err := os.Stat(home); err != nil {
		if os.IsNotExist(err) {
			perm := os.FileMode(0o700)
			err := os.Mkdir(home, perm)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	return home, nil
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

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
