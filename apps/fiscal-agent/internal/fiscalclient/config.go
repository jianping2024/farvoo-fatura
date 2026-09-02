package fiscalclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config is the LAN Fiscal Client local settings file.
type Config struct {
	AgentBase string `json:"agent_base"`
}

// NormalizeAgentBase returns http://host:port without trailing slash.
func NormalizeAgentBase(host, port string) (string, error) {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" {
		return "", fmt.Errorf("agent host required")
	}
	if strings.Contains(host, "://") || strings.Contains(host, "/") {
		return "", fmt.Errorf("agent host must be an IP or hostname only")
	}
	if port == "" {
		port = "17880"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("port must be 1–65535")
	}
	return fmt.Sprintf("http://%s:%d", host, n), nil
}

// NormalizeAgentBaseURL trims and validates a stored agent_base URL.
func NormalizeAgentBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("agent_base required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" {
		return "", fmt.Errorf("agent_base must use http")
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("agent_base host required")
	}
	if strings.TrimSpace(u.Path) != "" && u.Path != "/" {
		return "", fmt.Errorf("agent_base must not include a path")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
		port = "17880"
	}
	return NormalizeAgentBase(host, port)
}

// SplitAgentBase returns host and port from a normalized agent_base URL.
func SplitAgentBase(agentBase string) (host, port string, err error) {
	agentBase, err = NormalizeAgentBaseURL(agentBase)
	if err != nil {
		return "", "", err
	}
	u, err := url.Parse(agentBase)
	if err != nil {
		return "", "", err
	}
	host, port, err = net.SplitHostPort(u.Host)
	if err != nil {
		return u.Hostname(), "17880", nil
	}
	return host, port, nil
}

// ProbeHealth GET {agent_base}/local/v1/health — public route for connection test.
func ProbeHealth(agentBase string) error {
	agentBase, err := NormalizeAgentBaseURL(agentBase)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, agentBase+"/local/v1/health", nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach agent: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("agent health returned HTTP %d", res.StatusCode)
	}
	return nil
}
