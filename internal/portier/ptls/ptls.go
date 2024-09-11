package ptls

import (
	"io"
	"net"

	"github.com/marinator86/portier-cli/internal/portier/config"
)

func CreateClientAndBridge(conn net.Conn, config *config.PortierConfig, serviceContext *config.ServiceContext) (net.Conn, error) {

	conn1, conn2 := net.Pipe()

	conn1, err := decorateTLS(conn1, config, serviceContext)
	if err != nil {
		return nil, err
	}

	// io.Copy from conn to conn1 and vice versa
	go func() {
		io.Copy(conn1, conn)
		conn1.Close()
	}()
	go func() {
		io.Copy(conn, conn1)
		conn.Close()
	}()

	return conn2, nil
}

func decorateTLS(conn net.Conn, config *config.PortierConfig, serviceContext *config.ServiceContext) (net.Conn, error) {
	// TODO: Implement this function
	return conn, nil
}
