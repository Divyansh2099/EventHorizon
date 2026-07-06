package benchmarks

import (
	"testing"

	"github.com/eventhorizon/pkg/pool"
)

// BenchmarkBufferPool measures the speed of getting and putting
// a fixed-size buffer from the sync.Pool, proving zero allocs.
func BenchmarkBufferPool(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		buf := pool.GetBuffer()
		pool.PutBuffer(buf)
	}
}
