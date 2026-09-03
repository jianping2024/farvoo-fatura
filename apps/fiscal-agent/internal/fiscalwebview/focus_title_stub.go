//go:build !windows

package fiscalwebview

// FocusExistingByTitle is a no-op off Windows.
func FocusExistingByTitle(title string) bool { return false }
