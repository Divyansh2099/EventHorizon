package pool

import (
	"testing"
)

// BenchmarkPoolManager tests the allocation overhead of the Zero-Allocation Pool Manager
func BenchmarkPoolManager(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Test Buffer Pool
		buf := GetBuffer()
		buf[0] = 1
		PutBuffer(buf)
	}
}

// TestZeroAllocations ensures exactly 0 allocs per run over 1000 iterations
func TestZeroAllocations(t *testing.T) {
	allocs := testing.AllocsPerRun(1000, func() {
		buf := GetBuffer()
		buf[0] = 1
		PutBuffer(buf)
	})

	if allocs != 0 {
		t.Fatalf("Expected 0 allocations, but got %f allocs/op", allocs)
	}
}
