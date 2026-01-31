package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Session holds information about a sticky session
type Session struct {
	Backend   *Backend
	CreatedAt time.Time
	LastUsed  time.Time
}

// SessionManager manages sticky sessions
type SessionManager struct {
	// Session storage
	sessions    map[string]*Session
	sessionTTL  time.Duration
	cleanupInterval time.Duration
	mu          sync.RWMutex
	stopChan    chan bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(sessionTTL time.Duration) *SessionManager {
	if sessionTTL <= 0 {
		sessionTTL = 30 * time.Minute
	}
	
	return &SessionManager{
		sessions:        make(map[string]*Session),
		sessionTTL:      sessionTTL,
		cleanupInterval: 5 * time.Minute,
		stopChan:        make(chan bool),
	}
}

// GetBackendForRequest returns the backend for an existing session
func (sm *SessionManager) GetBackendForRequest(r *http.Request, pool *ServerPool) *Backend {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Try to get session ID from cookie first
	if cookie, err := r.Cookie("proxy_session"); err == nil && cookie.Value != "" {
		if session, exists := sm.sessions[cookie.Value]; exists {
			// Check if session is still valid
			if time.Since(session.LastUsed) < sm.sessionTTL {
				// Check if backend is still alive
				if session.Backend.IsAlive() {
					session.LastUsed = time.Now()
					return session.Backend
				}
			}
		}
	}
	
	// Try IP-based session as fallback
	ip := sm.extractClientIP(r)
	if session, exists := sm.sessions[ip]; exists {
		if time.Since(session.LastUsed) < sm.sessionTTL && session.Backend.IsAlive() {
			session.LastUsed = time.Now()
			return session.Backend
		}
	}
	
	return nil
}

// SetSession creates or updates a session for the client
func (sm *SessionManager) SetSession(w http.ResponseWriter, r *http.Request, backend *Backend) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sessionID := sm.generateSessionID(r)
	
	// Create or update session
	sm.sessions[sessionID] = &Session{
		Backend:   backend,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	
	// Also create IP-based session as backup
	ip := sm.extractClientIP(r)
	sm.sessions[ip] = &Session{
		Backend:   backend,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	
	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "proxy_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(sm.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionForBackend removes all sessions pointing to a dead backend
func (sm *SessionManager) ClearSessionForBackend(backend *Backend) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	for key, session := range sm.sessions {
		if session.Backend.GetID() == backend.GetID() {
			delete(sm.sessions, key)
		}
	}
}

// StartCleanup begins periodic cleanup of expired sessions
func (sm *SessionManager) StartCleanup() {
	ticker := time.NewTicker(sm.cleanupInterval)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				sm.cleanupExpiredSessions()
			case <-sm.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// StopCleanup halts the cleanup goroutine
func (sm *SessionManager) StopCleanup() {
	select {
	case sm.stopChan <- true:
	default:
	}
}

// cleanupExpiredSessions removes sessions that have expired
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	removed := 0
	
	for key, session := range sm.sessions {
		if now.Sub(session.LastUsed) > sm.sessionTTL {
			delete(sm.sessions, key)
			removed++
		}
	}
	
	if removed > 0 {
		fmt.Printf("Cleaned up %d expired sessions\n", removed)
	}
}

// extractClientIP extracts the client IP from the request
func (sm *SessionManager) extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (if behind another proxy)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	
	// Extract from RemoteAddr
	remoteAddr := r.RemoteAddr
	if colonIndex := strings.LastIndex(remoteAddr, ":"); colonIndex != -1 {
		return remoteAddr[:colonIndex]
	}
	
	return remoteAddr
}

// generateSessionID creates a unique session ID
func (sm *SessionManager) generateSessionID(r *http.Request) string {
	// Combine client IP and current time for uniqueness
	input := sm.extractClientIP(r) + time.Now().String() + r.UserAgent()
	
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter ID
}

// GetStats returns session manager statistics
func (sm *SessionManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	activeSessions := 0
	now := time.Now()
	
	for _, session := range sm.sessions {
		if now.Sub(session.LastUsed) <= sm.sessionTTL {
			activeSessions++
		}
	}
	
	return map[string]interface{}{
		"total_sessions":    len(sm.sessions),
		"active_sessions":   activeSessions,
		"session_ttl":       sm.sessionTTL.String(),
		"cleanup_interval":  sm.cleanupInterval.String(),
	}
}