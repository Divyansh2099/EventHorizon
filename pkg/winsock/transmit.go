package winsock

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	mswsock = windows.NewLazySystemDLL("mswsock.dll")
	procTransmitFile = mswsock.NewProc("TransmitFile")
)

// TransmitFileBuffers matches the Win32 TRANSMIT_FILE_BUFFERS struct.
type TransmitFileBuffers struct {
	Head       uintptr
	HeadLength uint32
	Tail       uintptr
	TailLength uint32
}

// TransmitFile wraps the native mswsock TransmitFile call to send a file over a socket.
func TransmitFile(
	hSocket windows.Handle,
	hFile windows.Handle,
	nNumberOfBytesToWrite uint32,
	nNumberOfBytesPerSend uint32,
	lpOverlapped *windows.Overlapped,
	lpTransmitBuffers uintptr, // typically nil unless headers/trailers are needed
	dwFlags uint32,
) error {
	ret, _, err := syscall.SyscallN(
		procTransmitFile.Addr(),
		uintptr(hSocket),
		uintptr(hFile),
		uintptr(nNumberOfBytesToWrite),
		uintptr(nNumberOfBytesPerSend),
		uintptr(unsafe.Pointer(lpOverlapped)),
		lpTransmitBuffers,
		uintptr(dwFlags),
	)

	// TransmitFile returns TRUE (1) on success, FALSE (0) on error.
	// In the context of IOCP, returning FALSE with ERROR_IO_PENDING is a success state.
	if ret == 0 {
		if err != 0 && err != syscall.ERROR_IO_PENDING {
			return err
		}
	}
	return nil
}
