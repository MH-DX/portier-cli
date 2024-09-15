package ptls

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func TestCreateClientAndBridge(t *testing.T) {
	// GIVEN
	// create pipe for encrypted communication
	commonDeviceID := uuid.MustParse("00000000-1111-0000-0000-000000000000")

	// Schema:
	// clientEnd <-net.Pipe()-> clientTLS (decorated) <-bridgeConnections()-> (decorated) serverTLS <-net.Pipe()-> serverEnd

	clientEnd, clientTLS := net.Pipe()
	serverEnd, serverTLS := net.Pipe()

	// create a self-signed certificate
	cert, key := getSelfSignedCert()

	// write a map of known hosts to a file
	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// calculate the fingerprint of the certificate
	fp := sha256.Sum256(tlsCert.Certificate[0])
	fpString := string(fp[:])
	knownHosts := map[string]string{
		commonDeviceID.String(): fmt.Sprintf("%x", fpString),
	}
	khStringWriter := &bytes.Buffer{}
	yaml.NewEncoder(khStringWriter).Encode(knownHosts)

	mockFileLoader := func(path string) ([]byte, error) {
		switch path {
		case "cert.pem":
			return cert, nil
		case "key.pem":
			return key, nil
		case "known_hosts":
			return khStringWriter.Bytes(), nil
		default:
			return nil, fmt.Errorf("unexpected path: %s", path)
		}
	}

	// create a PTLSConfig with the self-signed certificate, then create a TLS client and server
	ptls := NewPTLS(true, "cert.pem", "key.pem", "", "known_hosts", mockFileLoader)

	clientInner, handshaker, err := ptls.CreateClientAndBridge(clientTLS, commonDeviceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serverInner, err := ptls.CreateServerAndBridge(serverTLS, commonDeviceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// bridge the connections and perform the handshake
	bridgeConnections(clientInner, serverInner) // this mocks portier here actually

	err = handshaker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// WHEN
	// write 1kbyte message to client connection
	expectedMessage := make([]byte, 1024)
	rand.Read(expectedMessage)
	clientEnd.Write(expectedMessage)
	clientEnd.Close()

	// THEN
	buf := readFromConn(serverEnd)
	// assert that server connection received the message
	if len(buf) != len(expectedMessage) {
		t.Errorf("expected %d bytes, got %d", len(expectedMessage), len(buf))
	}
	for i := 0; i < len(expectedMessage); i++ {
		if expectedMessage[i] != buf[i] {
			t.Errorf("expected %v, got %v", expectedMessage[i], buf[i])
		}
	}
}

func bridgeConnections(client, server net.Conn) {
	go func() {
		io.Copy(server, client)
		server.Close()
	}()
	go func() {
		io.Copy(client, server)
		client.Close()
	}()
}

func readFromConn(conn net.Conn) []byte {
	buf := make([]byte, 1024)
	total := 0
	for {
		n, err := conn.Read(buf[total:])
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil
		}
		total += n
	}
	return buf
}

func getSelfSignedCert() ([]byte, []byte) {
	// openssl req -new -newkey rsa:2048 -keyout ca.key -x509 -sha256 -days 365 -out ca.crt -nodes

	CAPEMString := `-----BEGIN CERTIFICATE-----
MIIDyTCCArGgAwIBAgIUGwWJGoSm3ssgsIp+uj4JgwNjZvkwDQYJKoZIhvcNAQEL
BQAwdDELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDEtMCsGA1UEAwwkMDAwMDAwMDAtMTEx
MS0wMDAwLTAwMDAtMDAwMDAwMDAwMDAwMB4XDTI0MDkxMjIwMDMyN1oXDTI1MDkx
MjIwMDMyN1owdDELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAf
BgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDEtMCsGA1UEAwwkMDAwMDAw
MDAtMTExMS0wMDAwLTAwMDAtMDAwMDAwMDAwMDAwMIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEAwVZITSQFOfji6PfUasHvGH7a/+JFxksxDMXNuAQwnPFA
nU2yOwiSYtvzFHfrnxJBAJC8605SNiG1DiQJ6lWIaVeW6s4MRw0/apxgQxirpVIZ
gbRgLPTYniAtPVf+UPYBNI4za+NQ2UMSgJDo9+Nu60jpzmxxF9aTsjkELiPzytqi
0e0bVRgaLvKLuEbfpqEmZtlyQF0Kbfr++i0ZkbZM4VEQfaFaOc3yO22YVya8VWFG
T495V/qTg0YSdEDFGmj+lMZw6OOm6fH2P3GN7/PkqVp/T70ACpPaaZdRtbZzTQTK
uv7PAtpmx6INeCpkNiTuYHEifmrMAbFHPXdf0tMf0QIDAQABo1MwUTAdBgNVHQ4E
FgQUlPwcD5yLMJqsbfDP5XGQdnEM8NYwHwYDVR0jBBgwFoAUlPwcD5yLMJqsbfDP
5XGQdnEM8NYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAV8gd
odNr/F72yYkigWpEFFj6Sf8AG2npnhv5fTnGtHjHNKn7jLhxE5XUfAma/CWngw6E
PQXOtOrlJk1+0lcZvs7slCSS5JCdGptiTJHw8rCpS5J1CuJEk63kLGr0xyewfqYS
oddzCGjfhtYyFS871BqnQkyGvXOK8ACNIvnqqkSYdU9DpYbCjSgnDWqX3AyAAQDc
FAzL0ZVZ3z83ewM5Kk8XzLSgLPHLbHg08Z+RCC7cZ/uzNOovmKHiReVms5Fhw1BY
wbHLtct5NDXMHw6+0WlpKA516JpM0Zyb3T2TDar1SHSDNvwdisdw5jJSOizQSh/R
oeSb+iKrahseBoRX/g==
-----END CERTIFICATE-----`
	keyPEMString := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDBVkhNJAU5+OLo
99Rqwe8Yftr/4kXGSzEMxc24BDCc8UCdTbI7CJJi2/MUd+ufEkEAkLzrTlI2IbUO
JAnqVYhpV5bqzgxHDT9qnGBDGKulUhmBtGAs9NieIC09V/5Q9gE0jjNr41DZQxKA
kOj3427rSOnObHEX1pOyOQQuI/PK2qLR7RtVGBou8ou4Rt+moSZm2XJAXQpt+v76
LRmRtkzhURB9oVo5zfI7bZhXJrxVYUZPj3lX+pODRhJ0QMUaaP6UxnDo46bp8fY/
cY3v8+SpWn9PvQAKk9ppl1G1tnNNBMq6/s8C2mbHog14KmQ2JO5gcSJ+aswBsUc9
d1/S0x/RAgMBAAECggEAUhyNFIYp2hdEd+FZzAWVwHeQF4FIVRF2QZB48KCG7QDh
im2HNt9LHMWBlb3gymx8Qvs14VIgRHFIbjsMIwQ0rVjP9eWQ/VQ5DNRhZd3CeLJH
tvieqDlNDstnq1gF4Oi6VlHWsQmYOs3ru8LPzwg/AZq0AkG4PoGZtOXWSqpmTk6U
QWqbAoOB6uk7PD+/aKFdvi6gBouQMMP6+Bx0R0jjs3Ti/kVVQmCRvUAFewGNeebv
/I/djQMqqEYA9tIGrNaY5tUcDUiQXN6LOXaTfDfOxebidgE3JRxwXV6MQbxMjX2e
KAXrU/y3uCdA+LXDqTSfDCyvL6nHqaQHhGzqxbBbYQKBgQDgKyEqQuP16HzgxeKH
LkFQ2NZnhE3FeaMFcH1GVAjR6+Z2Z7AVN5E2jZZsN5j/2734Wec+IIObJp96RnJw
PHmKek+YEdwr2ZSwcDjighZUZC0QqCBia+81ue0kBXByHZLdD7yoAqjZlET8AIPk
VVYd6MDVVeF50b7OJgp59KKYXQKBgQDcymHydXUEHgkn2iN9IKR/maJNS2U9X2+5
j1uS0ylEngkt1xpZcq+gMu3KjEaP+7QZmCO24GoBQHIML+mKwXLU1AyJGFMyZDMJ
lb1S8Wm4+wNODQ/ZLdA+ZQdeq9XP8cjBRG5S+8dE8B8o1D4t66h+5dbKBNLCWHSM
1+LHa0ReBQKBgQCpwH9Q3W563R8Tp0YvT9uuOUXDBfFOxRmqGNEE3MYBET5oE4TH
zFhukzGBqWh2+BQXaR0vcre2Wb0Sfx5R17nCH3T+lye/HPj300OAYzo9lc56epZr
cYinirAFQwkvoS2BsVUPdVQfz6OdoVY/JlAcPhEoe+xOr4Jp4Wy1hYdLEQKBgQCW
juNvxKzA3AJ+TIA6yVGjOY61ip5E1ZmIPbvCSYAwrFuyCKaNLGmaomAI6NMNSCSt
91MTV8CxjdK3gMyOtA+sFdVef1nsWOt8s8FgmALyAyljxgBypo0EnzwBUMgCfuvY
7uMUb2CZH+z/mIu2IKbLsctgAx39LPh9OpIITptWSQKBgEUFIQEotYb6guT2+Hm6
NVm+Npn/lJLzJDPfV2jNnHNOKwSyiN/sVkGHNlCYPYsmOklBHu3M7EB5EnlfyMm7
vSMJnUtDc0GyG//5BAH8tQdfjxYfLGs4kSOw0U8Cy6Dk0G0tl/XmYvmMcYd5bDNK
mWNS+L7hG2IzWZMYz4dcINmd
-----END PRIVATE KEY-----`

	return []byte(CAPEMString), []byte(keyPEMString)
}
