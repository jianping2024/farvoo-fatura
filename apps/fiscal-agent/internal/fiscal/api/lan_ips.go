package api

import (
	"net"
	"sort"
	"strings"
)

// AgentLANIPv4 is the ONLY enumerator of non-loopback IPv4 addresses for Admin LAN UI.
func AgentLANIPv4() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			ip = ip.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			s := ip.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// PreferAgentLANIP picks a primary store-LAN address for copy hints.
func PreferAgentLANIP(ips []string) string {
	for _, ip := range ips {
		if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "172.") {
			return ip
		}
	}
	if len(ips) > 0 {
		return ips[0]
	}
	return ""
}
