package pool

import (
	"sync"
)

// DefaultBufferSize is exactly 4096 bytes.
const DefaultBufferSize = 4096

// Buffer array type representing our fixed-size static memory spaces.
// We use a named type to make pointer casting explicit and avoid heap escapes.
type Buffer [DefaultBufferSize]byte

// bufferPool holds pointers to static 4096-byte arrays.
var bufferPool = sync.Pool{
	New: func() any {
		var b Buffer
		return &b
	},
}

// GetBuffer retrieves a pointer to a fixed-size byte array from the pool.
func GetBuffer() *Buffer {
	return bufferPool.Get().(*Buffer)
}

// PutBuffer returns the buffer to the pool after clearing it.
func PutBuffer(b *Buffer) {
	*b = Buffer{}
	bufferPool.Put(b)
}

// OverlappedCtxPool manages iocp.OverlappedCtx structs if they need to be allocated independently.
var OverlappedCtxPool = sync.Pool{}

// RequestCtxPool handles zero-allocation parser requests.
var RequestCtxPool = sync.Pool{}

// ConnPool handles the zero-allocation connection contexts.
var ConnPool = sync.Pool{}
