package cmd

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ptls_create_cmd "github.com/mh-dx/portier-cli/cmd/ptls/create"
	portier "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/portier/ptls"
	"github.com/spf13/cobra"
)

// resolveTLSPaths retrieves TLS file locations from config.yaml if present.
// Missing values default to paths inside the provided home directory.
func resolveTLSPaths(home string) (cert, key, knownHosts string, err error) {
	cfg, err := config.LoadConfig(filepath.Join(home, "config.yaml"))
	if err != nil {
		return "", "", "", err
	}

	cert = cfg.PTLSConfig.CertFile
	key = cfg.PTLSConfig.KeyFile
	knownHosts = cfg.PTLSConfig.KnownHostsFile

	if cert == "" {
		cert = filepath.Join(home, "cert.pem")
	}
	if key == "" {
		key = filepath.Join(home, "key.pem")
	}
	if knownHosts == "" {
		knownHosts = filepath.Join(home, "known_hosts")
	}

	return cert, key, knownHosts, nil
}

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
			"--apiUrl", apiURL,
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

func ensureFingerprintUpToDate(cobraCmd *cobra.Command, home, apiURL, credentialsFile, certPath string) error {
	fmt.Fprintln(cobraCmd.OutOrStdout(), "Checking TLS certificate fingerprint registration (upload if needed)")
	creds, err := portier.LoadDeviceCredentials(home, filepath.Base(credentialsFile), apiURL)
	if err != nil {
		return err
	}

	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return fmt.Errorf("failed to decode certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	certManager := ptls.NewPTLSCertificateManager()
	fp, err := certManager.GetFingerprint(cert)
	if err != nil {
		return err
	}

	fps, err := portier.GetFingerprint(home, apiURL, []string{creds.DeviceID})
	if err != nil {
		return err
	}

	if existing, ok := fps[creds.DeviceID]; ok && existing == fp {
		fmt.Fprintln(cobraCmd.OutOrStdout(), "Fingerprint already registered")
		return nil
	}

	if err := portier.UploadFingerprint(home, apiURL, creds.DeviceID, fp); err != nil {
		return err
	}
	fmt.Fprintln(cobraCmd.OutOrStdout(), "Fingerprint uploaded")
	return nil
}
