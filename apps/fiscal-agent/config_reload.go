package main

// reloadAgentSessionConfig refreshes in-memory config from disk (configure UI saves).
func reloadAgentSessionConfig(sess *agentSession) {
	if sess == nil {
		return
	}
	path := sess.cfgPath
	if path == "" {
		path = defaultConfigPath()
	}
	cfg, err := loadConfig(path)
	if err != nil || cfg == nil {
		return
	}
	sess.cfg = cfg
}
