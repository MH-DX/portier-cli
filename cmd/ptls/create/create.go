package ptls_create_cmd

import (
	"fmt"
	"log"
	"os"

	api "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/ptls"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

// TODO: Implement the tls commands to create, update and upload TLS certificate FPs to the server
// TODO add command to trust a peer device by adding its TLS cert FP to known_hosts
// TODO add check for existing TLS configs, auto-add if not present

type tlsCreateOptions struct {
	HomeFolderPath      string
	CredentialsFileName string
	CertPath            string
	KeyPath             string
	KnownHostsFilePath  string
	UploadFingerprint   bool
	ApiURL              string
}

func defaultTLSOptions() *tlsCreateOptions {
	home, err := utils.Home()
	if err != nil {
		log.Fatalf("could not get home directory: %v", err)
	}

	return &tlsCreateOptions{
		HomeFolderPath:      home,
		CredentialsFileName: "credentials_device.yaml",
		CertPath:            fmt.Sprintf("%s/cert.pem", home),
		KeyPath:             fmt.Sprintf("%s/key.pem", home),
		KnownHostsFilePath:  fmt.Sprintf("%s/known_hosts", home),
		UploadFingerprint:   true,
		ApiURL:              "https://api.portier.dev/api",
	}
}

func NewCreatecmd() *cobra.Command {
	o := defaultTLSOptions()

	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create a TLS certificate, store it in the local filesystem, upload the fingerprint to the server",
		SilenceUsage: true,
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.CertPath, "cert", "C", o.CertPath, "path to the certificate file in PEM format")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.CredentialsFileName, "credentials", "c", o.CredentialsFileName, "credentials file name in home folder")
	cmd.Flags().StringVarP(&o.KeyPath, "key", "k", o.KeyPath, "path to the key file in PEM format")
	cmd.Flags().StringVarP(&o.KnownHostsFilePath, "knownHosts", "f", o.KnownHostsFilePath, "path to the known_hosts file")
	cmd.Flags().BoolVarP(&o.UploadFingerprint, "uploadFingerprint", "u", o.UploadFingerprint, "if set, will upload the certificate's fingerprint to the server")
	cmd.Flags().StringVarP(&o.ApiURL, "apiUrl", "a", o.ApiURL, "base URL of the portier API")

	return cmd
}

func (o *tlsCreateOptions) run(cmd *cobra.Command, args []string) error {

	// load credentials.yaml file
	// get the device ID from the credentials.yaml file
	credentials, err := api.LoadDeviceCredentials(o.HomeFolderPath, o.CredentialsFileName, o.ApiURL)
	if err != nil {
		return err
	}

	certManager := ptls.NewPTLSCertificateManager()
	cert, priv, err := certManager.CreateCertificate(credentials.DeviceID)
	if err != nil {
		return err
	}
	fmt.Println("Self-signed certificate created:")
	fmt.Println()
	fmt.Printf("CommonName: \t%s\n", cert.Subject)
	fmt.Printf("NotBefore: \t%s\n", cert.NotBefore)
	fmt.Printf("NotAfter: \t%s\n", cert.NotAfter)
	fmt.Printf("SerialNumber: \t%s\n", cert.SerialNumber)
	fmt.Printf("Algorithm: \t%s\n", cert.SignatureAlgorithm)
	fmt.Println()

	certPEM, keyPEM, err := certManager.ConvertCertificateToPEM(cert, priv)
	if err != nil {
		return err
	}

	// write cert and key to files
	if err := os.WriteFile(o.CertPath, certPEM, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(o.KeyPath, keyPEM, 0644); err != nil {
		return err
	}
	fmt.Printf("Certificate written to \t%s\n", o.CertPath)
	fmt.Printf("Private key written to \t%s\n", o.KeyPath)
	fmt.Println()

	if o.UploadFingerprint {
		fp, err := certManager.GetFingerprint(cert)
		if err != nil {
			return err
		}
		fmt.Println("The SHA-256 fingerprint of the certificate will be used to authenticate the device when it connects to other devices")
		fmt.Printf("Fingerprint: %s\n", fp)
		fmt.Println()
		fmt.Println("To allow this device to securely connect to another device, add the following line to the known_hosts file of the other device:")
		fmt.Printf("%s: %s\n", credentials.DeviceID, fp)
		fmt.Println("The known_hosts file is usually located at ~/.portier/known_hosts")
		fmt.Println()
		fmt.Printf("Hint: You can also use the trust-command on the other device:\n")
		fmt.Printf("> portier-cli tls trust -i %s\n", credentials.DeviceID)
		fmt.Println("This way, portier-cli will update the known_hosts file for you")
		fmt.Println()
		fmt.Println("Uploading fingerprint to the server (it is public)")
		err = api.UploadFingerprint(o.HomeFolderPath, o.ApiURL, credentials.DeviceID, fp)
		if err != nil {
			return err
		}
		fmt.Println("Fingerprint uploaded successfully")
		fmt.Println()
	}

	fmt.Println("Done")

	return nil
}
