//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	content, _ := os.ReadFile("pkg/server/handler.go")
	lines := strings.Split(string(content), "\n")
	
	var out []string
	skip := false
	for _, line := range lines {
		if strings.Contains(line, "// Check if we should recycle the connection or tear it down.") {
			out = append(out, line)
			out = append(out, `		if conn.IsWebSocket {
			// WebSockets do not currently support HTTP-style pipelining loops in this kernel
			// We just reset and issue a new WS read.
			conn.ReadLength = 0
			conn.ReadCursor = 0
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			conn.WriteOverlapped.Overlapped = windows.Overlapped{}
			conn.State = connection.StateWebSocket
			conn.ReadOverlapped.Op = iocp.OpRead
			s.postRead(conn)
		} else if conn.TlsCtxt.IsValid() && conn.State == connection.StateHandshake {
			if conn.StreamSizes.CbHeader > 0 {
				// TLS Handshake response sent. Now we transition to reading encrypted application data.
				conn.State = connection.StateReading`)
			skip = true
			continue
		}
		
		if skip && strings.Contains(line, "if conn.ReadCursor < conn.ReadLength {") {
			skip = false
		}
		
		if !skip {
			out = append(out, line)
		}
	}
	
	os.WriteFile("pkg/server/handler.go", []byte(strings.Join(out, "\n")), 0644)
}
