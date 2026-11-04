package fiscalwebview

import "errors"

// WindowTitle is the sole user-facing fiscal shell window title (Agent + Client).
const WindowTitle = "Farvoo 开票"

// Embedded RT_GROUP_ICON ids from akavel/rsrc (-ico order).
const (
	// AgentShellIconID is assets/agent_app_icon.ico when Agent embeds agent,fiscal.
	AgentShellIconID = 1
	// AgentFiscalIconID is assets/fiscal_app_icon.ico (A1) as the second Agent resource.
	AgentFiscalIconID = 2
	// ClientFiscalIconID is assets/fiscal_app_icon.ico when Client embeds fiscal only.
	ClientFiscalIconID = 1
)

// windowIconID is the RT_GROUP_ICON id used by WebView2 fiscal / HTML shells.
// Default = ClientFiscalIconID (1). Agent must call UseAgentFiscalWindowIcon before open.
var windowIconID uint = ClientFiscalIconID

// UseAgentFiscalWindowIcon sets WebView IconId to AgentFiscalIconID (2).
// ONLY call from Agent Windows binary (agent embeds agent+fiscal).
func UseAgentFiscalWindowIcon() {
	windowIconID = AgentFiscalIconID
}

// WindowIconID returns the active fiscal-window icon resource id (for tests / diagnostics).
func WindowIconID() uint {
	return windowIconID
}

// ErrUnsupportedPlatform is returned on non-Windows builds.
var ErrUnsupportedPlatform = errors.New("fiscal webview: windows only")

// Options configures a fiscal UI shell window.
type Options struct {
	// URL is the full entry URL (e.g. http://127.0.0.1:17880/).
	URL string
	// DataPath is the WebView2 user-data directory (cookie session).
	DataPath string
}

// HTMLWindowOptions configures a small HTML dialog shell (settings, etc.).
type HTMLWindowOptions struct {
	Title    string
	HTML     string
	DataPath string
	Width    uint
	Height   uint
	// Bind is the ONLY registration path for JS↔Go on HTML dialogs.
	// closeDialog terminates the window (Dispatch+Terminate); call from save/cancel.
	Bind func(closeDialog func()) map[string]interface{}
}
