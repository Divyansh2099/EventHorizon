# internal/router

**Responsibility:**
This package handles high-performance, lock-free HTTP request routing.

**Design:**
The router uses a radix tree (or similar read-optimized data structure) to achieve $O(k)$ lookup times where $k$ is the length of the path. It strictly avoids `sync.Mutex` in the read path, preferring atomic swaps or immutable route table swaps for dynamic updates.

**Zero Allocation:**
Route handlers accept the zero-allocation `*parser.Request` and write directly to the `*connection.Conn` buffer.
