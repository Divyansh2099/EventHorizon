package parser

import (
	"fmt"
	"testing"
	"github.com/eventhorizon/pkg/pool"
)

func TestParseKeepAlive(t *testing.T) {
	buf := &pool.Buffer{}
	wbuf := &pool.Buffer{}
	req := &RequestCtx{
		Buffer: buf[:],
		WriteBuffer: wbuf[:],
		Headers: make([]HeaderSpan, 128),
	}
	
	reqStr := "GET /api/shallow HTTP/1.1\r\nHost: 127.0.0.1:8080\r\n\r\n"
	copy(buf[:], reqStr)
	
	p := Parser{}
	p.Reset()
	bytesConsumed, err := p.Parse(uint32(len(reqStr)), req)
	
	fmt.Printf("Consumed: %d, Err: %v\n", bytesConsumed, err)
	fmt.Printf("Method: %q\n", req.MethodBytes())
	fmt.Printf("Path: %q\n", req.PathBytes())
	fmt.Printf("Version: %q\n", buf[req.Version.Start:req.Version.End])
	fmt.Printf("KeepAlive: %v\n", req.KeepAlive)
}
