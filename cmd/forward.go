package cmd

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"
	ptls_trust_cmd "github.com/mh-dx/portier-cli/cmd/ptls/trust"
	portierapi "github.com/mh-dx/portier-cli/internal/portier/api"
	"github.com/mh-dx/portier-cli/internal/portier/application"
	"github.com/mh-dx/portier-cli/internal/portier/config"
	"github.com/mh-dx/portier-cli/internal/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type forwardOptions struct {
	NoTLS        bool
	NoPersist    bool
	ConfigFile   string
	ApiTokenFile string
	ApiURL       string
}

func defaultForwardOptions() (*forwardOptions, error) {
	home, err := utils.Home()
	if err != nil {
		return nil, err
	}
	return &forwardOptions{
		ConfigFile:   filepath.Join(home, "config.yaml"),
		ApiTokenFile: filepath.Join(home, "credentials_device.yaml"),
		ApiURL:       "https://api.portier.dev/api",
	}, nil
}

func newForwardCmd() (*cobra.Command, error) {
	o, err := defaultForwardOptions()
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:   "forward <remoteName>:<remotePort>-><localName>:<localPort>",
		Short: "Forward a port from a remote device to a local port",
		Long:  "localName is optional and defaults to localhost if omitted (e.g. dev:80->8080)",
		Args:  cobra.ExactArgs(1),
		RunE:  o.run,
	}
	cmd.Flags().BoolVar(&o.NoTLS, "no-tls", false, "disable TLS encryption")
	cmd.Flags().BoolVar(&o.NoPersist, "no-persist", false, "do not store forwarding in config, means this forwarding won't be initialized after restart")
	cmd.Flags().StringVar(&o.ApiURL, "apiUrl", o.ApiURL, "base URL of the portier API")
	cmd.Flags().StringVar(&o.ConfigFile, "config", o.ConfigFile, "config file")
	cmd.Flags().StringVar(&o.ApiTokenFile, "apiToken", o.ApiTokenFile, "api token file")

	return cmd, nil
}

func (o *forwardOptions) parseSpec(spec string) (remoteDeviceName, remotePort, localHostName, localPort string, err error) {
	parts := strings.Split(spec, "->")
	if len(parts) != 2 {
		err = fmt.Errorf("invalid format")
		return
	}
	r := strings.SplitN(strings.TrimSpace(parts[0]), ":", 2)
	if len(r) != 2 {
		err = fmt.Errorf("invalid format")
		return
	}
	remoteDeviceName, remotePort = r[0], r[1]

	localPart := strings.TrimSpace(parts[1])
	if strings.Contains(localPart, ":") {
		l := strings.SplitN(localPart, ":", 2)
		if len(l) != 2 {
			err = fmt.Errorf("invalid format")
			return
		}
		localHostName, localPort = l[0], l[1]
	} else {
		localHostName = "localhost"
		localPort = localPart
	}
	return
}

func (o *forwardOptions) run(cmd *cobra.Command, args []string) error {
	remoteName, remotePort, localHostName, localPort, err := o.parseSpec(args[0])
	if err != nil {
		return err
	}

	home := filepath.Dir(o.ApiTokenFile)
	remoteID, err := portierapi.GetDeviceByName(home, o.ApiURL, remoteName)
	if err != nil {
		return err
	}

	// add log statement to show the remote ID
	fmt.Fprintf(cmd.OutOrStdout(), "Device %s has ID %s\n", remoteName, remoteID)

	cfg, err := config.LoadConfig(o.ConfigFile)
	if err != nil {
		return err
	}
	creds, err := config.LoadApiToken(o.ApiTokenFile)
	if err != nil {
		return err
	}

	if !o.NoTLS {
		cert := cfg.PTLSConfig.CertFile
		key := cfg.PTLSConfig.KeyFile
		known := cfg.PTLSConfig.KnownHostsFile
		if err := ensureTLSCertificate(cmd, filepath.Dir(o.ApiTokenFile), o.ApiTokenFile, o.ApiURL, cert, key, known); err != nil {
			return err
		}
	}

	tlsEnabled := !o.NoTLS
	if tlsEnabled {
		khPath := cfg.PTLSConfig.KnownHostsFile
		kh := make(map[string]string)
		if data, err := os.ReadFile(khPath); err == nil {
			yaml.Unmarshal(data, &kh)
		}
		if _, ok := kh[remoteID]; !ok {
			fmt.Fprintf(cmd.OutOrStdout(), "Device %s is not trusted for TLS encrypted communication. Please confirm downloading its fingerprint [Y/n] ", remoteName)
			reader := bufio.NewReader(cmd.InOrStdin())
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(answer)
			if strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
				return fmt.Errorf("TLS enabled, but the remote device ist not trusted. Aborting")
			} else {
				trustCmd := ptls_trust_cmd.NewTrustcmd()
				trustCmd.SetIn(cmd.InOrStdin())
				trustCmd.SetOut(cmd.OutOrStdout())
				args := []string{
					"--home", filepath.Dir(o.ApiTokenFile),
					"--knownHosts", khPath,
					"--apiUrl", o.ApiURL,
					"--credentials", filepath.Base(o.ApiTokenFile),
					"--ids", remoteID,
				}
				trustCmd.SetArgs(args)
				if err := trustCmd.Execute(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Device %s trusted. The remote device might need to trust this device as well.\n", remoteName)
			}
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Warning: remote device must allow connections without TLS")
	}

	remoteURL, _ := url.Parse(fmt.Sprintf("tcp://localhost:%s", remotePort))
	localURL, _ := url.Parse(fmt.Sprintf("tcp://%s:%s", localHostName, localPort))

	peerID, err := uuid.Parse(remoteID)
	if err != nil {
		return err
	}

	svc := config.Service{
		Name: fmt.Sprintf("forward-%s-%s", remoteName, remotePort),
		Options: config.ServiceOptions{
			URLLocal:     utils.YAMLURL{URL: localURL},
			URLRemote:    utils.YAMLURL{URL: remoteURL},
			PeerDeviceID: peerID,
			TLSEnabled:   tlsEnabled,
		},
	}

	cfg.Services = append(cfg.Services, svc)
	if !o.NoPersist {
		f, err := os.Create(o.ConfigFile)
		if err == nil {
			yaml.NewEncoder(f).Encode(cfg)
			f.Close()
		}
	}

	app := application.GetPortierApplication()
	if app.IsRunning() {
		if err := app.AddService(svc); err != nil {
			return err
		}
	} else {
		if err := app.StartServices(cfg, creds); err != nil {
			return err
		}
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	app.StopServices()
	return nil
}
