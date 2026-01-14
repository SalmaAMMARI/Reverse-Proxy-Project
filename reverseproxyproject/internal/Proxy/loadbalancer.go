package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
	"reverseproxyproject/internal/models" 
)

// RoundRobinBalancer implements the LoadBalancer interface with round-robin strategy
type RoundRobinBalancer struct {
	pool *models.ServerPool
}

// NewRoundRobinBalancer creates a new round-robin load balancer
func NewRoundRobinBalancer(pool *models.ServerPool) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		pool: pool,
	}
}
func (rr *RoundRobinBalancer) GetPool() *models.ServerPool {
	return rr.pool
}
func (rr *RoundRobinBalancer) RemoveBackend(backendUrl *url.URL) bool {
    return  rr.pool.RemoveBackend(backendUrl)
	
}

// GetNextValidPeer returns the next alive backend using round-robin algorithm
func (rr *RoundRobinBalancer) GetNextValidPeer() *models.Backend {
	// Get a snapshot of all backends (the whole pool)
	backends := rr.pool.GetBackends()
	
	if len(backends) == 0 {
		return nil 
	}

	// finding  an alive backend
	attempts := 0
	totalBackends := len(backends)
	
	for attempts < totalBackends {
		
		currentIndex := atomic.AddUint64(&rr.pool.Current, 1)
		
		// Calculate which backend to use
		// Using modulo (%) to wrap around when we reach the end
		backendIndex := int((currentIndex - 1) % uint64(totalBackends))
		
		
		backend := backends[backendIndex]
		
		//  Check if the backend is alive
		if backend.IsAlive() {
			return backend 
		}
		
		// If not alive try next one
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
func (rr *RoundRobinBalancer) SetBackendStatus(backendURL *url.URL, alive bool) {
	backend := rr.pool.GetBackendByURL(backendURL)
	if backend == nil {
		fmt.Printf("Cannot update status: backend %s not found\n", backendURL.String())
		return
	}
	
	oldStatus := backend.IsAlive()
	backend.SetAlive(alive)
	
	if oldStatus != alive {
		if alive {
			fmt.Printf("Backend %s is now ALIVE\n", backendURL.String())
		} else {
			fmt.Printf("Backend %s is now DEAD!!\n", backendURL.String())
		}
	}
}


// HealthCheck performs a health check on a single backend
func (rr *RoundRobinBalancer) HealthCheck(backend *models.Backend) {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	
	healthURL := backend.URL.String() + "/health" // check the health endpoint
	if backend.URL.Path == "" {
		healthURL = backend.URL.String() + "/" // Just check if server responds
	}
	
	resp, err := client.Get(healthURL)
	isAlive := false
	
	if err == nil && resp.StatusCode < 500 {
		isAlive = true
		resp.Body.Close()
	}
	
	
	rr.SetBackendStatus(backend.URL, isAlive)
}


// GetStatus returns current load balancer status for monitoring
func (rr *RoundRobinBalancer) GetStatus() map[string]interface{} {
	backends := rr.pool.GetBackends()
	
	backendStatus := make([]map[string]interface{}, len(backends))
	for i, backend := range backends {
		backendStatus[i] = map[string]interface{}{
			"url":                  backend.URL.String(),
			"alive":                backend.IsAlive(),
			"current_connections":  backend.GetConnections(),
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