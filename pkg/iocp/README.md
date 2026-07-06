# internal/iocp

**Responsibility:**
The `iocp` package forms the beating heart of EventHorizon's asynchronous event loop. It interfaces directly with Windows I/O Completion Ports (IOCP) to achieve maximally efficient thread sleeping and waking based on kernel-level network events.

**Interactions:**
- **`winsock`**: Receives `windows.Handle` socket identifiers created by the `winsock` package to associate them with the completion port.
- **`server`**: Provides the worker loop `RunWorker` which is spawned by the central server orchestrator.
- **`connection`**: Defines the `OverlappedExt` structure which `connection.Conn` embeds to smuggle state across the kernel boundary.

**Windows Concepts Explained:**
- **I/O Completion Ports (IOCP)**: IOCP is the Windows NT equivalent of `epoll` or `kqueue`, but fundamentally better suited for true asynchronous I/O (Proactor pattern vs Reactor pattern). Instead of telling us "the socket is ready to read," IOCP tells us "the read you requested has finished, and the data is in your buffer."
- **Overlapped I/O**: Windows requires an `OVERLAPPED` structure to track asynchronous operations. When an operation completes, the kernel returns a pointer to this exact structure.
- **LIFO Threading**: `GetQueuedCompletionStatus` wakes up waiting threads in a Last-In-First-Out (LIFO) order. This intentionally keeps a small subset of CPU cores extremely hot, maximizing CPU cache hit rates and minimizing context switching overhead.

**Optimization Decisions:**
- **Pointer Smuggling (`unsafe.Pointer`)**: In Go, returning an interface from a pool or passing an interface boundary causes a heap allocation. By strictly defining `OverlappedExt` with `windows.Overlapped` as its first physical memory layout field, we can pass it to the C-kernel and cast it back in Go safely using pointer arithmetic, avoiding all allocations.

**Extension Points:**
- The `OpType` enum can be expanded to include file I/O operations (`TransmitFile`) for zero-copy static asset serving in the future.
