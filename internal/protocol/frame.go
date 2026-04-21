package protocol

import (
	"encoding/binary"
	"errors"
)

type FrameType byte

const (
	FrameAuth FrameType = iota + 1
	FrameAuthOK
	FrameRegister
	FrameOpen
	FrameOpenResult
	FrameData
	FrameClose
	FramePing
	FramePong
	FrameError
)

type Frame struct {
	Type     FrameType
	StreamID uint32
	Payload  []byte
}

func EncodeFrame(frame Frame) ([]byte, error) {
	buf := make([]byte, 5+len(frame.Payload))
	buf[0] = byte(frame.Type)
	binary.BigEndian.PutUint32(buf[1:5], frame.StreamID)
	copy(buf[5:], frame.Payload)
	return buf, nil
}

func DecodeFrame(raw []byte) (Frame, error) {
	if len(raw) < 5 {
		return Frame{}, errors.New("frame too short")
	}

	frame := Frame{
		Type:     FrameType(raw[0]),
		StreamID: binary.BigEndian.Uint32(raw[1:5]),
		Payload:  make([]byte, len(raw[5:])),
	}
	copy(frame.Payload, raw[5:])
	return frame, nil
}
