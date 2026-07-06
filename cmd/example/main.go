package main

import (
	"log"
	"github.com/eventhorizon/pkg/eventhorizon"
)

func main() {
	// Initialize a new EventHorizon application on port 8082
	app := eventhorizon.New(8082)

	// Standard Route
	app.GET("/", func(c *eventhorizon.Context) {
		c.SendString("Hello Fast World!")
	})

	// Parameterized Route (Zero-Allocation!)
	app.GET("/users/:id", func(c *eventhorizon.Context) {
		// ParamBytes retrieves the segment directly from the hardware pinned buffer
		id := c.ParamBytes("id")
		
		// We can echo it back directly 
		c.SendString("User ID requested: " + string(id))
	})

	// JSON Endpoint
	app.GET("/api/status", func(c *eventhorizon.Context) {
		c.JSON(map[string]interface{}{
			"status": "online",
			"engine": "rio-sspi-tls",
		})
	})

	// Static File Route (Zero-Copy TransmitFile mapping)
	app.GET("/static", func(c *eventhorizon.Context) {
		c.SendFile("ws_test.html")
	})

	// POST endpoint for heavy payloads
	app.POST("/api/submit", func(c *eventhorizon.Context) {
		c.JSON(map[string]interface{}{
			"status": "received",
			"bytes":  c.Req.Body.End - c.Req.Body.Start,
		})
	})

	// Global WebSocket Endpoint
	app.WS("/ws", func(c *eventhorizon.Context, frame []byte) {
		log.Printf("Received WS Message: %s\n", string(frame))
		// We don't have a direct c.SendWS method yet, but it echoes automatically for now.
	})

	// Start the Server securely
	log.Fatal(app.Listen("cng_cert.pfx", "password"))
}
