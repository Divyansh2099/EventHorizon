package rio

import (
	"golang.org/x/sys/windows"
)

var (
	// WSAID_MULTIPLE_RIO is the GUID required to retrieve the RIO function pointers.
	WSAID_MULTIPLE_RIO = windows.GUID{
		Data1: 0x8509e081,
		Data2: 0x96dd,
		Data3: 0x4005,
		Data4: [8]byte{0xb1, 0x65, 0x9e, 0x2e, 0xe8, 0xc7, 0x9e, 0x3f},
	}
)

const (
	SIO_GET_MULTIPLE_EXTENSION_FUNCTION_POINTER = 0xc8000024
	RIO_IOCP_COMPLETION                         = 2
)

type RIO_BUFFERID uintptr
type RIO_CQ uintptr
type RIO_RQ uintptr

// RIO_EXTENSION_FUNCTION_TABLE holds the native function pointers for Windows RIO.
type RIO_EXTENSION_FUNCTION_TABLE struct {
	CbSize                   uint32
	RIOReceive               uintptr
	RIOReceiveEx             uintptr
	RIOSend                  uintptr
	RIOSendEx                uintptr
	RIOCloseCompletionQueue  uintptr
	RIOCreateCompletionQueue uintptr
	RIOCreateRequestQueue    uintptr
	RIODequeueCompletion     uintptr
	RIODeregisterBuffer      uintptr
	RIONotify                uintptr
	RIORegisterBuffer        uintptr
	RIOResizeCompletionQueue uintptr
	RIOResizeRequestQueue    uintptr
}

// RIO_BUF represents a chunk of registered memory used for send/recv operations.
type RIO_BUF struct {
	BufferId RIO_BUFFERID
	Offset   uint32
	Length   uint32
}

// RIO_NOTIFICATION_COMPLETION specifies the method used to notify the application
// when a RIO completion queue is not empty.
type RIO_NOTIFICATION_COMPLETION struct {
	Type          uint32
	_             uint32 // Padding to align the union on 8-byte boundary
	IocpHandle    windows.Handle
	CompletionKey uintptr
	Overlapped    uintptr // PVOID
}

// RIORESULT contains the results of a single completed RIO send or receive operation.
type RIORESULT struct {
	Status           int32
	BytesTransferred uint32
	SocketContext    uint64 // ULONGLONG
	RequestContext   uint64 // ULONGLONG
}
