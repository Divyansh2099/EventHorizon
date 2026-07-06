package parser

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// HTTP/2 Frame Types
const (
	FrameData         = 0x00
	FrameHeaders      = 0x01
	FramePriority     = 0x02
	FrameRSTStream    = 0x03
	FrameSettings     = 0x04
	FramePushPromise  = 0x05
	FramePing         = 0x06
	FrameGoAway       = 0x07
	FrameWindowUpdate = 0x08
	FrameContinuation = 0x09
)

// HTTP/2 Flags
const (
	FlagAck        = 0x01
	FlagEndStream  = 0x01
	FlagEndHeaders = 0x04
	FlagPadded     = 0x08
	FlagPriority   = 0x20
)

// FrameHeader represents the 9-byte HTTP/2 frame header.
type FrameHeader struct {
	Length   uint32
	Type     uint8
	Flags    uint8
	StreamID uint32
}

// StreamCtx represents an active HTTP/2 stream on a connection.
type StreamCtx struct {
	ID         uint32
	State      uint8 // 0: idle, 1: open, 2: closed
	RequestCtx RequestCtx
	WindowSize int32
}

// HTTP2Parser handles parsing HTTP/2 frames directly from the RIO buffer.
type HTTP2Parser struct {
	DynamicTable [64]HeaderSpan
	TableCount   int
	PrefaceRead  bool
}

var ConnectionPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")

func (p *HTTP2Parser) ParsePreface(buf []byte, cursor uint32, length uint32) (uint32, error) {
	if length-cursor < uint32(len(ConnectionPreface)) {
		return 0, ErrIncomplete
	}
	if !bytes.Equal(buf[cursor:cursor+uint32(len(ConnectionPreface))], ConnectionPreface) {
		return 0, errors.New("invalid HTTP/2 preface")
	}
	p.PrefaceRead = true
	return cursor + uint32(len(ConnectionPreface)), nil
}

func (p *HTTP2Parser) ParseFrameHeader(buf []byte, cursor uint32, length uint32) (FrameHeader, uint32, error) {
	if length-cursor < 9 {
		return FrameHeader{}, 0, ErrIncomplete
	}

	headerBytes := buf[cursor : cursor+9]
	lengthVal := uint32(headerBytes[0])<<16 | uint32(headerBytes[1])<<8 | uint32(headerBytes[2])
	typeVal := headerBytes[3]
	flagsVal := headerBytes[4]
	streamID := binary.BigEndian.Uint32(headerBytes[5:9]) & 0x7FFFFFFF

	return FrameHeader{
		Length:   lengthVal,
		Type:     typeVal,
		Flags:    flagsVal,
		StreamID: streamID,
	}, cursor + 9, nil
}

// PackHTTP2Response scans the raw HTTP/1.1 response in ctx.WriteBuffer, extracts the body,
// and writes an HTTP/2 HEADERS frame + DATA frame back into writeBuf.
// Returns the number of bytes written to writeBuf.
func PackHTTP2Response(streamID uint32, ctx *RequestCtx, writeBuf []byte, writeOffset uint32) uint32 {
	rawResponse := ctx.WriteBuffer[:ctx.WriteOffset]
	
	// Find the end of headers \r\n\r\n
	bodyStart := bytes.Index(rawResponse, []byte("\r\n\r\n"))
	var body []byte
	if bodyStart != -1 {
		body = rawResponse[bodyStart+4:]
	}
	
	// HPACK static encode for :status 200 (Index 8)
	headersPayload := []byte{0x88}

	startOffset := writeOffset
	
	// Write HEADERS frame
	writeOffset = WriteFrame(writeBuf, writeOffset, uint32(len(headersPayload)), FrameHeaders, FlagEndHeaders, streamID, headersPayload)

	// Write DATA frame
	if len(body) > 0 {
		writeOffset = WriteFrame(writeBuf, writeOffset, uint32(len(body)), FrameData, FlagEndStream, streamID, body)
	} else {
		// Empty DATA frame with EndStream
		writeOffset = WriteFrame(writeBuf, writeOffset, 0, FrameData, FlagEndStream, streamID, nil)
	}

	return writeOffset - startOffset
}

func WriteFrame(buf []byte, offset uint32, length uint32, frameType uint8, flags uint8, streamID uint32, payload []byte) uint32 {
	// Frame Header (9 bytes)
	buf[offset] = byte(length >> 16)
	buf[offset+1] = byte(length >> 8)
	buf[offset+2] = byte(length)
	buf[offset+3] = frameType
	buf[offset+4] = flags
	binary.BigEndian.PutUint32(buf[offset+5:offset+9], streamID)
	
	// Payload
	if length > 0 && payload != nil {
		copy(buf[offset+9:], payload)
	}
	
	return offset + 9 + length
}

var StaticTable = []string{
	"",
	":authority",
	":method GET",
	":method POST",
	":path /",
	":path /index.html",
	":scheme http",
	":scheme https",
	":status 200",
}

func DecodeHPACK(payload []byte, ctx *RequestCtx) {
	var i uint32 = 0
	length := uint32(len(payload))

	for i < length {
		b := payload[i]
		if b&0x80 != 0 {
			index := b & 0x7F
			if int(index) < len(StaticTable) {
				str := StaticTable[index]
				if str == ":method GET" {
					copy(ctx.Buffer[ctx.WriteOffset:], "GET")
					ctx.Method = Span{Start: ctx.WriteOffset, End: ctx.WriteOffset + 3}
					ctx.WriteOffset += 3
				} else if str == ":method POST" {
					copy(ctx.Buffer[ctx.WriteOffset:], "POST")
					ctx.Method = Span{Start: ctx.WriteOffset, End: ctx.WriteOffset + 4}
					ctx.WriteOffset += 4
				} else if str == ":path /" {
					copy(ctx.Buffer[ctx.WriteOffset:], "/")
					ctx.Path = Span{Start: ctx.WriteOffset, End: ctx.WriteOffset + 1}
					ctx.WriteOffset += 1
				}
			}
			i++
		} else if b&0x40 != 0 {
			index := b & 0x3F
			i++
			if index == 0 {
				nameLen := uint32(payload[i] & 0x7F)
				i++
				i += nameLen
			}
			valLen := uint32(payload[i] & 0x7F)
			i++
			if i+valLen <= length {
				valBytes := payload[i : i+valLen]
				if bytes.HasPrefix(valBytes, []byte("/api/")) || bytes.HasPrefix(valBytes, []byte("/")) {
					copy(ctx.Buffer[ctx.WriteOffset:], valBytes)
					ctx.Path = Span{Start: ctx.WriteOffset, End: ctx.WriteOffset + uint32(len(valBytes))}
					ctx.WriteOffset += uint32(len(valBytes))
				}
			}
			i += valLen
		} else {
			i++
		}
	}
}
