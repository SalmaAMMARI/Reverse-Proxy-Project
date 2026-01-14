package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reverseproxyproject/internal/models"
	"reverseproxyproject/internal/Proxy"
)

// AdminAPI handles the administrative interface
type AdminAPI struct {
	balancer      *proxy.RoundRobinBalancer
	healthChecker *proxy.HealthChecker
	port          int
}

func NewAdminAPI(balancer *proxy.RoundRobinBalancer, healthChecker *proxy.HealthChecker, port int) *AdminAPI {
	return &AdminAPI{
		balancer:      balancer,
		healthChecker: healthChecker,
		port:          port,
	}
}

// Start begins the Admin API server
func (api *AdminAPI) Start() {
	//  routes
	http.HandleFunc("/", api.handleRoot)
	http.HandleFunc("/status", api.handleStatus)
	http.HandleFunc("/health", api.handleHealth)
	http.HandleFunc("/backends", api.handleBackends)

	addr := fmt.Sprintf(":%d", api.port)
	log.Printf("Admin API starting on port %d", api.port)
	log.Printf("Endpoints:")
	log.Printf("GET  %s/status", addr)
	log.Printf("GET  %s/health", addr)
	log.Printf("POST %s/backends - Add backend", addr)
	log.Printf("DELETE %s/backends - Remove backend", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("Failed to start Admin API:", err)
	}
}

// handleRoot shows available endpoints
func (api *AdminAPI) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	endpoints := map[string]interface{}{
		"endpoints": map[string]string{
			"GET /status":    "Get proxy status and backend list",
			"GET /health":    "Get health checker status",
			"POST /backends": "Add a new backend (JSON: {\"url\": \"http://...\"})",
			"DELETE /backends": "Remove a backend (JSON: {\"url\": \"http://...\"})",
		},
		"documentation": "Reverse Proxy Admin API",
	}

	json.NewEncoder(w).Encode(endpoints)
}

// handleStatus returns current proxy status
func (api *AdminAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := api.balancer.GetStatus()
	json.NewEncoder(w).Encode(status)
}

// handleHealth returns health checker status
func (api *AdminAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var healthStats map[string]interface{}
	if api.healthChecker != nil {
		healthStats = api.healthChecker.GetHealthCheckStats()
	} else {
		healthStats = map[string]interface{}{
			"message": "Health checker not initialized",
		}
	}

	json.NewEncoder(w).Encode(healthStats)
}

// handleBackends handles adding/removing backends
func (api *AdminAPI) handleBackends(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		api.addBackend(w, r)
	case http.MethodDelete:
		api.removeBackend(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// addBackend adds a new backend to the pool
func (api *AdminAPI) addBackend(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if request.URL == "" {
		http.Error(w, `{"error": "URL is required"}`, http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid URL: %v"}`, err), http.StatusBadRequest)
		return
	}

	backend := &models.Backend{
		URL:   parsedURL,
		Alive: true, 
	}

	api.balancer.AddBackend(backend)

	response := map[string]interface{}{
		"message": "Backend added successfully",
		"backend": map[string]interface{}{
			"url":   backend.URL.String(),
			"alive": backend.IsAlive(),
		},
		"total_backends": api.balancer.GetStatus()["total_backends"],
	}

	json.NewEncoder(w).Encode(response)
}

// removeBackend removes a backend from the pool
func (api *AdminAPI) removeBackend(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if request.URL == "" {
		http.Error(w, `{"error": "URL is required"}`, http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid URL: %v"}`, err), http.StatusBadRequest)
		return
	}

	statusBefore := api.balancer.GetStatus()
	totalBefore := statusBefore["total_backends"].(int)

	removed := api.balancer.RemoveBackend(parsedURL)

	if !removed {
		http.Error(w, `{"error": "Backend not found"}`, http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"message":         "Backend removed successfully",
		"removed_backend": request.URL,
		"before_total":    totalBefore,
		"after_total":     api.balancer.GetStatus()["total_backends"],
	}

	json.NewEncoder(w).Encode(response)
}

