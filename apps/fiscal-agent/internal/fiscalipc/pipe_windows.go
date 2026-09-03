//go:build windows

package fiscalipc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
)

const pipeName = `\\.\pipe\FarvooFiscalAgent-v1`

const (
	openFiscalAttempts = 15
	openFiscalDelay    = 200 * time.Millisecond
)

var (
	serveMu  sync.Mutex
	serveGen int
)

// AgentInstanceRunning reports whether the tray Agent mutex is held by another process.
func AgentInstanceRunning() bool {
	name, err := syscall.UTF16PtrFromString(AgentMutexName)
	if err != nil {
		return false
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openMutexW := kernel32.NewProc("OpenMutexW")
	const mutexAllAccess = 0x001F0001
	handle, _, _ := openMutexW.Call(mutexAllAccess, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return false
	}
	_ = syscall.CloseHandle(syscall.Handle(handle))
	return true
}

// ServeAgentCommands listens for open-fiscal commands until ctx is cancelled.
// ONLY Agent IPC entry; listener stays open (accept loop) for concurrent shortcut clicks.
func ServeAgentCommands(ctx context.Context, onOpenFiscal func()) {
	if onOpenFiscal == nil {
		return
	}
	serveMu.Lock()
	serveGen++
	gen := serveGen
	serveMu.Unlock()

	go func() {
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			serveMu.Lock()
			stale := gen != serveGen
			serveMu.Unlock()
			if stale {
				return
			}
			ln, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
				SecurityDescriptor: "D:P(A;;GA;;;WD)",
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			servePipe(ctx, ln, gen, onOpenFiscal)
			_ = ln.Close()
		}
	}()
}

func servePipe(ctx context.Context, ln net.Listener, gen int, onOpenFiscal func()) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		serveMu.Lock()
		stale := gen != serveGen
		serveMu.Unlock()
		if stale {
			return
		}
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go handleConn(conn, onOpenFiscal)
	}
}

func handleConn(conn net.Conn, onOpenFiscal func()) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && err != io.EOF {
		return
	}
	if strings.TrimSpace(line) == CommandOpenFiscal {
		onOpenFiscal()
	}
	_, _ = io.WriteString(conn, "ok\n")
}

// RequestOpenFiscal is the sole IPC open path: retries while the Agent pipe comes up.
func RequestOpenFiscal() error {
	var last error
	for i := 0; i < openFiscalAttempts; i++ {
		last = requestOpenFiscalOnce()
		if last == nil {
			return nil
		}
		time.Sleep(openFiscalDelay)
	}
	return last
}

func requestOpenFiscalOnce() error {
	timeout := 3 * time.Second
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return fmt.Errorf("fiscal ipc: agent not reachable: %w", err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, CommandOpenFiscal+"\n"); err != nil {
		return err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "ok" {
		return fmt.Errorf("fiscal ipc: unexpected response %q", strings.TrimSpace(line))
	}
	return nil
}
