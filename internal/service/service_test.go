package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAlreadyInstalledError(t *testing.T) {
	require.True(t, isAlreadyInstalledError(errors.New("Init already exists: /etc/systemd/system/portier-cli.service")))
	require.True(t, isAlreadyInstalledError(errors.New("service portier-cli already exists")))
	require.False(t, isAlreadyInstalledError(errors.New("permission denied")))
	require.False(t, isAlreadyInstalledError(nil))
}
