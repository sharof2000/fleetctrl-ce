package utils

import (
	"net"
)

// GetLocalIP returns the non-loopback local IP address
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}

// GetAllLocalIPs returns all non-loopback local IP addresses
func GetAllLocalIPs() []string {
	var ips []string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	return ips
}

// IsLocalAddress checks if an address belongs to this host
func IsLocalAddress(address string, port string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	// Check localhost variants
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Check against all local IPs
	localIPs := GetAllLocalIPs()
	for _, localIP := range localIPs {
		if host == localIP {
			return true
		}
	}

	return false
}
