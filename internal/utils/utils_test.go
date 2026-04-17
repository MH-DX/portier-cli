package utils

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHomeUsesPortierHomeWhenHOMEUnset(t *testing.T) {
	t.Setenv("HOME", "")
	customHome := filepath.Join(t.TempDir(), "custom-portier-home")
	t.Setenv("PORTIER_HOME", customHome)

	home, err := resolveHome(func() (*user.User, error) {
		return nil, errors.New("lookupCurrentUser should not be called")
	})
	require.NoError(t, err)
	require.Equal(t, customHome, home)

	home, err = Home()
	require.NoError(t, err)
	require.Equal(t, customHome, home)

	info, err := os.Stat(home)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestHomeFallsBackToCurrentUserWhenHOMEUnset(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("PORTIER_HOME", "")

	fakeUserHome := filepath.Join(t.TempDir(), "fake-user-home")

	home, err := home(func() (*user.User, error) {
		return &user.User{HomeDir: fakeUserHome}, nil
	})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(fakeUserHome, ".portier"), home)

	info, err := os.Stat(home)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestHomeReturnsHelpfulErrorWhenNoHomeCanBeResolved(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("PORTIER_HOME", "")

	_, err := home(func() (*user.User, error) {
		return &user.User{}, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "HOME is not defined")
}
