package main

import (
	"context"
	"encoding/json"
	
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

// ==================== DATA MODELS ====================
type Backend struct {
	URL          *url.URL `json:"url"`
	Alive        bool     `json:"alive"`
	CurrentConns int64    `json:"current_connections"`
}

type ServerPool struct {
	backends []*Backend
	current  uint64
	mu       sync.RWMutex
}

// ==================== LOAD BALANCER ====================
func (s *ServerPool) GetNextValidPeer() *Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.backends) == 0 {
		return nil
	}

	// Round-robin with health check
	for i := 0; i < len(s.backends); i++ {
		next := atomic.AddUint64(&s.current, 1)
		index := int(next % uint64(len(s.backends)))
		backend := s.backends[index]
		
		if backend.Alive {
			return backend
		}
	}
	return nil
}

func (s *ServerPool) AddBackend(backendURL string) error {
	parsedURL, err := url.Parse(backendURL)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.backends = append(s.backends, &Backend{
		URL:   parsedURL,
		Alive: true,
	})
	s.mu.Unlock()
	
	log.Printf("Added backend: %s", backendURL)
	return nil
}

func (s *ServerPool) SetBackendStatus(backendURL string, alive bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, b := range s.backends {
		if b.URL.String() == backendURL {
			wasAlive := b.Alive
			b.Alive = alive
			
			if wasAlive && !alive {
				log.Printf("Backend %s is now DOWN", backendURL)
			} else if !wasAlive && alive {
				log.Printf("Backend %s is now UP", backendURL)
			}
			break
		}
	}
}

func (s *ServerPool) GetBackends() []*Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backends
}

// ==================== HEALTH CHECKER ====================
func startHealthChecker(pool *ServerPool) {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	
	go func() {
		for range ticker.C {
			backends := pool.GetBackends()
			for _, backend := range backends {
				go func(b *Backend) {
					client := &http.Client{Timeout: 5 * time.Second}
					
					// Try to ping the backend
					resp, err := client.Get(b.URL.String())
					if err != nil {
						pool.SetBackendStatus(b.URL.String(), false)
						return
					}
					defer resp.Body.Close()
					
					if resp.StatusCode < 200 || resp.StatusCode >= 400 {
						pool.SetBackendStatus(b.URL.String(), false)
					} else {
						pool.SetBackendStatus(b.URL.String(), true)
					}
				}(backend)
			}
		}
	}()
}

// ==================== REVERSE PROXY HANDLER ====================
type ProxyHandler struct {
	pool        *ServerPool
	rateLimiter *rate.Limiter
}

func NewProxyHandler(pool *ServerPool, rps int) *ProxyHandler {
	var limiter *rate.Limiter
	if rps > 0 {
		limiter = rate.NewLimiter(rate.Limit(rps), rps*2)
	}
	
	return &ProxyHandler{
		pool:        pool,
		rateLimiter: limiter,
	}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Rate limiting
	if h.rateLimiter != nil && !h.rateLimiter.Allow() {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	// Get backend
	backend := h.pool.GetNextValidPeer()
	if backend == nil {
		http.Error(w, "Service Unavailable - No healthy backends", http.StatusServiceUnavailable)
		return
	}

	// Increment connection count
	atomic.AddInt64(&backend.CurrentConns, 1)
	defer atomic.AddInt64(&backend.CurrentConns, -1)

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	
	// Add custom headers
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = backend.URL.Scheme
		req.URL.Host = backend.URL.Host
		req.Host = backend.URL.Host
		
		// Add proxy headers
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Proxy-Server", "Go-Reverse-Proxy/1.0")
	}

	// Error handling
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for backend %s: %v", backend.URL, err)
		h.pool.SetBackendStatus(backend.URL.String(), false)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// Serve the request
	proxy.ServeHTTP(w, r)
}

// ==================== ADMIN API ====================
type AdminAPI struct {
	pool *ServerPool
}

func (a *AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.URL.Path {
	case "/status":
		a.handleStatus(w, r)
	case "/add":
		a.handleAddBackend(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (a *AdminAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	backends := a.pool.GetBackends()
	
	active := 0
	for _, b := range backends {
		if b.Alive {
			active++
		}
	}
	
	response := map[string]interface{}{
		"total_backends":   len(backends),
		"active_backends":  active,
		"backends":         backends,
		"timestamp":        time.Now().Format(time.RFC3339),
	}
	
	json.NewEncoder(w).Encode(response)
}

func (a *AdminAPI) handleAddBackend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var data struct {
		URL string `json:"url"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	if err := a.pool.AddBackend(data.URL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	response := map[string]string{
		"message": "Backend added successfully",
		"url":     data.URL,
	}
	
	json.NewEncoder(w).Encode(response)
}

// ==================== MAIN FUNCTION ====================
func main() {
	log.Println("Starting Go Reverse Proxy Server...")
	
	// Create server pool
	pool := &ServerPool{}
	
	// Add default backends
	pool.AddBackend("http://localhost:9091")
	pool.AddBackend("http://localhost:9092")
	
	// Start health checker
	startHealthChecker(pool)
	
	// Create handlers
	proxyHandler := NewProxyHandler(pool, 100) // 100 requests per second limit
	adminAPI := &AdminAPI{pool: pool}
	
	// Create servers
	proxyServer := &http.Server{
		Addr:         ":8000",
		Handler:      proxyHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	
	adminServer := &http.Server{
		Addr:         ":8082",
		Handler:      adminAPI,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	
	// Start servers in goroutines
	go func() {
		log.Println("Reverse Proxy listening on :8000")
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Proxy server error: %v", err)
		}
	}()
	
	go func() {
		log.Println("Admin API listening on :9092")
		log.Println("  GET  /status  - Check backend status")
		log.Println("  POST /add     - Add new backend (JSON: {\"url\": \"http://...\"})")
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Admin server error: %v", err)
		}
	}()
	
	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	<-quit
	log.Println("Shutting down servers...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	var wg sync.WaitGroup
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		if err := proxyServer.Shutdown(ctx); err != nil {
			log.Printf("Proxy shutdown error: %v", err)
		}
	}()
	
	go func() {
		defer wg.Done()
		if err := adminServer.Shutdown(ctx); err != nil {
			log.Printf("Admin shutdown error: %v", err)
		}
	}()
	
	wg.Wait()
	log.Println("Servers stopped gracefully")
}
