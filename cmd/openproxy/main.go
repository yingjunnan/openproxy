package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openproxy/internal/client"
	"openproxy/internal/config"
	"openproxy/internal/server"
	"openproxy/internal/web"
)

func main() {
	configPath := flag.String("c", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	var provider web.StatusProvider

	if cfg.Mode == "server" {
		srv := server.NewServer(&cfg.Server)
		provider = srv
		
		// Start Server in goroutine
		go func() {
			if err := srv.Start(); err != nil {
				log.Fatalf("Server failed: %v", err)
			}
		}()
	} else {
		cli := client.NewClient(&cfg.Client)
		provider = cli
		
		// Start Client in goroutine
		go func() {
			// Basic reconnect loop
			for {
				if err := cli.Start(); err != nil {
					log.Printf("Client disconnected: %v. Retrying in 5s...", err)
					time.Sleep(5 * time.Second)
				} else {
					// Clean exit
					break
				}
			}
		}()
	}

	// Start Web UI
	go func() {
		if err := web.Start(cfg, *configPath, provider); err != nil {
			log.Printf("Web UI error: %v", err)
		}
	}()

	// Wait for signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down...")
}