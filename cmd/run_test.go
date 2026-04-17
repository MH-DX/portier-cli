package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunCommandDefinesUsableConfigAndApiTokenFlags(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PORTIER_HOME", homeDir)

	cmd, err := newRunCmd()
	require.NoError(t, err)

	configPath := filepath.Join(homeDir, "custom-config.yaml")
	apiTokenPath := filepath.Join(homeDir, "custom-credentials.yaml")

	require.NoError(t, cmd.Flags().Parse([]string{
		"--config", configPath,
		"--apiToken", apiTokenPath,
	}))
	require.NotNil(t, cmd.Flags().Lookup("config"))
	require.NotNil(t, cmd.Flags().Lookup("apiToken"))
	require.Nil(t, cmd.Flags().Lookup("config file"))
	require.Nil(t, cmd.Flags().Lookup("apiToken file"))

	options := &runOptions{}
	require.NoError(t, options.parseArgs(cmd, nil))
	require.Equal(t, configPath, options.ConfigFile)
	require.Equal(t, apiTokenPath, options.ApiTokenFile)
}
