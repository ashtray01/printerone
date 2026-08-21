package main

import (
	"net"
	"strings"
	"time"
)

// displayListenAddress keeps the server bound to all configured interfaces but
// turns an unspecified bind address into an IPv4 address that another machine
// on the LAN can actually use.
func displayListenAddress(listenAddress string) string {
	address := strings.TrimSpace(listenAddress)
	if address != "" && address != "0.0.0.0" && address != "::" {
		return address
	}

	if routeIP := preferredRouteIPv4(); routeIP != nil {
		return routeIP.String()
	}

	var candidates []net.IP
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, item := range interfaces {
			if item.Flags&net.FlagUp == 0 || item.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, addressErr := item.Addrs()
			if addressErr != nil {
				continue
			}
			for _, itemAddress := range addresses {
				ip, _, parseErr := net.ParseCIDR(itemAddress.String())
				if parseErr == nil {
					candidates = append(candidates, ip)
				}
			}
		}
	}
	if ip := bestIPv4(candidates); ip != nil {
		return ip.String()
	}
	return address
}

// A UDP connect performs only a local route lookup; no packet is sent. On XP
// this usually selects the adapter used for LAN traffic. Enumeration remains a
// fallback for isolated networks without a default route.
func preferredRouteIPv4() net.IP {
	connection, err := net.DialTimeout("udp4", "192.0.2.1:9", 250*time.Millisecond)
	if err != nil {
		return nil
	}
	defer connection.Close()
	local, ok := connection.LocalAddr().(*net.UDPAddr)
	if !ok || !usableIPv4(local.IP) {
		return nil
	}
	return local.IP.To4()
}

func bestIPv4(candidates []net.IP) net.IP {
	var best net.IP
	bestScore := 0
	for _, candidate := range candidates {
		if !usableIPv4(candidate) {
			continue
		}
		ip := candidate.To4()
		score := 2
		if privateIPv4(ip) {
			score = 3
		} else if ip[0] == 169 && ip[1] == 254 {
			score = 1
		}
		if score > bestScore {
			best = append(net.IP(nil), ip...)
			bestScore = score
		}
	}
	return best
}

func usableIPv4(ip net.IP) bool {
	four := ip.To4()
	if four == nil || four.IsLoopback() {
		return false
	}
	if four[0] == 0 || four[0] >= 224 || (four[0] == 255 && four[1] == 255 && four[2] == 255 && four[3] == 255) {
		return false
	}
	return true
}

func privateIPv4(ip net.IP) bool {
	four := ip.To4()
	if four == nil {
		return false
	}
	return four[0] == 10 ||
		(four[0] == 172 && four[1] >= 16 && four[1] <= 31) ||
		(four[0] == 192 && four[1] == 168)
}
