package protocol

import (
	"encoding/json"
	"io"
)

type MessageType string

const (
	TypeAuth      MessageType = "auth"
	TypeAuthResp  MessageType = "auth_resp"
	TypeRegTunnel MessageType = "reg_tunnel"
	TypeRegResp   MessageType = "reg_resp"
	TypeNewConn   MessageType = "new_conn"
	TypeProxyData MessageType = "proxy_data"
	TypePing      MessageType = "ping"
	TypePong      MessageType = "pong"
)

type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type AuthRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type RegTunnelRequest struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	RemotePort int    `json:"remote_port"`
}

type RegTunnelResponse struct {
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	RemotePort int    `json:"remote_port"` // Assigned port
	Error      string `json:"error,omitempty"`
}

type NewConnRequest struct {
	ConnID     string `json:"conn_id"`
	TunnelName string `json:"tunnel_name"`
}

type ProxyDataRequest struct {
	ConnID string `json:"conn_id"`
}

// Helper to read JSON message from connection
func ReadMessage(r io.Reader) (*Message, error) {
	var msg Message
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Helper to write JSON message to connection
func WriteMessage(w io.Writer, msgType MessageType, payload interface{}) error {
	pBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	msg := Message{
		Type:    msgType,
		Payload: pBytes,
	}
	
	encoder := json.NewEncoder(w)
	return encoder.Encode(msg)
}
