package rio

import (
	"fmt"
	"golang.org/x/sys/windows"
	"sync"
	"syscall"
	"unsafe"
)

var (
	// RioTable stores the global function pointers for Registered I/O.
	RioTable RIO_EXTENSION_FUNCTION_TABLE
)

// Engine manages the RIO state, memory allocation, and hardware registration.
type Engine struct {
	bufferSize uint32
	chunkSize  uint32
	memory     uintptr
	bufferId   RIO_BUFFERID

	// Allocator state
	mu         sync.Mutex
	freeChunks []uint32 // stores offsets of available RIO chunks
}

// NewEngine creates a new RIO engine configuration.
func NewEngine(bufferSize, chunkSize uint32) *Engine {
	return &Engine{
		bufferSize: bufferSize,
		chunkSize:  chunkSize,
	}
}

// Init loads the RIO extension functions, allocates page-locked memory,
// and registers the buffer with the Windows NT kernel.
func (e *Engine) Init() error {
	// 1. Create a dummy socket to load the extension function table
	socket, err := windows.WSASocket(
		syscall.AF_INET,
		syscall.SOCK_STREAM,
		syscall.IPPROTO_TCP,
		nil,
		0,
		windows.WSA_FLAG_OVERLAPPED,
	)
	if err != nil {
		return fmt.Errorf("WSASocket failed: %w", err)
	}
	defer windows.Closesocket(socket)

	// 2. Load RIO function table via WSAIoctl
	var bytesReturned uint32
	err = windows.WSAIoctl(
		socket,
		SIO_GET_MULTIPLE_EXTENSION_FUNCTION_POINTER,
		(*byte)(unsafe.Pointer(&WSAID_MULTIPLE_RIO)),
		uint32(unsafe.Sizeof(WSAID_MULTIPLE_RIO)),
		(*byte)(unsafe.Pointer(&RioTable)),
		uint32(unsafe.Sizeof(RioTable)),
		&bytesReturned,
		nil,
		0,
	)
	if err != nil {
		return fmt.Errorf("WSAIoctl failed to load RIO table: %w", err)
	}

	// 3. Allocate contiguous virtual memory
	mem, err := windows.VirtualAlloc(
		0,
		uintptr(e.bufferSize),
		windows.MEM_COMMIT|windows.MEM_RESERVE,
		windows.PAGE_READWRITE,
	)
	if err != nil {
		return fmt.Errorf("VirtualAlloc failed: %w", err)
	}
	e.memory = mem

	// 4. Register the buffer with the Windows NT kernel (locks memory pages)
	ret, _, errSys := syscall.SyscallN(
		RioTable.RIORegisterBuffer,
		e.memory,
		uintptr(e.bufferSize),
	)
	// RIO_INVALID_BUFFERID is defined as 0xFFFFFFFF (or ^uintptr(0))
	if ret == ^uintptr(0) {
		windows.VirtualFree(e.memory, 0, windows.MEM_RELEASE)
		e.memory = 0
		return fmt.Errorf("RIORegisterBuffer failed: %v", errSys)
	}
	e.bufferId = RIO_BUFFERID(ret)

	// 5. Initialize the zero-allocation chunk manager
	numChunks := e.bufferSize / e.chunkSize
	e.freeChunks = make([]uint32, 0, numChunks)
	for i := uint32(0); i < numChunks; i++ {
		e.freeChunks = append(e.freeChunks, i*e.chunkSize)
	}

	return nil
}

// GetChunk retrieves a registered memory chunk, bypassing the Go Garbage Collector.
func (e *Engine) GetChunk() (RIO_BUF, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.freeChunks) == 0 {
		return RIO_BUF{}, fmt.Errorf("out of RIO memory chunks")
	}

	// Pop the last chunk (fast O(1) operation)
	offset := e.freeChunks[len(e.freeChunks)-1]
	e.freeChunks = e.freeChunks[:len(e.freeChunks)-1]

	return RIO_BUF{
		BufferId: e.bufferId,
		Offset:   offset,
		Length:   e.chunkSize, // Initial length is the capacity of the chunk
	}, nil
}

// PutChunk returns a registered chunk to the allocator pool.
func (e *Engine) PutChunk(buf RIO_BUF) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.freeChunks = append(e.freeChunks, buf.Offset)
}

// GetSlice returns a zero-copy Go byte slice mapped directly to the hardware-registered memory.
func (e *Engine) GetSlice(buf RIO_BUF) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(e.memory+uintptr(buf.Offset))), buf.Length)
}

// Close safely unregisters hardware memory blocks and releases virtual memory.
func (e *Engine) Close() error {
	var errFinal error
	if e.bufferId != 0 && e.bufferId != RIO_BUFFERID(^uintptr(0)) {
		ret, _, errSys := syscall.SyscallN(RioTable.RIODeregisterBuffer, uintptr(e.bufferId))
		if ret != 0 {
			errFinal = fmt.Errorf("RIODeregisterBuffer failed: %v", errSys)
		}
	}
	if e.memory != 0 {
		err := windows.VirtualFree(e.memory, 0, windows.MEM_RELEASE)
		if err != nil && errFinal == nil {
			errFinal = fmt.Errorf("VirtualFree failed: %w", err)
		}
	}
	return errFinal
}
