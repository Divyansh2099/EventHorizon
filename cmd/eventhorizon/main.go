package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"golang.org/x/sys/windows"
	"github.com/eventhorizon/pkg/connection"
	"github.com/eventhorizon/pkg/metrics"
	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/server"
	"github.com/eventhorizon/pkg/tls"
)

func main() {
	// Start the out-of-band metrics stream and dashboard on port 8081
	go metrics.StartStreamer(":8081")

	port := 8082
	log.Printf("Starting EventHorizon on port %d...", port)

	// Phase 18: Initialize Schannel with our certificate
	if err := tls.InitSchannel("cng_cert.pfx", "password"); err != nil {
		fmt.Printf("Failed to initialize Schannel: %v\n", err)
	}

	srv := server.NewServer(port)
	
	handler := func(ctx *parser.RequestCtx) {
		ctx.SetStatusCode(200)
		if ctx.KeepAlive {
			ctx.Write([]byte("Content-Length: 13\r\nConnection: keep-alive\r\n\r\nHello, World!"))
		} else {
			ctx.Write([]byte("Content-Length: 13\r\nConnection: close\r\n\r\nHello, World!"))
		}
	}
	
	srv.Router.Handle("GET", "/", handler)
	srv.Router.Handle("GET", "/api/shallow", handler)
	srv.Router.Handle("GET", "/api/v1/nodes/leaf/item/details", handler)
	srv.Router.Handle("GET", "/api/stream-large", handler)
	srv.Router.Handle("POST", "/api/upload", handler)

	// Static file handler utilizing zero-copy TransmitFile
	srv.Router.Handle("GET", "/static", func(ctx *parser.RequestCtx) {
		pathPtr, err := windows.UTF16PtrFromString("ws_test.html")
		if err != nil {
			ctx.SetStatusCode(500)
			return
		}

		hFile, err := windows.CreateFile(
			pathPtr,
			windows.GENERIC_READ,
			windows.FILE_SHARE_READ,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED,
			0,
		)

		if err != nil {
			ctx.SetStatusCode(404)
			return
		}

		var fileInfo windows.ByHandleFileInformation
		err = windows.GetFileInformationByHandle(hFile, &fileInfo)
		if err != nil {
			windows.CloseHandle(hFile)
			ctx.SetStatusCode(500)
			return
		}

		// Prepare HTTP headers in WriteBuffer with Content-Length to prevent HTTP client hangs
		ctx.SetStatusCode(200)
		ctx.Write([]byte("Content-Type: text/html\r\nContent-Length: "))
		
		var numBuf [32]byte
		b := strconv.AppendUint(numBuf[:0], uint64(fileInfo.FileSizeHigh)<<32|uint64(fileInfo.FileSizeLow), 10)
		ctx.Write(b)

		if ctx.KeepAlive {
			ctx.Write([]byte("\r\nConnection: keep-alive\r\n\r\n"))
		} else {
			ctx.Write([]byte("\r\nConnection: close\r\n\r\n"))
		}

		fmt.Println("Static file route hit!")
		conn := (*connection.Conn)(ctx.Conn)

		// Bind the file handle to the server's IOCP
		if err := srv.AssociateFile(hFile, conn); err != nil {
			fmt.Println("AssociateFile failed:", err)
			windows.CloseHandle(hFile)
			ctx.SetStatusCode(500)
			return
		}

		conn.FileHandle = hFile
		conn.FileSize = uint64(fileInfo.FileSizeHigh)<<32 | uint64(fileInfo.FileSizeLow)
		conn.FileOffset = 0

		// Set flag so the server loop skips standard postWrite and initiates PostFileRead
		ctx.TransmittedFile = true
	})

	srv.Router.WSRoute = func(connPtr any, frame any) {
		conn := connPtr.(*connection.Conn)
		f := frame.(parser.WSFrame)
		
		if f.Opcode == 1 || f.Opcode == 2 { // Text or Binary
			if conn.WriteCursor == 0 {
				conn.WriteCursor = conn.StreamSizes.CbHeader
			}
			conn.WriteCursor = parser.WriteWSFrame(conn.WriteBuffer, conn.WriteCursor, f.Opcode, f.Payload)
		}
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("Server is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal to gracefully shut down the server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	srv.Stop()
	log.Println("Server stopped.")
}
