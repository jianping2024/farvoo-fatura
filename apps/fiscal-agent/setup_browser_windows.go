//go:build windows

package main

func openBrowser(url string) error {
	return shellExecute("open", url, "")
}
