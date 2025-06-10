package cmd

import (
	"bytes"
	"testing"
)

func TestForwardCommandHelp(t *testing.T) {
	cmd, err := newForwardCmd()
	if err != nil {
		t.Fatal(err)
	}
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"-h"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSpecDefaultsLocalhost(t *testing.T) {
	o, _ := defaultForwardOptions()
	remote, rport, host, lport, err := o.parseSpec("dev:80->8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote != "dev" || rport != "80" || host != "localhost" || lport != "8080" {
		t.Fatalf("unexpected parse result: %s %s %s %s", remote, rport, host, lport)
	}
}
