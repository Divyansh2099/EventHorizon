# internal/memory

**Responsibility:**
Manages the zero-allocation lifecycle of byte buffers utilized for network reads and writes.

**Interactions:**
- **`connection`**: `Conn` structs fetch `ReadBuffer` and `WriteBuffer` pointers from this pool upon checkout, and return them upon release.
- **`parser`**: The parser directly slices the buffers provided by this package.

**Optimization Decisions:**
- **`sync.Pool` with Pointers**: We pool `*[]byte` instead of `[]byte`. When placing slices into an `interface{}` (which `sync.Pool` requires), Go allocates memory to store the slice header on the heap. By pooling pointers to slices, we avoid this interface allocation penalty.
- **Aggressive Clearing**: We utilize Go 1.21's `clear()` built-in immediately upon returning a buffer to the pool. This prevents accidental data leakage (e.g., exposing a previous client's Authorization header to a new connection if parsing logic fails).

**Extension Points:**
- Future implementations could include slab allocators or segregated pools for different buffer sizes (e.g., 4KB for headers, 1MB for file streams).
