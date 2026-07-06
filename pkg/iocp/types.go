package iocp

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// OpType identifies the type of overlapped operation that was completed.
// This is necessary because a single socket might have multiple outstanding
// operations (e.g., an ongoing read and a pending write).
type OpType int

const (
	OpAccept OpType = iota
	OpRead
	OpWrite
	OpWSRead
	OpWSWrite
	OpTransmitFile
	OpResume
	OpFileRead
)

// OverlappedCtx extends the standard windows.Overlapped structure.
// This allows us to pass additional context through the Windows kernel
// and retrieve it upon I/O completion.
//
// By using pointer arithmetic/casting, when the OS returns a pointer to the
// embedded windows.Overlapped, we can safely cast it back to *OverlappedCtx
// to retrieve our custom context (Op and Conn) without heap allocations.
type OverlappedCtx struct {
	windows.Overlapped // MUST be the first field so we can cast back and forth
	Op   OpType
	Conn unsafe.Pointer // Pointer to the connection object
}

// AcceptCtx is a lightweight structure specifically designed for AcceptEx pre-posting.
// By decoupling AcceptEx from the massive connection structures, we save tens of megabytes
// of RAM per thousand pending sockets.
type AcceptCtx struct {
	Ctx          OverlappedCtx
	AcceptSocket windows.Handle
	Buffer       [88]byte // 88 bytes required for AcceptEx address resolution
}

// AcceptBatch represents a massive pre-allocated pool of empty buckets
// handed to the Windows kernel to catch incoming TCP connections simultaneously.
type AcceptBatch struct {
	Listener windows.Handle
	Contexts []AcceptCtx
}
