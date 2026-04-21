package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"gotunnel/internal/config"
	"gotunnel/internal/protocol"
)

const controlPath = "/connect"
const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 10 * time.Second
)

type Server struct {
	cfg             config.RelayConfig
	controlListener net.Listener
	publicListeners map[string]net.Listener

	httpServer *http.Server

	mu      sync.RWMutex
	session *session
}

type session struct {
	conn *websocket.Conn

	writerMu sync.Mutex
	targets  map[string]struct{}

	streamMu     sync.RWMutex
	streams      map[uint32]net.Conn
	openResults  map[uint32]chan protocol.OpenResult
	nextStreamID atomic.Uint32
}

func NewServer(cfg config.RelayConfig) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	controlListener, err := net.Listen("tcp", cfg.ControlAddr)
	if err != nil {
		return nil, fmt.Errorf("listen control: %w", err)
	}

	publicListeners := make(map[string]net.Listener, len(cfg.Ports))
	for _, mapping := range cfg.Ports {
		ln, err := net.Listen("tcp", mapping.ListenAddr)
		if err != nil {
			_ = controlListener.Close()
			for _, open := range publicListeners {
				_ = open.Close()
			}
			return nil, fmt.Errorf("listen public %s: %w", mapping.Name, err)
		}
		publicListeners[mapping.Name] = ln
	}

	s := &Server{
		cfg:             cfg,
		controlListener: controlListener,
		publicListeners: publicListeners,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(controlPath, s.handleControl)
	s.httpServer = &http.Server{Handler: mux}
	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		<-ctx.Done()
		_ = s.httpServer.Close()
		_ = s.controlListener.Close()
		for _, ln := range s.publicListeners {
			_ = ln.Close()
		}
		s.closeSession()
	}()

	for _, mapping := range s.cfg.Ports {
		ln := s.publicListeners[mapping.Name]
		go s.acceptPublic(ctx, mapping.Name, ln)
	}

	go func() {
		var err error
		if s.cfg.TLSCertFile != "" {
			err = s.httpServer.ServeTLS(s.controlListener, s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		} else {
			err = s.httpServer.Serve(s.controlListener)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) ControlURL() string {
	return "ws://" + s.controlListener.Addr().String() + controlPath
}

func (s *Server) ListenerAddress(name string) string {
	if ln, ok := s.publicListeners[name]; ok {
		return ln.Addr().String()
	}
	return ""
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	configureWebSocket(conn)

	sess, err := s.authenticate(conn)
	if err != nil {
		_ = conn.Close()
		return
	}

	s.replaceSession(sess)
	stopHeartbeat := make(chan struct{})
	go sess.heartbeat(stopHeartbeat)
	defer func() {
		close(stopHeartbeat)
		s.clearSession(sess)
		sess.close()
	}()

	if err := s.readLoop(sess); err != nil && !errors.Is(err, net.ErrClosed) {
		return
	}
}

func (s *Server) authenticate(conn *websocket.Conn) (*session, error) {
	authFrame, err := readFrame(conn)
	if err != nil {
		return nil, err
	}
	if authFrame.Type != protocol.FrameAuth {
		return nil, errors.New("expected auth frame")
	}

	var auth protocol.AuthRequest
	if err := json.Unmarshal(authFrame.Payload, &auth); err != nil {
		return nil, err
	}
	if !s.isAllowedToken(auth.Token) {
		_ = writeFrame(conn, protocol.Frame{Type: protocol.FrameError, Payload: []byte("unauthorized")})
		return nil, errors.New("unauthorized")
	}

	if err := writeFrame(conn, protocol.Frame{Type: protocol.FrameAuthOK}); err != nil {
		return nil, err
	}

	registerFrame, err := readFrame(conn)
	if err != nil {
		return nil, err
	}
	if registerFrame.Type != protocol.FrameRegister {
		return nil, errors.New("expected register frame")
	}

	var register protocol.RegisterRequest
	if err := json.Unmarshal(registerFrame.Payload, &register); err != nil {
		return nil, err
	}

	targets := make(map[string]struct{}, len(register.Targets))
	for _, target := range register.Targets {
		targets[target] = struct{}{}
	}

	return &session{
		conn:        conn,
		targets:     targets,
		streams:     make(map[uint32]net.Conn),
		openResults: make(map[uint32]chan protocol.OpenResult),
	}, nil
}

func (s *Server) readLoop(sess *session) error {
	for {
		frame, err := readFrame(sess.conn)
		if err != nil {
			return err
		}

		switch frame.Type {
		case protocol.FrameOpenResult:
			var result protocol.OpenResult
			if err := json.Unmarshal(frame.Payload, &result); err != nil {
				return err
			}
			sess.streamMu.Lock()
			ch := sess.openResults[frame.StreamID]
			delete(sess.openResults, frame.StreamID)
			sess.streamMu.Unlock()
			if ch != nil {
				ch <- result
			}
		case protocol.FrameData:
			sess.streamMu.RLock()
			targetConn := sess.streams[frame.StreamID]
			sess.streamMu.RUnlock()
			if targetConn != nil {
				if _, err := targetConn.Write(frame.Payload); err != nil {
					_ = targetConn.Close()
					sess.streamMu.Lock()
					delete(sess.streams, frame.StreamID)
					delete(sess.openResults, frame.StreamID)
					sess.streamMu.Unlock()
					_ = sess.write(protocol.Frame{Type: protocol.FrameClose, StreamID: frame.StreamID})
				}
			}
		case protocol.FrameClose:
			sess.streamMu.Lock()
			targetConn := sess.streams[frame.StreamID]
			delete(sess.streams, frame.StreamID)
			delete(sess.openResults, frame.StreamID)
			sess.streamMu.Unlock()
			if targetConn != nil {
				_ = targetConn.Close()
			}
		}
	}
}

func (s *Server) acceptPublic(ctx context.Context, name string, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go s.handlePublicConn(ctx, name, conn)
	}
}

func (s *Server) handlePublicConn(ctx context.Context, name string, publicConn net.Conn) {
	sess := s.waitForSessionFor(ctx, name, 3*time.Second)
	if sess == nil {
		_ = publicConn.Close()
		return
	}

	streamID := sess.nextStreamID.Add(1)
	resultCh := make(chan protocol.OpenResult, 1)

	sess.streamMu.Lock()
	sess.streams[streamID] = publicConn
	sess.openResults[streamID] = resultCh
	sess.streamMu.Unlock()

	defer func() {
		_ = sess.write(protocol.Frame{Type: protocol.FrameClose, StreamID: streamID})
		sess.streamMu.Lock()
		delete(sess.streams, streamID)
		delete(sess.openResults, streamID)
		sess.streamMu.Unlock()
		_ = publicConn.Close()
	}()

	payload, _ := json.Marshal(protocol.OpenRequest{Target: name})
	if err := sess.write(protocol.Frame{Type: protocol.FrameOpen, StreamID: streamID, Payload: payload}); err != nil {
		return
	}

	select {
	case result := <-resultCh:
		if !result.OK {
			return
		}
	case <-time.After(3 * time.Second):
		return
	case <-ctx.Done():
		return
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := publicConn.Read(buf)
		if n > 0 {
			if writeErr := sess.write(protocol.Frame{Type: protocol.FrameData, StreamID: streamID, Payload: append([]byte(nil), buf[:n]...)}); writeErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Server) isAllowedToken(token string) bool {
	for _, candidate := range s.cfg.AuthTokens {
		if token == candidate {
			return true
		}
	}
	return false
}

func (s *Server) replaceSession(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.close()
	}
	s.session = sess
}

func (s *Server) clearSession(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == sess {
		s.session = nil
	}
}

func (s *Server) currentSessionFor(name string) *session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.session == nil {
		return nil
	}
	if _, ok := s.session.targets[name]; !ok {
		return nil
	}
	return s.session
}

func (s *Server) waitForSessionFor(ctx context.Context, name string, timeout time.Duration) *session {
	deadline := time.Now().Add(timeout)
	for {
		if sess := s.currentSessionFor(name); sess != nil {
			return sess
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (s *Server) closeSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.close()
		s.session = nil
	}
}

func (s *session) write(frame protocol.Frame) error {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	return writeFrame(s.conn, frame)
}

func (s *session) close() {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	for id, conn := range s.streams {
		_ = conn.Close()
		delete(s.streams, id)
	}
	_ = s.conn.Close()
}

func (s *session) heartbeat(stop <-chan struct{}) {
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
