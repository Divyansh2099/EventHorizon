package connection

import (
	"runtime"
	"unsafe"

	"github.com/eventhorizon/pkg/iocp"
	"github.com/eventhorizon/pkg/metrics"
	"github.com/eventhorizon/pkg/pool"
	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/rio"
	"github.com/eventhorizon/pkg/winsock"
	"golang.org/x/sys/windows"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eventhorizon/pkg/tls"
)

// ActiveTracker globally maps active windows.Handle to *Conn for background timeout sweeping.
var ActiveTracker sync.Map

// State represents the current state of a connection in the event machine.
type State int

const (
	StateIdle State = iota
	StateReading
	StateHandshake
	StateParsing
	StateWriting
	StateWebSocket
	StateClosed
)

// Conn represents an active client connection.
// It pre-allocates and embeds the structures needed for Overlapped I/O,
// ensuring zero heap allocations occur during the request processing lifecycle.
type Conn struct {
	Socket windows.Handle
	State  State

	// Overlapped structures for accepting, reading, and writing.
	// These are passed to the Windows kernel. Because they are embedded directly
	// into the Conn struct, we avoid allocating them dynamically per-request.
	AcceptOverlapped iocp.OverlappedCtx
	ReadOverlapped   iocp.OverlappedCtx
	WriteOverlapped  iocp.OverlappedCtx

	// Buffers are drawn from the centralized memory pool to avoid allocations.
	// AcceptBuffer is used specifically for AcceptEx to store local and remote addresses.
	// Microsoft recommends adding 16 bytes to the maximum sockaddr length (which is ~28 bytes for IPv6).
	// We'll allocate a small static array for this to avoid pool overhead for 128 bytes.
	AcceptBuffer [128]byte

	ReadBuffer  []byte
	WriteBuffer []byte

	// Tracks the number of bytes populated in the ReadBuffer.
	ReadLength uint32
	// ReadCursor tracks where in the buffer parsing should start.
	ReadCursor uint32
	
	// WriteCursor tracks where in the WriteBuffer egress bytes are staged.
	WriteCursor uint32
	
	// KeepAlive indicates whether the connection should be recycled after a request.
	KeepAlive bool

	// IsWebSocket indicates if the socket has successfully upgraded protocols.
	IsWebSocket bool

	// WriteMu protects concurrent access to the WriteBuffer (e.g. for WS broadcasting).
	WriteMu sync.Mutex

	// TlsMu protects concurrent SSPI encryption/decryption on the same security context.
	TlsMu sync.Mutex

	// FileOverlapped tracks asynchronous disk reads.
	FileOverlapped iocp.OverlappedCtx

	// FileHandle for static file serving.
	FileHandle windows.Handle
	FileOffset uint64
	FileSize   uint64

	// RIO specific handles for zero-copy reads
	RioRQ          rio.RIO_RQ
	RioBuffer      rio.RIO_BUF
	RioWriteBuffer rio.RIO_BUF
	RioEngine      *rio.Engine

	// TransmitBuffers holds the headers/trailers for TransmitFile to avoid heap allocation.
	TransmitBuffers winsock.TransmitFileBuffers

	// StartTime stores the unix nanosecond timestamp when the request started.
	StartTime int64

	// TLS specific state
	TlsCtxt     tls.SecHandle
	StreamSizes tls.SecPkgContext_StreamSizes

	// HTTP/2 specific state
	Protocol    string
	H2Parser    parser.HTTP2Parser
	Streams     [100]parser.StreamCtx
	StreamCount uint32

	// TransmittedFile flag to indicate that we've initiated a zero-copy sendfile.
	TransmittedFile bool

	// LastActive holds the UnixNano timestamp of the last I/O operation.
	LastActive int64

	// pinner is used to prevent the Go Garbage Collector from moving this struct
	// while kernel asynchronous operations hold pointers to it.
	pinner runtime.Pinner
}

func init() {
	pool.ConnPool.New = func() any {
		c := &Conn{}
		// Initialize the self-referencing pointers and operation types.
		// This happens exactly once when the object is created by the pool.
		c.AcceptOverlapped.Op = iocp.OpAccept
		c.AcceptOverlapped.Conn = unsafe.Pointer(c)

		c.ReadOverlapped.Op = iocp.OpRead
		c.ReadOverlapped.Conn = unsafe.Pointer(c)

		c.WriteOverlapped.Op = iocp.OpWrite
		c.WriteOverlapped.Conn = unsafe.Pointer(c)

		c.FileOverlapped.Op = iocp.OpFileRead
		c.FileOverlapped.Conn = unsafe.Pointer(c)
		return c
	}
}

// GetConn retrieves a connection object from the pool.
// It initializes the socket, resets the state, and acquires buffers.
func GetConn(socket windows.Handle) *Conn {
	c := pool.ConnPool.Get().(*Conn)
	
	atomic.AddInt64(&metrics.Global.ConnsActive, 1)

	// Pin the connection object in memory so the garbage collector
	// doesn't free or move it while async IOCP calls are pending.
	c.pinner.Pin(c)

	c.Socket = socket
	c.State = StateIdle
	c.ReadLength = 0
	c.KeepAlive = false
	c.IsWebSocket = false
	c.FileHandle = windows.InvalidHandle
	c.FileOffset = 0
	c.FileSize = 0
	c.LastActive = time.Now().UnixNano()

	// Track the active connection for the Slowloris sweeper.
	ActiveTracker.Store(socket, c)

	// ReadBuffer will be dynamically mapped to the hardware RIO slice inside postRead.
	c.ReadBuffer = nil
	
	// If the connection already has a RIO chunk, map it now. Otherwise it will be mapped in postRead.
	if c.RioWriteBuffer.BufferId != 0 {
		// Wait, we need rioEngine to map it! But Conn doesn't have rioEngine!
		// It's safer to just reset RioWriteBuffer and let postRead fetch a new one!
	}
	c.RioWriteBuffer = rio.RIO_BUF{}
	c.RioBuffer = rio.RIO_BUF{}
	c.WriteBuffer = nil

	// TLS contexts are 0 by default. They are managed in handler.go.
	c.TlsCtxt = tls.SecHandle{}
	c.StreamSizes = tls.SecPkgContext_StreamSizes{}

	// Reset the underlying OS Overlapped state. For TCP stream sockets,
	// offsets are ignored, but it's crucial to clear the Internal fields
	// before issuing a new overlapping call.
	c.AcceptOverlapped.Overlapped = windows.Overlapped{}
	c.ReadOverlapped.Overlapped = windows.Overlapped{}
	c.WriteOverlapped.Overlapped = windows.Overlapped{}
	c.FileOverlapped.Overlapped = windows.Overlapped{}
	c.ReadOverlapped.Op = iocp.OpRead
	c.WriteOverlapped.Op = iocp.OpWrite
	c.FileOverlapped.Op = iocp.OpFileRead

	return c
}

// Release returns the connection and its associated resources to their
// respective pools. It safely closes the socket. It must be called exactly
// once when the connection lifecycle is complete.
func (c *Conn) Release() {
	if c.State == StateClosed {
		return
	}
	if c.State != StateIdle {
		atomic.AddInt64(&metrics.Global.ActiveHandlers, -1)
		metrics.BackpressureCond.Signal()
	}
	c.State = StateClosed
	
	atomic.AddInt64(&metrics.Global.ConnsActive, -1)

	// Close the native socket handle.
	if c.Socket != windows.InvalidHandle {
		ActiveTracker.Delete(c.Socket)
		windows.Closesocket(c.Socket)
		c.Socket = windows.InvalidHandle
	}

	// Return the RIO chunk to the custom allocator if one was acquired.
	if c.RioBuffer.BufferId != 0 && c.RioEngine != nil {
		c.RioEngine.PutChunk(c.RioBuffer)
	}
	if c.RioWriteBuffer.BufferId != 0 && c.RioEngine != nil {
		c.RioEngine.PutChunk(c.RioWriteBuffer)
	}
	c.RioRQ = 0
	c.RioBuffer = rio.RIO_BUF{}
	c.RioWriteBuffer = rio.RIO_BUF{}

	if c.FileHandle != windows.InvalidHandle {
		windows.CloseHandle(c.FileHandle)
		c.FileHandle = windows.InvalidHandle
	}

	// Clear the ReadBuffer slice mapping. (The RIO chunk will be returned separately)
	c.ReadBuffer = nil

	if c.WriteBuffer != nil {
		c.WriteBuffer = nil
	}
	
	c.ReadLength = 0
	c.ReadCursor = 0
	c.WriteCursor = 0

	// Free the TLS Context if one was allocated
	tls.DeleteContext(&c.TlsCtxt)

	// Return the connection struct itself to the connection pool.
	c.pinner.Unpin()
	pool.ConnPool.Put(c)
}
