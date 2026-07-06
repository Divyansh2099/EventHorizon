package main

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	target := "127.0.0.1:8080"
	req := []byte("GET / HTTP/1.1\r\nHost: localhost:8080\r\nConnection: close\r\n\r\n")

	concurrency := 100
	var requestsSent uint64

	log.Printf("Starting load generator against %s with %d concurrent workers...", target, concurrency)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			buf := make([]byte, 1024)
			for {
				conn, err := net.Dial("tcp", target)
				if err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}

				_, err = conn.Write(req)
				if err == nil {
					conn.Read(buf) // wait for response
					atomic.AddUint64(&requestsSent, 1)
				}
				conn.Close()
			}
		}()
	}

	// Print stats every second
	go func() {
		for {
			time.Sleep(time.Second)
			sent := atomic.SwapUint64(&requestsSent, 0)
			log.Printf("Throughput: %d req/sec", sent)
		}
	}()

	wg.Wait()
}
