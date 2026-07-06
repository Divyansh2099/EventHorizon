package main

import (
	"log"
	"sync"
	"golang.org/x/sys/windows"

	"github.com/eventhorizon/pkg/eventhorizon"
	"fmt"
)

func main() {
	app := eventhorizon.New(8082)

	// Global map to hold active clients for broadcasting
	var clients sync.Map // map[windows.Handle]*eventhorizon.Context

	// Serve the stunning Glassmorphism UI
	app.GET("/", func(c *eventhorizon.Context) {
		if err := c.SendFile("./cmd/chat/public/index.html"); err != nil {
			fmt.Println("ReadFile error:", err)
			return
		}
	})

	// Global WebSocket handler (echoes incoming messages to all connected clients)
	app.WS("/ws", func(c *eventhorizon.Context, frame []byte) {
		
		// 1. Register the new client if we haven't seen this socket handle yet.
		// (In a real app, you'd register on connect and remove on disconnect. 
		// For zero-allocation, we lazily register them on their first message.)
		handle := c.Conn.Socket
		clients.Store(handle, c)

		log.Printf("Received message from client [%v]: %s", handle, string(frame))

		// 2. Broadcast the EXACT byte slice (0 heap allocs for string conversion)
		// to every single registered client.
		clients.Range(func(key, value interface{}) bool {
			clientCtx := value.(*eventhorizon.Context)
			expectedHandle := key.(windows.Handle)
			
			// We check if the socket is still valid AND matches the original handle.
			// Because Conn objects are pooled, if a socket closes and the Conn is reused
			// for a new client before we clean it up, the Socket handle will change.
			if clientCtx.Conn.Socket != windows.InvalidHandle && clientCtx.Conn.Socket == expectedHandle {
				// Opcode 1 = Text Frame
				err := clientCtx.WriteWSFrame(1, frame)
				if err != nil {
					clients.Delete(key) // clean up dead connections
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
