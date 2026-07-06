package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/eventhorizon/pkg/eventhorizon"
	"golang.org/x/sys/windows"
)

// ServerStatus represents the response for the /api/status endpoint
type ServerStatus struct {
	Uptime      string `json:"uptime"`
	Connections int64  `json:"connections"`
	Version     string `json:"version"`
}

func main() {
	// 1. Generate the 5MB dummy file for the /download endpoint
	dummyPath := "./cmd/showcase/public/largefile.txt"
	if _, err := os.Stat(dummyPath); os.IsNotExist(err) {
		log.Println("Generating 5MB dummy file...")
		dummyContent := make([]byte, 5*1024*1024)
		for i := range dummyContent {
			dummyContent[i] = 'A' + byte(i%26)
		}
		if err := os.WriteFile(dummyPath, dummyContent, 0644); err != nil {
			log.Fatalf("Failed to write dummy file: %v", err)
		}
	}

	app := eventhorizon.New(8082)
	startTime := time.Now()

	// Global map to hold active WebSocket clients for broadcasting
	var clients sync.Map // map[windows.Handle]*eventhorizon.Context

	// Route 1 (Static): Serve the root '/' to deliver './public/index.html'
	app.GET("/", func(c *eventhorizon.Context) {
		if err := c.SendFile("./cmd/showcase/public/index.html"); err != nil {
			fmt.Println("ReadFile index.html error:", err)
		}
	})

	// Route 1b (Static Large File): Showcase the chunked disk reader
	app.GET("/download", func(c *eventhorizon.Context) {
		if err := c.SendFile("./cmd/showcase/public/largefile.txt"); err != nil {
			fmt.Println("ReadFile largefile.txt error:", err)
		}
	})

	// Route 2 (JSON API): Mock server stats
	app.GET("/api/status", func(c *eventhorizon.Context) {
		// In a real scenario we'd pull from atomic metrics
		activeCount := int64(0)
		clients.Range(func(key, value interface{}) bool {
			activeCount++
			return true
		})
		
		status := ServerStatus{
			Uptime:      time.Since(startTime).String(),
			Connections: activeCount,
			Version:     "1.0-ZeroAlloc",
		}
		
		if err := c.JSON(status); err != nil {
			fmt.Println("JSON serialization error:", err)
		}
	})

	// Route 3 (Radix Parameters): Lookup users by ID
	app.GET("/api/users/:id", func(c *eventhorizon.Context) {
		id := c.Param("id")
		
		// Return JSON using an anonymous struct
		response := struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		}{
			ID:   id,
			Name: "User " + id,
			Role: "EventHorizon Tester",
		}
		
		if err := c.JSON(response); err != nil {
			fmt.Println("JSON serialization error:", err)
		}
	})

	// Route 4 (WebSockets): Real-time chat hub
	app.WS("/ws", func(c *eventhorizon.Context, frame []byte) {
		handle := c.Conn.Socket
		clients.Store(handle, c)

		// log.Printf("Received message from client [%v]: %s", handle, string(frame))

		// Broadcast to all clients
		clients.Range(func(key, value interface{}) bool {
			clientCtx := value.(*eventhorizon.Context)
			expectedHandle := key.(windows.Handle)
			
			if clientCtx.Conn.Socket != windows.InvalidHandle && clientCtx.Conn.Socket == expectedHandle {
				// Opcode 1 = Text Frame
				err := clientCtx.WriteWSFrame(1, frame)
				if err != nil {
					clients.Delete(key)
				}
			} else {
				clients.Delete(key)
			}
			return true
		})
	})

	log.Println("Listening on https://127.0.0.1:8082")
	if err := app.Listen("cert.pem", "password"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
