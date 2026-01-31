package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
	"reverseproxyproject/internal/models"
)

// Backend extends models.Backend for proxy package
type Backend struct {
	*models.Backend
}

// ServerPool extends models.ServerPool for proxy package
type ServerPool struct {
	*models.ServerPool
}

// RoundRobinBalancer implements the LoadBalancer interface with round-robin strategy
type RoundRobinBalancer struct {
	pool *ServerPool
}

// NewRoundRobinBalancer creates a new round-robin load balancer
func NewRoundRobinBalancer(pool *models.ServerPool) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		pool: &ServerPool{pool},
	}
}

// GetPool returns the server pool
func (rr *RoundRobinBalancer) GetPool() *ServerPool {
	return rr.pool
}

// RemoveBackend removes a backend from the pool
func (rr *RoundRobinBalancer) RemoveBackend(backendUrl *url.URL) bool {
	return rr.pool.RemoveBackend(backendUrl)
}

// GetNextValidPeer returns the next alive backend using round-robin algorithm
func (rr *RoundRobinBalancer) GetNextValidPeer() *Backend {
	// Get a snapshot of all backends
	modelsBackends := rr.pool.GetBackends()
	backends := make([]*Backend, len(modelsBackends))
	for i, b := range modelsBackends {
		backends[i] = &Backend{b}
	}
	
	if len(backends) == 0 {
		return nil
	}

	// Find an alive backend
	attempts := 0
	totalBackends := len(backends)
	
	for attempts < totalBackends {
		// Atomically increment and get the current index
		currentIndex := atomic.AddUint64(&rr.pool.Current, 1)
		
		// Calculate which backend to use
		backendIndex := int((currentIndex - 1) % uint64(totalBackends))
		
		backend := backends[backendIndex]
		
		// Check if the backend is alive
		if backend.IsAlive() {
			return backend
		}
		
		// If not alive, try next one
		attempts++
	}

	return nil
}

// AddBackend adds a new backend to the load balancer
func (rr *RoundRobinBalancer) AddBackend(backend *models.Backend) {
	fmt.Printf("Adding new backend: %s\n", backend.URL.String())
	rr.pool.AddBackend(backend)
}

// SetBackendStatus updates the health status of a backend
func (rr *RoundRobinBalancer) SetBackendStatus(backendURL string, alive bool) {
	parsedURL, err := url.Parse(backendURL)
	if err != nil {
		fmt.Printf("Invalid URL: %s\n", backendURL)
		return
	}
	
	backend := rr.pool.GetBackendByURL(parsedURL)
	if backend == nil {
		fmt.Printf("Cannot update status: backend %s not found\n", backendURL)
		return
	}
	
	oldStatus := backend.IsAlive()
	backend.SetAlive(alive)
	
	if oldStatus != alive {
		if alive {
			fmt.Printf("Backend %s is now ALIVE\n", backendURL)
		} else {
			fmt.Printf("Backend %s is now DEAD!!\n", backendURL)
		}
	}
}

// HealthCheck performs a health check on a single backend
func (rr *RoundRobinBalancer) HealthCheck(backend *Backend) {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	
	healthURL := backend.URL.String() + "/health"
	if backend.URL.Path == "" {
		healthURL = backend.URL.String() + "/"
	}
	
	resp, err := client.Get(healthURL)
	isAlive := false
	
	if err == nil && resp.StatusCode < 500 {
		isAlive = true
		resp.Body.Close()
	}
	
	rr.SetBackendStatus(backend.URL.String(), isAlive)
}

// GetStatus returns current load balancer status for monitoring
func (rr *RoundRobinBalancer) GetStatus() map[string]interface{} {
	modelsBackends := rr.pool.GetBackends()
	backends := make([]*Backend, len(modelsBackends))
	for i, b := range modelsBackends {
		backends[i] = &Backend{b}
	}
	
	backendStatus := make([]map[string]interface{}, len(backends))
	for i, backend := range backends {
		backendStatus[i] = map[string]interface{}{
			"url":                  backend.URL.String(),
			"alive":                backend.IsAlive(),
			"current_connections":  backend.GetConnections(),
			"weight":               backend.GetWeight(),
		}
	}
	
	return map[string]interface{}{
		"total_backends":   len(backends),
		"alive_backends":   rr.pool.CountAlive(),
		"strategy":         "round-robin",
		"current_counter":  atomic.LoadUint64(&rr.pool.Current),
		"backends":         backendStatus,
	}
}