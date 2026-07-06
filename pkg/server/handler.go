package server

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
	"github.com/eventhorizon/pkg/connection"
	"github.com/eventhorizon/pkg/iocp"
	"github.com/eventhorizon/pkg/metrics"
	"github.com/eventhorizon/pkg/parser"
	"github.com/eventhorizon/pkg/rio"
	"github.com/eventhorizon/pkg/tls"
	"golang.org/x/sys/windows"
)

var wsMagicString = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func computeWSAccept(key []byte) []byte {
	var buf [128]byte
	n := copy(buf[:], key)
	copy(buf[n:], wsMagicString)
	hash := sha1.Sum(buf[:n+len(wsMagicString)])
	
	// We allocate the result since this only happens once per connection lifecycle
	out := make([]byte, 28)
	base64.StdEncoding.Encode(out, hash[:])
	return out
}

// handleIO is the callback function executed directly by IOCP worker goroutines
// when the Windows kernel notifies them of an I/O completion.
func (s *Server) handleIO(key uintptr, overlapped *iocp.OverlappedCtx, bytesTransferred uint32, err error) {
	if overlapped == nil {
		// This indicates a shutdown signal or a catastrophic port failure.
		return
	}

	// Because we embedded the Overlapped struct inside our Conn, and safely
	// stored a pointer to the Conn inside OverlappedCtx, we can retrieve
	// the active connection context with zero allocations.
	conn := (*connection.Conn)(overlapped.Conn)

	// An error typically means the client forcefully reset the connection
	// or the network dropped. Release resources immediately.
	if err != nil {
		conn.Release()
		return
	}

	if conn == nil {
		panic("conn is nil!")
	}
	if s.iocpPort == nil {
		panic("s.iocpPort is nil!")
	}

	switch overlapped.Op {
	case iocp.OpRead, iocp.OpResume:
		atomic.StoreInt64(&conn.LastActive, time.Now().UnixNano())
		if conn.ReadOverlapped.Op == iocp.OpRead {
			// 2. RIOReceive completed. We have bytes in our ReadBuffer.
			
			// Restore the original RIO chunk boundaries that were modified in postRead
			// to instruct the kernel to append bytes.
			conn.RioBuffer.Offset -= conn.ReadLength
			conn.RioBuffer.Length += conn.ReadLength

			if bytesTransferred == 0 {
				// A read of 0 bytes indicates a graceful TCP FIN from the client.
				conn.Release()
				return
			}

			if conn.ReadLength == 0 {
				conn.StartTime = time.Now().UnixNano()
			}

			atomic.AddUint64(&metrics.Global.Reads, 1)
			atomic.AddUint64(&metrics.Global.BytesIn, uint64(bytesTransferred))

			conn.ReadLength += bytesTransferred
		}

		// --- PHASE 18: TLS HANDSHAKE ---
		if conn.State == connection.StateHandshake {
			outBytes, extraLen, status, err := tls.ProcessHandshake(&conn.TlsCtxt, conn.ReadBuffer[conn.ReadCursor:conn.ReadLength])
			if err != nil {
				if err.Error() == "SEC_E_INCOMPLETE_MESSAGE" {
					// We need more bytes. Re-arm read.
					conn.ReadOverlapped.Overlapped = windows.Overlapped{}
					s.postRead(conn)
					return
				}
				fmt.Println("Handshake error:", err, "Status:", status)
				conn.Release()
				return
			}
			
			// We can remove the old if status == tls.SEC_E_INCOMPLETE_MESSAGE block below, 
			// because it's now handled by the err check above.
			
			// ProcessHandshake consumed bytes
			consumed := (conn.ReadLength - conn.ReadCursor) - extraLen
			conn.ReadCursor += consumed

			if len(outBytes) > 0 {
				copy(conn.WriteBuffer[conn.WriteCursor:], outBytes)
				conn.WriteCursor += uint32(len(outBytes))
			}
			
			if status == tls.SEC_E_OK {
				sizes, _ := tls.GetStreamSizes(&conn.TlsCtxt)
				conn.StreamSizes = sizes
				
				protocol, err := tls.GetALPNProtocol(&conn.TlsCtxt)
				if err == nil && protocol != "" {
					conn.Protocol = protocol
				} else {
					conn.Protocol = "http/1.1" // default fallback
				}

				if conn.WriteCursor > 0 {
					// Need to send the final handshake token first.
					// We'll transition to StateReading in OpWrite completion.
					s.postWrite(conn, conn.WriteCursor)
					return
				}
				
				conn.State = connection.StateReading
				if extraLen == 0 {
					conn.ReadCursor = 0
					conn.ReadLength = 0
					conn.ReadOverlapped.Overlapped = windows.Overlapped{}
					s.postRead(conn)
					return
				}
			} else if status == tls.SEC_I_CONTINUE_NEEDED {
				if conn.WriteCursor > 0 {
					s.postWrite(conn, conn.WriteCursor)
				} else {
					conn.ReadOverlapped.Overlapped = windows.Overlapped{}
					s.postRead(conn)
				}
				return
			}
		}

		// --- PHASE 18: TLS DECRYPTION & HTTP PARSING ---
		if conn.State == connection.StateReading || conn.State == connection.StateWebSocket {
			initialReadCursor := conn.ReadCursor
			if conn.State != connection.StateWebSocket {
				initialReadCursor = 0
			}

			for conn.ReadCursor < conn.ReadLength {
				encryptedBytes := conn.ReadBuffer[conn.ReadCursor:conn.ReadLength]
				conn.TlsMu.Lock()
				decryptedLen, extraLen, err := tls.Decrypt(&conn.TlsCtxt, encryptedBytes)
				conn.TlsMu.Unlock()
				
				if err != nil {
					if err.Error() == "SEC_E_INCOMPLETE_MESSAGE" {
						remaining := conn.ReadLength - conn.ReadCursor
						copy(conn.ReadBuffer[initialReadCursor:], conn.ReadBuffer[conn.ReadCursor:conn.ReadLength])
						conn.ReadLength = initialReadCursor + remaining
						conn.ReadCursor = initialReadCursor
						conn.ReadOverlapped.Overlapped = windows.Overlapped{}
						s.postRead(conn)
						return
					}
					fmt.Println("Decrypt error:", err)
					conn.Release()
					return
				}

				// The plaintext starts AFTER the header.
				plaintextStart := conn.ReadCursor + conn.StreamSizes.CbHeader
				
				if conn.State == connection.StateWebSocket && conn.ReadCursor > 0 {
					// We have shifted unparsed plaintext from a previous TLS record to the front of ReadBuffer.
					// The new plaintext is at plaintextStart. We must shift it backwards by CbHeader
					// to make it contiguous with the unparsed plaintext.
					copy(conn.ReadBuffer[conn.ReadCursor:], conn.ReadBuffer[plaintextStart : plaintextStart+decryptedLen])
					plaintextStart = 0
					decryptedLen += conn.ReadCursor // Total contiguous plaintext length
				}
				
				plaintextData := conn.ReadBuffer[plaintextStart : plaintextStart+decryptedLen]

				if conn.State == connection.StateWebSocket {
					wsCursor := uint32(0)
					if conn.ReadCursor > 0 {
						// Reset ReadCursor so we don't treat it as an offset for the next TLS record's decryption buffer
						conn.ReadCursor = 0 
					}
					
					for wsCursor < decryptedLen {
						frame, consumed, err := parser.ParseWSFrame(plaintextData[wsCursor:decryptedLen])
						if err == parser.ErrIncomplete {
							break
						} else if err != nil {
							conn.Release()
							return
						}
						
						if frame.Opcode == 0x8 { // Close
							conn.Release()
							return
						} else if frame.Opcode == 0x9 { // Ping
							// Write Pong frame directly to WriteBuffer (starting at CbHeader offset)
							conn.WriteCursor = conn.StreamSizes.CbHeader
							conn.WriteCursor = parser.WriteWSFrame(conn.WriteBuffer, conn.WriteCursor, 0xA, frame.Payload)
							
							totalSize := conn.WriteCursor - conn.StreamSizes.CbHeader
							conn.TlsMu.Lock()
							finalWriteLen, err := tls.Encrypt(&conn.TlsCtxt, conn.StreamSizes, conn.WriteBuffer, totalSize)
							conn.TlsMu.Unlock()
							if err != nil {
								conn.Release()
								return
							}
							conn.State = connection.StateWebSocket
							conn.WriteOverlapped.Op = iocp.OpWrite
							s.postWrite(conn, finalWriteLen)
							// Note: This breaks pipelining of multiple Pings in a single chunk,
							// but Pings are rarely pipelined.
							return 
						} else if frame.Opcode == 0x1 || frame.Opcode == 0x2 { // Text or Binary
							if s.Router.WSRoute != nil {
								s.Router.WSRoute(conn, frame)
							}
						}
						
						wsCursor += consumed
					}
					
					if wsCursor < decryptedLen {
						// Fragmented frame! Shift unparsed plaintext to the front of ReadBuffer.
						remaining := decryptedLen - wsCursor
						copy(conn.ReadBuffer, plaintextData[wsCursor:decryptedLen])
						
						// Setup ReadCursor to protect the plaintext from being overwritten by the next RIOReceive,
						// and to serve as the starting point for the NEXT tls.Decrypt.
						conn.ReadCursor = remaining
						conn.ReadLength = remaining
						conn.ReadOverlapped.Overlapped = windows.Overlapped{}
						s.postRead(conn)
						return
					}
					
					// Entire chunk consumed
					conn.ReadCursor = 0
					conn.ReadLength = 0
					conn.ReadOverlapped.Overlapped = windows.Overlapped{}
					s.postRead(conn)
					return
				}

				// CAPACITY CHECK
				const MAX_EXPECTED_RESPONSE_SIZE = 512
				requiredSpace := conn.StreamSizes.CbHeader + MAX_EXPECTED_RESPONSE_SIZE + conn.StreamSizes.CbTrailer
				if conn.WriteCursor+requiredSpace > uint32(len(conn.WriteBuffer)) {
					break
				}

				// Reserve space for TLS Header
				if conn.WriteCursor == 0 {
					conn.WriteCursor = conn.StreamSizes.CbHeader
				}

				if conn.Protocol == "h2" {
					h2Cursor := uint32(0)
					if !conn.H2Parser.PrefaceRead {
						newCursor, err := conn.H2Parser.ParsePreface(plaintextData, h2Cursor, decryptedLen)
						if err == parser.ErrIncomplete {
							break
						} else if err != nil {
							conn.Release()
							return
						}
						h2Cursor = newCursor
						// Send Server SETTINGS frame immediately after reading preface
						conn.WriteCursor = parser.WriteFrame(conn.WriteBuffer, conn.WriteCursor, 0, parser.FrameSettings, 0, 0, nil)
					}
					
					for h2Cursor < decryptedLen {
						frameHeader, newCursor, err := conn.H2Parser.ParseFrameHeader(plaintextData, h2Cursor, decryptedLen)
						if err == parser.ErrIncomplete {
							break
						} else if err != nil {
							conn.Release()
							return
						}
						
						payloadStart := newCursor
						payloadEnd := newCursor + frameHeader.Length
						if payloadEnd > decryptedLen {
							break // Need more bytes for the frame payload
						}
						
						if frameHeader.Type == parser.FrameSettings {
							if frameHeader.Flags&parser.FlagAck == 0 {
								// Send SETTINGS ACK
								conn.WriteCursor = parser.WriteFrame(conn.WriteBuffer, conn.WriteCursor, 0, parser.FrameSettings, parser.FlagAck, 0, nil)
							}
						} else if frameHeader.Type == parser.FrameHeaders {
							// Use the unused portion of ReadBuffer as temporary scratch space for the handler
							scratchBuf := conn.ReadBuffer[conn.ReadLength:]
							req := parser.GetRequestCtx(plaintextData, scratchBuf)
							req.Conn = unsafe.Pointer(conn)
							
							parser.DecodeHPACK(plaintextData[payloadStart:payloadEnd], req)
							
							handler := s.Router.Lookup(req.MethodBytes(), req.PathBytes(), req)
							if handler != nil {
								handler(req)
							} else {
								req.SetStatusCode(404)
								req.Write([]byte("Content-Length: 9\r\nConnection: close\r\n\r\nNot Found"))
							}
							
							// Pack response into HTTP/2 format, writing to WriteBuffer
							packedBytes := parser.PackHTTP2Response(frameHeader.StreamID, req, conn.WriteBuffer, conn.WriteCursor)
							conn.WriteCursor = packedBytes
							
							req.Release()
						}
						
						h2Cursor = payloadEnd
					}
					
					// Decrypt operates in-place, mark all consumed
					conn.ReadCursor = conn.ReadLength 
					
					conn.State = connection.StateWriting
					conn.KeepAlive = true
					
					// Apply TLS encrypt
					totalSize := conn.WriteCursor - conn.StreamSizes.CbHeader
					if totalSize > 0 {
						conn.TlsMu.Lock()
						finalWriteLen, err := tls.Encrypt(&conn.TlsCtxt, conn.StreamSizes, conn.WriteBuffer, totalSize)
						conn.TlsMu.Unlock()
						if err != nil {
							conn.Release()
							return
						}
						s.postWrite(conn, finalWriteLen)
					} else {
						// Nothing to write, just post read again
						conn.ReadOverlapped.Overlapped = windows.Overlapped{}
						s.postRead(conn)
					}
					return
				}

				p := parser.Parser{}
				p.Reset()

				req := parser.GetRequestCtx(plaintextData, conn.WriteBuffer[conn.WriteCursor:])
				req.Conn = unsafe.Pointer(conn)

				_, parseErr := p.Parse(decryptedLen, req)

				if parseErr != nil {
					if parseErr == parser.ErrIncomplete {
						// HTTP Incomplete. Since we decrypt in place, we cannot safely retain plaintext
						// if we need to fetch more encrypted bytes. Close for now.
						conn.Release()
						return
					}

					atomic.AddUint64(&metrics.Global.ParserErrors, 1)
					req.SetStatusCode(400)
					req.Write([]byte("Content-Length: 11\r\nConnection: close\r\n\r\nBad Request"))
					conn.State = connection.StateWriting
					conn.KeepAlive = false
					
					conn.WriteCursor += req.WriteOffset
					req.Release()
					break
				}

				atomic.AddUint64(&metrics.Global.RequestsParsed, 1)

				if req.IsWebSocket {
					conn.IsWebSocket = true
					var b64Buf [32]byte
					b64Len := parser.ComputeAcceptKey(req.WSKeyBytes(), b64Buf[:])
					
					conn.WriteCursor = conn.StreamSizes.CbHeader
					
					// Format the 101 Switching Protocols response directly into WriteBuffer
					respHeader := []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ")
					copy(conn.WriteBuffer[conn.WriteCursor:], respHeader)
					conn.WriteCursor += uint32(len(respHeader))
					
					copy(conn.WriteBuffer[conn.WriteCursor:], b64Buf[:b64Len])
					conn.WriteCursor += b64Len
					
					endHeaders := []byte("\r\n\r\n")
					copy(conn.WriteBuffer[conn.WriteCursor:], endHeaders)
					conn.WriteCursor += uint32(len(endHeaders))
					
					conn.State = connection.StateWebSocket
					
					totalSize := conn.WriteCursor - conn.StreamSizes.CbHeader
					conn.TlsMu.Lock()
					finalWriteLen, err := tls.Encrypt(&conn.TlsCtxt, conn.StreamSizes, conn.WriteBuffer, totalSize)
					conn.TlsMu.Unlock()
					if err != nil {
						conn.Release()
						return
					}
					conn.WriteOverlapped.Op = iocp.OpWrite
					s.postWrite(conn, finalWriteLen)
					return
				} else {
					handler := s.Router.Lookup(req.MethodBytes(), req.PathBytes(), req)
					if handler != nil {
						handler(req)
					} else {
						req.SetStatusCode(404)
						req.Write([]byte("Content-Length: 9\r\nConnection: close\r\n\r\nNot Found"))
					}
					conn.KeepAlive = req.KeepAlive
				}

				conn.WriteCursor += req.WriteOffset

				if req.TransmittedFile {
					// Record consumed TLS bytes before returning so KeepAlive can resume correctly
					recordSize := (conn.ReadLength - conn.ReadCursor) - extraLen
					conn.ReadCursor += recordSize
					
					s.iocpPort.Associate(conn.FileHandle, uintptr(unsafe.Pointer(conn)))
					s.PostFileRead(conn)
					req.Release()
					return
				}

				req.Release()

				// Record consumed TLS bytes
				recordSize := (conn.ReadLength - conn.ReadCursor) - extraLen
				conn.ReadCursor += recordSize

				if !conn.KeepAlive {
					break
				}
			}

			// Egress: Encrypt and send
			if conn.WriteCursor > conn.StreamSizes.CbHeader {
				plainLen := conn.WriteCursor - conn.StreamSizes.CbHeader
				conn.TlsMu.Lock()
				encryptedLen, err := tls.Encrypt(&conn.TlsCtxt, conn.StreamSizes, conn.WriteBuffer, plainLen)
				conn.TlsMu.Unlock()
				if err != nil {
					conn.Release()
					return
				}
				conn.State = connection.StateWriting
				s.postWrite(conn, encryptedLen)
			} else if conn.ReadCursor == conn.ReadLength {
				// Chunk fully consumed
				conn.ReadCursor = 0
				conn.ReadLength = 0
				conn.ReadOverlapped.Overlapped = windows.Overlapped{}
				s.postRead(conn)
			} else {
				// Extra unparsed bytes remaining
				remaining := conn.ReadLength - conn.ReadCursor
				copy(conn.ReadBuffer, conn.ReadBuffer[conn.ReadCursor:conn.ReadLength])
				conn.ReadLength = remaining
				conn.ReadCursor = 0
				
				conn.ReadOverlapped.Overlapped = windows.Overlapped{}
				conn.State = connection.StateReading
				conn.ReadOverlapped.Op = iocp.OpResume
				compKey := uintptr(unsafe.Pointer(conn))
				s.iocpPort.Post(0, compKey, &conn.ReadOverlapped.Overlapped)
			}
		}


	case iocp.OpWSWrite:
		conn.WriteMu.Unlock()
		return

	case iocp.OpWrite:
		atomic.StoreInt64(&conn.LastActive, time.Now().UnixNano())
		// 3. WSASend completed.
		if conn.StartTime > 0 {
			latency := uint64(time.Now().UnixNano() - conn.StartTime)
			metrics.Global.Tracker.Record(latency)
			conn.StartTime = 0
		}

		atomic.AddUint64(&metrics.Global.Writes, 1)
		atomic.AddUint64(&metrics.Global.BytesOut, uint64(bytesTransferred))
		
		// Reset WriteCursor upon successful transmission
		if conn.TlsCtxt.IsValid() {
			conn.WriteCursor = conn.StreamSizes.CbHeader
		} else {
			conn.WriteCursor = 0
		}

		// 3. DUAL-POLL DISPATCH CLARITY: OpWrite comes from RIODequeueCompletion.
		// If we are currently streaming a file, determine the next step.
		if conn.FileHandle != windows.InvalidHandle {
			if conn.FileOffset >= conn.FileSize {
				// We have transmitted the entire file.
				windows.CloseHandle(conn.FileHandle)
				conn.FileHandle = windows.InvalidHandle
			} else {
				// We must read the next chunk!
				s.PostFileRead(conn)
				return // Do not enter the KeepAlive check until the file is fully sent!
			}
		}

		// Check if we should recycle the connection or tear it down.
		if conn.IsWebSocket {
			// WebSockets do not currently support HTTP-style pipelining loops in this kernel
			// We just reset and issue a new WS read.
			conn.ReadLength = 0
			conn.ReadCursor = 0
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			conn.WriteOverlapped.Overlapped = windows.Overlapped{}
			conn.State = connection.StateWebSocket
			conn.ReadOverlapped.Op = iocp.OpRead
			s.postRead(conn)
		} else if conn.TlsCtxt.IsValid() && conn.State == connection.StateHandshake {
			if conn.StreamSizes.CbHeader > 0 {
				// TLS Handshake response sent. Now we transition to reading encrypted application data.
				conn.State = connection.StateReading
				if conn.ReadCursor < conn.ReadLength {
					remaining := conn.ReadLength - conn.ReadCursor
					copy(conn.ReadBuffer, conn.ReadBuffer[conn.ReadCursor:conn.ReadLength])
					conn.ReadLength = remaining
					conn.ReadCursor = 0
					conn.ReadOverlapped.Op = iocp.OpResume
					compKey := uintptr(unsafe.Pointer(conn))
					s.iocpPort.Post(0, compKey, &conn.ReadOverlapped.Overlapped)
				} else {
					conn.ReadLength = 0
					conn.ReadCursor = 0
					conn.ReadOverlapped.Overlapped = windows.Overlapped{}
					conn.WriteOverlapped.Overlapped = windows.Overlapped{}
					s.postRead(conn)
				}
			} else {
				// Intermediate handshake response sent.
				if conn.ReadCursor < conn.ReadLength {
					// We have more bytes in the buffer, resume parsing.
					conn.ReadOverlapped.Op = iocp.OpResume
					compKey := uintptr(unsafe.Pointer(conn))
					s.iocpPort.Post(0, compKey, &conn.ReadOverlapped.Overlapped)
				} else {
					// Need more handshake bytes from the client.
					conn.ReadOverlapped.Overlapped = windows.Overlapped{}
					s.postRead(conn)
				}
			}
		} else if conn.KeepAlive {
			if conn.ReadCursor < conn.ReadLength {
				// We have more pipelined bytes to parse! Resume the parsing loop.
				conn.ReadOverlapped.Overlapped = windows.Overlapped{}
				conn.State = connection.StateReading
				conn.ReadOverlapped.Op = iocp.OpResume
				compKey := uintptr(unsafe.Pointer(conn))
				s.iocpPort.Post(0, compKey, &conn.ReadOverlapped.Overlapped)
			} else {
				// Buffer is fully consumed. Wait for the next request.
				conn.ReadCursor = 0
				conn.ReadLength = 0
				conn.ReadOverlapped.Overlapped = windows.Overlapped{}
				conn.State = connection.StateReading
				s.postRead(conn)
			}
		} else {
			// Tear down the TCP connection gracefully and return buffers to the pool.
			conn.Release()
		}

	case iocp.OpTransmitFile:
		// 4. TransmitFile completed.
		if conn.StartTime > 0 {
			latency := uint64(time.Now().UnixNano() - conn.StartTime)
			metrics.Global.Tracker.Record(latency)
			conn.StartTime = 0
		}
		conn.WriteCursor = 0

		if conn.KeepAlive {
			s.postRead(conn)
		} else {
			conn.Release()
		}

	case iocp.OpFileRead:
		fmt.Println("OpFileRead complete, bytes:", bytesTransferred)
		// 3. DUAL-POLL DISPATCH CLARITY: OpFileRead comes from GetQueuedCompletionStatus (Standard IOCP).
		if bytesTransferred == 0 {
			// EOF or read error midway
			conn.Release()
			return
		}
		
		conn.FileOffset += uint64(bytesTransferred)
		
		totalSize := conn.WriteCursor + bytesTransferred - conn.StreamSizes.CbHeader
		if conn.TlsCtxt.IsValid() {
			conn.TlsMu.Lock()
			encryptedLen, err := tls.Encrypt(&conn.TlsCtxt, conn.StreamSizes, conn.WriteBuffer, totalSize)
			conn.TlsMu.Unlock()
			if err != nil {
				fmt.Println("File chunk encrypt error:", err)
				conn.Release()
				return
			}
			conn.State = connection.StateWriting
			conn.WriteOverlapped.Op = iocp.OpWrite
			s.postWrite(conn, encryptedLen)
		} else {
			conn.State = connection.StateWriting
			conn.WriteOverlapped.Op = iocp.OpWrite
			s.postWrite(conn, conn.WriteCursor + bytesTransferred)
		}

	case iocp.OpWSRead:
		if bytesTransferred == 0 {
			conn.Release()
			return
		}
		
		conn.ReadLength += bytesTransferred
		
		frame, consumed, err := parser.ParseWSFrame(conn.ReadBuffer[:conn.ReadLength])
		if err == parser.ErrIncomplete {
			if conn.ReadLength == uint32(len(conn.ReadBuffer)) {
				// Frame exceeds buffer size
				conn.Release()
				return
			}
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			s.postRead(conn) // Uses OpWSRead because we already mutated it
			return
		}

		if err != nil {
			conn.Release()
			return
		}
		
		conn.ReadCursor = consumed
		
		// If it's a close frame, release
		if frame.Opcode == 0x8 {
			conn.Release()
			return
		}
		
		// Handle frame application routing
		if s.Router.WSRoute != nil {
			s.Router.WSRoute(conn, frame)
		} else {
			// Echo server behavior by default if no WSRoute is defined
		}
		
		// Shift any remaining WS bytes and read again (WS pipelining)
		if conn.ReadCursor < conn.ReadLength {
			remaining := conn.ReadLength - conn.ReadCursor
			copy(conn.ReadBuffer[:remaining], conn.ReadBuffer[conn.ReadCursor:conn.ReadLength])
			conn.ReadLength = 0
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			
			compKey := uintptr(unsafe.Pointer(conn))
			s.iocpPort.Post(remaining, compKey, &conn.ReadOverlapped.Overlapped)
		} else {
			conn.ReadLength = 0
			conn.ReadOverlapped.Overlapped = windows.Overlapped{}
			s.postRead(conn)
		}
	}
}

// postRead submits an asynchronous RIOReceive operation to the kernel.
func (s *Server) postRead(conn *connection.Conn) {
	// Ensure the connection has an active RIO Buffer assigned
	if conn.RioBuffer.BufferId == 0 {
		var err error
		conn.RioBuffer, err = s.rioEngine.GetChunk()
		if err != nil {
			conn.Release()
			return
		}
		// Map the Go slice directly to the hardware-registered memory!
		conn.ReadBuffer = s.rioEngine.GetSlice(conn.RioBuffer)
	}
	if conn.RioWriteBuffer.BufferId == 0 {
		var err error
		conn.RioWriteBuffer, err = s.rioEngine.GetChunk()
		if err != nil {
			conn.Release()
			return
		}
		conn.WriteBuffer = s.rioEngine.GetSlice(conn.RioWriteBuffer)
	}

	// Ensure the operation type is correctly set for network reads.
	// This prevents bugs if postRead is called after an OpResume.
	if conn.ReadOverlapped.Op != iocp.OpWSRead {
		conn.ReadOverlapped.Op = iocp.OpRead
	}

	// We must slice the RIO_BUF to respect already-read pipelined bytes
	// We MUST modify the pinned conn structure directly, because RIOReceive
	// reads the RIO_BUF array asynchronously. Passing a stack pointer will cause WSAEFAULT.
	conn.RioBuffer.Offset += conn.ReadLength
	conn.RioBuffer.Length -= conn.ReadLength
	// RIOReceive(SocketQueue, pData, DataBufferCount, Flags, RequestContext)
	// We pass the OverlappedCtx pointer as the RequestContext!
	reqCtx := uintptr(unsafe.Pointer(&conn.ReadOverlapped))
	
	ret, _, errSys := syscall.SyscallN(
		rio.RioTable.RIOReceive,
		uintptr(conn.RioRQ),
		uintptr(unsafe.Pointer(&conn.RioBuffer)),
		1, // DataBufferCount
		0, // Flags
		reqCtx,
	)

	// If RIOReceive returns FALSE (0), the operation failed immediately.
	// We do not check ERROR_IO_PENDING for RIO because a return of TRUE (1)
	// indicates the request was successfully queued to the RequestQueue.
	if ret == 0 {
		if errSys != 0 {
			conn.Release()
		}
	}
}

// postWrite submits an asynchronous RIOSend operation to the kernel.
func (s *Server) postWrite(conn *connection.Conn, writeLen uint32) {
	// We MUST modify the pinned conn structure directly, because RIOSend
	// reads the RIO_BUF array asynchronously.
	// We preserve the original offset and just set the Length for this send.
	conn.RioWriteBuffer.Length = writeLen

	// RIOSend(SocketQueue, pData, DataBufferCount, Flags, RequestContext)
	reqCtx := uintptr(unsafe.Pointer(&conn.WriteOverlapped))
	
	ret, _, errSys := syscall.SyscallN(
		rio.RioTable.RIOSend,
		uintptr(conn.RioRQ),
		uintptr(unsafe.Pointer(&conn.RioWriteBuffer)),
		1, // DataBufferCount
		0, // Flags
		reqCtx,
	)

	// If RIOSend returns FALSE (0), the operation failed immediately.
	if ret == 0 {
		if errSys != 0 {
			conn.Release()
		}
	}
}

// PostFileRead initiates an asynchronous chunked file read directly into the RIO pinned memory.
func (s *Server) PostFileRead(conn *connection.Conn) {
	// 1. OVERLAPPED OFFSET ADVANCEMENT (CRITICAL)
	conn.FileOverlapped.Overlapped = windows.Overlapped{}
	conn.FileOverlapped.Op = iocp.OpFileRead
	conn.FileOverlapped.Overlapped.Offset = uint32(conn.FileOffset)
	conn.FileOverlapped.Overlapped.OffsetHigh = uint32(conn.FileOffset >> 32)
	
	// Ensure we have a WriteBuffer to read into.
	if conn.RioWriteBuffer.BufferId == 0 {
		var err error
		conn.RioWriteBuffer, err = s.rioEngine.GetChunk()
		if err != nil {
			conn.Release()
			return
		}
		conn.WriteBuffer = s.rioEngine.GetSlice(conn.RioWriteBuffer)
	}

	// 2. READ CAPACITY MATH
	remainingBuffer := uint32(len(conn.WriteBuffer)) - conn.WriteCursor
	if conn.TlsCtxt.IsValid() {
		remainingBuffer -= conn.StreamSizes.CbTrailer
	}
	remainingFile := conn.FileSize - conn.FileOffset
	
	bytesToRead := remainingBuffer
	if uint64(bytesToRead) > remainingFile {
		bytesToRead = uint32(remainingFile)
	}
	
	if bytesToRead == 0 {
		// Nothing to read! This shouldn't happen, but safe fallback.
		s.postWrite(conn, conn.WriteCursor)
		return
	}

	var bytesRead uint32 // Overlapped IO will return actual bytes read in the IOCP completion
	
	// Perform the async read directly into the RIO backing slice.
	err := windows.ReadFile(
		conn.FileHandle,
		conn.WriteBuffer[conn.WriteCursor:conn.WriteCursor+bytesToRead],
		&bytesRead,
		&conn.FileOverlapped.Overlapped,
	)
	
	if err != nil && err != windows.ERROR_IO_PENDING {
		conn.Release()
		return
	}
}
