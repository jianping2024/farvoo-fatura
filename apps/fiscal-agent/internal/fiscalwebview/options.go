package fiscalwebview

import "errors"

// WindowTitle is the sole user-facing fiscal shell window title (Agent + Client).
const WindowTitle = "Farvoo 开票"

// WindowIconID is the RT_GROUP_ICON resource id from rsrc (assets/app_icon.ico → rsrc_windows_*.syso).
// Must stay in lockstep with akavel/rsrc embed order (ico-only → group id 1).
const WindowIconID = 1

// ErrUnsupportedPlatform is returned on non-Windows builds.
var ErrUnsupportedPlatform = errors.New("fiscal webview: windows only")

// Options configures a fiscal UI shell window.
type Options struct {
	// URL is the full entry URL (e.g. http://127.0.0.1:17880/).
	URL string
	// DataPath is the WebView2 user-data directory (cookie session).
	DataPath string
}
