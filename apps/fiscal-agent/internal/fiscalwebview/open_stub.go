//go:build !windows

package fiscalwebview

// RequestOpen focuses an existing window or starts one asynchronously.
func RequestOpen(opts Options) error {
	return ErrUnsupportedPlatform
}

// RunWindow opens a blocking fiscal UI window until the user closes it.
func RunWindow(opts Options) error {
	return ErrUnsupportedPlatform
}

// RunHTMLWindow opens a blocking HTML dialog window.
func RunHTMLWindow(opts HTMLWindowOptions) error {
	return ErrUnsupportedPlatform
}
