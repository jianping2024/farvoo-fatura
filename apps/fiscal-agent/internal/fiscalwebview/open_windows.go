//go:build windows

package fiscalwebview

import (
	"fmt"
	"strings"
	"sync"

	"github.com/jchv/go-webview2"
)

var (
	windowMu   sync.Mutex
	activeHWND uintptr
	opening    bool
)

// RequestOpen focuses an existing shell window or queues one on the UI thread.
func RequestOpen(opts Options) error {
	opts.URL = strings.TrimSpace(opts.URL)
	if opts.URL == "" {
		return fmt.Errorf("fiscal webview: url required")
	}
	startUIThread()

	if hwnd, ok := takeExistingShellHWND(); ok {
		focusHWND(hwnd)
		return nil
	}

	windowMu.Lock()
	if opening {
		windowMu.Unlock()
		return nil
	}
	opening = true
	windowMu.Unlock()

	uiCmdCh <- uiCmd{opts: opts}
	return nil
}

// RunWindow opens a blocking fiscal UI window until the user closes it (Client main path).
func RunWindow(opts Options) error {
	opts.URL = strings.TrimSpace(opts.URL)
	if opts.URL == "" {
		return fmt.Errorf("fiscal webview: url required")
	}
	startUIThread()

	if hwnd, ok := takeExistingShellHWND(); ok {
		focusHWND(hwnd)
		return nil
	}

	done := make(chan error, 1)
	uiCmdCh <- uiCmd{opts: opts, done: done}
	return <-done
}

// RunHTMLWindow opens a blocking HTML dialog window on the UI thread.
func RunHTMLWindow(opts HTMLWindowOptions) error {
	if strings.TrimSpace(opts.HTML) == "" {
		return fmt.Errorf("fiscal webview: html required")
	}
	startUIThread()
	done := make(chan error, 1)
	html := opts
	uiCmdCh <- uiCmd{html: &html, done: done}
	return <-done
}

// takeExistingShellHWND is the sole gate for "shell already open" (incl. minimized).
// Uses IsWindow only so iconic windows still restore (do not require a ping response).
func takeExistingShellHWND() (uintptr, bool) {
	windowMu.Lock()
	defer windowMu.Unlock()
	if activeHWND == 0 {
		return 0, false
	}
	hwnd := activeHWND
	if !isWindow(hwnd) {
		activeHWND = 0
		return 0, false
	}
	return hwnd, true
}

func runWindowOnThread(opts Options) error {
	windowMu.Lock()
	opening = true
	windowMu.Unlock()
	defer func() {
		windowMu.Lock()
		opening = false
		windowMu.Unlock()
	}()

	wv := newWebView(webview2.WindowOptions{
		Title:  WindowTitle,
		Width:  1280,
		Height: 860,
		IconId: WindowIconID,
		Center: true,
	}, opts.DataPath)
	if wv == nil {
		return webView2MissingError()
	}
	trackHWND(wv)
	wv.Navigate(opts.URL)
	wv.Run()
	return nil
}

func runHTMLWindowOnThread(opts HTMLWindowOptions) error {
	width := opts.Width
	if width == 0 {
		width = 480
	}
	height := opts.Height
	if height == 0 {
		height = 360
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = WindowTitle
	}
	wv := newWebView(webview2.WindowOptions{
		Title:  title,
		Width:  width,
		Height: height,
		IconId: WindowIconID,
		Center: true,
	}, opts.DataPath)
	if wv == nil {
		return webView2MissingError()
	}
	for name, fn := range opts.Bind {
		if err := wv.Bind(name, fn); err != nil {
			return err
		}
	}
	wv.SetHtml(opts.HTML)
	wv.Run()
	return nil
}

func newWebView(win webview2.WindowOptions, dataPath string) webview2.WebView {
	return webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  dataPath,
		AutoFocus: true,
		WindowOptions: win,
	})
}

func trackHWND(wv webview2.WebView) {
	if ptr := wv.Window(); ptr != nil {
		windowMu.Lock()
		activeHWND = uintptr(ptr)
		windowMu.Unlock()
		defer func() {
			windowMu.Lock()
			activeHWND = 0
			windowMu.Unlock()
		}()
	}
}

func webView2MissingError() error {
	return fmt.Errorf("fiscal webview: failed to create WebView2 — install Microsoft Edge WebView2 Runtime")
}
