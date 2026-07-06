package metrics

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds global atomic counters grouped by abstraction layers.
type Metrics struct {
	// OS Kernel I/O layer
	Accepts uint64 `json:"accepts"`
	Reads   uint64 `json:"reads"`
	Writes  uint64 `json:"writes"`
	BytesIn uint64 `json:"bytesIn"`
	BytesOut uint64 `json:"bytesOut"`

	// Memory Pool layer
	ConnsActive    int64 `json:"connsActive"`
	ActiveHandlers int64 `json:"activeHandlers"`

	// HTTP Parser layer
	RequestsParsed uint64 `json:"requestsParsed"`
	ParserErrors   uint64 `json:"parserErrors"`

	// Telemetry
	Tracker *TelemetryTracker `json:"-"`

	// Expose percentiles to dashboard (in nanoseconds)
	P50 uint64 `json:"p50"`
	P95 uint64 `json:"p95"`
	P99 uint64 `json:"p99"`
}

var Global = Metrics{
	Tracker: &TelemetryTracker{},
}

var (
	BackpressureMu   sync.Mutex
	BackpressureCond = sync.NewCond(&BackpressureMu)
)

// StartStreamer spins up a dedicated lightweight HTTP server to stream metrics via SSE.
func StartStreamer(addr string) {
	Global.Tracker.StartSupervisor()

	mux := http.NewServeMux()
	
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for dashboard local development
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		// Force immediate header flush so the client connects instantly
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				// Capture current state safely (atomic loads prevent tearing, though not strictly required for JSON marshaling speed here)
				m := Metrics{
					Accepts:        atomic.LoadUint64(&Global.Accepts),
					Reads:          atomic.LoadUint64(&Global.Reads),
					Writes:         atomic.LoadUint64(&Global.Writes),
					BytesIn:        atomic.LoadUint64(&Global.BytesIn),
					BytesOut:       atomic.LoadUint64(&Global.BytesOut),
					ConnsActive:    atomic.LoadInt64(&Global.ConnsActive),
					RequestsParsed: atomic.LoadUint64(&Global.RequestsParsed),
					ParserErrors:   atomic.LoadUint64(&Global.ParserErrors),
					P50:            atomic.LoadUint64(&Global.Tracker.P50),
					P95:            atomic.LoadUint64(&Global.Tracker.P95),
					P99:            atomic.LoadUint64(&Global.Tracker.P99),
				}
				
				data, _ := json.Marshal(m)
				_, err := w.Write([]byte("data: " + string(data) + "\n\n"))
				if err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})

	mux.Handle("/", http.FileServer(http.Dir("./ui/dist")))

	log.Printf("Starting lightweight metrics stream and dashboard on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Metrics streamer failed: %v", err)
	}
}
