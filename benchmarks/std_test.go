package benchmarks

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkStdRouterLookup measures throughput and allocations for standard library router.
func BenchmarkStdRouterLookup(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler, _ := mux.Handler(req)
		if handler == nil {
			b.Fatal("handler not found")
		}
	}
}

// BenchmarkStdRouterLookupDeep measures throughput for a deep path in stdlib.
func BenchmarkStdRouterLookupDeep(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/users/info/settings/profile/avatar", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/api/v1/users/info/settings/profile/avatar", nil)
	
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler, _ := mux.Handler(req)
		if handler == nil {
			b.Fatal("handler not found")
		}
	}
}

// BenchmarkStdParser measures standard library HTTP parsing
func BenchmarkStdParser(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(rawRequest)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bufio.NewReader(bytes.NewReader(rawRequest))
		req, err := http.ReadRequest(reader)
		if err != nil {
			b.Fatal(err)
		}
		if req == nil {
			b.Fatal("failed to parse")
		}
	}
}

// BenchmarkStdParserShort measures standard library HTTP parsing on short request
func BenchmarkStdParserShort(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(shortRequest)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bufio.NewReader(bytes.NewReader(shortRequest))
		req, err := http.ReadRequest(reader)
		if err != nil {
			b.Fatal(err)
		}
		if req == nil {
			b.Fatal("failed to parse")
		}
	}
}
