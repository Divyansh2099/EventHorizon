//go:build ignore

package main

import (
	"crypto/tls"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	u := url.URL{Scheme: "wss", Host: "127.0.0.1:8082", Path: "/ws"}
	log.Printf("Connecting to %s", u.String())

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Dial error:", err)
	}
	defer c.Close()

	// Send message
	err = c.WriteMessage(websocket.TextMessage, []byte("Hello from Go Client!"))
	if err != nil {
		log.Println("Write error:", err)
		return
	}
	log.Println("Message sent!")

	// Read message back
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := c.ReadMessage()
	if err != nil {
		log.Println("Read error:", err)
		return
	}
	log.Printf("Received: %s", message)
}
