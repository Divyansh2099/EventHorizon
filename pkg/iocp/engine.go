package iocp

import (
	"fmt"
	"syscall"
	"unsafe"
	
	"github.com/eventhorizon/pkg/rio"
	"golang.org/x/sys/windows"
)

// Port represents an I/O Completion Port.
type Port struct {
	handle      windows.Handle
	globalRioCQ rio.RIO_CQ
	rioTable    *rio.RIO_EXTENSION_FUNCTION_TABLE
	rioNotifyOvl windows.Overlapped // Must remain valid for the lifetime of the CQ

	// Phase 25: AcceptEx Decoupling
	acceptExPtr   uintptr
	acceptBatch   *AcceptBatch
	AcceptHandler func(socket windows.Handle)
}

// CreatePort creates a new I/O Completion Port.
func CreatePort() (*Port, error) {
	// CreateIoCompletionPort with InvalidHandle creates a new, unassociated port.
	// The final parameter 0 means we allow as many concurrently executing threads
	// as there are processors in the system.
	handle, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateIoCompletionPort failed: %w", err)
	}
	return &Port{handle: handle}, nil
}

// Associate binds a socket handle to this completion port.
// Any I/O operations (like WSARecv, WSASend, AcceptEx) initiated on this socket
// will queue a completion packet to this port when they finish.
// The completionKey is returned in GetQueuedCompletionStatus, allowing us to
// identify which listener or context the completion belongs to.
func (p *Port) Associate(socket windows.Handle, completionKey uintptr) error {
	// We pass the existing port handle to associate the socket with it.
	_, err := windows.CreateIoCompletionPort(socket, p.handle, completionKey, 0)
	if err != nil {
		return fmt.Errorf("Associate failed: %w", err)
	}
	return nil
}

// Close closes the completion port handle, releasing system resources.
func (p *Port) Close() error {
	if p.globalRioCQ != 0 && p.rioTable != nil {
		syscall.SyscallN(p.rioTable.RIOCloseCompletionQueue, uintptr(p.globalRioCQ))
	}
	return windows.CloseHandle(p.handle)
}

// InitRIO initializes the global RIO Completion Queue.
// It creates the CQ, binds it to the IOCP, and immediately arms it.
func (p *Port) InitRIO(table *rio.RIO_EXTENSION_FUNCTION_TABLE, cqSize uint32) error {
	p.rioTable = table

	// Bind the CQ to our existing IOCP handle so we receive wakeups.
	// The Overlapped pointer must point to a valid struct for the lifetime of the CQ.
	notifyContext := rio.RIO_NOTIFICATION_COMPLETION{
		Type:          rio.RIO_IOCP_COMPLETION,
		IocpHandle:    p.handle,
		CompletionKey: RioCompletionKey,
		Overlapped:    uintptr(unsafe.Pointer(&p.rioNotifyOvl)),
	}

	// RIOCreateCompletionQueue(QueueSize, NotificationCompletion)
	ret, _, errSys := syscall.SyscallN(
		table.RIOCreateCompletionQueue,
		uintptr(cqSize),
		uintptr(unsafe.Pointer(&notifyContext)),
	)
	
	if ret == 0 { // RIO_INVALID_CQ is 0
		return fmt.Errorf("RIOCreateCompletionQueue failed: %v", errSys)
	}
	p.globalRioCQ = rio.RIO_CQ(ret)

	// Immediately arm the queue. This is strictly required to receive the first completion.
	// RIONotify(CQ)
	retNotify, _, errNotify := syscall.SyscallN(table.RIONotify, uintptr(p.globalRioCQ))
	if retNotify != 0 {
		// RIONotify returns NO_ERROR (0) on success.
		// If it returns SOCKET_ERROR (-1), we check the error.
		return fmt.Errorf("RIONotify failed during init (ret=%d): %v", retNotify, errNotify)
	}

	return nil
}

// CreateRequestQueue creates a RIO Request Queue for a specific socket and binds it to the global CQ.
func (p *Port) CreateRequestQueue(socket windows.Handle) (rio.RIO_RQ, error) {
	// RIOCreateRequestQueue(Socket, MaxOutstandingReceive, MaxReceiveDataBuffers, MaxOutstandingSend, MaxSendDataBuffers, ReceiveCQ, SendCQ, SocketContext)
	ret, _, errSys := syscall.SyscallN(
		p.rioTable.RIOCreateRequestQueue,
		uintptr(socket),
		1, // Max Receive per socket
		1,   // Max Receive Buffers
		1, // Max Send per socket
		1,   // Max Send Buffers
		uintptr(p.globalRioCQ), // Receive CQ
		uintptr(p.globalRioCQ), // Send CQ
		0,   // SocketContext
	)

	if ret == 0 { // RIO_INVALID_RQ is 0
		return 0, fmt.Errorf("RIOCreateRequestQueue failed: %v", errSys)
	}

	return rio.RIO_RQ(ret), nil
}

// Post queues a custom completion packet to the I/O Completion Port.
func (p *Port) Post(bytesTransferred uint32, completionKey uintptr, overlapped *windows.Overlapped) error {
	return windows.PostQueuedCompletionStatus(p.handle, bytesTransferred, completionKey, overlapped)
}

// ShutdownKey is a special completion key used to signal workers to exit.
// We use the maximum uintptr value to avoid collision with standard pointers or IDs.
const ShutdownKey = ^uintptr(0)

// RioCompletionKey is the special key that indicates a batch of RIO network operations has completed.
const RioCompletionKey = uintptr(0xDEADBEEF)

// IOHandler is the signature for the callback function that processes
// completed I/O operations.
type IOHandler func(key uintptr, overlapped *OverlappedCtx, bytesTransferred uint32, err error)

// RunWorker starts a continuous loop that waits for I/O completions on the port.
// This function should be run in its own goroutine. In a high-performance server,
// you typically spawn one worker goroutine per logical CPU core.
func (p *Port) RunWorker(handler IOHandler) {
	for {
		var bytesTransferred uint32
		var completionKey uintptr
		var overlapped *windows.Overlapped

		// GetQueuedCompletionStatus blocks the calling goroutine until an I/O
		// operation associated with this port completes, or a packet is posted manually.
		err := windows.GetQueuedCompletionStatus(
			p.handle,
			&bytesTransferred,
			&completionKey,
			&overlapped,
			windows.INFINITE, // Block indefinitely
		)

		// Check if we received a shutdown signal.
		if completionKey == ShutdownKey {
			return // Exit the goroutine gracefully
		}

		if overlapped == &p.rioNotifyOvl {
			// A RIO operation completed!
			// We MUST batch-dequeue the completions from the global CQ.
			// Windows guarantees we will not miss completions if we drain it fully.
			var results [64]rio.RIORESULT
			for {
				ret, _, _ := syscall.SyscallN(
					p.rioTable.RIODequeueCompletion,
					uintptr(p.globalRioCQ),
					uintptr(unsafe.Pointer(&results[0])),
					64,
				)
				
				if ret == 0 || ret == ^uintptr(0) {
					// Queue is empty or error occurred
					break
				}
				
				numCompletions := int32(ret)
				for i := int32(0); i < numCompletions; i++ {
					ext := (*OverlappedCtx)(unsafe.Pointer(uintptr(results[i].RequestContext)))
					if results[i].Status != 0 {
						results[i].BytesTransferred = 0
					}
					
					// Distribute the RIO completion to the IOCP thread pool!
					p.Post(results[i].BytesTransferred, RioCompletionKey, &ext.Overlapped)
				}
			}

			// RIO Design Requirement: Re-arm the completion queue after processing events
			// so the kernel will trigger another IOCP notification on the next packet.
			syscall.SyscallN(p.rioTable.RIONotify, uintptr(p.globalRioCQ))
			
			continue
		}

		// A nil overlapped pointer indicates the function failed before dequeuing
		// a completion packet, or it was a special post with no overlapped structure.
		if overlapped == nil {
			handler(completionKey, nil, 0, err)
			continue
		}

		// Because we always pass a pointer to OverlappedCtx to our WinSock calls,
		// and OverlappedCtx embeds windows.Overlapped as its first field,
		// it is safe to cast the returned pointer back to our extended struct.
		// This achieves zero-allocation context retrieval.
		ext := (*OverlappedCtx)(unsafe.Pointer(overlapped))

		if ext.Op == OpAccept {
			// Phase 25: Batched AcceptEx decoupling.
			// Cast back to AcceptCtx to recycle it and grab the socket.
			acceptCtx := (*AcceptCtx)(unsafe.Pointer(overlapped))
			clientSocket := acceptCtx.AcceptSocket

			// Pass the newly connected socket up to the server
			if p.AcceptHandler != nil {
				p.AcceptHandler(clientSocket)
			}

			// Generate a new empty bucket for this context
			newSock, err := CreateIOCPSocket()
			if err == nil {
				acceptCtx.AcceptSocket = newSock
				var bytesReceived uint32
				// Re-post AcceptEx to the kernel immediately!
				AcceptEx(
					p.acceptExPtr,
					p.acceptBatch.Listener,
					newSock,
					&acceptCtx.Buffer[0],
					0, 32, 32,
					&bytesReceived,
					&acceptCtx.Ctx.Overlapped,
				)
			}
			continue
		}

		handler(completionKey, ext, bytesTransferred, err)
	}
}

// Shutdown signals all workers listening on this port to exit.
// It posts a special completion packet for each worker to wake up and terminate.
func (p *Port) Shutdown(workerCount int) error {
	for i := 0; i < workerCount; i++ {
		// PostQueuedCompletionStatus allows us to manually inject a completion
		// packet into the IOCP queue. We use this to broadcast a shutdown signal.
		err := windows.PostQueuedCompletionStatus(p.handle, 0, ShutdownKey, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// Init initializes the Windows Sockets API (WinSock2).
// This function calls WSAStartup, which instructs the Windows OS to load
// the necessary DLLs and allocate resources for network operations in this process.
// It MUST be called before any other networking functions are used.
func Init() error {
	var data windows.WSAData
	// Request WinSock version 2.2 (the standard for modern Windows networking)
	// The WORD parameter is formed as (HighByte | LowByte << 8)
	// version 2.2 -> 2 | (2 << 8)
	err := windows.WSAStartup(uint32(2|2<<8), &data)
	if err != nil {
		return err
	}
	return nil
}

// Cleanup terminates the use of the WinSock2 library.
// It calls WSACleanup, freeing resources allocated by WSAStartup.
func Cleanup() error {
	return windows.WSACleanup()
}

// CreateIOCPSocket creates a new socket configured specifically for
// Overlapped I/O, which is required for Windows I/O Completion Ports (IOCP).
func CreateIOCPSocket() (windows.Handle, error) {
	// WSA_FLAG_OVERLAPPED is critical for async operations.
	// 0x100 is WSA_FLAG_REGISTERED_IO which is required for RIO APIs!
	socket, err := windows.WSASocket(
		syscall.AF_INET,             // IPv4
		syscall.SOCK_STREAM,         // TCP
		syscall.IPPROTO_TCP,         // TCP protocol
		nil,                         // No protocol info
		0,                           // Group ID (0 = none)
		windows.WSA_FLAG_OVERLAPPED | 0x100,
	)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("WSASocket failed: %w", err)
	}

	// Disable Nagle's algorithm. This is crucial for high-performance HTTP servers
	// as it prevents the OS from artificially delaying small packet sends to bundle them.
	// We want minimum latency.
	err = windows.SetsockoptInt(socket, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)
	if err != nil {
		windows.Closesocket(socket)
		return windows.InvalidHandle, fmt.Errorf("Setsockopt TCP_NODELAY failed: %w", err)
	}

	// Enable SO_REUSEADDR to allow the socket to be bound to an address
	// that is already in use. This prevents "address already in use" errors
	// during rapid restarts of the server.
	err = windows.SetsockoptInt(socket, windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
	if err != nil {
		windows.Closesocket(socket)
		return windows.InvalidHandle, fmt.Errorf("Setsockopt SO_REUSEADDR failed: %w", err)
	}

	return socket, nil
}

// BindAndListen binds the provided socket handle to the local network interfaces
// on the specified port, and places it into a listening state.
func BindAndListen(socket windows.Handle, port int) error {
	// Address 0.0.0.0 (INADDR_ANY) is implicit when Addr is zero-valued.
	addr := &windows.SockaddrInet4{Port: port}

	err := windows.Bind(socket, addr)
	if err != nil {
		return fmt.Errorf("Bind failed: %w", err)
	}

	// SOMAXCONN instructs the WinSock provider to set the backlog of pending
	// connections to a "reasonable" maximum value.
	err = windows.Listen(socket, windows.SOMAXCONN)
	if err != nil {
		return fmt.Errorf("Listen failed: %w", err)
	}

	return nil
}

var (
	// WSAID_ACCEPTEX is the Microsoft-defined GUID for the AcceptEx extension function.
	WSAID_ACCEPTEX = windows.GUID{
		Data1: 0xb5367df1,
		Data2: 0xcbac,
		Data3: 0x11cf,
		Data4: [8]byte{0x95, 0xca, 0x00, 0x80, 0x5f, 0x48, 0xa1, 0x92},
	}

	// WSAID_GETACCEPTEXSOCKADDRS is the GUID for GetAcceptExSockaddrs,
	// which is required to parse the addresses returned by AcceptEx.
	WSAID_GETACCEPTEXSOCKADDRS = windows.GUID{
		Data1: 0xb5367df2,
		Data2: 0xcbac,
		Data3: 0x11cf,
		Data4: [8]byte{0x95, 0xca, 0x00, 0x80, 0x5f, 0x48, 0xa1, 0x92},
	}
)

// LoadAcceptEx retrieves the AcceptEx function pointer for a specific socket.
// Microsoft recommends loading extension functions at runtime via WSAIoctl
// rather than linking against mswsock.lib directly for performance reasons.
func LoadAcceptEx(listener windows.Handle) (uintptr, error) {
	var acceptExPtr uintptr
	var bytesReturned uint32

	err := windows.WSAIoctl(
		listener,
		windows.SIO_GET_EXTENSION_FUNCTION_POINTER,
		(*byte)(unsafe.Pointer(&WSAID_ACCEPTEX)),
		uint32(unsafe.Sizeof(WSAID_ACCEPTEX)),
		(*byte)(unsafe.Pointer(&acceptExPtr)),
		uint32(unsafe.Sizeof(acceptExPtr)),
		&bytesReturned,
		nil,
		0,
	)

	if err != nil {
		return 0, fmt.Errorf("WSAIoctl SIO_GET_EXTENSION_FUNCTION_POINTER failed for AcceptEx: %w", err)
	}

	return acceptExPtr, nil
}

// LoadGetAcceptExSockaddrs retrieves the GetAcceptExSockaddrs function pointer.
func LoadGetAcceptExSockaddrs(listener windows.Handle) (uintptr, error) {
	var ptr uintptr
	var bytesReturned uint32

	err := windows.WSAIoctl(
		listener,
		windows.SIO_GET_EXTENSION_FUNCTION_POINTER,
		(*byte)(unsafe.Pointer(&WSAID_GETACCEPTEXSOCKADDRS)),
		uint32(unsafe.Sizeof(WSAID_GETACCEPTEXSOCKADDRS)),
		(*byte)(unsafe.Pointer(&ptr)),
		uint32(unsafe.Sizeof(ptr)),
		&bytesReturned,
		nil,
		0,
	)

	if err != nil {
		return 0, fmt.Errorf("WSAIoctl failed for GetAcceptExSockaddrs: %w", err)
	}

	return ptr, nil
}

// AcceptEx invokes the native AcceptEx function via the dynamically loaded pointer.
func AcceptEx(
	acceptExPtr uintptr,
	listenSocket windows.Handle,
	acceptSocket windows.Handle,
	receiveBuffer *byte,
	receiveDataLength uint32,
	localAddressLength uint32,
	remoteAddressLength uint32,
	bytesReceived *uint32,
	overlapped *windows.Overlapped,
) error {
	ret, _, err := syscall.SyscallN(
		acceptExPtr,
		uintptr(listenSocket),
		uintptr(acceptSocket),
		uintptr(unsafe.Pointer(receiveBuffer)),
		uintptr(receiveDataLength),
		uintptr(localAddressLength),
		uintptr(remoteAddressLength),
		uintptr(unsafe.Pointer(bytesReceived)),
		uintptr(unsafe.Pointer(overlapped)),
	)

	// AcceptEx returns TRUE (1) on immediate success, FALSE (0) on error or pending.
	// ERROR_IO_PENDING is the expected "success" state for asynchronous overlapped I/O.
	if ret == 0 {
		if err != 0 && err != syscall.ERROR_IO_PENDING {
			return err
		}
	}
	return nil
}
// PrePostAcceptBatch allocates a massive static pool of AcceptCtx buckets and hands them to the kernel.
func (p *Port) PrePostAcceptBatch(listener windows.Handle, batchSize int) error {
	acceptExPtr, err := LoadAcceptEx(listener)
	if err != nil {
		return err
	}
	p.acceptExPtr = acceptExPtr
	p.acceptBatch = &AcceptBatch{
		Listener: listener,
		Contexts: make([]AcceptCtx, batchSize),
	}

	for i := 0; i < batchSize; i++ {
		ctx := &p.acceptBatch.Contexts[i]
		ctx.Ctx.Op = OpAccept

		sock, err := CreateIOCPSocket()
		if err != nil {
			return err
		}
		ctx.AcceptSocket = sock

		var bytesReceived uint32
		err = AcceptEx(
			p.acceptExPtr,
			listener,
			ctx.AcceptSocket,
			&ctx.Buffer[0],
			0,
			32,
			32,
			&bytesReceived,
			&ctx.Ctx.Overlapped,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
