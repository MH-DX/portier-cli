package cmd

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	portier "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/taskcert"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
)

type taskEnrollOptions struct {
	APIURL         string
	TaskGUID       string
	TaskToken      string
	HomeFolderPath string
	OutputDir      string
}

func defaultTaskEnrollOptions() *taskEnrollOptions {
	home, err := utils.Home()
	if err != nil {
		log.Fatalf("could not get home directory: %v", err)
	}

	return &taskEnrollOptions{
		APIURL:         "https://api.portier.dev",
		HomeFolderPath: home,
	}
}

func newTaskEnrollCmd() *cobra.Command {
	o := defaultTaskEnrollOptions()

	cmd := &cobra.Command{
		Use:          "enroll",
		Aliases:      []string{"certificate"},
		Short:        "Enroll a short-lived TLS client certificate for a task",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE:         o.run,
	}

	cmd.Flags().StringVarP(&o.APIURL, "apiUrl", "a", o.APIURL, "base URL of the Portier backend")
	cmd.Flags().StringVarP(&o.TaskGUID, "task-guid", "g", o.TaskGUID, "task GUID")
	cmd.Flags().StringVarP(&o.TaskToken, "task-token", "t", o.TaskToken, "task token")
	cmd.Flags().StringVarP(&o.HomeFolderPath, "home", "H", o.HomeFolderPath, "home folder path")
	cmd.Flags().StringVarP(&o.OutputDir, "output-dir", "o", o.OutputDir, "directory where task certificate material will be stored")

	return cmd
}

func (o *taskEnrollOptions) run(cmd *cobra.Command, _ []string) error {
	o.TaskGUID = strings.TrimSpace(o.TaskGUID)
	o.TaskToken = strings.TrimSpace(o.TaskToken)
	o.APIURL = strings.TrimSpace(o.APIURL)

	if o.TaskGUID == "" {
		return fmt.Errorf("--task-guid is required")
	}
	if o.TaskToken == "" {
		return fmt.Errorf("--task-token is required")
	}
	if o.APIURL == "" {
		return fmt.Errorf("--apiUrl is required")
	}

	outputDir := strings.TrimSpace(o.OutputDir)
	if outputDir == "" {
		outputDir = filepath.Join(o.HomeFolderPath, "tasks", o.TaskGUID)
	}

	infoResponse, err := portier.GetTaskClientCertificateInfo(o.APIURL, o.TaskGUID, o.TaskToken)
	if err != nil {
		return err
	}

	notBefore, notAfter, err := validateTaskCertificateInfo(o.TaskGUID, infoResponse)
	if err != nil {
		return err
	}

	material, err := taskcert.Generate(infoResponse.TaskGUID, infoResponse.RequiredURISANs)
	if err != nil {
		return err
	}

	signResponse, err := portier.IssueTaskClientCertificate(o.APIURL, o.TaskGUID, o.TaskToken, string(material.CSRPEM))
	if err != nil {
		return err
	}

	if err := validateTaskCertificateResponse(infoResponse, signResponse); err != nil {
		return err
	}

	if err := taskcert.ValidateIssuedMaterials(
		[]byte(signResponse.Certificate),
		[]byte(signResponse.CertificateChain),
		material.PrivateKey,
		notBefore,
		notAfter,
		infoResponse.RequiredURISANs,
	); err != nil {
		return err
	}

	paths, err := taskcert.Store(outputDir, material.PrivateKeyPEM, []byte(signResponse.Certificate), []byte(signResponse.CertificateChain), &taskcert.Metadata{
		APIURL:          o.APIURL,
		TaskGUID:        signResponse.TaskGUID,
		TaskToken:       o.TaskToken,
		CustomerGUID:    signResponse.CustomerGUID,
		DeviceGUIDs:     append([]string(nil), signResponse.DeviceGUIDs...),
		Scope:           signResponse.Scope,
		NotBefore:       signResponse.NotBefore,
		NotAfter:        signResponse.NotAfter,
		RequiredURISANs: append([]string(nil), infoResponse.RequiredURISANs...),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Task certificate enrolled.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Scope: %s\n", signResponse.Scope)
	fmt.Fprintf(cmd.OutOrStdout(), "Devices: %d\n", len(signResponse.DeviceGUIDs))
	fmt.Fprintf(cmd.OutOrStdout(), "Validity: %s to %s\n", signResponse.NotBefore, signResponse.NotAfter)
	fmt.Fprintf(cmd.OutOrStdout(), "Stored in: %s\n", paths.OutputDir)
	fmt.Fprintf(cmd.OutOrStdout(), "Key: %s\n", paths.PrivateKeyPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Certificate: %s\n", paths.CertificatePath)
	fmt.Fprintf(cmd.OutOrStdout(), "Chain: %s\n", paths.CertificateChainPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Metadata: %s\n", paths.MetadataPath)

	return nil
}

func validateTaskCertificateInfo(expectedTaskGUID string, response *portier.TaskClientCertificateInfoResponse) (time.Time, time.Time, error) {
	if response == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response was empty")
	}
	if strings.TrimSpace(response.TaskGUID) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response did not include task_guid")
	}
	if response.TaskGUID != expectedTaskGUID {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response task_guid %q does not match requested task GUID %q", response.TaskGUID, expectedTaskGUID)
	}
	if strings.TrimSpace(response.Scope) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response did not include scope")
	}
	if len(response.RequiredURISANs) == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response did not include required_uri_sans")
	}

	notBefore, err := time.Parse(time.RFC3339, response.NotBefore)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response contained an invalid not_before value: %w", err)
	}
	notAfter, err := time.Parse(time.RFC3339, response.NotAfter)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response contained an invalid not_after value: %w", err)
	}
	if !notAfter.After(notBefore) {
		return time.Time{}, time.Time{}, fmt.Errorf("task certificate info response validity window is invalid")
	}

	return notBefore.UTC(), notAfter.UTC(), nil
}

func validateTaskCertificateResponse(infoResponse *portier.TaskClientCertificateInfoResponse, response *portier.TaskClientCertificateResponse) error {
	if response == nil {
		return fmt.Errorf("task certificate response was empty")
	}
	if response.TaskGUID != infoResponse.TaskGUID {
		return fmt.Errorf("task certificate response task_guid %q does not match the info response %q", response.TaskGUID, infoResponse.TaskGUID)
	}
	if response.CustomerGUID != infoResponse.CustomerGUID {
		return fmt.Errorf("task certificate response customer_guid %q does not match the info response %q", response.CustomerGUID, infoResponse.CustomerGUID)
	}
	if response.Scope != infoResponse.Scope {
		return fmt.Errorf("task certificate response scope %q does not match the info response %q", response.Scope, infoResponse.Scope)
	}
	if response.NotBefore != infoResponse.NotBefore {
		return fmt.Errorf("task certificate response not_before %q does not match the info response %q", response.NotBefore, infoResponse.NotBefore)
	}
	if response.NotAfter != infoResponse.NotAfter {
		return fmt.Errorf("task certificate response not_after %q does not match the info response %q", response.NotAfter, infoResponse.NotAfter)
	}
	if !equalStringSlices(response.DeviceGUIDs, infoResponse.DeviceGUIDs) {
		return fmt.Errorf("task certificate response device_guids do not match the info response")
	}
	if strings.TrimSpace(response.Certificate) == "" {
		return fmt.Errorf("task certificate response did not include certificate PEM")
	}
	if strings.TrimSpace(response.CertificateChain) == "" {
		return fmt.Errorf("task certificate response did not include certificate chain PEM")
	}

	return nil
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}
