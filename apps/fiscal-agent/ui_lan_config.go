package main

import (
	"log"
	"net"
	"os"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/api"
)

// loadAgentFiscalAllowLANState is the ONLY config.json reader for fiscal_allow_lan.
// set=false means key omitted (default off).
func loadAgentFiscalAllowLANState() (set bool, allow bool) {
	c, err := loadConfig(defaultConfigPath())
	if err != nil || c == nil || c.FiscalAllowLAN == nil {
		return false, false
	}
	return true, *c.FiscalAllowLAN
}

// setAgentFiscalAllowLAN is the ONLY config.json writer for fiscal_allow_lan.
func setAgentFiscalAllowLAN(allow bool) error {
	if lanOpsLockedAllow || lanOpsLockedBind {
		return api.ErrLanEnvLocked
	}
	path := defaultConfigPath()
	c, err := loadConfig(path)
	if err != nil {
		c = &config{}
	}
	c.FiscalAllowLAN = &allow
	return saveConfig(path, c)
}

// buildAgentLanAccessSnapshot is the ONLY Agent-side LAN status builder for Admin GET/PUT.
func buildAgentLanAccessSnapshot(listenBind string) api.LanAccessSnapshot {
	captureLanOpsEnvLocksOnce()
	envLocked := lanOpsLockedAllow || lanOpsLockedBind
	bind := strings.TrimSpace(listenBind)
	if bind == "" {
		bind = strings.TrimSpace(os.Getenv("FISCAL_BIND"))
	}
	if bind == "" {
		bind = "127.0.0.1:17880"
	}
	listeningLAN := bindIsLAN(bind)
	source := "default"
	desired := false
	if envLocked {
		source = "env"
		desired = os.Getenv("FISCAL_ALLOW_LAN") == "1" || listeningLAN
	} else if set, allow := loadAgentFiscalAllowLANState(); set {
		source = "config"
		desired = allow
	}
	port := "17880"
	if _, p, err := net.SplitHostPort(bind); err == nil && strings.TrimSpace(p) != "" {
		port = p
	}
	return api.LanAccessSnapshot{
		AllowLAN:        desired,
		ListeningLAN:    listeningLAN,
		BindAddr:        bind,
		Port:            port,
		Source:          source,
		EnvLocked:       envLocked,
		RestartRequired: !envLocked && desired != listeningLAN,
		AgentLANIPs:     api.AgentLANIPv4(),
	}
}

func bindIsLAN(bind string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(bind))
	if err != nil {
		host = strings.TrimSpace(bind)
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return false
	}
	return true
}

// agentLanAccessGet / agentLanAccessSet are the ONLY HandlerDeps callbacks for LAN Admin.
func agentLanAccessGet() (api.LanAccessSnapshot, error) {
	return buildAgentLanAccessSnapshot(embeddedListenBind()), nil
}

func agentLanAccessSet(allow bool) (api.LanAccessSnapshot, error) {
	if err := setAgentFiscalAllowLAN(allow); err != nil {
		return api.LanAccessSnapshot{}, err
	}
	// Apply listen change in-process (disk is authority). Avoids stale loopback from cold shell.
	if err := startEmbeddedFiscal(nil); err != nil {
		log.Printf("fiscal: lan rebind after save: %v", err)
	}
	snap := buildAgentLanAccessSnapshot(embeddedListenBind())
	snap.AllowLAN = allow
	snap.Source = "config"
	snap.RestartRequired = !snap.EnvLocked && allow != snap.ListeningLAN
	return snap, nil
}

func embeddedListenBind() string {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	u := strings.TrimPrefix(embeddedFiscalURL, "http://")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimRight(u, "/")
	return u
}
