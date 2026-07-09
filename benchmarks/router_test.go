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

// BenchmarkRouterLookupParametric measures throughput with a path parameter
func BenchmarkRouterLookupParametric(b *testing.B) {
	r := router.New()
	handler := func(ctx *parser.RequestCtx) {}
	
	r.Handle("GET", "/api/v1/users/:id", handler)
	
	method := []byte("GET")
	path := []byte("/api/v1/users/12345")
	
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

// BenchmarkRouterLookupDeep measures throughput with a deep static path
func BenchmarkRouterLookupDeep(b *testing.B) {
	r := router.New()
	handler := func(ctx *parser.RequestCtx) {}
	
	r.Handle("GET", "/api/v1/users/info/settings/profile/avatar", handler)
	
	method := []byte("GET")
	path := []byte("/api/v1/users/info/settings/profile/avatar")
	
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
