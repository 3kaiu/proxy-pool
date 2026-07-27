package fetcher

import (
	"net"

	"proxy-pool/internal/config"
)

var blockedNets []*net.IPNet

func init() {
	for _, cidr := range config.DefaultConfig.BlockedCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err == nil {
			blockedNets = append(blockedNets, ipnet)
		}
	}
}

func IsSafeProxy(ipPort string) bool {
	host, _, err := net.SplitHostPort(ipPort)
	if err != nil {
		host = ipPort // fallback if no port
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, n := range blockedNets {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
