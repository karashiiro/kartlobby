package network

import "net"

// GetFreePort returns a free port available on the system. If none are available, an error is returned
// and the port returns -1.
func GetFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	return port, nil
}
