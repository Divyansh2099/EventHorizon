# internal/server

**Responsibility:**
The central orchestrator of EventHorizon. It boots the engine, associates sockets with the IOCP, manages the worker pool, and dictates the finite state machine transitions for every connection.

**Interactions:**
- **Everything**: It imports `winsock`, `iocp`, `connection`, `parser`, and `memory` to tie the system together.

**Optimization Decisions:**
- **Async Accept Pipeline**: Instead of a dedicated `for { accept() }` goroutine which becomes a bottleneck under high connection rates, the server pre-posts a large batch (e.g., 100) of `AcceptEx` requests to the kernel. As soon as a client connects, they are immediately handed a socket and transition to the `OpRead` state, while the worker replenishes the `AcceptEx` pool asynchronously.
- **Inline Handling**: The `handleIO` callback executes directly on the IOCP worker goroutine. It does not spawn a new goroutine per request. This eliminates goroutine scheduling overhead, allowing the CPU cache to remain incredibly hot as it processes I/O completion, parses the request, and posts the response within microseconds on the same OS thread.

**Extension Points:**
- Injection of the `internal/router` to map requests to user-defined handlers.
- Implementation of keep-alive logic (resetting parser state and posting another `WSARecv` instead of closing the connection).
