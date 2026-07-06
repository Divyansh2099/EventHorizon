//go:build ignore

package main

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
	"crypto/x509"
)

var (
	secur32 = syscall.NewLazyDLL("Secur32.dll")
	crypt32 = syscall.NewLazyDLL("Crypt32.dll")

	procAcquireCredentialsHandleW = secur32.NewProc("AcquireCredentialsHandleW")
	procAcceptSecurityContext     = secur32.NewProc("AcceptSecurityContext")
	procCertOpenStore             = crypt32.NewProc("CertOpenStore")
	procCertEnumCertificatesInStore = crypt32.NewProc("CertEnumCertificatesInStore")
)

type CredHandle struct {
	dwLower uintptr
	dwUpper uintptr
}

type CtxtHandle struct {
	dwLower uintptr
	dwUpper uintptr
}

type SecBuffer struct {
	cbBuffer   uint32
	BufferType uint32
	pvBuffer   uintptr
}

type SecBufferDesc struct {
	ulVersion uint32
	cBuffers  uint32
	pBuffers  *SecBuffer
}

func main() {
	// 1. Get Cert
	storeName, _ := syscall.UTF16PtrFromString("MY")
	hStore, _, _ := syscall.SyscallN(procCertOpenStore.Addr(), 10, 0, 0, 0x00010000, uintptr(unsafe.Pointer(storeName)))
	
	var GlobalCertContext uintptr
	for {
		GlobalCertContext, _, _ = syscall.SyscallN(procCertEnumCertificatesInStore.Addr(), hStore, GlobalCertContext)
		if GlobalCertContext == 0 {
			break
		}
		var pcbData uint32
		ret, _, _ := syscall.SyscallN(crypt32.NewProc("CertGetCertificateContextProperty").Addr(), GlobalCertContext, 2, 0, uintptr(unsafe.Pointer(&pcbData)))
		if ret != 0 {
			certCtx := (*struct{
				DwCertEncodingType uint32
				PbCertEncoded      *byte
				CbCertEncoded      uint32
				PCertInfo          uintptr
				HCertStore         uintptr
			})(unsafe.Pointer(GlobalCertContext))
			certBytes := unsafe.Slice(certCtx.PbCertEncoded, certCtx.CbCertEncoded)
			if parsedCert, err := x509.ParseCertificate(certBytes); err == nil {
				if parsedCert.Subject.CommonName == "127.0.0.1" {
					fmt.Printf("Found cert: %s\n", parsedCert.Subject.String())
					break
				}
			}
		}
	}
	if GlobalCertContext == 0 {
		fmt.Println("Cert not found")
		return
	}

	// 2. Acquire Creds
	creds := make([]byte, 80)
	*(*uint32)(unsafe.Pointer(&creds[0])) = 0x00000004 // dwVersion
	*(*uint32)(unsafe.Pointer(&creds[4])) = 1 // cCreds
	*(*uintptr)(unsafe.Pointer(&creds[8])) = uintptr(unsafe.Pointer(&GlobalCertContext)) // paCred
	*(*uint32)(unsafe.Pointer(&creds[72])) = 0 // dwFlags

	var ServerCredHandle CredHandle
	var tsExpiry int64
	unifiedProvider, _ := syscall.UTF16PtrFromString("Microsoft Unified Security Protocol Provider")
	
	ret, _, _ := syscall.SyscallN(
		procAcquireCredentialsHandleW.Addr(),
		0,
		uintptr(unsafe.Pointer(unifiedProvider)),
		2, // SECPKG_CRED_INBOUND
		0,
		uintptr(unsafe.Pointer(&creds[0])),
		0,
		0,
		uintptr(unsafe.Pointer(&ServerCredHandle)),
		uintptr(unsafe.Pointer(&tsExpiry)),
	)
	if ret != 0 {
		fmt.Printf("Acquire failed: %x\n", ret)
		return
	}
	fmt.Printf("Acquired creds: %v\n", ServerCredHandle)

	// 3. Listen
	l, err := net.Listen("tcp", "127.0.0.1:8084")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Listening on 8084")
	conn, err := l.Accept()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Client connected")

	// 4. AcceptSecurityContext
	inBuf := make([]byte, 4096)
	n, _ := conn.Read(inBuf)
	fmt.Printf("Read %d bytes\n", n)

	inBuffers := []SecBuffer{
		{
			cbBuffer:   uint32(n),
			BufferType: 2, // SECBUFFER_TOKEN
			pvBuffer:   uintptr(unsafe.Pointer(&inBuf[0])),
		},
		{
			cbBuffer:   0,
			BufferType: 0, // SECBUFFER_EMPTY
			pvBuffer:   0,
		},
	}
	inDesc := SecBufferDesc{ulVersion: 0, cBuffers: 2, pBuffers: &inBuffers[0]}

	var outTokenBuf [4096]byte
	outBuffers := []SecBuffer{
		{
			cbBuffer:   0,
			BufferType: 2, // SECBUFFER_TOKEN
			pvBuffer:   0,
		},
		{
			cbBuffer:   0,
			BufferType: 17, // SECBUFFER_ALERT
			pvBuffer:   0,
		},
		{
			cbBuffer:   0,
			BufferType: 0, // SECBUFFER_EMPTY
			pvBuffer:   0,
		},
	}
	outDesc := SecBufferDesc{ulVersion: 0, cBuffers: 3, pBuffers: &outBuffers[0]}

	var contextHandle CtxtHandle
	var outFlags uint32
	var flags uint32 = 0x00000010 | 0x00000008 | 0x00000004 | 0x00010000 | 0x00000100 | 0x00008000

	ret, _, _ = syscall.SyscallN(
		procAcceptSecurityContext.Addr(),
		uintptr(unsafe.Pointer(&ServerCredHandle)),
		0,
		uintptr(unsafe.Pointer(&inDesc)),
		uintptr(flags),
		16, // SECURITY_NATIVE_DREP
		uintptr(unsafe.Pointer(&contextHandle)),
		uintptr(unsafe.Pointer(&outDesc)),
		uintptr(unsafe.Pointer(&outFlags)),
		uintptr(unsafe.Pointer(&tsExpiry)),
	)
	
	fmt.Printf("AcceptSecurityContext returned %x\n", ret)
	if ret == 0x00090312 { // SEC_I_CONTINUE_NEEDED
		fmt.Printf("Generated %d bytes to send\n", outBuffers[0].cbBuffer)
		conn.Write(outTokenBuf[:outBuffers[0].cbBuffer])
	}
	
	conn.Close()
	l.Close()
}
