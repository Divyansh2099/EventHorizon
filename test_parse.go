//go:build ignore

package main

import (
	"fmt"
	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/pool"
)

func main() {
	reqStr := "GET /ws_test.html HTTP/1.1\r\n" +
		"Host: localhost:8082\r\n" +
		"Connection: keep-alive\r\n" +
		"Upgrade-Insecure-Requests: 1\r\n" +
		"\r\n"

	buf := pool.GetBuffer()
	copy(buf[:], reqStr)

	req := parser.GetRequestCtx(buf, buf)
	p := parser.Parser{}
	p.Reset()

	bytesConsumed, err := p.Parse(uint32(len(reqStr)), req)
	fmt.Printf("Total length: %d\n", len(reqStr))
	fmt.Printf("BytesConsumed: %d\n", bytesConsumed)
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("IsWebSocket: %v\n", req.IsWebSocket)
}
