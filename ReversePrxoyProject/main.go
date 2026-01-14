package main

import (
	"log"
	"net/url"
	"time"

	"reverse-proxy-project/Config"
	"reverse-proxy-project/internal/admin"
	"reverse-proxy-project/internal/models"
	"reverse-proxy-project/internal/proxy"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Create server pool
	pool := &models.ServerPool{}

	// Add backends from config
	for _, backendURLStr := range cfg.Backends {
		parsedURL, err := url.Parse(backendURLStr)
		if err != nil {
			log.Printf("Invalid backend URL %s: %v", backendURLStr, err)
			continue
		}
		backend := &models.Backend{
			URL:   parsedURL,
			Alive: true,
		}
		pool.AddBackend(backend)
	}

	// Create load balancer
	balancer := proxy.NewRoundRobinBalancer(pool)

	// Create and start health checker
	healthChecker := proxy.NewHealthChecker(balancer, 10*time.Second)
	go healthChecker.Start()

	// Start proxy server
	go proxy.StartProxyServer(cfg, pool)

	// Create and start admin API
	adminAPI := admin.NewAdminAPI(balancer, healthChecker, 8081)
	adminAPI.Start() 
}
