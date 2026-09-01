package bootstrap

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// validateBindAddr enforces H2: non-loopback bind requires FISCAL_ALLOW_LAN=1.
func validateBindAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bootstrap: invalid bind addr %q: %w", addr, err)
	}
	host = strings.Trim(host, "[]")
	if host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "0.0.0.0" {
		if host == "0.0.0.0" && os.Getenv("FISCAL_ALLOW_LAN") != "1" {
			return fmt.Errorf("bootstrap: 0.0.0.0 bind requires FISCAL_ALLOW_LAN=1")
		}
		return nil
	}
	if os.Getenv("FISCAL_ALLOW_LAN") != "1" {
		return fmt.Errorf("bootstrap: non-loopback bind %s requires FISCAL_ALLOW_LAN=1", host)
	}
	return nil
}
