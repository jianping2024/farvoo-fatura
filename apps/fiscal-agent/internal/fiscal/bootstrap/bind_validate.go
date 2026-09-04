package bootstrap

import (
	"fmt"
	"net"
	"strings"
)

// validateBindAddr enforces H2: non-loopback / 0.0.0.0 bind requires AllowLAN on Options.
// Does not read process environment (Agent listen intent is disk fiscal_allow_lan only).
func validateBindAddr(addr string, allowLAN bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bootstrap: invalid bind addr %q: %w", addr, err)
	}
	host = strings.Trim(host, "[]")
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return nil
	}
	if !allowLAN {
		if host == "0.0.0.0" || host == "::" {
			return fmt.Errorf("bootstrap: %s bind requires AllowLAN", host)
		}
		return fmt.Errorf("bootstrap: non-loopback bind %s requires AllowLAN", host)
	}
	return nil
}
