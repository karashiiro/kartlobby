package network

import (
	"errors"
	"net"
)

// GetLocalIP returns the first IP address that can be used to access processes mounted on localhost.
func GetLocalIP() (*net.IP, error) {
	// https://stackoverflow.com/a/31551220/14226597
	ips, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, ip := range ips {
		if ipnet, ok := ip.(*net.IPNet); ok && ipnet.IP.IsPrivate() {
			if ipnet.IP.To4() != nil {
				return &ipnet.IP, nil
			}
		}
	}

	return nil, errors.New("no local IP addresses found")
}
