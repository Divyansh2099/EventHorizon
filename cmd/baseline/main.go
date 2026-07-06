package main

import (
	"log"
	"net/http"
)

func main() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello, World!"))
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/api/shallow", handler)
	http.HandleFunc("/api/v1/nodes/leaf/item/details", handler)
	http.HandleFunc("/api/stream-large", handler)
	http.HandleFunc("/api/upload", handler)

	log.Println("Starting Baseline Go net/http Server on :8083")
	if err := http.ListenAndServe(":8083", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
