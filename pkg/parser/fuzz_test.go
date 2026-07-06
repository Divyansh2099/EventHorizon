package parser

import (
	"testing"
)

func FuzzParseHTTP(f *testing.F) {
	// Add valid seed corpus
	f.Add([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	f.Add([]byte("POST /api/v1/users HTTP/2.0\r\nContent-Length: 5\r\n\r\nHello"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// The zero-allocation parser expects the buffer to be mutated or referenced directly
		ctx := GetRequestCtx(data, nil)
		p := &Parser{}
		p.Reset()

		// The goal of this test is to ensure Parse never panics (e.g. index out of range)
		// regardless of how malformed the input data is.
		p.Parse(uint32(len(data)), ctx)
		
		ctx.Release()
	})
}

func FuzzParseWSFrame(f *testing.F) {
	// Add valid seed corpus
	f.Add([]byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}) // Unmasked text "Hello"
	f.Add([]byte{0x81, 0x85, 0x37, 0xfa, 0x21, 0x3d, 0x7f, 0x9f, 0x4d, 0x51, 0x58}) // Masked text "Hello"

	f.Fuzz(func(t *testing.T, data []byte) {
		// Just ensure it doesn't panic
		ParseWSFrame(data)
	})
}

func FuzzParseFrameHeader(f *testing.F) {
	// Add valid seed corpus (9 byte header)
	f.Add([]byte{0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}) 

	f.Fuzz(func(t *testing.T, data []byte) {
		parser := &HTTP2Parser{}
		// Just ensure it doesn't panic
		parser.ParseFrameHeader(data, 0, uint32(len(data)))
	})
}
