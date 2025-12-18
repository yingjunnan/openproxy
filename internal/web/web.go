package web

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"openproxy/internal/config"
)

//go:embed static/*
var staticFiles embed.FS

type StatusProvider interface {
	GetStatus() interface{}
	AddTunnel(t config.Tunnel) error
	RemoveTunnel(name string) error
}

type Handler struct {
	Config     *config.Config
	ConfigPath string
	Provider   StatusProvider
}

func Start(cfg *config.Config, configPath string, provider StatusProvider) error {
	h := &Handler{
		Config:     cfg,
		ConfigPath: configPath,
		Provider:   provider,
	}

	// Setup FS for static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	
	// API Endpoints
	mux.HandleFunc("/api/config", h.handleConfig)
	mux.HandleFunc("/api/status", h.handleStatus)
	mux.HandleFunc("/api/tunnels", h.handleTunnels)
	
	// Static Files
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// Middleware for Auth
	handler := h.basicAuth(mux)

	addr := fmt.Sprintf(":%d", cfg.Web.Port)
	log.Printf("Web UI listening on %s", addr)
	return http.ListenAndServe(addr, handler)
}

func (h *Handler) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(h.Config.Web.Username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(h.Config.Web.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := h.Provider.GetStatus()
	json.NewEncoder(w).Encode(status)
}

func (h *Handler) handleTunnels(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var t config.Tunnel
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.Provider.AddTunnel(t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Also update config file
		h.Config.Client.Tunnels = append(h.Config.Client.Tunnels, t)
		config.SaveConfig(h.ConfigPath, h.Config)
		w.WriteHeader(http.StatusOK)
		return
	}
	
	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if err := h.Provider.RemoveTunnel(name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Update config file
		for i, t := range h.Config.Client.Tunnels {
			if t.Name == name {
				h.Config.Client.Tunnels = append(h.Config.Client.Tunnels[:i], h.Config.Client.Tunnels[i+1:]...)
				break
			}
		}
		config.SaveConfig(h.ConfigPath, h.Config)
		w.WriteHeader(http.StatusOK)
		return
	}
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(h.Config)
		return
	}

	if r.Method == http.MethodPost {
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update in-memory config
		// Note: This won't reload the running server/client logic automatically in this simple version.
		// A restart is required.
		*h.Config = newCfg

		// Save to file
		if err := config.SaveConfig(h.ConfigPath, &newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "saved", "message": "Configuration saved. Please restart the application to apply changes."})
	}
}
