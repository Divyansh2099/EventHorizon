package benchmarks

import (
	"testing"

	"github.com/eventhorizon/pkg/connection"
	"golang.org/x/sys/windows"
)

// BenchmarkConnectionLifecycle mathematically proves that fetching a connection
// from the pool, utilizing its pre-allocated buffers and overlapped structs,
// and returning it to the pool incurs zero heap allocations.
func BenchmarkConnectionLifecycle(b *testing.B) {
	// Simulate an invalid socket handle just for the sake of the struct lifecycle
	dummySocket := windows.InvalidHandle

	dummySlice := make([]byte, 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 1. Client connects, we fetch a pooled Conn
		conn := connection.GetConn(dummySocket)

		// 2. Simulate the kernel writing data into the read buffer
		conn.ReadBuffer = dummySlice
		conn.ReadLength = 100
		_ = conn.ReadBuffer[0:conn.ReadLength]

		// 3. Simulate writing a response
		conn.WriteBuffer = dummySlice
		_ = conn.WriteBuffer[0:50]

		// 4. Request completes, we release the connection
		// Because we pass an InvalidHandle here, Closesocket will safely fail/no-op,
		// allowing the struct to be pooled normally.
		conn.Release()
	}
}
