package tls

import (
	"crypto/x509"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	secur32 = windows.NewLazySystemDLL("Secur32.dll")
	crypt32 = windows.NewLazySystemDLL("Crypt32.dll")

	procAcquireCredentialsHandleW   = secur32.NewProc("AcquireCredentialsHandleW")
	procAcceptSecurityContext       = secur32.NewProc("AcceptSecurityContext")
	procFreeCredentialsHandle       = secur32.NewProc("FreeCredentialsHandle")
	procDeleteSecurityContext       = secur32.NewProc("DeleteSecurityContext")
	procEncryptMessage              = secur32.NewProc("EncryptMessage")
	procDecryptMessage              = secur32.NewProc("DecryptMessage")
	procQueryContextAttributesW     = secur32.NewProc("QueryContextAttributesW")

	procPFXImportCertStore          = crypt32.NewProc("PFXImportCertStore")
	procCertEnumCertificatesInStore = crypt32.NewProc("CertEnumCertificatesInStore")
	procCertFreeCertificateContext  = crypt32.NewProc("CertFreeCertificateContext")
	procCertCloseStore              = crypt32.NewProc("CertCloseStore")
)

const (
	SECPKG_CRED_INBOUND = 1
	SECPKG_CRED_OUTBOUND = 2

	SECBUFFER_EMPTY                 = 0
	SECBUFFER_DATA                  = 1
	SECBUFFER_TOKEN                 = 2
	SECBUFFER_EXTRA                 = 5
	SECBUFFER_STREAM_TRAILER        = 6
	SECBUFFER_STREAM_HEADER         = 7
	SECBUFFER_ALERT                 = 17
	SECBUFFER_APPLICATION_PROTOCOLS = 18

	ASC_REQ_SEQUENCE_DETECT = 0x00000008
	ASC_REQ_REPLAY_DETECT   = 0x00000004
	ASC_REQ_CONFIDENTIALITY = 0x00000010
	ASC_REQ_EXTENDED_ERROR  = 0x00008000
	ASC_REQ_ALLOCATE_MEMORY = 0x00000100
	ASC_REQ_STREAM          = 0x00010000

	SEC_E_OK                 = uint32(0x00000000)
	SEC_I_CONTINUE_NEEDED    = uint32(0x00090312)
	SEC_E_INCOMPLETE_MESSAGE = uint32(0x80090318)
	SEC_I_RENEGOTIATE        = uint32(0x00090321)

	SEC_APPLICATION_PROTOCOL_NEGOTIATION_EXT = 2

	SCH_CRED_V3           = 3
	SCH_USE_STRONG_CRYPTO = 0x00400000

	SECPKG_ATTR_STREAM_SIZES         = 4
	SECPKG_ATTR_APPLICATION_PROTOCOL = 35
	UNISP_NAME                       = "Schannel"
)

type SecHandle struct {
	dwLower uintptr
	dwUpper uintptr
}

func (s *SecHandle) IsValid() bool {
	return s.dwLower != 0 || s.dwUpper != 0
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

type SecPkgContext_StreamSizes struct {
	CbHeader         uint32
	CbTrailer        uint32
	CbMaximumMessage uint32
	CCBuffers        uint32
	CbBlockSize      uint32
}

type SecPkgContext_ApplicationProtocol struct {
	ProtoNegoStatus int32
	ProtoNegoExt    int32
	ProtocolIdSize  uint8
	ProtocolId      [255]byte
}

type SCH_CREDENTIALS struct {
	DwVersion               uint32
	DwCredFormat            uint32
	CCreds                  uint32
	PaCred                  *uintptr
	HCertStore              uintptr
	CMappers                uint32
	AphMappers              uintptr
	CSupportedAlgs          uint32
	PalgSupportedAlgs       uintptr
	GrbitEnabledProtocols   uint32
	DwMinimumCipherStrength uint32
	DwMaximumCipherStrength uint32
	DwSessionLifespan       uint32
	DwFlags                 uint32
	CTlsExts                uint32
	PTlsExts                uintptr
}

type SecApplicationProtocolList struct {
	ProtoNegoExt     uint32
	ProtocolListSize uint16
	ProtocolList     [9]byte // "\x08http/1.1"
}

type CRYPT_DATA_BLOB struct {
	cbData uint32
	pbData *byte
}

var ServerCredHandle SecHandle
var unifiedProvider *uint16

func init() {
	unifiedProvider, _ = syscall.UTF16PtrFromString("Schannel")
}

var GlobalCertContext uintptr

func InitSchannel(pfxPath, password string) error {
	storeName, _ := syscall.UTF16PtrFromString("MY")
	hStore, _, errSys := syscall.SyscallN(crypt32.NewProc("CertOpenStore").Addr(), 10 /* CERT_STORE_PROV_SYSTEM_W */, 0, 0, 0x00010000 /* CERT_SYSTEM_STORE_CURRENT_USER */, uintptr(unsafe.Pointer(storeName)))
	if hStore == 0 {
		return fmt.Errorf("CertOpenStore failed: %v", errSys)
	}

	for {
		GlobalCertContext, _, _ = syscall.SyscallN(procCertEnumCertificatesInStore.Addr(), hStore, GlobalCertContext)
		if GlobalCertContext == 0 {
			break
		}
		
		var pcbData uint32
		retGetProp, _, _ := syscall.SyscallN(crypt32.NewProc("CertGetCertificateContextProperty").Addr(), GlobalCertContext, 2 /*CERT_KEY_PROV_INFO_PROP_ID*/, 0, uintptr(unsafe.Pointer(&pcbData)))
		if retGetProp != 0 {
			// Read the pbCertEncoded to parse with x509
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
					fmt.Printf("Found certificate with private key! Subject: %s\n", parsedCert.Subject.String())
					break
				}
			}
		}
	}

	if GlobalCertContext == 0 {
		return fmt.Errorf("Could not find a valid certificate with a private key")
	}

	creds := struct {
		DwVersion               uint32
		CCreds                  uint32
		PaCred                  *uintptr
		HCertStore              uintptr
		CMappers                uint32
		AphMappers              uintptr
		CSupportedAlgs          uint32
		PalgSupportedAlgs       uintptr
		GrbitEnabledProtocols   uint32
		DwMinimumCipherStrength uint32
		DwMaximumCipherStrength uint32
		DwSessionLifespan       uint32
		DwFlags                 uint32
		DwCredFormat            uint32
	}{
		DwVersion:             0x00000004, // SCHANNEL_CRED_VERSION
		CCreds:                1,
		PaCred:                &GlobalCertContext,
		GrbitEnabledProtocols: 0,
		DwFlags:               0,
	}

	var tsExpiry int64
	ret, _, _ := syscall.SyscallN(
		procAcquireCredentialsHandleW.Addr(),
		0,
		uintptr(unsafe.Pointer(unifiedProvider)),
		SECPKG_CRED_INBOUND,
		0,
		uintptr(unsafe.Pointer(&creds)),
		0,
		0,
		uintptr(unsafe.Pointer(&ServerCredHandle)),
		uintptr(unsafe.Pointer(&tsExpiry)),
	)

	if uint32(ret) != SEC_E_OK {
		return fmt.Errorf("AcquireCredentialsHandleW failed with 0x%x", uint32(ret))
	}

	return nil
}

// ProcessHandshake wraps AcceptSecurityContext and handles the ALPN injection.
func ProcessHandshake(tlsCtxt *SecHandle, inBytes []byte) ([]byte, uint32, uint32, error) {
	// ALPN buffer
	alpn := SecApplicationProtocolList{
		ProtoNegoExt:     SEC_APPLICATION_PROTOCOL_NEGOTIATION_EXT,
		ProtocolListSize: 9,
	}
	// http/1.1
	alpn.ProtocolList[0] = 8
	copy(alpn.ProtocolList[1:9], []byte("http/1.1"))

	var inBuffers []SecBuffer
	if len(inBytes) > 0 {
		inBuffers = append(inBuffers, SecBuffer{
			cbBuffer:   uint32(len(inBytes)),
			BufferType: SECBUFFER_TOKEN,
			pvBuffer:   uintptr(unsafe.Pointer(&inBytes[0])),
		})
	}
	// Add ALPN to input buffers
	/*
	inBuffers = append(inBuffers, SecBuffer{
		cbBuffer:   18, // 4 (ProtoNegoExt) + 2 (ProtocolListSize) + 12 (ProtocolList) = 18 bytes without padding
		BufferType: SECBUFFER_APPLICATION_PROTOCOLS,
		pvBuffer:   uintptr(unsafe.Pointer(&alpn)),
	})
	*/
	// Schannel requires a SECBUFFER_EMPTY to return SECBUFFER_EXTRA
	inBuffers = append(inBuffers, SecBuffer{
		cbBuffer:   0,
		BufferType: SECBUFFER_EMPTY,
		pvBuffer:   0,
	})

	inDesc := SecBufferDesc{
		ulVersion: 0,
		cBuffers:  uint32(len(inBuffers)),
		pBuffers:  &inBuffers[0],
	}

	// Prepare Output Buffers (for the generated ServerHello/Token)
	outTokenBuf := make([]byte, 16384)
	outBuffers := []SecBuffer{
		{
			cbBuffer:   0,
			BufferType: SECBUFFER_TOKEN,
			pvBuffer:   0,
		},
		{
			cbBuffer:   0,
			BufferType: SECBUFFER_ALERT,
			pvBuffer:   0,
		},
		{
			cbBuffer:   0,
			BufferType: SECBUFFER_EMPTY,
			pvBuffer:   0,
		},
	}
	outDesc := SecBufferDesc{
		ulVersion: 0, // SECBUFFER_VERSION
		cBuffers:  uint32(len(outBuffers)),
		pBuffers:  &outBuffers[0],
	}

	// USE ASC_REQ_ALLOCATE_MEMORY to let Schannel allocate if it needs to
	var flags uint32 = ASC_REQ_CONFIDENTIALITY | ASC_REQ_SEQUENCE_DETECT | ASC_REQ_REPLAY_DETECT | ASC_REQ_STREAM | ASC_REQ_EXTENDED_ERROR | 0x00000100

	var outFlags uint32
	var tsExpiry int64

	var pInDesc uintptr
	if len(inBytes) > 0 {
		pInDesc = uintptr(unsafe.Pointer(&inDesc))
	}

	var pCtxtIn uintptr
	if tlsCtxt.dwLower != 0 || tlsCtxt.dwUpper != 0 {
		pCtxtIn = uintptr(unsafe.Pointer(tlsCtxt))
	}

	ret, _, _ := syscall.SyscallN(
		procAcceptSecurityContext.Addr(),
		uintptr(unsafe.Pointer(&ServerCredHandle)),
		pCtxtIn,
		pInDesc,
		uintptr(flags),
		16, // SECURITY_NATIVE_DREP
		uintptr(unsafe.Pointer(tlsCtxt)),
		uintptr(unsafe.Pointer(&outDesc)),
		uintptr(unsafe.Pointer(&outFlags)),
		uintptr(unsafe.Pointer(&tsExpiry)),
	)

	status := uint32(ret)
	
	// If there's a token to send back to the client
	var outBytes []byte
	var extraLen uint32
	
	if status == 0 || status == 0x00090312 { // SEC_E_OK or SEC_I_CONTINUE_NEEDED
		var outBytesReturned []byte
		if outBuffers[0].cbBuffer > 0 && outBuffers[0].pvBuffer != 0 {
			// Read from the allocated memory
			allocatedBytes := unsafe.Slice((*byte)(unsafe.Pointer(outBuffers[0].pvBuffer)), outBuffers[0].cbBuffer)
			outBytesReturned = make([]byte, len(allocatedBytes))
			copy(outBytesReturned, allocatedBytes)
			// Free the context buffer
			syscall.SyscallN(secur32.NewProc("FreeContextBuffer").Addr(), outBuffers[0].pvBuffer)
		} else {
			outBytesReturned = outTokenBuf[:outBuffers[0].cbBuffer]
		}
		outBytes = outBytesReturned
	}

	for i := 0; i < len(inBuffers); i++ {
		if inBuffers[i].BufferType == SECBUFFER_EXTRA {
			extraLen = inBuffers[i].cbBuffer
		}
	}
	
	if status != 0 && status != 0x00090312 && status != SEC_E_INCOMPLETE_MESSAGE {
		return outBytes, extraLen, status, fmt.Errorf("AcceptSecurityContext failed with status 0x%x", status)
	}
	
	return outBytes, extraLen, status, nil
}

// Decrypt performs in-place decryption of TLS application data.
// Returns the length of decrypted plaintext, length of any extra unprocessed bytes, and any error.
func Decrypt(tlsCtxt *SecHandle, ioBuf []byte) (uint32, uint32, error) {
	buffers := [4]SecBuffer{
		{
			cbBuffer:   uint32(len(ioBuf)),
			BufferType: SECBUFFER_DATA,
			pvBuffer:   uintptr(unsafe.Pointer(&ioBuf[0])),
		},
		{BufferType: SECBUFFER_EMPTY},
		{BufferType: SECBUFFER_EMPTY},
		{BufferType: SECBUFFER_EMPTY},
	}

	desc := SecBufferDesc{
		ulVersion: 0,
		cBuffers:  4,
		pBuffers:  &buffers[0],
	}

	ret, _, _ := syscall.SyscallN(
		procDecryptMessage.Addr(),
		uintptr(unsafe.Pointer(tlsCtxt)),
		uintptr(unsafe.Pointer(&desc)),
		0,
		0,
	)

	status := uint32(ret)
	if status == SEC_E_INCOMPLETE_MESSAGE {
		return 0, 0, fmt.Errorf("SEC_E_INCOMPLETE_MESSAGE")
	} else if status != SEC_E_OK && status != SEC_I_RENEGOTIATE {
		return 0, 0, fmt.Errorf("DecryptMessage failed: 0x%x", status)
	}

	var decryptedLen uint32
	var extraLen uint32

	for i := 0; i < 4; i++ {
		if buffers[i].BufferType == SECBUFFER_DATA {
			decryptedLen = buffers[i].cbBuffer
		} else if buffers[i].BufferType == SECBUFFER_EXTRA {
			extraLen = buffers[i].cbBuffer
		}
	}

	return decryptedLen, extraLen, nil
}

// Encrypt performs in-place encryption of TLS application data inside a pre-partitioned slice.
func Encrypt(tlsCtxt *SecHandle, sizes SecPkgContext_StreamSizes, writeBuf []byte, plainLen uint32) (uint32, error) {
	// The writeBuf MUST have space at the front for CbHeader and space at the back for CbTrailer.
	// We expect the plaintext to be already placed at writeBuf[sizes.CbHeader : sizes.CbHeader+plainLen].
	
	buffers := [4]SecBuffer{
		{
			cbBuffer:   sizes.CbHeader,
			BufferType: SECBUFFER_STREAM_HEADER,
			pvBuffer:   uintptr(unsafe.Pointer(&writeBuf[0])),
		},
		{
			cbBuffer:   plainLen,
			BufferType: SECBUFFER_DATA,
			pvBuffer:   uintptr(unsafe.Pointer(&writeBuf[sizes.CbHeader])),
		},
		{
			cbBuffer:   sizes.CbTrailer,
			BufferType: SECBUFFER_STREAM_TRAILER,
			pvBuffer:   uintptr(unsafe.Pointer(&writeBuf[sizes.CbHeader+plainLen])),
		},
		{
			BufferType: SECBUFFER_EMPTY,
		},
	}

	desc := SecBufferDesc{
		ulVersion: 0,
		cBuffers:  4,
		pBuffers:  &buffers[0],
	}

	ret, _, _ := syscall.SyscallN(
		procEncryptMessage.Addr(),
		uintptr(unsafe.Pointer(tlsCtxt)),
		0,
		uintptr(unsafe.Pointer(&desc)),
		0,
	)

	if uint32(ret) != SEC_E_OK {
		return 0, fmt.Errorf("EncryptMessage failed: 0x%x", ret)
	}

	totalLen := buffers[0].cbBuffer + buffers[1].cbBuffer + buffers[2].cbBuffer
	return totalLen, nil
}

// GetStreamSizes retrieves the header/trailer requirements for the TLS context.
func GetStreamSizes(tlsCtxt *SecHandle) (SecPkgContext_StreamSizes, error) {
	var sizes SecPkgContext_StreamSizes
	ret, _, _ := syscall.SyscallN(
		procQueryContextAttributesW.Addr(),
		uintptr(unsafe.Pointer(tlsCtxt)),
		SECPKG_ATTR_STREAM_SIZES,
		uintptr(unsafe.Pointer(&sizes)),
	)
	if uint32(ret) != SEC_E_OK {
		return sizes, fmt.Errorf("QueryContextAttributesW failed: 0x%x", ret)
	}
	return sizes, nil
}

// GetALPNProtocol retrieves the negotiated ALPN protocol from the security context.
func GetALPNProtocol(tlsCtxt *SecHandle) (string, error) {
	var alpn SecPkgContext_ApplicationProtocol
	ret, _, _ := syscall.SyscallN(
		procQueryContextAttributesW.Addr(),
		uintptr(unsafe.Pointer(tlsCtxt)),
		SECPKG_ATTR_APPLICATION_PROTOCOL,
		uintptr(unsafe.Pointer(&alpn)),
	)
	if uint32(ret) != SEC_E_OK {
		return "", fmt.Errorf("QueryContextAttributesW failed: 0x%x", ret)
	}
	if alpn.ProtoNegoStatus == 1 { // SEC_APPLICATION_PROTOCOL_NEGOTIATION_STATUS_SUCCESS
		if alpn.ProtocolIdSize > 0 {
			return string(alpn.ProtocolId[:alpn.ProtocolIdSize]), nil
		}
	}
	return "", nil
}

// DeleteContext frees the security context handle
func DeleteContext(tlsCtxt *SecHandle) {
	if tlsCtxt.dwLower != 0 || tlsCtxt.dwUpper != 0 {
		syscall.SyscallN(procDeleteSecurityContext.Addr(), uintptr(unsafe.Pointer(tlsCtxt)))
		tlsCtxt.dwLower = 0
		tlsCtxt.dwUpper = 0
	}
}
