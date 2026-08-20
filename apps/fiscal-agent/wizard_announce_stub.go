//go:build !windows

package main

func announceWizardURL(title, url string) {
	if err := openBrowser(url); err != nil {
		agentLogLocale("zh", "log_browser_open_fail")
		agentLogTech(nil, "log_wizard_open", url)
	}
}
