package eventhorizon

import (
	"bytes"
	"encoding/json"
	"strconv"
	"path/filepath"

	"github.com/eventhorizon/pkg/connection"
	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/rio"
	"github.com/eventhorizon/pkg/iocp"
	"github.com/eventhorizon/pkg/tls"
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

// Context wraps the internal EventHorizon request and connection models,
// providing a clean API for developers without triggering heap allocations.
type Context struct {
	Req  *parser.RequestCtx
	Conn *connection.Conn
}

// Param retrieves a path parameter natively extracted by the Radix Tree.
func (c *Context) Param(key string) string {
	val := c.Req.GetParam(key)
	if val == nil {
		return ""
	}
	// We cast to string unsafely if we want zero alloc, or copy. 
	// For ease of use, standard string conversion.
	return string(val)
}

// ParamBytes retrieves a path parameter directly from pinned hardware memory (0 alloc).
func (c *Context) ParamBytes(key string) []byte {
	return c.Req.GetParam(key)
}

// Query retrieves a query parameter from the URL path.
func (c *Context) Query(key string) []byte {
	path := c.Req.PathBytes()
	idx := bytes.IndexByte(path, '?')
	if idx == -1 {
		return nil
	}
	queryStr := path[idx+1:]
	
	// A simple, zero-alloc iteration over key=value pairs
	for len(queryStr) > 0 {
		ampIdx := bytes.IndexByte(queryStr, '&')
		var pair []byte
		if ampIdx == -1 {
			pair = queryStr
			queryStr = nil
		} else {
			pair = queryStr[:ampIdx]
			queryStr = queryStr[ampIdx+1:]
		}

		eqIdx := bytes.IndexByte(pair, '=')
		if eqIdx != -1 {
			if string(pair[:eqIdx]) == key {
				return pair[eqIdx+1:]
			}
		} else {
			if string(pair) == key {
				return []byte("")
			}
		}
	}
	return nil
}

// SendString appends a plaintext string to the connection's RIO write buffer.
func (c *Context) SendString(s string) {
	c.Req.SetStatusCode(200)
	c.Req.Write([]byte("Content-Type: text/plain\r\nContent-Length: "))
	
	var numBuf [32]byte
	b := strconv.AppendInt(numBuf[:0], int64(len(s)), 10)
	c.Req.Write(b)

	if c.Req.KeepAlive {
		c.Req.Write([]byte("\r\nConnection: keep-alive\r\n\r\n"))
	} else {
		c.Req.Write([]byte("\r\nConnection: close\r\n\r\n"))
	}

	c.Req.Write([]byte(s))
}

// JSON marshals a generic interface into the RIO write buffer.
func (c *Context) JSON(v interface{}) error {
	c.Req.SetStatusCode(200)
	// We write headers later because we don't know the exact JSON length until we marshal.
	// But we can marshal directly into the unused space of WriteBuffer!
	
	// Temporarily marshal to a local slice pointing into the write buffer
	// (Note: standard encoding/json still allocates internally, but we can save the final copy)
	data, err := json.Marshal(v)
	if err != nil {
		c.Req.SetStatusCode(500)
		return err
	}
	
	c.Req.Write([]byte("Content-Type: application/json\r\nContent-Length: "))
	var numBuf [32]byte
	b := strconv.AppendInt(numBuf[:0], int64(len(data)), 10)
	c.Req.Write(b)

	if c.Req.KeepAlive {
		c.Req.Write([]byte("\r\nConnection: keep-alive\r\n\r\n"))
	} else {
		c.Req.Write([]byte("\r\nConnection: close\r\n\r\n"))
	}

	c.Req.Write(data)
	return nil
}

// SendFile streams a file directly from disk into the RIO buffer using overlapped I/O.
func (c *Context) SendFile(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(filepath.FromSlash(path))
	if err != nil {
		c.Req.SetStatusCode(500)
		return err
	}

	hFile, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED,
		0,
	)

	if err != nil {
		c.Req.SetStatusCode(404)
		return err
	}

	var fileInfo windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(hFile, &fileInfo)
	if err != nil {
		windows.CloseHandle(hFile)
		c.Req.SetStatusCode(500)
		return err
	}

	c.Req.SetStatusCode(200)
	
	// Best-effort content type
	ext := filepath.Ext(path)
	contentType := "application/octet-stream"
	switch ext {
	case ".html": contentType = "text/html"
	case ".css": contentType = "text/css"
	case ".js": contentType = "application/javascript"
	case ".json": contentType = "application/json"
	case ".png": contentType = "image/png"
	case ".jpg", ".jpeg": contentType = "image/jpeg"
	case ".ico": contentType = "image/x-icon"
	}
	
	c.Req.Write([]byte("Content-Type: " + contentType + "\r\nContent-Length: "))
	
	var numBuf [32]byte
	b := strconv.AppendUint(numBuf[:0], uint64(fileInfo.FileSizeHigh)<<32|uint64(fileInfo.FileSizeLow), 10)
	c.Req.Write(b)

	if c.Req.KeepAlive {
		c.Req.Write([]byte("\r\nConnection: keep-alive\r\n\r\n"))
	} else {
		c.Req.Write([]byte("\r\nConnection: close\r\n\r\n"))
	}

	c.Conn.FileHandle = hFile
	c.Conn.FileSize = uint64(fileInfo.FileSizeHigh)<<32 | uint64(fileInfo.FileSizeLow)
	c.Conn.FileOffset = 0
	
	c.Req.TransmittedFile = true
	return nil
}

// WriteWSFrame natively formats an RFC 6455 WebSocket frame and triggers an asynchronous write.
// This operates independently of the HTTP request loop, enabling zero-allocation broadcasting.
func (c *Context) WriteWSFrame(opcode byte, payload []byte) error {
	c.Conn.WriteMu.Lock()

	// Offset for TLS header
	headerOffset := c.Conn.StreamSizes.CbHeader
	
	bytesWritten := parser.WriteWSFrame(c.Conn.WriteBuffer, headerOffset, opcode, payload)
	
	plainLen := bytesWritten
	c.Conn.WriteCursor = headerOffset + plainLen
	
	// TLS Encrypt
	finalWriteLen := plainLen
	if c.Conn.TlsCtxt.IsValid() {
		var err error
		c.Conn.TlsMu.Lock()
		finalWriteLen, err = tls.Encrypt(&c.Conn.TlsCtxt, c.Conn.StreamSizes, c.Conn.WriteBuffer, plainLen)
		c.Conn.TlsMu.Unlock()
		if err != nil {
			c.Conn.WriteMu.Unlock()
			c.Conn.Release()
			return err
		}
	}
	
	// Trigger RIOSend manually since we are outside the standard request handler
	c.Conn.WriteOverlapped.Op = iocp.OpWSWrite
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
		c.Conn.WriteMu.Unlock()
		if errSys != 0 {
			c.Conn.Release()
			return errSys
		}
	}
	return nil
}
