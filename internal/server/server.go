package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"openproxy/internal/config"
	"openproxy/internal/protocol"
)

type Server struct {
	Config       *config.ServerConfig
	tunnelMgr    *TunnelManager
	listener     net.Listener
	running      bool
	mu           sync.Mutex
	pendingConns map[string]PendingConn
	pendingMu    sync.Mutex
}

type PendingConn struct {
	Conn   net.Conn
	Tunnel *Tunnel
}

type TunnelManager struct {
	tunnels map[string]*Tunnel
	mu      sync.RWMutex
}

type Tunnel struct {
	Name        string
	Protocol    string
	RemotePort  int
	Listener    net.Listener
	ControlConn net.Conn
	ActiveConns int64
}

func NewServer(cfg *config.ServerConfig) *Server {
	return &Server{
		Config:       cfg,
		tunnelMgr:    &TunnelManager{tunnels: make(map[string]*Tunnel)},
		pendingConns: make(map[string]PendingConn),
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.Config.ControlPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.running = true
	log.Printf("Server listening on control port %d", s.Config.ControlPort)

	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Accept error: %v", err)
			}
			continue
		}
		go s.handleControlConnection(conn)
	}
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	// TODO: Cleanup tunnels
}

func (s *Server) handleControlConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New control connection from %s", conn.RemoteAddr())

	// 1. Auth
	if err := s.handshake(conn); err != nil {
		log.Printf("Handshake failed: %v", err)
		return
	}

	// 2. Loop for commands (Register Tunnel, Ping, etc.)
	decoder := json.NewDecoder(conn)
	for {
		var msg protocol.Message
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Printf("Control read error: %v", err)
			}
			return
		}

		switch msg.Type {
		case protocol.TypeRegTunnel:
			var req protocol.RegTunnelRequest
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				log.Printf("Invalid reg payload: %v", err)
				continue
			}
			s.handleRegisterTunnel(conn, req)
		case protocol.TypePing:
			protocol.WriteMessage(conn, protocol.TypePong, nil)
		case protocol.TypeProxyData:
			var req protocol.ProxyDataRequest
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				log.Printf("Invalid proxy data payload: %v", err)
				return
			}
			s.handleProxyData(conn, req)
			return // This connection is now used for data, stop control loop
		}
	}
}

func (s *Server) handleProxyData(clientConn net.Conn, req protocol.ProxyDataRequest) {
	s.pendingMu.Lock()
	pc, ok := s.pendingConns[req.ConnID]
	if ok {
		delete(s.pendingConns, req.ConnID)
	}
	s.pendingMu.Unlock()

	if !ok {
		log.Printf("Pending connection %s not found", req.ConnID)
		return
	}
	publicConn := pc.Conn
	tunnel := pc.Tunnel

	defer func() {
		publicConn.Close()
		atomic.AddInt64(&tunnel.ActiveConns, -1)
	}()

	// Bridge connections
	log.Printf("Bridging connection %s", req.ConnID)
	go io.Copy(publicConn, clientConn)
	io.Copy(clientConn, publicConn)
}

func (s *Server) handshake(conn net.Conn) error {
	var msg protocol.Message
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		return err
	}

	if msg.Type != protocol.TypeAuth {
		return fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	var req protocol.AuthRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return err
	}

	if req.Token != s.Config.Token {
		protocol.WriteMessage(conn, protocol.TypeAuthResp, protocol.AuthResponse{Success: false, Error: "Invalid Token"})
		return fmt.Errorf("invalid token")
	}

	return protocol.WriteMessage(conn, protocol.TypeAuthResp, protocol.AuthResponse{Success: true})
}

func (s *Server) handleRegisterTunnel(controlConn net.Conn, req protocol.RegTunnelRequest) {
	// Validate Port Range
	if s.Config.PortRange != "" {
		parts := strings.Split(s.Config.PortRange, "-")
		if len(parts) == 2 {
			min, _ := strconv.Atoi(parts[0])
			max, _ := strconv.Atoi(parts[1])
			if req.RemotePort < min || req.RemotePort > max {
				resp := protocol.RegTunnelResponse{
					Name:       req.Name,
					Success:    false,
					Error:      fmt.Sprintf("Port %d is out of allowed range %s", req.RemotePort, s.Config.PortRange),
				}
				protocol.WriteMessage(controlConn, protocol.TypeRegResp, resp)
				return
			}
		}
	}

	// Start listener for this tunnel
	addr := fmt.Sprintf(":%d", req.RemotePort)
	ln, err := net.Listen("tcp", addr)
	
	resp := protocol.RegTunnelResponse{
		Name:       req.Name,
		RemotePort: req.RemotePort,
		Success:    true,
	}

	if err != nil {
		resp.Success = false
		resp.Error = err.Error()
		protocol.WriteMessage(controlConn, protocol.TypeRegResp, resp)
		return
	}

	t := &Tunnel{
		Name:        req.Name,
		Protocol:    req.Protocol,
		RemotePort:  req.RemotePort,
		Listener:    ln,
		ControlConn: controlConn,
	}
	s.tunnelMgr.mu.Lock()
	s.tunnelMgr.tunnels[req.Name] = t
	s.tunnelMgr.mu.Unlock()

	protocol.WriteMessage(controlConn, protocol.TypeRegResp, resp)
	log.Printf("Tunnel %s registered on port %d", req.Name, req.RemotePort)

	// Accept public connections for this tunnel
	go s.acceptTunnelConnections(t)
}

func (s *Server) acceptTunnelConnections(t *Tunnel) {
	defer t.Listener.Close()
	for {
		publicConn, err := t.Listener.Accept()
		if err != nil {
			log.Printf("Tunnel %s accept error: %v", t.Name, err)
			return
		}
		
		go s.handlePublicConnection(t, publicConn)
	}
}

func (s *Server) handlePublicConnection(t *Tunnel, publicConn net.Conn) {
	atomic.AddInt64(&t.ActiveConns, 1)
	
	connID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Store pending connection
	s.pendingMu.Lock()
	s.pendingConns[connID] = PendingConn{Conn: publicConn, Tunnel: t}
	s.pendingMu.Unlock()
	
	// Notify client to open a new connection for data
	req := protocol.NewConnRequest{
		ConnID:     connID,
		TunnelName: t.Name,
	}

	if err := protocol.WriteMessage(t.ControlConn, protocol.TypeNewConn, req); err != nil {
		log.Printf("Failed to notify client of new connection: %v", err)
		s.pendingMu.Lock()
		delete(s.pendingConns, connID)
		s.pendingMu.Unlock()
		publicConn.Close()
		atomic.AddInt64(&t.ActiveConns, -1)
		return
	}
	
	log.Printf("New public connection on %s (ID: %s), waiting for client...", t.Name, connID)
	
	// Set a timeout?
	time.AfterFunc(10*time.Second, func() {
		s.pendingMu.Lock()
		if pc, ok := s.pendingConns[connID]; ok {
			pc.Conn.Close()
			delete(s.pendingConns, connID)
			atomic.AddInt64(&pc.Tunnel.ActiveConns, -1)
			log.Printf("Connection %s timed out waiting for client", connID)
		}
		s.pendingMu.Unlock()
	})
}

func (s *Server) GetStatus() interface{} {
	s.tunnelMgr.mu.RLock()
	defer s.tunnelMgr.mu.RUnlock()
	
	var tunnels []map[string]interface{}
	for _, t := range s.tunnelMgr.tunnels {
		tunnels = append(tunnels, map[string]interface{}{
			"name": t.Name,
			"protocol": t.Protocol,
			"remote_port": t.RemotePort,
			"active_conns": atomic.LoadInt64(&t.ActiveConns),
		})
	}
	return map[string]interface{}{
		"mode": "server",
		"control_port": s.Config.ControlPort,
		"tunnels_count": len(tunnels),
		"tunnels": tunnels,
	}
}

func (s *Server) AddTunnel(t config.Tunnel) error {
	return fmt.Errorf("server mode does not support adding tunnels manually")
}

func (s *Server) RemoveTunnel(name string) error {
	return fmt.Errorf("server mode does not support removing tunnels manually")
}
