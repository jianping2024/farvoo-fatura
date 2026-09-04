package main

import (
	"log"
	"net"
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
	path := defaultConfigPath()
	c, err := loadConfig(path)
	if err != nil {
		c = &config{}
	}
	c.FiscalAllowLAN = &allow
	return saveConfig(path, c)
}

const (
	fiscalListenLoopback = "127.0.0.1:17880"
	fiscalListenLAN      = "0.0.0.0:17880"
)

// resolveFiscalListenBind is the ONLY Agent listen-address resolver.
// Disk fiscal_allow_lan only — never reads FISCAL_ALLOW_LAN / FISCAL_BIND.
func resolveFiscalListenBind() (allowLAN bool, bind string) {
	if set, allow := loadAgentFiscalAllowLANState(); set && allow {
		return true, fiscalListenLAN
	}
	return false, fiscalListenLoopback
}

// buildAgentLanAccessSnapshot is the ONLY Agent-side LAN status builder for Admin GET/PUT.
func buildAgentLanAccessSnapshot(listenBind string) api.LanAccessSnapshot {
	bind := strings.TrimSpace(listenBind)
	if bind == "" {
		_, bind = resolveFiscalListenBind()
	}
	listeningLAN := bindIsLAN(bind)
	source := "default"
	desired := false
	if set, allow := loadAgentFiscalAllowLANState(); set {
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
		RestartRequired: desired != listeningLAN,
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
	// Apply listen change in-process (disk is authority).
	if err := startEmbeddedFiscal(nil); err != nil {
		log.Printf("fiscal: lan rebind after save: %v", err)
	}
	snap := buildAgentLanAccessSnapshot(embeddedListenBind())
	snap.AllowLAN = allow
	snap.Source = "config"
	snap.RestartRequired = allow != snap.ListeningLAN
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
