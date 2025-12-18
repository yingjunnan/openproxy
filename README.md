# OpenProxy

[English](README.md) | [中文](README_CN.md)

A lightweight, high-performance Intranet Penetration Tool written in Go. OpenProxy allows you to expose local servers behind NATs and firewalls to the public internet over secure tunnels.

![Dashboard Preview](https://via.placeholder.com/800x400?text=OpenProxy+Dashboard+Preview)

## Features

- **Dual Mode**: Single binary acts as both Server and Client.
- **Modern Dashboard**: Built-in Vue 3 + Bootstrap 5 web interface for real-time monitoring.
- **Secure**: Token-based authentication and server-side port range restrictions.
- **Real-time Metrics**: Live traffic activity charts and connection tracking.
- **Hot-Pluggable Tunnels**: Clients can add/remove tunnels dynamically via the Web UI without restarting.
- **Cross-Platform**: Compiles to a single binary for Windows, Linux, and macOS.

## Getting Started

### Prerequisites

- Go 1.23+ (for building from source)

### Installation

1.  **Clone the repository**
    ```bash
    git clone https://github.com/yingjunnan/openproxy.git
    cd openproxy
    ```

2.  **Build**
    ```bash
    go build -o openproxy cmd/openproxy/main.go
    # On Windows: go build -o openproxy.exe cmd/openproxy/main.go
    ```

## Usage

### 1. Server Setup

1.  Create a configuration file `config.yaml` based on `config.example.yaml`.
2.  Set `mode: server`.
3.  Configure `server` section (control port, token).
4.  Run:
    ```bash
    ./openproxy -c config.yaml
    ```
5.  Access Dashboard: `http://localhost:8080` (Default creds: admin/password)

### 2. Client Setup

1.  Create a configuration file (e.g., `client.yaml`) based on `config.example.yaml`.
2.  Set `mode: client`.
3.  Configure `client` section (server address, token).
4.  Run:
    ```bash
    ./openproxy -c client.yaml
    ```
5.  Access Dashboard: `http://localhost:8081` (Make sure to change web port if running on same machine)

## Configuration

See [config.example.yaml](config.example.yaml) for a fully commented configuration file.

```yaml
mode: client
web:
  port: 8081
  username: admin
  password: secret
client:
  server_addr: "x.x.x.x:7000"
  token: "secret-token"
  tunnels:
    - name: web
      local_addr: "127.0.0.1:80"
      remote_port: 10080
```

## Development

The project structure is organized as follows:

- `cmd/openproxy`: Main entry point.
- `internal/server`: Server-side logic (Control listener, Connection manager).
- `internal/client`: Client-side logic (Tunnel registration, Traffic bridging).
- `internal/protocol`: Custom TCP protocol definitions.
- `internal/web`: Web server and API handlers.
- `internal/web/static`: Frontend assets (Vue 3 app).

## License

MIT
