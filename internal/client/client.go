package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"openproxy/internal/config"
	"openproxy/internal/protocol"
)

type Client struct {
	Config      *config.ClientConfig
	controlConn net.Conn
	mu          sync.Mutex
	connected   bool
}

func NewClient(cfg *config.ClientConfig) *Client {
	return &Client{Config: cfg}
}

func (c *Client) Start() error {
	// 1. Connect to Server
	conn, err := net.Dial("tcp", c.Config.ServerAddr)
	if err != nil {
		return err
	}
	// Do not defer conn.Close() immediately, handle it in cleanup
	log.Printf("Connected to server %s", c.Config.ServerAddr)

	c.mu.Lock()
	c.controlConn = conn
	c.connected = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.controlConn = nil
		c.connected = false
		c.mu.Unlock()
		conn.Close()
	}()

	// 2. Auth
	if err := c.authenticate(conn); err != nil {
		return err
	}
	log.Println("Authentication successful")

	// 3. Register Tunnels
	for _, t := range c.Config.Tunnels {
		if err := c.registerTunnel(conn, t); err != nil {
			log.Printf("Failed to register tunnel %s: %v", t.Name, err)
			continue // Or return error?
		}
	}

	// 4. Heartbeat & Command Loop
	go c.heartbeat(conn)

	decoder := json.NewDecoder(conn)
	for {
		var msg protocol.Message
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v", err)
			}
			return err
		}

		switch msg.Type {
		case protocol.TypeNewConn:
			var req protocol.NewConnRequest
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				log.Printf("Invalid new_conn payload: %v", err)
				continue
			}
			go c.handleNewConn(req)
		case protocol.TypePong:
			// log.Println("Pong received")
		}
	}
}

func (c *Client) authenticate(conn net.Conn) error {
	req := protocol.AuthRequest{Token: c.Config.Token}
	if err := protocol.WriteMessage(conn, protocol.TypeAuth, req); err != nil {
		return err
	}

	var msg protocol.Message
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		return err
	}

	if msg.Type != protocol.TypeAuthResp {
		return fmt.Errorf("unexpected auth response type: %s", msg.Type)
	}

	var resp protocol.AuthResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("auth failed: %s", resp.Error)
	}
	return nil
}

func (c *Client) registerTunnel(conn net.Conn, t config.Tunnel) error {
	req := protocol.RegTunnelRequest{
		Name:       t.Name,
		Protocol:   t.Protocol,
		RemotePort: t.RemotePort,
	}

	if err := protocol.WriteMessage(conn, protocol.TypeRegTunnel, req); err != nil {
		return err
	}

	// We expect a synchronous response for each registration to ensure it worked
	var msg protocol.Message
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		return err
	}

	if msg.Type != protocol.TypeRegResp {
		return fmt.Errorf("unexpected reg response type: %s", msg.Type)
	}

	var resp protocol.RegTunnelResponse
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Error)
	}
	
	log.Printf("Tunnel %s registered successfully on port %d", t.Name, resp.RemotePort)
	return nil
}

func (c *Client) heartbeat(conn net.Conn) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			currentConn := c.controlConn
			c.mu.Unlock()
			
			if currentConn == nil {
				return
			}

			if err := protocol.WriteMessage(currentConn, protocol.TypePing, nil); err != nil {
				log.Printf("Heartbeat failed: %v", err)
				return
			}
		}
	}
}

func (c *Client) handleNewConn(req protocol.NewConnRequest) {
	// Find local address for this tunnel
	var localAddr string
	// We need to look up in the config (which might have changed dynamically)
	// or we should pass the updated config reference.
	// Since c.Config is a pointer, and Web UI updates the content of that pointer, we should see the new tunnels here!
	for _, t := range c.Config.Tunnels {
		if t.Name == req.TunnelName {
			localAddr = t.LocalAddr
			break
		}
	}

	if localAddr == "" {
		log.Printf("Unknown tunnel name: %s", req.TunnelName)
		return
	}

	// 1. Dial Local Service
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		log.Printf("Failed to dial local service %s: %v", localAddr, err)
		return
	}
	defer localConn.Close()

	// 2. Dial Server Control Port (Data connection)
	serverConn, err := net.Dial("tcp", c.Config.ServerAddr)
	if err != nil {
		log.Printf("Failed to dial server data conn: %v", err)
		return
	}
	defer serverConn.Close()

	// 3. Handshake as Proxy Data
	authReq := protocol.AuthRequest{Token: c.Config.Token}
	if err := protocol.WriteMessage(serverConn, protocol.TypeAuth, authReq); err != nil {
		log.Printf("Data conn auth write failed: %v", err)
		return
	}
	
	// Read Auth Resp
	var msg protocol.Message
	if err := json.NewDecoder(serverConn).Decode(&msg); err != nil {
		log.Printf("Data conn auth read failed: %v", err)
		return
	}

	proxyReq := protocol.ProxyDataRequest{ConnID: req.ConnID}
	if err := protocol.WriteMessage(serverConn, protocol.TypeProxyData, proxyReq); err != nil {
		log.Printf("Data conn proxy req failed: %v", err)
		return
	}

	// 4. Bridge
	go io.Copy(localConn, serverConn)
	io.Copy(serverConn, localConn)
}

func (c *Client) GetStatus() interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]interface{}{
		"mode": "client",
		"server_addr": c.Config.ServerAddr,
		"connected": c.connected,
		"tunnels": c.Config.Tunnels,
	}
}

func (c *Client) AddTunnel(t config.Tunnel) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check duplicate name
	for _, existing := range c.Config.Tunnels {
		if existing.Name == t.Name {
			return fmt.Errorf("tunnel name %s already exists", t.Name)
		}
	}

	// If connected, register immediately
	if c.connected && c.controlConn != nil {
		if err := c.registerTunnel(c.controlConn, t); err != nil {
			return err
		}
	}
	
	// Note: We don't update c.Config.Tunnels here because the Web handler does it.
	return nil
}

func (c *Client) RemoveTunnel(name string) error {
	// Not fully implemented on protocol level (can't unregister on server yet),
	// but we can allow removing from client config so it doesn't reconnect.
	return nil
}