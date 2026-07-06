//go:build ignore

package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
	"github.com/eventhorizon/pkg/rio"
)

func main() {
	var data windows.WSAData
	err := windows.WSAStartup(uint32(2|2<<8), &data)
	if err != nil {
		panic(err)
	}
	defer windows.WSACleanup()

	socket, err := windows.WSASocket(
		syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP,
		nil, 0, windows.WSA_FLAG_OVERLAPPED,
	)
	if err != nil {
		panic(err)
	}
	defer windows.Closesocket(socket)

	var table rio.RIO_EXTENSION_FUNCTION_TABLE
	var bytesReturned uint32
	err = windows.WSAIoctl(
		socket,
		rio.SIO_GET_MULTIPLE_EXTENSION_FUNCTION_POINTER,
		(*byte)(unsafe.Pointer(&rio.WSAID_MULTIPLE_RIO)),
		uint32(unsafe.Sizeof(rio.WSAID_MULTIPLE_RIO)),
		(*byte)(unsafe.Pointer(&table)),
		uint32(unsafe.Sizeof(table)),
		&bytesReturned, nil, 0,
	)
	if err != nil {
		panic(err)
	}

	handle, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 0)
	if err != nil {
		panic(err)
	}
	defer windows.CloseHandle(handle)

	var notifyOvl windows.Overlapped
	notifyContext := rio.RIO_NOTIFICATION_COMPLETION{
		Type:          rio.RIO_IOCP_COMPLETION,
		IocpHandle:    handle,
		CompletionKey: 0xDEADBEEF,
		Overlapped:    uintptr(unsafe.Pointer(&notifyOvl)),
	}

	ret, _, errSys := syscall.SyscallN(
		table.RIOCreateCompletionQueue,
		1024,
		uintptr(unsafe.Pointer(&notifyContext)),
	)
	
	if ret == 0 {
		fmt.Printf("FAILED: %v\n", errSys)
	} else {
		fmt.Printf("SUCCESS CQ: %d\n", ret)
	}
}
