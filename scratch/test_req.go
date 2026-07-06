//go:build ignore

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost: 10,
		},
	}

	// Warmup
	for i := 0; i < 5; i++ {
		resp, err := client.Get("https://127.0.0.1:8082/users/99")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	start := time.Now()
	for i := 0; i < 1000; i++ {
		resp, err := client.Get("https://127.0.0.1:8082/users/99")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	elapsed := time.Since(start)
	fmt.Printf("1000 requests took: %s\n", elapsed)
	fmt.Printf("Latency per request: %s\n", elapsed/1000)
}
