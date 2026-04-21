package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"gotunnel/internal/config"
	"gotunnel/internal/protocol"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 10 * time.Second
)

type Client struct {
	cfg     config.AgentConfig
	targets map[string]string
}

type clientSession struct {
	conn *websocket.Conn

	writerMu sync.Mutex

	streamMu sync.RWMutex
	streams  map[uint32]net.Conn
}

func NewClient(cfg config.AgentConfig) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	targets := make(map[string]string, len(cfg.Targets))
	for _, target := range cfg.Targets {
		targets[target.Name] = target.LocalAddr
	}

	return &Client{
		cfg:     cfg,
		targets: targets,
	}, nil
}

func (c *Client) Start(ctx context.Context) error {
	backoff := time.Second

	for {
		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && errors.Is(err, errUnauthorized) {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

var errUnauthorized = errors.New("unauthorized")

func (c *Client) runOnce(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.cfg.RelayURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	configureWebSocket(conn)

	sess := &clientSession{
		conn:    conn,
		streams: make(map[uint32]net.Conn),
	}
	defer sess.close()
	stopHeartbeat := make(chan struct{})
	go sess.heartbeat(stopHeartbeat)
	defer close(stopHeartbeat)

	authPayload, _ := json.Marshal(protocol.AuthRequest{Token: c.cfg.AuthToken})
	if err := sess.write(protocol.Frame{Type: protocol.FrameAuth, Payload: authPayload}); err != nil {
		return err
	}

	authReply, err := readFrame(conn)
	if err != nil {
		return err
	}
	if authReply.Type == protocol.FrameError {
		return errUnauthorized
	}
	if authReply.Type != protocol.FrameAuthOK {
		return fmt.Errorf("unexpected auth reply: %v", authReply.Type)
	}

	targetNames := make([]string, 0, len(c.targets))
	for name := range c.targets {
		targetNames = append(targetNames, name)
	}
	registerPayload, _ := json.Marshal(protocol.RegisterRequest{Targets: targetNames})
	if err := sess.write(protocol.Frame{Type: protocol.FrameRegister, Payload: registerPayload}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		frame, err := readFrame(conn)
		if err != nil {
			return err
		}

		switch frame.Type {
		case protocol.FrameOpen:
			if err := c.handleOpen(sess, frame); err != nil {
				return err
			}
		case protocol.FrameData:
			sess.streamMu.RLock()
			targetConn := sess.streams[frame.StreamID]
			sess.streamMu.RUnlock()
			if targetConn != nil {
				if _, err := targetConn.Write(frame.Payload); err != nil {
					_ = targetConn.Close()
				}
			}
		case protocol.FrameClose:
			sess.streamMu.Lock()
			targetConn := sess.streams[frame.StreamID]
			delete(sess.streams, frame.StreamID)
			sess.streamMu.Unlock()
			if targetConn != nil {
				_ = targetConn.Close()
			}
		}
	}
}

func (c *Client) handleOpen(sess *clientSession, frame protocol.Frame) error {
	var req protocol.OpenRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil {
		return err
	}

	localAddr, ok := c.targets[req.Target]
	if !ok {
		payload, _ := json.Marshal(protocol.OpenResult{OK: false, Error: "unknown target"})
		return sess.write(protocol.Frame{Type: protocol.FrameOpenResult, StreamID: frame.StreamID, Payload: payload})
	}

	targetConn, err := net.DialTimeout("tcp", localAddr, 2*time.Second)
	if err != nil {
		payload, _ := json.Marshal(protocol.OpenResult{OK: false, Error: err.Error()})
		return sess.write(protocol.Frame{Type: protocol.FrameOpenResult, StreamID: frame.StreamID, Payload: payload})
	}

	sess.streamMu.Lock()
	sess.streams[frame.StreamID] = targetConn
	sess.streamMu.Unlock()

	payload, _ := json.Marshal(protocol.OpenResult{OK: true})
	if err := sess.write(protocol.Frame{Type: protocol.FrameOpenResult, StreamID: frame.StreamID, Payload: payload}); err != nil {
		_ = targetConn.Close()
		return err
	}

	go c.pumpLocal(sess, frame.StreamID, targetConn)
	return nil
}

func (c *Client) pumpLocal(sess *clientSession, streamID uint32, targetConn net.Conn) {
	defer func() {
		sess.streamMu.Lock()
		delete(sess.streams, streamID)
		sess.streamMu.Unlock()
		_ = targetConn.Close()
		_ = sess.write(protocol.Frame{Type: protocol.FrameClose, StreamID: streamID})
	}()

	buf := make([]byte, 32*1024)
	for {
		n, err := targetConn.Read(buf)
		if n > 0 {
			if writeErr := sess.write(protocol.Frame{Type: protocol.FrameData, StreamID: streamID, Payload: append([]byte(nil), buf[:n]...)}); writeErr != nil {
				return
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
}

func (s *clientSession) write(frame protocol.Frame) error {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	return writeFrame(s.conn, frame)
}

func (s *clientSession) close() {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	for id, conn := range s.streams {
		_ = conn.Close()
		delete(s.streams, id)
	}
}

func (s *clientSession) heartbeat(stop <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.writerMu.Lock()
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := s.conn.WriteMessage(websocket.PingMessage, nil)
			s.writerMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func readFrame(conn *websocket.Conn) (protocol.Frame, error) {
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return protocol.Frame{}, err
	}
	return protocol.DecodeFrame(raw)
}

func writeFrame(conn *websocket.Conn, frame protocol.Frame) error {
	raw, err := protocol.EncodeFrame(frame)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteMessage(websocket.BinaryMessage, raw)
}

func configureWebSocket(conn *websocket.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
}
