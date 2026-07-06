package parser

import (
	"bytes"
	"errors"
	"unsafe"

	"github.com/eventhorizon/pkg/pool"
)

// Span represents an absolute index boundary within a static buffer.
type Span struct {
	Start uint32
	End   uint32
}

// HeaderSpan represents the absolute index boundaries for a header key-value pair.
type HeaderSpan struct {
	Key   Span
	Value Span
}

const maxHeaders = 128

// RequestCtx stores absolute integer offsets instead of Go slice headers,
// eliminating the 24-byte per-field overhead entirely.
type RequestCtx struct {
	Buffer      []byte // Reference to the static memory array (read buffer)
	WriteBuffer []byte // Reference to the static memory array (write buffer)
	WriteOffset uint32

	Method  Span
	Path    Span
	Version Span
	Body    Span

	KeepAlive     bool
	IsWebSocket   bool
	WSKey         Span
	ContentLength uint32
	Headers       []HeaderSpan
	headerCount   int

	Params     [16]ParamSpan
	paramCount int

	Conn            unsafe.Pointer // Pointer to connection for syscalls
	TransmittedFile bool           // Flag indicating TransmitFile was invoked
}

// ParamSpan represents the key and the index boundary for a path parameter.
type ParamSpan struct {
	Key   string
	Value Span
}

func init() {
	pool.RequestCtxPool.New = func() any {
		return &RequestCtx{
			Headers: make([]HeaderSpan, maxHeaders),
		}
	}
}

// Reset clears the context for the next request in the pipelined lifecycle.
func (r *RequestCtx) Reset(buf []byte, writeBuf []byte) {
	r.Buffer = buf
	r.WriteBuffer = writeBuf
	r.WriteOffset = 0
	r.headerCount = 0
	r.paramCount = 0
	r.KeepAlive = false
	r.IsWebSocket = false
	r.TransmittedFile = false
}

// AddParam stores a parameter span safely.
func (r *RequestCtx) AddParam(key string, start, end uint32) {
	if r.paramCount < len(r.Params) {
		r.Params[r.paramCount].Key = key
		r.Params[r.paramCount].Value = Span{Start: start, End: end}
		r.paramCount++
	}
}

// GetParam retrieves a parameter by key, extracting the byte slice without allocations.
func (r *RequestCtx) GetParam(key string) []byte {
	for i := 0; i < r.paramCount; i++ {
		if r.Params[i].Key == key {
			return r.Buffer[r.Params[i].Value.Start:r.Params[i].Value.End]
		}
	}
	return nil
}

// GetRequestCtx retrieves a zero-allocation context from the pool.
func GetRequestCtx(readBuf []byte, writeBuf []byte) *RequestCtx {
	ctx := pool.RequestCtxPool.Get().(*RequestCtx)
	ctx.Buffer = readBuf
	ctx.WriteBuffer = writeBuf
	ctx.WriteOffset = 0
	ctx.headerCount = 0
	ctx.Method = Span{}
	ctx.Path = Span{}
	ctx.Version = Span{}
	ctx.Body = Span{}
	ctx.IsWebSocket = false
	ctx.WSKey = Span{}
	ctx.KeepAlive = false
	ctx.ContentLength = 0
	ctx.TransmittedFile = false
	return ctx
}

// Release returns the context to the pool.
func (ctx *RequestCtx) Release() {
	ctx.Buffer = nil
	ctx.WriteBuffer = nil
	pool.RequestCtxPool.Put(ctx)
}

// AddHeader registers a new header boundary.
func (ctx *RequestCtx) AddHeader(key, value Span) {
	if ctx.headerCount < len(ctx.Headers) {
		ctx.Headers[ctx.headerCount].Key = key
		ctx.Headers[ctx.headerCount].Value = value
		ctx.headerCount++
	}
}

// MethodBytes returns the method slice directly mapped to the static buffer.
func (ctx *RequestCtx) MethodBytes() []byte {
	return ctx.Buffer[ctx.Method.Start:ctx.Method.End]
}

// BodyBytes returns the body as a byte slice.
func (ctx *RequestCtx) BodyBytes() []byte {
	return ctx.Buffer[ctx.Body.Start:ctx.Body.End]
}

// WSKeyBytes returns the Sec-WebSocket-Key as a byte slice.
func (ctx *RequestCtx) WSKeyBytes() []byte {
	return ctx.Buffer[ctx.WSKey.Start:ctx.WSKey.End]
}

// PathBytes returns the path slice.
func (ctx *RequestCtx) PathBytes() []byte {
	return ctx.Buffer[ctx.Path.Start:ctx.Path.End]
}

// IsMethod provides a zero-allocation way to check the method against a byte slice (e.g. GET).
func (ctx *RequestCtx) IsMethod(m []byte) bool {
	return bytes.Equal(ctx.MethodBytes(), m)
}

// Write copies data into the WriteBuffer and advances the WriteOffset.
func (ctx *RequestCtx) Write(b []byte) {
	n := copy(ctx.WriteBuffer[ctx.WriteOffset:], b)
	ctx.WriteOffset += uint32(n)
}

// SetStatusCode pre-formats standard HTTP headers into the WriteBuffer.
// In a full implementation, you would dynamically map status codes to strings.
func (ctx *RequestCtx) SetStatusCode(code int) {
	// For high-performance zero-allocation, we statically match common codes
	// instead of using fmt.Sprintf or strconv.Itoa.
	switch code {
	case 101:
		ctx.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n"))
	case 200:
		ctx.Write([]byte("HTTP/1.1 200 OK\r\n"))
	case 400:
		ctx.Write([]byte("HTTP/1.1 400 Bad Request\r\n"))
	case 404:
		ctx.Write([]byte("HTTP/1.1 404 Not Found\r\n"))
	case 500:
		ctx.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n"))
	default:
		// Fallback for other codes (would be pre-cached in production)
		ctx.Write([]byte("HTTP/1.1 200 OK\r\n"))
	}
}

// State defines the current state of the HTTP parser.
type State int

const (
	StateMethod State = iota
	StatePath
	StateVersion
	StateHeaderKey
	StateHeaderValue
	StateComplete
	StateError
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrBufferFull = errors.New("request headers exceeded maximum buffer size")
	ErrIncomplete = errors.New("incomplete request")
)

// Parser handles the zero-copy scanning of HTTP payloads.
type Parser struct {
	State       State
	startOffset uint32
	currentKey  Span
}

// Reset clears the parser state for the next request cycle.
func (p *Parser) Reset() {
	p.State = StateMethod
	p.startOffset = 0
	p.currentKey = Span{}
}

// Parse processes the incoming byte buffer linearly, populating boundaries on RequestCtx.
// It returns the number of bytes consumed and any parsing error.
func (p *Parser) Parse(length uint32, ctx *RequestCtx) (uint32, error) {
	var i uint32 = 0
	buf := ctx.Buffer

	for i < length {
		c := buf[i]

		switch p.State {
		case StateMethod:
			if c == ' ' {
				ctx.Method = Span{Start: p.startOffset, End: i}
				p.State = StatePath
				p.startOffset = i + 1
			}
		case StatePath:
			if c == ' ' {
				ctx.Path = Span{Start: p.startOffset, End: i}
				p.State = StateVersion
				p.startOffset = i + 1
			}
		case StateVersion:
			if c == '\n' {
				if i > 0 && buf[i-1] == '\r' {
					ctx.Version = Span{Start: p.startOffset, End: i - 1}
				} else {
					ctx.Version = Span{Start: p.startOffset, End: i}
				}
				
				// Set default keep-alive based on HTTP version
				verBytes := buf[ctx.Version.Start:ctx.Version.End]
				if bytes.Equal(verBytes, []byte("HTTP/1.1")) {
					ctx.KeepAlive = true
				} else {
					ctx.KeepAlive = false
				}
				
				p.State = StateHeaderKey
				p.startOffset = i + 1
			}
		case StateHeaderKey:
			if c == '\n' {
				// End of headers section.
				if ctx.ContentLength > 0 {
					if length-(i+1) < ctx.ContentLength {
						return i, ErrIncomplete
					}
					ctx.Body = Span{Start: i + 1, End: i + 1 + ctx.ContentLength}
					p.State = StateComplete
					return i + 1 + ctx.ContentLength, nil
				}
				
				p.State = StateComplete
				ctx.Body = Span{Start: i + 1, End: i + 1}
				return i + 1, nil
			} else if c == ':' {
				p.currentKey = Span{Start: p.startOffset, End: i}
				p.State = StateHeaderValue
				p.startOffset = i + 1

				// Fast-forward past spaces
				for p.startOffset < length && buf[p.startOffset] == ' ' {
					p.startOffset++
					i++
				}
			}
		case StateHeaderValue:
			if c == '\n' {
				var val Span
				if i > 0 && buf[i-1] == '\r' {
					val = Span{Start: p.startOffset, End: i - 1}
				} else {
					val = Span{Start: p.startOffset, End: i}
				}
				ctx.AddHeader(p.currentKey, val)
				
				// Check for explicit Connection header
				keyBytes := buf[p.currentKey.Start:p.currentKey.End]
				valBytes := buf[val.Start:val.End]
				if bytes.EqualFold(keyBytes, []byte("Connection")) {
					if bytes.EqualFold(valBytes, []byte("keep-alive")) {
						ctx.KeepAlive = true
					} else if bytes.EqualFold(valBytes, []byte("close")) {
						ctx.KeepAlive = false
					}
					// Check for "Upgrade" in Connection header
					if bytes.Contains(valBytes, []byte("Upgrade")) || bytes.Contains(valBytes, []byte("upgrade")) {
						ctx.IsWebSocket = true
					}
				} else if bytes.EqualFold(keyBytes, []byte("Upgrade")) {
					if bytes.EqualFold(valBytes, []byte("websocket")) {
						ctx.IsWebSocket = true
					}
				} else if bytes.EqualFold(keyBytes, []byte("Sec-WebSocket-Key")) {
					ctx.WSKey = val
				} else if bytes.EqualFold(keyBytes, []byte("Content-Length")) {
					var n uint32
					for _, b := range valBytes {
						if b >= '0' && b <= '9' {
							n = n*10 + uint32(b-'0')
						}
					}
					ctx.ContentLength = n
				}

				p.State = StateHeaderKey
				p.startOffset = i + 1
			}
		}

		i++
	}

	if p.State != StateComplete {
		return i, ErrIncomplete
	}

	return i, nil
}
