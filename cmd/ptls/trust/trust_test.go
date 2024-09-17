package tls_trust_cmd

import (
	"bytes"
	"testing"
)

func TestSingleDeviceID(t *testing.T) {
	// GIVEN
	cmd := NewTrustcmd()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"-a=www.test.de", "-i=1234,5678"})
	// WHEN
	cmd.ExecuteC()
}
