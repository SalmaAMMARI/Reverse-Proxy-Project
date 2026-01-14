//The handler uses the loadbalancer to pick a backend , then it receives the request , forwards it to the server and returns teh answer to the client
package proxy

import (
	
	"fmt"
	"log"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"time"
	"reverseproxyproject/Config"
	"reverseproxyproject/internal/models"
)

// ProxyHandler handles incoming HTTP requests and forwards them to backends
type ProxyHandler struct {
	balancer *RoundRobinBalancer
	config   *config.Config
}

func NewProxyHandler(balancer *RoundRobinBalancer, cfg *config.Config) *ProxyHandler {
	return &ProxyHandler{
		balancer: balancer,
		config:   cfg,
	}
}

// StartProxyServer starts the main proxy server
func StartProxyServer(cfg *config.Config, pool *models.ServerPool) {
	balancer := NewRoundRobinBalancer(pool)
	handler := NewProxyHandler(balancer, cfg)
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	
	log.Printf(" Reverse Proxy starting on :%d", cfg.Port)
	log.Printf("Strategy: %s", cfg.Strategy)
	log.Printf(" Backends: %d", len(pool.Backends))
	log.Println("Ready to forward requests!")
	log.Println("------------------------------------------")
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(" Failed to start proxy:", err)
	}
}

// ServeHTTP is the main handler for incoming requests
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	log.Printf(" [%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
	
	backend := p.balancer.GetNextValidPeer()
	if backend == nil {
		p.handleNoBackends(w, r)
		return
	}
	
	//  which backend was selected
	log.Printf(" Forwarding to: %s", backend.URL.String())
	
	// Increment connection count for this backend
	backend.IncrementConnections()
	defer backend.DecrementConnections() // Decrement when done
	
	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", r.URL.Scheme)
		req.Header.Set("X-Proxy-Server", "Go-Reverse-Proxy")
		
		log.Printf("  Forwarding: %s %s", req.Method, req.URL.String())
	}
	//  handle errors 
	
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf(" Proxy error to %s: %v", backend.URL.String(), err)
		
		p.balancer.SetBackendStatus(backend.URL, false)
		
		// Try another backend if available
		log.Println("  Retrying with different backend...")
		backend := p.balancer.GetNextValidPeer()
		if backend != nil {
			newProxy := httputil.NewSingleHostReverseProxy(backend.URL)
			newProxy.ServeHTTP(w, r)
		} else {
			http.Error(w, "Service Unavailable - All backends are down", 
				http.StatusServiceUnavailable)
		}
	}
	
	// successful responses
	proxy.ModifyResponse = func(resp *http.Response) error {
		duration := time.Since(startTime)
		log.Printf("  Response from %s: %d (%v)", 
			backend.URL.String(), resp.StatusCode, duration)
		return nil
	}
	
	//  Forward the request
	proxy.ServeHTTP(w, r)
}

// handleNoBackends handles the case when no backends are available
func (p *ProxyHandler) handleNoBackends(w http.ResponseWriter, r *http.Request) {
	log.Printf(" No backends available for %s %s", r.Method, r.URL.Path)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	
	errorResponse := map[string]interface{}{
		"error":   "Service Unavailable",
		"message": "No backend servers are currently available",
		"time":    time.Now().Format(time.RFC3339),
		"backends_status": map[string]interface{}{
			"total":   p.balancer.GetStatus()["total_backends"],
			"alive":   p.balancer.GetStatus()["alive_backends"],
		},
	}
	
	jsonResponse, _ := json.MarshalIndent(errorResponse, "", "  ")
	w.Write(jsonResponse)
}