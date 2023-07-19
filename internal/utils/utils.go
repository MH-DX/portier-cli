package utils

import "os"

// Home returns the home directory of the current user withouth a trailing slash.
func Home() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	home = home + "/.portier"

	if customHome := os.Getenv("PORTIER_HOME"); customHome != "" {
		home = customHome
	}

	if _, err := os.Stat(home); err != nil {
		if os.IsNotExist(err) {
			err := os.Mkdir(home, 0700)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	return home, nil
}
