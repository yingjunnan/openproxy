# OpenProxy

[English](README.md) | [中文](README_CN.md)

OpenProxy 是一个使用 Go 语言编写的轻量级高性能内网穿透工具。它可以帮助你将位于 NAT 或防火墙后的本地服务暴露到公网。

![Dashboard Preview](https://via.placeholder.com/800x400?text=OpenProxy+Dashboard+Preview)

## 功能特性

- **双模式运行**：单个二进制文件通过配置可作为服务端或客户端运行。
- **现代化仪表盘**：内置 Vue 3 + Bootstrap 5 Web 界面，支持实时监控。
- **安全可靠**：基于 Token 的身份验证和服务端端口范围限制。
- **实时指标**：提供实时的流量活动图表和连接追踪。
- **热插拔隧道**：客户端可通过 Web UI 动态添加/删除隧道，无需重启服务。
- **跨平台**：可编译为 Windows, Linux, macOS 单一可执行文件。

## 快速开始

### 环境要求

- Go 1.23+ (如果需要从源码编译)

### 安装

1.  **克隆仓库**
    ```bash
    git clone https://github.com/yingjunnan/openproxy.git
    cd openproxy
    ```

2.  **编译**
    ```bash
    go build -o openproxy cmd/openproxy/main.go
    # Windows 用户: go build -o openproxy.exe cmd/openproxy/main.go
    ```

## 使用指南

### 1. 服务端部署 (Server)

1.  基于 `config.example.yaml` 创建配置文件 `config.yaml`。
2.  设置 `mode: server`。
3.  配置 `server` 部分（控制端口、Token 等）。
4.  运行：
    ```bash
    ./openproxy -c config.yaml
    ```
5.  访问仪表盘：`http://localhost:8080` (默认账号: admin/password)

### 2. 客户端部署 (Client)

1.  基于 `config.example.yaml` 创建配置文件（如 `client.yaml`）。
2.  设置 `mode: client`。
3.  配置 `client` 部分（服务器地址、Token）。
4.  运行：
    ```bash
    ./openproxy -c client.yaml
    ```
5.  访问仪表盘：`http://localhost:8081` (如果在同一台机器运行，请确保修改 web 端口)

## 配置说明

请参考 [config.example.yaml](config.example.yaml) 查看包含详细注释的配置模版。

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

## 开发架构

项目目录结构如下：

- `cmd/openproxy`: 程序入口。
- `internal/server`: 服务端逻辑（监听控制端口、连接管理）。
- `internal/client`: 客户端逻辑（隧道注册、流量桥接）。
- `internal/protocol`: 自定义 TCP 协议定义。
- `internal/web`: Web 服务器及 API 处理器。
- `internal/web/static`: 前端资源 (Vue 3 应用)。

## 许可证

MIT
