package metrics

import (
	"sort"
	"sync/atomic"
	"time"
)

// TelemetryTracker provides zero-allocation p50/p95/p99 latency tracking.
type TelemetryTracker struct {
	Index  uint64
	Window [100000]uint64
	
	// Calculated percentiles in nanoseconds
	P50 uint64
	P95 uint64
	P99 uint64
}

// Record safely records a latency measurement into the circular buffer.
func (t *TelemetryTracker) Record(nanos uint64) {
	idx := atomic.AddUint64(&t.Index, 1) % 100000
	atomic.StoreUint64(&t.Window[idx], nanos)
}

// StartSupervisor launches the low-priority background loop to calculate percentiles.
func (t *TelemetryTracker) StartSupervisor() {
	go func() {
		// Allocate a single scratch slice once to avoid heap allocations on every tick.
		scratch := make([]uint64, 100000)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Copy the current window to our scratch buffer
			// Because the worker threads are constantly writing via atomic, we do a best-effort
			// copy here without locking. In a ring buffer, this is acceptable for statistical percentiles.
			for i := 0; i < 100000; i++ {
				scratch[i] = atomic.LoadUint64(&t.Window[i])
			}

			// In-place sort the scratch buffer
			sort.Slice(scratch, func(i, j int) bool {
				return scratch[i] < scratch[j]
			})

			// Find the actual number of recorded samples (ignore leading zeros if not full)
			count := 0
			for count < 100000 && scratch[count] == 0 {
				count++
			}
			
			validSamples := 100000 - count
			if validSamples > 0 {
				p50Idx := count + int(float64(validSamples)*0.50)
				p95Idx := count + int(float64(validSamples)*0.95)
				p99Idx := count + int(float64(validSamples)*0.99)

				// Ensure bounds
				if p50Idx >= 100000 { p50Idx = 99999 }
				if p95Idx >= 100000 { p95Idx = 99999 }
				if p99Idx >= 100000 { p99Idx = 99999 }

				atomic.StoreUint64(&t.P50, scratch[p50Idx])
				atomic.StoreUint64(&t.P95, scratch[p95Idx])
				atomic.StoreUint64(&t.P99, scratch[p99Idx])
			}
		}
	}()
}
