package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGlobalFlagsStripsRecognizedFlags(t *testing.T) {
	options, args, err := parseGlobalFlags([]string{
		"-logfile", "/root/.portier/portier-cli.log",
		"service", "run",
		"-c", "/root/.portier/config.yaml",
		"-t", "/root/.portier/credentials_device.yaml",
		"-l", "/root/.portier/portier-cli.log",
	})
	require.NoError(t, err)

	require.Equal(t, "/root/.portier/portier-cli.log", options.logfile)
	require.Equal(t, []string{
		"service", "run",
		"-c", "/root/.portier/config.yaml",
		"-t", "/root/.portier/credentials_device.yaml",
		"-l", "/root/.portier/portier-cli.log",
	}, args)
}

func TestParseGlobalFlagsLeavesSubcommandArgsUntouched(t *testing.T) {
	options, args, err := parseGlobalFlags([]string{
		"service", "run",
		"-c", "/root/.portier/config.yaml",
	})
	require.NoError(t, err)

	require.Empty(t, options.logfile)
	require.Equal(t, []string{
		"service", "run",
		"-c", "/root/.portier/config.yaml",
	}, args)
}
