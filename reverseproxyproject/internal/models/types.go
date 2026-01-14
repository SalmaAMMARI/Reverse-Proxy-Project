package models

import (
    "net/url"
    "sync"
    "sync/atomic"
)

// Backend represents a single backend server in the pool
type Backend struct {
    // URL is the address of the backend server 
    URL *url.URL `json:"url"`
    
    // Alive indicates whether the backend is currently healthy and responsive
    Alive bool `json:"alive"`
    
    // CurrentConnections tracks the number of active connections to this backend
    CurrentConnections int64 `json:"current_connections"`
    
    mu sync.RWMutex
}

// SetAlive safely updates the Alive status of the backend
func (b *Backend) SetAlive(alive bool) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.Alive = alive
}

// IsAlive safely reads the Alive status of the backend
func (b *Backend) IsAlive() bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.Alive
}

// IncrementConnections atomically increases the connection count
func (b *Backend) IncrementConnections() {
    atomic.AddInt64(&b.CurrentConnections, 1)
}

// DecrementConnections atomically decreases the connection count
func (b *Backend) DecrementConnections() {
    atomic.AddInt64(&b.CurrentConnections, -1)
}

// GetConnections atomically reads the connection count
func (b *Backend) GetConnections() int64 {
    return atomic.LoadInt64(&b.CurrentConnections)
}

// ServerPool holds a collection of backend servers
type ServerPool struct {
    
    Backends []*Backend `json:"backends"`
    
    // Current is an atomic counter used for Round-Robin load balancing
    // It tracks which backend should be selected next
    Current uint64 `json:"current"`
    
    mu sync.RWMutex
}

func (sp *ServerPool) AddBackend(backend *Backend) {
    sp.mu.Lock()
    defer sp.mu.Unlock()
    sp.Backends = append(sp.Backends, backend)
}
func (sp *ServerPool) RemoveBackend(targetURL *url.URL) bool{
    sp.mu.Lock()
    defer sp.mu.Unlock()
    
    for i, backend := range sp.Backends {
        if backend.URL.String() == targetURL.String() {
            sp.Backends = append(sp.Backends[:i], sp.Backends[i+1:]...)
            return true
        }
    }
    return false 
}

//Get all the backend s of the server Pool
func (sp *ServerPool) GetBackends() []*Backend {
    sp.mu.RLock()
    defer sp.mu.RUnlock()
    
    // Return a copy to avoid concurrent modification issues
    backends := make([]*Backend, len(sp.Backends))
    copy(backends, sp.Backends)
    return backends
}

// Get a backend by its URL
func (sp *ServerPool) GetBackendByURL(targetURL *url.URL) *Backend {
    sp.mu.RLock()
    defer sp.mu.RUnlock()
    
    for _, backend := range sp.Backends {
        if backend.URL.String() == targetURL.String() {
            return backend
        }
    }
    return nil
}

// Returns the num of the backends in the server Pool
func (sp *ServerPool) Count() int {
    sp.mu.RLock()
    defer sp.mu.RUnlock()
    return len(sp.Backends)
}

// Returns the number of the alive backends 
func (sp *ServerPool) CountAlive() int {
    sp.mu.RLock()
    defer sp.mu.RUnlock()
    
    count := 0
    for _, backend := range sp.Backends {
        if backend.IsAlive() {
            count++
        }
    }
    return count
}

type LoadBalancer interface {
    // GetNextValidPeer returns the next available backend server
    // It should only return backends that are marked as Alive
    // Returns nil if no alive backends are available
    GetNextValidPeer() *Backend
    
    // AddBackend adds a new backend server to the load balancer
    AddBackend(backend *Backend)
    
    // SetBackendStatus updates the health status of a backend
    // alive: true if the backend is healthy, false if it's down
    SetBackendStatus(url *url.URL, alive bool)
}