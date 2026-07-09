package benchmarks

import (
	"testing"

	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/pool"
)

var rawRequest = []byte("GET /hello-world/api/v1/data HTTP/1.1\r\nHost: localhost:8080\r\nUser-Agent: wrk/4.2.0\r\nAccept: */*\r\nConnection: keep-alive\r\n\r\n")

// BenchmarkZeroCopyParser proves that parsing a standard HTTP request
// directly from the socket buffer incurs zero heap allocations. It also
// measures the raw throughput of the state machine.
func BenchmarkZeroCopyParser(b *testing.B) {
	p := parser.Parser{}
	var readBuf pool.Buffer
	copy(readBuf[:], rawRequest)
	var writeBuf pool.Buffer

	b.ReportAllocs()
	b.SetBytes(int64(len(rawRequest)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p.Reset()
		req := parser.GetRequestCtx(readBuf[:], writeBuf[:])

		// Parse in-place directly from the raw byte slice
		_, err := p.Parse(uint32(len(rawRequest)), req)
		if err != nil {
			b.Fatal(err)
		}

		// Ensure we parsed correctly (prevents compiler from optimizing the loop away)
		if req.Method.End == 0 {
			b.Fatal("failed to parse")
		}

		// Release back to the pool
		req.Release()
	}
}

var shortRequest = []byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")

// BenchmarkZeroCopyParserShort proves that parsing a short standard HTTP request
// directly from the socket buffer incurs zero heap allocations.
func BenchmarkZeroCopyParserShort(b *testing.B) {
	p := parser.Parser{}
	var readBuf pool.Buffer
	copy(readBuf[:], shortRequest)
	var writeBuf pool.Buffer

	b.ReportAllocs()
	b.SetBytes(int64(len(shortRequest)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p.Reset()
		req := parser.GetRequestCtx(readBuf[:], writeBuf[:])

		_, err := p.Parse(uint32(len(shortRequest)), req)
		if err != nil {
			b.Fatal(err)
		}

		if req.Method.End == 0 {
			b.Fatal("failed to parse")
		}

		req.Release()
	}
}
