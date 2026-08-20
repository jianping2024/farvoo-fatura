// Dev-only fake RAW printer: TCP :9100 + lightweight ESC/POS decode to stdout.
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

func main() {
	name := env("PRINTER_NAME", "fake")
	port := env("PRINTER_PORT", "9100")
	addr := "0.0.0.0:" + port

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[%s] fake printer on %s (ESC/POS decode → stdout)", name, addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConn(conn, name)
	}
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func handleConn(conn net.Conn, name string) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	data, err := io.ReadAll(conn)
	if err != nil && len(data) == 0 {
		log.Printf("[%s] read: %v", name, err)
		return
	}

	fmt.Printf("\n[%s] ═══════ ticket (%d bytes) ═══════\n", name, len(data))
	fmt.Print(decodeEscPos(data))
	fmt.Printf("[%s] ═══════ end ═══════\n\n", name)
}

// decodeEscPos turns common thermal commands into readable dev output (not bit-perfect).
func decodeEscPos(data []byte) string {
	var b strings.Builder
	i := 0
	for i < len(data) {
		if data[i] == 0x1B && i+1 < len(data) {
			switch data[i+1] {
			case 0x40: // init
				b.WriteString("\n--- ESC @ (initialize) ---\n")
				i += 2
				continue
			case 0x21: // print mode / font
				if i+2 < len(data) {
					n := data[i+2]
					b.WriteString(fmt.Sprintf("\n[ESC ! n=%d → emphasis/width/height bits]\n", n))
					i += 3
					continue
				}
			case 0x74: // code page
				if i+2 < len(data) {
					b.WriteString(fmt.Sprintf("\n[ESC t n=%d → character code table]\n", data[i+2]))
					i += 3
					continue
				}
			}
		}
		if data[i] == 0x1D && i+1 < len(data) {
			switch data[i+1] {
			case 0x56: // cut
				b.WriteString("\n--- GS V (paper cut) ---\n")
				if i+2 < len(data) && data[i+2] <= 3 {
					i += 3
				} else {
					i += 2
				}
				continue
			}
		}

		if data[i] == '\n' {
			b.WriteByte('\n')
			i++
			continue
		}
		if data[i] == '\r' {
			i++
			continue
		}

		if data[i] >= 0x20 && data[i] < 0x7F {
			b.WriteByte(data[i])
			i++
			continue
		}

		if data[i] >= 0x80 {
			r, size := utf8.DecodeRune(data[i:])
			if r != utf8.RuneError && size > 0 {
				b.WriteRune(r)
				i += size
				continue
			}
		}

		b.WriteString(fmt.Sprintf("<0x%02X>", data[i]))
		i++
	}
	return b.String()
}
