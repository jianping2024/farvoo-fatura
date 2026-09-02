//go:build windows

package fiscalwebview

import (
	"runtime"
	"sync"
)

type uiCmd struct {
	opts Options
	html *HTMLWindowOptions
	done chan error
}

var (
	uiOnce  sync.Once
	uiReady chan struct{}
	uiCmdCh chan uiCmd
)

// startUIThread is the ONLY WebView2 message-loop entry (Agent + Client).
func startUIThread() {
	uiOnce.Do(func() {
		uiReady = make(chan struct{})
		uiCmdCh = make(chan uiCmd, 8)
		go func() {
			runtime.LockOSThread()
			close(uiReady)
			for cmd := range uiCmdCh {
				var err error
				if cmd.html != nil {
					err = runHTMLWindowOnThread(*cmd.html)
				} else {
					err = runWindowOnThread(cmd.opts)
				}
				if cmd.done != nil {
					cmd.done <- err
				}
			}
		}()
		<-uiReady
	})
}
