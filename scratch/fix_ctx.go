//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	content, _ := os.ReadFile("pkg/eventhorizon/context.go")
	lines := strings.Split(string(content), "\n")
	
	var out []string
	skip := false
	for _, line := range lines {
		if strings.Contains(line, "// WriteWSFrame natively formats an RFC 6455 WebSocket frame") {
			out = append(out, `// WriteWSFrame natively formats an RFC 6455 WebSocket frame and triggers an asynchronous write.
// This operates independently of the HTTP request loop, enabling zero-allocation broadcasting.
func (c *Context) WriteWSFrame(opcode byte, payload []byte) error {
	c.Conn.WriteMu.Lock()
	defer c.Conn.WriteMu.Unlock()

	// Offset for TLS header
	headerOffset := c.Conn.StreamSizes.CbHeader
	c.Conn.WriteCursor = headerOffset
	
	buf := c.Conn.WriteBuffer
	buf[headerOffset] = 0x80 | opcode // FIN bit + opcode

	payloadLen := len(payload)
	offset := headerOffset + 2
	
	if payloadLen < 126 {
		buf[headerOffset+1] = byte(payloadLen)
	} else if payloadLen <= 65535 {
		buf[headerOffset+1] = 126
		buf[headerOffset+2] = byte(payloadLen >> 8)
		buf[headerOffset+3] = byte(payloadLen)
		offset = headerOffset + 4
	} else {
		buf[headerOffset+1] = 127
		buf[headerOffset+2] = byte(payloadLen >> 56)
		buf[headerOffset+3] = byte(payloadLen >> 48)
		buf[headerOffset+4] = byte(payloadLen >> 40)
		buf[headerOffset+5] = byte(payloadLen >> 32)
		buf[headerOffset+6] = byte(payloadLen >> 24)
		buf[headerOffset+7] = byte(payloadLen >> 16)
		buf[headerOffset+8] = byte(payloadLen >> 8)
		buf[headerOffset+9] = byte(payloadLen)
		offset = headerOffset + 10
	}

	// Zero-allocation payload copy
	copy(buf[offset:], payload)
	
	plainLen := (offset - headerOffset) + uint32(payloadLen)
	c.Conn.WriteCursor = headerOffset + plainLen
	
	// TLS Encrypt
	finalWriteLen := plainLen
	if c.Conn.TlsCtxt.IsValid() {
		var err error
		finalWriteLen, err = tls.Encrypt(&c.Conn.TlsCtxt, c.Conn.StreamSizes, c.Conn.WriteBuffer, plainLen)
		if err != nil {
			c.Conn.Release()
			return err
		}
	}
	
	// Trigger RIOSend manually since we are outside the standard request handler
	c.Conn.RioWriteBuffer.Length = finalWriteLen
	reqCtx := uintptr(unsafe.Pointer(&c.Conn.WriteOverlapped))
	
	ret, _, errSys := syscall.SyscallN(
		rio.RioTable.RIOSend,
		uintptr(c.Conn.RioRQ),
		uintptr(unsafe.Pointer(&c.Conn.RioWriteBuffer)),
		1, // DataBufferCount
		0, // Flags
		reqCtx,
	)

	if ret == 0 {
		if errSys != 0 {
			c.Conn.Release()
			return errSys
		}
	}
	return nil
}`)
			skip = true
			continue
		}
		
		if skip {
			continue // skip rest of file since it's the last function
		}
		
		if !skip {
			out = append(out, line)
		}
	}
	
	os.WriteFile("pkg/eventhorizon/context.go", []byte(strings.Join(out, "\n")), 0644)
}
