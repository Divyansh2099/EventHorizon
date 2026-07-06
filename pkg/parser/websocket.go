package parser

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
)

// WSFrame represents a parsed WebSocket frame mapped directly to a static buffer.
// The Payload field is a slice pointing exactly to the unmasked data within the original ReadBuffer.
type WSFrame struct {
	Fin     bool
	Opcode  byte
	Payload []byte
}

// ParseWSFrame decodes the WebSocket frame in-place and unmasks it without heap allocations.
// It returns the parsed frame, the total number of bytes consumed by the entire frame, and any error.
func ParseWSFrame(buf []byte) (WSFrame, uint32, error) {
	if len(buf) < 2 {
		return WSFrame{}, 0, ErrIncomplete
	}

	fin := (buf[0] & 0x80) != 0
	opcode := buf[0] & 0x0F
	
	masked := (buf[1] & 0x80) != 0
	payloadLen := uint64(buf[1] & 0x7F)
	
	offset := uint32(2)

	if payloadLen == 126 {
		if len(buf) < 4 {
			return WSFrame{}, 0, ErrIncomplete
		}
		payloadLen = uint64(binary.BigEndian.Uint16(buf[2:4]))
		offset = 4
	} else if payloadLen == 127 {
		if len(buf) < 10 {
			return WSFrame{}, 0, ErrIncomplete
		}
		payloadLen = binary.BigEndian.Uint64(buf[2:10])
		offset = 10
	}
	
	var maskKey []byte
	if masked {
		if len(buf) < int(offset)+4 {
			return WSFrame{}, 0, ErrIncomplete
		}
		maskKey = buf[offset : offset+4]
		offset += 4
	}
	
	if payloadLen > uint64(len(buf)) || uint64(offset)+payloadLen < payloadLen {
		return WSFrame{}, 0, ErrIncomplete
	}

	if uint64(len(buf)) < uint64(offset)+payloadLen {
		return WSFrame{}, 0, ErrIncomplete
	}
	
	payload := buf[uint64(offset) : uint64(offset)+payloadLen]
	
	// Zero-allocation inline unmasking! We XOR directly onto the buffer.
	if masked {
		for i := uint64(0); i < payloadLen; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}
	
	totalConsumed := uint32(offset) + uint32(payloadLen)
	
	return WSFrame{
		Fin:     fin,
		Opcode:  opcode,
		Payload: payload,
	}, totalConsumed, nil
}

// ComputeAcceptKey computes the Sec-WebSocket-Accept key without heap allocations.
func ComputeAcceptKey(wsKey []byte, dst []byte) uint32 {
	// Magic GUID for WebSockets: 258EAFA5-E914-47DA-95CA-C5AB0DC85B11 (36 bytes)
	var shaBuf [64]byte // Stack array to avoid heap allocation
	keyLen := len(wsKey)
	
	copy(shaBuf[:keyLen], wsKey)
	copy(shaBuf[keyLen:keyLen+36], []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	
	hash := sha1.Sum(shaBuf[:keyLen+36])
	
	// Encode to base64 directly into the destination buffer
	base64.StdEncoding.Encode(dst, hash[:])
	
	return uint32(base64.StdEncoding.EncodedLen(20))
}

// WriteWSFrame formats a WebSocket frame (Server-to-Client, unmasked) into a buffer.
// Returns the number of bytes written.
func WriteWSFrame(dst []byte, offset uint32, opcode byte, payload []byte) uint32 {
	payloadLen := len(payload)
	startOffset := offset
	
	// Byte 0: FIN (0x80) | opcode
	dst[offset] = 0x80 | opcode
	offset++
	
	// Byte 1: Mask (0x00 for server->client) | Payload length
	if payloadLen < 126 {
		dst[offset] = byte(payloadLen)
		offset++
	} else if payloadLen <= 65535 {
		dst[offset] = 126
		offset++
		binary.BigEndian.PutUint16(dst[offset:offset+2], uint16(payloadLen))
		offset += 2
	} else {
		dst[offset] = 127
		offset++
		binary.BigEndian.PutUint64(dst[offset:offset+8], uint64(payloadLen))
		offset += 8
	}
	
	if payloadLen > 0 {
		copy(dst[offset:offset+uint32(payloadLen)], payload)
		offset += uint32(payloadLen)
	}
	
	return offset - startOffset
}
