//go:build !windows

package fiscalclient

// RunSettings opens the Agent connection settings UI. Returns agent_base or "" if cancelled.
func RunSettings(host, port string) (string, error) {
	return "", ErrSettingsUnsupported
}

// ErrSettingsUnsupported is returned when settings UI is unavailable on this platform.
var ErrSettingsUnsupported = ErrPlatform

// ErrPlatform indicates the fiscal client shell is Windows-only.
var ErrPlatform = errPlatform{}

type errPlatform struct{}

func (errPlatform) Error() string { return "fiscal client: windows only" }
