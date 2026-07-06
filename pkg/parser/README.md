# internal/parser

**Responsibility:**
Implements a strict, zero-allocation state machine for parsing raw HTTP requests directly from the socket buffer.

**Interactions:**
- **`connection`**: Receives the `Conn.ReadBuffer` slice to parse.
- **`server`**: The handler triggers the parser when a `WSARecv` completes.

**Optimization Decisions:**
- **Zero-Copy Slicing**: Go strings are immutable, meaning converting a byte slice to a string forces a heap allocation and a memory copy. This parser strictly avoids string conversions. The resulting `Request` struct contains `[]byte` fields (like `Method`, `Path`) that are simply slice headers pointing into the original `ReadBuffer`.
- **Pre-allocated Header Arrays**: The `Request` struct avoids dynamic slice growth by pre-allocating an array of 128 `Header` structs. During parsing, we simply increment an index and overwrite the `Header` struct with the new key/value slice headers.
- **Pool Driven**: `Request` objects are recycled via `sync.Pool` just like network buffers.

**Extension Points:**
- Support for chunked transfer encoding (currently out of scope for Phase 2).
- HTTP/2 frame parsing (requires an entirely different state machine, but fits the zero-copy philosophy).
