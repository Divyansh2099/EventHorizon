package rio

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestEngine_InitAndClose(t *testing.T) {
	// Initialize Winsock for the test environment
	var data windows.WSAData
	err := windows.WSAStartup(uint32(2|2<<8), &data)
	if err != nil {
		t.Fatalf("WSAStartup failed: %v", err)
	}
	defer windows.WSACleanup()

	// 256MB is typical for high-throughput, but we'll use 16MB for unit testing
	// to avoid destroying CI environments while still proving VirtualAlloc and RIO work.
	bufferSize := uint32(16 * 1024 * 1024)
	chunkSize := uint32(4096)

	engine := NewEngine(bufferSize, chunkSize)
	err = engine.Init()
	if err != nil {
		t.Fatalf("Failed to initialize RIO engine: %v", err)
	}

	// Verify we can get a chunk successfully
	chunk, err := engine.GetChunk()
	if err != nil {
		t.Fatalf("Failed to get RIO chunk: %v", err)
	}

	// Verify the chunk has valid properties
	if chunk.BufferId == 0 || chunk.BufferId == RIO_BUFFERID(^uintptr(0)) {
		t.Errorf("Invalid RIO BufferID returned from allocator")
	}
	if chunk.Length != chunkSize {
		t.Errorf("Expected chunk length %d, got %d", chunkSize, chunk.Length)
	}

	// Return the chunk to the allocator
	engine.PutChunk(chunk)

	// Clean up the engine (deregisters hardware memory and frees VirtualAlloc)
	err = engine.Close()
	if err != nil {
		t.Fatalf("Failed to close RIO engine safely: %v", err)
	}
}
