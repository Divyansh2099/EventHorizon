# internal/connection

**Responsibility:**
Manages the lifecycle, state, and resources of an active client socket. 

**Interactions:**
- **`memory`**: Borrows buffers for network traffic.
- **`iocp`**: Inherits `OverlappedExt` structures to pass to the kernel.
- **`server`**: The server orchestrator uses this state machine to dictate IOCP transitions.

**Windows Concepts Explained:**
- **Pinned Memory**: When an overlapped operation like `WSARecv` is posted to the kernel, the buffer and the `Overlapped` structure MUST NOT move in memory until the operation completes. Because the Go garbage collector moves memory during compaction, it is critical that these structures are managed carefully, often kept alive dynamically or allocated outside normal GC visibility if necessary (though Go's GC pinning logic handles standard `sys/windows` calls safely for the duration of the syscall).

**Optimization Decisions:**
- **Embedded Structs**: Instead of allocating a new `windows.Overlapped` struct every time we call `WSARecv`, the `Conn` struct contains `AcceptOverlapped`, `ReadOverlapped`, and `WriteOverlapped` directly within its contiguous memory footprint. When a `Conn` is recycled via `sync.Pool`, these structures are reused indefinitely.

**Extension Points:**
- State machine can be expanded to support `StateUpgraded` for WebSockets.
