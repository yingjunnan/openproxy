package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode   string       `yaml:"mode" json:"mode"` // "server" or "client"
	Web    WebConfig    `yaml:"web" json:"web"`
	Server ServerConfig `yaml:"server" json:"server"`
	Client ClientConfig `yaml:"client" json:"client"`
}

type WebConfig struct {
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type ServerConfig struct {
	ControlPort int    `yaml:"control_port" json:"control_port"`
	Token       string `yaml:"token" json:"token"`
	PortRange   string `yaml:"port_range" json:"port_range"` // e.g. "10000-20000"
}

type ClientConfig struct {
	ServerAddr  string   `yaml:"server_addr" json:"server_addr"`
	Token       string   `yaml:"token" json:"token"`
	Tunnels     []Tunnel `yaml:"tunnels" json:"tunnels"`
}

type Tunnel struct {
	Name       string `yaml:"name" json:"name"`
	Protocol   string `yaml:"protocol" json:"protocol"` // tcp, http, etc.
	LocalAddr  string `yaml:"local_addr" json:"local_addr"`
	RemotePort int    `yaml:"remote_port" json:"remote_port"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) Validate() error {
	if c.Mode != "server" && c.Mode != "client" {
		return fmt.Errorf("invalid mode: %s", c.Mode)
	}
	return nil
}