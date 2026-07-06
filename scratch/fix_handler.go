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
		if strings.Contains(line, "// Echo server behavior by default if no WSRoute is defined") {
			out = append(out, line)
			out = append(out, `		}
		
		// Shift any remaining WS bytes and read again (WS pipelining)
		if conn.ReadCursor < conn.ReadLength {
			remaining := conn.ReadLength - conn.ReadCursor
			copy(conn.ReadBuffer[:remaining], conn.ReadBuffer[conn.ReadCursor:conn.ReadLength])
			conn.ReadLength = 0
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			
			compKey := uintptr(unsafe.Pointer(conn))
			s.iocpPort.Post(remaining, compKey, &conn.ReadOverlapped.Overlapped)
		} else {
			conn.ReadLength = 0
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			s.postRead(conn)
		}
	}
}

// postRead submits an asynchronous RIOReceive operation to the kernel.
func (s *Server) postRead(conn *connection.Conn) {
	// Ensure the connection has an active RIO Buffer assigned
	if conn.RioBuffer.BufferId == 0 {
		var err error
		conn.RioBuffer, err = s.rioEngine.GetChunk()
		if err != nil {
			conn.Release()
			return
		}
		// Map the Go slice directly to the hardware-registered memory!
		conn.ReadBuffer = s.rioEngine.GetSlice(conn.RioBuffer)
	}
	if conn.RioWriteBuffer.BufferId == 0 {
		var err error
		conn.RioWriteBuffer, err = s.rioEngine.GetChunk()
		if err != nil {
			conn.Release()
			return
		}
		conn.WriteBuffer = s.rioEngine.GetSlice(conn.RioWriteBuffer)
	}

	// Ensure the operation type is correctly set for network reads.
	// This prevents bugs if postRead is called after an OpResume.
	if conn.ReadOverlapped.Op != iocp.OpWSRead {
		conn.ReadOverlapped.Op = iocp.OpRead
	}

	// We must slice the RIO_BUF to respect already-read pipelined bytes
	// We MUST modify the pinned conn structure directly, because RIOReceive
	// reads the RIO_BUF array asynchronously. Passing a stack pointer will cause WSAEFAULT.
	conn.RioBuffer.Offset += conn.ReadLength
	conn.RioBuffer.Length -= conn.ReadLength`)
			skip = true
			continue
		}
		
		if skip && strings.Contains(line, "// RIOReceive(SocketQueue, pData, DataBufferCount, Flags, RequestContext)") {
			skip = false
		}
		
		if !skip {
			out = append(out, line)
		}
	}
	
	os.WriteFile("pkg/server/handler.go", []byte(strings.Join(out, "\n")), 0644)
}
