//go:build windows

package main

import "fmt"

// announceWizardURL opens the local setup page in the default browser.
// No modal when the browser opens — the page itself has instructions (avoids dialog hidden behind the browser).
func announceWizardURL(_ string, url string) {
	if err := openBrowser(url); err != nil {
		loc := loadTrayUILocale()
		messageBoxOK(
			uiT(loc, "wizard_browser_fail_title"),
			fmt.Sprintf(uiT(loc, "wizard_browser_fail_body"), url),
		)
	}
}
