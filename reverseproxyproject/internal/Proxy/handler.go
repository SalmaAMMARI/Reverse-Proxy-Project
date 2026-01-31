package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
	config "reverseproxyproject/Config"
)

// ProxyHandler handles incoming HTTP requests and forwards them to backends
type ProxyHandler struct {
	balancer       LoadBalancerInterface
	config         *config.Config
	sessionManager *SessionManager
}

// LoadBalancerInterface extends the base interface for proxy handler
type LoadBalancerInterface interface {
	GetNextValidPeer() *Backend
	AddBackend(backend *Backend)
	SetBackendStatus(url string, alive bool)
	GetPool() *ServerPool
	GetStatus() map[string]interface{}
}

func NewProxyHandler(balancer LoadBalancerInterface, cfg *config.Config) *ProxyHandler {
	handler := &ProxyHandler{
		balancer: balancer,
		config:   cfg,
	}
	
	// Initialize session manager if sticky sessions are enabled
	if cfg.StickySessions {
		handler.sessionManager = NewSessionManager(30 * time.Minute)
		go handler.sessionManager.StartCleanup()
	}
	
	return handler
}

// StartProxyServer starts the main proxy server
func StartProxyServer(cfg *config.Config, balancer LoadBalancerInterface) {
	handler := NewProxyHandler(balancer, cfg)
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	
	log.Printf("Reverse Proxy starting on :%d", cfg.Port)
	log.Printf("Strategy: %s", cfg.Strategy)
	log.Printf("Sticky Sessions: %v", cfg.StickySessions)
	log.Printf("HTTPS Enabled: %v", cfg.EnableHTTPS)
	
	pool := balancer.GetPool()
	if pool != nil {
		log.Printf("Backends: %d", len(pool.GetBackends()))
	}
	log.Println("Ready to forward requests!")
	log.Println("------------------------------------------")
	
	if cfg.EnableHTTPS && cfg.CertFile != "" && cfg.KeyFile != "" {
		log.Printf("Starting HTTPS server with cert: %s, key: %s", cfg.CertFile, cfg.KeyFile)
		if err := server.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTPS proxy:", err)
		}
	} else {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start proxy:", err)
		}
	}
}

// ServeHTTP is the main handler for incoming requests
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
	
	var backend *Backend
	
	// Check for sticky session first if enabled
	if p.config.StickySessions && p.sessionManager != nil {
		backend = p.sessionManager.GetBackendForRequest(r, p.balancer.GetPool())
		if backend != nil {
			log.Printf("Using sticky session for backend: %s", backend.URL.String())
		}
	}
	
	// If no sticky session, use load balancer
	if backend == nil {
		backend = p.balancer.GetNextValidPeer()
		if backend == nil {
			p.handleNoBackends(w, r)
			return
		}
		
		// Create new sticky session if enabled
		if p.config.StickySessions && p.sessionManager != nil {
			p.sessionManager.SetSession(w, r, backend)
			log.Printf("Created new sticky session for backend: %s", backend.URL.String())
		}
	}
	
	// Log which backend was selected
	log.Printf("Forwarding to: %s", backend.URL.String())
	
	// Increment connection count for this backend
	backend.IncrementConnections()
	defer backend.DecrementConnections() // Decrement when done
	
	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	
	// Customize the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// Add proxy headers
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)
		scheme := r.URL.Scheme
		if scheme == "" {
			if p.config.EnableHTTPS {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		req.Header.Set("X-Forwarded-Proto", scheme)
		req.Header.Set("X-Proxy-Server", "Go-Reverse-Proxy")
		
		// Add sticky session info if enabled
		if p.config.StickySessions && p.sessionManager != nil {
			if cookie, err := r.Cookie("proxy_session"); err == nil {
				req.Header.Set("X-Sticky-Session-ID", cookie.Value)
			}
		}
		
		log.Printf("  Forwarding: %s %s", req.Method, req.URL.String())
	}
	
	// Handle proxy errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error to %s: %v", backend.URL.String(), err)
		
		// Mark backend as dead
		p.balancer.SetBackendStatus(backend.URL.String(), false)
		
		// Clear sticky session if enabled
		if p.config.StickySessions && p.sessionManager != nil {
			p.sessionManager.ClearSessionForBackend(backend)
		}
		
		// Try another backend if available
		log.Println("  Retrying with different backend...")
		newBackend := p.balancer.GetNextValidPeer()
		if newBackend != nil {
			// Update sticky session to new backend
			if p.config.StickySessions && p.sessionManager != nil {
				p.sessionManager.SetSession(w, r, newBackend)
			}
			
			newProxy := httputil.NewSingleHostReverseProxy(newBackend.URL)
			newProxy.ServeHTTP(w, r)
		} else {
			http.Error(w, "Service Unavailable - All backends are down", 
				http.StatusServiceUnavailable)
		}
	}
	
	// Log successful responses
	proxy.ModifyResponse = func(resp *http.Response) error {
		duration := time.Since(startTime)
		log.Printf("  Response from %s: %d (%v)", 
			backend.URL.String(), resp.StatusCode, duration)
		
		// Add backend info header for debugging
		resp.Header.Set("X-Backend-Served-By", backend.URL.String())
		if p.config.StickySessions {
			resp.Header.Set("X-Sticky-Session", "enabled")
		}
		
		return nil
	}
	
	// Forward the request
	proxy.ServeHTTP(w, r)
}

// handleNoBackends handles the case when no backends are available
func (p *ProxyHandler) handleNoBackends(w http.ResponseWriter, r *http.Request) {
	log.Printf("No backends available for %s %s", r.Method, r.URL.Path)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	
	status := p.balancer.GetStatus()
	
	errorResponse := map[string]interface{}{
		"error":   "Service Unavailable",
		"message": "No backend servers are currently available",
		"time":    time.Now().Format(time.RFC3339),
		"backends_status": map[string]interface{}{
			"total":   status["total_backends"],
			"alive":   status["alive_backends"],
		},
		"sticky_sessions": p.config.StickySessions,
		"https_enabled":   p.config.EnableHTTPS,
	}
	
	jsonResponse, _ := json.MarshalIndent(errorResponse, "", "  ")
	w.Write(jsonResponse)
}