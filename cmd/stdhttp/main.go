package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "online",
			"engine": "net-http-tls",
		})
	})

	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		// Basic parameter extraction for /users/:id
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User ID requested: " + id))
	})

	mux.HandleFunc("/api/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "received",
			"bytes":  len(body),
		})
	})

	log.Println("Starting standard library server on port 8083...")
	// We run on 8083 to avoid port conflicts during side-by-side comparisons
	// For standard library to use the PFX, we usually need to convert it to PEM.
	// We'll see if ListenAndServeTLS accepts it (it usually expects PEM).
	// But Wait, EventHorizon uses "cng_cert.pfx". Let's try it.
	err := http.ListenAndServeTLS(":8083", "cert.pem", "key.pem", mux)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
