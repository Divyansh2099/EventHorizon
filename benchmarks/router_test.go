package benchmarks

import (
	"testing"

	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/router"
)

// BenchmarkRouterLookup measures the throughput and zero-allocation properties
// of the routing tree.
func BenchmarkRouterLookup(b *testing.B) {
	r := router.New()
	handler := func(ctx *parser.RequestCtx) {}
	
	// Register route
	r.Handle("GET", "/api/v1/users", handler)
	
	method := []byte("GET")
	path := []byte("/api/v1/users")
	
	b.ReportAllocs()
	b.ResetTimer()
	
	req := &parser.RequestCtx{}
	for i := 0; i < b.N; i++ {
		h := r.Lookup(method, path, req)
		if h == nil {
			b.Fatal("handler not found")
		}
	}
}
