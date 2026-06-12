package utils

import (
	"net"
)

// GetLocalIP returns the preferred outbound IP address of this machine
func GetLocalIP() string {
	// Try to connect to a public DNS to determine the outbound IP.
	// No packets are sent for a UDP "connection"; this just selects the
	// routable source interface, which is correct on multi-NIC hosts.
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback: pick any non-loopback IPv4 from the interfaces
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
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
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
