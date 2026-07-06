package server

import (
	"fmt"
	"runtime"
	"sync"

	"unsafe"

	"github.com/eventhorizon/pkg/connection"
	"github.com/eventhorizon/pkg/iocp"
	
	"github.com/eventhorizon/pkg/metrics"
	"github.com/eventhorizon/pkg/rio"
	"github.com/eventhorizon/pkg/router"
	"golang.org/x/sys/windows"
	"sync/atomic"
	"time"
)

// Server coordinates the IOCP engine, memory pools, and parser.
type Server struct {
	port       int
	listener   windows.Handle
	iocpPort   *iocp.Port
	acceptEx   uintptr
	rioEngine  *rio.Engine
	done       chan struct{}

	// WaitGroup to cleanly shut down worker goroutines.
	wg sync.WaitGroup

	// The number of concurrent AcceptEx calls kept pending in the kernel.
	// This ensures there's always a socket ready when a client connects,
	// bypassing the traditional single-threaded accept() bottleneck.
	acceptPoolSize int

	// router holds the lock-free route tree.
	Router *router.Router
	
	// numWorkers dictates how many worker goroutines are spawned to process IOCP completions.
	numWorkers int

	// MaxConcurrentConnections restricts the number of active handlers to prevent memory exhaustion.
	MaxConcurrentConnections int64
}

// NewServer initializes a Server instance.
func NewServer(port int) *Server {
	poolSize := 1000
	return &Server{
		port:           port,
		acceptPoolSize: poolSize, // High-performance queue depth to prevent starvation under burst load
		Router:         router.New(),
		done:           make(chan struct{}),
		numWorkers:     runtime.NumCPU() * 4, // Overprovision to prevent starvation when GetQueuedCompletionStatus blocks OS threads
		MaxConcurrentConnections: 50000,
	}
}

// Start begins the event loop. It initializes WinSock, binds the port,
// creates the IOCP queue, spawns workers, and posts the initial AcceptEx calls.
func (s *Server) Start() error {
	if err := iocp.Init(); err != nil {
		return fmt.Errorf("iocp.Init failed: %w", err)
	}

	listener, err := iocp.CreateIOCPSocket()
	if err != nil {
		return fmt.Errorf("CreateIOCPSocket failed: %w", err)
	}
	s.listener = listener

	if err := iocp.BindAndListen(s.listener, s.port); err != nil {
		return fmt.Errorf("BindAndListen failed: %w", err)
	}

	// Create the primary I/O Completion Port.
	port, err := iocp.CreatePort()
	if err != nil {
		return err
	}
	s.iocpPort = port

	// Associate the listener socket with the IOCP.
	// We use 0 as the completion key since AcceptEx completions are identified
	// by the Overlapped pointer, not the completion key.
	if err := s.iocpPort.Associate(s.listener, 0); err != nil {
		return err
	}

	// Initialize the RIO Engine
	// 256MB contiguous buffer, chunked into 4096-byte RIO_BUF segments.
	s.rioEngine = rio.NewEngine(256*1024*1024, 4096)
	if err := s.rioEngine.Init(); err != nil {
		return fmt.Errorf("rioEngine.Init failed: %w", err)
	}

	// Create global RIO Completion Queue
	// We allocate a CQ with sufficient slots to match our concurrent connection limits.
	if err := s.iocpPort.InitRIO(&rio.RioTable, uint32(s.MaxConcurrentConnections*2)); err != nil {
		return fmt.Errorf("iocpPort.InitRIO failed: %w", err)
	}

	// Spawn worker goroutines. While 1 per logical core is standard,
	// because GetQueuedCompletionStatus blocks the underlying OS thread,
	// overprovisioning ensures there is always a thread ready to wake up
	// when the kernel signals a completion event.
	s.wg.Add(s.numWorkers)
	for i := 0; i < s.numWorkers; i++ {
		go func() {
			defer s.wg.Done()
			s.iocpPort.RunWorker(s.handleIO) // defined in handler.go
		}()
	}

	s.iocpPort.AcceptHandler = s.OnAccept

	// Pre-post the initial batch of async accepts using lightweight structs in the kernel.
	if err := s.iocpPort.PrePostAcceptBatch(s.listener, s.acceptPoolSize); err != nil {
		return fmt.Errorf("PrePostAcceptBatch failed: %w", err)
	}

	// Phase 26: Start the Slowloris connection sweeper.
	s.wg.Add(1)
	go s.sweeper()

	return nil
}

// AssociateFile binds a file handle to the underlying IOCP worker threads.
func (s *Server) AssociateFile(hFile windows.Handle, conn *connection.Conn) error {
	compKey := uintptr(unsafe.Pointer(conn))
	return s.iocpPort.Associate(hFile, compKey)
}

// OnAccept is called directly by the IOCP engine worker when a new client connects.
func (s *Server) OnAccept(acceptSocket windows.Handle) {
	atomic.AddInt64(&metrics.Global.ActiveHandlers, 1)
	atomic.AddUint64(&metrics.Global.Accepts, 1)

	// Inherit the listening socket's context. This is required for AcceptEx sockets
	// to function identically to sockets returned by a standard accept().
	err := windows.Setsockopt(acceptSocket, windows.SOL_SOCKET, windows.SO_UPDATE_ACCEPT_CONTEXT, (*byte)(unsafe.Pointer(&s.listener)), int32(unsafe.Sizeof(s.listener)))
	if err != nil {
		windows.Closesocket(acceptSocket)
		return
	}

	// Retrieve a zero-allocation connection object from our memory pool.
	conn := connection.GetConn(acceptSocket)

	// Create a dedicated RIO Request Queue for this new client socket, binding it to the global RIO CQ.
	rioRQ, err := s.iocpPort.CreateRequestQueue(acceptSocket)
	if err != nil {
		conn.Release()
		windows.Closesocket(acceptSocket)
		return
	}
	conn.RioRQ = rioRQ
	conn.RioEngine = s.rioEngine

	compKey := uintptr(unsafe.Pointer(conn))
	if assocErr := s.iocpPort.Associate(conn.Socket, compKey); assocErr != nil {
		conn.Release()
		return
	}

	// Advance the state machine and issue the first asynchronous read.
	conn.State = connection.StateHandshake
	s.postRead(conn)
}

// Stop initiates a graceful shutdown of the server.
func (s *Server) Stop() {
	if s.iocpPort != nil {
		if s.done != nil {
			close(s.done)
		}
		s.iocpPort.Shutdown(s.numWorkers)
		s.wg.Wait()
		s.iocpPort.Close()
	}

	if s.listener != windows.InvalidHandle {
		windows.Closesocket(s.listener)
	}
	if s.rioEngine != nil {
		s.rioEngine.Close()
	}
	iocp.Cleanup()
}

// sweeper runs in the background and enforces connection timeouts.
// It mitigates Slowloris attacks by forcibly closing sockets that haven't
// completed an I/O operation in the last 5 seconds.
func (s *Server) sweeper() {
	defer s.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Calculate the threshold timestamp (30 seconds for HTTP)
			threshold := time.Now().Add(-30 * time.Second).UnixNano()

			connection.ActiveTracker.Range(func(key, value interface{}) bool {
				conn := value.(*connection.Conn)
				
				// WebSockets are long-lived and handle their own Ping/Pong timeouts.
				if conn.IsWebSocket {
					return true
				}
				
				// Lock-free check: if the connection's last activity is older than the threshold
				if atomic.LoadInt64(&conn.LastActive) < threshold {
					// Forcibly close the socket. The Windows kernel will immediately generate 
					// a failed IOCP completion packet for any pending reads/writes, which our 
					// worker threads will intercept and route to conn.Release().
					windows.Closesocket(conn.Socket)
				}
				return true // Continue iterating
			})
		case <-s.done:
			return
		}
	}
}


