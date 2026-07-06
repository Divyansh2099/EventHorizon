package parser

import (
	"bytes"
	"testing"
)

func TestParseWSFrame(t *testing.T) {
	// Construct a raw WebSocket text frame: FIN + Text Opcode (0x81), Masked (0x80) + Length 5 (0x05)
	// Masking key: 0x01, 0x02, 0x03, 0x04
	// Payload: "Hello" (unmasked: 0x48, 0x65, 0x6C, 0x6C, 0x6F)
	// Masked Payload: 
	// 'H' (0x48) ^ 0x01 = 0x49
	// 'e' (0x65) ^ 0x02 = 0x67
	// 'l' (0x6C) ^ 0x03 = 0x6F
	// 'l' (0x6C) ^ 0x04 = 0x68
	// 'o' (0x6F) ^ 0x01 = 0x6E
	
	rawFrame := []byte{
		0x81, 0x85, // FIN+Text, Masked+Len(5)
		0x01, 0x02, 0x03, 0x04, // Masking Key
		0x49, 0x67, 0x6F, 0x68, 0x6E, // Masked "Hello"
	}

	frame, consumed, err := ParseWSFrame(rawFrame)
	if err != nil {
		t.Fatalf("ParseWSFrame failed: %v", err)
	}

	if !frame.Fin {
		t.Errorf("Expected FIN to be true")
	}

	if frame.Opcode != 1 {
		t.Errorf("Expected Opcode 1 (Text), got %d", frame.Opcode)
	}

	if consumed != uint32(len(rawFrame)) {
		t.Errorf("Expected consumed %d, got %d", len(rawFrame), consumed)
	}

	if !bytes.Equal(frame.Payload, []byte("Hello")) {
		t.Errorf("Expected unmasked payload 'Hello', got '%s'", string(frame.Payload))
	}

	// Verify ZERO allocation by checking if the Payload slice points directly to the original buffer
	// We can mutate the Payload and verify the original buffer changes
	frame.Payload[0] = 'M'
	if rawFrame[6] != 'M' {
		t.Errorf("Expected in-place mutation to affect original buffer. Payload is allocating!")
	}
}
