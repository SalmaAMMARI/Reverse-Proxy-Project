package proxy

import (
	"fmt"
	"log"
	"time"
	"reverse-proxy-project/internal/models"
)

// HealthChecker runs periodic health checks on backends
type HealthChecker struct {
	balancer *RoundRobinBalancer
	interval time.Duration
	stopChan chan bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(balancer *RoundRobinBalancer, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		balancer: balancer,
		interval: interval,
		stopChan: make(chan bool),
	}
}

// Start begins the periodic health checking
func (hc *HealthChecker) Start() {
	log.Printf(" Health checker starting (checking every %v)", hc.interval)
	
	ticker := time.NewTicker(hc.interval)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				hc.checkAllBackends()
			case <-hc.stopChan:
				ticker.Stop()
				log.Println(" Health checker stopped")
				return
			}
		}
	}()
}

//  halt the health checker
func (hc *HealthChecker) Stop() {
	hc.stopChan <- true
}

// check  every backend in the pool
func (hc *HealthChecker) checkAllBackends() {
	log.Printf(" Checking all backends...")
	
	
	status := hc.balancer.GetStatus()
	beforeAlive := status["alive_backends"].(int)
	
	backends := hc.balancer.pool.GetBackends()
	checked := 0
	
	for _, backend := range backends {
		hc.checkSingleBackend(backend)
		checked++
	}
	
	status = hc.balancer.GetStatus()
	afterAlive := status["alive_backends"].(int)
	
	log.Printf("Health check complete: %d backends checked", checked)
	
	if beforeAlive != afterAlive {
		log.Printf(" Status change: %d → %d alive backends", 
			beforeAlive, afterAlive)
	}
}

//  health check on one backend
func (hc *HealthChecker) checkSingleBackend(backend *models.Backend) {
	// Use the HealthCheck method from loadbalancer.go
	hc.balancer.HealthCheck(backend)
}

//  statistics about health checks
func (hc *HealthChecker) GetHealthCheckStats() map[string]interface{} {
	status := hc.balancer.GetStatus()
	
	return map[string]interface{}{
		"health_check_interval": hc.interval.String(),
		"total_backends":        status["total_backends"],
		"alive_backends":        status["alive_backends"],
		"last_check":            time.Now().Format(time.RFC3339),
	}
}