package ptls

import (
	"io"
	"net"
	"testing"
)

func TestCreateClientAndBridge(t *testing.T) {
	// GIVEN
	// create listener on random port
	listener, _ := net.Listen("tcp", "localhost:0")
	// create client connection to listener
	clientConn, _ := net.Dial("tcp", listener.Addr().String())
	// create server connection from listener
	serverConn, _ := listener.Accept()
	bridgeConn, err := CreateClientAndBridge(serverConn, nil, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// WHEN
	// write 100kbyte message to client connection
	clientConn.Write(make([]byte, 100000))
	clientConn.Close()

	// THEN
	// read from server connection
	buf := make([]byte, 100000)
	total := 0
	// read until EOF
	for {
		n, err := bridgeConn.Read(buf[total:])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		total += n
	}
	// assert that server connection received the message
	if total != 100000 {
		t.Errorf("expected 100000 bytes, got %d", total)
	}
}

func TestClosingOfServerConn(t *testing.T) {
	// GIVEN
	// create listener on random port
	listener, _ := net.Listen("tcp", "localhost:0")
	// create client connection to listener
	clientConn, _ := net.Dial("tcp", listener.Addr().String())
	// create server connection from listener
	serverConn, _ := listener.Accept()
	bridgeConn, err := CreateClientAndBridge(serverConn, nil, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// WHEN
	bridgeConn.Close()

	// THEN
	// assert that server connection is closed
	buf := make([]byte, 1)
	_, err = serverConn.Read(buf)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	// assert that client connection is closed
	_, err = clientConn.Read(buf)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
