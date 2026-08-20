//go:build !windows

package protect

import "fmt"

func sealDPAPI([]byte) (string, error) {
	return "", fmt.Errorf("protect: dpapi only available on Windows")
}

func openDPAPI(string) ([]byte, error) {
	return nil, fmt.Errorf("protect: dpapi only available on Windows")
}
