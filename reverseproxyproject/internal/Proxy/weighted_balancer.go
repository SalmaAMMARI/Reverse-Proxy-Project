package proxy

import (
	"math/rand"
	"net/url"
	"sync/atomic"
	"time"
)

// WeightedRoundRobinBalancer implements weighted round-robin load balancing
type WeightedRoundRobinBalancer struct {
	pool     *ServerPool
	totalWeight int
}

// NewWeightedRoundRobinBalancer creates a new weighted round-robin balancer
func NewWeightedRoundRobinBalancer(pool *ServerPool) *WeightedRoundRobinBalancer {
	wb := &WeightedRoundRobinBalancer{
		pool: pool,
	}
	wb.updateTotalWeight()
	return wb
}

// GetNextValidPeer returns the next backend using weighted round-robin
func (wb *WeightedRoundRobinBalancer) GetNextValidPeer() *Backend {
	backends := wb.pool.GetBackends()
	
	if len(backends) == 0 {
		return nil
	}
	
	// Filter alive backends
	aliveBackends := make([]*Backend, 0)
	aliveWeights := make([]int, 0)
	
	for _, b := range backends {
		if b.IsAlive() {
			aliveBackends = append(aliveBackends, b)
			aliveWeights = append(aliveWeights, b.GetWeight())
		}
	}
	
	if len(aliveBackends) == 0 {
		return nil
	}
	
	// Calculate total weight of alive backends
	totalAliveWeight := 0
	for _, weight := range aliveWeights {
		totalAliveWeight += weight
	}
	
	if totalAliveWeight == 0 {
		// All weights are zero, fall back to equal distribution
		index := rand.Intn(len(aliveBackends))
		return aliveBackends[index]
	}
	
	// Weighted selection
	selected := rand.Intn(totalAliveWeight)
	current := 0
	
	for i, backend := range aliveBackends {
		current += aliveWeights[i]
		if selected < current {
			return backend
		}
	}
	
	// Fallback to first alive backend
	return aliveBackends[0]
}

// AddBackend adds a new backend to the load balancer
func (wb *WeightedRoundRobinBalancer) AddBackend(backend *Backend) {
	wb.pool.AddBackend(backend)
	wb.updateTotalWeight()
}

// SetBackendStatus updates the health status of a backend
func (wb *WeightedRoundRobinBalancer) SetBackendStatus(backendURL string, alive bool) {
	parsedURL, err := url.Parse(backendURL)
	if err != nil {
		return
	}
	
	backend := wb.pool.GetBackendByURL(parsedURL)
	if backend == nil {
		return
	}
	
	oldStatus := backend.IsAlive()
	backend.SetAlive(alive)
	
	if oldStatus != alive {
		if alive {
			fmt.Printf("Backend %s is now ALIVE (Weight: %d)\n", backendURL, backend.GetWeight())
		} else {
			fmt.Printf("Backend %s is now DEAD!! (Weight: %d)\n", backendURL, backend.GetWeight())
		}
	}
}

// HealthCheck performs a health check on a backend
func (wb *WeightedRoundRobinBalancer) HealthCheck(backend *Backend) {
	client := &http.Client{
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
	
	wb.SetBackendStatus(backend.URL.String(), isAlive)
}

// GetStatus returns current load balancer status
func (wb *WeightedRoundRobinBalancer) GetStatus() map[string]interface{} {
	backends := wb.pool.GetBackends()
	
	backendStatus := make([]map[string]interface{}, len(backends))
	totalWeight := 0
	
	for i, backend := range backends {
		backendStatus[i] = map[string]interface{}{
			"url":                  backend.URL.String(),
			"alive":                backend.IsAlive(),
			"current_connections":  backend.GetConnections(),
			"weight":               backend.GetWeight(),
		}
		if backend.IsAlive() {
			totalWeight += backend.GetWeight()
		}
	}
	
	aliveBackends := 0
	for _, backend := range backends {
		if backend.IsAlive() {
			aliveBackends++
		}
	}
	
	return map[string]interface{}{
		"total_backends":    len(backends),
		"alive_backends":    aliveBackends,
		"total_weight":      totalWeight,
		"strategy":          "weighted-round-robin",
		"current_counter":   atomic.LoadUint64(&wb.pool.Current),
		"backends":          backendStatus,
	}
}

// GetPool returns the server pool
func (wb *WeightedRoundRobinBalancer) GetPool() *ServerPool {
	return wb.pool
}

// RemoveBackend removes a backend from the pool
func (wb *WeightedRoundRobinBalancer) RemoveBackend(backendURL *url.URL) bool {
	removed := wb.pool.RemoveBackend(backendURL)
	if removed {
		wb.updateTotalWeight()
	}
	return removed
}

// updateTotalWeight recalculates the total weight of all backends
func (wb *WeightedRoundRobinBalancer) updateTotalWeight() {
	backends := wb.pool.GetBackends()
	wb.totalWeight = 0
	
	for _, backend := range backends {
		if backend.IsAlive() {
			wb.totalWeight += backend.GetWeight()
		}
	}
}