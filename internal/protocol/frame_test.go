package protocol_test

import (
	"bytes"
	"testing"

	"gotunnel/internal/protocol"
)

func TestEncodeDecodeFrameRoundTrip(t *testing.T) {
	original := protocol.Frame{
		Type:     protocol.FrameData,
		StreamID: 42,
		Payload:  []byte("hello tunnel"),
	}

	encoded, err := protocol.EncodeFrame(original)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := protocol.DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Fatalf("unexpected frame type: got %v want %v", decoded.Type, original.Type)
	}
	if decoded.StreamID != original.StreamID {
		t.Fatalf("unexpected stream id: got %d want %d", decoded.StreamID, original.StreamID)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Fatalf("unexpected payload: got %q want %q", decoded.Payload, original.Payload)
	}
}

func TestDecodeFrameRejectsShortPayload(t *testing.T) {
	if _, err := protocol.DecodeFrame([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected short frame to fail decode")
	}
}
