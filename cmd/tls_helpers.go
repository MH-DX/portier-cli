package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ptls_create_cmd "github.com/mh-dx/portier-cli/cmd/ptls/create"
	"github.com/spf13/cobra"
)

// ensureTLSCertificate checks if certificate and key exist. If not, it prompts
// the user to create them using the ptls create command. The paths should be
// absolute paths to the cert and key files.
func ensureTLSCertificate(cobraCmd *cobra.Command, home, credentialsFile, apiURL, certPath, keyPath, knownHosts string) error {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil
		}
	}

	fmt.Fprintf(cobraCmd.OutOrStdout(), "No TLS certificate found at %s.\n", certPath)
	fmt.Fprintln(cobraCmd.OutOrStdout(), "Creating a certificate improves security and is recommended.")
	fmt.Fprint(cobraCmd.OutOrStdout(), "Create one now? [Y/n] ")

	reader := bufio.NewReader(cobraCmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" || strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
		createCmd := ptls_create_cmd.NewCreatecmd()
		createCmd.SetIn(cobraCmd.InOrStdin())
		createCmd.SetOut(cobraCmd.OutOrStdout())
		args := []string{
			"--home", home,
			"--credentials", filepath.Base(credentialsFile),
			"--cert", certPath,
			"--key", keyPath,
			"--knownHosts", knownHosts,
			"--apiUrl", strings.TrimSuffix(apiURL, "/api"),
		}
		createCmd.SetArgs(args)
		if err := createCmd.Execute(); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(cobraCmd.OutOrStdout(), "Skipping certificate creation. TLS connections may fail.")
	}

	return nil
}
